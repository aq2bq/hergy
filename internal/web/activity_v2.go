package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
)

// This file draws the activity screen (r-8a67, spec-activity 2). The page is a
// shell: the client script draws every row from GET /api/activity, so a
// re-reading changes what is on the screen without the page being built again.
// The reading itself is the app's, handed in as an Activity.

// a2PageData is the activity page's shell: the band, and the one sentence the
// page needs that is not in the answer.
type a2PageData struct {
	Nav navData
	// Words is the island of the sentences the screen says when there is
	// nothing to show. Every other word comes with the answer, because the
	// server writes it (spec-activity 3, V1).
	Words template.JS
}

// a2Page draws the activity screen: the 一覧 tab, and an empty list the script
// fills. Nothing about a team is written into the page itself, so what a person
// reads after a re-reading is the answer of that moment and not what the page
// was built with (spec-activity 2.6).
func (s *Server) a2Page(w http.ResponseWriter, r *http.Request) {
	words, err := json.Marshal(map[string]string{"none": i18n.T("status.none")})
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data := a2PageData{Nav: s.navOf("activity"), Words: template.JS(words)} //nolint:gosec // words is json.Marshal output, which escapes <, > and &, so it cannot close the script element that carries it
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "a2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
