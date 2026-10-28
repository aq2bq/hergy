package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
)

// templates and static are embedded: hergy serve is one binary, and the pages it
// serves come from it, not from wherever it is run.
//
//go:embed templates/*.html static
var files embed.FS

const (
	// shutdownGrace is how long a stop waits for the requests in flight.
	shutdownGrace = 5 * time.Second
	// maxWrite caps the body of one write, so a request cannot fill memory.
	maxWrite = 1 << 20
)

// Server answers the screens over HTTP. Build one with New and hand it to Serve.
type Server struct {
	cat *catalog.Catalog
	// act is how the activity screen reads the running teams, or nil when the
	// server was built without one (spec-activity 6 A1).
	act Activity
	// apply is the drift between the saved definitions and the running teams,
	// or nil when the activity reading cannot say what differs.
	apply Applier
	// hq is the headquarters reading (d-73fc), or nil when the server was built
	// without one: the routes answer 503 rather than showing a made-up list.
	hq HQ
	// projects reads one listed project's layer, for a screen that names a
	// project beside a team, role or profile.
	projects CatalogProvider
	token    string
	tpl      *template.Template
	// handler is the routes with the guards in front of them.
	handler http.Handler
	// addr is this server's own address ("127.0.0.1:8787"), set when the socket
	// is open; every request is checked against it.
	addr string
}

// New builds the server for one run: the catalog every screen reads through, and
// how the activity screen reads the running teams. The templates are parsed here,
// so a broken one stops hergy serve at once instead of on the first request; the
// token is made fresh for this run, and a write without it is refused.
//
// act may be nil, which is a server that finds no team running: the screen says
// so rather than showing a made-up zero (spec-activity 2.7).
func New(cat *catalog.Catalog, act Activity) (*Server, error) {
	if cat == nil {
		return nil, errors.New("web: New needs a catalog")
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	tpl, err := template.New("").Funcs(Funcs()).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	s := &Server{cat: cat, act: act, token: token, tpl: tpl}
	if applier, ok := act.(Applier); ok {
		s.apply = applier
	}
	if hq, ok := act.(HQ); ok {
		s.hq = hq
	}
	if projects, ok := act.(CatalogProvider); ok {
		s.projects = projects
	}
	s.handler = s.guard(s.routes(static))
	if WrapForTest != nil {
		s.handler = WrapForTest(s.handler)
	}
	return s, nil
}

// WrapForTest wraps every Server's handler when a test sets it. The API
// contract tests put internal/apicontract's checker there, so each request a
// test sends to /api/ and each answer it gets is held to openapi.yaml
// (n-e3de). hergy serve never sets it.
var WrapForTest func(http.Handler) http.Handler

// activity is the reading behind the activity screen, never nil: a server built
// without one reads as a herdr with nothing in it.
func (s *Server) activity() Activity {
	if s.act == nil {
		return noActivity{}
	}
	return s.act
}

// Serve listens on 127.0.0.1:port and answers until ctx is done. ready, when
// set, is called once with the URL to open, after the socket is open and before
// the first request is answered.
func (s *Server) Serve(ctx context.Context, port int, ready func(url string)) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return i18n.Errorf("error.serve_bind", addr, err)
	}
	s.addr = listener.Addr().String()
	if ready != nil {
		ready(s.URL())
	}
	server := &http.Server{Handler: s.handler, ReadHeaderTimeout: 10 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	err = server.Shutdown(shutdownCtx)
	<-done
	return err
}

// URL is the address to open in a browser. It is set once the socket is open.
func (s *Server) URL() string {
	if s.addr == "" {
		return ""
	}
	return "http://" + s.addr
}

// routes is the whole table of the UI (spec-nav.md 2): the two screens, the
// places their old pages used to stand, and the static files. Every other URL,
// including the ones the old WebUI answered, is not found — and is not moved,
// because a browser remembers a 301 and would keep away once a screen of that
// name is made (spec-nav.md 2「消すもの」).
func (s *Server) routes(static fs.FS) http.Handler {
	mux := http.NewServeMux()

	// 開いて最初の画面 (spec-nav.md 1.3): / is where a person should start. With
	// a team of a listed project running (without projects:, a team of this
	// project) it is the activity screen — what the human is being asked for is
	// why they came (spec-activity 2.1, S6) — and with none it is the team
	// definitions. A 302, not a 301: which of the two it is
	// changes as teams come and go.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if s.activity().List().Running > 0 {
			http.Redirect(w, r, "/activity", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/teams", http.StatusFound)
	})

	// アクティビティ (r-8a67): the running teams, one row each, drawn from GET
	// /api/activity. The old /activity/<team> URLs are gone; the opened row is
	// the page's own state (spec-activity 2.3).
	mux.HandleFunc("GET /activity", s.a2Page)

	// ロール (r-778c): the three tabs, their checks and their writes. The old
	// page-per-role URLs move to the opened row (設計 6 節).
	mux.HandleFunc("GET /roles", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "roles") })
	mux.HandleFunc("GET /roles/assign", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "assign") })
	mux.HandleFunc("GET /roles/yaml", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "yaml") })
	mux.HandleFunc("GET /roles/{role}", s.r2OldRole)
	mux.HandleFunc("GET /roles/{role}/new", s.r2OldRoleNew)
	mux.HandleFunc("GET /roles/{role}/{rule}", s.r2OldRoleRule)

	// チーム定義 (r-0f8c): the list and the YAML tab. The figure's and the
	// agreement's checks and every write go through /api/team (spec-boundary 5),
	// so this screen owns only its two pages. The old /teams that moved to
	// /definitions is gone: /teams is this screen now.
	mux.HandleFunc("GET /teams", func(w http.ResponseWriter, r *http.Request) { s.t2Page(w, r, "list") })
	mux.HandleFunc("GET /teams/yaml", func(w http.ResponseWriter, r *http.Request) { s.t2Page(w, r, "yaml") })

	// プロファイル (r-c683): the three tabs.
	mux.HandleFunc("GET /profiles", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "profiles") })
	mux.HandleFunc("GET /profiles/assign", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "assign") })
	mux.HandleFunc("GET /profiles/yaml", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "yaml") })
	mux.HandleFunc("GET /profiles/{name}", s.oldProfile)

	// 受け口 (spec-boundary 5, r-2cc2): every kind of thing the screens read and
	// write answers under /api/, in one shape, through one set of handlers. The
	// old per-screen URLs (/profiles/check ほか, /roles/check ほか) are gone.
	// The band and the sheet a person opens after a change (r-a8fa). They are
	// own paths, not a kind: the count is read on every page and the send is one
	// write over several running teams.
	// 反映 (d-36ef): the drift of every running team against its saved
	// definition, the way hergy would put each member right, and the band's
	// count. The sheet reads the drift and the apply does the work.
	mux.HandleFunc("GET /api/apply", s.apiApplyList)
	mux.HandleFunc("GET /api/apply/count", s.apiApplyCount)
	mux.HandleFunc("POST /api/apply", s.apiApply)

	// 本部 (d-73fc, r-b7e1): the projects a person lists, the lists of every
	// project and the global layer, the workspaces, and starting and stopping a
	// team from the screens, and raising a member of a running one (r-5d1a).
	mux.HandleFunc("GET /api/hq/projects", s.hqProjects)
	mux.HandleFunc("POST /api/hq/projects", s.hqAddProject)
	mux.HandleFunc("DELETE /api/hq/projects", s.hqRemoveProject)
	mux.HandleFunc("GET /api/hq/teams", s.hqGroups(apiTeamKind))
	mux.HandleFunc("GET /api/hq/roles", s.hqGroups(apiRoleKind))
	mux.HandleFunc("GET /api/hq/profiles", s.hqGroups(apiProfileKind))
	mux.HandleFunc("GET /api/hq/running", s.hqRunning)
	mux.HandleFunc("GET /api/hq/workspaces", s.hqWorkspaces)
	mux.HandleFunc("GET /api/hq/launch/options", s.hqLaunchOptions)
	mux.HandleFunc("POST /api/hq/launch", s.hqLaunch)
	mux.HandleFunc("GET /api/hq/launches/{id}", s.hqLaunchState)
	mux.HandleFunc("DELETE /api/hq/launches/{id}", s.hqLaunchCancel)
	mux.HandleFunc("POST /api/hq/stop", s.hqStop)

	mux.HandleFunc("GET /api/{kind}", s.apiList)
	mux.HandleFunc("GET /api/{kind}/{name}", s.apiGet)
	mux.HandleFunc("POST /api/{kind}/check", s.apiCheck)
	mux.HandleFunc("PUT /api/{kind}/{name}", s.apiSave)
	mux.HandleFunc("DELETE /api/{kind}/{name}", s.apiDelete)

	// アクティビティは読むだけ (spec-activity 3): its three writes are refused with 405
	// here, in front of the write guard, because a caller that asks whether the
	// URL takes a write should be told it does not rather than that it is
	// missing a token. The kind's own three methods are the same answer for a
	// caller that goes around these (api_activity.go).
	mux.HandleFunc("POST /api/activity/check", s.apiActivityReadOnly)
	mux.HandleFunc("PUT /api/activity/{team}", s.apiActivityReadOnly)
	mux.HandleFunc("DELETE /api/activity/{team}", s.apiActivityReadOnly)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("/", s.notFound)
	return mux
}

// guard holds the promises that stand in front of every request: the Host has to
// be this server's own name, and every answer carries the policy that keeps the
// page to this server's own files.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostOK(r) {
			http.Error(w, t("web.error.host"), http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// hostOK reports whether a request was addressed to this server.
func (s *Server) hostOK(r *http.Request) bool {
	_, bound, err := net.SplitHostPort(s.addr)
	if err != nil {
		return false
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, port = r.Host, ""
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
	default:
		return false
	}
	if port == "" {
		return bound == "80"
	}
	return port == bound
}

// problem is what a failed screen shows: the band, and the reason on it
// (spec-nav.md 1.6).
func (s *Server) problem(w http.ResponseWriter, _ *http.Request, err error) {
	s.navPage(w, http.StatusInternalServerError, tf("web.error.internal", err))
}

// notFound answers a URL no screen claims: the band and the sentence for a read,
// and the sentence alone for a write, the way the old WebUI answered one
// (spec-nav.md 1.6).
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	s.navPage(w, http.StatusNotFound, t("web.error.not_found"))
}

// newToken makes the secret of one run: 32 random bytes in hex.
func newToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
