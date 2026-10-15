package web

import (
	"context"
	"errors"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
)

// fakePort answers the screens from tables a test fills in, so a screen can be
// tested without a project on disk.
type fakePort struct {
	teams map[string][]Team
	roots map[string]string
}

func (p fakePort) Teams(layer string) []Team     { return p.teams[layer] }
func (p fakePort) LayerRoot(layer string) string { return p.roots[layer] }

// demoPort is the port most tests use: one readable and one broken local
// definition, and one global definition.
func demoPort() fakePort {
	return fakePort{
		teams: map[string][]Team{
			LayerLocal: {
				{Name: "demo", Members: 3, Rules: 8, NoCheck: 2},
				{Name: "broken", Broken: ".hergy/teams/broken/team.yaml: version must be 1"},
			},
			LayerGlobal: {{Name: "shared", Members: 1, Rules: 2, NoCheck: 0}},
		},
		roots: map[string]string{LayerLocal: ".hergy", LayerGlobal: "~/.config/hergy"},
	}
}

// newTestServer builds a server with an address of its own, so the Host check
// has something to compare with. Nothing is listening: these tests call the
// handler directly, and the Serve tests open a real socket.
func newTestServer(t *testing.T, port Port, screens ...Screen) *Server {
	t.Helper()
	s, err := New(port, screens...)
	if err != nil {
		t.Fatal(err)
	}
	s.addr = "127.0.0.1:8787"
	return s
}

// do sends one request through the handler, with the Host a browser would send
// unless the test says otherwise.
func do(t *testing.T, s *Server, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	if r.Host == "example.com" {
		r.Host = s.addr
	}
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	return w
}

// get is one GET.
func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, httptest.NewRequest(http.MethodGet, path, nil))
}

// post is one form POST to a write path. The server is taken so a test reads
// the same way it calls the others; a POST only needs the path and the body.
func post(t *testing.T, _ *Server, path, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s does not contain %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Errorf("%s contains %q, which belongs to another screen:\n%s", what, bad, body)
		}
	}
}

// TestTeamListShowsOneRowPerDefinition: the team list is the root screen. Each
// row carries the name, the members and the count of rules with no check, and a
// definition that cannot be read keeps its row and shows why.
func TestTeamListShowsOneRowPerDefinition(t *testing.T) {
	s := newTestServer(t, demoPort())
	w := get(t, s, "/")

	if w.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("GET / Content-Type = %q, want text/html", got)
	}
	mustContain(t, "the team list", w.Body.String(),
		i18n.T("web.teams.title"),
		i18n.Tf("web.layer.name", i18n.T("label."+LayerLocal)),
		"demo",
		i18n.Tf("web.teams.members", 3),
		i18n.Tf("web.teams.no_check", 2, 8),
		"broken",
		".hergy/teams/broken/team.yaml: version must be 1",
	)
	// The local layer is the one being shown, so the global definition is not on
	// this page.
	mustNotContain(t, "the local team list", w.Body.String(), "shared")
}

// TestLayerToggleShowsTheOtherLayer: ?layer=global shows the global layer's
// definitions, its root, and the mark that says this layer reaches other
// projects. The local layer carries no such mark.
func TestLayerToggleShowsTheOtherLayer(t *testing.T) {
	s := newTestServer(t, demoPort())

	global := get(t, s, "/?layer=global")
	if global.Code != http.StatusOK {
		t.Fatalf("GET /?layer=global = %d, want 200", global.Code)
	}
	mustContain(t, "the global team list", global.Body.String(),
		"shared",
		"~/.config/hergy",
		i18n.T("web.frame.global_note"),
	)
	mustNotContain(t, "the global team list", global.Body.String(), "demo", ".hergy/teams/broken")

	local := get(t, s, "/")
	mustNotContain(t, "the local team list", local.Body.String(), i18n.T("web.frame.global_note"))
	mustContain(t, "the local team list", local.Body.String(), ".hergy")
}

// TestTheLayerTravelsInTheLinks: a link inside the frame keeps the layer being
// shown, so pressing it does not silently go back to the local layer.
func TestTheLayerTravelsInTheLinks(t *testing.T) {
	s := newTestServer(t, demoPort())
	w := get(t, s, "/?layer=global")
	mustContain(t, "the frame", w.Body.String(),
		`href="/?layer=global"`,
		`href="/?layer=local"`,
	)
}

// TestUnknownPathIsNotFound: a path no screen claims is a 404, not the first
// screen: a screen that answers a path it was not written for would hide a
// mistake in a link.
func TestUnknownPathIsNotFound(t *testing.T) {
	s := newTestServer(t, demoPort())
	w := get(t, s, "/nope")
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /nope = %d, want 404", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, i18n.T("web.error.not_found")) {
		t.Errorf("GET /nope does not say there is no such screen:\n%s", body)
	}
}

// TestHostMustBeThisServer: only a request addressed to this server's own name
// and port is answered, so a page elsewhere cannot reach the UI through a name
// it controls.
func TestHostMustBeThisServer(t *testing.T) {
	s := newTestServer(t, demoPort())
	cases := []struct {
		host string
		want int
	}{
		{"127.0.0.1:8787", http.StatusOK},
		{"localhost:8787", http.StatusOK},
		{"[::1]:8787", http.StatusOK},
		{"127.0.0.1:9999", http.StatusForbidden},
		{"127.0.0.1", http.StatusForbidden},
		{"evil.example:8787", http.StatusForbidden},
		{"localhost.evil.example:8787", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Host = tc.host
			w := do(t, s, r)
			if w.Code != tc.want {
				t.Fatalf("Host %q = %d, want %d", tc.host, w.Code, tc.want)
			}
			if tc.want == http.StatusForbidden {
				if body := w.Body.String(); !strings.Contains(body, i18n.T("web.error.host")) {
					t.Errorf("the refusal does not say why:\n%s", body)
				}
			}
		})
	}
}

// TestEveryAnswerCarriesThePolicy: the content policy is on the page, on a
// static file, and on a refusal too, so nothing a page loads can come from
// anywhere but this server.
func TestEveryAnswerCarriesThePolicy(t *testing.T) {
	s := newTestServer(t, demoPort())
	for _, path := range []string{"/", "/static/hergy.css", "/nope"} {
		w := get(t, s, path)
		if got := w.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
			t.Errorf("GET %s: Content-Security-Policy = %q, want default-src 'self'", path, got)
		}
	}
}

// TestStaticFilesComeFromTheBinary: the stylesheet and the embedded htmx are
// served, with the license htmx comes under.
func TestStaticFilesComeFromTheBinary(t *testing.T) {
	s := newTestServer(t, demoPort())
	for path, want := range map[string]string{
		"/static/hergy.css":          "table.teams",
		"/static/htmx-2.0.4.min.js":  "htmx",
		"/static/htmx-2.0.4.LICENSE": "Zero-Clause BSD",
	} {
		w := get(t, s, path)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
			continue
		}
		if body := w.Body.String(); !strings.Contains(body, want) {
			t.Errorf("GET %s does not look like the file it should be", path)
		}
	}
}

// writeScreen is a screen with one write path, so a test can ask what a write
// promises.
type writeScreen struct {
	calls int
}

func (s *writeScreen) Name() string { return "write" }

func (s *writeScreen) Routes() []Route {
	return []Route{{
		Path:  "/save",
		Write: true,
		Handler: func(*http.Request) (View, error) {
			s.calls++
			return View{Title: "saved", Body: HTML("<p>saved</p>")}, nil
		},
	}}
}

// TestAWriteNeedsPostAndThisRunsToken: a write is a POST that carries the token
// of this run, and an Origin the browser sends has to be this server's own. A
// request that fails any of them never reaches the screen's handler.
func TestAWriteNeedsPostAndThisRunsToken(t *testing.T) {
	screen := &writeScreen{}
	s := newTestServer(t, demoPort(), screen)

	good := "token=" + s.token
	cases := []struct {
		name   string
		build  func(t *testing.T) *http.Request
		want   int
		reason string
	}{
		{
			name:   "a GET changes nothing",
			build:  func(_ *testing.T) *http.Request { return httptest.NewRequest(http.MethodGet, "/save", nil) },
			want:   http.StatusMethodNotAllowed,
			reason: i18n.T("web.error.method"),
		},
		{
			name:   "a POST with no token",
			build:  func(t *testing.T) *http.Request { return post(t, s, "/save", "value=1") },
			want:   http.StatusForbidden,
			reason: i18n.T("web.error.token"),
		},
		{
			name:   "a POST with another token",
			build:  func(t *testing.T) *http.Request { return post(t, s, "/save", "token=not-this-one") },
			want:   http.StatusForbidden,
			reason: i18n.T("web.error.token"),
		},
		{
			name: "a POST with the token in the URL, not the body",
			build: func(t *testing.T) *http.Request {
				r := post(t, s, "/save?token="+s.token, "")
				return r
			},
			want:   http.StatusForbidden,
			reason: i18n.T("web.error.token"),
		},
		{
			name: "a POST from another page",
			build: func(t *testing.T) *http.Request {
				r := post(t, s, "/save", good)
				r.Header.Set("Origin", "http://evil.example")
				return r
			},
			want:   http.StatusForbidden,
			reason: i18n.T("web.error.origin"),
		},
		{
			name: "a POST of this page",
			build: func(t *testing.T) *http.Request {
				r := post(t, s, "/save", good)
				r.Header.Set("Origin", "http://127.0.0.1:8787")
				return r
			},
			want: http.StatusOK,
		},
		{
			name:  "a POST from a client that sends no Origin",
			build: func(t *testing.T) *http.Request { return post(t, s, "/save", good) },
			want:  http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := screen.calls
			w := do(t, s, tc.build(t))
			if w.Code != tc.want {
				t.Fatalf("%s = %d, want %d:\n%s", tc.name, w.Code, tc.want, w.Body.String())
			}
			if tc.reason != "" {
				if body := w.Body.String(); !strings.Contains(body, tc.reason) {
					t.Errorf("%s does not say why:\n%s", tc.name, body)
				}
			}
			wantCalls := before
			if tc.want == http.StatusOK {
				wantCalls++
			}
			if screen.calls != wantCalls {
				t.Errorf("%s: the screen's handler ran %d more time(s), want %d",
					tc.name, screen.calls-before, wantCalls-before)
			}
		})
	}
}

// formScreen is a screen with a form and the write that form sends to. It
// renders the token of the run into the form (Token), so a test can follow the
// whole way a page makes a write.
type formScreen struct {
	saved string
}

func (s *formScreen) Name() string { return "form" }

func (s *formScreen) Routes() []Route {
	return []Route{
		{
			Path: "/form",
			Handler: func(r *http.Request) (View, error) {
				body := `<form method="post" action="/form/save">` +
					`<input type="hidden" name="token" value="` + Token(r) + `">` +
					`<input name="text" value=""></form>`
				return View{Title: "form", Body: HTML(body)}, nil
			},
		},
		{
			Path:  "/form/save",
			Write: true,
			Handler: func(r *http.Request) (View, error) {
				s.saved = r.PostFormValue("text")
				return View{Title: "saved", Body: HTML("<p>saved</p>")}, nil
			},
		},
	}
}

// tokenField is the hidden field a form carries.
var tokenField = regexp.MustCompile(`name="token" value="([^"]*)"`)

// TestAFormCarriesTheTokenOfTheRun: the page a screen renders holds the token of
// this run, a write that carries what the page held is accepted, and a write
// carrying another run's token is not. This is the whole way the editing screens
// write (Route.Write).
func TestAFormCarriesTheTokenOfTheRun(t *testing.T) {
	screen := &formScreen{}
	s := newTestServer(t, demoPort(), screen)

	page := get(t, s, "/form")
	m := tokenField.FindStringSubmatch(page.Body.String())
	if m == nil {
		t.Fatalf("the form carries no token field:\n%s", page.Body.String())
	}
	if m[1] != s.token {
		t.Errorf("the form carries the token %q, want this run's", m[1])
	}

	saved := do(t, s, post(t, s, "/form/save", "token="+m[1]+"&text=hello"))
	if saved.Code != http.StatusOK {
		t.Fatalf("the write with the page's token = %d, want 200:\n%s", saved.Code, saved.Body.String())
	}
	if screen.saved != "hello" {
		t.Errorf("the screen saved %q, want hello", screen.saved)
	}

	other := newTestServer(t, demoPort())
	stolen := do(t, s, post(t, s, "/form/save", "token="+other.token+"&text=stolen"))
	if stolen.Code != http.StatusForbidden {
		t.Errorf("a write with another run's token = %d, want 403", stolen.Code)
	}
	if screen.saved != "hello" {
		t.Errorf("a write with another run's token saved %q", screen.saved)
	}
}

// failingScreen is a screen whose handler fails, so a test can see what a
// failure shows.
type failingScreen struct{}

func (failingScreen) Name() string { return "failing" }

func (failingScreen) Routes() []Route {
	return []Route{{
		Path: "/boom",
		Handler: func(*http.Request) (View, error) {
			return View{}, errors.New("the file went away")
		},
	}}
}

// TestAScreenFailureShowsWhy: a failure inside a screen is one page with the
// reason on it, in the frame, and not half a page and a broken one.
func TestAScreenFailureShowsWhy(t *testing.T) {
	s := newTestServer(t, demoPort(), failingScreen{})
	w := get(t, s, "/boom")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET /boom = %d, want 500", w.Code)
	}
	mustContain(t, "the failure", w.Body.String(), "the file went away", i18n.T("web.frame.panel"))
}

// fragmentScreen is a screen that answers a fragment, the way a swap into the
// right panel does.
type fragmentScreen struct{}

func (fragmentScreen) Name() string { return "fragment" }

func (fragmentScreen) Routes() []Route {
	return []Route{{
		Path: "/fragment",
		Handler: func(*http.Request) (View, error) {
			return View{Title: "fragment", Body: HTML("<p>only this</p>"), Bare: true}, nil
		},
	}}
}

// TestAFragmentIsWrittenWithoutTheFrame: a Bare answer is the body alone, so
// what a swap puts into a panel is not a whole page inside a panel.
func TestAFragmentIsWrittenWithoutTheFrame(t *testing.T) {
	s := newTestServer(t, demoPort(), fragmentScreen{})
	w := get(t, s, "/fragment")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /fragment = %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "<p>only this</p>" {
		t.Errorf("the fragment is %q, want the body alone", body)
	}
}

// namedScreen is one screen of a test, with the name and body the test gives.
type namedScreen struct {
	name string
	path string
}

func (s namedScreen) Name() string { return s.name }

func (s namedScreen) Routes() []Route {
	return []Route{{
		Path: s.path,
		Handler: func(*http.Request) (View, error) {
			return View{Title: s.name, Body: HTML("<p>" + s.name + "</p>")}, nil
		},
	}}
}

// TestTheNavigationShowsEveryScreen: the frame lists the screens it was given,
// and the one being shown is marked as the current page.
func TestTheNavigationShowsEveryScreen(t *testing.T) {
	s := newTestServer(t, demoPort(), namedScreen{name: "rules", path: "/rules"})

	list := get(t, s, "/")
	mustContain(t, "the navigation", list.Body.String(),
		i18n.T("web.teams.title"),
		`href="/?layer=local"`,
		"rules",
		`href="/rules?layer=local"`,
		`aria-current="page"`,
	)

	other := get(t, s, "/rules")
	if other.Code != http.StatusOK {
		t.Fatalf("GET /rules = %d, want 200", other.Code)
	}
	mustContain(t, "the rules screen", other.Body.String(), "<p>rules</p>", i18n.T("web.teams.title"))
}

// TestInlineRendersOnlyBoldAndCode is the promise B and D build their detail
// views on: the whole text is escaped first, and only **bold** and `code` are
// turned into markup (spec/10 の 4 節).
func TestInlineRendersOnlyBoldAndCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is left alone", "read the file", "read the file"},
		{"bold", "**stop** and look", "<strong>stop</strong> and look"},
		{"code", "run `hergy status`", "run <code>hergy status</code>"},
		{"both", "**stop**: run `hergy status`", "<strong>stop</strong>: run <code>hergy status</code>"},
		{"two pairs", "`a` and `b`", "<code>a</code> and <code>b</code>"},
		{"a raw tag is shown, not run", "<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"a link stays text", "[hergy](http://example.com)", "[hergy](http://example.com)"},
		{"a quote cannot end an attribute", `" onload="x`, `&#34; onload=&#34;x`},
		{"an unpaired bold mark shows", "wait for **the answer", "wait for **the answer"},
		{"an unpaired backtick shows", "wait for `the answer", "wait for `the answer"},
		{"a tag inside code is escaped", "`<b>`", "<code>&lt;b&gt;</code>"},
		{"ampersands are escaped", "a & b", "a &amp; b"},
		{"a line break is kept", "one\ntwo", "one\ntwo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(Inline(tc.in)); got != tc.want {
				t.Errorf("Inline(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestInlineThroughATemplate: the function is what the templates use, so a
// screen that renders a rule's text through the template gets the same escaping
// as a screen that calls it directly.
func TestInlineThroughATemplate(t *testing.T) {
	tmpl, err := template.New("t").Funcs(Funcs()).Parse(`{{inline .}}`)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, "**go** `<now>`"); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "<strong>go</strong> <code>&lt;now&gt;</code>"; got != want {
		t.Errorf("the template rendered %q, want %q", got, want)
	}
}

// TestServeAnswersOnTheBoundAddress: the server really listens, on 127.0.0.1,
// and a browser reaching it through its own name is answered. Ctrl-C (a canceled
// context) stops it, and Serve waits for the requests in flight.
func TestServeAnswersOnTheBoundAddress(t *testing.T) {
	s := newTestServer(t, demoPort())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, 0, func(url string) { urls <- url }) }()

	var url string
	select {
	case url = <-urls:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not report its URL")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("Serve reported the URL %q, want a 127.0.0.1 address", url)
	}
	if !strings.Contains(url, s.addr) {
		t.Fatalf("Serve reported the URL %q, but is listening on %q", url, s.addr)
	}

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Get(url + "/")
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", url, resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("a real answer carries %q, want default-src 'self'", got)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve stopped with %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after its context was canceled")
	}
}

// TestServeReportsAPortItCannotHave: a port already in use stops hergy serve
// with the address it could not take, and not with a half-open server.
func TestServeReportsAPortItCannotHave(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	_, port, err := net.SplitHostPort(taken.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	s := newTestServer(t, demoPort())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Serve(ctx, n, nil)
	if err == nil {
		t.Fatal("Serve took a port that was already in use")
	}
	want := i18n.Tf("error.serve_bind", "127.0.0.1:"+port, "")
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("Serve said %q, want it to start with %q", err.Error(), want)
	}
}

// assetScreen is a screen that may bring files of its own: its stylesheet, its
// script, and something it draws with.
type assetScreen struct {
	path   string
	assets Assets
}

func (s *assetScreen) Name() string { return s.path }

func (s *assetScreen) Routes() []Route {
	return []Route{{
		Path: "/" + s.path,
		Handler: func(*http.Request) (View, error) {
			return View{Title: s.path, Body: HTML(`<p class="chart">chart</p>`)}, nil
		},
	}}
}

func (s *assetScreen) Assets() Assets { return s.assets }

// assetFiles are the files a test screen brings: a stylesheet, a script, and a
// file it draws with.
func assetFiles() fstest.MapFS {
	return fstest.MapFS{
		"chart.css": {Data: []byte(".chart { color: teal; }\n")},
		"chart.js":  {Data: []byte("// chart\n")},
		"chart.svg": {Data: []byte("<svg></svg>\n")},
	}
}

// TestAScreenBringsItsOwnFiles: a screen's stylesheet and script are in the
// page's head and served from it, and the rest of what it brings is served too.
// This is what keeps two screens from having to share one stylesheet, and the
// only way a screen can bring a style at all: the policy refuses a style or a
// script written into the page itself.
func TestAScreenBringsItsOwnFiles(t *testing.T) {
	screen := &assetScreen{
		path:   "drawn",
		assets: Assets{Name: "drawn", Files: assetFiles(), Stylesheets: []string{"chart.css"}, Scripts: []string{"chart.js"}},
	}
	s := newTestServer(t, demoPort(), screen)
	page := get(t, s, "/drawn")
	body := page.Body.String()
	mustContain(t, "the frame", body,
		`<link rel="stylesheet" href="/static/drawn/chart.css">`,
		`<script src="/static/drawn/chart.js" defer></script>`)
	if strings.Index(body, "/static/htmx-2.0.4.min.js") > strings.Index(body, "/static/drawn/chart.js") {
		t.Errorf("the screen's script loads before HTMX, which it may use:\n%s", body)
	}
	if strings.Contains(body, "style=") {
		t.Errorf("the page carries a style of its own, which the policy refuses:\n%s", body)
	}

	css := get(t, s, "/static/drawn/chart.css")
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), ".chart") {
		t.Errorf("GET /static/drawn/chart.css = %d %q", css.Code, css.Body.String())
	}
	if got := css.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("a screen's own file carries %q, want the policy", got)
	}
	svg := get(t, s, "/static/drawn/chart.svg")
	if svg.Code != http.StatusOK || !strings.Contains(svg.Body.String(), "<svg>") {
		t.Errorf("GET /static/drawn/chart.svg = %d %q", svg.Code, svg.Body.String())
	}
}

// TestAScreenWithBrokenAssetsStopsTheServer: a file named but not brought, a
// name that cannot go into a URL, and a screen that brings no files at all are
// caught when the server is built, not by a person meeting a broken link.
func TestAScreenWithBrokenAssetsStopsTheServer(t *testing.T) {
	for what, assets := range map[string]Assets{
		"a stylesheet that is not there": {Name: "drawn", Files: assetFiles(), Stylesheets: []string{"gone.css"}},
		"a script that is not there":     {Name: "drawn", Files: assetFiles(), Scripts: []string{"gone.js"}},
		"a name that is not one URL word": {Name: "drawn/here", Files: assetFiles(),
			Stylesheets: []string{"chart.css"}},
		"no files at all": {Name: "drawn"},
	} {
		screen := &assetScreen{path: "drawn", assets: assets}
		if _, err := New(demoPort(), screen); err == nil {
			t.Errorf("%s: New accepted it", what)
		}
	}
}

// TestTwoScreensCannotServeUnderOneName: the second screen that would take the
// first one's paths is refused, rather than quietly displacing it.
func TestTwoScreensCannotServeUnderOneName(t *testing.T) {
	first := &assetScreen{path: "one", assets: Assets{Name: "drawn", Files: assetFiles()}}
	second := &assetScreen{path: "two", assets: Assets{Name: "drawn", Files: assetFiles()}}
	if _, err := New(demoPort(), first, second); err == nil {
		t.Error("New let two screens serve their files under one name")
	}
}

// panelScreen opens with something in the right panel, the way a screen opening
// on a rule being edited does, and answers the panel on its own when something
// is picked.
type panelScreen struct{ panel template.HTML }

func (s *panelScreen) Name() string { return "picked" }

func (s *panelScreen) Routes() []Route {
	return []Route{
		{
			Path: "/picked",
			Handler: func(*http.Request) (View, error) {
				return View{
					Title: "picked",
					Body:  HTML(`<a hx-target="#` + PanelID + `" href="/picked/one">one</a>`),
					Panel: s.panel,
				}, nil
			},
		},
		{
			Path: "/picked/one",
			Handler: func(*http.Request) (View, error) {
				return View{Body: HTML("<h2>one</h2>"), Bare: true}, nil
			},
		},
	}
}

// TestThePanelShowsWhatThePageOpensWith: the panel the constant names is where a
// screen puts what its page opens with, and a screen that puts nothing there
// leaves the frame's guidance. A panel swapped in on its own is a fragment, with
// no frame around it.
func TestThePanelShowsWhatThePageOpensWith(t *testing.T) {
	s := newTestServer(t, demoPort(), &panelScreen{panel: HTML(`<h2 class="picked">the rule</h2>`)})
	body := get(t, s, "/picked").Body.String()
	mustContain(t, "the page", body,
		`id="`+PanelID+`"`,
		`hx-target="#`+PanelID+`"`,
		`<h2 class="picked">the rule</h2>`)
	if strings.Contains(body, i18n.T("web.frame.panel_empty")) {
		t.Errorf("a page that opens with something in the panel still shows the guidance:\n%s", body)
	}

	empty := newTestServer(t, demoPort(), &panelScreen{})
	mustContain(t, "a page whose panel is empty", get(t, empty, "/picked").Body.String(),
		i18n.T("web.frame.panel"), i18n.T("web.frame.panel_empty"))

	fragment := get(t, s, "/picked/one")
	if fragment.Code != http.StatusOK {
		t.Fatalf("GET /picked/one = %d, want 200", fragment.Code)
	}
	if strings.Contains(fragment.Body.String(), "<html") || !strings.Contains(fragment.Body.String(), "<h2>one</h2>") {
		t.Errorf("the panel fragment is not a fragment: %q", fragment.Body.String())
	}
}

// TestTheFrameTurnsOffWhatHTMXWouldWriteItself: HTMX writes a <style> for its
// loading mark into the head by default, which the policy refuses (so the mark
// would not show either). The frame turns that off, the mark's look lives in the
// stylesheet the frame links, and inline evaluation is off too.
func TestTheFrameTurnsOffWhatHTMXWouldWriteItself(t *testing.T) {
	page := get(t, newTestServer(t, demoPort()), "/").Body.String()
	mustContain(t, "the frame", page,
		`<meta name="htmx-config" content='{"includeIndicatorStyles":false,"allowEval":false}'>`)
	if strings.Contains(page, "<style") {
		t.Errorf("the page would carry a <style>:\n%s", page)
	}
	css, err := fs.ReadFile(files, "static/hergy.css")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "hergy.css", string(css), ".htmx-indicator", ".htmx-request .htmx-indicator")
}
