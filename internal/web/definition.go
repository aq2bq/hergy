package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

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

// definitionPage draws the detail, with one field open when open names one. A
// field URL that names something the definition does not hold is a 404: the
// page says so in the field's place, rather than opening nothing (N2).
func (s *Server) definitionPage(name string, open panelFn) (view, error) {
	def, err := s.readDefinition(name)
	if err != nil {
		v, rerr := s.brokenDefinition(name, err)
		if rerr != nil {
			return v, rerr
		}
		if errors.Is(err, catalog.ErrNotFound) {
			// A name no layer holds is a 404, the way a field is (R3).
			v.status = http.StatusNotFound
		}
		return v, nil
	}
	data := s.definitionData(def, open)
	v, err := s.renderDefinition(data)
	if err != nil {
		return v, err
	}
	if data.Panel.Missing {
		v.status = http.StatusNotFound
	}
	return v, nil
}

// panelFn fills the open field of one definition. A field the definition does
// not hold comes back with Missing set, so the page can be a 404 (N2).
type panelFn func(*definition.Definition) panel

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
		Heading: fieldTexts(findings, ""),
		Members: s.memberRows(def, findings),
		Workers: workerRows(def),
		Turns:   turnRows(def),
		Running: runningFor(running, def.Name),
	}
	data.Agreement = s.teamAgreement(def)
	data.Unused = s.unusedFor(def.Name)
	openKey := ""
	if open != nil {
		data.Panel = open(def)
		data.PanelOpen = true
		openKey = data.Panel.Key
	}
	data.Figure = s.figure(def, nil, nodeProblems(findings), openKey)
	return data
}

// nodeProblems is the sentence the check writes about each node, by the node's
// name (the human is "human"), for the figure and the node's field (R1).
func nodeProblems(findings []check.Finding) map[string]string {
	out := map[string]string{}
	for _, finding := range findings {
		name, ok := strings.CutPrefix(finding.Field, "node:")
		if !ok {
			continue
		}
		if _, seen := out[name]; !seen {
			out[name] = finding.Text
		}
	}
	return out
}

// memberFindings is what the check says about one member: every sentence about
// the member, and the ones that belong to its role and its profile fields (R1).
func memberFindings(findings []check.Finding, index int) (all, role, profile []string) {
	base := fmt.Sprintf("members[%d]", index)
	for _, finding := range findings {
		switch {
		case finding.Field == base:
			all = append(all, finding.Text)
		case strings.HasPrefix(finding.Field, base+"."):
			all = append(all, finding.Text)
		}
		switch finding.Field {
		case base + ".role":
			role = append(role, finding.Text)
		case base + ".profile":
			profile = append(profile, finding.Text)
		}
	}
	return all, role, profile
}

// renderDefinition renders the detail page.
func (s *Server) renderDefinition(data definitionData) (view, error) {
	body, err := s.show("definition.html", data)
	if err != nil {
		return view{}, err
	}
	trail := []crumb{
		{Name: t("web.band.definitions"), Href: "/definitions"},
		{Name: data.Name, Href: definitionPath(data.Name)},
	}
	if data.PanelOpen && data.Panel.Crumb != "" && !data.Panel.Missing {
		// The trail names the open field, and the step before it closes it (N3).
		trail = append(trail, crumb{Name: data.Panel.Crumb})
	} else {
		trail[len(trail)-1].Href = ""
	}
	return view{title: tf("web.definition.title", data.Name), band: bandDefinitions, crumb: trail, body: body}, nil
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
		// The find-the-file sentence is for the one who typed a name; the
		// screen shows the sentence without the CLI's next line (N7).
		data.Broken = withoutNext(err.Error())
	}
	return s.renderDefinition(data)
}

// newStop is the first sentence that stops a save and is not already there: the
// change brings it in, so it is what to show beside the field (設計 7.3、T1).
func newStop(before, after []check.Finding) (string, bool) {
	had := make(map[string]bool, len(before))
	for _, finding := range before {
		if finding.Stops {
			had[finding.Text] = true
		}
	}
	for _, finding := range after {
		if finding.Stops && !had[finding.Text] {
			return finding.Text, true
		}
	}
	return "", false
}

// handover is what a save tells the running teams to take up, as the screen
// shows it: the teams that have something to type, and, when every running team
// has nothing, the one line that says so (a team with no commands is not named,
// so no team name stands beside a sentence about it).
type handover struct {
	Teams   []effect.TeamCommands
	Nothing bool
}

// handoverOf drops the running teams with no commands to type.
func handoverOf(commands []effect.TeamCommands) handover {
	out := handover{Nothing: len(commands) > 0}
	for _, team := range commands {
		if len(team.Commands) == 0 {
			continue
		}
		out.Teams = append(out.Teams, team)
		out.Nothing = false
	}
	return out
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

// withoutNext drops the "next: <command>" line a CLI error carries: a screen
// shows no next command (E2, N7).
func withoutNext(text string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "next:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// missionFindings is the sentences beside the mission field.
func missionFindings(findings []check.Finding) []string {
	return fieldTexts(findings, "mission")
}

// memberRow is one row of the members table.
type memberRow struct {
	Name            string
	Href            string
	Role            string
	RoleHref        string
	RoleFindings    []string
	Profile         string
	ProfileHref     string
	ProfileFindings []string
	Kind            string
	Findings        []string
	Turns           []turnEnd
}

// turnEnd is one end of a turn: what it is called, and where its field is.
type turnEnd struct {
	Name string
	Href string
}

// memberRows is the members table (設計 3.4 (2)), with the sentences the check
// writes about each member and its fields (R1).
func (s *Server) memberRows(def *definition.Definition, findings []check.Finding) []memberRow {
	rows := make([]memberRow, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		all, role, profile := memberFindings(findings, i)
		row := memberRow{
			Name:            member.Name,
			Href:            memberPath(def.Name, member.Name),
			Role:            member.Role,
			RoleHref:        rolePath(member.Role),
			Profile:         member.Profile,
			ProfileHref:     profilePath(member.Profile),
			Kind:            s.kindOf(member.Profile),
			Findings:        all,
			RoleFindings:    role,
			ProfileFindings: profile,
		}
		for _, edge := range def.Graph.Out(member.ID()) {
			if !isTurn(edge) {
				continue
			}
			row.Turns = append(row.Turns, turnEnd{Name: nodeLabel(edge.To), Href: turnHref(def.Name, edge.From.String(), edge.To.String())})
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
		rows = append(rows, turnRow{Label: turnLabel(edge), Href: turnHref(def.Name, edge.From.String(), edge.To.String())})
	}
	return rows
}

// turnHref is the field of one turn.
func turnHref(def, from, to string) string { return turnPath(def, from, to) }

// isTurn reports whether an edge is one a settings file wrote in turns:. A
// worker's two edges are made by workers: and are shown with the worker, not as
// a turn (I12c, 設計 3.4).
func isTurn(edge definition.Edge) bool {
	return edge.From.Kind != definition.KindWorker && edge.To.Kind != definition.KindWorker
}

// nodeLabel is one end of a turn as a person reads it: the human, a member's
// short name, or a worker by its purpose (d-a347 の 7, E1).
func nodeLabel(id definition.NodeID) string {
	switch id.Kind {
	case definition.KindHuman:
		return t("label.human")
	case definition.KindWorker:
		_, purpose := cutWorker(id.Name)
		return tf("web.worker_name", purpose)
	default:
		return id.Name
	}
}

// turnLabel is one turn as a person reads it: "lead → spec", with the arrow a
// settings file's turns: line writes with two hyphens (設計 4 節).
func turnLabel(edge definition.Edge) string {
	return nodeLabel(edge.From) + " → " + nodeLabel(edge.To)
}

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
	Name         string
	Href         string
	Path         string
	Unused       string
	Broken       string
	Heading      []string
	Mission      string
	Notice       []string
	Saved        string
	Preview      []string
	NoticeReload bool
	Handover     handover
	Token        string
	Hash         string
	Draft        string
	Figure       figureData
	Members      []memberRow
	Workers      []workerRow
	Turns        []turnRow
	Agreement    *agreementSection
	Running      []turnEnd
	Panel        panel
	PanelOpen    bool
}
