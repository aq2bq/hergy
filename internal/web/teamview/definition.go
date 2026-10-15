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

// Routes are the definition page and the panel it opens in the right column. The
// figure's member boxes point at the same panel, so the panel of this package
// lives here.
func (s *definitionScreen) Routes() []web.Route {
	return []web.Route{
		{Path: "/team", Handler: s.show},
		{Path: "/panel/member", Handler: s.memberPanel},
	}
}

// Assets are the stylesheet of these screens. The frame mounts a name once, so
// one screen names it and the others use its classes.
func (s *definitionScreen) Assets() web.Assets { return assets }

func (s *definitionScreen) show(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	name, teams := s.choose(layer, r.URL.Query().Get("name"))
	data := definitionData{
		Layer:     layer,
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Name:      name,
		Teams:     pickTeams(teams, name),
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
		for _, member := range def.Members {
			data.Members = append(data.Members, memberRowOf(member))
			for _, worker := range member.Workers {
				data.Workers = append(data.Workers, workerRow{
					Member: member.Name, Purpose: worker.Purpose, Profile: worker.Profile,
				})
			}
		}
	}
	view, err := s.body("definition.html", data)
	view.Title = s.Name()
	return view, err
}

// definitionData is what the definition page is drawn from.
type definitionData struct {
	Layer     string
	LayerName string
	Name      string
	Path      string // team.yaml, as a person writes it
	Broken    string
	Teams     []pickTeam
	Docs      []Doc
	Members   []memberRow
	Workers   []workerRow
}

// pickTeam is one chip of the picker: a team definition of the layer, and
// whether Herdr reports it running.
type pickTeam struct {
	Name    string
	Active  bool
	Running bool
}

// pickTeams is the picker's chips, in the order the layer lists them.
func pickTeams(teams []web.Team, current string) []pickTeam {
	out := make([]pickTeam, 0, len(teams))
	for _, team := range teams {
		out = append(out, pickTeam{Name: team.Name, Active: team.Name == current, Running: team.Running})
	}
	return out
}

// memberRow is one row of the member table: the short name a rule writes, the
// role, the engine (claude, opencode) the member runs, and the profile it takes
// its settings from.
type memberRow struct {
	Name    string
	Role    string
	Kind    string
	Profile string
}

func memberRowOf(member Member) memberRow {
	return memberRow{Name: member.Name, Role: member.Role, Kind: member.Kind, Profile: member.Profile}
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
