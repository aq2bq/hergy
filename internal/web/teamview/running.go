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
// The page reads Herdr again every five seconds (HTMX's every 5s) while the
// box in its bar is ticked, which is how it starts; stopping that box stops the
// reading, and "herdr から読み直す" then reads once. hergy holds no state of its
// own either way: the box travels as the auto argument of this URL. A Herdr that
// is missing or unreadable is one line on this screen; the other screens keep
// working.
type runningScreen struct{ base }

// Name is what the navigation calls this screen.
func (s *runningScreen) Name() string { return i18n.T("web.teamview.running.title") }

// Routes are the page, the part of it the page reads again on its own, and the
// one write of these screens. Both reads answer the same way: the fragment HTMX
// swaps in when HTMX asked, and the whole page when a person opens the path
// themselves.
func (s *runningScreen) Routes() []web.Route {
	return []web.Route{
		{Path: "/running", Handler: s.show},
		{Path: "/running/list", Handler: s.show},
		{Path: "/running/focus", Write: true, Handler: s.focus},
	}
}

func (s *runningScreen) show(r *http.Request) (web.View, error) {
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
		Layer:  layer,
		Auto:   autoOf(r),
		Token:  web.Token(r),
		Legend: legend(),
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

// autoOf says whether the page reads Herdr again every five seconds. Reading
// again is what happens by default, so only auto=0 turns it off: a link, a
// bookmark, or the navigation entry keeps reading, and the one box that stops it
// writes the zero.
func autoOf(r *http.Request) bool { return r.URL.Query().Get("auto") != "0" }

// runningData is what the running-team page is drawn from.
type runningData struct {
	Layer  string
	Auto   bool   // read Herdr again every five seconds; off stops it
	Token  string // the token every form of this run carries
	At     string // when Herdr was read, as a person reads a clock
	Notice string // one line: Herdr could not be read, or the focus failed
	Teams  []liveTeam
	Legend []word
}

// liveTeam is one running team as the page shows it: where it came from, its
// members and workers, and the judgement over them.
type liveTeam struct {
	Name       string
	Definition string
	TabID      string
	Known      bool   // the layer has a team definition of this name
	Broken     string // why that definition cannot be read
	Rows       []liveRow
	// Blocked says somebody waits on the human: the card is outlined in the
	// danger colour and the judgement is written in it.
	Blocked bool
	// Verdict is the judgement line, and VerdictClass the colour it is written
	// in ("warn" while somebody waits, "calm" when nobody works, else empty).
	Verdict      string
	VerdictClass string
}

// liveRow is one member or worker of a running team.
type liveRow struct {
	Name     string
	State    string // the word the pill is coloured by
	Text     string // what the pill writes: the state, or "not present"
	Meaning  string // what that state means, on the pill and beside it
	Sentence string // the role, the engine, and what the terminal says
	PaneID   string
	Worker   bool
}

func liveTeamOf(team LiveTeam) liveTeam {
	verdict := status.Judge(team.Rows)
	out := liveTeam{
		Name:         team.Team,
		Definition:   team.Definition,
		TabID:        team.TabID,
		Known:        team.Known,
		Broken:       team.Broken,
		Blocked:      len(verdict.Waiting) > 0,
		Verdict:      verdict.Text(),
		VerdictClass: verdictClass(verdict),
	}
	// The judgement is empty only when somebody is working and nobody waits:
	// the line then says that, the way hergy status leaves it unsaid.
	if out.Verdict == "" {
		out.Verdict = i18n.T("web.teamview.running.working")
	}
	for _, row := range team.Rows {
		word := stateWord(row)
		out.Rows = append(out.Rows, liveRow{
			Name:     row.Name,
			State:    word,
			Text:     stateText(row),
			Meaning:  stateMeaning(word),
			Sentence: sentence(row),
			PaneID:   row.PaneID,
			Worker:   row.Worker,
		})
	}
	return out
}

// verdictClass is the colour the judgement line is written in: warn while
// somebody waits on the human, calm when nobody is working, and the plain
// colour otherwise.
func verdictClass(v status.Verdict) string {
	switch {
	case len(v.Waiting) > 0:
		return "warn"
	case v.Working == 0:
		return "calm"
	}
	return ""
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
