package teamview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// runningScreen is the screen of the teams Herdr reports as running right now.
// It reads them the way hergy status reads them, shows what hergy status does
// not (each member's terminal title and pane), and holds the one button of
// these screens: moving Herdr's focus to a member's pane.
//
// The page reads Herdr again every five seconds (HTMX's every 5s), so hergy
// holds no state of its own. A Herdr that is missing or unreadable is one line
// on this screen; the other screens keep working.
type runningScreen struct{ base }

// Name is what the navigation calls this screen.
func (s *runningScreen) Name() string { return i18n.T("web.teamview.running.title") }

// Routes are the page, the list it reads again on its own, and the one write of
// these screens.
func (s *runningScreen) Routes() []web.Route {
	return []web.Route{
		{Path: "/running", Handler: s.show},
		{Path: "/running/list", Handler: s.list},
		{Path: "/running/focus", Write: true, Handler: s.focus},
	}
}

func (s *runningScreen) show(r *http.Request) (web.View, error) {
	return s.answer(r, s.data(r, nil))
}

// list answers the part of the page that is read again: HTMX asks for it every
// five seconds and when the button is pressed.
func (s *runningScreen) list(r *http.Request) (web.View, error) {
	return s.answer(r, s.data(r, nil))
}

// focus moves Herdr's focus to one member's pane, then answers with the list
// read again: the pane Herdr has just seen changes that list, so the person sees
// what the move did. A focus that fails is the one line at the top of the list.
func (s *runningScreen) focus(r *http.Request) (web.View, error) {
	return s.answer(r, s.data(r, s.port.Focus(r.PostFormValue("pane"))))
}

// answer writes the list as the fragment HTMX swaps in, when HTMX asked for it,
// and as the whole page otherwise: a person who opens the path themselves sees
// the page, not a fragment of it.
func (s *runningScreen) answer(r *http.Request, data runningData) (web.View, error) {
	if htmx(r) {
		return s.bare("live-list", data)
	}
	view, err := s.body("running.html", data)
	view.Title = s.Name()
	return view, err
}

func (s *runningScreen) data(r *http.Request, err error) runningData {
	layer := web.LayerOf(r)
	herdr, readErr := s.port.Running(layer)
	data := runningData{
		Layer:     layer,
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Token:     web.Token(r),
		Legend:    legend(),
	}
	if readErr != nil {
		data.Notice = notice(readErr)
		return data
	}
	data.At = herdr.At.Format("15:04:05")
	if err != nil {
		data.Notice = notice(err)
	}
	for _, team := range herdr.Teams {
		data.Teams = append(data.Teams, liveTeamOf(team))
	}
	return data
}

// runningData is what the running-team page is drawn from.
type runningData struct {
	Layer     string
	LayerName string
	Token     string // the token every form of this run carries
	At        string // when Herdr was read, as a person reads a clock
	Notice    string // one line: Herdr could not be read, or the focus failed
	Teams     []liveTeam
	Legend    []word
}

// liveTeam is one running team as the page shows it: where it came from, its
// members and workers, and the judgement over them.
type liveTeam struct {
	Name       string
	Definition string
	TabID      string
	Known      bool // the layer has a team definition of this name
	Rows       []liveRow
	Verdict    string
}

// liveRow is one member or worker of a running team.
type liveRow struct {
	Name     string
	State    string // the word the pill is coloured by
	Text     string // what the pill writes: the state, or "not present"
	Sentence string // the role, the engine, and what the terminal says
	PaneID   string
	Worker   bool
}

func liveTeamOf(team LiveTeam) liveTeam {
	out := liveTeam{
		Name:       team.Team,
		Definition: team.Definition,
		TabID:      team.TabID,
		Known:      team.Known,
		Verdict:    status.Judge(team.Rows).Text(),
	}
	for _, row := range team.Rows {
		out.Rows = append(out.Rows, liveRow{
			Name:     row.Name,
			State:    stateWord(row),
			Text:     stateText(row),
			Sentence: sentence(row),
			PaneID:   row.PaneID,
			Worker:   row.Worker,
		})
	}
	return out
}

// stateText is what a row's pill writes: the state Herdr reports, or the word
// hergy status prints for a name with no live agent.
func stateText(row status.Row) string {
	if stateWord(row) == "gone" {
		return i18n.T("status.not_live")
	}
	return row.State
}

// htmx reports whether HTMX made this request. HTMX sets the header itself, so
// a browser that asks for the same path plainly gets the whole page.
func htmx(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }
