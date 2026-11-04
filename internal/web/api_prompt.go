package web

import (
	"net/http"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// PromptReader reads recorded bodies, never sends or records a prompt.
type PromptReader interface {
	PromptHistory(project, definition, place, member, running string) (PromptHistory, error)
}

// PromptRun identifies a live instance and its execution project.
type PromptRun struct {
	Team    string `json:"team"`
	Project string `json:"project"`
}

// RecordedPrompt is the exact blob body, without a sending preamble.
type RecordedPrompt struct {
	Text    string `json:"text"`
	Command string `json:"command"`
	Time    string `json:"time"`
}

// PromptHistory distinguishes absent, unreadable and recorded histories.
type PromptHistory struct {
	Runs     []PromptRun     `json:"runs"`
	Selected *PromptRun      `json:"selected"`
	Status   string          `json:"status"`
	Reason   string          `json:"reason"`
	Recorded *RecordedPrompt `json:"recorded"`
}
type promptComparisonIn struct {
	apiIn
	Member      string `json:"member"`
	RunningTeam string `json:"running_team"`
}
type promptComparison struct {
	PromptHistory
	Segments   []definition.PromptSegment `json:"segments"`
	Current    *string                    `json:"current"`
	Problems   []apiProblem               `json:"problems"`
	Diff       []PromptDiffLine           `json:"diff"`
	Project    string                     `json:"project"`
	Executable string                     `json:"executable"`
	Language   string                     `json:"language"`
	Team       string                     `json:"team"`
	Member     string                     `json:"member"`
	Draft      bool                       `json:"draft"`
}

func (s *Server) apiPromptComparison(w http.ResponseWriter, r *http.Request) {
	if !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[promptComparisonIn](w, r)
	if !ok {
		return
	}
	scoped, refusal := s.withProject(in.Project)
	if refusal != nil {
		apiAnswer(w, nil, refusal)
		return
	}
	if in.Place != "global" && in.Place != "project" {
		apiAnswer(w, nil, refuse(422, "置き場所を選んでください"))
		return
	}
	if in.Name == "" || in.Name == "." || in.Name == ".." || filepath.Base(in.Name) != in.Name || in.Member == "" {
		apiAnswer(w, nil, refuse(422, "定義名とメンバー名を確認してください"))
		return
	}
	out, refusal := scoped.comparePrompt(in)
	apiAnswer(w, out, refusal)
}

func (s *Server) comparePrompt(in promptComparisonIn) (*promptComparison, *apiErr) {
	body, refusal := t2DecodeOps(in.apiIn)
	if refusal != nil {
		return nil, refusal
	}
	out := &promptComparison{
		PromptHistory: PromptHistory{Runs: []PromptRun{}, Status: "missing", Reason: "このメンバーへ渡した本文の記録がありません"},
		Segments:      []definition.PromptSegment{},
		Problems:      []apiProblem{}, Diff: []PromptDiffLine{}, Team: in.Name, Member: in.Member,
		Draft:      len(body.ops) > 0 || in.New || in.Base != nil,
		Executable: promptExecutable(), Language: i18n.Language(), Project: s.cat.Base(),
	}
	if !in.New && in.Base == nil {
		if reader, ok := s.act.(PromptReader); ok {
			history, err := reader.PromptHistory(s.cat.Base(), in.Name, in.Place, in.Member, in.RunningTeam)
			if err != nil {
				out.Status = "error"
				out.Reason = err.Error()
			} else {
				out.PromptHistory = history
			}
		}
	}
	if out.Selected != nil {
		scoped, failure := s.withProject(out.Selected.Project)
		if failure != nil {
			return nil, failure
		}
		s = scoped
		out.Project = s.cat.Base()
		out.Team = out.Selected.Team
	}
	cat, refusal := s.promptCatalog(in)
	if refusal != nil {
		return nil, refusal
	}
	report := check.Team(check.TeamInput{Catalog: cat, Name: in.Name, Layer: catalog.Layer(in.Place), Ops: body.ops})
	out.Problems = t2FigureProblems(report.Problems, cat.Display(cat.TeamYAML(in.Name, catalog.Layer(in.Place))))
	for _, p := range out.Problems {
		if p.Sev == "err" {
			return out, nil
		}
	}
	overlay, failure := (apiTeam{s: &Server{cat: cat}}).overlay(in.Name, in.Place, body.ops)
	if failure != nil {
		return nil, failure
	}
	def, err := overlay.Definition(in.Name, catalog.Layer(in.Place))
	if err != nil {
		return nil, refuse(422, err.Error())
	}
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		if node.Name != in.Member {
			continue
		}
		built, err := prompt.Member(def, node, prompt.Run{Team: out.Team, Base: out.Project, Executable: out.Executable, Lang: out.Language, Reader: overlay.Reader(), Sources: overlay, Display: overlay.Display})
		if err != nil {
			return nil, refuse(422, err.Error())
		}
		out.Current = &built.Text
		out.Segments = built.Segments
		if out.Recorded != nil {
			out.Diff = PromptDiff(out.Recorded.Text, built.Text)
		}
		return out, nil
	}
	return nil, refuse(422, "この定義にメンバーがありません")
}

func (s *Server) promptCatalog(in promptComparisonIn) (*catalog.Catalog, *apiErr) {
	path := s.cat.TeamYAML(in.Name, catalog.Layer(in.Place))
	if in.New || in.Base != nil {
		row, refusal := s.t2DraftRowOf(in.apiIn)
		if refusal != nil {
			return nil, refusal
		}
		return s.cat.With(path, row.Team).With(s.t2AgreeFile(in.Name, catalog.Layer(in.Place)), row.Agree), nil
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil, refuse(422, err.Error())
	}
	if in.Hash != source.Hash(text) {
		return nil, refuse(409, "定義が変わりました。ファイルからリロードしてください")
	}
	return s.cat.With(path, text), nil
}
