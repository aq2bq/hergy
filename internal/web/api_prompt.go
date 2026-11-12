package web

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// PromptReader reads recorded bodies and the live teams of one definition. It
// never sends or records a prompt.
type PromptReader interface {
	// PromptRuns is every team of one definition that runs, in herdr's order.
	PromptRuns(project domainplace.Project, definition, place string) ([]PromptRun, error)
	// PromptHistory is the recorded body of one member of the team named by
	// running. The team is chosen by promptsFor; this reads the record and says
	// whether the member is live.
	PromptHistory(project domainplace.Project, definition, place, member, running string) (PromptHistory, error)
}

// PromptRun identifies a live instance and its execution project.
type PromptRun struct {
	Team    domainname.TeamRef  `json:"team"`
	Project domainplace.Project `json:"project"`
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
	Member string `json:"member"`
}
type promptComparison struct {
	PromptHistory
	Segments   []definition.PromptSegment `json:"segments"`
	Current    *string                    `json:"current"`
	Problems   []apiProblem               `json:"problems"`
	Diff       []PromptDiffLine           `json:"diff"`
	Project    domainplace.Project        `json:"project"`
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
	scoped, refusal := s.withProjectInput(in.Project)
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

// promptAsk is one question to the assembly owner: the definition and the draft
// (with the person's pick of a running team), and, for a caller that already
// read them, the catalog and definition.
type promptAsk struct {
	In promptComparisonIn
	// WithRuns reads the live teams and decides the name. The check of a draft
	// that is not saved yet has none.
	WithRuns bool
	// Cat and Def are what the caller already built (the check's overlay): the
	// owner assembles with them instead of reading the definition again.
	Cat *catalog.Catalog
	Def *definition.Definition
	// Worker asks for one worker's preview (a hire's text) instead of members.
	Worker *apiWorkerIn
}

// promptNames is which name the text carries, and why (⑤ r-ba9b, d-002b): the
// team the person picked, the only running team, the definition's own name when
// none runs, when more than one runs un-picked, or when the live reading fails.
type promptNames struct {
	Team     string
	Naming   domainname.Naming
	Kind     string // "run" | "definition"
	Status   string // "picked" | "single" | "none" | "choose" | "error"
	Runs     []PromptRun
	Selected *PromptRun
	Reason   string // Status == "error": why the live reading failed
}

// promptText is one member's built text, before it is put in an answer's shape.
type promptText struct {
	Name     string
	Text     string
	Segments []definition.PromptSegment
}

// promptBuilt is what the owner answers: the name and why, the built texts, and
// the catalog and definition the caller may need for its own readings (the
// comparison's problem gate).
type promptBuilt struct {
	Names promptNames
	Built []promptText
	// Worker is the one worker's text when the ask named a worker, and WorkerErr
	// is why it could not be built (no member hires for that purpose, and so on).
	Worker    *promptText
	WorkerErr error
	Problems  []apiProblem
	Project   domainplace.Project
	Cat       *catalog.Catalog
	Def       *definition.Definition
	DefErr    error
	Errors    map[string]error
}

// promptsFor is the one place a member's or worker's text is built (⑤ r-ba9b):
// the name is chosen here, and prompt.Member / prompt.Worker are called here and
// nowhere else in internal/web. The caller hands in the draft, or the catalog it
// already built, and draws what comes back.
func (s *Server) promptsFor(ask promptAsk) (*promptBuilt, *apiErr) {
	in := ask.In
	target, bad := s.resourceSeen(domainplace.Definition, in.Place.String(), in.Name)
	if bad != nil {
		return nil, bad
	}
	body, bad := t2DecodeOps(in.apiIn)
	if bad != nil {
		return nil, bad
	}
	query := ops.ResourcePromptInput{Target: ops.ExistingResource(target), Hash: in.Hash, Ops: body.ops, Pick: in.RunningTeam, Member: in.Member, WithRuns: ask.WithRuns, Catalog: ask.Cat, Definition: ask.Def, Executable: promptExecutable(), Language: i18n.Language()}
	if in.New || in.Base != nil {
		row, bad := s.t2DraftRowOf(in.apiIn)
		if bad != nil {
			return nil, bad
		}
		query.Draft = &ops.DefinitionDraft{Team: row.Team, Agreement: row.Agree, AgreementExists: row.AgreementExists}
	}
	if ask.Worker != nil {
		w := ask.Worker
		query.Worker = &ops.ResourceWorkerPrompt{Name: w.Name, Purpose: w.Purpose, Task: w.Task, From: w.From, Preview: w.Preview}
	}
	query.Runs = func(project domainplace.Project, definition, place string) ([]ops.PromptRun, error) {
		runs, err := s.promptRuns(project, definition, place)
		out := make([]ops.PromptRun, 0, len(runs))
		for _, run := range runs {
			out = append(out, ops.PromptRun{Team: run.Team, Project: run.Project})
		}
		return out, err
	}
	query.AtProject = func(project domainplace.Project) (*catalog.Catalog, error) {
		cat, bad := s.catalogAt(project)
		if bad != nil {
			return nil, errors.New(bad.Text)
		}
		return cat, nil
	}
	out, failure := s.resources().PreviewResourcePrompts(query)
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	built := &promptBuilt{Names: promptNamesOf(out.Names), Project: out.Project, Cat: out.Cat, Def: out.Def, DefErr: out.DefErr, WorkerErr: out.WorkerErr, Errors: out.Errors}
	for _, text := range out.Built {
		built.Built = append(built.Built, promptText{Name: text.Name, Text: text.Text, Segments: text.Segments})
	}
	if out.Worker != nil {
		w := out.Worker
		built.Worker = &promptText{Name: w.Name, Text: w.Text, Segments: w.Segments}
	}
	return built, nil
}
func promptNamesOf(in ops.ResourcePromptNames) promptNames {
	out := promptNames{Team: in.Team, Naming: in.Naming, Kind: in.Kind, Status: in.Status, Reason: in.Reason, Runs: []PromptRun{}}
	for _, run := range in.Runs {
		out.Runs = append(out.Runs, PromptRun{Team: run.Team, Project: run.Project})
	}
	if in.Selected != nil {
		out.Selected = &PromptRun{Team: in.Selected.Team, Project: in.Selected.Project}
	}
	return out
}
func (s *Server) promptRuns(project domainplace.Project, definition, place string) ([]PromptRun, error) {
	if reader, ok := s.act.(PromptReader); ok {
		return reader.PromptRuns(project, definition, place)
	}
	return []PromptRun{}, nil
}

// legacyHistory is the answer's history shape from the name facts and the
// record: the answer keeps its fields while the team's choice moved to the owner.
func legacyHistory(names promptNames, readerOK, asked bool, history PromptHistory, readErr error) PromptHistory {
	out := PromptHistory{Runs: names.Runs, Selected: names.Selected}
	if !readerOK {
		out.Status, out.Reason = "missing", "このメンバーへ渡した本文の記録がありません"
		return out
	}
	switch names.Status {
	case "error":
		out.Status, out.Reason = "error", names.Reason
	case "choose":
		out.Status, out.Reason = "choose", "実行中のチームを選んでください"
	case "none":
		out.Status, out.Reason = "stopped", "動いているチームがないため、今の文だけを表示します"
	}
	if asked {
		if readErr != nil {
			out.Status, out.Reason = "error", readErr.Error()
		} else {
			out.Status, out.Reason, out.Recorded = history.Status, history.Reason, history.Recorded
		}
	}
	if out.Status == "" {
		out.Status, out.Reason = "missing", "このメンバーへ渡した本文の記録がありません"
	}
	return out
}

func (s *Server) comparePrompt(in promptComparisonIn) (*promptComparison, *apiErr) {
	body, refusal := t2DecodeOps(in.apiIn)
	if refusal != nil {
		return nil, refusal
	}
	out := &promptComparison{
		Segments: []definition.PromptSegment{},
		Problems: []apiProblem{}, Diff: []PromptDiffLine{}, Team: in.Name, Member: in.Member,
		Draft:      len(body.ops) > 0 || in.New || in.Base != nil,
		Executable: promptExecutable(), Language: i18n.Language(), Project: s.cat.Base(),
	}
	withRuns := !in.New && in.Base == nil
	built, refusal := s.promptsFor(promptAsk{In: in, WithRuns: withRuns})
	if refusal != nil {
		return nil, refusal
	}
	out.Project = built.Project
	out.Team = built.Names.Team
	readerOK, asked := false, false
	var history PromptHistory
	var readErr error
	if withRuns {
		if reader, ok := s.act.(PromptReader); ok {
			readerOK = true
			if built.Names.Selected != nil {
				asked = true
				history, readErr = reader.PromptHistory(built.Project, in.Name, in.Place.String(), in.Member, built.Names.Selected.Team.String())
			}
		}
	}
	out.PromptHistory = legacyHistory(built.Names, readerOK, asked, history, readErr)
	if built.Cat != nil {
		report := check.Team(check.TeamInput{Catalog: built.Cat, Name: in.Name, Layer: in.Place.Layer(), Ops: body.ops})
		out.Problems = t2FigureProblems(report.Problems, built.Cat.Display(built.Cat.TeamYAML(in.Name, in.Place.Layer())))
		for _, p := range out.Problems {
			if p.Sev == "err" {
				return out, nil
			}
		}
	}
	if built.DefErr != nil {
		return nil, refuse(422, built.DefErr.Error())
	}
	if len(built.Built) == 0 {
		if err := built.Errors[in.Member]; err != nil {
			return nil, refuse(422, err.Error())
		}
		return nil, refuse(422, "この定義にメンバーがありません")
	}
	text := built.Built[0]
	out.Current = &text.Text
	out.Segments = text.Segments
	if out.Recorded != nil {
		out.Diff = PromptDiff(out.Recorded.Text, text.Text)
	}
	return out, nil
}
