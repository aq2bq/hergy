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
	cat   *catalog.Catalog
	token string
	tpl   *template.Template
	// handler is the routes with the guards in front of them.
	handler http.Handler
	// addr is this server's own address ("127.0.0.1:8787"), set when the socket
	// is open; every request is checked against it.
	addr string
}

// New builds the server for one run: the catalog every screen reads through. The
// templates are parsed here, so a broken one stops hergy serve at once instead of
// on the first request; the token is made fresh for this run, and a write
// without it is refused.
func New(cat *catalog.Catalog) (*Server, error) {
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
	s := &Server{cat: cat, token: token, tpl: tpl}
	s.handler = s.guard(s.routes(static))
	return s, nil
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

	// 開いて最初の画面 (spec-nav.md 1.3): / is the profile screen's list, with no
	// filter and no row open, and never the place a person last stood. A 302, not
	// a 301: once the activity and team-definition screens are here, / still
	// points at the profile screen.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/profiles", http.StatusFound)
	})

	// ロール (r-778c): the three tabs, their checks and their writes. The old
	// page-per-role URLs move to the opened row (設計 6 節).
	mux.HandleFunc("GET /roles", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "roles") })
	mux.HandleFunc("GET /roles/assign", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "assign") })
	mux.HandleFunc("GET /roles/yaml", func(w http.ResponseWriter, r *http.Request) { s.r2Page(w, r, "yaml") })
	mux.HandleFunc("GET /roles/{role}", s.r2OldRole)
	mux.HandleFunc("GET /roles/{role}/new", s.r2OldRoleNew)
	mux.HandleFunc("GET /roles/{role}/{rule}", s.r2OldRoleRule)
	mux.HandleFunc("POST /roles/check", s.p2Post(s.r2Check))
	mux.HandleFunc("POST /roles/yaml/check", s.p2Post(s.r2YAMLCheck))
	mux.HandleFunc("POST /roles/save", s.p2Post(s.r2Save))
	mux.HandleFunc("POST /roles/delete", s.p2Post(s.r2Delete))
	mux.HandleFunc("POST /roles/yaml/save", s.p2Post(s.r2YAMLSave))
	mux.HandleFunc("POST /roles/assign/save", s.p2Post(s.r2AssignSave))

	// プロファイル (r-c683): the three tabs, their checks and their writes.
	mux.HandleFunc("GET /profiles", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "profiles") })
	mux.HandleFunc("GET /profiles/assign", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "assign") })
	mux.HandleFunc("GET /profiles/yaml", func(w http.ResponseWriter, r *http.Request) { s.p2Page(w, r, "yaml") })
	mux.HandleFunc("GET /profiles/{name}", s.oldProfile)
	mux.HandleFunc("POST /profiles/check", s.p2Post(s.p2Check))
	mux.HandleFunc("POST /profiles/yaml/check", s.p2Post(s.p2YAMLCheck))
	mux.HandleFunc("POST /profiles/save", s.p2Post(s.p2Save))
	mux.HandleFunc("POST /profiles/delete", s.p2Post(s.p2Delete))
	mux.HandleFunc("POST /profiles/yaml/save", s.p2Post(s.p2YAMLSave))
	mux.HandleFunc("POST /profiles/assign/save", s.p2Post(s.p2AssignSave))

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
