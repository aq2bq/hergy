package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"go.yaml.in/yaml/v3"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the team kind (spec-boundary 4, 5, 8): the team definitions the
// team screen reads and writes, together with the agreement that belongs to each
// one. Its checks are all of check.Team and check.Agreement and its writes are
// all definition.EditYAML and rules.Rewrite, so the kind makes no sentence of its
// own (spec-teams X1) and every line an edit does not touch comes back byte for
// byte.
//
// One team definition holds two documents, so apiIn.Part says which one a
// payload is about: "team" is the figure (team.yaml) and "agreement" is the
// team's own agreement.md.

// apiTeam is the team kind.
type apiTeam struct{ s *Server }

// apiTeamIn is the team kind's own draft, carried in apiIn.Doc. Ops are the
// figure's edits, Op is the one more op a card is trying (so the answer carries
// its effect), and Doc is the agreement's own document. Members and Workers are
// the rows a 割り当て tab that still sends rows writes (r-8e85).
type apiTeamIn struct {
	Ops     []t2Op            `json:"ops"`
	Op      *t2Op             `json:"op,omitempty"`
	Doc     *r2Doc            `json:"doc,omitempty"`
	Members []apiTeamDraftRow `json:"members"`
	Workers []apiTeamDraftRow `json:"workers"`
}

// apiTeamSummary is the team list's columns: the description, who is on the team,
// and how much the team's own agreement holds. The per-member amount is gone
// (d-b87a); the definition's own size and budget are below.
type apiTeamSummary struct {
	Description string          `json:"description"`
	Members     []apiTeamMember `json:"members"`
	Workers     int             `json:"workers"`
	Broken      bool            `json:"broken,omitempty"`
	// Size is the definition's own size, counted the way d-00a9 says: the part
	// of its rules that goes into the prompt and the words of its templates.
	// Limit is the provisional budget it is shown against. The figure's check
	// answers the same pair, so the list's column and the panel count one number
	// (r-2f83).
	Size  int `json:"size"`
	Limit int `json:"limit"`
}

// apiTeamMember is one member of a team definition: the name a person writes,
// the role they play and the profile they run with.
type apiTeamMember struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Profile string `json:"profile"`
}

// apiTeamWorker is one worker a member may hire.
type apiTeamWorker struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// apiTeamDoc is one team definition as the screen reads it: the description, the
// members, the workers, the turns of the draft, the prompt's own lines, and the
// team's own agreement.
type apiTeamDoc struct {
	Description string          `json:"description"`
	Members     []apiTeamMember `json:"members"`
	Workers     []apiTeamWorker `json:"workers"`
	Turns       []t2Turn        `json:"turns"`
	Agreement   *apiTeamAgree   `json:"agreement,omitempty"`
	Broken      bool            `json:"broken,omitempty"`
}

// apiTeamDraft is the doc a 割り当て tab that still sends rows writes: the picks
// it changed, and nothing else. A member's role or profile is left out when that
// pick did not move, so a card that only changed roles does not blank the
// profiles (spec-boundary 6, r-8e85). The team screen sends its picks as the
// kind's own ops instead (t2Op); both are read.
type apiTeamDraft struct {
	Name    string            `json:"name"`
	Members []apiTeamDraftRow `json:"members"`
	Workers []apiTeamDraftRow `json:"workers"`
}

type apiTeamDraftRow struct {
	Name    string  `json:"name"`
	From    string  `json:"from"`
	Purpose string  `json:"purpose"`
	Role    *string `json:"role"`
	Profile *string `json:"profile"`
}

// apiTeamAgree is a team's own agreement as the screen reads it: where it
// stands, its hash, whether the file is there at all, how many rules it holds,
// and the document itself. A team with no agreement.md answers with Has false
// and no document, so the screen draws the empty agreement a save would write.
type apiTeamAgree struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Has  bool   `json:"has"`
	Doc  *r2Doc `json:"doc,omitempty"`
}

// apiTeamGet is one team definition in one place, the answer of GET
// /api/team/{name}.
type apiTeamGet struct {
	Doc       apiTeamDoc `json:"doc"`
	Text      string     `json:"text"`
	Hash      string     `json:"hash"`
	Path      string     `json:"path"`
	Effective string     `json:"effective"`
	Hidden    []string   `json:"hidden"`
}

// apiTeamEffect is what one more op would do: the prompt's own lines it moves,
// and the problems it would add (spec-teams 1.4.4, X3).
type apiTeamEffect struct {
	Lines    []check.EffectLine `json:"lines"`
	Problems []apiProblem       `json:"problems"`
}

// List is GET /api/team: one item per definition, the hashes of the files
// behind them, and the files themselves for the YAML tab.
func (k apiTeam) List() (any, *apiErr) {
	return k.s.t2List(), nil
}

// listLayers is List over the layers given: a headquarters group is one layer's
// rows, decided by the same owner the band asks (r-ea17).
func (k apiTeam) listLayers(layers []string) apiList {
	return k.s.t2ListLayers(layers)
}

// t2List is the team list answer: one row per place, taken from the owner
// (ShownRows) so the rows are the set and order the band counts (r-ea17), plus
// the YAML tab's files.
func (s *Server) t2List() apiList {
	return s.t2ListLayers([]string{source.LayerLocal, source.LayerGlobal})
}

// t2ListLayers is the list over the layers given.
func (s *Server) t2ListLayers(layers []string) apiList {
	out := apiList{Hashes: map[string]string{}}
	ver := s.cat.Version()
	seenFile := map[string]bool{}
	for _, layer := range []string{source.LayerLocal, source.LayerGlobal} {
		place := domainplace.Selection(layer).API()
		if !slices.Contains(layers, layer) {
			continue
		}
		for _, listed := range ops.ListResourceRows(s.cat, catalog.KindTeam, layer) {
			item := s.t2ItemAt(listed.Name, place, ver)
			item.Mark = markWord(listed)
			out.Items = append(out.Items, item)
			for _, file := range s.t2FilesAt(listed.Name, place) {
				if seenFile[file.Path] {
					continue
				}
				seenFile[file.Path] = true
				out.Files = append(out.Files, file)
			}
		}
	}
	return out
}

// t2ItemAt is one definition at one place: the place's reading, the problem
// when its team.yaml cannot be read, and the place's own summary.
func (s *Server) t2ItemAt(name, place, ver string) apiItem {
	layer := domainplace.Selection(place).Layer()
	path := s.cat.TeamYAML(name, layer)
	item := apiItem{Name: name, Places: map[string]apiPlace{}, Effective: place}
	if why, state := s.unreadable(path, ""); why != "" {
		item.Places[place] = apiPlace{
			Doc: s.t2DocOf(name, layer, nil), Path: s.cat.Display(path),
			Problems: apiCounts{Err: 1}, Problem: why, State: state,
		}
		item.Problems = item.Places[place].Problems
		item.Summary = s.t2SummaryOf(name, layer, nil, ver)
		return item
	}
	text, _ := s.cat.Reader().Read(source.Doc{Path: path})
	def, _ := s.cat.Definition(name, layer)
	item.Places[place] = apiPlace{
		Doc: s.t2DocOf(name, layer, def), Text: text, Hash: source.Hash(text),
		Path: s.cat.Display(path), Problems: s.t2CountsOf(name, layer, ver),
	}
	item.Problems = item.Places[place].Problems
	item.Summary = s.t2SummaryOf(name, layer, def, ver)
	return item
}

// t2FilesAt is the YAML tab's files of one place: team.yaml and agreement.md.
// fileMark is the row's mark for one of its files: the owner's answer, so the
// YAML tab's badge is not the file's layer (r-ea17).
func (s *Server) fileMark(name, place string) string {
	layer := domainplace.Selection(place).Layer()
	for _, row := range ops.ListResourceRows(s.cat, catalog.KindTeam, layer) {
		if row.Name == name {
			return markWord(row)
		}
	}
	return ""
}

func (s *Server) t2FilesAt(name, place string) []apiFile {
	layer := domainplace.Selection(place).Layer()
	var out []apiFile
	for _, one := range []struct{ kind, path string }{
		{"team", s.cat.TeamYAML(name, layer)},
		{"agreement", s.t2AgreeFile(name, layer)},
	} {
		if there, _, _ := s.cat.FileState(one.path); !there {
			continue
		}
		if why, _ := s.unreadable(one.path, ""); why != "" {
			out = append(out, apiFile{
				Place: place, Name: name, Kind: one.kind, Label: t2PlaceWord(place),
				Path: s.cat.Display(one.path), Problem: why,
				Problems: []apiProblem{{Sev: "err", At: "line", Line: 1, Text: why}},
			})
			continue
		}
		out = append(out, s.t2FileOf(place, one.kind, name, one.path))
	}
	return out
}

// t2DocOf is one place's team definition in the screen's shape.
func (s *Server) t2DocOf(name, layer string, def *definition.Definition) apiTeamDoc {
	return s.t2DocWithAgree(def, s.t2AgreeOf(name, layer))
}

// t2DocWithAgree is t2DocOf with the agreement given rather than read: a
// definition that is not there yet has no agreement.md to read, so its rules
// come from the text the row stands on (spec-teams B2).
func (s *Server) t2DocWithAgree(def *definition.Definition, agree *apiTeamAgree) apiTeamDoc {
	out := apiTeamDoc{Agreement: agree}
	if def == nil || def.Graph == nil {
		out.Broken = true
		return out
	}
	out.Description = def.Description
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		out.Members = append(out.Members, apiTeamMember{Name: member.Name.String(), Role: member.Role, Profile: member.Profile})
	}
	for _, w := range def.Graph.Workers {
		out.Workers = append(out.Workers, apiTeamWorker{From: w.From, Purpose: w.Purpose.String(), Profile: w.Profile})
	}
	out.Turns = t2TurnsOf(def)
	return out
}

// promptExecutable is the hergy binary a preview's hire command names. The serve
// process is the same binary the command line is, so a prompt the screen shows
// names what a real hire would.
func promptExecutable() string {
	path, err := os.Executable()
	if err != nil || path == "" {
		return "hergy"
	}
	return path
}

// t2SummaryOf is the list's columns for one definition.
func (s *Server) t2SummaryOf(name, layer string, def *definition.Definition, ver string) apiTeamSummary {
	key := "team.summary|" + s.cat.Base().String() + "|" + name + "|" + layer + "|" + ver
	return s.cached(key, func() any { return s.buildSummaryOf(name, layer, def) }).(apiTeamSummary)
}

// buildSummaryOf is one team's columns, built when no held answer stands for the
// version of its files.
func (s *Server) buildSummaryOf(name, layer string, def *definition.Definition) apiTeamSummary {
	out := apiTeamSummary{}
	if def == nil || def.Graph == nil {
		// The team screen draws 説明 — and no numbers for a file that does not
		// read (spec-teams 1.9), but the role screen's 割り当て tab still counts
		// the people the file names: the roles a member plays stand in the text
		// whether or not the definition reads (roles_v2_data.go did the same).
		out.Broken = true
		if text, err := s.cat.Reader().Read(source.Doc{Path: s.cat.TeamYAML(name, layer)}); err == nil {
			out.Members = t2MembersOfText(text)
		}
	} else {
		out.Description = def.Description
		out.Workers = len(def.Graph.Workers)
		for _, m := range def.Graph.Members {
			out.Members = append(out.Members, apiTeamMember{Name: m.Name.String(), Role: m.Role, Profile: m.Profile})
		}
	}
	// The same size and budget the figure's check answers (d-00a9, r-2f83): a
	// file that does not read has no size, but the budget it will be held to is
	// still known.
	out.Limit = check.TeamCharLimit
	if def != nil {
		out.Size = check.TeamSizeChars(s.cat, def)
	}
	return out
}

// t2MembersOfText is the people a team.yaml names, read from the text so a file
// that does not read still names them.
func t2MembersOfText(text string) []apiTeamMember {
	var raw struct {
		Members []struct {
			Name string `yaml:"name"`
			Role string `yaml:"role"`
		} `yaml:"members"`
	}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil
	}
	out := make([]apiTeamMember, 0, len(raw.Members))
	for _, m := range raw.Members {
		out = append(out, apiTeamMember{Name: m.Name, Role: m.Role})
	}
	return out
}

// t2AgreeOf is one layer's agreement.md for a definition: the document, the
// count of rules it holds (the ones kept out of the prompt included), and
// whether the file is there at all.
func (s *Server) t2AgreeOf(name, layer string) *apiTeamAgree {
	path := s.t2AgreeFile(name, layer)
	out := &apiTeamAgree{Path: s.cat.Display(path)}
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return out
	}
	out.Has, out.Hash = true, source.Hash(text)
	out.Doc = r2DocOf(text, path)
	return out
}

// t2AgreeFile is a team's own agreement.md in one layer.
func (s *Server) t2AgreeFile(name, layer string) string {
	return filepath.Join(filepath.Dir(s.cat.TeamYAML(name, layer)), "agreement.md")
}

// t2CountsOf is how many errors and warnings one place's definition carries:
// the one check's findings, counted the way the list's column does (spec-teams
// 1.2). The row, the panel and the YAML head read this same check (d-a575).
func (s *Server) t2CountsOf(name, layer, ver string) apiCounts {
	key := "team.counts|" + s.cat.Base().String() + "|" + name + "|" + layer + "|" + ver
	return s.cached(key, func() any { return s.buildCountsOf(name, layer) }).(apiCounts)
}

// buildCountsOf is one definition's counts, built when no held answer stands for
// the version of its files.
func (s *Server) buildCountsOf(name, layer string) apiCounts {
	var out apiCounts
	team := check.Team(check.TeamInput{Catalog: s.cat, Name: name, Layer: layer})
	for _, finding := range team.Problems {
		if finding.Stops {
			out.Err++
		} else {
			out.Warn++
		}
	}
	return out
}

// t2FileOf is one file of the YAML tab, with the problems its own check finds.
// A team.yaml finding carries a field rather than a line, so the line of the
// key it names travels beside it (spec-teams 1.8).
func (s *Server) t2FileOf(place, kind, name, path string) apiFile {
	text, _ := s.cat.Reader().Read(source.Doc{Path: path})
	key := "team.file|" + s.cat.Base().String() + "|" + place + "|" + kind + "|" + name + "|" + path + "|" + source.Hash(text) + "|" + s.fileMark(name, place)
	return s.cached(key, func() any { return s.buildFileOf(place, kind, name, path, text) }).(apiFile)
}

// buildFileOf is one YAML tab file with its problems, built when no held answer
// stands for the file's version.
func (s *Server) buildFileOf(place, kind, name, path, text string) apiFile {
	layer := domainplace.Selection(place).Layer()
	display := s.cat.Display(path)
	out := apiFile{
		Place: place, Name: name, Kind: kind, Label: t2PlaceWord(place),
		Path: display, Hash: source.Hash(text), Text: text, Mark: s.fileMark(name, place),
	}
	if kind == "agreement" {
		report := check.Agreement(check.AgreementInput{Catalog: s.cat, Name: name, Place: layer, Text: text})
		out.Problems = r2Problems(report.Problems, true)
		return out
	}
	for _, finding := range check.Team(check.TeamInput{Catalog: s.cat.With(path, text), Name: name, Layer: layer}).Problems {
		sev := "warn"
		if finding.Stops {
			sev = "err"
		}
		out.Problems = append(out.Problems, apiProblem{
			Sev: sev, At: "line", Field: finding.Field, Line: t2YamlLine(text, finding.Field), Text: withoutOwnHead(finding.Text, display),
			Role: finding.Role,
		})
	}
	return out
}

// t2PlaceWord is a place as a person reads it.
func t2PlaceWord(place string) string {
	if place == catalog.PlaceGlobal {
		return "グローバル"
	}
	return "プロジェクト"
}

// t2YamlLine is the line of a team.yaml field, the line a finding of that field
// stands on. A field about a node or a rule belongs to another document, and a
// field the YAML does not hold has no line.
func t2YamlLine(text, field string) int {
	if text == "" || field == "" || strings.HasPrefix(field, "node:") || strings.HasPrefix(field, "rule:") {
		return 0
	}
	root := &yaml.Node{}
	if err := yaml.Unmarshal([]byte(text), root); err != nil || len(root.Content) == 0 {
		return 0
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return 0
	}
	key, rest := field, ""
	if i := strings.IndexAny(field, ".["); i >= 0 {
		key, rest = field[:i], field[i:]
	}
	node := t2MappingValue(doc, key)
	if node == nil {
		return 0
	}
	if rest == "" || rest[0] != '[' {
		return node.Line
	}
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return node.Line
	}
	index, err := strconv.Atoi(rest[1:end])
	if err != nil || index < 0 || index >= len(node.Content) {
		return node.Line
	}
	item := node.Content[index]
	after := strings.TrimPrefix(rest[end+1:], ".")
	if after == "" {
		return item.Line
	}
	if value := t2MappingValue(item, after); value != nil {
		return value.Line
	}
	return item.Line
}

// t2MappingValue is the value a mapping node keeps under one key.
func t2MappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// Get is GET /api/team/{name}?place=: one definition as one place reads it.
func (k apiTeam) Get(name, place string) (any, *apiErr) {
	s := k.s
	if place != catalog.PlaceProject && place != catalog.PlaceGlobal {
		return nil, refuse(http.StatusBadRequest, "unknown place")
	}
	layer := domainplace.Selection(place).Layer()
	target, bad := s.resourceSeen(domainplace.Definition, place, name)
	if bad != nil {
		return nil, bad
	}
	file, failure := s.resources().ReadResource(ops.ExistingResource(target), ops.ResourceMain)
	if failure != nil || !file.Exists {
		return nil, refuse(404, t("web.error.not_found"))
	}
	path, text := file.Path, file.Text
	def, _ := s.cat.Definition(name, layer)
	effective, hidden := s.effectiveOf(catalog.KindTeam, name)
	return apiTeamGet{
		Doc: s.t2DocOf(name, layer, def), Text: text, Hash: source.Hash(text),
		Path: s.cat.Display(path), Effective: effective, Hidden: hidden,
	}, nil
}

// Check is POST /api/team/check: what is wrong with a draft, an effect, or a
// text. The payload's own shape says which: a Doc is a document's draft, no Doc
// is a whole text the YAML tab is typing.
func (k apiTeam) Check(in apiIn) (any, *apiErr) {
	if in.RenameTo != nil {
		preview, refusal := k.s.renamePreview(in.Name, *in.RenameTo, in.Place.String())
		return apiCheck{Problems: []apiProblem{}, Rename: preview}, refusal
	}
	if in.Part == "worker" {
		return k.checkWorkerPrompt(in)
	}
	if in.New || in.Base != nil {
		if in.Part == "agreement" {
			return k.checkDraftAgreement(in)
		}
		return k.checkDraft(in)
	}
	if len(in.Doc) == 0 {
		return k.checkYAML(in)
	}
	if in.Part == "agreement" {
		return k.checkAgreement(in)
	}
	return k.checkFigure(in)
}

// apiWorkerIn is the payload of a worker prompt preview: the worker's short
// name, the purpose it is hired for, the task, and the member that hires it
// (empty means the one member whose workers: name the purpose).
type apiWorkerIn struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Task    string `json:"task"`
	From    string `json:"from"`
	// Preview is the screen's reading before a hire: the name and task are the
	// screen's placeholders, replaced by <ワーカー名> and <依頼> when drawn, so
	// the herdr name's length is not held against them (r-933c, d-26ce).
	Preview bool `json:"preview"`
}

// checkWorkerPrompt answers one worker's prompt as segments, before the hire
// (r-4cf9): the same text hire --dry builds, with the draft's unsaved edits and
// the running team's name (⑤ r-ba9b, d-002b).
func (k apiTeam) checkWorkerPrompt(in apiIn) (any, *apiErr) {
	if in.Name == "" || (in.Place != catalog.PlaceProject && in.Place != catalog.PlaceGlobal) {
		return nil, refuse(http.StatusUnprocessableEntity, i18n.T("web.error.not_found"))
	}
	var body apiWorkerIn
	if err := json.Unmarshal(in.Doc, &body); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if body.Name == "" || body.Purpose == "" || body.Task == "" {
		return nil, refuse(http.StatusUnprocessableEntity, i18n.T("error.worker_prompt_required"))
	}
	built, refusal := k.s.promptsFor(promptAsk{In: promptComparisonIn{apiIn: in}, WithRuns: true, Worker: &body})
	if refusal != nil {
		return nil, refusal
	}
	if built.DefErr != nil {
		return nil, refuse(http.StatusUnprocessableEntity, built.DefErr.Error())
	}
	// Preview placeholders remain display values. Only a real hire's prompt
	// confirmation constructs the same checked worker target as spawn.Plan.
	if !body.Preview {
		workerName, err := domainname.ParseWorkerName(body.Name)
		if err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, i18n.T("error.worker_name_pattern"))
		}
		if _, err := built.Names.Naming.WorkerTarget(workerName); err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, i18n.Tf("error.herdr_name_pattern", built.Names.Naming.Worker(body.Name)))
		}
	}
	if built.WorkerErr != nil {
		return nil, refuse(http.StatusUnprocessableEntity, built.WorkerErr.Error())
	}
	if built.Worker == nil {
		return nil, refuse(http.StatusUnprocessableEntity, i18n.T("error.worker_prompt_required"))
	}
	return apiCheck{Prompt: built.Worker.Segments, PromptTeam: built.Names.Team}, nil
}

// checkFigure answers what is wrong with the figure a panel holds: the saved
// text with the panel's ops applied, checked as if it were saved (spec-teams
// C2). When the payload carries the one op a card is trying, the answer carries
// that op's effect too. The answer carries the topology tab's YAML preview as
// well: the text a save would write and the lines one edit changed (spec-teams
// B7).
func (k apiTeam) checkFigure(in apiIn) (any, *apiErr) {
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	query := ops.ResourceInspection{Kind: domainplace.Definition, Place: in.Place.At(k.s.cat.Base()), Name: in.Name, Ops: body.ops}
	if body.in.Op != nil {
		op, err := body.in.Op.op()
		if err != nil {
			return nil, refuse(422, err.Error())
		}
		query.Effect = &op
	}
	checked, failure := k.s.resources().CheckResource(query)
	if failure != nil {
		return nil, k.s.resourceFailure(failure, nil, false)
	}
	own := k.s.cat.Display(checked.Path)
	out := apiCheck{Problems: t2FigureProblems(checked.Team.Problems, own), Limit: checked.Limit, Size: checked.Size}
	out.Preview = apiTeamPreview{Lines: definition.YAMLLines(checked.Edited), Diff: check.LineDiff(t2LineTexts(definition.YAMLLines(checked.Before)), t2LineTexts(definition.YAMLLines(checked.Edited)))}
	if checked.Effect != nil {
		out.Effect = apiTeamEffect{Lines: checked.Effect.Lines, Problems: t2FigureProblems(checked.Effect.Problems, own)}
	}
	return out, nil
}

// checkAgreement answers what is wrong with a team's agreement, checked with
// the figure's draft laid over the team.yaml (spec-teams C6, 1.5).
func (k apiTeam) checkAgreement(in apiIn) (any, *apiErr) {
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	if body.in.Doc == nil {
		return nil, refuse(422, "agreement: one document is needed")
	}
	checked, failure := k.s.resources().CheckResource(ops.ResourceInspection{Kind: domainplace.Definition, Place: in.Place.At(k.s.cat.Base()), Name: in.Name, Ops: body.ops, Part: ops.ResourceAgreement, Text: body.in.Doc.Text})
	if failure != nil {
		return nil, k.s.resourceFailure(failure, nil, false)
	}
	return r2CheckAnswer(checked.Document, false), nil
}

// t2DecodeOps reads a figure's or an agreement's draft: the ops it holds, and
// the one op a card is trying, in the model's words.
type t2Body struct {
	in  apiTeamIn
	ops []definition.Op
}

func t2DecodeOps(in apiIn) (t2Body, *apiErr) {
	var body apiTeamIn
	if err := json.Unmarshal(in.Doc, &body); err != nil {
		return t2Body{}, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	ops, err := t2Ops(body.Ops)
	if err != nil {
		return t2Body{}, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if len(ops) == 0 {
		// A 割り当て tab that still sends rows: the picks it names are the ops
		// (r-8e85).
		ops = apiTeamRowOps(apiTeamDraft{Members: body.Members, Workers: body.Workers})
	}
	return t2Body{in: body, ops: ops}, nil
}

// t2FigureProblems is the figure's findings in the boundary's shape: the field
// each one names is where a screen shows it (spec-boundary 5). The file the
// definition's own findings stand in — its team.yaml — is named by the panel's
// head, so the sentences leave that name to the head; the command line keeps it,
// because its detail line stands on its own (spec/32 D2).
func t2FigureProblems(findings []check.Finding, own string) []apiProblem {
	out := make([]apiProblem, 0, len(findings))
	for _, finding := range findings {
		sev := "warn"
		if finding.Stops {
			sev = "err"
		}
		out = append(out, apiProblem{
			Sev: sev, At: finding.Field, Field: figureFieldName(finding.Field), Text: withoutOwnHead(finding.Text, own),
			Role: finding.Role,
		})
	}
	return out
}

// figureFieldName is the name of the field a figure finding stands beside, from
// the place the finding names: the field is the place, and the name is the key
// at its end (members[2].role is beside role; workers[0] beside workers). A rule
// finding names the rule itself as its field, and a node finding the node.
func figureFieldName(at string) string {
	switch {
	case at == "", strings.HasPrefix(at, "rule:"), strings.HasPrefix(at, "node:"):
		return at
	case strings.HasSuffix(at, "]"):
		if before, _, ok := strings.Cut(at, "["); ok {
			return before
		}
		return at
	}
	if dot := strings.LastIndex(at, "."); dot >= 0 {
		return at[dot+1:]
	}
	return at
}

// withoutOwnHead is one problem's sentence as a screen draws it: the name of the
// file the problem stands in comes off, because the place the screen draws the
// problem — a panel's head, the YAML tab's file — gives that name already. What
// the sentence keeps is the names of the other files it is about, which no line
// above gives.
func withoutOwnHead(text, file string) string {
	return strings.TrimPrefix(text, file+": ")
}

// checkYAML answers what is wrong with the YAML a person is typing: the same
// check, read from the text (spec-teams 1.8).
func (k apiTeam) checkYAML(in apiIn) (any, *apiErr) {
	place, kind, ok := k.fileOf(in.Name, in)
	if !ok {
		return nil, refuse(404, t("web.error.not_found"))
	}
	part := ops.ResourceMain
	if kind == "agreement" {
		part = ops.ResourceAgreement
	}
	checked, failure := k.s.resources().CheckResource(ops.ResourceInspection{Kind: domainplace.Definition, Place: domainplace.Selection(place).At(k.s.cat.Base()), Name: in.Name, Text: in.Text, WholeText: true, Part: part})
	if failure != nil {
		return nil, k.s.resourceFailure(failure, nil, false)
	}
	out := apiCheck{}
	if kind == "agreement" {
		out.Problems = r2Problems(checked.Document.Problems, true)
		return out, nil
	}
	display := k.s.cat.Display(checked.Path)
	for _, finding := range checked.Team.Problems {
		sev := "warn"
		if finding.Stops {
			sev = "err"
		}
		out.Problems = append(out.Problems, apiProblem{Sev: sev, At: "line", Field: finding.Field, Line: t2YamlLine(in.Text, finding.Field), Text: withoutOwnHead(finding.Text, display), Role: finding.Role})
	}
	return out, nil
}

// fileOf is which file a YAML tab payload is about: the definition's name (the
// path's for a write, the payload's for a check), the place from the payload,
// and the kind from the word the part carries.
func (k apiTeam) fileOf(name string, in apiIn) (place, kind string, ok bool) {
	kind = "team"
	if in.Part == "agreement" {
		kind = "agreement"
	}
	if name == "" || (in.Place != catalog.PlaceProject && in.Place != catalog.PlaceGlobal) {
		return "", "", false
	}
	return in.Place.String(), kind, true
}

// Save is PUT /api/team/{name}: the figure's ops, the agreement's document, or
// a file's whole text.
func (k apiTeam) Save(name string, in apiIn) (any, *apiErr) {
	if in.New || in.Base != nil || in.Part == "folder" {
		return k.saveFolder(name, in)
	}
	if len(in.Doc) == 0 {
		return k.saveYAML(name, in)
	}
	if in.Part == "agreement" {
		return k.saveAgreement(name, in)
	}
	return k.saveFigure(name, in)
}

// saveFigure writes a panel's ops into its team.yaml, in one pass: every line
// the ops do not touch comes back byte for byte. It is refused only when a
// problem the saved file did not hold has appeared (spec-teams 1.3, X1).
func (k apiTeam) saveFigure(name string, in apiIn) (any, *apiErr) {
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	ref, bad := k.s.resourceRef(domainplace.Definition, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	_, failure := k.s.resources().SaveResource(ref, ops.ResourceEdit{Ops: body.ops, Hash: in.Hash})
	if failure != nil {
		return nil, k.s.resourceFailure(failure, k.s.teamDoc(name, in.Place.Layer()), false)
	}
	return k.savedAnswer(name, in.Place.String()), nil
}

// saveAgreement writes a team's own agreement.md from the panel's document.
// An agreement the team does not have yet is written whole (spec-teams 1.5).
func (k apiTeam) saveAgreement(name string, in apiIn) (any, *apiErr) {
	s := k.s
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	if body.in.Doc == nil {
		return nil, refuse(422, "agreement: one document is needed")
	}
	ref, bad := s.resourceRef(domainplace.Definition, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	saved, failure := s.resources().SaveResource(ref, ops.ResourceEdit{Part: ops.ResourceAgreement, Text: body.in.Doc.Text, Hash: in.Hash})
	if failure != nil {
		return nil, s.resourceFailure(failure, ruleDoc(s.cat.Display(failure.Path)), false)
	}
	out := k.savedAnswer(name, in.Place.String())
	out.Doc = r2DocOf(saved.Text, saved.Path)
	out.Text = saved.Text
	out.Hash = saved.Hash
	out.Path = s.cat.Display(saved.Path)
	return out, nil
}

// saveYAML writes one file's text as the person wrote it. It is refused only
// when a problem the saved file did not hold has appeared (spec-teams X3).
func (k apiTeam) saveYAML(name string, in apiIn) (any, *apiErr) {
	s := k.s
	place, kind, ok := k.fileOf(name, in)
	if !ok {
		return nil, refuse(404, t("web.error.not_found"))
	}
	ref, bad := s.resourceRef(domainplace.Definition, place, name)
	if bad != nil {
		return nil, bad
	}
	part := ops.ResourceMain
	if kind == "agreement" {
		part = ops.ResourceAgreement
	}
	saved, failure := s.resources().SaveResource(ref, ops.ResourceEdit{Part: part, Text: in.Text, Hash: in.Hash})
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	out := k.savedAnswer(name, place)
	out.Text = saved.Text
	out.Hash = saved.Hash
	out.Path = s.cat.Display(saved.Path)
	return out, nil
}

// savedAnswer is the answer of a write: the one definition as GET /api/team
// reads it now, with the whole list under it (spec-boundary 5).
func (k apiTeam) savedAnswer(name, place string) apiSaved {
	list := k.s.t2List()
	out := apiSaved{Items: list.Items, Hashes: list.Hashes, Files: list.Files}
	if one, refusal := k.Get(name, place); refusal == nil {
		if got, ok := one.(apiTeamGet); ok {
			out.Doc, out.Text, out.Hash, out.Path = got.Doc, got.Text, got.Hash, got.Path
			out.Effective, out.Hidden = got.Effective, got.Hidden
		}
	}
	return out
}

// Delete is DELETE /api/team/{name}?place=&hash=&agreement_hash=: take one
// definition's folder out of its layer — team.yaml and agreement.md — and the
// directory itself when the two files were all it held (spec-teams B6). Each
// file the screen read must still be the one it read, so a file that moved under
// the screen is not removed.
func (k apiTeam) Delete(name string, in apiIn) (any, *apiErr) {
	target, bad := k.s.resourceSeen(domainplace.Definition, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	if failure := k.s.resources().DeleteResource(ops.ExistingResource(target), ops.ResourceDelete{Hash: in.Hash, AgreementHash: in.AgreementHash}); failure != nil {
		return nil, k.s.resourceFailure(failure, nil, true)
	}
	return k.List()
}

// t2SortedNames is the profile names the worker picker offers, in the order a
// person reads them.
func (s *Server) t2SortedNames() []string {
	profiles, _, err := s.cat.Profiles()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// apiTeamRowOps is the ops one row-shaped draft means: a 割り当て tab that still
// sends rows writes the picks it names, one line each (spec-boundary 6, r-8e85).
// A row leaves a pick out when it did not move, so a card that only changed
// roles does not blank the profiles.
func apiTeamRowOps(doc apiTeamDraft) []definition.Op {
	var ops []definition.Op
	for _, row := range doc.Members {
		if row.Profile != nil {
			ops = append(ops, definition.MemberProfile(row.Name, *row.Profile))
		}
		if row.Role != nil {
			ops = append(ops, definition.MemberRole(row.Name, *row.Role))
		}
	}
	for _, row := range doc.Workers {
		if row.Profile != nil {
			ops = append(ops, definition.WorkerProfile(row.From, row.Purpose, *row.Profile))
		}
	}
	return ops
}
