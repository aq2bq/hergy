package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
)

// agreement is the shared agreement every team of the project reads (設計 3.6):
// its rules, read only in this stage, and who reads them.
func (s *Server) agreement(_ *http.Request) (view, error) {
	return s.agreementPage("")
}

// agreementRule is one rule of the shared agreement, as a panel.
func (s *Server) agreementRule(r *http.Request) (view, error) {
	return s.agreementPage(r.PathValue("rule"))
}

// agreementPage draws the shared agreement, with one rule open when open names
// it. A rule the document does not hold is a 404 (N2). A place with no shared
// agreement keeps its page: the one line says there is none (段 4 adds "make
// one").
func (s *Server) agreementPage(open string) (view, error) {
	data := agreementData{}
	trail := []crumb{{Name: t("web.band.definitions"), Href: "/definitions"}, {Name: t("web.common.title"), Href: "/agreement"}}
	doc, ok := s.cat.SharedAgreement()
	if !ok {
		data.Missing = true
		if open != "" {
			data.MissingOne = tf("web.missing.shared_rule", open)
			trail[len(trail)-1].Href = ""
			return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail, status: http.StatusNotFound}, "agreement.html", data)
		}
		trail[len(trail)-1].Href = ""
		data.Crumb = trail
		return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail}, "agreement.html", data)
	}
	data.Path = s.cat.Display(doc.Path)
	data.Readers = readerLines(s.cat.Readers(doc.Path))
	parsed, err := s.ruleDoc(doc)
	if err != nil {
		data.Broken = s.breakLine(doc.Path, err)
		trail[len(trail)-1].Href = ""
		data.Crumb = trail
		return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail}, "agreement.html", data)
	}
	data.Sections = s.sectionsOf(parsed, func(id string) string { return "/agreement/" + id })
	if open != "" {
		card, found := cardOf(parsed, open)
		if !found {
			data.MissingOne = tf("web.missing.shared_rule", open)
			trail[len(trail)-1].Href = ""
			return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail, status: http.StatusNotFound}, "agreement.html", data)
		}
		data.Panel = panel{Title: tf("web.panel.rule", open), Key: "rule:" + open, Parent: "/agreement", Crumb: open, Rule: &card}
		data.PanelOpen = true
		trail = append(trail, crumb{Name: open})
	}
	data.Crumb = trail
	return s.pageBody(view{title: t("web.common.title"), band: bandDefinitions, crumb: trail}, "agreement.html", data)
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
	Crumb      []crumb
	Path       string
	Readers    []string
	Sections   []ruleSection
	Missing    bool
	MissingOne string
	Broken     string
	Panel      panel
	PanelOpen  bool
}
