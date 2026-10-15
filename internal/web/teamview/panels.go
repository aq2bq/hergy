package teamview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// These are the panels the screens open in the right panel: one member (its
// settings and the prompt hergy hands it), one rule file (the rules in it), and
// one rule (its text). They answer fragments, which is what an HTMX swap puts
// in the panel. Nothing here writes: the two screens are read-only.

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
	data := memberPanelData{Layer: layer, Def: def, Member: member}
	states, live := s.liveStates(layer, def.Name)
	data.Live = live != nil
	if row, found := states[herdrName(live, def.Name, name)]; found {
		data.Row, data.HasRow = row, true
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

// docPanel is the panel for one rule file of a definition.
func (s *definitionScreen) docPanel(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	query := r.URL.Query()
	def, err := s.port.Definition(layer, query.Get("name"))
	if err != nil {
		return web.View{}, err
	}
	doc, ok := def.Doc(query.Get("key"))
	if !ok {
		return s.missing(i18n.Tf("web.teamview.panel.no_doc", query.Get("key")))
	}
	return s.bare("doc-panel.html", docPanelData{Layer: layer, Def: def, Doc: doc})
}

// rulePanel is the panel for one rule: what the figure's arrows point at.
func (s *definitionScreen) rulePanel(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	query := r.URL.Query()
	def, err := s.port.Definition(layer, query.Get("name"))
	if err != nil {
		return web.View{}, err
	}
	doc, ok := def.Doc(query.Get("doc"))
	if !ok {
		return s.missing(i18n.Tf("web.teamview.panel.no_doc", query.Get("doc")))
	}
	rule, ok := doc.Rule(query.Get("rule"))
	if !ok {
		return s.missing(i18n.Tf("web.teamview.panel.no_rule", query.Get("rule")))
	}
	return s.bare("rule-panel.html", rulePanelData{Layer: layer, Def: def, Doc: doc, Rule: rule})
}

// memberPanelData is what the member panel is drawn from.
type memberPanelData struct {
	Layer   string
	Def     Definition
	Member  Member
	Live    bool // Herdr reports the definition running
	HasRow  bool // Herdr has an agent under this member's name
	Row     status.Row
	State   string
	Text    string // the state as the pill writes it
	Meaning string
}

// docPanelData is what the rule-file panel is drawn from.
type docPanelData struct {
	Layer string
	Def   Definition
	Doc   Doc
}

// rulePanelData is what the rule panel is drawn from.
type rulePanelData struct {
	Layer string
	Def   Definition
	Doc   Doc
	Rule  Rule
}
