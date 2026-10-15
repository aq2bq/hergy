package teamview

import (
	"errors"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// The panel these screens open in the right column: one member, with its
// settings and the prompt hergy hands it. It answers the whole drawer element,
// which is what an HTMX swap puts in the right column, and what the definition
// page carries when it is opened with a member named. Nothing here writes: these
// screens are read-only, and a rule file or a rule of the figure opens the rules
// screen (internal/web/rulesview), which is where a person changes them.

// errNoMember says the definition has no member of that name: the panel answers
// with the reason instead of a member.
var errNoMember = errors.New("teamview: no such member")

// memberPanel is the panel for one member, as a fragment: what an HTMX swap
// puts in the right column when a link or a form asks for it.
func (s *definitionScreen) memberPanel(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	query := r.URL.Query()
	body, err := s.panelBody(layer, query.Get("name"), query.Get("member"))
	if err != nil {
		return web.View{}, err
	}
	return web.View{Body: web.HTML(body), Bare: true}, nil
}

// closePanel answers the empty drawer: closing the panel swaps the same element
// back with hidden, so the frame makes room and the person stays where they are.
func (s *definitionScreen) closePanel(_ *http.Request) (web.View, error) {
	return s.bare("panel-closed.html", nil)
}

// panelBody renders the member's panel as the whole drawer element. A member
// the definition does not have comes back as the same element with the reason
// in it, so a stale link shows why rather than nothing.
func (s *definitionScreen) panelBody(layer, name, member string) (string, error) {
	data, err := s.memberPanelOf(layer, name, member)
	if errors.Is(err, errNoMember) {
		return s.render("problem.html", panelProblem{
			Layer:  layer,
			Reason: i18n.Tf("web.teamview.panel.no_member", member),
		})
	}
	if err != nil {
		return "", err
	}
	return s.render("member-panel.html", data)
}

// memberPanelOf reads one member's panel data: the settings, what Herdr says of
// the member right now, and the rule files hergy joins into its prompt.
func (s *definitionScreen) memberPanelOf(layer, name, member string) (memberPanelData, error) {
	def, err := s.port.Definition(layer, name)
	if err != nil {
		return memberPanelData{}, err
	}
	entry, ok := def.Member(member)
	if !ok {
		return memberPanelData{}, errNoMember
	}
	data := memberPanelData{
		Layer:  layer,
		Team:   def.Name,
		Member: entry,
		Docs:   memberDocs(def, entry),
	}
	states, live := s.liveStates(layer, def.Name)
	data.Live = live != nil
	if row, found := states[herdrName(live, def.Name, member)]; found {
		data.Row = row
	}
	if data.Live {
		// A member Herdr lists no agent for is "gone": hergy status shows the
		// same row, so the panel says the same thing.
		data.State = stateWord(data.Row)
		data.Text = stateText(data.Row)
		data.Meaning = stateMeaning(data.State)
	}
	return data, nil
}

// memberDocs are the rule files one member reads, in the order hergy joins them
// into its prompt: what every member reads, then the member's own role file.
func memberDocs(def Definition, member Member) []Doc {
	out := append([]Doc{}, def.Docs...)
	if member.RoleFile != nil {
		out = append(out, *member.RoleFile)
	}
	return out
}

// memberPanelData is what the member panel is drawn from.
type memberPanelData struct {
	Layer  string
	Team   string // the team definition the member belongs to
	Member Member
	Live   bool // Herdr reports the definition running
	Row    status.Row
	State  string
	Text   string // the state as the pill writes it
	// Meaning is what that state means, beside the pill.
	Meaning string
	// Docs are the rule files the member reads, in the order hergy joins them.
	Docs []Doc
}

// panelProblem is what the drawer shows when it cannot show a member: the layer
// the request came from, and the one line saying why.
type panelProblem struct {
	Layer  string
	Reason string
}
