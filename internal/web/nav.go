package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
)

// navData is the band every screen hangs at the top (spec-nav.md 1.1): where
// the person is, which project this is, and the counts. Current is the screen
// the page is ("activity", "profiles", "roles" or "teams"), or empty for a page
// that is no screen, which marks none of them.
type navData struct {
	Current  string
	Project  string
	Teams    int
	Profiles int
	Roles    int
	// Activity is how many teams of this project are running, and Waiting how
	// many of their members are waiting for the human (spec-activity 2.1).
	Activity int
	Waiting  int
	// Counts says whether the band may show the activity's two numbers: herdr
	// was read, and there is at least one team running. With no team running the
	// band is 活動 alone, and with a herdr that could not be read it writes no
	// number either, rather than a zero it does not know (spec-activity 2.7).
	Counts bool
	Key    string
}

// navOf gathers the band for one screen. The pages that carry the band take it
// from here, so they cannot count the same thing differently.
func (s *Server) navOf(current string) navData {
	project, shared, _ := s.p2Read()
	list := s.activity().List()
	return navData{
		Current:  current,
		Project:  filepath.Base(s.cat.Base()),
		Teams:    len(s.cat.Definitions()),
		Profiles: len(p2Names(project, shared)),
		Roles:    s.navRoles(),
		Activity: list.Running,
		Waiting:  list.Waiting,
		Counts:   list.Herdr.OK && list.Running > 0,
		Key:      navKey(s.cat.Base()),
	}
}

// navRoles is the number of roles the band shows: the names a document stands
// behind, in either place. It is the same number the role screen's own count
// (savedRoles().length) writes back into the band when a role is added or
// removed, so the number does not jump when the page's script draws. A name
// only a team definition writes has no document yet and is not counted
// (spec-nav.md 1.0).
func (s *Server) navRoles() int {
	n := 0
	for _, entry := range s.cat.Roles() {
		if entry.HasDocument {
			n++
		}
	}
	return n
}

// navKey is the name this project's band remembers things under, so two
// projects served from one machine do not share the place a person left open or
// the changes they had not saved: the first twelve hex digits of the SHA-256 of
// the project's root (spec-nav.md 1.0).
func navKey(base string) string {
	root, err := filepath.Abs(base)
	if err != nil {
		root = base
	}
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:12]
}

// navPage answers a page that is no screen: the band, and one line. status is
// 404 for a URL nothing claims and 500 for a screen that could not be drawn
// (spec-nav.md 1.6). The band is the one on the 404 page too, so a person can
// walk to the screens from it.
func (s *Server) navPage(w http.ResponseWriter, status int, text string) {
	data := struct {
		Nav  navData
		Text string
	}{Nav: s.navOf(""), Text: text}
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "nav_page", data); err != nil {
		http.Error(w, text, status)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
