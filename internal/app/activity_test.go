package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// The activity screen's reading (internal/app/activity.go, spec-activity 6 A1)
// over a small stand-in for the real-scale fixture (spec-activity 5): a project
// with core's definition, two teams of it running (core, and core-hotfix booted
// with --as), one worker of core's, one agent of a person's tab, and one team of
// another project. herdr and gy are the fixtures this package already uses, with
// their answers written as files.

// activityNow is the clock the tests read the screen at: a fixed moment, so
// every length and every time is one a test can write down.
var activityNow = time.Date(2026, 10, 21, 13, 52, 0, 0, time.Local)

// inJapanese reads the screen in Japanese: the words this screen writes are the
// ones the fixture and the spec spell out (3 V1). The language is put back, so a
// test in this package that reads something else is not left reading Japanese.
func inJapanese(t *testing.T) {
	t.Helper()
	useJapanese(t)
	t.Cleanup(func() {
		if err := i18n.Use(i18n.Fallback); err != nil {
			t.Error(err)
		}
	})
}

// useJapanese loads the locales and reads in Japanese from here on. Every command
// picks the language again when it runs (app.go), so a test that runs one asks
// for Japanese again afterwards.
func useJapanese(t *testing.T) {
	t.Helper()
	initLocales(t)
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
}

// calls is every call a fake was given, and empty when the fake was never called
// (a script that is never run writes no log).
func calls(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

// activityFixture is the project, the fake herdr and the fake gy.
type activityFixture struct {
	*fixture
	// herdrLog and gyLog hold every call the fakes were given.
	herdrLog string
	gyLog    string
}

// newActivityFixture writes the project and both fakes, and the running teams'
// records, and points the reader at them.
func newActivityFixture(t *testing.T) *activityFixture {
	t.Helper()
	f := newFixture(t)
	mustRun(t, "new", "core")
	// A command picks the language itself when it runs, so Japanese is asked for
	// after the one that made the project.
	inJapanese(t)
	// core's definition: two members, one of them handing the turn to the human,
	// and one worker a member hired (its agent runs in a tab of its own).
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml"), "version: 1\n"+
		"description: 注文 API を作り直す\n"+
		"members:\n"+
		"  - name: lead\n    role: lead\n    profile: fast\n"+
		"  - name: impl-a\n    role: builder\n    profile: fast\n"+
		"workers:\n"+
		"  - from: lead\n    purpose: 依存の更新\n    profile: fast\n"+
		"turns:\n"+
		"  - human -> lead\n"+
		"  - lead -> human\n"+
		"  - lead -> impl-a\n"+
		"  - impl-a -> lead\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), profilesYAML("--fast", "--slow"))
	// A project that uses gy: gycli reads the questions only where gy.toml is.
	writeFile(t, filepath.Join(f.proj, "gy.toml"), "# the fixture's marker\n")

	af := &activityFixture{fixture: f}
	af.herdrLog = af.activityHerdr(t)
	af.gyLog = af.activityGy(t)
	af.activityAnswers(t)

	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	for _, one := range []struct{ team, definition string }{
		{"core", "core"},
		// core-hotfix runs from core's definition, so its row carries 定義 core.
		{"core-hotfix", "core"},
	} {
		record := launchlog.Record{
			Time: activityNow.Format(time.RFC3339), Command: launchlog.CommandUp,
			Hergy: "hergy (test)", Project: f.proj, Layer: "local",
			Definition: one.definition, Team: one.team,
			TeamYAML: filepath.Join(f.proj, ".hergy", "teams", one.definition, "team.yaml"),
			Hash:     "test",
		}
		if err := log.Append(record); err != nil {
			t.Fatal(err)
		}
	}
	return af
}

// activityHerdr writes a fake herdr over this test's own answers, and returns the
// file it records every call in.
func (f *activityFixture) activityHerdr(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(f.cfg, "activity-herdr")
	log := filepath.Join(f.cfg, "activity-herdr.log")
	script := `#!/bin/sh
d='` + dir + `'
log='` + log + `'
printf '%s\n' "$*" >> "$log"
if [ -z "$2" ]; then
  echo "herdr $1"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $1 $x"; done
  exit 0
fi
case "$1 $2" in
  "tab list")   cat "$d/tabs.json" ;;
  "agent list") cat "$d/agents.json" ;;
  "agent read")
    if [ -f "$d/screens/$3.txt" ]; then cat "$d/screens/$3.txt"; else echo '{"error":{"code":"agent_not_found"}}'; exit 1; fi ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// activityGy writes a fake gy over this test's own answers, and returns the file
// it records every call in.
func (f *activityFixture) activityGy(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(f.cfg, "activity-gy")
	log := filepath.Join(f.cfg, "activity-gy.log")
	script := `#!/bin/sh
d='` + dir + `'
log='` + log + `'
printf '%s\n' "$*" >> "$log"
case "$*" in
  "list --type question --status open --json") cat "$d/open.json" ;;
  "list --since 0 --type question --json") cat "$d/writes.json" ;;
  "show "*" --json") if [ -f "$d/show/$2.json" ]; then cat "$d/show/$2.json"; else echo '[]'; fi ;;
  *) echo '[]' ;;
esac
`
	path := filepath.Join(f.cfg, "gy")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", path)
	return log
}

// activityAnswers writes what the two fakes answer: herdr's tabs and agents,
// core-lead's pane, and gy's questions.
func (f *activityFixture) activityAnswers(t *testing.T) {
	t.Helper()
	// Tabs: core's own, core-hotfix's (booted from core's definition), the tab
	// hergy made for the worker core-deps, one team of another project, and a
	// person's tab. The worker's tab is what says core-deps is core's: the name
	// alone would hand it to no one.
	f.writeHerdr(t, "tabs.json", `{"result":{"tabs":[
 {"tab_id":"t1","label":"core"},
 {"tab_id":"t2","label":"core-hotfix"},
 {"tab_id":"t3","label":"worker:core/deps"},
 {"tab_id":"t4","label":"other"},
 {"tab_id":"t5","label":"notes"}]}}`)
	f.writeHerdr(t, "agents.json", `{"result":{"agents":[
 {"name":"core-lead","agent_status":"idle","tab_id":"t1","state_change_seq":90},
 {"name":"core-impl-a","agent_status":"idle","tab_id":"t1","state_change_seq":88},
 {"name":"core-hotfix-impl-a","agent_status":"idle","tab_id":"t2","state_change_seq":30},
 {"name":"core-deps","agent_status":"idle","tab_id":"t3","state_change_seq":80},
 {"name":"other-lead","agent_status":"working","tab_id":"t4","state_change_seq":5},
 {"name":"notes","agent_status":"idle","tab_id":"t5","state_change_seq":5}]}}`)
	f.writeHerdr(t, filepath.Join("screens", "core-lead.txt"),
		"⏺ impl-a と impl-b の差分をまとめました。\n\n  q-7c2e に 2 つの案を書きました。\n\n")
	// gy: three questions of core-lead's, one of another writer's, and one that
	// nobody's team read should ever ask about.
	writeFile(t, filepath.Join(f.cfg, "activity-gy", "open.json"), `[
 {"id":"q-7c2e","kind":"Question","status":"open","title":"注文 API のページ送りを cursor に替えるか","created":"`+activityNow.Add(-165*time.Minute).Format(time.RFC3339)+`"},
 {"id":"q-51b9","kind":"Question","status":"open","title":"注文の取り消しを 24 時間までに限るか","created":"`+activityNow.Add(-25*time.Hour).Format(time.RFC3339)+`"},
 {"id":"q-0d4a","kind":"Question","status":"open","title":"結合テストを夜の CI にまわしてよいか","created":"`+activityNow.Add(-50*time.Hour).Format(time.RFC3339)+`"},
 {"id":"q-9e10","kind":"Question","status":"open","title":"索引を足す時間帯を決めてほしい","created":"`+activityNow.Add(-250*time.Minute).Format(time.RFC3339)+`"}]`)
	writeFile(t, filepath.Join(f.cfg, "activity-gy", "writes.json"), `[
 {"actor":"core-lead","node":"q-7c2e","what":"created","why":"question open q-7c2e"},
 {"actor":"core-lead","node":"q-51b9","what":"created","why":"question open q-51b9"},
 {"actor":"core-lead","node":"q-0d4a","what":"created","why":"question open q-0d4a"},
 {"actor":"core-hotfix-impl-a","node":"q-9e10","what":"created","why":"question open q-9e10"}]`)
	writeFile(t, filepath.Join(f.cfg, "activity-gy", "show", "q-7c2e.json"),
		`[{"data":{"Question":{"decider":"human","options":["cursor に替える","offset のままにする"]}}}]`)
	for _, id := range []string{"q-51b9", "q-0d4a", "q-9e10"} {
		writeFile(t, filepath.Join(f.cfg, "activity-gy", "show", id+".json"), `[{"data":{"Question":{}}}]`)
	}
}

// writeHerdr writes one file under the fake herdr's own directory.
func (f *activityFixture) writeHerdr(t *testing.T, name, text string) {
	t.Helper()
	writeFile(t, filepath.Join(f.cfg, "activity-herdr", name), text)
}

// stateFile writes the watcher's memory (6 A2), so a serve that is started while
// the agents are already stopped knows when they stopped.
func (f *activityFixture) stateFile(t *testing.T, entries map[string]string) {
	t.Helper()
	keys := slices.Sorted(maps.Keys(entries))
	parts := make([]string, 0, len(entries))
	for _, key := range keys {
		parts = append(parts, strconv.Quote(key)+":"+entries[key])
	}
	writeFile(t, filepath.Join(f.state, "hergy", "activity.json"), "{"+strings.Join(parts, ",")+"}")
}

// reader builds the reading the server would be handed, at the fixed clock.
func (f *activityFixture) reader(t *testing.T) *activityRead {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	return read
}

// entry is one watcher entry as the state file writes it.
func entry(seq int, at time.Time, lower bool) string {
	return `{"seq":` + strconv.Itoa(seq) + `,"at":"` + at.Format(time.RFC3339) + `","lower":` +
		map[bool]string{true: "true", false: "false"}[lower] + `}`
}

// TestActivityListReadsTheRunningTeams: the list holds this project's running
// teams in the order of activity.Sort, with the state, the member the line is
// about and how long it has been that way, and the band's two numbers
// (spec-activity 2.2, 5.3).
func TestActivityListReadsTheRunningTeams(t *testing.T) {
	f := newActivityFixture(t)
	// The agents were already stopped when this serve started: core-lead two
	// hours forty minutes ago, and core-hotfix's members four hours and five
	// minutes ago, as a lower bound (5.3).
	f.stateFile(t, map[string]string{
		"core-lead":          entry(90, activityNow.Add(-160*time.Minute), false),
		"core-impl-a":        entry(88, activityNow.Add(-167*time.Minute), false),
		"core-hotfix-impl-a": entry(30, activityNow.Add(-245*time.Minute), true),
		"core-deps":          entry(80, activityNow.Add(-210*time.Minute), false),
	})

	list := f.reader(t).List()

	if !list.Herdr.OK || list.Herdr.Text != "" {
		t.Errorf("herdr = %+v, want a herdr that read", list.Herdr)
	}
	if list.Running != 2 || list.Waiting != 1 {
		t.Errorf("running/waiting = %d/%d, want 2/1", list.Running, list.Waiting)
	}
	if len(list.Items) != 2 {
		t.Fatalf("the list holds %d teams, want core and core-hotfix: %+v", len(list.Items), list.Items)
	}

	// core waits for the human (its lead handed the turn over), and it comes
	// before a stopped team (spec-activity 2.2).
	core := list.Items[0]
	if core.Name != "core" || core.Summary.State != "turn" {
		t.Errorf("the first tile is %+v, want core waiting for the human", core)
	}
	if core.Summary.Definition != "core" {
		t.Errorf("core's definition = %q, want core", core.Summary.Definition)
	}
	if core.Summary.Since == nil {
		t.Fatalf("core's row has no since: %+v", core)
	}
	if core.Summary.Since.Len != "2 時間 40 分" || !strings.HasSuffix(core.Summary.Since.At, " から") {
		t.Errorf("core's since = %+v, want 2 時間 40 分 and a から", core.Summary.Since)
	}
	if got := activityNow.Add(-160 * time.Minute).Format("15:04"); !strings.Contains(core.Summary.Since.At, got) {
		t.Errorf("core's since is %q, want the moment %s", core.Summary.Since.At, got)
	}
	if core.Error != "" {
		t.Errorf("core's row carries the error %q", core.Error)
	}

	// core-hotfix stopped, and its definition is not its name: the row carries
	// the definition so the screen can write 定義 core (S4).
	hotfix := list.Items[1]
	if hotfix.Name != "core-hotfix" || hotfix.Summary.State != "stalled" {
		t.Errorf("the second tile is %+v, want core-hotfix stopped", hotfix)
	}
	if hotfix.Summary.Definition != "core" {
		t.Errorf("core-hotfix's definition = %q, want core", hotfix.Summary.Definition)
	}
	if hotfix.Summary.Since == nil || hotfix.Summary.Since.Len != "4 時間 5 分以上" || !hotfix.Summary.Since.Lower {
		t.Errorf("core-hotfix's since = %+v, want 4 時間 5 分以上", hotfix.Summary.Since)
	}
	if hotfix.Summary.Since != nil && !strings.HasSuffix(hotfix.Summary.Since.At, " より前") {
		t.Errorf("core-hotfix's since is %q, want a より前", hotfix.Summary.Since.At)
	}
}

// TestActivityListKeepsOnlyThisProject: a team running on this machine for
// another project is not the screen's, and a team this machine has no record of
// belongs here when this project's own catalog holds its definition
// (spec-activity 1).
func TestActivityListKeepsOnlyThisProject(t *testing.T) {
	f := newActivityFixture(t)

	// The same names, but the newest record of each says another project. The
	// other team has no record at all, and no definition here either.
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	for _, one := range []struct{ team, definition string }{{"core", "core"}, {"core-hotfix", "core"}} {
		if err := log.Append(launchlog.Record{
			Time: activityNow.Format(time.RFC3339), Command: launchlog.CommandUp,
			Hergy: "hergy (test)", Project: filepath.Join(f.cfg, "another"), Layer: "local",
			Definition: one.definition, Team: one.team,
			TeamYAML: filepath.Join(f.cfg, "another", one.definition, "team.yaml"), Hash: "test",
		}); err != nil {
			t.Fatal(err)
		}
	}

	list := f.reader(t).List()

	if list.Running != 0 || len(list.Items) != 0 {
		t.Fatalf("the list holds %+v, want nothing of this project", list.Items)
	}
	// The band writes no number at all then, rather than a zero it does not know.
	if !list.Herdr.OK {
		t.Errorf("herdr = %+v, want a herdr that read", list.Herdr)
	}
}

// TestActivityListSaysWhenHerdrIsNotThere: a herdr that cannot be reached is one
// line in its own words, and no row and no number (spec-activity 2.7).
func TestActivityListSaysWhenHerdrIsNotThere(t *testing.T) {
	f := newActivityFixture(t)
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-such-herdr"))

	list := f.reader(t).List()

	if list.Herdr.OK || list.Herdr.Text == "" {
		t.Fatalf("herdr = %+v, want the sentence of a herdr that is not there", list.Herdr)
	}
	if list.Running != 0 || list.Waiting != 0 || len(list.Items) != 0 {
		t.Errorf("a herdr that could not be read still made %+v", list)
	}
}

// TestActivityListLeavesGyAlone: reading the list reads herdr, the launch record
// and the team definitions, and never gy or a pane: the rows are about how the
// teams are, and the questions come with the row that is opened (spec-activity 3,
// V2).
func TestActivityListLeavesGyAlone(t *testing.T) {
	f := newActivityFixture(t)

	f.reader(t).List()

	if calls := calls(t, f.gyLog); calls != "" {
		t.Errorf("the list called gy:\n%s", calls)
	}
	log := calls(t, f.herdrLog)
	if !strings.Contains(log, "tab list") || !strings.Contains(log, "agent list") {
		t.Errorf("the list did not read herdr:\n%s", log)
	}
	if strings.Contains(log, "agent read") {
		t.Errorf("the list read a pane:\n%s", log)
	}
}

// TestActivityListReadsHerdrOnEveryRequest: the current Herdr snapshot is read
// for every request, so a later answer cannot use an old state.
func TestActivityListReadsHerdrOnEveryRequest(t *testing.T) {
	f := newActivityFixture(t)
	read := f.reader(t)

	read.List()
	read.List()

	if n := strings.Count(calls(t, f.herdrLog), "tab list"); n != 2 {
		t.Errorf("herdr was asked for its tabs %d times in two reads, want 2", n)
	}
}

func TestAskingLineKeepsOnlyOneUsefulLine(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"question", "●  続けてもよいですか？\nほかの行", "続けてもよいですか？"},
		{"fallback", "\n最初の行\n次の行\n", "最初の行"},
		{"empty", " \n \n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := askingLine(tc.text); got != tc.want {
				t.Errorf("askingLine(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestActivityOneReadsTheTileMembersAndQuestion: one team's tile carries its
// members, workers and only the question line shown on its waiting row.
func TestActivityOneReadsTheTileMembersAndQuestion(t *testing.T) {
	f := newActivityFixture(t)
	f.stateFile(t, map[string]string{
		"core-lead": entry(90, activityNow.Add(-160*time.Minute), false),
	})

	doc, ok := f.reader(t).One("core")
	if !ok {
		t.Fatal("core is running, so its opened part is there")
	}
	if want := "⏺ impl-a と impl-b の差分をまとめました。"; doc.Question != want {
		t.Errorf("the waiting row question is %q, want %q", doc.Question, want)
	}

	// The members, with the one that handed the turn over marked, and core's
	// worker under the worker heading (2.5, X1).
	states := map[string]string{}
	for _, node := range doc.Nodes {
		states[node.Name] = node.State
	}
	if states["lead"] != "handed" || states["impl-a"] != "idle" {
		t.Errorf("the members are %v, want lead handed and impl-a idle", states)
	}
	if len(doc.Hired) != 1 || doc.Hired[0].Name != "deps" || doc.Hired[0].Herdr != "core-deps" {
		t.Errorf("the workers are %+v, want deps of core-deps", doc.Hired)
	}
}

// TestActivityOneLeavesAWorkingTeamAlone: a working team's opened part is the
// right-hand column alone: no member the line is about, so no question and no
// pane (spec-activity 2.4, 2.5).
func TestActivityOneLeavesAWorkingTeamAlone(t *testing.T) {
	f := newActivityFixture(t)

	// core's lead works, so the team is working.
	f.writeHerdr(t, "agents.json", `{"result":{"agents":[
 {"name":"core-lead","agent_status":"working","tab_id":"t1","state_change_seq":91},
 {"name":"core-impl-a","agent_status":"idle","tab_id":"t1","state_change_seq":88}]}}`)

	doc, ok := f.reader(t).One("core")
	if !ok {
		t.Fatal("core is running")
	}
	if len(doc.Nodes) != 2 || len(doc.Hired) != 0 {
		t.Errorf("a working team's tile has %d members and %d workers, want 2 and 0", len(doc.Nodes), len(doc.Hired))
	}
}

// TestActivityOneIsNotFoundForATeamThatIsNotRunning: the handler answers 404 for
// a team the list does not hold (spec-activity 3).
func TestActivityOneIsNotFoundForATeamThatIsNotRunning(t *testing.T) {
	f := newActivityFixture(t)
	read := f.reader(t)

	// A team of another project, and a team that stopped.
	for _, name := range []string{"other", "gone"} {
		if _, ok := read.One(name); ok {
			t.Errorf("the opened part of %s is there, want none", name)
		}
	}
}

// TestActivityOneSaysWhenTheDefinitionDoesNotRead: a definition that cannot be
// read is still a row, with the same words hergy status writes, and no member,
// question or pane (spec-activity 2.2, 6 A1).
func TestActivityOneSaysWhenTheDefinitionDoesNotRead(t *testing.T) {
	f := newActivityFixture(t)
	// A team running from a definition this project does not have.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml"), "version: 1\ndescription: \n")
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	if err := log.Append(launchlog.Record{
		Time: activityNow.Format(time.RFC3339), Command: launchlog.CommandUp,
		Hergy: "hergy (test)", Project: f.proj, Layer: "local",
		Definition: "broken", Team: "broken", TeamYAML: filepath.Join(f.proj, ".hergy", "teams", "broken", "team.yaml"),
		Hash: "test",
	}); err != nil {
		t.Fatal(err)
	}
	// broken's tab and members.
	f.writeHerdr(t, "tabs.json", `{"result":{"tabs":[{"tab_id":"t1","label":"broken"}]}}`)
	f.writeHerdr(t, "agents.json", `{"result":{"agents":[
 {"name":"broken-lead","agent_status":"idle","tab_id":"t1","state_change_seq":7}]}}`)

	read := f.reader(t)
	list := read.List()
	if len(list.Items) != 1 || list.Items[0].Name != "broken" || list.Items[0].Error == "" {
		t.Fatalf("the list is %+v, want broken with the reason its definition does not read", list.Items)
	}
	// hergy status's own words for a definition that does not read (A1).
	status := f.statusError(t, "broken")
	if list.Items[0].Error != status {
		t.Errorf("the row says %q, want hergy status's %q", list.Items[0].Error, status)
	}

	doc, ok := read.One("broken")
	if !ok {
		t.Fatal("broken's tile data is missing")
	}
	if doc.Question != "" {
		t.Errorf("a definition that does not read has question %q, want none", doc.Question)
	}
	if len(doc.Nodes) != 1 || doc.Nodes[0].Name != "lead" || doc.Nodes[0].Herdr != "broken-lead" {
		t.Errorf("the members are %+v, want the live agent broken-lead as lead", doc.Nodes)
	}
}

// statusError is what hergy status says about one running team's definition, for
// the tests that check the two agree on why a definition does not read (6 A1).
// It is read in Japanese, the language the screen is read in, because the
// sentence is the thing being compared.
func (f *activityFixture) statusError(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, code := run("status", "--json", "--lang", "ja")
	if code != 0 {
		t.Fatalf("hergy status exited %d:\n%s", code, stderr)
	}
	var list statusJSONList
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("hergy status --json is not readable: %v\n%s", err, stdout)
	}
	for _, team := range list.Teams {
		if team.Team == name {
			return team.Error
		}
	}
	t.Fatalf("hergy status does not list %s:\n%s", name, stdout)
	return ""
}

// TestActivityWatchRemembersAcrossServes: an agent whose state_change_seq did
// not move keeps the moment the earlier serve saw, and an agent that appeared
// since is taken from now on (6 A2).
func TestActivityWatchRemembersAcrossServes(t *testing.T) {
	f := newActivityFixture(t)
	// The file knows core-lead (seq 90), and nothing about core-impl-a, which
	// appeared while nobody watched.
	f.stateFile(t, map[string]string{
		"core-lead": entry(90, activityNow.Add(-160*time.Minute), false),
	})

	list := f.reader(t).List()
	core := list.Items[0]
	if core.Summary.Since == nil || core.Summary.Since.Len != "2 時間 40 分" {
		t.Errorf("core's since = %+v, want the moment the earlier serve saw", core.Summary.Since)
	}
	if core.Summary.Since != nil && core.Summary.Since.Lower {
		t.Errorf("a state the file dates is not a lower bound: %+v", core.Summary.Since)
	}
}

// TestActivityScreensThatReadTheTeamsAgree: hergy status and the screen read the
// running teams through one function (6 A1), so a herdr that cannot be read is
// the same failure to both.
func TestActivityScreensThatReadTheTeamsAgree(t *testing.T) {
	f := newActivityFixture(t)
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-such-herdr"))

	_, stderr, code := run("status")
	if code == 0 {
		t.Fatalf("hergy status succeeded with no herdr:\n%s", stderr)
	}
	list := f.reader(t).List()
	if list.Herdr.Text == "" || !strings.Contains(stderr, strings.TrimSpace(list.Herdr.Text)) {
		t.Errorf("status says %q and the screen says %q, want one reason", stderr, list.Herdr.Text)
	}
}

// TestActivityStatePath: the watcher's memory is the state directory's, so a
// hergy serve that is started again remembers (6 A2).
func TestActivityStatePath(t *testing.T) {
	f := newActivityFixture(t)
	t.Setenv("XDG_STATE_HOME", f.state)
	want := filepath.Join(f.state, "hergy", "activity.json")
	if got := activityStatePath(); got != want {
		t.Errorf("activityStatePath() = %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "relative")
	t.Setenv("HOME", filepath.Join(f.cfg, "home"))
	want = filepath.Join(f.cfg, "home", ".local", "state", "hergy", "activity.json")
	if got := activityStatePath(); got != want {
		t.Errorf("activityStatePath() = %q, want %q", got, want)
	}
}

// TestActivityTimeWordsAreTheServers: every length and every moment the screen
// writes comes from here, so the script writes none of them (spec-activity 3,
// V1). The words are spelled out rather than left to a pattern, because they are
// what a person reads.
func TestActivityTimeWordsAreTheServers(t *testing.T) {
	inJapanese(t)
	cases := []struct {
		at    time.Duration
		lower bool
		len   string
	}{
		{25 * time.Minute, false, "25 分"},
		{time.Hour + 5*time.Minute, false, "1 時間 5 分"},
		{2 * time.Hour, false, "2 時間"},
		{30 * time.Second, false, "30 秒"},
		{0, false, "1 秒"},
		{4*time.Hour + 5*time.Minute, true, "4 時間 5 分以上"},
	}
	for _, one := range cases {
		if got := activity.Length(one.at, one.lower); got != one.len {
			t.Errorf("Length(%s, %v) = %q, want %q", one.at, one.lower, got, one.len)
		}
	}
	if got := activity.SinceAt(activityNow, activityNow.Add(-18*time.Minute), false); !regexp.MustCompile(`^\d\d:\d\d から$`).MatchString(got) {
		t.Errorf("SinceAt = %q, want a clock and から", got)
	}
}

// TestActivityNodesCarryTheTerminalTitleAndTheHirer: the opened team says what
// each agent's terminal says it is doing, exactly as herdr gives the title, and
// which member added each worker, from the hire line of the launch record
// (d-b4b9). A worker with no hire line has no member, and the screen says no
// が追加 for it.
func TestActivityNodesCarryTheTerminalTitleAndTheHirer(t *testing.T) {
	f := newActivityFixture(t)
	f.writeHerdr(t, "tabs.json", `{"result":{"tabs":[
 {"tab_id":"t1","label":"core"},
 {"tab_id":"t2","label":"core-hotfix"},
 {"tab_id":"t3","label":"worker:core/deps"},
 {"tab_id":"t6","label":"worker:core/scout"},
 {"tab_id":"t4","label":"other"},
 {"tab_id":"t5","label":"notes"}]}}`)
	f.writeHerdr(t, "agents.json", `{"result":{"agents":[
 {"name":"core-lead","agent_status":"idle","tab_id":"t1","state_change_seq":90,"terminal_title_stripped":"r-7f3c の差分を読んでいる"},
 {"name":"core-impl-a","agent_status":"idle","tab_id":"t1","state_change_seq":88},
 {"name":"core-hotfix-impl-a","agent_status":"idle","tab_id":"t2","state_change_seq":30},
 {"name":"core-deps","agent_status":"idle","tab_id":"t3","state_change_seq":80,"terminal_title_stripped":"依存を更新している"},
 {"name":"core-scout","agent_status":"idle","tab_id":"t6","state_change_seq":70},
 {"name":"other-lead","agent_status":"working","tab_id":"t4","state_change_seq":5},
 {"name":"notes","agent_status":"idle","tab_id":"t5","state_change_seq":5}]}}`)
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	if err := log.Append(launchlog.Record{
		Time: activityNow.Format(time.RFC3339), Command: launchlog.CommandHire,
		Hergy: "hergy (test)", Project: f.proj, Team: "core",
		Hire: &launchlog.Hire{Worker: "core-deps", From: "lead", Purpose: "依存の更新", Tab: "worker:core/deps"},
	}); err != nil {
		t.Fatal(err)
	}

	doc, ok := f.reader(t).One("core")
	if !ok {
		t.Fatal("core is running, so its opened part is there")
	}
	lead := nodeNamed(t, doc.Nodes, "lead")
	if lead.Doing != "r-7f3c の差分を読んでいる" {
		t.Errorf("lead's doing = %q, want the title herdr gives", lead.Doing)
	}
	if implA := nodeNamed(t, doc.Nodes, "impl-a"); implA.Doing != "" {
		t.Errorf("impl-a has no title, so its doing = %q, want none", implA.Doing)
	}
	deps := nodeNamed(t, doc.Hired, "deps")
	if deps.From != "lead" {
		t.Errorf("deps was hired by %q, want lead (the hire line)", deps.From)
	}
	if deps.Doing != "依存を更新している" {
		t.Errorf("deps' doing = %q, want its title", deps.Doing)
	}
	scout := nodeNamed(t, doc.Hired, "scout")
	if scout.From != "" {
		t.Errorf("scout has no hire line, so its from = %q, want none", scout.From)
	}
}

// nodeNamed is one node of an opened team by its short name.
func nodeNamed(t *testing.T, nodes []web.ActivityNode, name string) web.ActivityNode {
	t.Helper()
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
	}
	t.Fatalf("no node %q in %+v", name, nodes)
	return web.ActivityNode{}
}

// TestATeamOfTheSameNameInTwoProjectsKeepsItsOwnDefinition: two listed projects
// run a team of the same name, and the other project's launch is the newer one.
// The tab's panes stand in this project, so the row is this project's team, read
// from this project's definition: the record of a project and a name is the one a
// team is read from, not the newest record of its name (r-87c8, d-73fc).
func TestATeamOfTheSameNameInTwoProjectsKeepsItsOwnDefinition(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "core")
	alpha := f.proj
	beta := filepath.Join(filepath.Dir(f.proj), "other")
	if err := copyTree(filepath.Join(alpha, ".hergy"), filepath.Join(beta, ".hergy")); err != nil {
		t.Fatal(err)
	}
	for _, one := range []struct{ dir, description string }{{alpha, "alpha の説明"}, {beta, "beta の説明"}} {
		path := filepath.Join(one.dir, ".hergy", "teams", "core", "team.yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		mine := ""
		for line := range strings.SplitSeq(string(raw), "\n") {
			if strings.HasPrefix(line, "description:") {
				mine = line
				break
			}
		}
		writeFile(t, path, strings.Replace(string(raw), mine, "description: "+one.description, 1))
	}

	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	// alpha's launch first, beta's second: beta's is the newest of the name core.
	for _, one := range []struct {
		when time.Time
		proj string
	}{
		{time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), alpha},
		{time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC), beta},
	} {
		if err := log.Append(launchlog.Record{
			Time: one.when.Format(time.RFC3339), Command: launchlog.CommandUp,
			Project: one.proj, Layer: "local", Definition: "core", Team: "core",
			TeamYAML: filepath.Join(one.proj, ".hergy", "teams", "core", "team.yaml"), Hash: "test",
		}); err != nil {
			t.Fatal(err)
		}
	}

	env := func(root string) team.Env {
		return team.Env{ProjectRoot: root, GlobalDir: filepath.Join(f.cfg, "hergy"), LocalDir: filepath.Join(root, ".hergy")}
	}
	views := []projectView{
		{Path: alpha, Name: "alpha", Env: env(alpha), Cat: env(alpha).Catalog(source.Disk{})},
		{Path: beta, Name: "other", Env: env(beta), Cat: env(beta).Catalog(source.Disk{})},
	}
	read := herdrNow{
		tabs: []team.Tab{{
			ID: "t1", Label: "core", WorkspaceID: "w1",
			Agents: []string{"core-lead"}, Panes: []string{"core-lead"}, PaneDirs: []string{alpha},
		}},
		agentsRead: true,
		seen:       map[string]activity.Watch{},
	}
	out := readRunningTeamsIn(views, read, false)
	if len(out) != 1 {
		t.Fatalf("the reading has %d teams, want the one running", len(out))
	}
	one := out[0]
	if !samePath(one.Project, alpha) {
		t.Errorf("the team stands in %q, want the project its panes stand in, %q", one.Project, alpha)
	}
	if one.Built == nil {
		t.Fatalf("the team's definition did not read: %q", one.Broken)
	}
	if got, want := one.Built.team.Definition.Description, "alpha の説明"; got != want {
		t.Errorf("the team runs from the definition of %q, want %q", got, want)
	}
}

// TestActivityQuestionSaysWhenTheScreenCannotBeRead: a member waiting for an
// answer whose screen herdr cannot read still has a question, in the server's
// words, rather than an empty field (r-1db7 L5).
func TestActivityQuestionSaysWhenTheScreenCannotBeRead(t *testing.T) {
	f := newActivityFixture(t)
	f.stateFile(t, map[string]string{"core-lead": entry(90, activityNow.Add(-160*time.Minute), false)})
	// core-lead waits for an answer, and herdr cannot read what its screen asks.
	f.writeHerdr(t, "agents.json", `{"result":{"agents":[
 {"name":"core-lead","agent_status":"blocked","tab_id":"t1","state_change_seq":90},
 {"name":"core-impl-a","agent_status":"idle","tab_id":"t1","state_change_seq":88},
 {"name":"core-hotfix-impl-a","agent_status":"idle","tab_id":"t2","state_change_seq":30},
 {"name":"core-deps","agent_status":"idle","tab_id":"t3","state_change_seq":80},
 {"name":"other-lead","agent_status":"working","tab_id":"t4","state_change_seq":5},
 {"name":"notes","agent_status":"idle","tab_id":"t5","state_change_seq":5}]}}`)
	if err := os.Remove(filepath.Join(f.cfg, "activity-herdr", "screens", "core-lead.txt")); err != nil {
		t.Fatal(err)
	}
	doc, ok := f.reader(t).One("core")
	if !ok {
		t.Fatal("core is running, so its opened part is there")
	}
	if want := i18n.T("web.activity.question_unread"); doc.Question != want {
		t.Errorf("the waiting row question is %q, want %q", doc.Question, want)
	}
}
