package teamview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// definitionScreen is the screen of one team definition: the documents every
// member reads, the members and the workers they may call. It is read-only; the
// one thing a person does here is open a member's prompt in the right panel.
type definitionScreen struct{ base }

// Name is what the navigation calls this screen.
func (s *definitionScreen) Name() string { return i18n.T("web.teamview.definition.title") }

// InNav says the navigation does not list this screen (d-207f): it is opened
// from the left list, and from the teams, running and figure screens, so a tab
// for it would show the same place twice.
func (s *definitionScreen) InNav() bool { return false }

// Routes are the definition page, the panel it opens in the right column, and
// the fragment that closes that panel again. The figure's member boxes point at
// the same panel, so the panel of this package lives here.
func (s *definitionScreen) Routes() []web.Route {
	return []web.Route{
		{Path: "/team", Handler: s.show},
		{Path: "/panel/member", Handler: s.memberPanel},
		{Path: "/panel/close", Handler: s.closePanel},
	}
}

// show answers the definition page. A link may name one member (?member=), and
// then the page opens with that member in the right panel: the button in the
// member table is a link to the same page, so a browser with no script still
// swaps the panel by following it (d-207f).
func (s *definitionScreen) show(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	name, teams := s.choose(layer, r.URL.Query().Get("name"))
	data := definitionData{
		Layer:     layer,
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Name:      name,
	}
	if name != "" {
		for _, team := range teams {
			if team.Name == name {
				data.Launch = pickTeamOf(team, name)
				data.HasLaunch = true
				break
			}
		}
	}
	// A layer with no team definition has nothing to read: the page says so
	// rather than asking the app for a definition that is not there.
	if name != "" {
		def, err := s.port.Definition(layer, name)
		if err != nil {
			return web.View{}, err
		}
		data.Path = def.Path
		data.Broken = def.Broken
		data.Docs = def.Docs
		shared := sharedRules(def)
		for _, member := range def.Members {
			data.Members = append(data.Members, memberRowOf(member, shared))
			for _, worker := range member.Workers {
				data.Workers = append(data.Workers, workerRow{
					Member: member.Name, Purpose: worker.Purpose, Profile: worker.Profile,
				})
			}
		}
	}
	open := r.URL.Query().Get("member")
	for i := range data.Members {
		data.Members[i].Open = data.Members[i].Name == open
	}
	view, err := s.body("definition.html", data)
	if err != nil {
		return web.View{}, err
	}
	view.Title = s.Name()
	if open != "" {
		// The member panel is the whole drawer element: the frame puts it in
		// the right column when this page is opened with a member named.
		panel, err := s.panelBody(layer, name, open)
		if err != nil {
			return web.View{}, err
		}
		view.Drawer = web.HTML(panel)
	}
	return view, nil
}

// definitionData is what the definition page is drawn from.
type definitionData struct {
	Layer     string
	LayerName string
	Name      string
	Path      string // team.yaml, as a person writes it
	Broken    string
	// Launch is the layer's row for this definition, with the launch record
	// and the hash it wrote; HasLaunch is false when the layer does not list
	// the definition (a name from another layer's link, say).
	Launch    pickTeam
	HasLaunch bool
	Docs      []Doc
	Members   []memberRow
	Workers   []workerRow
}

// pickTeam is one team definition of the layer as the definition screen and the
// figure's picker need it: the name, whether the figure is showing it, whether
// Herdr reports it running, and the launch record the team list counts.
type pickTeam struct {
	Name     string
	Active   bool
	Running  bool
	Launched string // when the definition was last launched here, empty for never
	Hash     string // the head of the hash that launch wrote
	Same     bool   // the files on disk build that same hash now
}

// pickTeams is the picker's chips, in the order the layer lists them.
func pickTeams(teams []web.Team, current string) []pickTeam {
	out := make([]pickTeam, 0, len(teams))
	for _, team := range teams {
		out = append(out, pickTeamOf(team, current))
	}
	return out
}

// pickTeamOf turns one team-list row into the shape these screens read.
func pickTeamOf(team web.Team, current string) pickTeam {
	return pickTeam{
		Name:     team.Name,
		Active:   team.Name == current,
		Running:  team.Running,
		Launched: team.Launched,
		Hash:     team.Hash,
		Same:     team.Same,
	}
}

// memberRow is one row of the member table: the short name a rule writes, the
// role, the engine (claude, opencode) the member runs, the profile it takes its
// settings from, how many rules its prompt carries, and how many workers it may
// hire.
type memberRow struct {
	Name    string
	Role    string
	Kind    string
	Profile string
	Rules   int // the rules the member's prompt carries: shared docs + role file
	Workers int
	// Open says this member's prompt is in the right panel on this answer.
	Open bool
}

// memberRowOf turns one member into its row. shared is how many rules the
// documents every member reads carry, which is the head of this member's prompt.
func memberRowOf(member Member, shared int) memberRow {
	rules := shared
	if member.RoleFile != nil {
		rules += len(member.RoleFile.Rules)
	}
	return memberRow{
		Name: member.Name, Role: member.Role, Kind: member.Kind, Profile: member.Profile,
		Rules: rules, Workers: len(member.Workers),
	}
}

// sharedRules is how many rules the documents every member reads carry.
func sharedRules(def Definition) int {
	total := 0
	for _, doc := range def.Docs {
		total += len(doc.Rules)
	}
	return total
}

// workerRow is one row of the worker table: a member, and a worker it may hire.
type workerRow struct {
	Member  string
	Purpose string
	Profile string
}

// liveStates is each member's herdr state, by the member's herdr name, and the
// running team they come from. The panel and the figure overlay it when Herdr
// says the definition is running; a Herdr that cannot be read leaves every
// member unmarked.
func (b base) liveStates(layer, name string) (map[string]status.Row, *LiveTeam) {
	herdr, err := b.port.Running(layer)
	if err != nil {
		return nil, nil
	}
	live := liveFor(herdr, name)
	if live == nil {
		return nil, nil
	}
	out := make(map[string]status.Row, len(live.Rows))
	for _, row := range live.Rows {
		out[row.Name] = row
	}
	return out, live
}
