package web

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
)

// This file is the one boundary the screens read and write through
// (spec-boundary 5, d-89b7): every kind of thing a screen shows is handled by
// one Go type with the same five methods, and one set of handlers serves them
// all under /api/. The server hands out JSON only, and the screen draws from it.
//
// The promises the write side keeps are the ones the old per-screen handlers
// kept: an Origin that comes with a write must be this server's own, the body is
// capped, and the token of this run has to travel on the request. The token
// travels in the X-Hergy-Token header, so one shape covers POST, PUT and DELETE
// alike (a DELETE carries no body).

// apiIn is one check's or one write's payload. The fields a kind does not read
// stay empty; each kind is the one that says which fields it means.
type apiIn struct {
	Place       string          `json:"place"`
	Name        string          `json:"name"`
	Original    string          `json:"original"`
	Doc         json.RawMessage `json:"doc"`
	Text        string          `json:"text"`
	Hash        string          `json:"hash"`
	RenameUsers bool            `json:"rename_users"`
	Open        []string        `json:"open"`
	Changes     json.RawMessage `json:"changes"`
	Base        *apiBase        `json:"base"`
}

// apiBase is the file a duplicate or a copy stands on (spec-roles 1.2.9).
type apiBase struct {
	Place string `json:"place"`
	Name  string `json:"name"`
}

// apiErr is a refusal: the status, the sentence a person reads, and, when the
// kind knows it, the field the sentence belongs to (422) or the hash of the file
// as it is now (409).
type apiErr struct {
	status int
	Text   string `json:"text"`
	At     string `json:"at,omitempty"`
	Hash   string `json:"hash,omitempty"`
}

// refuse is a refusal with the sentence alone.
func refuse(status int, text string) *apiErr { return &apiErr{status: status, Text: text} }

// refuseAt is a refusal that names the field the sentence belongs to.
func refuseAt(status int, at, text string) *apiErr {
	return &apiErr{status: status, At: at, Text: text}
}

// apiKind is one kind of thing a screen reads and writes: profiles, roles, team
// definitions (and, in the request after this one, the team's agreement). One
// type holds each kind's own rules; the handlers below know none of them
// (spec-boundary 8).
type apiKind interface {
	// List is GET /api/{kind}: the items and the hashes of the files behind
	// them.
	List() (any, *apiErr)
	// Get is GET /api/{kind}/{name}?place=: one item's document.
	Get(name, place string) (any, *apiErr)
	// Check is POST /api/{kind}/check: what is wrong with a draft or a text.
	Check(in apiIn) (any, *apiErr)
	// Save is PUT /api/{kind}/{name}: write one document and answer with it and
	// the list as it now reads.
	Save(name string, in apiIn) (any, *apiErr)
	// Delete is DELETE /api/{kind}/{name}?place=&hash=: take one document away.
	Delete(name, place, hash string) (any, *apiErr)
}

// apiKindOf is the kind a path names, if a screen has one for it.
func (s *Server) apiKindOf(name string) (apiKind, bool) {
	switch name {
	case apiProfileKind:
		return apiProfile{s: s}, true
	case apiRoleKind:
		return apiRole{s: s}, true
	case apiTeamKind:
		return apiTeam{s: s}, true
	}
	return nil, false
}

// The names of the kinds, the words of the /api/ URLs. They are the wire's, not
// a person's: a screen reads them, a person never does.
const (
	apiProfileKind = "profile"
	apiRoleKind    = "role"
	apiTeamKind    = "team"
)

// apiList answers GET /api/{kind}.
func (s *Server) apiList(w http.ResponseWriter, r *http.Request) {
	kind, ok := s.apiKindOf(r.PathValue("kind"))
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	value, refusal := kind.List()
	apiAnswer(w, value, refusal)
}

// apiGet answers GET /api/{kind}/{name}.
func (s *Server) apiGet(w http.ResponseWriter, r *http.Request) {
	kind, ok := s.apiKindOf(r.PathValue("kind"))
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	value, refusal := kind.Get(r.PathValue("name"), r.URL.Query().Get("place"))
	apiAnswer(w, value, refusal)
}

// apiCheck answers POST /api/{kind}/check.
func (s *Server) apiCheck(w http.ResponseWriter, r *http.Request) {
	kind, ok := s.apiKindOf(r.PathValue("kind"))
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	in, ok := s.apiRead(w, r)
	if !ok {
		return
	}
	value, refusal := kind.Check(in)
	apiAnswer(w, value, refusal)
}

// apiSave answers PUT /api/{kind}/{name}.
func (s *Server) apiSave(w http.ResponseWriter, r *http.Request) {
	kind, ok := s.apiKindOf(r.PathValue("kind"))
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	in, ok := s.apiRead(w, r)
	if !ok {
		return
	}
	value, refusal := kind.Save(r.PathValue("name"), in)
	apiAnswer(w, value, refusal)
}

// apiDelete answers DELETE /api/{kind}/{name}. A delete carries no body, so its
// place and hash travel in the query and its token in the header.
func (s *Server) apiDelete(w http.ResponseWriter, r *http.Request) {
	kind, ok := s.apiKindOf(r.PathValue("kind"))
	if !ok {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	if !s.apiGuard(w, r) {
		return
	}
	value, refusal := kind.Delete(r.PathValue("name"), r.URL.Query().Get("place"), r.URL.Query().Get("hash"))
	apiAnswer(w, value, refusal)
}

// apiRead is the guard and the payload every write but a delete carries.
func (s *Server) apiRead(w http.ResponseWriter, r *http.Request) (apiIn, bool) {
	if !s.apiGuard(w, r) {
		return apiIn{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWrite)
	var in apiIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, t("web.error.token"), http.StatusUnprocessableEntity)
		return apiIn{}, false
	}
	return in, true
}

// apiGuard holds the two promises in front of a write: the Origin, when one
// travels, is this server's own, and the token of this run is on the request
// (spec-boundary 5, 設計 6 節).
func (s *Server) apiGuard(w http.ResponseWriter, r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		http.Error(w, t("web.error.origin"), http.StatusForbidden)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Hergy-Token")), []byte(s.token)) != 1 {
		http.Error(w, t("web.error.token"), http.StatusForbidden)
		return false
	}
	return true
}

// apiAnswer writes one answer: the value as JSON, or the refusal with the status
// and the sentence it carries.
func apiAnswer(w http.ResponseWriter, value any, refusal *apiErr) {
	if refusal != nil {
		writeJSON(w, refusal.status, refusal)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

// writeJSON writes one JSON body with its status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, i18n.Tf("web.error.internal", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
