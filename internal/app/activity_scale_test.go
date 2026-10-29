package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// The activity screen's real-scale fixture (spec-activity 5 章), read without a
// browser so that each check of the activity screen stands at its own size: the tests
// here hold the reading behind GET /api/activity, GET /api/activity/{team} and
// the judgement of one team, over testdata/teams-v2 overlaid with
// testdata/activity — a mock herdr and the project's team and launch records.

// ---- the fixture's own moment ------------------------------------------------------------

// The activity run is read at its own clock, not the machine's (r-f647). The
// screens write the date before a moment that is not today's (activity.Clock),
// and the fixture's times are counted back from now (@T-<分>@), so a run that
// leans on the wall clock wants "22:50 より前" around midnight where the screen,
// rightly, writes "10/20 22:50 より前": the clock the run happens to sit at is
// not the screen's business.
var (
	// e2eActivityZone is the zone the run reads its clocks in.
	e2eActivityZone = time.FixedZone("JST", 9*60*60)
	// e2eActivityNow is the moment the run's clock stands at when it begins.
	e2eActivityNow = time.Date(2026, 10, 21, 0, 30, 0, 0, e2eActivityZone)
)

// ---- laying the fixture out --------------------------------------------------------------

// copyActivityFixture lays the activity screen's real-scale fixture under one
// fixture at the run's own moment (e2eActivityNow). It is the moment a test reads
// its clocks at, so what the rows say does not depend on when the run happens.
func copyActivityFixture(t *testing.T, f *fixture, cwd string) time.Time {
	t.Helper()
	return copyActivityFixtureAt(t, f, cwd, e2eActivityNow)
}

// copyActivityFixtureAt lays the activity screen's real-scale fixture under one
// fixture: bin/, herdr/ and gy/ under the config directory hergy reads the fake
// binaries from, project/ over the project (on top of the team definitions the
// teams screen's fixture holds), and state/ under the state directory. It
// returns the moment the fixture's own places are counted from, so a test can
// say what the lengths should read: @T-<minutes>@ is that many minutes before it
// (it is what the design's fixture writes).
//
// The teams the fixture shows were booted from the definitions of the teams
// screen's own fixture, so both of its layers are laid out: the project's
// .hergy, which holds the definitions and their roles, and the machine's hergy,
// which holds the profiles the leads read — without it the definitions do not
// build, and the rows would show the sentence hergy status writes for one that
// does not.
func copyActivityFixtureAt(t *testing.T, f *fixture, cwd string, at time.Time) time.Time {
	t.Helper()
	src := filepath.Join(cwd, "..", "..", "testdata")
	copyDir(t, filepath.Join(src, "teams-v2", "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "teams-v2", "config", "hergy"), filepath.Join(f.cfg, "hergy"))
	work := filepath.Join(f.cfg, "activity")
	for _, part := range []string{"bin", "herdr", "gy"} {
		copyActivityPlaces(t, filepath.Join(src, "activity", part), filepath.Join(work, part), at, f.proj)
	}
	copyActivityPlaces(t, filepath.Join(src, "activity", "project"), f.proj, at, f.proj)
	copyActivityPlaces(t, filepath.Join(src, "activity", "state"), f.state, at, f.proj)
	// The screen reads this project's language the way every command does.
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	t.Setenv("HERGY_HERDR", filepath.Join(work, "bin", "herdr"))
	t.Setenv("HERGY_GY", filepath.Join(work, "bin", "gy"))
	f.herdrSocket = startActivityHerdrSocket(t, work)
	return at
}

// activityPlace is one place the fixture leaves in its files: @T-<minutes>@, a
// moment that many minutes before the fixture was laid out.
var activityPlace = regexp.MustCompile(`@T-([0-9]+)@`)

// copyActivityPlaces copies one directory tree, filling in the places the
// fixture leaves (@T-<minutes>@ and @PROJECT@) and keeping every file's own
// permission bits, because the fake herdr and gy are scripts that must stay
// runnable. A name ending in .tmpl is written without it: the fixture marks the
// files whose places are filled in, and hergy's own state files carry no such
// suffix.
func copyActivityPlaces(t *testing.T, from, to string, at time.Time, project string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, strings.TrimSuffix(rel, ".tmpl"))
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, activityPlaces(raw, at, project), info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// activityPlaces fills in one file's places: @PROJECT@ is the project the teams
// were launched from, and @T-<minutes>@ is that many minutes before now, as
// RFC 3339 — the shape hergy's own state files write.
func activityPlaces(raw []byte, at time.Time, project string) []byte {
	text := strings.ReplaceAll(string(raw), "@PROJECT@", project)
	text = activityPlace.ReplaceAllStringFunc(text, func(place string) string {
		minutes, err := strconv.Atoi(strings.Trim(place, "@T-"))
		if err != nil {
			return place
		}
		return at.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339)
	})
	return []byte(text)
}

// copyDir copies a directory tree.
func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}

// ---- the serve the reads stand on --------------------------------------------------------

// serveEnv is the environment the screens read, built the way Run builds it.
func serveEnv(t *testing.T) team.Env {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// startServe runs hergy serve with --no-open on a working XDG and a made-up
// herdr, and returns the URL it prints and a stop function.
func startServe(t *testing.T, f *fixture) (string, func()) {
	t.Helper()
	if f != nil && f.herdrSocket != "" {
		if os.Getenv("HERDR_SOCKET_PATH") == "" {
			t.Setenv("HERDR_SOCKET_PATH", f.herdrSocket)
		}
	} else {
		startHerdrCLIProxy(t)
	}
	env := serveEnv(t)
	opts := options.Options{Port: 0, NoOpen: true}
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, env, &opts, urlWriter{urls}, io.Discard)
	}()
	select {
	case url := <-urls:
		return url, func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("serve did not stop")
			}
		}
	case err := <-done:
		cancel()
		t.Fatalf("serve stopped before it answered: %v", err)
	case <-time.After(20 * time.Second):
		cancel()
		t.Fatal("serve printed no URL")
	}
	return "", func() {}
}

// urlWriter picks the URL out of what runServe prints.
type urlWriter struct{ urls chan string }

func (w urlWriter) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(string(p), "\n") {
		if i := strings.Index(line, "http://"); i >= 0 {
			select {
			case w.urls <- strings.TrimSpace(line[i:]):
			default:
			}
		}
	}
	return len(p), nil
}

// navGet asks the real serve for one URL without following a move, so the
// answer itself — its status and its page — is what the test reads.
func navGet(t *testing.T, base, path string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(body)
}

// ---- reading the API's answers -----------------------------------------------------------

// readJSONText reads one answer of the serve into one value.
func readJSONText(t *testing.T, what, text string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), into); err != nil {
		t.Fatalf("%s is not JSON: %v: %s", what, err, text)
	}
}

// The two answers the screen is built from, as the boundary's shape (3 章): the
// parts a walk reads, and no more.
type activityListItem struct {
	Name    string `json:"name"`
	Summary struct {
		Definition string         `json:"definition"`
		State      string         `json:"state"`
		Since      *activitySince `json:"since"`
	} `json:"summary"`
	Error string `json:"error"`
}

// activitySince is a length and the moment it began, in the server's words (A4).
type activitySince struct {
	Len string `json:"len"`
	At  string `json:"at"`
}

// activityListAnswer is GET /api/activity's answer.
type activityListAnswer struct {
	Items   []activityListItem `json:"items"`
	Running int                `json:"running"`
	Waiting int                `json:"waiting"`
	Herdr   struct {
		OK   bool   `json:"ok"`
		Text string `json:"text"`
	} `json:"herdr"`
}

// activityItemNames is the list's teams in the order it answers them.
func activityItemNames(items []activityListItem) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return strings.Join(names, ",")
}

// activityWrite asks the serve for one write on the activity kind, which is a
// screen someone reads (3 章).
func activityWrite(t *testing.T, base, method, path string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

// activityCode is one script with its comments taken out: what the screen's own
// code says, rather than what its notes say about it (V1).
func activityCode(script string) string {
	code := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(script, "")
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if before, _, ok := strings.Cut(line, "//"); ok {
			lines[i] = before
		}
	}
	return strings.Join(lines, "\n")
}

// activityScript is the activity screen's own source: every module the screen is
// made of, under internal/web/static/activity. Before the rewrite it was the one
// classic script static/activity-v2.js (webui-preact-design 6 章).
func activityScript(t *testing.T, cwd string) string {
	t.Helper()
	dir := filepath.Join(cwd, "..", "web", "static", "activity")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the activity screen's files: %v", err)
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		b.WriteString(readFile(t, filepath.Join(dir, e.Name())))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		t.Fatalf("the activity screen has no script under %s", dir)
	}
	return b.String()
}

// serveAt is the run's own moment: the serve reads its clock from it, so a walk
// over the fixture is the same wherever and whenever it happens (r-f647). The
// caller puts the clock back when the test is done.
func serveAt(t *testing.T) {
	t.Helper()
	was := serveClock
	serveClock = func() time.Time { return e2eActivityNow }
	t.Cleanup(func() { serveClock = was })
}

// ---- ACTIVITY-C4 -------------------------------------------------------------------------

// TestACTIVITY_C4_judgesATeamFromTheAgents judges one team from the fixture's
// agent states and seq: core waits for the human, its lead alone handing the
// turn over (handed), and its members stand in the order 5.2 lists them — the
// one waiting for the human first, then the stopped ones, the newest stop first
// (6 A3). No member of this fixture is 動いていない (absent): every member the
// definition names has a live agent.
func TestACTIVITY_C4_judgesATeamFromTheAgents(t *testing.T) {
	useJapanese(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	at := copyActivityFixture(t, f, cwd)

	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	r := newActivityRead(env)
	r.port = serveHerdrPort()
	r.now = func() time.Time { return at }

	doc, ok := r.One("core")
	if !ok {
		t.Fatal("core is running, so its opened part is there")
	}
	// The team waits for the human, and the line is about the lead that handed
	// the turn over.

	// The fixture's agents: the lead handed the turn to the human, every other
	// member stopped (analyst's agent reports done, which is one way to stop).
	wantState := map[string]string{
		"lead": "handed", "impl-a": "idle", "impl-b": "idle", "reviewer": "idle", "analyst": "idle",
	}
	names := make([]string, 0, len(doc.Nodes))
	for _, node := range doc.Nodes {
		names = append(names, node.Name)
		if want, ok := wantState[node.Name]; ok && node.State != want {
			t.Errorf("core's %s reads %q, want %q", node.Name, node.State, want)
		}
		if node.State == "absent" {
			t.Errorf("core's %s reads 動いていない although the fixture runs an agent for it", node.Name)
		}
	}
	// メンバーの並び: 人間を待つ・止まっている（止まった時刻の新しい順）・動いていない.
	if got := strings.Join(names, ","); got != "lead,impl-a,impl-b,reviewer,analyst" {
		t.Errorf("core's members stand %q, want the newest stop first", got)
	}
	if len(doc.Nodes) != 5 {
		t.Errorf("core lists %d members, want its own five", len(doc.Nodes))
	}
}

// ---- ACTIVITY-C5 -------------------------------------------------------------------------

// TestACTIVITY-C5_saysHowLongFromTheFixedClock makes every length and every
// moment from a fixed "now" and the fixture's stopped moments: core waits
// 2 時間 40 分 from a clock, and core-hotfix is stopped 4 時間 5 分以上 from a
// moment that is a lower bound. The moment is placed so every stopped moment falls on the same
// calendar day as it, so the clock is written without a date; the wall clock is
// never read, and the screen's own script makes none of these words (V1).
func TestACTIVITY_C5_saysHowLongFromTheFixedClock(t *testing.T) {
	useJapanese(t)
	wasZone := time.Local
	time.Local = e2eActivityZone
	t.Cleanup(func() { time.Local = wasZone })
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// The reading moment is put ~13 hours after e2eActivityNow, so that every
	// stopped moment the fixture counts back from it (2 時間 40 分 and 4 時間
	// 5 分 ago) stands on the same calendar day as it and the clock is written
	// alone ("10:50 から", not "10/20 22:50 から").
	at := e2eActivityNow.Add(13 * time.Hour)
	f := newFixture(t)
	copyActivityFixtureAt(t, f, cwd, at)

	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	r := newActivityRead(env)
	r.port = serveHerdrPort()
	r.now = func() time.Time { return at }

	// core: the lead handed the turn over 2 時間 40 分 ago, and the moment it
	// began reads as a clock with から.
	list := r.List()
	var coreRow *web.ActivityItem
	var hotfixRow *web.ActivityItem
	for i := range list.Items {
		switch list.Items[i].Name {
		case "core":
			coreRow = &list.Items[i]
		case "core-hotfix":
			hotfixRow = &list.Items[i]
		}
	}
	if coreRow == nil || coreRow.Summary.Since == nil {
		t.Fatalf("core's tile carries no いつから: %+v", coreRow)
	}
	if coreRow.Summary.Since.Len != "2 時間 40 分" {
		t.Errorf("core has waited %q, want 2 時間 40 分", coreRow.Summary.Since.Len)
	}
	if want := at.Add(-160*time.Minute).Format("15:04") + " から"; coreRow.Summary.Since.At != want {
		t.Errorf("core has waited since %q, want %q", coreRow.Summary.Since.At, want)
	}
	// The member the row is about carries the same moment in the words the row
	// reads: the server writes them, so the screen writes no time word of its own
	// (d-b4b9 audit 2).
	coreDoc, ok := r.One("core")
	if !ok {
		t.Fatal("core is running, so its tile data is there")
	}
	var leadNode *web.ActivityNode
	for i := range coreDoc.Nodes {
		if coreDoc.Nodes[i].Name == "lead" {
			leadNode = &coreDoc.Nodes[i]
		}
	}
	if leadNode == nil {
		t.Fatalf("core has no lead node: %+v", coreDoc.Nodes)
	}
	if leadNode.SinceAt != coreRow.Summary.Since.At {
		t.Errorf("the lead's いつから reads %q, want the tile's %q", leadNode.SinceAt, coreRow.Summary.Since.At)
	}

	// core-hotfix: its members were already stopped when the watching began, so
	// the length is a lower bound and the moment reads より前.
	if hotfixRow == nil || hotfixRow.Summary.Since == nil {
		t.Fatalf("core-hotfix's tile carries no いつから: %+v", hotfixRow)
	}
	if hotfixRow.Summary.Since.Len != "4 時間 5 分以上" || !hotfixRow.Summary.Since.Lower {
		t.Errorf("core-hotfix is stopped %q (lower=%v), want 4 時間 5 分以上", hotfixRow.Summary.Since.Len, hotfixRow.Summary.Since.Lower)
	}
	if want := at.Add(-245*time.Minute).Format("15:04") + " より前"; hotfixRow.Summary.Since.At != want {
		t.Errorf("core-hotfix is stopped since %q, want %q", hotfixRow.Summary.Since.At, want)
	}

	// V1: none of these words is the screen's own. The scripts the browser runs
	// make no length, no moment and no 何分前 of their own — the server says every
	// one of them (spec-activity 3). The screen is a set of modules since the
	// Preact rewrite (webui-preact-design 6 章), so every file under
	// static/activity/ is read.
	script := activityCode(activityScript(t, cwd))
	for _, word := range []string{"時間", "分", "秒", "から", "以上", "より前", "日", "時"} {
		if strings.Contains(script, word) {
			t.Errorf("the screen's own script says %q, which the server is the one to say", word)
		}
	}
}

// ---- ACTIVITY-S3 -------------------------------------------------------------------------

// TestACTIVITY_S3_listsTheRunningTeams reads GET /api/activity over a real serve:
// the four rows of this project's running teams in activity.Sort's order, what
// each says at a glance, and the band's two numbers. The person's own work tab
// is no team and is not listed.
func TestACTIVITY_S3_listsTheRunningTeams(t *testing.T) {
	useJapanese(t)
	serveAt(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	copyActivityFixture(t, f, cwd)
	base, stop := startServe(t, f)
	defer stop()

	resp, page := navGet(t, base, "/api/activity")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity answers %d", resp.StatusCode)
	}
	var list activityListAnswer
	readJSONText(t, "GET /api/activity", page, &list)

	if !list.Herdr.OK {
		t.Fatalf("herdr = %+v, want a herdr that read", list.Herdr)
	}
	if got := activityItemNames(list.Items); got != "core,migrate,core-hotfix,research" {
		t.Errorf("the rows are %q, want activity.Sort's order", got)
	}
	if list.Running != 4 || list.Waiting != 2 {
		t.Errorf("the band reads running=%d waiting=%d, want 4 and 2", list.Running, list.Waiting)
	}
	want := []struct{ name, state, definition string }{
		{"core", "turn", "core"},
		{"migrate", "answer", "migrate"},
		// core-hotfix was booted from core's definition, so its row names 定義 core.
		{"core-hotfix", "stalled", "core"},
		{"research", "working", "research"},
	}
	if len(list.Items) != len(want) {
		t.Fatalf("the list holds %d rows, want %d", len(list.Items), len(want))
	}
	for i, one := range want {
		item := list.Items[i]
		if item.Name != one.name || item.Summary.State != one.state || item.Summary.Definition != one.definition {
			t.Errorf("tile %d is %s/%s/%s, want %s/%s/%s", i,
				item.Name, item.Summary.State, item.Summary.Definition,
				one.name, one.state, one.definition)
		}
	}
	// 人の作業用のタブ (the fixture's "notes") is no team of this project.
	for _, item := range list.Items {
		if item.Name == "notes" {
			t.Errorf("the list holds %q, which is a person's work tab and no team", item.Name)
		}
	}
}

// ---- ACTIVITY-C2 -------------------------------------------------------------------------

// TestACTIVITY_C2_readsOnlyTileData reads the read-only API and confirms that
// gy is not queried and the opened tile no longer carries the detail fields.
func TestACTIVITY_C2_readsOnlyTileData(t *testing.T) {
	useJapanese(t)
	serveAt(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	copyActivityFixture(t, f, cwd)
	work := filepath.Join(f.cfg, "activity")
	log := filepath.Join(work, "gy-calls.log")
	wrap := filepath.Join(work, "bin", "gy-log")
	writeFile(t, wrap, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+strconv.Quote(log)+
		"\nexec "+strconv.Quote(filepath.Join(work, "bin", "gy"))+" \"$@\"\n")
	if err := os.Chmod(wrap, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", wrap)
	base, stop := startServe(t, f)
	defer stop()

	resp, _ := navGet(t, base, "/api/activity")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity answers %d", resp.StatusCode)
	}
	resp, page := navGet(t, base, "/api/activity/core")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity/core answers %d", resp.StatusCode)
	}
	if strings.TrimSpace(calls(t, log)) != "" {
		t.Errorf("the activity API called gy: %s", strings.TrimSpace(calls(t, log)))
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal([]byte(page), &shape); err != nil {
		t.Fatalf("GET /api/activity/core is not JSON: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(shape["doc"], &doc); err != nil {
		t.Fatalf("the tile document is not JSON: %v", err)
	}
	for _, field := range []string{"tail", "questions", "gy", "role", "last"} {
		if _, ok := doc[field]; ok {
			t.Errorf("the tile document still carries unused field %q", field)
		}
	}
	if _, ok := doc["question"]; !ok {
		t.Error("the tile document has no question field for the waiting row")
	}
	for _, one := range []struct{ method, path string }{
		{"POST", "/api/activity/check"},
		{"PUT", "/api/activity/core"},
		{"DELETE", "/api/activity/core"},
	} {
		resp, body := activityWrite(t, base, one.method, one.path)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s answers %d, want 405", one.method, one.path, resp.StatusCode)
		}
		if got := strings.TrimSpace(body); got != i18n.T("web.error.read_only") {
			t.Errorf("%s %s says %q, want the read-only sentence", one.method, one.path, got)
		}
	}
}

// ---- ACTIVITY-E1 -------------------------------------------------------------------------

// TestACTIVITY-E1_saysWhenHerdrIsNotThere reads GET /api/activity with a herdr
// that cannot be read and with a herdr that reports no tab at all: the first
// answers with herdr's own sentence and no number, the second with no row and a
// running count of zero. Each case stands on a serve of its own.
func TestACTIVITY_E1_saysWhenHerdrIsNotThere(t *testing.T) {
	useJapanese(t)
	serveAt(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("herdr is not there", func(t *testing.T) {
		f := newFixture(t)
		copyActivityFixture(t, f, cwd)
		t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-herdr"))
		t.Setenv("HERDR_SOCKET_PATH", filepath.Join(f.cfg, "no-herdr.sock"))
		base, stop := startServe(t, f)
		defer stop()

		resp, page := navGet(t, base, "/api/activity")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/activity answers %d", resp.StatusCode)
		}
		var list activityListAnswer
		readJSONText(t, "GET /api/activity", page, &list)

		if list.Herdr.OK || list.Herdr.Text == "" {
			t.Errorf("herdr = %+v, want the sentence of a herdr that is not there", list.Herdr)
		}
		if len(list.Items) != 0 || list.Running != 0 || list.Waiting != 0 {
			t.Errorf("a herdr that could not be read still made %+v", list)
		}
	})

	t.Run("no tabs", func(t *testing.T) {
		f := newFixture(t)
		copyActivityFixture(t, f, cwd)
		writeFile(t, filepath.Join(f.cfg, "activity", "herdr", "tabs.json"), `{"result":{"tabs":[]}}`+"\n")
		base, stop := startServe(t, f)
		defer stop()

		resp, page := navGet(t, base, "/api/activity")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/activity answers %d", resp.StatusCode)
		}
		var list activityListAnswer
		readJSONText(t, "GET /api/activity", page, &list)

		if !list.Herdr.OK {
			t.Errorf("herdr = %+v, want a herdr that read", list.Herdr)
		}
		if len(list.Items) != 0 || list.Running != 0 {
			t.Errorf("a herdr with no tab still made %+v, want no row and running 0", list)
		}
	})
}
