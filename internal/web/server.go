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
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
)

// templates and static are embedded: hergy serve is one binary, and the pages
// it serves come from it, not from wherever it is run.
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
	port    Port
	token   string
	screens []Screen
	frame   *template.Template
	handler http.Handler
	// addr is this server's own address, "127.0.0.1:8787"; it is set when the
	// socket is open, and every request is checked against it.
	addr string
}

// New builds the server for one run. It parses the templates here, so a broken
// template stops hergy serve at once instead of on the first request, and it
// puts the team list in front of the screens the caller adds. The token is made
// here too, fresh for every run: every form carries it, and a write without it
// is refused.
func New(port Port, screens ...Screen) (*Server, error) {
	if port == nil {
		return nil, errors.New("web: New needs a Port")
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	frame, err := template.New("").Funcs(Funcs()).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	s := &Server{port: port, token: token, frame: frame}
	s.screens = append([]Screen{&teamList{port: port, templates: frame}}, screens...)

	mux := http.NewServeMux()
	for _, screen := range s.screens {
		for _, route := range screen.Routes() {
			mux.Handle(route.Path, s.route(screen, route))
		}
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	// A path no screen claims is not found, not the first screen: a screen that
	// answers a path it was not written for would hide a mistake in a link.
	mux.Handle("/", notFound())
	s.handler = s.guard(mux)
	return s, nil
}

// Serve listens on 127.0.0.1:port and answers until ctx is done. ready, when
// set, is called once with the URL to open, after the socket is open and before
// the first request is answered. On ctx.Done the server takes no new request
// and waits up to shutdownGrace for the ones in flight, so a screen halfway
// through its answer is not cut off.
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
	server := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
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
	if err != nil {
		return err
	}
	return nil
}

// URL is the address to open in a browser. It is set once the socket is open.
func (s *Server) URL() string {
	if s.addr == "" {
		return ""
	}
	return "http://" + s.addr
}

// guard holds the promises that stand in front of every request: the Host has
// to be this server's own name, and every answer carries the policy that keeps
// the page to this server's own files.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostOK(r) {
			http.Error(w, i18n.T("web.error.host"), http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// hostOK reports whether a request was addressed to this server. A browser that
// reached it wrote 127.0.0.1 or localhost and the port; another name, or another
// port, is refused, so a page elsewhere cannot reach the UI through a name it
// controls.
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
		// A browser leaves the port out only for the default one.
		return bound == "80"
	}
	return port == bound
}

// checkWrite is what a write promises: POST, the token of this run, and, when
// the browser sends an Origin, this server's own. A write that fails any of them
// is refused and its handler never runs.
func (s *Server) checkWrite(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, i18n.T("web.error.method"), http.StatusMethodNotAllowed)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		http.Error(w, i18n.T("web.error.origin"), http.StatusForbidden)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWrite)
	token := r.PostFormValue("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		http.Error(w, i18n.T("web.error.token"), http.StatusForbidden)
		return false
	}
	return true
}

// route is one path of one screen, with the promises of its own. The token of
// this run is put on the request before the handler runs, so a screen that
// renders a form can write it into the form (Token).
func (s *Server) route(screen Screen, route Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case route.Write:
			if !s.checkWrite(w, r) {
				return
			}
		case r.Method != http.MethodGet && r.Method != http.MethodHead:
			http.Error(w, i18n.T("web.error.method"), http.StatusMethodNotAllowed)
			return
		}
		view, err := route.Handler(r.WithContext(context.WithValue(r.Context(), tokenKey{}, s.token)))
		if err != nil {
			s.problem(w, r, screen, err)
			return
		}
		s.write(w, r, screen, view)
	})
}

// tokenKey is how the token of the run travels on a request.
type tokenKey struct{}

// Token is the token of this run. A screen renders it into the hidden field of
// every form it writes, and the server refuses a write that does not carry it
// (Route.Write). It is on the request a screen is given, and on no other.
func Token(r *http.Request) string {
	token, _ := r.Context().Value(tokenKey{}).(string)
	return token
}

// notFound answers a path no screen claims.
func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, i18n.T("web.error.not_found"), http.StatusNotFound)
	})
}

// write renders one answer inside the frame. The page is rendered into a buffer
// first, so a template that fails halfway is one error page and not half a page
// followed by half an error.
func (s *Server) write(w http.ResponseWriter, r *http.Request, screen Screen, view View) {
	status := view.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if view.Bare {
		w.WriteHeader(status)
		if _, err := w.Write([]byte(view.Body)); err != nil {
			return
		}
		return
	}
	var buf bytes.Buffer
	if err := s.frame.ExecuteTemplate(&buf, "layout.html", s.frameData(r, screen, view)); err != nil {
		s.problem(w, r, screen, err)
		return
	}
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return
	}
}

// problem is what a screen's failure shows: the frame, with the reason on it.
// The reason is shown because this server answers one person on their own
// machine, and a blank page would say less than the error does.
func (s *Server) problem(w http.ResponseWriter, r *http.Request, screen Screen, err error) {
	view := View{
		Title:  screen.Name(),
		Body:   problemBody(i18n.Tf("web.error.internal", err)),
		Status: http.StatusInternalServerError,
	}
	s.write(w, r, screen, view)
}

// page is what the frame template is given.
type page struct {
	Lang      string
	Title     string
	Nav       []link
	Layers    []link
	LayerRoot string
	Global    bool
	Body      template.HTML
}

// link is one path in the frame: what it is called, and whether it is the one
// being shown.
type link struct {
	Name   string
	Href   string
	Active bool
}

// frameData fills the frame: the screens as the navigation, the two layers as
// the toggle, and the layer being shown.
func (s *Server) frameData(r *http.Request, current Screen, view View) page {
	layer := LayerOf(r)
	data := page{
		Lang:      i18n.Language(),
		Title:     view.Title,
		LayerRoot: s.port.LayerRoot(layer),
		Global:    layer == LayerGlobal,
		Body:      view.Body,
	}
	for _, screen := range s.screens {
		if routes := screen.Routes(); len(routes) > 0 {
			data.Nav = append(data.Nav, link{
				Name:   screen.Name(),
				Href:   hrefOf(routes[0].Path) + "?layer=" + layer,
				Active: screen == current,
			})
		}
	}
	for _, name := range Layers {
		data.Layers = append(data.Layers, link{
			Name:   i18n.T("label." + name),
			Href:   r.URL.Path + "?layer=" + name,
			Active: name == layer,
		})
	}
	return data
}

// hrefOf writes a route's pattern the way a browser asks for it: the method
// dropped, and the exact root written as "/".
func hrefOf(pattern string) string {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:] // a "GET /x" pattern
	}
	return strings.Replace(pattern, "/{$}", "/", 1)
}

// LayerOf reads the layer a request asks for; without one, the local layer. The
// layer travels in the URL (?layer=global), so a link, a form and a fragment
// request all carry the layer they were made in.
func LayerOf(r *http.Request) string {
	if r.URL.Query().Get("layer") == LayerGlobal {
		return LayerGlobal
	}
	return LayerLocal
}

// newToken makes the secret of one run: 32 random bytes in hex. Only this
// process and the pages it renders know it.
func newToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
