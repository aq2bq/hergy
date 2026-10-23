package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
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
// team's own agreement.yaml.

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

// apiTeamSummary is the team list's columns: the mission, who is on the team,
// and how much the team's own agreement holds. The per-member amount is gone
// (d-b87a); the definition's own size and budget are below.
type apiTeamSummary struct {
	Mission  string          `json:"mission"`
	Members  []apiTeamMember `json:"members"`
	Workers  int             `json:"workers"`
	Rules    int             `json:"rules"`
	Excluded int             `json:"excluded,omitempty"`
	Broken   bool            `json:"broken,omitempty"`
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
	// Segments is the member's prompt cut into the pieces it was built from,
	// in order; joining their texts gives the prompt exactly (r-4cf9).
	Segments []definition.PromptSegment `json:"segments,omitempty"`
}

// apiTeamWorker is one worker a member may hire.
type apiTeamWorker struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// apiTeamDoc is one team definition as the screen reads it: the mission, the
// members, the workers, the turns of the draft, the prompt's own lines, and the
// team's own agreement.
type apiTeamDoc struct {
	Mission   string          `json:"mission"`
	Members   []apiTeamMember `json:"members"`
	Workers   []apiTeamWorker `json:"workers"`
	Turns     []t2Turn        `json:"turns"`
	Prompt    t2Prompt        `json:"prompt"`
	Agreement *apiTeamAgree   `json:"agreement,omitempty"`
	Broken    bool            `json:"broken,omitempty"`
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
// and the document itself. A team with no agreement.yaml answers with Has false
// and no document, so the screen draws the empty agreement a save would write.
type apiTeamAgree struct {
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	Has      bool   `json:"has"`
	Rules    int    `json:"rules"`
	Excluded int    `json:"excluded,omitempty"`
	Doc      *r2Doc `json:"doc,omitempty"`
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

// t2List is the team list answer: the rows the screen reads, as items, plus the
// YAML tab's files.
func (s *Server) t2List() apiList {
	out := apiList{Hashes: map[string]string{}}
	for _, listing := range s.cat.Definitions() {
		out.Items = append(out.Items, s.t2ItemOf(listing))
		out.Files = append(out.Files, s.t2FilesOf(listing)...)
	}
	return out
}

// t2ItemOf is one team definition as the list carries it: each place's reading
// becomes the item's reading of that place, and the effective place's columns
// its summary.
func (s *Server) t2ItemOf(listing catalog.Listing) apiItem {
	item := apiItem{Name: listing.Name, Places: map[string]apiPlace{}}
	for _, place := range t2PlacesOf(listing) {
		layer := catalog.Layer(place)
		path := s.cat.TeamYAML(listing.Name, layer)
		text, err := s.cat.Reader().Read(source.Doc{Path: path})
		if err != nil {
			continue
		}
		def, _ := s.cat.Definition(listing.Name, layer)
		item.Places[place] = apiPlace{
			Doc:      s.t2DocOf(listing.Name, layer, def),
			Text:     text,
			Hash:     source.Hash(text),
			Path:     s.cat.Display(path),
			Problems: s.t2CountsOf(listing.Name, layer),
		}
	}
	effective, hidden := t2Effective(listing)
	item.Effective, item.Hidden = effective, hidden
	if place, ok := item.Places[effective]; ok {
		item.Problems = place.Problems
	}
	def, _ := s.cat.Definition(listing.Name, catalog.Layer(effective))
	item.Summary = s.t2SummaryOf(listing.Name, catalog.Layer(effective), def)
	return item
}

// t2Effective is which place of a definition is in effect and which is hidden,
// the same words a row's pills carry (spec-teams 1.4.5).
func t2Effective(listing catalog.Listing) (string, []string) {
	switch {
	case listing.Local && listing.Global:
		return catalog.PlaceProject, []string{catalog.PlaceGlobal}
	case listing.Global:
		return catalog.PlaceGlobal, nil
	}
	return catalog.PlaceProject, nil
}

// t2DocOf is one place's team definition in the screen's shape.
func (s *Server) t2DocOf(name, layer string, def *definition.Definition) apiTeamDoc {
	return s.t2DocWithAgree(s.cat, name, def, s.t2AgreeOf(name, layer))
}

// t2DocWithAgree is t2DocOf with the agreement given rather than read: a
// definition that is not there yet has no agreement.yaml to read, so its rules
// come from the text the row stands on (spec-teams B2).
func (s *Server) t2DocWithAgree(cat *catalog.Catalog, name string, def *definition.Definition, agree *apiTeamAgree) apiTeamDoc {
	out := apiTeamDoc{Agreement: agree}
	if def == nil || def.Graph == nil {
		out.Broken = true
		return out
	}
	out.Mission = def.Mission
	run := prompt.Run{
		Team: name, Executable: promptExecutable(), Base: cat.Base(),
		Reader: cat.Reader(), Sources: cat, Display: cat.Display,
	}
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		entry := apiTeamMember{Name: member.Name, Role: member.Role, Profile: member.Profile}
		if built, err := prompt.Member(def, member, run); err == nil {
			entry.Segments = built.Segments
		}
		out.Members = append(out.Members, entry)
	}
	for _, w := range def.Graph.Workers {
		out.Workers = append(out.Workers, apiTeamWorker{From: w.From, Purpose: w.Purpose, Profile: w.Profile})
	}
	out.Turns = t2TurnsOf(def)
	out.Prompt = t2PromptOf(def, name)
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

// apiMemberPrompt is one member's prompt as the check answers it before a save.
type apiMemberPrompt struct {
	Name     string                     `json:"name"`
	Segments []definition.PromptSegment `json:"segments"`
}

// t2SummaryOf is the list's columns for one definition.
func (s *Server) t2SummaryOf(name, layer string, def *definition.Definition) apiTeamSummary {
	out := apiTeamSummary{}
	if def == nil || def.Graph == nil {
		// The team screen draws 使命 — and no numbers for a file that does not
		// read (spec-teams 1.9), but the role screen's 割り当て tab still counts
		// the people the file names: the roles a member plays stand in the text
		// whether or not the definition reads (roles_v2_data.go did the same).
		out.Broken = true
		if text, err := s.cat.Reader().Read(source.Doc{Path: s.cat.TeamYAML(name, layer)}); err == nil {
			out.Members = t2MembersOfText(text)
		}
	} else {
		out.Mission = def.Mission
		out.Workers = len(def.Graph.Workers)
		for _, m := range def.Graph.Members {
			out.Members = append(out.Members, apiTeamMember{Name: m.Name, Role: m.Role, Profile: m.Profile})
		}
	}
	agree := s.t2AgreeOf(name, layer)
	out.Rules, out.Excluded = agree.Rules, agree.Excluded
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

// t2AgreeOf is one layer's agreement.yaml for a definition: the document, the
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
	doc, _ := rules.ReadAll(text, out.Path)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	out.Rules = len(doc.IncludedRules()) + doc.ExcludedCount()
	out.Excluded = doc.ExcludedCount()
	out.Doc = r2DocOf(doc, out.Hash, "")
	return out
}

// t2AgreeFile is a team's own agreement.yaml in one layer.
func (s *Server) t2AgreeFile(name, layer string) string {
	return filepath.Join(filepath.Dir(s.cat.TeamYAML(name, layer)), "agreement.yaml")
}

// t2CountsOf is how many errors and warnings one place's definition carries:
// the one check's findings, counted the way the list's column does (spec-teams
// 1.2). The row, the panel and the YAML head read this same check (d-a575).
func (s *Server) t2CountsOf(name, layer string) apiCounts {
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

// t2FilesOf is every team.yaml and agreement.yaml a layer holds for one name,
// the files the YAML tab can open.
func (s *Server) t2FilesOf(listing catalog.Listing) []apiFile {
	var out []apiFile
	add := func(place, kind, path string, hidden bool) {
		if path == "" {
			return
		}
		if _, err := s.cat.Reader().Read(source.Doc{Path: path}); err != nil {
			return
		}
		out = append(out, s.t2FileOf(place, kind, listing.Name, path, hidden))
	}
	if listing.Local {
		add(catalog.PlaceProject, "team", listing.LocalPath, false)
		add(catalog.PlaceProject, "agreement", filepath.Join(filepath.Dir(listing.LocalPath), "agreement.yaml"), false)
	}
	if listing.Global {
		add(catalog.PlaceGlobal, "team", listing.GlobalPath, listing.Local)
		add(catalog.PlaceGlobal, "agreement", filepath.Join(filepath.Dir(listing.GlobalPath), "agreement.yaml"), listing.Local)
	}
	return out
}

// t2FileOf is one file of the YAML tab, with the problems its own check finds.
// A team.yaml finding carries a field rather than a line, so the line of the
// key it names travels beside it (spec-teams 1.8).
func (s *Server) t2FileOf(place, kind, name, path string, hidden bool) apiFile {
	layer := catalog.Layer(place)
	display := s.cat.Display(path)
	text, _ := s.cat.Reader().Read(source.Doc{Path: path})
	out := apiFile{
		Place: place, Name: name, Kind: kind, Label: t2PlaceWord(place),
		Path: display, Hash: source.Hash(text), Text: text, Hidden: hidden,
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
			Role: finding.Role, RuleID: finding.RuleID,
		})
	}
	return out
}

// t2PlacesOf is every layer this name is in, the project's first.
func t2PlacesOf(listing catalog.Listing) []string {
	var out []string
	if listing.Local {
		out = append(out, catalog.PlaceProject)
	}
	if listing.Global {
		out = append(out, catalog.PlaceGlobal)
	}
	return out
}

// t2PlaceWord is a place as a person reads it.
func t2PlaceWord(place string) string {
	if place == catalog.PlaceGlobal {
		return "共通"
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
	layer := catalog.Layer(place)
	path := s.cat.TeamYAML(name, layer)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	def, _ := s.cat.Definition(name, layer)
	effective, hidden := s.t2Places(name)
	return apiTeamGet{
		Doc: s.t2DocOf(name, layer, def), Text: text, Hash: source.Hash(text),
		Path: s.cat.Display(path), Effective: effective, Hidden: hidden,
	}, nil
}

// t2Places is which place of a definition is in effect and which is hidden,
// asked of the catalog rather than of a listing.
func (s *Server) t2Places(name string) (string, []string) {
	_, projectErr := s.cat.Reader().Read(source.Doc{Path: s.cat.TeamYAML(name, catalog.Layer(catalog.PlaceProject))})
	_, globalErr := s.cat.Reader().Read(source.Doc{Path: s.cat.TeamYAML(name, catalog.Layer(catalog.PlaceGlobal))})
	switch {
	case projectErr == nil && globalErr == nil:
		return catalog.PlaceProject, []string{catalog.PlaceGlobal}
	case globalErr == nil:
		return catalog.PlaceGlobal, nil
	}
	return catalog.PlaceProject, nil
}

// Check is POST /api/team/check: what is wrong with a draft, an effect, or a
// text. The payload's own shape says which: a Doc is a document's draft, no Doc
// is a whole text the YAML tab is typing.
func (k apiTeam) Check(in apiIn) (any, *apiErr) {
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
}

// checkWorkerPrompt answers one worker's prompt as segments, before the hire
// (r-4cf9). It is the same prompt hire --dry builds, for the screen that shows
// what a worker would receive.
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
	layer := catalog.Layer(in.Place)
	def, err := k.s.cat.Definition(in.Name, layer)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	lead, err := t2HiringMember(def, body.Purpose, body.From)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	herdr := in.Name + "-" + body.Name
	if !profile.NamePattern.MatchString(herdr) {
		return nil, refuse(http.StatusUnprocessableEntity, i18n.Tf("error.herdr_name_pattern", herdr))
	}
	built, err := prompt.Worker(prompt.WorkerInput{
		Team: in.Name, Lead: in.Name + "-" + lead.Name, LeadRole: lead.Role,
		Purpose: body.Purpose, Name: herdr, Task: body.Task,
	})
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	return apiCheck{Prompt: built.Segments}, nil
}

// t2HiringMember is the member a worker preview hires through: the one from:
// names, or the one whose workers: pair this purpose. Two candidates are a
// refusal, because the screen has to say which (the command line asks too).
func t2HiringMember(def *definition.Definition, purpose, from string) (*definition.MemberNode, error) {
	if def == nil || def.Graph == nil {
		return nil, i18n.Errorf("error.no_member", from)
	}
	if from != "" {
		for i := range def.Graph.Members {
			member := &def.Graph.Members[i]
			if member.Name == from || def.Name+"-"+member.Name == from {
				return member, nil
			}
		}
		return nil, i18n.Errorf("error.no_member", from)
	}
	var names []string
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		for _, worker := range def.Graph.Workers {
			if worker.From == member.Name && worker.Purpose == purpose {
				names = append(names, member.Name)
				break
			}
		}
	}
	switch len(names) {
	case 0:
		return nil, i18n.Errorf("error.worker_prompt_none", purpose)
	case 1:
		for i := range def.Graph.Members {
			if def.Graph.Members[i].Name == names[0] {
				return &def.Graph.Members[i], nil
			}
		}
	}
	return nil, i18n.Errorf("error.from_required_many", strings.Join(names, i18n.T("list.separator")))
}

// checkFigure answers what is wrong with the figure a panel holds: the saved
// text with the panel's ops applied, checked as if it were saved (spec-teams
// C2). When the payload carries the one op a card is trying, the answer carries
// that op's effect too. The answer carries the topology tab's YAML preview as
// well: the text a save would write and the lines one edit changed (spec-teams
// B7).
func (k apiTeam) checkFigure(in apiIn) (any, *apiErr) {
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	s := k.s
	layer := catalog.Layer(in.Place)
	path := s.cat.TeamYAML(in.Name, layer)
	own := s.cat.Display(path)
	team := check.Team(check.TeamInput{Catalog: s.cat, Name: in.Name, Layer: layer, Ops: body.ops})
	out := apiCheck{Problems: t2FigureProblems(team.Problems, own)}
	saved, _ := s.cat.Reader().Read(source.Doc{Path: path})
	draft, err := t2ApplyOps(saved, own, body.ops)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	out.Preview = apiTeamPreview{
		Lines: definition.YAMLLines(draft),
		Diff:  check.LineDiff(t2LineTexts(definition.YAMLLines(saved)), t2LineTexts(definition.YAMLLines(draft))),
	}
	if body.in.Op != nil {
		op, err := body.in.Op.op()
		if err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, err.Error())
		}
		report := check.TeamEffect(check.TeamInput{Catalog: k.s.cat, Name: in.Name, Layer: layer, Ops: body.ops}, op)
		out.Effect = apiTeamEffect{Lines: report.Lines, Problems: t2FigureProblems(report.Problems, own)}
	}
	// The definition's own size, counted the way d-00a9 says, for the screen
	// that shows it against the provisional budget (r-2f83). It is the same
	// number the size warning counts, and the one the list's row carries.
	out.Limit = check.TeamCharLimit
	if overlay, refusal := k.overlay(in.Name, in.Place, body.ops); refusal == nil {
		if def, err := overlay.Definition(in.Name, layer); err == nil {
			out.Size = check.TeamSizeChars(overlay, def)
			out.Members = memberPrompts(overlay, def, in.Name)
		}
	}
	return out, nil
}

// memberPrompts is every member's prompt in the overlaid definition, as the
// segments the member's attribute shows (r-4cf9). A member whose prompt cannot
// be built has none; the check's problems say why.
func memberPrompts(cat *catalog.Catalog, def *definition.Definition, team string) []apiMemberPrompt {
	if def == nil || def.Graph == nil {
		return nil
	}
	run := prompt.Run{
		Team: team, Executable: promptExecutable(), Base: cat.Base(),
		Reader: cat.Reader(), Sources: cat, Display: cat.Display,
	}
	out := make([]apiMemberPrompt, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		built, err := prompt.Member(def, member, run)
		if err != nil {
			continue
		}
		out = append(out, apiMemberPrompt{Name: member.Name, Segments: built.Segments})
	}
	return out
}

// checkAgreement answers what is wrong with a team's agreement, checked with
// the figure's draft laid over the team.yaml (spec-teams C6, 1.5).
func (k apiTeam) checkAgreement(in apiIn) (any, *apiErr) {
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	if body.in.Doc == nil {
		return nil, refuse(http.StatusUnprocessableEntity, "agreement: one document is needed")
	}
	cat, refusal := k.overlay(in.Name, in.Place, body.ops)
	if refusal != nil {
		return nil, refusal
	}
	report := check.Agreement(check.AgreementInput{
		Catalog: cat, Name: in.Name, Place: catalog.Layer(in.Place), Draft: body.in.Doc.document(),
	})
	return r2CheckAnswer(report, false), nil
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

// overlay is the catalog with one definition's team.yaml replaced by the saved
// text with the ops applied: the text an agreement check reads through, so its
// arrows are judged against the figure's draft turns.
func (k apiTeam) overlay(name, place string, ops []definition.Op) (*catalog.Catalog, *apiErr) {
	layer := catalog.Layer(place)
	path := k.s.cat.TeamYAML(name, layer)
	text, err := k.s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	for _, op := range ops {
		next, err := definition.EditYAML([]byte(text), k.s.cat.Display(path), op)
		if err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, err.Error())
		}
		text = string(next)
	}
	return k.s.cat.With(path, text), nil
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
			Sev: sev, At: finding.Field, Field: finding.Field, Text: withoutOwnHead(finding.Text, own),
			Role: finding.Role, RuleID: finding.RuleID,
		})
	}
	return out
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
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	layer := catalog.Layer(place)
	out := apiCheck{}
	if kind == "agreement" {
		report := check.Agreement(check.AgreementInput{Catalog: k.s.cat, Name: in.Name, Place: layer, Text: in.Text})
		out.Problems = r2Problems(report.Problems, true)
		return out, nil
	}
	path := k.s.cat.TeamYAML(in.Name, layer)
	display := k.s.cat.Display(path)
	findings := check.Team(check.TeamInput{Catalog: k.s.cat.With(path, in.Text), Name: in.Name, Layer: layer}).Problems
	for _, finding := range findings {
		sev := "warn"
		if finding.Stops {
			sev = "err"
		}
		out.Problems = append(out.Problems, apiProblem{
			Sev: sev, At: "line", Field: finding.Field, Line: t2YamlLine(in.Text, finding.Field), Text: withoutOwnHead(finding.Text, display),
			Role: finding.Role, RuleID: finding.RuleID,
		})
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
	return in.Place, kind, true
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
	s := k.s
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	if err := s.t2LayerDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	path := s.cat.TeamYAML(name, layer)
	display := s.cat.Display(path)
	text, readErr := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if readErr == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	if refusal := t2RefuseNewTeamError(s.cat, name, layer, body.ops); refusal != nil {
		return nil, refusal
	}
	edited := text
	for _, op := range body.ops {
		next, err := definition.EditYAML([]byte(edited), display, op)
		if err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, err.Error())
		}
		edited = string(next)
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return r2WriteError(err, display)
	}
	return k.savedAnswer(name, in.Place), nil
}

// saveAgreement writes a team's own agreement.yaml from the panel's document.
// An agreement the team does not have yet is written whole (spec-teams 1.5).
func (k apiTeam) saveAgreement(name string, in apiIn) (any, *apiErr) {
	s := k.s
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	if body.in.Doc == nil {
		return nil, refuse(http.StatusUnprocessableEntity, "agreement: one document is needed")
	}
	if err := s.t2LayerDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	path := s.t2AgreeFile(name, layer)
	display := s.cat.Display(path)
	text, readErr := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if readErr == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	draft := body.in.Doc
	var edited string
	var err error
	if readErr != nil {
		edited, err = rules.Emit(draft.document())
	} else {
		saved, _ := rules.ReadAll(text, display)
		if saved == nil {
			saved = &rules.Document{Path: path}
		}
		written, rewriteErr := rules.Rewrite([]byte(text), saved, rules.Draft{
			Doc: draft.document(), From: draft.froms(), SectionFrom: draft.sectionFroms(),
			SaveFrom: rules.SaveFromTeamDefinition,
		})
		edited, err = string(written), rewriteErr
	}
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return r2WriteError(err, display)
	}
	out := k.savedAnswer(name, in.Place)
	if one, refusal := k.Get(name, in.Place); refusal == nil {
		if got, ok := one.(apiTeamGet); ok {
			agree := got.Doc.Agreement
			out.Doc, out.Text, out.Hash, out.Path = agree.Doc, edited, source.Hash(edited), agree.Path
		}
	}
	return out, nil
}

// saveYAML writes one file's text as the person wrote it. It is refused only
// when a problem the saved file did not hold has appeared (spec-teams X3).
func (k apiTeam) saveYAML(name string, in apiIn) (any, *apiErr) {
	s := k.s
	place, kind, ok := k.fileOf(name, in)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	if err := s.t2LayerDir(place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(place)
	path := s.cat.TeamYAML(name, layer)
	if kind == "agreement" {
		path = s.t2AgreeFile(name, layer)
	}
	display := s.cat.Display(path)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if err == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	if kind == "agreement" {
		if refusal := t2RefuseNewRuleError(display, text, in.Text); refusal != nil {
			return nil, refusal
		}
	} else if refusal := t2RefuseNewTeamText(s.cat, path, name, layer, text, in.Text); refusal != nil {
		return nil, refusal
	}
	if err := source.WriteIfUnchanged(path, hash, in.Text); err != nil {
		return r2WriteError(err, display)
	}
	out := k.savedAnswer(name, place)
	out.Text, out.Hash, out.Path = in.Text, source.Hash(in.Text), display
	return out, nil
}

// t2RefuseNewTeamError is the refusal for the first error a panel's ops would
// add to the saved team.yaml, or nil.
func t2RefuseNewTeamError(cat *catalog.Catalog, name, layer string, ops []definition.Op) *apiErr {
	known := map[string]bool{}
	for _, finding := range check.Team(check.TeamInput{Catalog: cat, Name: name, Layer: layer}).Problems {
		if finding.Stops {
			known[finding.Text] = true
		}
	}
	for _, finding := range check.Team(check.TeamInput{Catalog: cat, Name: name, Layer: layer, Ops: ops}).Problems {
		if finding.Stops && !known[finding.Text] && !t2TurnGateFinding(finding) {
			return refuse(http.StatusUnprocessableEntity, finding.Text)
		}
	}
	return nil
}

// t2RefuseNewTeamText is the refusal for the first error typed team.yaml holds
// that the saved one did not, or nil.
func t2RefuseNewTeamText(cat *catalog.Catalog, path, name, layer, before, after string) *apiErr {
	known := map[string]bool{}
	for _, finding := range check.Team(check.TeamInput{Catalog: cat.With(path, before), Name: name, Layer: layer}).Problems {
		if finding.Stops {
			known[finding.Text] = true
		}
	}
	for _, finding := range check.Team(check.TeamInput{Catalog: cat.With(path, after), Name: name, Layer: layer}).Problems {
		if finding.Stops && !known[finding.Text] && !t2TurnGateFinding(finding) {
			return refuse(http.StatusUnprocessableEntity, withoutOwnHead(finding.Text, cat.Display(path)))
		}
	}
	return nil
}

// t2TurnGateFinding is the one kind of new stop a save lets through (d-9abb):
// a node with no turn (I12a) or a definition no turn reaches the human from
// (I12b). They are kept as team-definition problems, so the row, the panel,
// validate and up say them; a save that is a step in a longer edit can keep its
// work. Every other new stop still refuses.
func t2TurnGateFinding(finding check.Finding) bool {
	return finding.Kind == check.KindGraph && strings.HasPrefix(finding.Field, "node:")
}

// t2RefuseNewRuleError is the refusal for the first rule file error a typed
// text holds that the saved one did not, or nil.
func t2RefuseNewRuleError(display, before, after string) *apiErr {
	_, saved := rules.ReadAll(before, display)
	known := map[string]bool{}
	for _, problem := range saved {
		if problem.Sev == rules.SevError {
			known[problem.Text] = true
		}
	}
	_, typed := rules.ReadAll(after, display)
	for _, problem := range typed {
		if problem.Sev == rules.SevError && !known[problem.Text] {
			return refuse(http.StatusUnprocessableEntity, problem.Text)
		}
	}
	return nil
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
// definition's folder out of its layer — team.yaml and agreement.yaml — and the
// directory itself when the two files were all it held (spec-teams B6). Each
// file the screen read must still be the one it read, so a file that moved under
// the screen is not removed.
func (k apiTeam) Delete(name string, in apiIn) (any, *apiErr) {
	s := k.s
	layer := catalog.Layer(in.Place)
	teamPath := s.cat.TeamYAML(name, layer)
	agreePath := s.t2AgreeFile(name, layer)
	for _, one := range []struct{ path, hash string }{
		{teamPath, in.Hash},
		{agreePath, in.AgreementHash},
	} {
		if one.hash == "" {
			continue
		}
		text, err := s.cat.Reader().Read(source.Doc{Path: one.path})
		pathHash := source.Hash("")
		if err == nil {
			pathHash = source.Hash(text)
		}
		if one.hash != pathHash {
			return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(one.path)))
		}
	}
	for _, path := range []string{teamPath, agreePath} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, refuse(http.StatusInternalServerError, err.Error())
		}
	}
	// The directory goes too when nothing else is left in it.
	_ = os.Remove(filepath.Dir(teamPath))
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
