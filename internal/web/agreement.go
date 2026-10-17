package web

import (
	"errors"
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
)

// agreement is the shared agreement every team of the place reads (設計 3.6):
// its rules, who reads them, and, where no layer holds one, the way to make one
// (W8).
func (s *Server) agreement(_ *http.Request) (view, error) {
	return s.rulesPage(s.sharedPlace(), nil, saved{}, "", false)
}

// agreementRule is one rule of the shared agreement, open.
func (s *Server) agreementRule(r *http.Request) (view, error) {
	return s.openRule(r, s.sharedPlace())
}

// agreementNew is the card the shared agreement is adding a rule with.
func (s *Server) agreementNew(r *http.Request) (view, error) {
	return s.addRule(r, s.sharedPlace())
}

// saveAgreementRule writes one rule of the shared agreement.
func (s *Server) saveAgreementRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.sharedPlace(), false)
}

// addAgreementRule adds a rule to the end of one section of the shared agreement.
func (s *Server) addAgreementRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.sharedPlace(), true)
}

// writeShared is the write of the shared agreement itself: the second press of
// 消す takes a rule out, and 作る writes the agreement where it is not there yet
// (W8).
func (s *Server) writeShared(r *http.Request) (view, error) {
	if r.PostFormValue("make") != "" {
		place := s.madeSharedPlace()
		if place.Doc.Path == "" {
			return view{}, errors.New("web: the catalog does not say where a new document goes")
		}
		return s.writeDocument(r, place)
	}
	return s.writeDocument(r, s.sharedPlace())
}

// agreementPage draws the shared agreement: its sections of cards, who reads it,
// the card that is open, and, after a write, the saved line (3.6). A rule the
// document does not hold is a 404 (N2); a place with no shared agreement keeps
// its page and offers to make one (W8).
func (s *Server) agreementPage(place rulesPlace, open *ruleOpen, line saved, notice string, reload bool) (view, error) {
	data := agreementData{
		Saved:        line.Line,
		Handover:     line.Handover,
		Notice:       notice,
		NoticeReload: reload,
	}
	doc, ok := s.cat.SharedAgreement()
	if !ok {
		data.Missing = true
		data.Make = s.makeForm(catalog.NewAgreement, "", "/agreement")
		if open != nil && !open.Add {
			data.MissingOne = tf("web.missing.shared_rule", open.ID)
			return s.agreementView(data, http.StatusNotFound)
		}
		return s.agreementView(data, 0)
	}
	place.Doc = doc
	data.Path = s.cat.Display(doc.Path)
	data.Readers = readerLines(s.cat.Readers(doc.Path))
	parsed, err := s.ruleDoc(doc)
	if err != nil {
		data.Broken = s.breakLine(doc.Path, err)
		return s.agreementView(data, 0)
	}
	warnings := s.ruleTexts(place)
	data.Sections = s.sectionsOf(parsed, agreementRulePath, place.addURL, warnings)
	if open != nil {
		data.Panel = s.ruleCard(place, *open, notice, reload, line.Line != "", warnings)
		data.PanelOpen = true
		data.Notice = ""
		if data.Panel.Missing {
			return s.agreementView(data, http.StatusNotFound)
		}
	}
	return s.agreementView(data, 0)
}

// agreementView draws the shared agreement's page: the trail names the open card
// when there is one (N3).
func (s *Server) agreementView(data agreementData, status int) (view, error) {
	trail := []crumb{{Name: t("web.band.definitions"), Href: "/definitions"}, {Name: t("web.common.title"), Href: "/agreement"}}
	if data.PanelOpen && data.Panel.Crumb != "" {
		trail = append(trail, crumb{Name: data.Panel.Crumb})
	} else {
		trail[len(trail)-1].Href = ""
	}
	data.Crumb = trail
	return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail, status: status}, "agreement.html", data)
}

// readerLines is who reads one document, one line per definition with how many
// members ("hergy-dev の 2 人").
func readerLines(refs []catalog.MemberRef) []string {
	counts := map[string]int{}
	var order []string
	for _, ref := range refs {
		if counts[ref.Definition] == 0 {
			order = append(order, ref.Definition)
		}
		counts[ref.Definition]++
	}
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, tf("web.agreement.readers", name, counts[name]))
	}
	return out
}

// agreementData is what the shared agreement page is given.
type agreementData struct {
	Crumb        []crumb
	Path         string
	Readers      []string
	Sections     []ruleSection
	Missing      bool
	MissingOne   string
	Broken       string
	Make         *makeForm
	Notice       string
	NoticeReload bool
	Saved        string
	Handover     handover
	Panel        panel
	PanelOpen    bool
}
