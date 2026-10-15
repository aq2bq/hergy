package teamview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// The panel these screens open in the right column: one member, with its
// settings and the prompt hergy hands it. It answers a fragment, which is what
// an HTMX swap puts in the panel. Nothing here writes: these screens are
// read-only, and a rule file or a rule of the figure opens the rules screen
// (internal/web/rulesview), which is where a person changes them.

// memberPanel is the panel for one member.
func (s *definitionScreen) memberPanel(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	query := r.URL.Query()
	def, err := s.port.Definition(layer, query.Get("name"))
	if err != nil {
		return web.View{}, err
	}
	name := query.Get("member")
	member, ok := def.Member(name)
	if !ok {
		return s.missing(i18n.Tf("web.teamview.panel.no_member", name))
	}
	data := memberPanelData{Member: member}
	states, live := s.liveStates(layer, def.Name)
	data.Live = live != nil
	if row, found := states[herdrName(live, def.Name, name)]; found {
		data.Row = row
	}
	if data.Live {
		// A member Herdr lists no agent for is "gone": hergy status shows the
		// same row, so the panel says the same thing.
		data.State = stateWord(data.Row)
		data.Text = stateText(data.Row)
		data.Meaning = stateMeaning(data.State)
	}
	return s.bare("member-panel.html", data)
}

// memberPanelData is what the member panel is drawn from.
type memberPanelData struct {
	Member  Member
	Live    bool // Herdr reports the definition running
	Row     status.Row
	State   string
	Text    string // the state as the pill writes it
	Meaning string
}
