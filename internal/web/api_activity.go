package web

import "net/http"

// The activity kind (spec-activity 3, r-8a67): the running teams, read from
// herdr and the launch record. It is the one kind of the boundary that is read
// only, so the three writes are refused with 405 at the route, before the write
// guard, and a caller that asks what the URL takes is answered honestly rather
// than with a complaint about a token it could not have.
//
// The reading itself is not here: internal/app reads herdr, gy and the launch
// record and hands the answer over (6 A1). This kind only puts it in the
// boundary's shape.

// apiActivityKind is the wire's name for the activity in /api/ URLs.
const apiActivityKind = "activity"

// apiActivity is the activity kind.
type apiActivity struct{ s *Server }

// List is GET /api/activity: the running teams and the band's numbers.
func (a apiActivity) List() (any, *apiErr) {
	return a.s.activity().List(), nil
}

// Get is GET /api/activity/{team}: one team's opened part. A team that is not
// running is not there (404).
func (a apiActivity) Get(name, _ string) (any, *apiErr) {
	doc, ok := a.s.activity().One(name)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	return struct {
		Doc ActivityDoc `json:"doc"`
	}{Doc: doc}, nil
}

// Check, Save and Delete are the writes this kind has none of. The routes above
// the guard answer the 405, so these are only reached by a caller that goes
// around them.
func (apiActivity) Check(apiIn) (any, *apiErr) {
	return nil, refuse(http.StatusMethodNotAllowed, t("web.error.read_only"))
}

func (apiActivity) Save(string, apiIn) (any, *apiErr) {
	return nil, refuse(http.StatusMethodNotAllowed, t("web.error.read_only"))
}

func (apiActivity) Delete(string, apiIn) (any, *apiErr) {
	return nil, refuse(http.StatusMethodNotAllowed, t("web.error.read_only"))
}

// apiActivityReadOnly answers a write on the activity kind: 405, whatever the
// request carries. The activity is a screen someone reads.
func (s *Server) apiActivityReadOnly(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, t("web.error.read_only"), http.StatusMethodNotAllowed)
}

// apiActivityFocus moves to a currently running member, with the ordinary write guard.
func (s *Server) apiActivityFocus(w http.ResponseWriter, r *http.Request) {
	if !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[struct {
		Team   string `json:"team"`
		Member string `json:"member"`
	}](w, r)
	if !ok {
		return
	}
	focuser, ok := s.activity().(interface{ FocusMember(string, string) error })
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	answerHQ(w, map[string]bool{"ok": true}, focuser.FocusMember(in.Team, in.Member))
}
