package web

import (
	"errors"
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// definition is the detail of one team definition (設計 3.4): its mission (the
// one field this stage writes), the figure, the members, the workers and the
// turns as tables, the team agreement, and the teams running from it.
func (s *Server) definition(r *http.Request) (view, error) {
	return s.definitionPage(r.PathValue("name"), nil)
}

// definitionPage draws the detail, with one field open when open says so.
func (s *Server) definitionPage(name string, open panelFn) (view, error) {
	def, err := s.readDefinition(name)
	if err != nil {
		return s.brokenDefinition(name, err)
	}
	data := s.definitionData(def, open)
	return s.renderDefinition(data)
}

// panelFn fills the open field of one definition, and whether the field's own
// URL named something this definition holds.
type panelFn func(*definition.Definition) (panel, bool)

// definitionData fills the detail's data from a definition that read.
func (s *Server) definitionData(def *definition.Definition, open panelFn) definitionData {
	findings := s.findings(def.Name)
	running, _ := s.runningTeams()
	data := definitionData{
		Name:    def.Name,
		Href:    definitionPath(def.Name),
		Path:    s.cat.Display(def.TeamYAML),
		Mission: def.Mission,
		Notice:  missionFindings(findings),
		Token:   s.token,
		Hash:    s.hashOf(def.TeamYAML),
		Draft:   "definition:" + def.Name + ":mission",
		Members: s.memberRows(def),
		Workers: workerRows(def),
		Turns:   turnRows(def),
		Running: runningFor(running, def.Name),
	}
	data.Agreement = s.teamAgreement(def)
	data.Unused = s.unusedFor(def.Name)
	openKey := ""
	if open != nil {
		if field, ok := open(def); ok {
			data.Panel = field
			data.PanelOpen = true
			openKey = field.Key
		}
	}
	data.Figure = s.figure(def, nil, openKey)
	return data
}

// renderDefinition renders the detail page.
func (s *Server) renderDefinition(data definitionData) (view, error) {
	body, err := s.show("definition.html", data)
	if err != nil {
		return view{}, err
	}
	crumb := []crumb{{Name: t("web.band.definitions"), Href: "/definitions"}, {Name: data.Name}}
	return view{title: tf("web.definition.title", data.Name), band: bandDefinitions, crumb: crumb, body: body}, nil
}

// brokenDefinition is the detail of a definition that cannot be read: the
// sentence validate would write, and the path, and nothing else (設計 3.4).
func (s *Server) brokenDefinition(name string, err error) (view, error) {
	path := s.teamYAMLOf(name)
	data := definitionData{
		Name:   name,
		Href:   definitionPath(name),
		Path:   s.cat.Display(path),
		Broken: s.breakLine(readErrorPath(err, path), err),
	}
	if errors.Is(err, catalog.ErrNotFound) {
		data.Broken = err.Error()
	}
	return s.renderDefinition(data)
}

// saveMission writes the mission (設計 3.4): the one field this stage writes.
// It edits one key, checks the change against the model, and writes only when
// the file is still the one the page read (設計 7.3).
func (s *Server) saveMission(r *http.Request) (view, error) {
	name := r.PathValue("name")
	def, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: def.TeamYAML})
	if err != nil {
		return view{}, err
	}
	mission := r.PostFormValue("mission")
	edited, err := definition.EditYAML([]byte(text), definition.Mission(mission))
	if err != nil {
		return s.definitionRefused(def, mission, err.Error())
	}
	with := s.cat.With(def.TeamYAML, string(edited))
	findings := check.Definition(check.Input{Catalog: with, Name: name})
	if msg, ok := stops(findings, "mission"); ok {
		return s.definitionRefused(def, mission, msg)
	}
	if msg, ok := stops(findings, ""); ok {
		return s.definitionRefused(def, mission, msg)
	}
	if err := source.WriteIfUnchanged(def.TeamYAML, r.PostFormValue("hash"), string(edited)); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return s.definitionRefused(def, mission, tf("web.saved.clash", s.cat.Display(def.TeamYAML)))
		}
		return view{}, err
	}
	return s.definitionSaved(name)
}

// definitionRefused draws the detail with one sentence beside the mission field
// and the change not written. The field keeps the text the person wrote, so
// nothing is lost (設計 3.0 の「書きかけ」).
func (s *Server) definitionRefused(def *definition.Definition, mission, notice string) (view, error) {
	data := s.definitionData(def, nil)
	data.Mission = mission
	data.Notice = append([]string{notice}, data.Notice...)
	return s.renderDefinition(data)
}

// definitionSaved draws the detail after a save: the new value, the saved line,
// and what a running team has to take up.
func (s *Server) definitionSaved(name string) (view, error) {
	def, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	running, _ := s.runningTeams()
	data := s.definitionData(def, nil)
	data.Saved = tf("web.saved.line", s.cat.Display(def.TeamYAML), "mission")
	data.Commands = effect.Commands(s.missionChanges(def), running)
	return s.renderDefinition(data)
}

// missionChanges is what a mission save does to the members' prompts: a mission
// is in no prompt, so nothing moves. The call is made rather than assumed, so
// the saved line's handover is the model's answer, not this package's.
func (s *Server) missionChanges(def *definition.Definition) []effect.Change {
	view := s.view(def, s.cat.Reader())
	return effect.Of(view, view)
}

// view is one read state of a definition, for effect.Of.
func (s *Server) view(def *definition.Definition, reader source.Reader) effect.View {
	profiles, _, _ := s.cat.Profiles()
	return effect.View{
		Def:      def,
		Profiles: profiles,
		Run: prompt.Run{
			Team:    def.Name,
			Base:    s.cat.Base(),
			Reader:  reader,
			Sources: s.cat,
			Display: s.cat.Display,
		},
	}
}

// hashOf is the hash of one file's text, or the hash of nothing when it cannot
// be read. The page carries it so a save can tell whether the file moved.
func (s *Server) hashOf(path string) string {
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return source.Hash("")
	}
	return source.Hash(text)
}

// teamYAMLOf is the team.yaml a definition name resolves to, the project's file
// first (the one the screens read). Empty when no layer holds one.
func (s *Server) teamYAMLOf(name string) string {
	for _, entry := range s.entries() {
		if entry.Name == name {
			return entry.Path
		}
	}
	return ""
}

// unusedFor is the global file of the same name that is not used, as a path, or
// empty (d-f730 の 3).
func (s *Server) unusedFor(name string) string {
	for _, entry := range s.entries() {
		if entry.Name == name && entry.UnusedPath != "" {
			return s.cat.Display(entry.UnusedPath)
		}
	}
	return ""
}

// entries is the catalog's list of the team definitions.
func (s *Server) entries() []catalog.Entry {
	return s.cat.Entries()
}

// runningFor is the running teams booted from one definition, as links to their
// own screens.
func runningFor(running []effect.Running, definition string) []turnEnd {
	var out []turnEnd
	for _, entry := range running {
		if entry.Definition == definition {
			out = append(out, turnEnd{Name: entry.Team, Href: teamPath(entry.Team)})
		}
	}
	return out
}

// runningLinks is running teams as links to their own screens.
func runningLinks(running []effect.Running) []turnEnd {
	out := make([]turnEnd, 0, len(running))
	for _, entry := range running {
		out = append(out, turnEnd{Name: entry.Team, Href: teamPath(entry.Team)})
	}
	return out
}

// missionFindings is the sentences beside the mission field.
func missionFindings(findings []check.Finding) []string {
	return fieldTexts(findings, "mission")
}

// memberRow is one row of the members table.
type memberRow struct {
	Name        string
	Href        string
	Role        string
	RoleHref    string
	Profile     string
	ProfileHref string
	Kind        string
	Turns       []turnEnd
}

// turnEnd is one end of a turn: what it is called, and where its field is.
type turnEnd struct {
	Name string
	Href string
}

// memberRows is the members table (設計 3.4 (2)).
func (s *Server) memberRows(def *definition.Definition) []memberRow {
	rows := make([]memberRow, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		row := memberRow{
			Name:        member.Name,
			Href:        memberPath(def.Name, member.Name),
			Role:        member.Role,
			RoleHref:    rolePath(member.Role),
			Profile:     member.Profile,
			ProfileHref: profilePath(member.Profile),
			Kind:        s.kindOf(member.Profile),
		}
		for _, edge := range def.Graph.Out(member.ID()) {
			row.Turns = append(row.Turns, turnEnd{Name: edge.To.String(), Href: turnHref(def.Name, edge.From.String(), edge.To.String())})
		}
		rows = append(rows, row)
	}
	return rows
}

// kindOf is the kind of one profile name, empty when there is none.
func (s *Server) kindOf(name string) string {
	profile, ok := s.cat.Profile(name)
	if !ok {
		return ""
	}
	return profile.Kind
}

// workerRow is one row of the workers table.
type workerRow struct {
	From        string
	FromHref    string
	Purpose     string
	Profile     string
	ProfileHref string
	Href        string
}

// workerRows is the workers table (設計 3.4 (3)).
func workerRows(def *definition.Definition) []workerRow {
	rows := make([]workerRow, 0, len(def.Graph.Workers))
	for _, worker := range def.Graph.Workers {
		rows = append(rows, workerRow{
			From:        worker.From,
			FromHref:    memberPath(def.Name, worker.From),
			Purpose:     worker.Purpose,
			Profile:     worker.Profile,
			ProfileHref: profilePath(worker.Profile),
			Href:        workerPath(def.Name, worker.From, worker.Purpose),
		})
	}
	return rows
}

// turnRow is one row of the turns table.
type turnRow struct {
	Label string
	Href  string
}

// turnRows is the turns table (設計 3.4 (4)): one row per line of turns:.
func turnRows(def *definition.Definition) []turnRow {
	rows := make([]turnRow, 0, len(def.Graph.Turns))
	for _, edge := range def.Graph.Turns {
		rows = append(rows, turnRow{Label: edge.String(), Href: turnHref(def.Name, edge.From.String(), edge.To.String())})
	}
	return rows
}

// turnHref is the field of one turn.
func turnHref(def, from, to string) string { return turnPath(def, from, to) }

// teamAgreement is the team agreement as a section of cards (設計 3.4 (5)), read
// only in this stage.
func (s *Server) teamAgreement(def *definition.Definition) *agreementSection {
	if !def.HasAgreement {
		return &agreementSection{Empty: true}
	}
	doc, err := s.ruleDoc(def.Agreement)
	if err != nil {
		return &agreementSection{Broken: s.breakLine(def.Agreement.Path, err)}
	}
	return &agreementSection{
		Path:     s.cat.Display(def.Agreement.Path),
		Sections: s.sectionsOf(doc, func(id string) string { return rulePath(def.Name, id) }),
	}
}

// agreementSection is one rule document drawn as sections of cards.
type agreementSection struct {
	Path     string
	Sections []ruleSection
	Empty    bool
	Broken   string
}

// definitionData is what the detail template is given.
type definitionData struct {
	Name      string
	Href      string
	Path      string
	Unused    string
	Broken    string
	Mission   string
	Notice    []string
	Saved     string
	Commands  []effect.TeamCommands
	Token     string
	Hash      string
	Draft     string
	Figure    figureData
	Members   []memberRow
	Workers   []workerRow
	Turns     []turnRow
	Agreement *agreementSection
	Running   []turnEnd
	Panel     panel
	PanelOpen bool
}
