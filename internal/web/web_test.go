package web

import (
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/topology"
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
	root        string
	proj        string
	global      string
	running     []effect.Running
	agents      map[string][]activity.Agent
	questions   []activity.Question
	questionErr error
	launch      map[string]Launch
	foreign     map[string]*definition.Definition
	foreignView map[string]effect.View
	port        *fakePort
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
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n    args: [--x]\n    env:\n      A: one\n  planner:\n    kind: codex\n")
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

// writeTeam writes one team definition with a member, three workers, a turn to
// the human, a role document for each role, a team agreement and a shared one.
func (f *fixture) writeTeam(t *testing.T, name, mission string) {
	t.Helper()
	f.write(t, filepath.Join("teams", name, "team.yaml"), "version: 1\nmission: "+mission+"\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\nworkers:\n  - from: lead\n    purpose: 実装\n    profile: coder\n  - from: lead\n    purpose: 試作\n    profile: coder\n  - from: lead\n    purpose: 調査\n    profile: planner\n")
	f.write(t, filepath.Join("teams", name, "agreement.yaml"), "version: 1\nsections:\n  - rules:\n      - id: TEAM-1\n        text: このチームの約束。\n        origin:\n          - kind: incident\n            date: \"2026-01-01\"\n")
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
func (f *fixture) post(t *testing.T, server *Server, target string, form url.Values) (*http.Response, string) {
	t.Helper()
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

// fakePort is the live half the screens read, made up by the test. It records
// the panes a press moved to, so a test can say whether focus was called.
type fakePort struct {
	f       *fixture
	focused []string
}

func (p *fakePort) Running() ([]effect.Running, error) { return p.f.running, nil }

func (p *fakePort) Agents(team string) ([]activity.Agent, error) { return p.f.agents[team], nil }

func (p *fakePort) Tail(string, int) (string, error) { return "lead is waiting\n", nil }

func (p *fakePort) Focus(name string) error {
	p.focused = append(p.focused, name)
	return nil
}

func (p *fakePort) Questions() ([]activity.Question, error) {
	return p.f.questions, p.f.questionErr
}

func (p *fakePort) Launched(team string) (Launch, bool) {
	launch, ok := p.f.launch[team]
	return launch, ok
}

func (p *fakePort) LaunchedDefinition(team string) (*definition.Definition, error) {
	if def, ok := p.f.foreign[team]; ok {
		return def, nil
	}
	return nil, errors.New("no launch record")
}

func (p *fakePort) LaunchedView(team string) (effect.View, bool) {
	view, ok := p.f.foreignView[team]
	return view, ok
}

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
		{"/definitions/demo/agreement/new?section=0", "新しい規則"},
		{"/activity", "活動"},
		{"/activity/demo", "チーム demo"},
		{"/roles", "ロール"},
		{"/roles/lead", "ロール lead"},
		{"/roles/lead/LEAD-1", "規則 LEAD-1"},
		{"/roles/lead/new?section=0", "新しい規則"},
		{"/agreement", "共通の約束"},
		{"/agreement/SHARED-1", "規則 SHARED-1"},
		{"/agreement/new?section=0", "新しい規則"},
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
		if strings.Contains(body, "[[") {
			t.Errorf("GET %s shows an unheld locale key (N1)", test.path)
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
		case http.StatusMovedPermanently:
		default:
			t.Errorf("GET %s: status %d, want a page or a move", path, resp.StatusCode)
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

	resp, body := f.post(t, server, "/definitions/demo", url.Values{
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
	resp, body = f.post(t, server, "/definitions/demo", url.Values{
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
	_, body = f.post(t, server, "/definitions/demo", url.Values{
		"mission": {"x"},
		"hash":    {"deadbeef"},
	})
	if !strings.Contains(body, "変わりました") || !strings.Contains(body, "読み直す") {
		t.Errorf("a stale page was not refused with the clash line and a reload link:\n%s", body)
	}

	// No token: the handler never runs.
	resp, _ = f.post(t, server, "/definitions/demo", url.Values{"mission": {"x"}, "token": {""}})
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

// TestPeopleReadNamesNotIdentity: a turn, its ends and the human are written the
// way a person says them — 人間, a member's short name, a worker's purpose — and
// never by the identity a settings file writes (L3, E1).
func TestPeopleReadNamesNotIdentity(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	for _, path := range []string{"/definitions/demo", "/definitions/demo/members/lead", "/definitions/demo/turns/lead/builder", "/definitions/demo/human"} {
		_, body := f.get(t, server, path)
		for _, gone := range []string{"worker:", ">human<", "human ->", "-> lead", "-> builder"} {
			if strings.Contains(body, gone) {
				t.Errorf("GET %s shows the identity %q", path, gone)
			}
		}
	}
	_, body := f.get(t, server, "/definitions/demo")
	for _, want := range []string{"人間 → lead", "lead → builder", "builder → 人間"} {
		if !strings.Contains(body, want) {
			t.Errorf("the definition does not hold %q", want)
		}
	}
	_, body = f.get(t, server, "/definitions/demo/turns/lead/builder")
	if !strings.Contains(body, "lead → builder") {
		t.Errorf("the edge'field is not titled lead → builder")
	}
}

// between is the text of a page from one mark to the next, empty when either is
// missing.
func between(body, from, to string) string {
	i := strings.Index(body, from)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	before, _, ok := strings.Cut(rest, to)
	if !ok {
		return rest
	}
	return before
}

// TestActivityLeavesOutATeamItCannotJudge: a running team whose definition is
// not here gets one line, with no find-the-file sentence and no next command,
// and is in no count (L4).
func TestActivityLeavesOutATeamItCannotJudge(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "crm", Definition: "agile-dev-2"}}
	server := f.server(t)
	resp, body := f.get(t, server, "/activity")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /activity: status %d", resp.StatusCode)
	}
	if !strings.Contains(body, "この場所にチーム定義 agile-dev-2 はありません") {
		t.Errorf("the one line about the missing definition is not there:\n%s", body)
	}
	for _, gone := range []string{"探した場所", "next:", "人間を待っている", "動いているチーム 0"} {
		if strings.Contains(body, gone) {
			t.Errorf("the activity still holds %q", gone)
		}
	}
}

// TestSavedLineDoesNotNameATeamWithNothingToDo: a mission moves no prompt, so a
// running team with nothing to take up is not named; the one line says it
// instead (L5).
func TestSavedLineDoesNotNameATeamWithNothingToDo(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "crm", Definition: "demo"}}
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	_, body := f.post(t, server, "/definitions/demo", url.Values{
		"mission": {"保存後の使命"},
		"hash":    {source.Hash(readFile(t, path))},
	})
	saved := between(body, `class="saved"`, "</div>")
	if !strings.Contains(saved, "動いているチームに渡すものはありません") {
		t.Errorf("the saved line does not say nothing moved:\n%s", saved)
	}
	if strings.Contains(saved, "crm") {
		t.Errorf("the saved line names a team with nothing to do:\n%s", saved)
	}
}

// TestMissingFieldsAreNotFound: a field URL that names something the definition
// does not hold answers 404 with the definition and one line where the field
// would be, rather than a page that opens nothing (N2). The shared agreement and
// a role have their own lines.
func TestMissingFieldsAreNotFound(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct{ path, line string }{
		{"/definitions/demo/members/nosuch", "メンバー nosuch はありません"},
		{"/definitions/demo/workers/lead/nosuch", "ワーカー nosuch（lead が呼ぶ）はありません"},
		{"/definitions/demo/turns/lead/nosuch", "ターン lead → nosuch はありません"},
		{"/definitions/demo/agreement/NOPE", "規則 NOPE はありません"},
		{"/definitions/demo/agreement2", ""},
		{"/roles/nosuch", "ロール nosuch はありません"},
		{"/roles/lead/NOPE", "ロール lead に規則 NOPE はありません"},
		{"/agreement/NOPE", "共通の約束に規則 NOPE はありません"},
	}
	for _, test := range cases {
		resp, body := f.get(t, server, test.path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", test.path, resp.StatusCode)
			continue
		}
		if test.line != "" && !strings.Contains(body, test.line) {
			t.Errorf("GET %s does not say %q", test.path, test.line)
		}
	}
}

// TestTheTrailNamesTheOpenField: a field's page carries the field in the trail,
// with the step before it closing the field (N3).
func TestTheTrailNamesTheOpenField(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct{ path, last string }{
		{"/definitions/demo", "demo"},
		{"/definitions/demo/members/lead", "lead"},
		{"/definitions/demo/turns/lead/builder", "lead → builder"},
		{"/definitions/demo/human", "人間"},
		{"/definitions/demo/agreement/TEAM-1", "TEAM-1"},
		{"/roles/lead/LEAD-1", "LEAD-1"},
	}
	for _, test := range cases {
		_, body := f.get(t, server, test.path)
		trail := between(body, `class="crumb"`, "</nav>")
		if !strings.Contains(trail, "<b>"+test.last+"</b>") {
			t.Errorf("GET %s: the trail %q does not end at %q", test.path, strings.TrimSpace(trail), test.last)
		}
	}
	// The step before the field closes it: it links to the definition's page.
	_, body := f.get(t, server, "/definitions/demo/members/lead")
	trail := between(body, `class="crumb"`, "</nav>")
	if !strings.Contains(trail, `href="/definitions/demo"`) {
		t.Errorf("the trail does not link back to the definition: %q", trail)
	}
}

// TestFocusMovesOnlyAKnownPane: a press moves herdr's focus only to a pane the
// team's own agents hold, and the line it shows belongs to that answer alone
// (N4).
func TestFocusMovesOnlyAKnownPane(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{
		{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 3},
		{Herdr: "demo-hired", Node: "", Status: "working", Seq: 4},
	}
	server := f.server(t)

	resp, body := f.post(t, server, "/activity/demo/focus", url.Values{"pane": {"demo-lead"}})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "チーム demo の lead のペインに移りました") {
		t.Fatalf("the press did not move: %d\n%s", resp.StatusCode, body)
	}
	if len(f.port.focused) != 1 || f.port.focused[0] != "demo-lead" {
		t.Errorf("focus was called with %v, want demo-lead once", f.port.focused)
	}
	// A hired worker is named by its herdr name.
	if _, body := f.post(t, server, "/activity/demo/focus", url.Values{"pane": {"demo-hired"}}); !strings.Contains(body, "チーム demo の demo-hired のペインに移りました") {
		t.Errorf("a hired worker's pane is not named by its herdr name:\n%s", body)
	}
	// A pane that is not the team's is refused, and no focus is moved.
	before := len(f.port.focused)
	_, body = f.post(t, server, "/activity/demo/focus", url.Values{"pane": {"elsewhere"}})
	if !strings.Contains(body, "そのペインはありません") {
		t.Errorf("an unknown pane is not refused:\n%s", body)
	}
	if len(f.port.focused) != before {
		t.Errorf("an unknown pane moved the focus")
	}
	// The moved line is not kept: the next plain read does not carry it.
	if _, body := f.get(t, server, "/activity"); strings.Contains(body, "のペインに移りました") {
		t.Errorf("the moved line was kept on the server:\n%s", body)
	}
}

// TestTwoTeamsKeepTheirLengths: the watcher is handed every running team's
// agents in one call per draw, so reading a second team does not take the first
// team's length again on the next draw (S2).
func TestTwoTeamsKeepTheirLengths(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "a", Definition: "demo"}, {Team: "b", Definition: "demo"}}
	f.agents["a"] = []activity.Agent{{Herdr: "a-lead", Node: "lead", Status: "idle", Seq: 1}}
	f.agents["b"] = []activity.Agent{{Herdr: "b-lead", Node: "lead", Status: "idle", Seq: 1}}
	server := f.server(t)

	start := time.Now()
	_, _, first := server.observeAll(f.running, start)
	later := start.Add(10 * time.Minute)
	agents, errs, second := server.observeAll(f.running, later)

	for _, name := range []string{"a-lead", "b-lead"} {
		if _, ok := first[name]; !ok {
			t.Fatalf("the first draw did not watch %s: %v", name, first)
		}
		if !second[name].At.Equal(start) {
			t.Errorf("the second draw took %s's length again: seen at %s, want %s", name, second[name].At, start)
		}
	}
	// And both teams read a length of at least ten minutes.
	known, byName, _ := server.sections(f.running, agents, errs, second, nil, later)
	if len(known) != 2 {
		t.Fatalf("the draw judged %d teams, want 2", len(known))
	}
	for _, verdict := range known {
		section := byName[verdict.Name]
		if section.Unjudged {
			t.Errorf("team %s was not judged", verdict.Name)
		}
		if verdict.Stalled < 10*time.Minute {
			t.Errorf("team %s reads %s, want at least 10 minutes", verdict.Name, verdict.Stalled)
		}
		if !verdict.Lower {
			t.Errorf("team %s lost the lower-bound mark", verdict.Name)
		}
	}
}

// TestASaveIsRefusedOnlyForANewStop: a stop the definition already had does not
// keep a person from writing the mission (設計 7.3、T1).
func TestASaveIsRefusedOnlyForANewStop(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	// The role document is gone, so the definition already has a stop; the
	// mission can still be written.
	if err := os.Remove(filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	resp, body := f.post(t, server, "/definitions/demo", url.Values{
		"mission": {"それでも書ける使命"},
		"hash":    {source.Hash(readFile(t, path))},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, "書きました") {
		t.Errorf("the save was refused for a stop that was already there:\n%s", body)
	}
	if !strings.Contains(readFile(t, path), "mission: それでも書ける使命") {
		t.Errorf("the mission was not written")
	}
}

// TestTheKindIsAnAttributeInFullWidthParentheses: the profile's kind is written
// the same way wherever it stands (T3).
func TestTheKindIsAnAttributeInFullWidthParentheses(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo")
	if !strings.Contains(body, "（claude）") {
		t.Errorf("the kind is not in full-width parentheses:\n%s", body)
	}
	if strings.Contains(body, "(claude)") {
		t.Errorf("the kind is in half-width parentheses somewhere")
	}
}

// TestTheActivityCountRedrawsWithTheList: the numbers sit inside the region the
// tick replaces, and the region asks for its tick from the script rather than
// polling while the tab is hidden (N9).
func TestTheActivityCountRedrawsWithTheList(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 3}}
	server := f.server(t)
	_, body := f.get(t, server, "/activity")
	start := strings.Index(body, `id="activity-list"`)
	if start < 0 {
		t.Fatalf("the activity list is not there")
	}
	end := strings.Index(body[start:], "</div>")
	if end < 0 {
		t.Fatalf("the activity list is not closed")
	}
	region := body[start : start+end]
	if !strings.Contains(region, `class="count"`) {
		t.Errorf("the numbers are outside the region that redraws:\n%s", region)
	}
	if !strings.Contains(region, `hx-trigger="hergy-tick"`) || !strings.Contains(region, `data-poll="5000"`) {
		t.Errorf("the region does not redraw on the visible tick:\n%s", region)
	}
}

// TestWordsComeFromTheGlossary: the kind is a profile's attribute and is called
// kind, and an origin's kind is said the way validate counts it (N5, N6).
func TestWordsComeFromTheGlossary(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/profiles")
	if !strings.Contains(body, ">kind<") {
		t.Errorf("the profiles list does not call the kind kind:\n%s", body)
	}
	if strings.Contains(body, "種類") {
		t.Errorf("the profiles list says 種類 for kind")
	}
	_, body = f.get(t, server, "/definitions/demo/agreement/TEAM-1")
	if !strings.Contains(body, ">インシデント</option>") {
		t.Errorf("an origin of the incident kind is not said as インシデント:\n%s", body)
	}
	// The kind a settings file writes travels as the value of the card's own
	// choice; what a person reads is the word beside it (N6, 段 4).
	if strings.Contains(body, ">incident<") {
		t.Errorf("an origin shows its kind raw")
	}
}

// TestAMissingDefinitionSaysItWithoutTheNextCommand: the find-the-file sentence
// is shown, without the CLI's next line (N7).
func TestAMissingDefinitionSaysItWithoutTheNextCommand(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	resp, body := f.get(t, server, "/definitions/nosuch")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET a missing definition: status %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(body, "チーム定義 nosuch") {
		t.Errorf("the page does not name the missing definition")
	}
	if strings.Contains(body, "next:") {
		t.Errorf("the page shows the CLI's next line")
	}
}

// TestNoSharedAgreementSaysSoInTheListToo: the list's first row and the shared
// agreement's page agree that there is none (N8).
func TestNoSharedAgreementSaysSoInTheListToo(t *testing.T) {
	f := newFixture(t)
	if err := os.Remove(filepath.Join(f.proj, ".hergy", "agreement.yaml")); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	_, body := f.get(t, server, "/")
	if !strings.Contains(body, "共通の約束はありません") {
		t.Errorf("the list does not say there is no shared agreement:\n%s", body)
	}
	if strings.Contains(body, "規則 0 件") {
		t.Errorf("the list says the shared agreement holds no rules")
	}
}

// TestTheFigureCarriesTheEdgeEnds: a turn's arrow carries its two ends, for the
// edge's own screen and stage 3 (設計 4 節、T2).
func TestTheFigureCarriesTheEdgeEnds(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo")
	if !strings.Contains(body, `data-from="lead" data-to="builder"`) {
		t.Errorf("the turn's arrow does not carry its ends")
	}
	if !strings.Contains(body, `data-from="builder" data-to="human"`) {
		t.Errorf("the turn to the human does not carry its ends")
	}
	// Every arrow links to a field that opens: walking the links is what
	// TestEveryLinkResolves does (S1).
	if !strings.Contains(body, `href="/definitions/demo/workers/lead/`) {
		t.Errorf("a worker's arrow does not open the worker's field")
	}
}

// TestAMissingPlaceSaysItOnce: a field the definition does not hold names the
// place in its title and says it is not there once, in the body (R2), and a
// missing definition or profile is a 404 too (R3).
func TestAMissingPlaceSaysItOnce(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct{ path, title, line string }{
		{"/definitions/demo/members/nosuch", "メンバー nosuch", "メンバー nosuch はありません"},
		{"/definitions/demo/workers/lead/nosuch", "ワーカー nosuch（lead が呼ぶ）", "ワーカー nosuch（lead が呼ぶ）はありません"},
		{"/definitions/demo/turns/lead/nosuch", "lead → nosuch", "ターン lead → nosuch はありません"},
	}
	for _, test := range cases {
		resp, body := f.get(t, server, test.path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", test.path, resp.StatusCode)
			continue
		}
		if !strings.Contains(body, `<h2 class="panel-title">`+test.title+`</h2>`) {
			t.Errorf("GET %s: the title is not %q", test.path, test.title)
		}
		if got := strings.Count(body, test.line); got != 1 {
			t.Errorf("GET %s: the sentence appears %d times, want once", test.path, got)
		}
	}
	for _, path := range []string{"/definitions/nosuch", "/profiles/nosuch"} {
		if resp, _ := f.get(t, server, path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", path, resp.StatusCode)
		}
	}
}

// TestTheDefinitionShowsWhatStopsIt: a stop the check writes is shown where its
// field is — the member's row and field, and the mission's field — so the
// detail says what validate says (R1).
func TestTheDefinitionShowsWhatStopsIt(t *testing.T) {
	f := newFixture(t)
	// The role document is gone, so the check stops on the member that reads it.
	if err := os.Remove(filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	want := "builder.yaml"
	_, body := f.get(t, server, "/definitions/demo")
	if !strings.Contains(body, want) {
		t.Errorf("the detail does not say the role document is missing:\n%s", body)
	}
	// The member's own field says it too.
	_, body = f.get(t, server, "/definitions/demo/members/builder")
	if !strings.Contains(body, want) {
		t.Errorf("the member's field does not say the role document is missing:\n%s", body)
	}
	// And the list's row says the definition cannot be started.
	_, body = f.get(t, server, "/")
	if !strings.Contains(body, want) {
		t.Errorf("the list does not say the definition cannot be started:\n%s", body)
	}
	// The row carries no counts: it is not a definition that reads clean.
	if strings.Contains(body, `mission: `) {
		t.Errorf("the list shows a broken row like a whole one")
	}
}

// asked is one question gy holds, as the port hands it in.
func asked(id, title, writer string, at time.Time) activity.Question {
	return activity.Question{
		ID: id, Title: title, Options: []string{"A 案", "B 案"},
		Decider: "po", At: at, Writer: writer,
	}
}

// TestActivityShowsTheProjectsQuestions: gy's open questions stand at the top of
// the activity as this project's questions, with their options and who decides
// them (設計 3.1・5.3). A question asked by a writer no team carries is not on
// any team's own section.
func TestActivityShowsTheProjectsQuestions(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3}}
	f.questions = []activity.Question{asked("q-1", "どちらへ進むか", "hergy-redesign-lead", time.Now())}
	server := f.server(t)

	_, body := f.get(t, server, "/activity")
	for _, want := range []string{"このプロジェクトの問い", "どちらへ進むか", "選択肢: A 案、B 案", "決める人 po"} {
		if !strings.Contains(body, want) {
			t.Errorf("the activity does not hold %q:\n%s", want, body)
		}
	}
	card := between(body, `class="teamcard"`, "</section>")
	if strings.Contains(card, "どちらへ進むか") {
		t.Errorf("a question of another writer stands on the team's own section:\n%s", card)
	}
}

// TestATeamsOwnWriterShowsTheQuestionInItsSection: a question asked by one of a
// team's herdr names stands on that team's own section too (W4).
func TestATeamsOwnWriterShowsTheQuestionInItsSection(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3}}
	f.questions = []activity.Question{asked("q-1", "どちらへ進むか", "demo-lead", time.Now())}
	server := f.server(t)

	_, body := f.get(t, server, "/activity")
	card := between(body, `class="teamcard"`, "</section>")
	if !strings.Contains(card, "どちらへ進むか") {
		t.Errorf("the team's own section does not carry the question its writer asked:\n%s", card)
	}
}

// TestActivitySaysWhenGyCannotBeRead: a gy that is there and cannot be read is
// one line where the questions would stand, and the teams are still drawn
// (設計 3.1, I17).
func TestActivitySaysWhenGyCannotBeRead(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3}}
	f.questionErr = errors.New("boom")
	server := f.server(t)

	_, body := f.get(t, server, "/activity")
	if !strings.Contains(body, "gy の問いを読めません") || !strings.Contains(body, "boom") {
		t.Errorf("the activity does not say gy could not be read:\n%s", body)
	}
	if !strings.Contains(body, "チーム demo") {
		t.Errorf("the teams are missing because gy failed:\n%s", body)
	}
}

// TestTheTeamDetailShowsTheLaunchAndWhatIsUndelivered: the team's detail carries
// the launch time and one line per member whose prompt changed since the launch,
// with the handover, and shows nothing of the difference (C9, W9). The prompts
// are rebuilt from the facts the record kept, and a record from before those
// were kept shows no change at all (spec/43 S1).
func TestTheTeamDetailShowsTheLaunchAndWhatIsUndelivered(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	f.agents["demo"] = []activity.Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3}}
	f.launch = map[string]Launch{"demo": {
		At:         time.Date(2026, 10, 17, 9, 12, 0, 0, time.Local),
		Prompts:    map[string]string{"lead": "deadbeef"},
		Executable: "/usr/local/bin/hergy",
		Project:    f.proj,
		Lang:       "ja",
	}}
	server := f.server(t)

	_, body := f.get(t, server, "/activity/demo")
	if !strings.Contains(body, "起動 10/17 09:12") {
		t.Errorf("the detail does not show when the team was launched:\n%s", body)
	}
	if !strings.Contains(body, "起動のあとで変わったもの: lead のプロンプト") {
		t.Errorf("the detail does not name the member whose prompt changed:\n%s", body)
	}
	if !strings.Contains(body, "hergy reprompt lead --team demo") {
		t.Errorf("the detail shows no handover for the change:\n%s", body)
	}
	if strings.Contains(body, "deadbeef") {
		t.Errorf("the detail shows a hash, where only the member's name belongs")
	}

	// A record from before the facts were kept cannot be rebuilt, so nothing is
	// said about a change: a wrong line is worse than none.
	f.launch["demo"] = Launch{At: time.Date(2026, 10, 17, 9, 12, 0, 0, time.Local), Prompts: map[string]string{"lead": "deadbeef"}}
	_, body = f.get(t, server, "/activity/demo")
	if !strings.Contains(body, "起動 10/17 09:12") {
		t.Errorf("an old record shows no launch time:\n%s", body)
	}
	if strings.Contains(body, "起動のあとで変わったもの") {
		t.Errorf("an old record shows an undelivered change it cannot know:\n%s", body)
	}
}

// foreignTeam reads one definition of another project, the way the port hands
// one in (W16).
func foreignTeam(t *testing.T, name, mission string) *definition.Definition {
	t.Helper()
	yaml := "version: 1\nmission: " + mission + "\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> human\n"
	def, err := definition.Read(definition.Input{
		YAML:         yaml,
		Name:         name,
		Layer:        source.LayerLocal,
		TeamYAMLPath: "/elsewhere/.hergy/teams/" + name + "/team.yaml",
		TeamDir:      "/elsewhere/.hergy/teams/" + name,
		Profiles:     map[string]profile.Profile{"coder": {Name: "coder", Kind: "claude"}},
		Base:         "/elsewhere",
		Reader:       source.Disk{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return def
}

// TestATeamFromAnotherProjectIsJudgedAndCounted: a team whose definition this
// place does not hold is read from the project its launch record names, judged
// like any other and counted, with no link to a definition page this place
// cannot answer, and with the place that project stands named beside its
// definition (W16, spec/43 N3).
func TestATeamFromAnotherProjectIsJudgedAndCounted(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "crm", Definition: "agile"}}
	f.agents["crm"] = []activity.Agent{{Herdr: "crm-lead", Node: "lead", Status: "blocked", Seq: 5}}
	f.foreign = map[string]*definition.Definition{"crm": foreignTeam(t, "agile", "crm の使命")}
	f.launch = map[string]Launch{"crm": {Project: "/elsewhere/crm"}}
	server := f.server(t)

	resp, body := f.get(t, server, "/activity")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /activity: status %d", resp.StatusCode)
	}
	if !strings.Contains(body, "人間を待っているチーム 1・動いているチーム 1") {
		t.Errorf("the team from another project is not counted:\n%s", body)
	}
	if !strings.Contains(body, "止まっています") {
		t.Errorf("the team from another project is not judged:\n%s", body)
	}
	if !strings.Contains(body, "（agile から起動・/elsewhere/crm）") {
		t.Errorf("the section does not name the project the definition was read from:\n%s", body)
	}
	if strings.Contains(body, "この場所にチーム定義") {
		t.Errorf("the team was not read from its own project:\n%s", body)
	}

	_, body = f.get(t, server, "/activity/crm")
	if !strings.Contains(body, "チーム crm") {
		t.Errorf("the team's detail does not open:\n%s", body)
	}
	if !strings.Contains(body, "（agile から起動・/elsewhere/crm）") {
		t.Errorf("the detail does not name the project the definition was read from:\n%s", body)
	}
	if strings.Contains(body, "/definitions/agile") {
		t.Errorf("the detail links to a definition page this place cannot answer:\n%s", body)
	}
}

// TestTheTeamLineReadsWithNoStraySpace is spec/43 N2: the team's second line is
// the definition it was booted from and the launch time, with nothing between
// them but the separator; a link to the definition stands on its own line, so no
// tag stands between the two parts of the line.
func TestTheTeamLineReadsWithNoStraySpace(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "exp", Definition: "demo"}}
	f.agents["exp"] = []activity.Agent{{Herdr: "exp-lead", Node: "lead", Status: "idle", Seq: 3}}
	f.launch = map[string]Launch{"exp": {
		At:         time.Date(2026, 10, 17, 9, 20, 0, 0, time.Local),
		Prompts:    map[string]string{"lead": "x"},
		Executable: "/usr/local/bin/hergy",
		Project:    f.proj,
		Lang:       "ja",
	}}
	server := f.server(t)

	_, body := f.get(t, server, "/activity/exp")
	if got, want := between(body, `class="sub"`, "</p>"), `class="sub">（demo から起動）・10/17 09:20`; got != want {
		t.Errorf("the team's line is\n%q\nwant\n%q", got, want)
	}
	// The definition's own page is its own line: it is reachable, and it does
	// not sit inside the line above.
	if !strings.Contains(body, `<p class="sub"><a href="/definitions/demo">チーム定義 demo</a></p>`) {
		t.Errorf("the definition's page is not linked on its own line:\n%s", body)
	}
}

// TestTheQuestionTimeIsReadInThisMachinesZone is spec/43 N1: gy writes a
// question's time in UTC, and the screen reads it in the zone this machine is
// in, the way it reads a launch record's time.
func TestTheQuestionTimeIsReadInThisMachinesZone(t *testing.T) {
	was := time.Local
	time.Local = time.FixedZone("test", 9*60*60)
	t.Cleanup(func() { time.Local = was })

	asked := time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)
	if got, want := stamp(asked), "10/17 11:00"; got != want {
		t.Errorf("stamp of a UTC time = %q, want %q", got, want)
	}
}

// TestAnUnjudgedTeamSaysItOnce is walk-6 P1: a team whose definition is not here
// gets one line, and the team's name and the definition it was started from are
// not repeated from the line above it.
func TestAnUnjudgedTeamSaysItOnce(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "crm", Definition: "agile-dev-2"}}
	server := f.server(t)

	_, body := f.get(t, server, "/activity")
	card := between(body, `class="teamcard"`, "</section>")
	if !strings.Contains(card, "この場所にチーム定義 agile-dev-2 はありません") {
		t.Errorf("the one line about the missing definition is not there:\n%s", card)
	}
	if got := strings.Count(card, "agile-dev-2"); got != 2 {
		// Once in the section's own line and once in the one line below it.
		t.Errorf("the definition is named %d times in the section, want twice:\n%s", got, card)
	}
	if strings.Contains(card, "チーム crm（") || strings.Contains(card, "（agile-dev-2 から起動）。") {
		t.Errorf("the one line repeats the line above it:\n%s", card)
	}
}

// teamYAML is the team.yaml of the fixture's demo definition.
func (f *fixture) teamYAML() string {
	return filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
}

// savedLine is the line one save wrote, and what it says the change does to the
// members' prompts (C2).
func savedLine(body string) string { return between(body, `class="saved"`, "</div>") }

// TestAMembersRoleSavesAndKeepsEveryOtherLine: the role is written from the
// field (C1), the saved line says what the change moves, and a running team gets
// a command to take the change up (C2, 設計 3.4).
func TestAMembersRoleSavesAndKeepsEveryOtherLine(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"role"},
		"role": {"builder"},
		"hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if want := strings.Replace(before, "role: lead", "role: builder", 1); readFile(t, path) != want {
		t.Errorf("the save changed more than the role:\n got %q\nwant %q", readFile(t, path), want)
	}
	saved := savedLine(body)
	if !strings.Contains(saved, "書きました") {
		t.Errorf("the save shows no saved line: %q", saved)
	}
	if !strings.Contains(saved, "『ロールの文書』が変わります") || !strings.Contains(saved, "『名簿』が変わります") {
		t.Errorf("the saved line does not say what the change moves: %q", saved)
	}
	if !strings.Contains(saved, "hergy reprompt --all --team demo") {
		t.Errorf("the saved line does not say how a running team takes it up: %q", saved)
	}
	// The field is open again and reads the new value (C3).
	if !strings.Contains(body, `data-origin="builder"`) {
		t.Errorf("the open field does not read the value just written")
	}
}

// TestAMembersProfileSavesAndKeepsEveryOtherLine: a profile is written by name
// alone (I7), and the saved line says how the running team takes it up.
func TestAMembersProfileSavesAndKeepsEveryOtherLine(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":      {"profile"},
		"profile": {"planner"},
		"hash":    {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	want := strings.Replace(before, "name: lead\n    role: lead\n    profile: coder", "name: lead\n    role: lead\n    profile: planner", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the save changed more than the profile:\n got %q\nwant %q", got, want)
	}
	saved := savedLine(body)
	if !strings.Contains(saved, "書きました") {
		t.Errorf("the save shows no saved line: %q", saved)
	}
	// A profile that moves the kind and the env is a down and up: the pane is
	// the only place the env reaches (設計 3.0 の表).
	if !strings.Contains(saved, "hergy down demo") || !strings.Contains(saved, "hergy up demo --as demo") {
		t.Errorf("the saved line does not say the pane has to be made again: %q", saved)
	}
}

// TestAddingAndRemovingAWorker: a worker is hired and taken away from the
// member's field (設計 3.4 の表). The press that takes it away says what the hire
// loses before anything is written (I6), and a purpose written blank is refused
// with validate's sentence.
func TestAddingAndRemovingAWorker(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":      {"worker-add"},
		"purpose": {"検証"},
		"profile": {"planner"},
		"hash":    {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the hire answered %d", resp.StatusCode)
	}
	added := readFile(t, path)
	if !strings.HasSuffix(added, "  - from: lead\n    purpose: 検証\n    profile: planner\n") {
		t.Errorf("the hire did not write the entry in the file's own shape:\n%s", added)
	}
	if !strings.Contains(savedLine(body), "の「ワーカー 検証」を書きました。") {
		t.Errorf("the saved line does not name the worker's key by its purpose: %q", savedLine(body))
	}
	if !strings.Contains(savedLine(body), "『呼べるワーカー』に 検証 が入ります") {
		t.Errorf("the saved line does not say the worker was added: %q", savedLine(body))
	}

	// The press that chooses the removal shows what will be lost first (I6), and
	// writes nothing.
	_, body = f.get(t, server, "/definitions/demo/members/lead?remove="+url.QueryEscape("検証"))
	if !strings.Contains(body, "hergy hire --purpose 検証 は断られます") {
		t.Errorf("the removal does not say what the hire loses:\n%s", body)
	}
	if !strings.Contains(body, `class="confirm"`) {
		t.Errorf("the removal offers no way to write it")
	}
	if readFile(t, path) != added {
		t.Errorf("the press wrote the file")
	}

	// Writing it takes the entry away, leaving the file as it was.
	_, body = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":     {"worker-remove"},
		"worker": {"検証"},
		"hash":   {source.Hash(added)},
	})
	if got := readFile(t, path); got != before {
		t.Errorf("the removal left\n got %q\nwant %q", got, before)
	}
	if !strings.Contains(savedLine(body), "の「ワーカー 検証」を書きました。") {
		t.Errorf("the saved line does not name the worker's key by its purpose: %q", savedLine(body))
	}
	if !strings.Contains(savedLine(body), "『呼べるワーカー』から 検証 が消えます") {
		t.Errorf("the saved line does not say the worker was taken away: %q", savedLine(body))
	}

	// A purpose written blank is refused in the field's own words, and nothing
	// is written (N2).
	resp, body = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":      {"worker-add"},
		"purpose": {"  "},
		"profile": {"planner"},
		"hash":    {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "目的を書いてください") {
		t.Errorf("a blank purpose was not refused in the field's own words: %d\n%s", resp.StatusCode, body)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused hire wrote the file")
	}
}

// TestRemovingAWorkerFromItsOwnFieldLandsOnTheMember: the field a worker stood in
// is gone once the worker is, so the write lands on the member that hired it —
// the field that still holds the worker list — rather than on a field that says
// the worker it just took away is not there (N1).
func TestRemovingAWorkerFromItsOwnFieldLandsOnTheMember(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	// The press that chooses the removal names the member's field as where the
	// write goes.
	_, body := f.get(t, server, "/definitions/demo/workers/lead/試作?remove="+url.QueryEscape("試作"))
	confirm := between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, `action="/definitions/demo/members/lead"`) {
		t.Errorf("the removal does not write to the member's field:\n%s", confirm)
	}
	if !strings.Contains(confirm, "hergy hire --purpose 試作 は断られます") {
		t.Errorf("the removal does not say what the hire loses:\n%s", confirm)
	}

	// Writing it draws the member's field, with the saved line, and never says
	// the worker it just took away is not there.
	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op": {"worker-remove"}, "worker": {"試作"}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the removal answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, `<h2 class="panel-title">メンバー lead</h2>`) {
		t.Fatalf("the removal did not land on the member's field:\n%s", body)
	}
	if !strings.Contains(savedLine(body), "の「ワーカー 試作」を書きました。") {
		t.Errorf("the member's field shows no saved line: %q", savedLine(body))
	}
	if strings.Contains(body, "にワーカー 試作") {
		t.Errorf("the page says the worker it just took away is not there:\n%s", body)
	}
	after := readFile(t, path)
	for _, kept := range []string{"purpose: 実装", "purpose: 調査"} {
		if !strings.Contains(after, kept) {
			t.Errorf("the removal took more than the one worker: %s", kept)
		}
	}
	if strings.Contains(after, "purpose: 試作") {
		t.Errorf("the removal wrote nothing")
	}

	// A write that reaches the worker's own URL by hand lands there too.
	resp, body = f.post(t, server, "/definitions/demo/workers/lead/調査", url.Values{
		"op": {"worker-remove"}, "hash": {source.Hash(after)},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `<h2 class="panel-title">メンバー lead</h2>`) {
		t.Errorf("the worker's own field did not land on the member's: %d", resp.StatusCode)
	}
}

// TestTheWorkerRowNamesThePurpose: each worker's row names the purpose it is
// hired for, as a link to its own field, so a person can tell which row writes
// which worker (S1).
func TestTheWorkerRowNamesThePurpose(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead")
	rows := between(body, `<dt>ワーカー</dt>`, `<dt>ワーカーを足す</dt>`)
	if rows == "" {
		t.Fatalf("the member's field holds no worker row:\n%s", body)
	}
	for _, purpose := range []string{"実装", "試作", "調査"} {
		want := `<a href="` + workerPath("demo", "lead", purpose) + `">` + purpose + `</a>`
		if !strings.Contains(rows, want) {
			t.Errorf("no worker row names %q as a link to its own field:\n%s", purpose, rows)
		}
	}
	// And the field that hires one says what to write there (S1).
	if !strings.Contains(body, `<label class="fl">目的 <input type="text" name="purpose"`) {
		t.Errorf("the purpose input of the hire has no label:\n%s", body)
	}
}

// TestAnEmptyInputIsRefusedInTheFieldsOwnWords: an input a person left empty is
// refused by the field's own name, never with the place a broken file would be
// told about (N2). What the file itself holds is still validate's sentence, from
// the check.
func TestAnEmptyInputIsRefusedInTheFieldsOwnWords(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	hash := source.Hash(before)
	cases := []struct {
		target, want string
		form         url.Values
	}{
		{target: "/definitions/demo/members/lead", want: "ロールを選んでください", form: url.Values{"op": {"role"}, "role": {""}, "hash": {hash}}},
		{target: "/definitions/demo/members/lead", want: "プロファイルを選んでください", form: url.Values{"op": {"profile"}, "profile": {""}, "hash": {hash}}},
		{target: "/definitions/demo/members/lead", want: "目的を書いてください", form: url.Values{"op": {"worker-add"}, "purpose": {""}, "profile": {"coder"}, "hash": {hash}}},
		{target: "/definitions/demo/workers/lead/試作", want: "プロファイルを選んでください", form: url.Values{"op": {"worker-profile"}, "profile": {""}, "hash": {hash}}},
	}
	for _, test := range cases {
		resp, body := f.post(t, server, test.target, test.form)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, test.want) {
			t.Errorf("POST %s: %d, want %q\n%s", test.target, resp.StatusCode, test.want, body)
		}
		for _, internal := range []string{"workers[", "members["} {
			if strings.Contains(body, internal) {
				t.Errorf("POST %s: the refusal shows the file's place %q", test.target, internal)
			}
		}
	}
	if readFile(t, path) != before {
		t.Errorf("a refused input wrote the file")
	}
}

// TestTheSavedLineNamesTheKeyInItsOwnQuotes: the saved line names the key it
// wrote between the language's own quotes, and a page-level field (the mission)
// is named the same way as a field of a panel (N3).
func TestTheSavedLineNamesTheKeyInItsOwnQuotes(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"role"},
		"role": {"builder"},
		"hash": {source.Hash(before)},
	})
	if saved := savedLine(body); !strings.Contains(saved, "の「ロール」を書きました。") {
		t.Errorf("the saved line names the key as %q", saved)
	}

	written := readFile(t, path)
	_, body = f.post(t, server, "/definitions/demo", url.Values{
		"mission": {"新しい使命"},
		"hash":    {source.Hash(written)},
	})
	saved := between(body, `class="saved"`, "</div>")
	if !strings.Contains(saved, "の「使命」を書きました。") {
		t.Errorf("the mission's saved line names the key as %q", saved)
	}
	if !strings.Contains(saved, "どのメンバーのプロンプトも変わりません") {
		t.Errorf("the mission's saved line says nothing about the prompts: %q", saved)
	}
}

// TestPickingAValueShowsWhatItWouldDo: a pick that is not written yet shows what
// it would do to the members' prompts beside the field, and writes nothing (T1).
// A pick that would be refused shows the stop instead, and offers no write.
func TestPickingAValueShowsWhatItWouldDo(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.get(t, server, "/definitions/demo/members/lead?preview=role&role=builder")
	role := between(body, `data-key="role"`, "</form>")
	if !strings.Contains(role, "『ロールの文書』が変わります") {
		t.Errorf("the pick shows no line about what it would do:\n%s", role)
	}
	if !strings.Contains(role, `data-origin="lead"`) || !strings.Contains(body, `value="builder" selected`) {
		t.Errorf("the field does not read the picked value and keep the file's own:\n%s", role)
	}
	if !strings.Contains(body, "保存する") {
		t.Errorf("the pick offers no way to write it:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("the preview wrote the file")
	}

	// A pick on a worker's own profile is looked at the same way, and names the
	// worker it belongs to.
	_, body = f.get(t, server, "/definitions/demo/members/lead?preview=worker-profile&worker="+url.QueryEscape("試作")+"&profile=planner")
	worker := between(body, `data-key="worker/lead/試作"`, "</form>")
	if !strings.Contains(worker, "『呼べるワーカー』の 試作 のプロファイルが変わります") {
		t.Errorf("a worker's pick shows no line about what it would do:\n%s", worker)
	}
	if readFile(t, path) != before {
		t.Errorf("the preview wrote the file")
	}

	// A pick that would stop the definition says so, and takes the write away
	// (the way the save road does).
	if err := os.Remove(filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")); err != nil {
		t.Fatal(err)
	}
	_, body = f.get(t, server, "/definitions/demo/members/lead?preview=role&role=builder")
	role = between(body, `data-key="role"`, "</form>")
	if !strings.Contains(role, "builder.yaml") {
		t.Errorf("a refused pick does not say what stops it:\n%s", role)
	}
	if strings.Contains(role, `type="submit"`) {
		t.Errorf("a refused pick still offers the write:\n%s", role)
	}
	if !strings.Contains(role, "取り消す") {
		t.Errorf("a refused pick leaves no way out:\n%s", role)
	}
}

// TestAWorkersProfileIsWrittenFromItsOwnField: the worker's own field changes the
// profile, and its table row is the same write (設計 3.4 の表).
func TestAWorkersProfileIsWrittenFromItsOwnField(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/workers/lead/試作", url.Values{
		"op":      {"worker-profile"},
		"profile": {"planner"},
		"hash":    {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	want := strings.Replace(before, "purpose: 試作\n    profile: coder", "purpose: 試作\n    profile: planner", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the save changed more than the worker's profile:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(savedLine(body), "の「ワーカー 試作」を書きました。") {
		t.Errorf("the saved line does not name the worker's key by its purpose: %q", savedLine(body))
	}
	if !strings.Contains(savedLine(body), "『呼べるワーカー』の 試作 のプロファイルが変わります") {
		t.Errorf("the saved line does not say the profile moved: %q", savedLine(body))
	}

	// The same write from the member's field leaves the same file.
	resp, _ = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":      {"worker-profile"},
		"worker":  {"試作"},
		"profile": {"coder"},
		"hash":    {source.Hash(want)},
	})
	if resp.StatusCode != http.StatusOK || readFile(t, path) != before {
		t.Errorf("the member's field did not write the same place: %d", resp.StatusCode)
	}
}

// TestTheMemberFieldOffersTheNamesTheCatalogResolves: the profile is picked by
// name and its kind stands beside the pick as an attribute, never inside the
// choice (I7); the role is picked from the roles hergy resolves.
func TestTheMemberFieldOffersTheNamesTheCatalogResolves(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead")

	profiles := between(body, `name="profile" data-draft="field:demo:members/lead:profile"`, "</select>")
	if got := optionTexts(profiles); !reflect.DeepEqual(got, []string{"coder", "planner"}) {
		t.Errorf("the profile choices are %v, want the profile names alone", got)
	}
	if !strings.Contains(profiles, `data-kind="claude"`) {
		t.Errorf("a profile choice does not carry its kind as an attribute:\n%s", profiles)
	}
	if strings.Contains(profiles, "（") || strings.Contains(profiles, "(") {
		t.Errorf("a choice carries more than the name:\n%s", profiles)
	}
	if !strings.Contains(body, `<span class="kind" data-kind-show>claude</span>`) {
		t.Errorf("the kind of the picked profile does not stand beside the pick")
	}

	roles := between(body, `name="role" data-draft="field:demo:members/lead:role"`, "</select>")
	if got := optionTexts(roles); !reflect.DeepEqual(got, []string{"builder", "lead"}) {
		t.Errorf("the role choices are %v, want the roles hergy resolves", got)
	}
}

// TestEveryWritableFieldKeepsItsDraftAndItsOrigin: every field of a member and
// of a worker carries the key the script keeps its half-written value under and
// the value the file writes, so 1b's writing-in-progress works on this round's
// fields too (C5).
func TestEveryWritableFieldKeepsItsDraftAndItsOrigin(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead")
	for _, want := range []string{
		`data-draft="field:demo:members/lead:role" data-origin="lead"`,
		`data-draft="field:demo:members/lead:profile" data-origin="coder"`,
		`data-draft="field:demo:workers/lead/試作:profile" data-origin="coder"`,
		`data-draft="field:demo:members/lead:worker-add:purpose" data-origin=""`,
		`data-draft="field:demo:members/lead:worker-add:profile" data-origin="coder"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the member's field does not keep %q", want)
		}
	}
}

// TestARoleWithoutADocumentIsRefusedWithValidatesSentence: a save that brings in
// a stop is refused with the sentence the check writes, beside the field, and
// the file is left alone (設計 7.3).
func TestARoleWithoutADocumentIsRefusedWithValidatesSentence(t *testing.T) {
	f := newFixture(t)
	// The role document is gone, so moving a member onto that role would stop
	// the definition.
	if err := os.Remove(filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"role"},
		"role": {"builder"},
		"hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused save wrote the file")
	}
	edited, err := definition.EditYAML([]byte(before), "team.yaml", definition.MemberRole("lead", "builder"))
	if err != nil {
		t.Fatal(err)
	}
	with := f.catalog().With(path, string(edited))
	want := firstStopText(check.Definition(check.Input{Catalog: with, Name: "demo"}))
	if want == "" {
		t.Fatal("the check wrote no stop for the change")
	}
	if !strings.Contains(body, want) {
		t.Errorf("the panel does not say what validate says (%q):\n%s", want, body)
	}
}

// TestTheSavedHandoverIsRealCommands: every command a saved field shows is taken
// apart by the real command definitions, so the line a person copies is one
// hergy takes (I16).
func TestTheSavedHandoverIsRealCommands(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	before := readFile(t, f.teamYAML())

	_, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"role"},
		"role": {"builder"},
		"hash": {source.Hash(before)},
	})
	commands := codeTexts(savedLine(body))
	if len(commands) == 0 {
		t.Fatalf("the saved line holds no command:\n%s", savedLine(body))
	}
	for _, line := range commands {
		argv := strings.Fields(line)
		if len(argv) == 0 || argv[0] != "hergy" {
			t.Errorf("the command %q does not start with hergy", line)
			continue
		}
		if _, err := options.Parse(argv[1:]); err != nil {
			t.Errorf("the command %q does not parse: %v", line, err)
		}
	}
}

// TestAMemberFieldRefusesAStalePageAndNeedsTheToken: a file that moved under the
// page is refused with the clash line and a link that reads it again, and a
// write without the token of the run never reaches the handler (設計 3.0・7.3).
func TestAMemberFieldRefusesAStalePageAndNeedsTheToken(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"role"},
		"role": {"builder"},
		"hash": {"deadbeef"},
	})
	if !strings.Contains(body, "変わりました") || !strings.Contains(body, "読み直す") {
		t.Errorf("a stale page was not refused with the clash line and a reload link:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("a stale page wrote the file")
	}

	resp, _ := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":    {"role"},
		"role":  {"builder"},
		"token": {""},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a write without the token answered %d, want 403", resp.StatusCode)
	}
}

// TestAMissingMemberWriteIsNotFound: a write to a field the definition does not
// hold is the 404 the read is, and nothing is written (N2).
func TestAMissingMemberWriteIsNotFound(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	resp, body := f.post(t, server, "/definitions/demo/members/nosuch", url.Values{
		"op":   {"role"},
		"role": {"lead"},
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("the write answered %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(body, "メンバー nosuch はありません") {
		t.Errorf("the page does not say the member is not there")
	}
}

// optionTexts is the text of every <option> in one select block, so a test can
// say a choice carries the name alone (I7).
func optionTexts(block string) []string {
	var out []string
	for rest := block; ; {
		i := strings.Index(rest, "<option ")
		if i < 0 {
			return out
		}
		_, after, ok := strings.Cut(rest[i:], ">")
		if !ok {
			return out
		}
		text, tail, ok := strings.Cut(after, "</option>")
		if !ok {
			return out
		}
		out = append(out, text)
		rest = tail
	}
}

// codeTexts is the text of every <code> block, in the order the page writes it.
func codeTexts(body string) []string {
	var out []string
	for rest := body; ; {
		i := strings.Index(rest, "<code")
		if i < 0 {
			return out
		}
		_, after, ok := strings.Cut(rest[i:], ">")
		if !ok {
			return out
		}
		text, tail, ok := strings.Cut(after, "</code>")
		if !ok {
			return out
		}
		out = append(out, text)
		rest = tail
	}
}

// firstStopText is the first sentence that stops a definition, which is the one
// a save that brings it in is refused with.
func firstStopText(findings []check.Finding) string {
	for _, finding := range findings {
		if finding.Stops {
			return finding.Text
		}
	}
	return ""
}

// turnTeam writes the demo definition as the I13 example: a lead that hands the
// turn to spec and back to the human, and spec that hands it back to lead.
func (f *fixture) turnTeam(t *testing.T) {
	t.Helper()
	f.write(t, filepath.Join("teams", "demo", "team.yaml"),
		"version: 1\nmission: demo の使命\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: coder\n"+
			"  - name: spec\n    role: spec\n    profile: coder\n"+
			"turns:\n  - human -> lead\n  - lead -> spec\n  - spec -> lead\n  - lead -> human\n")
	f.write(t, filepath.Join("roles", "spec.yaml"), "version: 1\nsections:\n  - rules:\n      - id: SPEC-1\n        text: spec は確かめる。\n")
}

// lastHumanTeam writes a demo definition where one member hands the turn to the
// human, so taking that one turn away leaves the human unreached (I12b) while
// every node keeps a turn out.
func (f *fixture) lastHumanTeam(t *testing.T) {
	t.Helper()
	f.write(t, filepath.Join("teams", "demo", "team.yaml"),
		"version: 1\nmission: demo の使命\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: coder\n"+
			"  - name: builder\n    role: builder\n    profile: coder\n"+
			"turns:\n  - human -> lead\n  - lead -> builder\n  - builder -> lead\n  - builder -> human\n")
}

// memberNode is one member's identity, for an edit made by hand in a test.
func memberNode(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// promptText is the bootstrap prompt one member is handed, the text `hergy up
// --dry` prints: it is built from the definition on disk.
func promptText(t *testing.T, s *Server, name, member string) string {
	t.Helper()
	def, err := s.cat.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := def.Graph.Member(member)
	if !ok {
		t.Fatalf("%s has no member %s", name, member)
	}
	built, err := prompt.Member(def, &node, s.view(def, s.cat.Reader()).Run)
	if err != nil {
		t.Fatal(err)
	}
	return built.Text
}

// leadTurns is the 「ターンを渡す相手」 part of lead's bootstrap prompt: the part
// a turn change moves (I13).
func leadTurns(t *testing.T, s *Server) string {
	t.Helper()
	def, err := s.cat.Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	return prompt.TurnsNote("", prompt.TurnsOf(def, "lead", "demo"))
}

// TestATurnIsAddedFromTheFieldAndKeepsEveryOtherLine: a member's field hands the
// turn to one more party (設計 3.4・4 節). The save adds one line of turns: and
// leaves every other line as it was, and the saved line names the turn and what
// it does to the prompt.
func TestATurnIsAddedFromTheFieldAndKeepsEveryOtherLine(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	resp, body := f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"turn-add"},
		"to":   {"human"},
		"hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the add answered %d", resp.StatusCode)
	}
	want := strings.Replace(before, "  - builder -> human\n", "  - builder -> human\n  - lead -> human\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the add changed more than the one turn:\n got %q\nwant %q", got, want)
	}
	saved := savedLine(body)
	if !strings.Contains(saved, "の「ターン lead → 人間」を書きました。") {
		t.Errorf("the saved line does not name the turn: %q", saved)
	}
	if !strings.Contains(saved, "lead のプロンプトの『ターンを渡す相手』に人間が入ります") {
		t.Errorf("the saved line does not say what the change does: %q", saved)
	}
	if !strings.Contains(body, `<h2 class="panel-title">メンバー lead</h2>`) {
		t.Errorf("the add did not stay on the member's field:\n%s", body)
	}
	if !strings.Contains(body, `<a href="/definitions/demo/turns/lead/human">人間</a>`) {
		t.Errorf("the new turn does not stand in the field as a person reads it:\n%s", body)
	}

	// A form that names no party is refused in the field's own words, and the
	// file is left alone (N2).
	resp, body = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"turn-add"},
		"to":   {""},
		"hash": {source.Hash(want)},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "ターンを渡す相手を選んでください") {
		t.Errorf("a blank party was not refused in the field's own words: %d\n%s", resp.StatusCode, body)
	}
	if readFile(t, path) != want {
		t.Errorf("a refused add wrote the file")
	}
}

// TestATurnIsTakenAwayFromItsRowAndFromItsOwnField: the row's press and the
// edge's own field choose the same removal, and the write lands on the field of
// the party that hands the turn on, because the edge's own field is gone once
// the turn is (設計 3.4・4 節、N1).
func TestATurnIsTakenAwayFromItsRowAndFromItsOwnField(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	// The row's press shows what the removal would do and writes nothing.
	_, body := f.get(t, server, "/definitions/demo/members/lead?turn=builder")
	if !strings.Contains(body, "lead のプロンプトの『ターンを渡す相手』から builder（ロール builder）が消えます") {
		t.Errorf("the removal does not say what it would do:\n%s", body)
	}
	if !strings.Contains(body, `class="confirm"`) {
		t.Errorf("the removal offers no way to write it:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("the press wrote the file")
	}

	// The edge's own field chooses the same removal, and writes to the party's
	// field.
	_, body = f.get(t, server, "/definitions/demo/turns/lead/builder?turn=builder")
	confirm := between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, `action="/definitions/demo/members/lead"`) {
		t.Errorf("the edge's removal does not write to the party's field:\n%s", confirm)
	}
	if !strings.Contains(confirm, "から builder（ロール builder）が消えます") {
		t.Errorf("the edge's removal says nothing about the prompt:\n%s", confirm)
	}

	// Writing it takes out the one line and leaves the rest.
	_, body = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"turn-remove"},
		"to":   {"builder"},
		"hash": {source.Hash(before)},
	})
	want := strings.Replace(before, "  - lead -> builder\n", "", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the removal changed more than the one turn:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(savedLine(body), "の「ターン lead → builder」を書きました。") {
		t.Errorf("the saved line does not name the turn: %q", savedLine(body))
	}
}

// TestTheHumanHandsTheTurnOnToo: the human's own field adds and takes away the
// turns the human hands on, and a human's turn moves no member's prompt (設計
// 3.4 の表、4 節).
func TestTheHumanHandsTheTurnOnToo(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.get(t, server, "/definitions/demo/human?preview=turn-add&to=builder")
	if !strings.Contains(body, "人間が builder にターンを渡せるようになります（プロンプトは変わりません）") {
		t.Errorf("a human's turn does not say what it does on the human's side:\n%s", body)
	}
	if strings.Contains(body, "どのメンバーのプロンプトも変わりません") {
		t.Errorf("a human's turn says only that no prompt moves:\n%s", body)
	}

	_, body = f.post(t, server, "/definitions/demo/human", url.Values{
		"op":   {"turn-add"},
		"to":   {"builder"},
		"hash": {source.Hash(before)},
	})
	want := strings.Replace(before, "  - builder -> human\n", "  - builder -> human\n  - human -> builder\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the human's add changed more than the one turn:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(savedLine(body), "の「ターン 人間 → builder」を書きました。") {
		t.Errorf("the saved line does not name the human's turn: %q", savedLine(body))
	}

	// The removal is chosen from the human's field and writes to it, because
	// the field is the one that still holds the human's turns.
	_, body = f.get(t, server, "/definitions/demo/human?turn=builder")
	confirm := between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, `action="/definitions/demo/human"`) {
		t.Errorf("the human's removal does not write to the human's field:\n%s", confirm)
	}
	_, body = f.post(t, server, "/definitions/demo/human", url.Values{
		"op":   {"turn-remove"},
		"to":   {"builder"},
		"hash": {source.Hash(want)},
	})
	if readFile(t, path) != before {
		t.Errorf("the human's removal did not leave the file as it was:\n%s", readFile(t, path))
	}
	if !strings.Contains(savedLine(body), "の「ターン 人間 → builder」を書きました。") {
		t.Errorf("the saved line does not name the human's turn: %q", savedLine(body))
	}
}

// TestRemovingATurnMovesOnlyThePartyThatHandsIt is the I13 golden on the
// screens: taking lead -> spec away says so beside the row, and the prompt the
// party is handed (`hergy up --dry`) no longer names spec, while every other
// member's prompt is one byte the same.
func TestRemovingATurnMovesOnlyThePartyThatHandsIt(t *testing.T) {
	f := newFixture(t)
	f.turnTeam(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	specBefore := promptText(t, server, "demo", "spec")
	beforeTurns := leadTurns(t, server)
	if !strings.Contains(beforeTurns, "spec") {
		t.Fatalf("lead's prompt does not name spec before the change:\n%s", beforeTurns)
	}

	_, body := f.get(t, server, "/definitions/demo/members/lead?turn=spec")
	if !strings.Contains(body, "lead のプロンプトの『ターンを渡す相手』から spec（ロール spec）が消えます") {
		t.Errorf("the removal does not say what leaves:\n%s", body)
	}

	_, body = f.post(t, server, "/definitions/demo/members/lead", url.Values{
		"op":   {"turn-remove"},
		"to":   {"spec"},
		"hash": {source.Hash(before)},
	})
	if want := strings.Replace(before, "  - lead -> spec\n", "", 1); readFile(t, path) != want {
		t.Errorf("the removal changed more than the one turn:\n got %q\nwant %q", readFile(t, path), want)
	}
	if !strings.Contains(savedLine(body), "の「ターン lead → spec」を書きました。") {
		t.Errorf("the saved line does not name the turn: %q", savedLine(body))
	}
	if afterTurns := leadTurns(t, server); strings.Contains(afterTurns, "spec") {
		t.Errorf("lead's prompt still hands the turn to spec:\n%s", afterTurns)
	}
	if specAfter := promptText(t, server, "demo", "spec"); specAfter != specBefore {
		t.Errorf("spec's prompt moved, but nothing says so:\n got %q\nwant %q", specAfter, specBefore)
	}
}

// TestTheLastTurnIsRefusedWithValidatesSentence: taking the last turn out of a
// node, or the last turn that reaches the human, is refused before anything is
// written, with the sentence `hergy validate` writes, and the field offers no
// write (4 節、I12a・I12b).
func TestTheLastTurnIsRefusedWithValidatesSentence(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	// The last turn out of a member: the choice shows the sentence and takes the
	// write away.
	_, body := f.get(t, server, "/definitions/demo/members/builder?turn=human")
	confirm := between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, "メンバー builder から出るターンがありません") {
		t.Errorf("the choice does not say the last turn cannot go:\n%s", confirm)
	}
	if strings.Contains(confirm, `type="submit"`) {
		t.Errorf("a choice that would stop the definition still offers the write:\n%s", confirm)
	}
	if !strings.Contains(confirm, "取り消す") {
		t.Errorf("a refused choice leaves no way out:\n%s", confirm)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused choice wrote the file")
	}

	// A hand-sent write is refused with the same sentence, and writes nothing.
	edited, err := definition.EditYAML([]byte(before), "team.yaml", definition.RemoveTurn(memberNode("builder"), (definition.Human{}).ID()))
	if err != nil {
		t.Fatal(err)
	}
	with := f.catalog().With(path, string(edited))
	want := firstStopText(check.Definition(check.Input{Catalog: with, Name: "demo"}))
	if want == "" {
		t.Fatal("the check wrote no stop for the change")
	}
	resp, body := f.post(t, server, "/definitions/demo/members/builder", url.Values{
		"op":   {"turn-remove"},
		"to":   {"human"},
		"hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the refused write answered %d", resp.StatusCode)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused write wrote the file")
	}
	if !strings.Contains(body, html.EscapeString(want)) {
		t.Errorf("the field does not say what validate says (%q):\n%s", want, body)
	}

	// The human's own last turn out is refused the same way.
	_, body = f.get(t, server, "/definitions/demo/human?turn=lead")
	confirm = between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, "人間から出るターンがありません") {
		t.Errorf("the human's last turn is not refused with validate's sentence:\n%s", confirm)
	}
	if strings.Contains(confirm, `type="submit"`) {
		t.Errorf("the human's refused choice still offers the write:\n%s", confirm)
	}
}

// TestTheLastTurnToTheHumanIsRefused: taking away the one turn that reaches the
// human is refused with validate's sentence (I12b), even though every node still
// hands the turn on.
func TestTheLastTurnToTheHumanIsRefused(t *testing.T) {
	f := newFixture(t)
	f.lastHumanTeam(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.get(t, server, "/definitions/demo/members/builder?turn=human")
	confirm := between(body, `class="confirm"`, "</form>")
	if !strings.Contains(confirm, "人間へ届くターンがありません") {
		t.Errorf("the choice does not say the human would be unreached:\n%s", confirm)
	}
	if strings.Contains(confirm, `type="submit"`) {
		t.Errorf("a choice that would stop the definition still offers the write:\n%s", confirm)
	}

	edited, err := definition.EditYAML([]byte(before), "team.yaml", definition.RemoveTurn(memberNode("builder"), (definition.Human{}).ID()))
	if err != nil {
		t.Fatal(err)
	}
	with := f.catalog().With(path, string(edited))
	want := firstStopText(check.Definition(check.Input{Catalog: with, Name: "demo"}))
	if !strings.Contains(want, "人間へ届くターンがありません") {
		t.Fatalf("the check's first stop is %q, want the one about the human", want)
	}
	_, body = f.post(t, server, "/definitions/demo/members/builder", url.Values{
		"op":   {"turn-remove"},
		"to":   {"human"},
		"hash": {source.Hash(before)},
	})
	if readFile(t, path) != before {
		t.Errorf("a refused write wrote the file")
	}
	if !strings.Contains(body, html.EscapeString(want)) {
		t.Errorf("the field does not say what validate says (%q):\n%s", want, body)
	}
}

// TestTheFigureCountsOneArrowPerEdge: the figure draws one arrow for each edge
// of the graph, whether or not a field is open (設計 4 節).
func TestTheFigureCountsOneArrowPerEdge(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	def, err := f.catalog().Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/definitions/demo", "/definitions/demo/members/lead", "/definitions/demo/human"} {
		_, body := f.get(t, server, path)
		if got := strings.Count(body, `<path class="edge`); got != len(def.Graph.Edges) {
			t.Errorf("GET %s: the figure draws %d arrows, want %d", path, got, len(def.Graph.Edges))
		}
	}
}

// TestTheHandlesStandOnlyOnTheOpenNodesOtherSide: the "＋" handle is on the
// parties the open node does not hand the turn to yet, on the members and the
// human only, and nowhere when no such field is open (設計 4 節).
func TestTheHandlesStandOnlyOnTheOpenNodesOtherSide(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct {
		path, want, gone string
	}{
		{"/definitions/demo", "", ""},
		{"/definitions/demo/members/lead", `to=human`, `to=builder`},
		{"/definitions/demo/members/builder", `to=lead`, `to=human`},
		{"/definitions/demo/human", `to=builder`, `to=lead`},
		{"/definitions/demo/workers/lead/実装", "", ""},
	}
	for _, test := range cases {
		_, body := f.get(t, server, test.path)
		handles := strings.Count(body, `class="handle"`)
		if test.want == "" {
			if handles != 0 {
				t.Errorf("GET %s: the figure offers %d handles with no party's field open", test.path, handles)
			}
			continue
		}
		if handles != 1 {
			t.Errorf("GET %s: the figure offers %d handles, want one", test.path, handles)
		}
		handle := between(body, `class="handle"`, `<circle`)
		if !strings.Contains(handle, test.want) {
			t.Errorf("GET %s: the handle does not lead to %q (%s)", test.path, test.want, handle)
		}
		if test.gone != "" && strings.Contains(handle, test.gone) {
			t.Errorf("GET %s: a handle stands on a party the node already hands the turn to (%s)", test.path, handle)
		}
		if strings.Contains(body, "to=worker") {
			t.Errorf("GET %s: the figure offers a handle to a worker", test.path)
		}
	}
}

// TestTheTurnPickerPicksNothingUntilAPartyIs: the field that hands the turn on
// offers an empty first choice, so a field that is just drawn writes nothing,
// and names the human the way a person says it (設計 4 節).
func TestTheTurnPickerPicksNothingUntilAPartyIs(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead")
	form := between(body, `data-key="turn-add"`, "</form>")
	if !strings.Contains(form, `<option value="" selected>相手を選ぶ</option>`) {
		t.Errorf("the picker does not start on no party:\n%s", form)
	}
	if !strings.Contains(form, `data-origin=""`) {
		t.Errorf("the picker does not carry an empty origin:\n%s", form)
	}
	if !strings.Contains(form, `<option value="human">人間</option>`) {
		t.Errorf("the picker does not name the human as a person says it:\n%s", form)
	}
	if strings.Contains(form, `value="builder"`) {
		t.Errorf("the picker offers a party the node already hands the turn to:\n%s", form)
	}
}

// TestTheHandleAndThePickMakeTheSameChoice: the handle leads to the same choice
// the pick makes, showing the line a save would print, the edge dashed, and
// nothing written; a second press goes back to the field without the choice
// (設計 4 節).
func TestTheHandleAndThePickMakeTheSameChoice(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)

	_, body := f.get(t, server, "/definitions/demo/members/lead?preview=turn-add&to=human")
	form := between(body, `data-key="turn-add"`, "</form>")
	if !strings.Contains(form, `value="human" selected`) {
		t.Errorf("the pick's form does not read the chosen party:\n%s", form)
	}
	if !strings.Contains(form, "lead のプロンプトの『ターンを渡す相手』に人間が入ります") {
		t.Errorf("the pick shows no line about what a save would do:\n%s", form)
	}
	if !strings.Contains(body, `<path class="edge pending-add"`) {
		t.Errorf("the chosen edge is not drawn dashed before the write:\n%s", body)
	}
	if !strings.Contains(body, `<a class="handle" href="/definitions/demo/members/lead"`) {
		t.Errorf("a second press would not go back to the field without the choice:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("the pick wrote the file")
	}
}

// TestTheRuleCardWritesEveryField: one rule's card carries the five things a
// rule holds — the whole text, its reason, its origins, its check and its note —
// each writable from the start, with the buttons a change shows (C1, 設計 3.8).
func TestTheRuleCardWritesEveryField(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/agreement/TEAM-1")
	for _, want := range []string{
		`name="text"`, `name="why"`, `name="origin_kind"`, `name="origin_date"`,
		`name="origin_refs"`, `name="origin_note"`, `name="tests"`, `name="note"`,
		`data-change`, `data-revert`, `name="action" value="delete"`,
		// The button that takes the rule out names it (spec/42 T3).
		"TEAM-1 を消す",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the card does not carry %s:\n%s", want, body)
		}
	}
	// The card in the document is the id and the first clause of the text, never
	// the id alone (E1).
	_, list := f.get(t, server, "/definitions/demo")
	if !strings.Contains(list, "TEAM-1") || !strings.Contains(list, "このチームの約束。") {
		t.Errorf("the card is not its id and its first clause:\n%s", list)
	}
	if !strings.Contains(list, "/definitions/demo/agreement/new?section=0") {
		t.Errorf("the section offers no card to add a rule:\n%s", list)
	}
}

// TestARuleIsWrittenAndKeepsTheRestOfTheDocument: a save writes the one rule and
// leaves the document's other rules as they were, and the saved line names the
// file, the rule and what a running team has to type (設計 7.3、3.0 の表).
func TestARuleIsWrittenAndKeepsTheRestOfTheDocument(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	resp, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text":        {"書き換えた本文。"},
		"why":         {"理由も書く。"},
		"note":        {"メモ。"},
		"tests":       {"go test ./internal/web, 目で見る"},
		"hash":        {source.Hash(before)},
		"origin_kind": {"incident", ""},
		"origin_date": {"2026-02-02", ""},
		"origin_refs": {"d-1, d-2", ""},
		"origin_note": {"何が起きたか", ""},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	want := "version: 1\nsections:\n  - rules:\n" +
		"      - id: TEAM-1\n" +
		"        text: 書き換えた本文。\n" +
		"        why: 理由も書く。\n" +
		"        note: メモ。\n" +
		"        origin:\n" +
		"          - kind: incident\n" +
		"            date: \"2026-02-02\"\n" +
		"            refs:\n" +
		"              - d-1\n" +
		"              - d-2\n" +
		"            note: 何が起きたか\n" +
		"        tests:\n" +
		"          - go test ./internal/web\n" +
		"          - 目で見る\n"
	if after := readFile(t, path); after != want {
		t.Errorf("the document is\n%q\nwant\n%q", after, want)
	}
	if !strings.Contains(body, "の「規則 TEAM-1」を書きました。") {
		t.Errorf("the saved line does not name the file and the rule:\n%s", body)
	}
	if !strings.Contains(body, "hergy reprompt --all --team demo") {
		t.Errorf("the saved line does not say what the running team types:\n%s", body)
	}
	// The card that was written is drawn from the file, and its form carries the
	// mark that tells the script to drop the draft it kept (C5, 3.0).
	if !strings.Contains(body, `class="ruleform"`) || !strings.Contains(body, `data-saved="1"`) {
		t.Errorf("the written card does not drop its draft:\n%s", body)
	}
}

// TestASaveWithNothingChangedLeavesTheFile: a card saved as it stands writes
// nothing — the values are the ones the file already holds — and the line says
// so (spec/42 R1).
func TestASaveWithNothingChangedLeavesTheFile(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	_, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text":        {"このチームの約束。"},
		"hash":        {source.Hash(before)},
		"origin_kind": {"incident", ""},
		"origin_date": {"2026-01-01", ""},
	})
	if after := readFile(t, path); after != before {
		t.Errorf("a save that changed nothing rewrote the file:\n%q\nwant\n%q", after, before)
	}
	if !strings.Contains(body, "「規則 TEAM-1」は変わっていません。") {
		t.Errorf("the save does not say nothing changed:\n%s", body)
	}
	if strings.Contains(between(body, `class="saved"`, "</div>"), "書きました") {
		t.Errorf("a save that changed nothing says it wrote:\n%s", body)
	}
	// A file that moved under the page is refused (3.0 のぶつかり).
	_, body = f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text": {"x"}, "hash": {"deadbeef"},
	})
	if !strings.Contains(body, "変わりました") || !strings.Contains(body, "読み直す") {
		t.Errorf("a stale page was not refused with the clash line:\n%s", body)
	}
}

// TestARuleSaveIsRefusedWithTheValidateSentence: a rule that takes an id another
// document of a reader holds is refused with the sentence hergy validate prints,
// and the file is left as it was (設計 3.8 の断る、I18).
func TestARuleSaveIsRefusedWithTheValidateSentence(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "agreement.yaml")
	before := readFile(t, path)

	// The same edit, read through the catalog, is what the check judges: the
	// sentence the screen shows is the one check writes.
	doc, err := rules.Parse(before, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AddRule(0, rules.Rule{ID: "LEAD-1", Text: "重なった ID。"}); err != nil {
		t.Fatal(err)
	}
	edited, err := rules.Emit(doc)
	if err != nil {
		t.Fatal(err)
	}
	var want string
	for _, finding := range check.Definition(check.Input{Catalog: f.catalog().With(path, edited), Name: "demo"}) {
		if finding.Stops {
			want = finding.Text
			break
		}
	}
	if want == "" {
		t.Fatalf("the overlap stops nothing; the test cannot tell what the screen should say")
	}

	resp, body := f.post(t, server, "/agreement/new?section=0", url.Values{
		"id": {"LEAD-1"}, "text": {"重なった ID。"}, "hash": {source.Hash(before)},
		"origin_kind": {""},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the refused save answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, want) {
		t.Errorf("the refusal does not say what validate says (%q):\n%s", want, body)
	}
	if after := readFile(t, path); after != before {
		t.Errorf("the refused save wrote the document:\n%q", after)
	}
}

// TestTheCardCarriesTheRuleWarning: a rule whose text writes a turn the graph
// does not hold gets validate's warning on its own card, and the same sentence
// stands with the card in the document (設計 3.8、spec/33 H2).
func TestTheCardCarriesTheRuleWarning(t *testing.T) {
	f := newFixture(t)
	f.write(t, "teams/demo/agreement.yaml", "version: 1\nsections:\n  - rules:\n      - id: TEAM-1\n        text: lead → impl に渡す。\n")
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	var want string
	for _, finding := range check.Definition(check.Input{Catalog: f.catalog(), Name: "demo"}) {
		if finding.Field == "rule:"+path+"#TEAM-1" {
			want = finding.Text
		}
	}
	if want == "" {
		t.Fatalf("the rule text claims no unknown turn; the test cannot tell what the card should say")
	}
	_, body := f.get(t, server, "/definitions/demo/agreement/TEAM-1")
	if !strings.Contains(body, want) {
		t.Errorf("the card does not carry validate's warning %q:\n%s", want, body)
	}
	_, list := f.get(t, server, "/definitions/demo")
	if !strings.Contains(list, want) {
		t.Errorf("the card in the document does not carry validate's warning:\n%s", list)
	}
	// The same warning reaches a role document's card, through the definitions
	// that read it.
	f.write(t, "roles/lead.yaml", "version: 1\nsections:\n  - rules:\n      - id: LEAD-1\n        text: lead → impl に渡す。\n")
	_, body = f.get(t, server, "/roles/lead/LEAD-1")
	if !strings.Contains(body, "impl") {
		t.Errorf("a role document's card does not carry the warning:\n%s", body)
	}
}

// TestTheNewRuleGoesAtTheEndOfTheSection: 足す opens a card with the id the core
// proposes, and saving it puts the rule at the end of its section (設計 3.8).
func TestTheNewRuleGoesAtTheEndOfTheSection(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/agreement/new?section=0")
	if !strings.Contains(body, `value="TEAM-2"`) {
		t.Errorf("the core's proposal is not offered:\n%s", body)
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	resp, body := f.post(t, server, "/definitions/demo/agreement/new?section=0", url.Values{
		"id": {"TEAM-2"}, "text": {"足した規則。"}, "hash": {source.Hash(before)},
		"origin_kind": {""},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the add answered %d", resp.StatusCode)
	}
	want := before + "      - id: TEAM-2\n        text: 足した規則。\n"
	if after := readFile(t, path); after != want {
		t.Errorf("the document is\n%q\nwant\n%q", after, want)
	}
	if !strings.Contains(body, "TEAM-2") || !strings.Contains(body, "を書きました") {
		t.Errorf("the saved line does not name the new rule:\n%s", body)
	}
	// 足す without text is refused with the reader's own sentence.
	if after := readFile(t, path); after != want {
		t.Fatalf("the document moved")
	}
	// 本文 that is empty, and a date the format cannot hold, are said by the
	// field's own name, not by the file's numbering (spec/42 N3).
	_, body = f.post(t, server, "/definitions/demo/agreement/new?section=0", url.Values{
		"id": {"TEAM-3"}, "hash": {source.Hash(want)}, "origin_kind": {""},
	})
	if !strings.Contains(body, "本文を書いてください") {
		t.Errorf("a rule with no text is not refused by its field's name:\n%s", body)
	}
	if strings.Contains(body, "sections[") {
		t.Errorf("the refusal shows the file's own numbering:\n%s", body)
	}
	_, body = f.post(t, server, "/definitions/demo/agreement/new?section=0", url.Values{
		"id": {"TEAM-3"}, "text": {"日付の壊れた規則。"}, "hash": {source.Hash(want)},
		"origin_kind": {"incident"}, "origin_date": {"2026-13-99"},
	})
	if !strings.Contains(body, "日付は YYYY-MM-DD で書いてください") {
		t.Errorf("a date the format refuses is not refused by its field's name:\n%s", body)
	}
	// The card the person was writing in stays open, with the add form.
	if !strings.Contains(body, `class="ruleform"`) || !strings.Contains(body, `name="id"`) {
		t.Errorf("the refusal closed the card the person was writing in:\n%s", body)
	}
	if after := readFile(t, path); after != want {
		t.Errorf("a refused add wrote the document:\n%q", after)
	}
}

// TestTheRuleIsDeletedOnTheSecondPress: 消す asks once more in place and writes
// nothing on the first press; the second press takes the rule out and leaves the
// document's other rules (設計 3.8、W15).
func TestTheRuleIsDeletedOnTheSecondPress(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	_, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{"action": {"delete"}})
	// The first press asks once more in place: the card's button becomes the one
	// that writes, beside the way out, and no sentence is needed (spec/42 T3).
	if !strings.Contains(body, `name="action" value="delete-now"`) || !strings.Contains(body, "やめる") {
		t.Errorf("the first press does not ask again:\n%s", body)
	}
	if strings.Contains(body, "もう一度押すと消えます") {
		t.Errorf("the first press still writes the ask sentence")
	}
	if readFile(t, path) != before {
		t.Errorf("the first press wrote the document")
	}
	resp, body := f.post(t, server, "/definitions/demo/agreement", url.Values{
		"rule": {"TEAM-1"}, "action": {"delete-now"}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the delete answered %d", resp.StatusCode)
	}
	if after := readFile(t, path); after == before || strings.Contains(after, "TEAM-1") {
		t.Errorf("the second press did not take the rule out:\n%q", after)
	}
	// A rule that is taken out says so, not 書きました (spec/42 N2).
	if !strings.Contains(body, "の「規則 TEAM-1」を消しました。") {
		t.Errorf("the delete shows no saved line:\n%s", body)
	}
}

// TestADocumentThatIsNotThereIsMade: a role a member plays whose document no
// layer holds offers 作る, which writes an empty document at the place K2 names
// (W8). The shared agreement is made the same way.
func TestADocumentThatIsNotThereIsMade(t *testing.T) {
	f := newFixture(t)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")
	if err := os.Remove(rolePath); err != nil {
		t.Fatal(err)
	}
	agreementPath := filepath.Join(f.proj, ".hergy", "agreement.yaml")
	if err := os.Remove(agreementPath); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)

	_, body := f.get(t, server, "/roles")
	if !strings.Contains(body, `action="/roles/builder"`) {
		t.Errorf("the roles list offers no way to make the document:\n%s", body)
	}
	// The place is written beside 作る, before it is pressed (spec/42 T1).
	if !strings.Contains(body, filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")) {
		t.Errorf("the roles list does not say where the document is made:\n%s", body)
	}
	_, body = f.get(t, server, "/")
	if !strings.Contains(body, `action="/agreement"`) {
		t.Errorf("the definitions list offers no way to make the shared agreement:\n%s", body)
	}
	for _, made := range []struct{ path, action string }{
		{rolePath, "/roles/builder"},
		{agreementPath, "/agreement"},
	} {
		resp, body := f.post(t, server, made.action, url.Values{"hash": {source.Hash("")}, "make": {"1"}})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST %s answered %d", made.action, resp.StatusCode)
		}
		if got := readFile(t, made.path); got != "version: 1\nsections:\n  - heading: 規則\n" {
			t.Errorf("%s is\n%q\nwant an empty document with the heading 規則", made.path, got)
		}
		if !strings.Contains(body, "を作りました") {
			t.Errorf("%s shows no made line:\n%s", made.action, body)
		}
	}
	// The document that was made reads: the role's page holds the empty section
	// with the card to add the first rule to.
	_, body = f.get(t, server, "/roles/builder")
	if !strings.Contains(body, "/roles/builder/new?section=0") {
		t.Errorf("the made role document offers no way to add a rule:\n%s", body)
	}
}

// TestTheHandoverNamesEveryTeamThatReadsTheDocument: a role document is read by
// the members that play the role in every definition, so every running team is
// told to take the change up (設計 3.0 の表、C8).
func TestTheHandoverNamesEveryTeamThatReadsTheDocument(t *testing.T) {
	f := newFixture(t)
	f.writeTeam(t, "other", "other の使命")
	f.running = []effect.Running{{Team: "a", Definition: "demo"}, {Team: "b", Definition: "other"}}
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	before := readFile(t, path)
	_, body := f.post(t, server, "/roles/lead/LEAD-1", url.Values{
		"text": {"lead は最初に動く（直した）。"}, "hash": {source.Hash(before)},
		"origin_kind": {""},
	})
	for _, team := range []string{"a", "b"} {
		if !strings.Contains(body, "<b>"+team+":</b>") {
			t.Errorf("the handover does not name team %s:\n%s", team, body)
		}
	}
	if got := strings.Count(body, "hergy reprompt --all --team"); got != 2 {
		t.Errorf("the handover holds %d reprompts, want one per reading team:\n%s", got, body)
	}
}

// TestRuleSaveKeepsTheLinesTheFileWrote: a document a person wrote by hand keeps
// its blank lines, its comment, its quoting and its flow refs when one rule is
// written; only that rule's own lines move (K1=B, d-7b58).
func TestRuleSaveKeepsTheLinesTheFileWrote(t *testing.T) {
	f := newFixture(t)
	hand := "version: 1\n" +
		"title: 手で書いた文書\n" +
		"\n" +
		"# 約束\n" +
		"sections:\n" +
		"  - heading: 約束\n" +
		"    rules:\n" +
		"      - id: TEAM-1\n" +
		"        text: | -\n" +
		"          1 行目の本文。\n" +
		"          2 行目の本文。\n" +
		"        why: '理由はシングルクォート'\n" +
		"        origin:\n" +
		"          - kind: incident\n" +
		"            date: \"2026-01-01\"\n" +
		"            refs: [d-1, d-2]\n" +
		"      - id: TEAM-2\n" +
		"        text: TEAM-2 の本文。\n" +
		"        note: TEAM-2 のメモ\n"
	hand = strings.Replace(hand, "text: | -", "text: |-", 1)
	f.write(t, "teams/demo/agreement.yaml", hand)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	resp, body := f.post(t, server, "/definitions/demo/agreement/TEAM-2", url.Values{
		"text": {"TEAM-2 の本文。"}, "note": {"書き換えたメモ"}, "hash": {source.Hash(hand)},
		"origin_kind": {""},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, "の「規則 TEAM-2」を書きました。") {
		t.Errorf("the saved line does not name the rule:\n%s", body)
	}
	after := readFile(t, path)
	want := strings.Replace(hand, "        note: TEAM-2 のメモ", "        note: 書き換えたメモ", 1)
	if after != want {
		t.Errorf("the document is\n%q\nwant\n%q", after, want)
	}
	for _, kept := range []string{"why: '理由はシングルクォート'", "refs: [d-1, d-2]", "# 約束", "\n\n# 約束"} {
		if !strings.Contains(after, kept) {
			t.Errorf("the save changed %q:\n%s", kept, after)
		}
	}
}

// TestASaveWithTheSameValuesTellsNoTeamToTakeItUp: a rule written back with the
// values the file already holds reads the same to every member, so no running
// team is told to type anything (spec/42 N1).
func TestASaveWithTheSameValuesTellsNoTeamToTakeItUp(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	_, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text":        {"このチームの約束。"},
		"hash":        {source.Hash(before)},
		"origin_kind": {"incident"},
		"origin_date": {"2026-01-01"},
	})
	saved := between(body, `class="saved"`, "</div>")
	if !strings.Contains(saved, "動いているチームに渡すものはありません") {
		t.Errorf("a save with the same values tells a team to take it up:\n%s", saved)
	}
	if !strings.Contains(saved, "「規則 TEAM-1」は変わっていません。") {
		t.Errorf("the saved line does not say nothing changed:\n%s", saved)
	}
}

// TestAHandWrittenSavesNothingWhenTheValuesAreTheSame is spec/42 R1: a rule a
// person wrote with a quoted text and a flow sequence for its refs is not
// rewritten when the save changes no value — the field's values are the ones the
// file holds — and the line says the document did not change.
func TestAHandWrittenSavesNothingWhenTheValuesAreTheSame(t *testing.T) {
	f := newFixture(t)
	hand := "version: 1\ntitle: 手で書いた文書\n\nsections:\n" +
		"  - heading: 約束\n" +
		"    rules:\n" +
		"      - id: TEAM-1\n" +
		"        text: '台帳から始めて台帳に書く。'\n" +
		"        origin:\n" +
		"          - kind: incident\n" +
		"            date: \"2026-01-01\"\n" +
		"            refs: [d-1, d-2]\n"
	f.write(t, "teams/demo/agreement.yaml", hand)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	resp, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text":        {"台帳から始めて台帳に書く。"},
		"hash":        {source.Hash(hand)},
		"origin_kind": {"incident"},
		"origin_date": {"2026-01-01"},
		"origin_refs": {"d-1, d-2"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if after := readFile(t, path); after != hand {
		t.Errorf("a save with the same values rewrote the file:\n%q\nwant\n%q", after, hand)
	}
	if !strings.Contains(body, "「規則 TEAM-1」は変わっていません。") {
		t.Errorf("the save does not say nothing changed:\n%s", body)
	}
}

// TestAShapeTheScreenCannotWriteNamesTheDocument: a rule document whose sections
// are one flow sequence cannot be written, and the sentence names that document
// rather than team.yaml (spec/42 R2).
func TestAShapeTheScreenCannotWriteNamesTheDocument(t *testing.T) {
	f := newFixture(t)
	f.write(t, "teams/demo/agreement.yaml", "version: 1\nsections: [{rules: [{id: TEAM-1, text: x}]}]\n")
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	resp, body := f.post(t, server, "/definitions/demo/agreement/TEAM-1", url.Values{
		"text": {"y"}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the refused save answered %d", resp.StatusCode)
	}
	panel := between(body, `<h2 class="panel-title">`, "</aside>")
	if !strings.Contains(panel, path+" の sections は、この画面では書き換えられない形です") {
		t.Errorf("the refusal does not name the document:\n%s", panel)
	}
	if strings.Contains(panel, "team.yaml") {
		t.Errorf("the refusal names team.yaml:\n%s", panel)
	}
	if after := readFile(t, path); after != before {
		t.Errorf("the refused save wrote the document:\n%q", after)
	}
}
func (f *fixture) writeGlobal(t *testing.T, rel, text string) {
	t.Helper()
	path := filepath.Join(f.global, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTeam writes one team definition with a member, a worker, a turn to the
// human, a role document for each role, a team agreement and a shared one.

func (f *fixture) profileFile() string {
	return filepath.Join(f.proj, ".hergy", "profiles.yaml")
}

// commandLines is every command a rendered handover holds, as the copy button's
// code element writes it (設計 3.0: Command.Line is the command alone).

func commandLines(body string) []string {
	var out []string
	for _, part := range strings.Split(body, `<code id="cmd-`)[1:] {
		_, rest, ok := strings.Cut(part, ">")
		if !ok {
			continue
		}
		line, _, ok := strings.Cut(rest, "</code>")
		if !ok {
			continue
		}
		out = append(out, line)
	}
	return out
}

// optionValues is the value of every <option> a page writes, in order.

func optionValues(body string) []string {
	var out []string
	for _, part := range strings.Split(body, `<option value="`)[1:] {
		value, _, ok := strings.Cut(part, `"`)
		if !ok {
			continue
		}
		out = append(out, value)
	}
	return out
}

// TestProfileSaveWritesOnlyThatProfile: the kind, the args and the env are
// written into the profiles.yaml the profile was read from, in hergy's one
// written form, and no other profile moves (設計 3.7・7.3).

func TestProfileSaveWritesOnlyThatProfile(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.profileFile()
	resp, body := f.post(t, server, "/profiles/coder", url.Values{
		"kind":     {"opencode"},
		"args":     {"--y\n--z"},
		"envname":  {"A", "B"},
		"envvalue": {"one", "two"},
		"hash":     {source.Hash(readFile(t, path))},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	if !strings.Contains(body, "「プロファイル coder」を書きました") {
		t.Errorf("the saved line does not name the file and the key:\n%s", body)
	}
	// The args keep the form the file wrote them in (a flow sequence on the
	// key's line), the changed values are written, and every other line — the
	// other profile above all — is exactly as it was (spec/41 S1).
	want := "version: 1\n" +
		"profiles:\n" +
		"  coder:\n" +
		"    kind: opencode\n" +
		"    args: [--y, --z]\n" +
		"    env:\n" +
		"      A: one\n" +
		"      B: two\n" +
		"  planner:\n" +
		"    kind: codex\n"
	if got := readFile(t, path); got != want {
		t.Errorf("the save wrote\n%q\nwant\n%q", got, want)
	}
	// The field shows what was written, one arg to a line.
	if !strings.Contains(body, "--y\n--z</textarea>") {
		t.Errorf("the args box does not hold the written args:\n%s", body)
	}
	// The same profile read again still holds them.
	_, body = f.get(t, server, "/profiles/coder")
	if !strings.Contains(body, "--y\n--z</textarea>") {
		t.Errorf("the args box does not read them back:\n%s", body)
	}
	if !strings.Contains(body, `value="B"`) || !strings.Contains(body, "two</textarea>") {
		t.Errorf("the env rows do not read the written pair back:\n%s", body)
	}
}

// TestProfileSaveRefused: a save that cannot be written is refused with one
// sentence beside the field, the values a person wrote stay, and the file does
// not move (設計 3.7 の「失敗の 1 行」, C4).

func TestProfileSaveRefused(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.profileFile()
	before := readFile(t, path)

	// An env row with a value and no name is not something to write; the values
	// the person wrote stay on the page (C4).
	resp, body := f.post(t, server, "/profiles/coder", url.Values{
		"kind":     {"claude"},
		"args":     {"typed-arg"},
		"envname":  {"", "A"},
		"envvalue": {"a-value", "one"},
		"hash":     {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "env の組を直してください") {
		t.Errorf("a nameless env row was not refused: %d\n%s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "typed-arg</textarea>") || !strings.Contains(body, "a-value</textarea>") {
		t.Errorf("the refused save did not keep what the person wrote:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused save wrote the file")
	}

	// A kind the file cannot read stops every definition that reads it, so the
	// save is refused with validate's own sentence.
	resp, body = f.post(t, server, "/profiles/coder", url.Values{
		"kind":     {""},
		"args":     {"--x"},
		"envname":  {"A"},
		"envvalue": {"one"},
		"hash":     {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the refused save answered %d", resp.StatusCode)
	}
	if strings.Contains(body, "書きました") {
		t.Errorf("a save that stops a definition was written:\n%s", body)
	}
	if !strings.Contains(body, `class="findings"`) {
		t.Errorf("the refusal shows no sentence:\n%s", body)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused save wrote the file")
	}

	// A file that moved under the page is refused with the clash line and the
	// reload link.
	_, body = f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"claude"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {"deadbeef"},
	})
	if !strings.Contains(body, "変わりました") || !strings.Contains(body, "読み直す") {
		t.Errorf("a stale page was not refused with the clash line and a reload link:\n%s", body)
	}
}

// TestProfileChangeHandsOverRealCommands: a kind or args change is a refresh of
// the members that run the profile, an env change is a down and an up, and every
// command is taken apart by the real command definitions (設計 3.0 の表,
// d-a8b1 追加の決め 5).

func TestProfileChangeHandsOverRealCommands(t *testing.T) {
	f := newFixture(t)
	f.running = []effect.Running{{Team: "demo", Definition: "demo"}}
	server := f.server(t)
	path := f.profileFile()

	check := func(body string) []string {
		t.Helper()
		lines := commandLines(body)
		for _, line := range lines {
			if _, err := options.Parse(strings.Fields(line)[1:]); err != nil {
				t.Errorf("the command %q does not parse: %v", line, err)
			}
		}
		return lines
	}

	_, body := f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"opencode"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(readFile(t, path))},
	})
	lines := check(body)
	if !strings.Contains(strings.Join(lines, "\n"), "hergy refresh builder --team demo") {
		t.Errorf("a kind change does not refresh a member that runs the profile:\n%s", strings.Join(lines, "\n"))
	}

	_, body = f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"opencode"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"two"},
		"hash": {source.Hash(readFile(t, path))},
	})
	lines = check(body)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "hergy down demo") || !strings.Contains(joined, "hergy up demo --as demo") {
		t.Errorf("an env change does not hand over a down and an up:\n%s", joined)
	}
}

// TestProfileKindChoicesPutTheUsedKindsFirst: the kinds the profiles.yaml
// already uses come first, then herdr's own order (設計 3.7, spec/38 T4).

func TestProfileKindChoicesPutTheUsedKindsFirst(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/profiles/coder")
	choices := optionValues(body)
	if len(choices) != len(profile.Kinds()) {
		t.Fatalf("the kinds offered are %d, want %d", len(choices), len(profile.Kinds()))
	}
	if choices[0] != "claude" || choices[1] != "codex" {
		t.Errorf("the kind choices start %v, want the used kinds first", choices[:2])
	}
}

// TestAProfileNamedAfterItsKindWarns: a profile whose name is its kind gets
// validate's warning on its own page (d-a477).

func TestAProfileNamedAfterItsKindWarns(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n  claude:\n    kind: claude\n")
	server := f.server(t)
	_, body := f.get(t, server, "/profiles/claude")
	if !strings.Contains(body, i18n.Tf("validate.profile_named_after_kind", "claude")) {
		t.Errorf("a profile named after its kind does not warn:\n%s", body)
	}
	_, body = f.get(t, server, "/profiles/coder")
	if strings.Contains(body, i18n.Tf("validate.profile_named_after_kind", "coder")) {
		t.Errorf("a profile not named after its kind warns")
	}
}

// TestAProfileInUseIsShownAndNotDeleted: a profile a member or a worker runs
// names who uses it and is not taken away; an unused one goes, and the others
// stay (設計 3.7, I8).

func TestAProfileInUseIsShownAndNotDeleted(t *testing.T) {
	f := newFixture(t)
	// One profile nothing runs, beside the one a member and a worker run.
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n    args: [--x]\n    env:\n      A: one\n  planner:\n    kind: codex\n  spare:\n    kind: gemini\n")
	server := f.server(t)
	path := f.profileFile()
	before := readFile(t, path)

	// The detail shows a member and a worker that run it (I8).
	_, body := f.get(t, server, "/profiles/coder")
	for _, want := range []string{"demo の lead", "demo の lead が呼ぶ 試作"} {
		if !strings.Contains(body, want) {
			t.Errorf("the profile does not name the user %q:\n%s", want, body)
		}
	}
	// And it cannot be taken away while they name it.
	resp, body := f.post(t, server, "/profiles/coder/delete", url.Values{"hash": {source.Hash(before)}})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "demo の lead が使っています") {
		t.Errorf("the delete of a used profile does not say who uses it: %d\n%s", resp.StatusCode, body)
	}
	if readFile(t, path) != before {
		t.Errorf("a refused delete wrote the file")
	}

	// One nothing runs goes, and the rest of the file stays.
	resp, body = f.post(t, server, "/profiles/spare/delete", url.Values{"hash": {source.Hash(before)}})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "「プロファイル spare」を消しました") {
		t.Errorf("the delete of an unused profile was not written: %d\n%s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	if strings.Contains(after, "spare") {
		t.Errorf("the deleted profile is still in the file:\n%s", after)
	}
	if !strings.Contains(after, "  coder:") {
		t.Errorf("the delete took another profile with it:\n%s", after)
	}
}

// TestAddingAProfileWritesItAtTheNewPlace: the list's 足す writes a new profile
// where K2 says (C13), keeps the others, and refuses a name hergy already
// resolves (設計 3.7).

func TestAddingAProfileWritesItAtTheNewPlace(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.profileFile()
	resp, body := f.post(t, server, "/profiles", url.Values{
		"name": {"tester"}, "kind": {"gemini"},
		"hash": {source.Hash(readFile(t, path))},
	})
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "「プロファイル tester」を書きました") {
		t.Fatalf("the new profile was not written: %d\n%s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	if !strings.Contains(after, "  tester:\n    kind: gemini\n") {
		t.Errorf("the new profile is not in the file:\n%s", after)
	}
	if !strings.Contains(after, "  coder:") {
		t.Errorf("the add took another profile with it:\n%s", after)
	}

	// A name hergy already resolves is refused, and nothing is written.
	_, body = f.post(t, server, "/profiles", url.Values{
		"name": {"coder"}, "kind": {"claude"},
		"hash": {source.Hash(after)},
	})
	if !strings.Contains(body, "もうあります") {
		t.Errorf("adding a name that is already there was not refused:\n%s", body)
	}
	if readFile(t, path) != after {
		t.Errorf("a refused add wrote the file")
	}

	// A name that is its kind is written, and validates' warning comes with it
	// (d-a477).
	_, body = f.post(t, server, "/profiles", url.Values{
		"name": {"gemini"}, "kind": {"gemini"},
		"hash": {source.Hash(after)},
	})
	if !strings.Contains(body, "書きました") {
		t.Errorf("a profile named after its kind was not written:\n%s", body)
	}
	if !strings.Contains(body, i18n.Tf("validate.profile_named_after_kind", "gemini")) {
		t.Errorf("the add of a name that is its kind does not warn:\n%s", body)
	}
}

// TestProfileKindChoicesCountEveryResolvedProfile is N1: the kinds offered come
// from every profile this catalog resolves, the project's file and the outside
// one together, and the kinds used by the most profiles come first (spec/41 N1,
// spec/38 T4).

func TestProfileKindChoicesCountEveryResolvedProfile(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n")
	f.writeGlobal(t, "profiles.yaml", "version: 1\nprofiles:\n  outside-one:\n    kind: codex\n  outside-two:\n    kind: codex\n")
	server := f.server(t)

	_, body := f.get(t, server, "/profiles")
	choices := optionValues(body)
	if len(choices) != len(profile.Kinds()) {
		t.Fatalf("the kinds offered are %d, want %d", len(choices), len(profile.Kinds()))
	}
	if choices[0] != "codex" {
		t.Errorf("the kinds start %v, want codex: two profiles outside the project run it", choices[:2])
	}
	if choices[1] != "claude" {
		t.Errorf("the kinds start %v, want claude next: one profile runs it", choices[:2])
	}
}

// TestProfileKindChoicesAskWhenNothingIsUsed is N1: when no profile runs a kind,
// the list leads with an empty choice, and adding with it left empty is refused
// rather than guessed at.

func TestProfileKindChoicesAskWhenNothingIsUsed(t *testing.T) {
	f := newFixture(t)
	if err := os.Remove(f.profileFile()); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)

	_, body := f.get(t, server, "/profiles")
	choices := optionValues(body)
	if len(choices) != len(profile.Kinds())+1 || choices[0] != "" {
		t.Fatalf("the kinds start %v, want an empty choice first", choices[:1])
	}
	if !strings.Contains(body, "kind を選ぶ") {
		t.Errorf("the empty choice carries no label:\n%s", body)
	}
	_, body = f.post(t, server, "/profiles", url.Values{
		"name": {"newp"}, "kind": {""}, "hash": {source.Hash("")},
	})
	if !strings.Contains(body, "kind を選んでください") {
		t.Errorf("an add with no kind was not refused:\n%s", body)
	}
	if _, err := os.Stat(f.profileFile()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused add wrote the file")
	}
}

// TestAddingAProfileWithNoNameIsRefused is N4: an empty name is refused beside
// the field, not with the file's own sentence (spec/41 N4).

func TestAddingAProfileWithNoNameIsRefused(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.profileFile())

	_, body := f.post(t, server, "/profiles", url.Values{
		"name": {"  "}, "kind": {"claude"},
		"hash": {source.Hash(before)},
	})
	if !strings.Contains(body, "プロファイルの名前を書いてください") {
		t.Errorf("an add with no name is not refused with the screen's own line:\n%s", body)
	}
	if strings.Contains(body, "profiles.:") {
		t.Errorf("the file's own sentence is shown for an empty name:\n%s", body)
	}
	if readFile(t, f.profileFile()) != before {
		t.Errorf("a refused add wrote the file")
	}
}

// TestProfileSaveSaysWhoIsLaunchedDifferently is walk-5 A2: a change to a
// profile the members run says whose launch (or whose pane's env) moves, with
// effect's own line (設計 3.0 の表, I16).

func TestProfileSaveSaysWhoIsLaunchedDifferently(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.profileFile()

	// A kind change moves how the members that run the profile are started.
	_, body := f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"opencode"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(readFile(t, path))},
	})
	preview := between(body, `class="preview"`, "</ul>")
	for _, want := range []string{"lead の起動のしかたが変わります", "builder の起動のしかたが変わります"} {
		if !strings.Contains(preview, want) {
			t.Errorf("the saved line does not say %q:\n%s", want, body)
		}
	}

	// An env change moves the panes the members are made with.
	_, body = f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"opencode"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"two"},
		"hash": {source.Hash(readFile(t, path))},
	})
	preview = between(body, `class="preview"`, "</ul>")
	if !strings.Contains(preview, "ペインの環境変数が変わります") {
		t.Errorf("the saved line does not say the pane env moved:\n%s", body)
	}
}

// TestTheAddFormSaysWhereItWrites is T2: the file a new profile goes into stands
// beside the add form, before it is pressed (K2).

func TestTheAddFormSaysWhereItWrites(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/profiles")
	form := between(body, `class="add"`, "</form>")
	if !strings.Contains(form, f.profileFile()) {
		t.Errorf("the add form does not say where it writes:\n%s", form)
	}
}

// handWritten is a profiles.yaml a person wrote: a comment at the head, a blank
// line, a comment per profile, the profiles in an order that is not by name, one
// profile's args as a flow sequence with a comment after it, and another's env as
// a flow mapping (spec/41 S1).
const handWritten = "version: 1\n" +
	"# 手で書いたファイル\n" +
	"\n" +
	"profiles:\n" +
	"  planner:\n" +
	"    # 計画と仕様\n" +
	"    kind: claude\n" +
	"    args: [\"--model\", \"opus\"]   # 重いほう\n" +
	"    env:\n" +
	"      A: one\n" +
	"  coder:\n" +
	"    kind: opencode\n" +
	"    args:\n" +
	"      - --y\n" +
	"    env: {B: two}\n"

// TestProfileSaveKeepsEveryLineItDidNotChange is spec/41 S1: a save writes the
// place that moved, and not one byte of the rest — the comments, the blank line,
// the order, the closing quote of a value nobody touched, and the form the file
// wrote (a flow sequence on the key's line, a block sequence under it).
func TestProfileSaveKeepsEveryLineItDidNotChange(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", handWritten)
	server := f.server(t)
	path := f.profileFile()

	// The kind changes: the line above it, the value below it and the other
	// profile are the file's own lines, byte for byte.
	_, body := f.post(t, server, "/profiles/planner", url.Values{
		"kind": {"codex"}, "args": {"--model\nopus"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(readFile(t, path))},
	})
	if !strings.Contains(body, "書きました") {
		t.Fatalf("the save was refused:\n%s", body)
	}
	want := strings.Replace(handWritten, "    kind: claude\n", "    kind: codex\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the save wrote\n%q\nwant\n%q", got, want)
	}

	// The args change: the flow form and its comment stay, and so does the
	// block form of the other profile's args.
	_, body = f.post(t, server, "/profiles/planner", url.Values{
		"kind": {"codex"}, "args": {"--model\nsonnet"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(readFile(t, path))},
	})
	if !strings.Contains(body, "書きました") {
		t.Fatalf("the args save was refused:\n%s", body)
	}
	want = strings.Replace(want, `args: ["--model", "opus"]   # 重いほう`, "args: [--model, sonnet]   # 重いほう", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the args save wrote\n%q\nwant\n%q", got, want)
	}
}

// TestProfileSaveWithTheSameValuesChangesNothing is spec/41 S1: a save that
// changes nothing writes nothing at all, so a hand-written file comes back byte
// for byte — even where the file's own form differs from hergy's (a flow value, a
// quoted item, a comment).
func TestProfileSaveWithTheSameValuesChangesNothing(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", handWritten)
	server := f.server(t)
	path := f.profileFile()
	before := readFile(t, path)

	_, body := f.post(t, server, "/profiles/planner", url.Values{
		"kind": {"claude"}, "args": {"--model\nopus"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(before)},
	})
	if !strings.Contains(body, "書きました") {
		t.Fatalf("the same-value save was refused:\n%s", body)
	}
	if got := readFile(t, path); got != before {
		t.Errorf("a save that changed nothing wrote\n%q\nwant\n%q", got, before)
	}
}

// TestAProfileAddGoesAfterTheLastOneAndKeepsTheFile: the new profile is appended
// after the last one — not in name order — and every line the person wrote stays
// (spec/41 S1).
func TestAProfileAddGoesAfterTheLastOneAndKeepsTheFile(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", handWritten)
	server := f.server(t)
	path := f.profileFile()

	_, body := f.post(t, server, "/profiles", url.Values{
		"name": {"tester"}, "kind": {"gemini"},
		"hash": {source.Hash(readFile(t, path))},
	})
	if !strings.Contains(body, "書きました") {
		t.Fatalf("the add was refused:\n%s", body)
	}
	want := handWritten + "  tester:\n    kind: gemini\n"
	if got := readFile(t, path); got != want {
		t.Errorf("the add wrote\n%q\nwant\n%q", got, want)
	}
}

// TestProfilePreviewNamesTheDefinitionWhenTwoAreInvolved is spec/41 R1: a profile
// two definitions run says which definition's member each line is about, so the
// same sentence twice tells two members apart; one definition's change is said the
// way it always was.
func TestProfilePreviewNamesTheDefinitionWhenTwoAreInvolved(t *testing.T) {
	f := newFixture(t)
	// A second definition whose lead runs coder too and whose builder runs
	// planner: one profile reaches two definitions, the other one.
	f.write(t, filepath.Join("teams", "other", "team.yaml"), "version: 1\nmission: other の使命\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: planner\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\n")
	server := f.server(t)
	path := f.profileFile()

	_, body := f.post(t, server, "/profiles/coder", url.Values{
		"kind": {"gemini"}, "args": {"--x"},
		"envname": {"A"}, "envvalue": {"one"},
		"hash": {source.Hash(readFile(t, path))},
	})
	preview := between(body, `class="preview"`, "</ul>")
	for _, want := range []string{
		"demo の lead の起動のしかたが変わります",
		"other の lead の起動のしかたが変わります",
	} {
		if !strings.Contains(preview, want) {
			t.Errorf("the line %q is not there:\n%s", want, body)
		}
	}

	// planner reaches the second definition's builder alone: the member stands
	// alone.
	_, body = f.post(t, server, "/profiles/planner", url.Values{
		"kind": {"opencode"}, "args": {""},
		"envname": {""}, "envvalue": {""},
		"hash": {source.Hash(readFile(t, path))},
	})
	preview = between(body, `class="preview"`, "</ul>")
	if !strings.Contains(preview, "builder の起動のしかたが変わります") || strings.Contains(preview, "other の builder") {
		t.Errorf("one definition's change names its definition:\n%q", preview)
	}
}

// TestTheCoarseArrowIsPressableWhereItLooks: the line a person presses is thin,
// so each arrow carries a wider line that cannot be seen inside the same link
// (walk-1b P1). The invisible line follows the same path, and a pending edge has
// none (it is not written yet).
func TestTheCoarseArrowIsPressableWhereItLooks(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	def, err := f.catalog().Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	_, body := f.get(t, server, "/definitions/demo")
	if got := strings.Count(body, `<path class="hit"`); got != len(def.Graph.Edges) {
		t.Errorf("the figure draws %d wide lines, want %d", got, len(def.Graph.Edges))
	}
	link := between(body, `<a href="`+turnHref("demo", "lead", "builder")+`"`, "</a>")
	if !strings.Contains(link, `<path class="hit"`) || !strings.Contains(link, `class="edge"`) {
		t.Errorf("the wide line is not in the arrow's own link:\n%s", link)
	}
	d := attrOf(link, `class="hit"`, "d")
	if d == "" || !strings.Contains(link, ` d="`+d+`"`) {
		t.Errorf("the wide line does not follow the arrow's path (%q):\n%s", d, link)
	}
}

// attrOf is one attribute of the tag that holds a marker, or "".
func attrOf(body, marker, name string) string {
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	tag := body[i:]
	if end := strings.Index(tag, ">"); end >= 0 {
		tag = tag[:end]
	}
	i = strings.Index(tag, name+`="`)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(name)+2:]
	if before, _, ok := strings.Cut(rest, `"`); ok {
		return before
	}
	return ""
}

// TestTheMemberBoxNamesTheRoleAndTheProfile: the box's second line says which
// name is the role and which is the profile, so "lead、planner" is not read as
// two roles (walk-1b P3).
func TestTheMemberBoxNamesTheRoleAndTheProfile(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo")
	if !strings.Contains(body, `>ロール lead・プロファイル coder</text>`) {
		t.Errorf("the member's box does not name the role and the profile:\n%s", body)
	}
}

// TestTheNamesOfTurnsAndWorkersStandOnce: the parties one node hands the turn
// to, and the workers one member hires, are one name (dt) with a value (dd)
// each, rather than the name repeated for every row (walk-2 P4).
func TestTheNamesOfTurnsAndWorkersStandOnce(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead")

	turns := between(body, `<dt>ターンを渡す相手</dt>`, `<dt>ターンを足す</dt>`)
	if got := strings.Count(body, `<dt>ターンを渡す相手</dt>`); got != 1 {
		t.Errorf("the turn's name stands %d times, want once", got)
	}
	if got := strings.Count(turns, "<dd>"); got != 1 {
		t.Errorf("the member hands the turn to %d parties, want 1", got)
	}

	workers := between(body, `<dt>ワーカー</dt>`, `<dt>ワーカーを足す</dt>`)
	if got := strings.Count(body, `<dt>ワーカー</dt>`); got != 1 {
		t.Errorf("the workers' name stands %d times, want once", got)
	}
	if got := strings.Count(workers, "<dd>"); got != 3 {
		t.Errorf("the member hires %d workers, want 3", got)
	}
}

// TestAChosenTurnIsDrawnInALane: the dashed edge a press chose stands in a lane
// to the right of the members, the way the arrow will stand once it is written,
// instead of being drawn straight through the boxes (walk-3 P2).
func TestAChosenTurnIsDrawnInALane(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead?preview=turn-add&to=human")
	chosen := between(body, `class="edge pending-add"`, "</path>")
	if chosen == "" {
		t.Fatalf("the chosen turn is not drawn:\n%s", body)
	}
	if !strings.Contains(chosen, " C") {
		t.Errorf("the chosen turn is not drawn as the arrows are (no bend):\n%s", chosen)
	}
	def, err := f.catalog().Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	layout := topology.LayoutGraph(def.Graph, definition.Edge{From: (definition.Human{}).ID(), To: definition.NodeID{Kind: definition.KindMember, Name: "builder"}})
	if len(layout.Edges) != len(def.Graph.Edges) {
		t.Errorf("the pending edge changed the graph's own arrows: %d, want %d", len(layout.Edges), len(def.Graph.Edges))
	}
	if len(layout.Pending) != 1 || layout.Pending[0].LaneX <= layout.MemberRight {
		t.Errorf("the pending edge has no lane of its own: %+v", layout.Pending)
	}
}

// TestAPreviewShowsTheButtonsWithoutScript: a preview URL picks a value before
// anything is typed, so the page draws the save and revert buttons already shown
// rather than hidden behind a script (設計 7.3).
func TestAPreviewShowsTheButtonsWithoutScript(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, body := f.get(t, server, "/definitions/demo/members/lead?preview=role&role=builder")
	form := between(body, `data-key="role"`, "</form>")
	if form == "" {
		t.Fatalf("the role's field is not drawn:\n%s", body)
	}
	if !strings.Contains(form, `data-change>`) {
		t.Errorf("the preview's buttons are hidden:\n%s", form)
	}
	if strings.Contains(form, "data-change hidden") {
		t.Errorf("the preview's buttons are hidden:\n%s", form)
	}
	// A field nobody picked still hides them.
	_, body = f.get(t, server, "/definitions/demo/members/lead")
	if form := between(body, `data-key="role"`, "</form>"); !strings.Contains(form, "data-change hidden") {
		t.Errorf("the unpicked field shows its buttons:\n%s", form)
	}
}

// TestAnOriginDraftKeepsItsKeyWhenTheRowsGrow: the row a person fills in keeps
// its own key however many origins the rule holds, so a draft is not moved onto
// an origin a save added above it (spec/42 T2).
func TestAnOriginDraftKeepsItsKeyWhenTheRowsGrow(t *testing.T) {
	first := originRows([]rules.Origin{{Kind: "incident", Date: "2026-01-01"}})
	more := originRows([]rules.Origin{{Kind: "incident", Date: "2026-01-01"}, {Kind: "instruction", Date: "2026-02-01"}})
	if len(first) != 2 || len(more) != 3 {
		t.Fatalf("the rows are %d and %d, want 2 and 3", len(first), len(more))
	}
	if first[0].Key != more[0].Key {
		t.Errorf("the first origin's key moved: %q then %q", first[0].Key, more[0].Key)
	}
	if first[len(first)-1].Key != "new" || more[len(more)-1].Key != "new" {
		t.Errorf("the row to fill in has no key of its own: %q and %q", first[len(first)-1].Key, more[len(more)-1].Key)
	}
}
