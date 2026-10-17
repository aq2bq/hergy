package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
	"go.uber.org/goleak"
)

// TestMain loads the locales, so the screens render the words a person reads,
// and then fails when a test leaves a goroutine behind (d-b28a).
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	if err := i18n.Use("ja"); err != nil {
		panic(err)
	}
	goleak.VerifyTestMain(m)
}

// fixture is one working directory with a project's own .hergy and an outside
// settings directory, so every screen reads files a test wrote and never the
// machine's own.
type fixture struct {
	root    string
	proj    string
	global  string
	running []effect.Running
	agents  map[string][]activity.Agent
	port    *fakePort
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		root:   root,
		proj:   filepath.Join(root, "proj"),
		global: filepath.Join(root, "global"),
		agents: map[string][]activity.Agent{},
	}
	for _, dir := range []string{f.proj, f.global} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.writeTeam(t, "demo", "demo の使命")
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n    args: [--x]\n    env:\n      A: one\n")
	f.write(t, "roles/lead.yaml", "version: 1\nsections:\n  - rules:\n      - id: LEAD-1\n        text: lead は最初に動く。\n")
	f.write(t, "roles/builder.yaml", "version: 1\nsections:\n  - rules:\n      - id: BUILD-1\n        text: builder は作る。\n")
	f.write(t, "agreement.yaml", "version: 1\nsections:\n  - rules:\n      - id: SHARED-1\n        text: 全員が読む。\n        why: だから。\n")
	f.port = &fakePort{f: f}
	return f
}

// write writes one file under the project's own settings directory.
func (f *fixture) write(t *testing.T, rel, text string) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTeam writes one team definition with a member, a worker, a turn to the
// human, a role document for each role, a team agreement and a shared one.
func (f *fixture) writeTeam(t *testing.T, name, mission string) {
	t.Helper()
	f.write(t, filepath.Join("teams", name, "team.yaml"), "version: 1\nmission: "+mission+"\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\nworkers:\n  - from: lead\n    purpose: 試作\n    profile: coder\n")
	f.write(t, filepath.Join("teams", name, "agreement.yaml"), "version: 1\nsections:\n  - rules:\n      - id: TEAM-1\n        text: このチームの約束。\n")
}

// catalog is the catalog the screens read through.
func (f *fixture) catalog() *catalog.Catalog {
	return catalog.New(f.global, filepath.Join(f.proj, ".hergy"), f.proj, source.Disk{}, func(path string) string { return path }).InRepo(true)
}

// server builds the server over the fixture, with the fake live side.
func (f *fixture) server(t *testing.T) *Server {
	t.Helper()
	server, err := New(f.catalog(), f.port)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// get runs one GET against the server's own handler, with the Host check passing
// as it does for a browser.
func (f *fixture) get(t *testing.T, server *Server, target string) (*http.Response, string) {
	t.Helper()
	server.addr = "127.0.0.1:8787"
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "127.0.0.1:8787"
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(body)
}

// post runs one POST with the token of the run.
func (f *fixture) post(t *testing.T, server *Server, form url.Values) (*http.Response, string) {
	t.Helper()
	const target = "/definitions/demo"
	server.addr = "127.0.0.1:8787"
	if form == nil {
		form = url.Values{}
	}
	if _, ok := form["token"]; !ok && server.token != "" {
		form.Set("token", server.token)
	}
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(body)
}

// fakePort is the live half the screens read, made up by the test.
type fakePort struct{ f *fixture }

func (p *fakePort) Running() ([]effect.Running, error) { return p.f.running, nil }

func (p *fakePort) Agents(team string) ([]activity.Agent, error) { return p.f.agents[team], nil }

func (p *fakePort) Tail(string, int) (string, error) { return "lead is waiting\n", nil }

func (p *fakePort) Focus(string) error { return nil }

// TestEveryPathAnswers: every path of the UI answers the whole page, with the
// heading and the band of the screen it is (設計 8 節).
func TestEveryPathAnswers(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 3}}
	server := f.server(t)

	paths := []struct{ path, want string }{
		{"/", "チーム定義"},
		{"/definitions", "チーム定義"},
		{"/definitions/demo", "チーム定義 demo"},
		{"/definitions/demo/members/lead", "メンバー lead"},
		{"/definitions/demo/workers/lead/試作", "ワーカー"},
		{"/definitions/demo/human", "人間"},
		{"/definitions/demo/turns/lead/builder", "渡す側"},
		{"/definitions/demo/agreement", "チームの約束"},
		{"/definitions/demo/agreement/TEAM-1", "規則 TEAM-1"},
		{"/activity", "活動"},
		{"/activity/demo", "チーム demo"},
		{"/roles", "ロール"},
		{"/roles/lead", "ロール lead"},
		{"/roles/lead/LEAD-1", "規則 LEAD-1"},
		{"/agreement", "共通の約束"},
		{"/agreement/SHARED-1", "規則 SHARED-1"},
		{"/profiles", "プロファイル"},
		{"/profiles/coder", "プロファイル coder"},
	}
	for _, test := range paths {
		resp, body := f.get(t, server, test.path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d", test.path, resp.StatusCode)
			continue
		}
		if !strings.Contains(body, test.want) {
			t.Errorf("GET %s does not hold %q", test.path, test.want)
		}
		if !strings.Contains(body, `aria-current="page"`) {
			t.Errorf("GET %s marks no band as where the person is", test.path)
		}
	}
}

// TestEveryLinkResolves: every href a page writes answers 200 or 301, so no
// screen holds a dead link (設計 8 節).
func TestEveryLinkResolves(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 3}}
	server := f.server(t)

	seen := map[string]bool{}
	queue := []string{"/", "/definitions/demo", "/activity", "/activity/demo", "/roles", "/roles/lead", "/agreement", "/profiles", "/profiles/coder"}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] {
			continue
		}
		seen[path] = true
		if strings.HasPrefix(path, "/static/") {
			continue
		}
		resp, body := f.get(t, server, path)
		switch resp.StatusCode {
		case http.StatusOK:
			for _, href := range hrefs(body) {
				if strings.HasPrefix(href, "/") && !seen[href] {
					queue = append(queue, href)
				}
			}
		case http.StatusMovedPermanently, http.StatusNotFound:
		default:
			t.Errorf("GET %s: status %d", path, resp.StatusCode)
		}
	}
	if len(seen) < 15 {
		t.Errorf("the walk reached only %d paths: %v", len(seen), seen)
	}
}

// hrefs is every href a page writes, in the order it writes them.
func hrefs(body string) []string {
	var out []string
	for rest := body; ; {
		i := strings.Index(rest, `href="`)
		if i < 0 {
			return out
		}
		rest = rest[i+len(`href="`):]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j:]
	}
}

// TestOldPathsMoveOrAreGone: the old WebUI's paths move to their new one with a
// 301, and the ones whose concept is not one thing are 404 (設計 6 節).
func TestOldPathsMoveOrAreGone(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	moved := map[string]string{
		"/teams":            "/definitions",
		"/running":          "/activity",
		"/team":             "/definitions",
		"/team?name=demo":   "/definitions/demo",
		"/profiles?layer=x": "/profiles",
	}
	for from, to := range moved {
		resp, _ := f.get(t, server, from)
		if resp.StatusCode != http.StatusMovedPermanently {
			t.Errorf("GET %s: status %d, want 301", from, resp.StatusCode)
			continue
		}
		if got := resp.Header.Get("Location"); got != to {
			t.Errorf("GET %s moves to %q, want %q", from, got, to)
		}
	}
	for _, gone := range []string{"/rules", "/rules/x", "/diff", "/bump", "/bump/x", "/figure", "/panel/member"} {
		resp, _ := f.get(t, server, gone)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", gone, resp.StatusCode)
		}
	}
}

// readFile reads one file as it stands.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestMissionSavesRefusesAndKeepsEveryOtherLine: the mission is the one field
// this stage writes. A save changes one key and leaves the rest of the file as
// it was; a mission over one line is refused, a file that moved under the page
// is refused, and a write without the token of the run never reaches the
// handler (設計 3.4・7.3・8 節).
func TestMissionSavesRefusesAndKeepsEveryOtherLine(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	before := readFile(t, path)

	resp, body := f.post(t, server, url.Values{
		"mission": {"新しい使命です"},
		"hash":    {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, "書きました") {
		t.Errorf("the save shows no saved line:\n%s", body)
	}
	after := readFile(t, path)
	if !strings.Contains(after, "mission: 新しい使命です") {
		t.Errorf("the mission was not written:\n%s", after)
	}
	if want := strings.Replace(before, "mission: demo の使命", "mission: 新しい使命です", 1); after != want {
		t.Errorf("the save changed more than the mission:\n got %q\nwant %q", after, want)
	}

	// A mission over one line cannot be written into this shape, and the file
	// is left alone.
	resp, body = f.post(t, server, url.Values{
		"mission": {"one\ntwo"},
		"hash":    {source.Hash(after)},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `class="findings"`) {
		t.Errorf("a two-line mission was not refused: %d\n%s", resp.StatusCode, body)
	}
	if readFile(t, path) != after {
		t.Errorf("a refused save wrote the file")
	}

	// A mission the file itself writes over one line is shown with validate's
	// sentence, and a save is refused while it stands.
	broken := strings.Replace(after, "mission: 新しい使命です", "mission: \"one\\ntwo\"", 1)
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, body := f.get(t, server, "/definitions/demo"); !strings.Contains(body, "1 行") {
		t.Errorf("a two-line mission in the file is not shown with validate's sentence:\n%s", body)
	}
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}

	// A file that moved under the page is refused too.
	_, body = f.post(t, server, url.Values{
		"mission": {"x"},
		"hash":    {"deadbeef"},
	})
	if !strings.Contains(body, "変わりました") {
		t.Errorf("a stale page was not refused with the clash line:\n%s", body)
	}

	// No token: the handler never runs.
	resp, _ = f.post(t, server, url.Values{"mission": {"x"}, "token": {""}})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a write without the token answered %d, want 403", resp.StatusCode)
	}
}

// TestWebNamesNoLayer: the new package passes no layer to the model, makes no
// catalog of its own, and never reads through the disk rather than the
// catalog's reader (設計 8 節の「口の守り」).
func TestWebNamesNoLayer(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		text := readFile(t, name)
		for _, gone := range []string{"Layer:", "source.LayerLocal", "source.LayerGlobal", "catalog.New(", "source.Disk{}"} {
			if strings.Contains(text, gone) {
				t.Errorf("%s holds %q", name, gone)
			}
		}
	}
}

// repoRoot is the module root, found by walking up to go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
}

// maxDefinitionBytes and maxActivityBytes are the ceilings the two heaviest
// bodies are held to. They were measured over this project's own hergy-dev when
// the screens landed, and the ratchet moves them down only.
const (
	maxDefinitionBytes = 24_000
	maxActivityBytes   = 12_000
)

// TestBodySizeOverHergyDev: the definition detail and the activity are measured
// over this project's own definition, and the ceiling moves down only (設計 8 節
// の「文字数の上限」, 追加の決め 6).
func TestBodySizeOverHergyDev(t *testing.T) {
	root := repoRoot(t)
	cat := catalog.New(t.TempDir(), filepath.Join(root, ".hergy"), root, source.Disk{}, func(p string) string { return p }).InRepo(true)
	if _, err := cat.Read("hergy-dev"); err != nil {
		t.Skipf("this project keeps no readable hergy-dev: %v", err)
	}
	port := &fakePort{f: &fixture{
		running: []effect.Running{{Team: "hergy-dev", Definition: "hergy-dev"}},
		agents:  map[string][]activity.Agent{},
	}}
	server, err := New(cat, port)
	if err != nil {
		t.Fatal(err)
	}
	if _, body := (&fixture{}).get(t, server, "/definitions/hergy-dev"); len(body) > maxDefinitionBytes {
		t.Errorf("the definition detail is %d bytes, over the %d ceiling", len(body), maxDefinitionBytes)
	}
	if _, body := (&fixture{}).get(t, server, "/activity"); len(body) > maxActivityBytes {
		t.Errorf("the activity is %d bytes, over the %d ceiling", len(body), maxActivityBytes)
	}
}
