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
	"time"

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
	known, byName, _ := server.sections(f.running, agents, errs, second, later)
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
	if !strings.Contains(body, "インシデント") {
		t.Errorf("an origin of the incident kind is not said as インシデント:\n%s", body)
	}
	if strings.Contains(body, "incident") {
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
