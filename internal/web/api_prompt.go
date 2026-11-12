package web

import (
	"net/http"
	"path/filepath"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// PromptReader reads recorded bodies and the live teams of one definition. It
// never sends or records a prompt.
type PromptReader interface {
	// PromptRuns is every team of one definition that runs, in herdr's order.
	PromptRuns(project, definition, place string) ([]PromptRun, error)
	// PromptHistory is the recorded body of one member of the team named by
	// running. The team is chosen by promptsFor; this reads the record and says
	// whether the member is live.
	PromptHistory(project, definition, place, member, running string) (PromptHistory, error)
}

// PromptRun identifies a live instance and its execution project.
type PromptRun struct {
	Team    domainname.TeamRef `json:"team"`
	Project string             `json:"project"`
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
	Project   string
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
	out := &promptBuilt{
		Names:  promptNames{Team: in.Name, Kind: "definition", Status: "none", Runs: []PromptRun{}},
		Errors: map[string]error{},
	}
	project := s.cat.Base()
	if ask.Cat != nil {
		project = ask.Cat.Base()
	}
	out.Project = project
	if ask.WithRuns {
		runs, err := s.promptRuns(project, in.Name, in.Place)
		if err != nil {
			out.Names.Status, out.Names.Reason = "error", err.Error()
		} else {
			out.Names = selectPromptName(in.Name, runs, in.RunningTeam)
		}
	}
	// Select the naming branch once, before checks or text construction.
	if out.Names.Kind == "run" {
		out.Names.Naming = domainname.RunningNames(out.Names.Selected.Team)
	} else {
		d, _ := domainname.ParseDefinitionName(out.Names.Team)
		out.Names.Naming = domainname.DefinitionNames(d)
	}
	basis, refusal := s.promptBasisFor(ask, project, out.Names)
	if refusal != nil {
		return nil, refusal
	}
	out.Project, out.Cat, out.Def, out.DefErr = basis.project, basis.cat, basis.def, basis.defErr
	if basis.def == nil || basis.def.Graph == nil {
		return out, nil
	}
	if ask.Worker != nil {
		out.Worker, out.WorkerErr = promptWorkerText(out.Names.Naming, basis.def, ask.Worker)
		return out, nil
	}
	out.Built = promptMemberTexts(out.Names.Naming, out.Project, basis.overlay, basis.def, in.Member, out.Errors)
	return out, nil
}

// promptBasis is what a prompt's text stands on: the catalog the caller's
// problems are checked against, the overlay the text reads through, and the
// definition itself.
type promptBasis struct {
	project      string
	cat, overlay *catalog.Catalog
	def          *definition.Definition
	defErr       error
}

// promptBasisFor is the catalog and definition the name's prompt is built from.
// The caller's own catalog is used when it has one (the check); otherwise the
// comparison's catalog is read, in the project the chosen run stands in.
func (s *Server) promptBasisFor(ask promptAsk, project string, names promptNames) (*promptBasis, *apiErr) {
	if ask.Cat != nil {
		basis := &promptBasis{project: project, cat: ask.Cat, overlay: ask.Cat, def: ask.Def}
		basis.readDefinition(ask.In.Name, ask.In.Place)
		return basis, nil
	}
	if names.Selected != nil && !sameProjectPath(names.Selected.Project, project) {
		scoped, refusal := s.withProject(names.Selected.Project)
		if refusal != nil {
			return nil, refusal
		}
		s = scoped
		project = s.cat.Base()
	}
	cat, refusal := s.promptCatalog(ask.In)
	if refusal != nil {
		return nil, refusal
	}
	body, refusal := t2DecodeOps(ask.In.apiIn)
	if refusal != nil {
		return nil, refusal
	}
	overlay, failure := (apiTeam{s: &Server{cat: cat}}).overlay(ask.In.Name, ask.In.Place, body.ops)
	if failure != nil {
		return nil, failure
	}
	basis := &promptBasis{project: project, cat: cat, overlay: overlay}
	basis.readDefinition(ask.In.Name, ask.In.Place)
	return basis, nil
}

// readDefinition reads the definition once, when the caller did not hand one in.
func (b *promptBasis) readDefinition(name, place string) {
	if b.def != nil || b.overlay == nil {
		return
	}
	b.def, b.defErr = b.overlay.Definition(name, catalog.Layer(place))
}

// promptWorkerText builds the one worker's preview with the team name the
// prompt is named by.
func promptWorkerText(team domainname.Naming, def *definition.Definition, worker *apiWorkerIn) (*promptText, error) {
	lead, err := t2HiringMember(def, worker.Purpose, worker.From)
	if err != nil {
		return nil, err
	}
	built, err := prompt.Worker(prompt.WorkerInput{
		Team: team.String(), Lead: team.Member(lead.Name), LeadRole: lead.Role,
		Purpose: worker.Purpose, Name: team.Worker(worker.Name), Task: worker.Task,
	})
	if err != nil {
		return nil, err
	}
	return &promptText{Name: team.Worker(worker.Name), Text: built.Text, Segments: built.Segments}, nil
}

// promptMemberTexts builds every member's text, or the one the caller named. A
// member whose text cannot be built is left out; the caller's problems say why.
func promptMemberTexts(team domainname.Naming, project string, overlay *catalog.Catalog, def *definition.Definition, member string, failures map[string]error) []promptText {
	run := prompt.Run{
		Team: team, Base: project, Executable: promptExecutable(),
		Lang: i18n.Language(), Reader: overlay.Reader(), Sources: overlay, Display: overlay.Display,
	}
	var out []promptText
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		if member != "" && node.Name.String() != member {
			continue
		}
		built, err := prompt.Member(def, node, run)
		if err != nil {
			failures[node.Name.String()] = err
			continue
		}
		out = append(out, promptText{Name: node.Name.String(), Text: built.Text, Segments: built.Segments})
	}
	return out
}

// promptRuns reads the live teams through the reader, or answers none when the
// server has no live reading (a command, or a test without one).
func (s *Server) promptRuns(project, definition, place string) ([]PromptRun, error) {
	if reader, ok := s.act.(PromptReader); ok {
		return reader.PromptRuns(project, definition, place)
	}
	return []PromptRun{}, nil
}

// sameProjectPath is whether two spellings name one project: the run's project
// (as herdr and the launch record name it) and this catalog's own.
func sameProjectPath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// selectPromptName is the one name rule: the picked team when it runs, the only
// running team when none was picked, the definition's name otherwise.
func selectPromptName(definition string, runs []PromptRun, picked string) promptNames {
	out := promptNames{Runs: runs, Team: definition, Kind: "definition", Status: "none"}
	if picked != "" {
		for i := range runs {
			if runs[i].Team.String() == picked {
				selected := runs[i]
				out.Team, out.Kind, out.Status, out.Selected = picked, "run", "picked", &selected
				return out
			}
		}
		// A pick that no longer runs is not taken: the person still has to
		// choose among the teams that do (or none does).
		if len(runs) > 0 {
			out.Status = "choose"
		}
		return out
	}
	if len(runs) == 1 {
		selected := runs[0]
		out.Team, out.Kind, out.Status, out.Selected = runs[0].Team.String(), "run", "single", &selected
		return out
	}
	if len(runs) > 1 {
		out.Status = "choose"
	}
	return out
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
				history, readErr = reader.PromptHistory(built.Project, in.Name, in.Place, in.Member, built.Names.Selected.Team.String())
			}
		}
	}
	out.PromptHistory = legacyHistory(built.Names, readerOK, asked, history, readErr)
	if built.Cat != nil {
		report := check.Team(check.TeamInput{Catalog: built.Cat, Name: in.Name, Layer: catalog.Layer(in.Place), Ops: body.ops})
		out.Problems = t2FigureProblems(report.Problems, built.Cat.Display(built.Cat.TeamYAML(in.Name, catalog.Layer(in.Place))))
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
