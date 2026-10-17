package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
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
	port  Port
	token string
	tpl   *template.Template
	// handler is the routes with the guards in front of them.
	handler http.Handler
	// addr is this server's own address ("127.0.0.1:8787"), set when the socket
	// is open; every request is checked against it.
	addr string
	// watcher remembers when each agent's current state was first seen, so the
	// activity can say how long a node has waited even though herdr reports no
	// time (設計 5.2、spec/37 N5). It is the only state this server keeps
	// between two answers.
	watcher *activity.Watcher
}

// New builds the server for one run: the catalog every screen reads through, and
// the Port the live side comes in on. The templates are parsed here, so a broken
// one stops hergy serve at once instead of on the first request; the token is
// made fresh for this run, and a write without it is refused.
func New(cat *catalog.Catalog, port Port) (*Server, error) {
	if cat == nil {
		return nil, errors.New("web: New needs a catalog")
	}
	if port == nil {
		return nil, errors.New("web: New needs a Port")
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
	s := &Server{
		cat: cat, port: port, token: token, tpl: tpl,
		watcher: activity.NewWatcher(),
	}
	mux := s.routes(static)
	s.handler = s.guard(s.dropLayer(mux))
	return s, nil
}

// dropLayer moves a request that still carries the old layer parameter to the
// same path without it, so a bookmark from the 0.6 screens lands on the new one
// (設計 6 節).
func (s *Server) dropLayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" && r.URL.Query().Has("layer") {
			q := r.URL.Query()
			q.Del("layer")
			target := r.URL.Path
			if encoded := q.Encode(); encoded != "" {
				target += "?" + encoded
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently) //nolint:gosec // G710: the target is this request's own path, with one query parameter removed
			return
		}
		next.ServeHTTP(w, r)
	})
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

// view is one answer: the body a screen built, its title, the band it stands in
// and the trail above it.
type view struct {
	title  string
	band   string
	crumb  []crumb
	body   template.HTML
	status int
}

// handlerFn answers one path: it takes the request and returns the page, or the
// reason it could not.
type handlerFn func(*http.Request) (view, error)

// routes is the whole table of the UI. Every path is a real path, and every one
// answers the whole page; the old WebUI's paths are moved with 301 or refused
// with 404 (設計 6 節).
func (s *Server) routes(static fs.FS) http.Handler {
	mux := http.NewServeMux()

	// A GET pattern answers HEAD too (net/http), so one registration is enough:
	// a second, explicit HEAD pattern beside a literal path (…/agreement/new)
	// and a wildcard one (…/agreement/{rule}) would not be comparable.
	get := func(pattern string, fn handlerFn) {
		mux.Handle("GET "+pattern, s.answer(fn))
	}
	post := func(pattern string, fn handlerFn) {
		mux.Handle("POST "+pattern, s.write(fn))
	}

	// チーム定義 (3.3・3.4).
	get("/{$}", s.definitions)
	get("/definitions", s.definitions)
	get("/definitions/{name}", s.definition)
	post("/definitions/{name}", s.saveMission)
	get("/definitions/{name}/members/{member}", s.memberPanel)
	get("/definitions/{name}/workers/{from}/{purpose}", s.workerPanel)
	get("/definitions/{name}/human", s.humanPanel)
	get("/definitions/{name}/turns/{from}/{to}", s.turnPanel)
	get("/definitions/{name}/agreement", s.agreementPanel)
	get("/definitions/{name}/agreement/new", s.teamNew)
	get("/definitions/{name}/agreement/{rule}", s.teamRule)
	post("/definitions/{name}/agreement", s.writeTeamAgreement)
	post("/definitions/{name}/agreement/new", s.addTeamRule)
	post("/definitions/{name}/agreement/{rule}", s.saveTeamRule)

	// 活動 (3.1・3.2).
	get("/activity", s.activityList)
	get("/activity/{team}", s.teamActivity)
	post("/activity/{team}/focus", s.focus)

	// ロール (3.5).
	get("/roles", s.roles)
	get("/roles/{role}", s.role)
	get("/roles/{role}/new", s.roleNew)
	get("/roles/{role}/{rule}", s.roleRule)
	post("/roles/{role}", s.writeRole)
	post("/roles/{role}/new", s.addRoleRule)
	post("/roles/{role}/{rule}", s.saveRoleRule)

	// 共通の約束 (3.6).
	get("/agreement", s.agreement)
	get("/agreement/new", s.agreementNew)
	get("/agreement/{rule}", s.agreementRule)
	post("/agreement", s.writeShared)
	post("/agreement/new", s.addAgreementRule)
	post("/agreement/{rule}", s.saveAgreementRule)

	// プロファイル (3.7).
	get("/profiles", s.profiles)
	get("/profiles/{name}", s.profile)

	// 古い URL (設計 6 節): the ones whose concept is one thing move with 301,
	// the rest are not found.
	mux.HandleFunc("GET /teams", redirect("/definitions"))
	mux.HandleFunc("HEAD /teams", redirect("/definitions"))
	mux.HandleFunc("GET /running", redirect("/activity"))
	mux.HandleFunc("HEAD /running", redirect("/activity"))
	mux.HandleFunc("GET /team", s.oldTeam)
	mux.HandleFunc("HEAD /team", s.oldTeam)
	for _, gone := range []string{"/rules", "/rules/", "/diff", "/bump", "/bump/", "/figure", "/panel/"} {
		mux.HandleFunc("GET "+gone, notFound)
		mux.HandleFunc("HEAD "+gone, notFound)
	}

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("/", notFound)
	return mux
}

// answer runs one read handler inside the frame. The token of this run is put on
// the request before the handler runs, so a screen that renders a form can write
// it into the form (Token).
func (s *Server) answer(fn handlerFn) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		if err != nil {
			s.problem(w, r, err)
			return
		}
		s.render(w, v)
	})
}

// write runs one write handler: it promises POST, the token of this run, and,
// when the browser sends an Origin, this server's own. A write that fails any of
// them is refused and its handler never runs.
func (s *Server) write(fn handlerFn) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, t("web.error.origin"), http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxWrite)
		if subtle.ConstantTimeCompare([]byte(r.PostFormValue("token")), []byte(s.token)) != 1 {
			http.Error(w, t("web.error.token"), http.StatusForbidden)
			return
		}
		v, err := fn(r)
		if err != nil {
			s.problem(w, r, err)
			return
		}
		s.render(w, v)
	})
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

// frame is what the layout is given: the bands, the trail, the token and the
// body.
type frame struct {
	Lang  string
	Title string
	Bands []band
	Crumb []crumb
	Token string
	Body  template.HTML
}

// render writes one page inside the frame. The page is rendered into a buffer
// first, so a template that fails halfway is one error page, not half a page
// followed by half an error.
func (s *Server) render(w http.ResponseWriter, v view) {
	status := v.status
	if status == 0 {
		status = http.StatusOK
	}
	var buf bytes.Buffer
	data := frame{
		Lang:  i18n.Language(),
		Title: v.title,
		Bands: bandsOf(v.band),
		Crumb: v.crumb,
		Token: s.token,
		Body:  v.body,
	}
	if err := s.tpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		s.problem(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// problem is what a failed screen shows: the frame, with the reason on it.
func (s *Server) problem(w http.ResponseWriter, _ *http.Request, err error) {
	body := problemBody(tf("web.error.internal", err))
	page := view{title: t("web.error.title"), body: body, status: http.StatusInternalServerError}
	var buf bytes.Buffer
	data := frame{Lang: i18n.Language(), Title: page.title, Bands: bandsOf(""), Body: page.body}
	if renderErr := s.tpl.ExecuteTemplate(&buf, "layout.html", data); renderErr != nil {
		http.Error(w, tf("web.error.internal", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(page.status)
	_, _ = w.Write(buf.Bytes())
}

// read renders one screen's template into a View body.
func (s *Server) show(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return HTML(buf.String()), nil
}

// pageBody renders the body template named name with data and wraps it in a
// view.
func (s *Server) pageBody(v view, name string, data any) (view, error) {
	body, err := s.show(name, data)
	if err != nil {
		return view{}, err
	}
	v.body = body
	return v, nil
}

// redirect answers an old path with a 301 to its new one.
func redirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusMovedPermanently)
	}
}

// notFound answers a path no screen claims, and every old path whose concept is
// not one thing (設計 6 節).
func notFound(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, t("web.error.not_found"), http.StatusNotFound)
}

// oldTeam moves the old "/team?name=x" to the definition it named, and the bare
// "/team" to the list.
func (s *Server) oldTeam(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Redirect(w, r, "/definitions", http.StatusMovedPermanently)
		return
	}
	http.Redirect(w, r, definitionPath(name), http.StatusMovedPermanently) //nolint:gosec // G710: the target is this package's own path, escaped by definitionPath
}

// newToken makes the secret of one run: 32 random bytes in hex.
func newToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
