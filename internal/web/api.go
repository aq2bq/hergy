package web

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aq2bq/hergy/internal/definition"
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
	Part        string          `json:"part"`
	Original    string          `json:"original"`
	Doc         json.RawMessage `json:"doc"`
	Text        string          `json:"text"`
	Hash        string          `json:"hash"`
	RenameUsers bool            `json:"rename_users"`
	Changes     json.RawMessage `json:"changes"`
	Base        *apiBase        `json:"base"`
	// New says the name is one that is not there yet: the starter team's own
	// files are the text its draft stands on (spec-teams B2, C8).
	New bool `json:"new"`
	// AgreementHash is the second file's hash, for a kind whose one name is a
	// folder of two documents: it rides a DELETE in the query beside Hash
	// (spec-teams B6).
	AgreementHash string `json:"agreement_hash"`
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

// apiList is the answer of GET /api/{kind} (spec-boundary 5): one item per name
// the kind holds, the hashes of the files behind them, and, for a kind whose
// files hold many names, the files themselves — the reading the YAML tab draws.
type apiList struct {
	Items  []apiItem         `json:"items"`
	Hashes map[string]string `json:"hashes"`
	Files  []apiFile         `json:"files,omitempty"`
}

// apiItem is one name of a kind's list: where it stands, which place is in
// effect, the columns the list shows, and what is wrong with it.
type apiItem struct {
	Name      string              `json:"name"`
	Places    map[string]apiPlace `json:"places"`
	Effective string              `json:"effective"`
	Hidden    []string            `json:"hidden"`
	Summary   any                 `json:"summary"`
	Problems  apiCounts           `json:"problems"`
}

// apiCounts is how many errors and warnings one row carries.
type apiCounts struct {
	Err  int `json:"err"`
	Warn int `json:"warn"`
}

// apiPlace is one place's reading of one name: the document in the kind's own
// shape, the text of the file it stands in, its hash, where it stands, and the
// few facts only a reading can know (who names it, whether it was just written).
type apiPlace struct {
	Doc       any       `json:"doc"`
	Text      string    `json:"text"`
	Hash      string    `json:"hash"`
	Path      string    `json:"path"`
	Readers   int       `json:"readers,omitempty"`
	SavedLine bool      `json:"savedLine,omitempty"`
	Problems  apiCounts `json:"problems"`
}

// apiFile is one file of a kind whose files hold many names: what the YAML tab
// draws, with the problems of the whole text. Name, Kind and Hidden belong to a
// kind whose one name holds more than one file (a team definition's team.yaml
// and agreement.yaml): the name the file belongs to, which of the two it is,
// and whether another place's file of the same name hides it.
type apiFile struct {
	Place    string       `json:"place"`
	Label    string       `json:"label"`
	Path     string       `json:"path"`
	Text     string       `json:"text"`
	Hash     string       `json:"hash"`
	Name     string       `json:"name,omitempty"`
	Kind     string       `json:"kind,omitempty"`
	Hidden   bool         `json:"hidden,omitempty"`
	Problems []apiProblem `json:"problems"`
}

// apiProblem is one thing a check found (spec-boundary 5): how bad it is, where
// it stands (a doc field's path, or "line" for a text), and the sentence the
// server made. The field's own name, the line it sits on and the rule it belongs
// to travel with it, so a screen puts its mark where it looks without reading
// the text again.
type apiProblem struct {
	Sev        string `json:"sev"`
	At         string `json:"at"`
	Field      string `json:"field,omitempty"`
	Text       string `json:"text"`
	Line       int    `json:"line,omitempty"`
	Section    int    `json:"section"`
	Rule       int    `json:"rule"`
	Definition string `json:"definition,omitempty"`
	// Role is the role whose document a combination finding names, and RuleID
	// the rule in it, so the screen can open /roles?open=<role>&rule=<id>
	// (d-8bbf). Both are empty when the finding names no role's rule.
	Role   string `json:"role,omitempty"`
	RuleID string `json:"rule_id,omitempty"`
}

// apiCheck is the answer of POST /api/{kind}/check (spec-boundary 5): what is
// wrong, and, where the kind has them, the Markdown a member reads, the effect
// of the change and the people that read it. A kind that has none of the latter
// leaves them out.
type apiCheck struct {
	Problems []apiProblem `json:"problems"`
	Preview  any          `json:"preview,omitempty"`
	Effect   any          `json:"effect,omitempty"`
	Readers  any          `json:"readers,omitempty"`
	// Limit is the provisional size budget the check holds this unit to, for a
	// screen that shows a document's size against it (d-8bbf). A kind whose
	// screen shows no budget leaves it out.
	Limit int `json:"limit,omitempty"`
	// Size is the unit's own size in characters, counted the way d-00a9 says:
	// the part of a guide that goes into the prompt. It is 0 when the kind has
	// no size to show.
	Size int `json:"size,omitempty"`
	// Writes is what a save of this draft would write, for a kind whose one
	// name is a folder rather than one file (spec-teams B2). A kind that writes
	// one file leaves it out.
	Writes []string `json:"writes,omitempty"`
	// Members is each member's prompt as the pieces it was built from, for a
	// team's check answer (r-4cf9).
	Members []apiMemberPrompt `json:"members,omitempty"`
	// Prompt is one prompt's pieces, for a kind that answers a single prompt
	// (the worker preview).
	Prompt []definition.PromptSegment `json:"prompt,omitempty"`
}

// apiSaved is the answer of PUT /api/{kind}/{name}: the one document as GET
// /api/{kind}/{name} reads it now, and the whole list below it, because the
// counts move with the save (spec-boundary 5).
type apiSaved struct {
	Doc       any               `json:"doc"`
	Text      string            `json:"text"`
	Hash      string            `json:"hash"`
	Path      string            `json:"path"`
	Effective string            `json:"effective,omitempty"`
	Hidden    []string          `json:"hidden,omitempty"`
	Items     []apiItem         `json:"items,omitempty"`
	Hashes    map[string]string `json:"hashes,omitempty"`
	Files     []apiFile         `json:"files,omitempty"`
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
	// Delete is DELETE /api/{kind}/{name}?place=&hash=&agreement_hash=: take
	// one document away.
	Delete(name string, in apiIn) (any, *apiErr)
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
	case apiActivityKind:
		return apiActivity{s: s}, true
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
	value, refusal := kind.Delete(r.PathValue("name"), apiIn{
		Place:         r.URL.Query().Get("place"),
		Hash:          r.URL.Query().Get("hash"),
		AgreementHash: r.URL.Query().Get("agreement_hash"),
	})
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
