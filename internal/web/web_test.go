package web

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
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
	root   string
	proj   string
	global string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		root:   root,
		proj:   filepath.Join(root, "proj"),
		global: filepath.Join(root, "global"),
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
// the human, a team agreement and a role document.
func (f *fixture) writeTeam(t *testing.T, name, mission string) {
	t.Helper()
	f.write(t, filepath.Join("teams", name, "team.yaml"), "version: 1\nmission: "+mission+"\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\nworkers:\n  - from: lead\n    purpose: 実装\n    profile: coder\n  - from: lead\n    purpose: 試作\n    profile: coder\n  - from: lead\n    purpose: 調査\n    profile: planner\n")
	f.write(t, filepath.Join("teams", name, "agreement.yaml"), "version: 1\nsections:\n  - rules:\n      - id: TEAM-1\n        text: このチームの約束。\n        note: このチームの約束を書いた理由。\n")
}

// teamYAML is the team.yaml of the fixture's demo definition.
func (f *fixture) teamYAML() string {
	return filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
}

// readFile is one file as written.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// catalog is the catalog the screens read through.
func (f *fixture) catalog() *catalog.Catalog {
	return catalog.New(f.global, filepath.Join(f.proj, ".hergy"), f.proj, source.Disk{}, func(path string) string { return path }).InRepo(true)
}

// server builds the server over the fixture, with no activity reading: the
// fixture's project has no team running, which is what the screen says
// (spec-activity 2.7).
func (f *fixture) server(t *testing.T) *Server {
	t.Helper()
	server, err := New(f.catalog(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// fakeActivity is a reading with an answer a test writes: the servers a fixture
// builds read nothing (nothing is running), and these say what one running team
// looks like without a herdr being started.
type fakeActivity struct {
	list ActivityList
	docs map[string]ActivityDoc
}

func (f fakeActivity) List() ActivityList { return f.list }

func (f fakeActivity) One(name string) (ActivityDoc, bool) {
	doc, ok := f.docs[name]
	return doc, ok
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

// TestNAV_O1_RootGoesToTheRunningScreenOrTheDefinitions: opening the server's own
// address lands on /teams with nothing running and on /activity with a team of
// this project running, each with a 302 — not a 301, because which of the two it
// is changes as teams come and go (spec-nav.md 1.3, spec-activity 2.1, S6).
func TestNAV_O1_RootGoesToTheRunningScreenOrTheDefinitions(t *testing.T) {
	// Nothing running: the team definitions.
	f := newFixture(t)
	resp, body := f.get(t, f.server(t), "/")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET / answers %d, want 302", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/teams" {
		t.Errorf("GET / with nothing running goes to %q, want /teams", got)
	}
	if strings.Contains(body, `class="screens"`) {
		t.Errorf("the move carries a page:\n%s", body)
	}

	// One team of this project running: the activity screen, because what the
	// human is being asked for is why they came.
	server, err := New(f.catalog(), fakeActivity{
		list: ActivityList{Items: []ActivityItem{{Name: "demo"}}, Running: 1, Herdr: ActivityNote{OK: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, _ = f.get(t, server, "/")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET / answers %d, want 302", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/activity" {
		t.Errorf("GET / with a team running goes to %q, want /activity", got)
	}
}

// TestTheActivityCountsStandOnEveryBand: the band's 活動 and 待ち are the
// activity reading's own numbers, and the activity screen's band shows the same
// ones, because it is one band on every screen (spec-activity 2.1).
func TestTheActivityCountsStandOnEveryBand(t *testing.T) {
	f := newFixture(t)
	server, err := New(f.catalog(), fakeActivity{
		list: ActivityList{Items: []ActivityItem{{Name: "demo"}}, Running: 4, Waiting: 2, Herdr: ActivityNote{OK: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, profiles := f.get(t, server, "/profiles")
	if want := `<a class="scr" href="/activity" data-screen="activity">活動<span class="n">4</span><span class="wait">待ち 2</span></a>`; !strings.Contains(profiles, want) {
		t.Errorf("the profile screen's band does not hold %s:\n%s", want, profiles)
	}
	_, activity := f.get(t, server, "/activity")
	for _, want := range []string{
		`<span class="scr on" aria-current="page" data-screen="activity">活動<span class="n">4</span><span class="wait">待ち 2</span></span>`,
		`<span class="tab">一覧</span>`,
		`<main class="list" id="list">`,
		`/static/activity-v2.css`,
		`/static/activity-v2.js`,
		`<script id="activity-words" type="application/json">`,
	} {
		if !strings.Contains(activity, want) {
			t.Errorf("the activity screen does not hold %s:\n%s", want, activity)
		}
	}
	// The opened rows are the page's own state, not a URL of their own (2.3).
	if !strings.Contains(activity, "動いているチームはありません") {
		t.Errorf("the activity screen does not carry the word for an empty list:\n%s", activity)
	}
}

// TestTheBandWritesNoActivityNumberThatIsNotThere: a project with nothing
// running, and a herdr that could not be read, both leave 活動 without a number
// and without 待ち, rather than writing a zero that is not known
// (spec-activity 2.7).
func TestTheBandWritesNoActivityNumberThatIsNotThere(t *testing.T) {
	f := newFixture(t)
	_, none := f.get(t, f.server(t), "/profiles")
	if !strings.Contains(none, `<a class="scr" href="/activity" data-screen="activity">活動</a>`) {
		t.Errorf("the band writes a number with nothing running:\n%s", none)
	}
	if strings.Contains(none, `class="wait"`) {
		t.Errorf("the band writes 待ち with nothing waiting:\n%s", none)
	}

	server, err := New(f.catalog(), fakeActivity{
		list: ActivityList{Items: []ActivityItem{}, Herdr: ActivityNote{OK: false, Text: "herdr を実行できません"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, broken := f.get(t, server, "/activity")
	if !strings.Contains(broken, `<span class="scr on" aria-current="page" data-screen="activity">活動</span>`) {
		t.Errorf("the band writes a number for a herdr it could not read:\n%s", broken)
	}
}

// TestTheBandCountsTheRolesThatHaveADocument: the band's role number is the
// number the screen's own count keeps (savedRoles().length), so a name only a
// team definition writes — a row with no document yet — is shown in the list but
// not counted, and the number does not jump when the page's script draws
// (spec-nav.md 1.0).
func TestTheBandCountsTheRolesThatHaveADocument(t *testing.T) {
	f := newFixture(t)
	f.write(t, filepath.Join("teams", "demo", "team.yaml"), "version: 1\nmission: demo\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: ghost\n    role: ghost\n    profile: coder\nturns:\n  - human -> lead\n")
	server := f.server(t)
	_, body := f.get(t, server, "/roles")
	// The list carries three names — builder, ghost and lead — and the band
	// counts the two a document stands behind (S1).
	if !strings.Contains(body, `data-screen="roles">ロール<span class="n">2</span>`) {
		t.Errorf("the band counts the roles with a document, want 2:\n%s", body)
	}
	if !strings.Contains(body, `"name":"ghost"`) {
		t.Errorf("the list does not hold the role with no document:\n%s", body)
	}
}

// TestTheBandStandsOnAnErrorAndTheGuardHolds: a screen that failed still shows
// the band, and the promises in front of every request hold (spec-nav.md 1.6).
func TestTheBandStandsOnAnErrorAndTheGuardHolds(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	server.addr = "127.0.0.1:8787"

	rec := httptest.NewRecorder()
	server.navPage(rec, http.StatusInternalServerError, tf("web.error.internal", errors.New("boom")))
	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("a failed screen answers %d, want 500", resp.StatusCode)
	}
	if !strings.Contains(string(body), `class="screens"`) || !strings.Contains(string(body), "boom") {
		t.Errorf("a failed screen shows no band and no reason:\n%s", body)
	}

	// Another Host is refused before any screen runs.
	req := httptest.NewRequest(http.MethodGet, "/profiles", nil)
	req.Host = "example.com"
	rec = httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a request for another host answers %d, want 403", rec.Code)
	}

	// A write without the token of this run never reaches its handler.
	req = httptest.NewRequest(http.MethodPost, "/api/profile/check", strings.NewReader(`{"place":"project"}`))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a write without the token answers %d, want 403", rec.Code)
	}

	// A write from another page is refused too.
	req = httptest.NewRequest(http.MethodPost, "/api/profile/check", strings.NewReader(`{"place":"project"}`))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hergy-Token", server.token)
	req.Header.Set("Origin", "http://example.com")
	rec = httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a write from another page answers %d, want 403", rec.Code)
	}
}

// TestEveryBandLinkResolves: every href the band writes answers a page or a
// move, so the band holds no dead link (spec-nav.md 2).
func TestEveryBandLinkResolves(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	seen := map[string]bool{}
	queue := []string{"/profiles", "/roles", "/nope"}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if seen[path] || strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "#") {
			continue
		}
		seen[path] = true
		resp, body := f.get(t, server, path)
		switch resp.StatusCode {
		case http.StatusOK:
			for _, href := range hrefs(body) {
				if strings.HasPrefix(href, "/") && !seen[href] {
					queue = append(queue, href)
				}
			}
		case http.StatusMovedPermanently, http.StatusFound, http.StatusNotFound:
		default:
			t.Errorf("GET %s answers %d", path, resp.StatusCode)
		}
	}
	for _, path := range []string{"/", "/profiles", "/roles", "/nope"} {
		if !seen[path] {
			t.Errorf("the walk did not reach %s", path)
		}
	}
}

// TestTheBandKeyBelongsToItsProject: the band's storage key is the project's own
// root, so two projects served from one machine do not share the place a person
// left open (spec-nav.md 1.0).
func TestTheBandKeyBelongsToItsProject(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	here, there := navKey(f.proj), navKey(other.proj)
	if here == there {
		t.Errorf("two projects share the band's key %q", here)
	}
	if len(here) != 12 {
		t.Errorf("the band's key is %q, want 12 hex digits", here)
	}
}
