package app

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// hqHerdr is a fake herdr with two workspaces and two teams, one in each. It
// answers the calls the headquarters makes: the tabs with their workspace, the
// panes hergy named, the agents, and the workspaces.
const hqHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[
    {"tab_id":"t1","label":"core","workspace_id":"w3"},
    {"tab_id":"t2","label":"ledger","workspace_id":"w2"}]}}' ;;
  "agent list") echo '{"result":{"agents":[
    {"name":"core-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1","state_change_seq":2},
    {"name":"ledger-lead","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2","state_change_seq":1}]}}' ;;
  "pane list") echo '{"result":{"panes":[
    {"pane_id":"p1","label":"core-lead","tab_id":"t1","workspace_id":"w3"},
    {"pane_id":"p2","label":"ledger-lead","tab_id":"t2","workspace_id":"w2"}]}}' ;;
  "workspace list") echo '{"result":{"workspaces":[
    {"workspace_id":"w1","label":"proj-dev","focused":false},
    {"workspace_id":"w2","label":"ledger-dev","focused":false},
    {"workspace_id":"w3","label":"scratch","focused":false}]}}' ;;
  "workspace get") echo '{"result":{"workspace":{"workspace_id":"'"$3"'","label":"proj-dev"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// hqFixture is the fixture with another project listed beside the current one,
// and a launch record of the team in each.
func hqFixture(t *testing.T) (*fixture, team.Env, string) {
	t.Helper()
	f := newFixture(t)
	mustRun(t, "new", "core")
	other := filepath.Join(filepath.Dir(f.proj), "other")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(filepath.Join(f.proj, ".hergy"), filepath.Join(other, ".hergy")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(other, ".hergy", "teams", "core"), filepath.Join(other, ".hergy", "teams", "ledger")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\nprojects:\n  - "+f.proj+"\n  - "+other+"\n")
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	for _, one := range []struct{ teamName, project string }{
		{"core", f.proj}, {"ledger", other},
	} {
		if err := log.Append(launchlog.Record{
			Time: "2026-10-24T09:00:00+09:00", Command: launchlog.CommandUp,
			Project: one.project, Layer: "local", Definition: one.teamName, Team: one.teamName,
			TeamYAML:    filepath.Join(one.project, ".hergy", "teams", one.teamName, "team.yaml"),
			Workspace:   map[string]string{f.proj: "proj-dev", other: "ledger-dev"}[one.project],
			WorkspaceID: map[string]string{f.proj: "w1", other: "w2"}[one.project],
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.herdr(t, hqHerdr)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return f, env, other
}

// TestProjectsReadsTheListedProjectsAndTheGlobalLayer: the headquarters returns
// the projects config.yaml lists, in its order, with their own counts, and the
// global layer once.
func TestProjectsReadsTheListedProjectsAndTheGlobalLayer(t *testing.T) {
	_, env, other := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	projects, err := read.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if projects.Mode != "listed" || len(projects.Projects) != 2 {
		t.Fatalf("Projects = %+v", projects)
	}
	if projects.Projects[0].Path != env.ProjectRoot || projects.Projects[1].Path != other {
		t.Errorf("the order is %q, %q", projects.Projects[0].Path, projects.Projects[1].Path)
	}
	if projects.Current.Listed != true || projects.Current.Defs != 1 {
		t.Errorf("the current project = %+v", projects.Current)
	}
	if projects.Projects[1].Defs != 1 {
		t.Errorf("the other project's definitions = %d, want 1", projects.Projects[1].Defs)
	}
	if projects.Global.Defs != len(catGlobalDefs(t, env)) {
		t.Errorf("the global definitions = %d", projects.Global.Defs)
	}
}

// TestActivityAndRunningAgreeAboutTheListedProjects: the activity screen and the
// headquarters read one judgement, so the same teams, states and workspaces come
// back from both (lead's addition 1).
func TestActivityAndRunningAgreeAboutTheListedProjects(t *testing.T) {
	_, env, other := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	list := read.List()
	running := read.Running()
	if len(list.Items) != 2 || len(running.Teams) != 2 {
		t.Fatalf("activity has %d rows, running has %d teams", len(list.Items), len(running.Teams))
	}
	states := map[string]string{}
	for _, item := range list.Items {
		states[item.Name] = item.Summary.State
		if item.Project == "" || item.ProjectName == "" || item.Workspace.Label == "" {
			t.Errorf("a row does not say its project and workspace: %+v", item)
		}
	}
	for _, one := range running.Teams {
		if !stateMatches(states[one.Team], one.Members) {
			t.Errorf("%s: running members %+v do not agree with the activity state %q", one.Team, one.Members, states[one.Team])
		}
		if one.Team == "ledger" && one.Project != other {
			t.Errorf("ledger belongs to %q, want %q", one.Project, other)
		}
	}
}

// stateMatches says the state the activity judged is one the running members can
// show: a team with a blocked member is answer, and one with no working member is
// stalled.
func stateMatches(state string, members []web.HQMember) bool {
	blocked, working := false, false
	for _, member := range members {
		blocked = blocked || member.State == "blocked"
		working = working || member.State == "active"
	}
	switch state {
	case "answer":
		return blocked
	case "working":
		return working && !blocked
	case "stalled", "turn":
		return !blocked
	}
	return false
}

// TestProjectCatalogRefusesAProjectNobodyListed: a screen reads and writes only
// the listed projects (or the one serve was started in).
func TestProjectCatalogRefusesAProjectNobodyListed(t *testing.T) {
	f, env, other := hqFixture(t)
	read := newActivityRead(env)
	if _, err := read.ProjectCatalog(other); err != nil {
		t.Fatalf("a listed project was refused: %v", err)
	}
	if _, err := read.ProjectCatalog(filepath.Join(f.proj, "..", "stranger")); err == nil {
		t.Error("a project nobody listed was accepted")
	}
}

// TestLaunchOptionsOffersTheLastWorkspaceThenTheFreeOnes: the start sheet's order
// is the last workspace, a free one, then the busy ones which cannot be chosen
// (spec 5 章).
func TestLaunchOptionsOffersTheLastWorkspaceThenTheFreeOnes(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	options, err := read.LaunchOptions(env.ProjectRoot, "core", "project")
	if err != nil {
		t.Fatal(err)
	}
	if options.LastWorkspace == nil || options.LastWorkspace.ID != "w1" {
		t.Errorf("last workspace = %+v, want w1", options.LastWorkspace)
	}
	if len(options.Workspaces) != 3 {
		t.Fatalf("workspaces = %+v", options.Workspaces)
	}
	if options.Workspaces[0].ID != "w1" || options.Workspaces[0].Why != "last" || !options.Workspaces[0].Selectable {
		t.Errorf("the first workspace = %+v, want the last one", options.Workspaces[0])
	}
	busy := options.Workspaces[len(options.Workspaces)-1]
	if busy.Selectable || busy.Team == "" {
		t.Errorf("the busy workspace = %+v, want it last and not selectable", busy)
	}
	if options.SuggestedName != "core-"+filepath.Base(env.ProjectRoot) {
		t.Errorf("suggested name = %q", options.SuggestedName)
	}
	if len(options.Clash) != 1 || options.Clash[0].Team != "core" {
		t.Errorf("clash = %+v, want the running core", options.Clash)
	}
}

// catGlobalDefs is how many team definitions the global layer holds, for a test
// that does not want to write the number down.
func catGlobalDefs(t *testing.T, env team.Env) []string {
	t.Helper()
	dir := filepath.Join(env.GlobalDir, "teams")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, entry.Name())
		}
	}
	return out
}

// copyTree lays one tree over another, for a test that needs a second project.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

// TestStartLaunchRefusesANameThatIsRunning: a start whose name a running team
// already uses is refused with the name hergy suggests (hergy up --as).
func TestStartLaunchRefusesANameThatIsRunning(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	_, err := read.StartLaunch(web.HQLaunchIn{
		Project: env.ProjectRoot, Definition: "core", Place: "project",
		Workspace: web.HQWorkspaceIn{ID: "w1"},
	})
	if err == nil {
		t.Fatal("a taken name was accepted")
	}
	if !strings.Contains(err.Error(), "core") {
		t.Errorf("the refusal does not name the team: %v", err)
	}
}

// TestStartLaunchRefusesABusyWorkspace: one workspace holds one team (d-73fc).
func TestStartLaunchRefusesABusyWorkspace(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	_, err := read.StartLaunch(web.HQLaunchIn{
		Project: env.ProjectRoot, Definition: "core", Place: "project", Name: "core-second",
		Workspace: web.HQWorkspaceIn{ID: "w3"},
	})
	if err == nil {
		t.Fatal("a busy workspace was accepted")
	}
	if !strings.Contains(err.Error(), "core") {
		t.Errorf("the refusal does not name the team in the workspace: %v", err)
	}
}

// TestGlobalOptionsSaysWhichProjectsCanTakeTheDefinition: a project that defines
// the same name itself cannot start the global one (spec 5 章).
func TestGlobalOptionsSaysWhichProjectsCanTakeTheDefinition(t *testing.T) {
	_, env, other := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	options, err := read.LaunchOptions("", "core", "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Projects) != 2 {
		t.Fatalf("projects = %+v", options.Projects)
	}
	for _, one := range options.Projects {
		want := one.Path == other
		if one.Available != want {
			t.Errorf("%s available = %v, want %v", one.Path, one.Available, want)
		}
	}
}

// TestStartLaunchRefusesAGlobalDefinitionInAShadowingProject: the project's own
// same-named definition is the one hergy would use, so the start is refused
// rather than silently starting another team (spec 5 章).
func TestStartLaunchRefusesAGlobalDefinitionInAShadowingProject(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	_, err := read.StartLaunch(web.HQLaunchIn{
		Project: env.ProjectRoot, Definition: "core", Place: "global",
		Workspace: web.HQWorkspaceIn{ID: "w1"},
	})
	if err == nil {
		t.Fatal("a global definition shadowed by the project was accepted")
	}
	if !strings.Contains(err.Error(), "core") {
		t.Errorf("the refusal does not name the definition: %v", err)
	}
}

// TestALaunchSaysWhereAStartupQuestionWaits: the progress an API caller reads
// names the member, the place and the command while a member asks a question at
// startup, and goes back to starting when the answer is in (d-d589).
func TestALaunchSaysWhereAStartupQuestionWaits(t *testing.T) {
	h := &hqRead{act: &activityRead{}}
	op := &launchOp{launch: web.HQLaunch{
		ID: "l-1", Team: "core", Definition: "core", Project: "/p", ProjectName: "p",
		Phase: "starting", Steps: []web.HQLaunchStep{},
	}}
	h.observe(op, launcher.Event{Stage: "起動しています"})
	h.observe(op, launcher.Event{
		Stage: "待っています",
		Waiting: &launcher.Waiting{
			Member: "lead", Herdr: "core-lead", Workspace: "p-dev", Tab: "core", Pane: "core-lead",
		},
	})
	state := op.snapshot()
	if state.Phase != "awaiting" || state.Waiting == nil {
		t.Fatalf("the launch = %+v, want awaiting with a waiting member", state)
	}
	if state.Waiting.Herdr != "core-lead" || state.Waiting.Pane != "core-lead" || state.Waiting.Command != "hergy up core" {
		t.Errorf("waiting = %+v", state.Waiting)
	}
	h.observe(op, launcher.Event{State: "bootstrap submitted", Member: "lead"})
	h.observe(op, launcher.Event{Stage: "送っています"})
	h.observe(op, launcher.Event{Waiting: nil})
	state = op.snapshot()
	if state.Phase != "starting" || state.Waiting != nil {
		t.Errorf("after the answer the launch = %+v, want starting", state)
	}
	h.finish(op, "done", "")
	if state = op.snapshot(); state.Phase != "done" {
		t.Errorf("the finished launch = %+v", state)
	}
}

// stateHerdr is a fake herdr whose answers are files a test may rewrite, so a
// test can make a team appear and disappear the way another serve's stop does.
// A pane close empties the tabs, panes and agents: what herdr reports after a
// team is put away.
func (f *fixture) stateHerdr(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(f.cfg, "state-herdr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tabs := filepath.Join(dir, "tabs.json")
	panes := filepath.Join(dir, "panes.json")
	agents := filepath.Join(dir, "agents.json")
	spaces := filepath.Join(dir, "workspaces.json")
	f.writeHerdrState(t, dir, true)
	script := `#!/bin/sh
d='` + dir + `'
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") cat '` + tabs + `' ;;
  "pane list") cat '` + panes + `' ;;
  "agent list") cat '` + agents + `' ;;
  "workspace list") cat '` + spaces + `' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "pane close")
    printf '{"result":{"tabs":[]}}' > '` + tabs + `'
    printf '{"result":{"panes":[]}}' > '` + panes + `'
    printf '{"result":{"agents":[]}}' > '` + agents + `'
    echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return dir
}

// writeHerdrState writes the files the stateful fake answers: one team, or
// nothing.
func (f *fixture) writeHerdrState(t *testing.T, dir string, present bool) {
	t.Helper()
	tabs := `{"result":{"tabs":[]}}`
	panes := `{"result":{"panes":[]}}`
	agents := `{"result":{"agents":[]}}`
	if present {
		tabs = `{"result":{"tabs":[{"tab_id":"t1","label":"wave","workspace_id":"w1"}]}}`
		panes = `{"result":{"panes":[{"pane_id":"p1","label":"wave-lead","tab_id":"t1","workspace_id":"w1"}]}}`
		agents = `{"result":{"agents":[{"name":"wave-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}]}}`
	}
	for name, text := range map[string]string{"tabs.json": tabs, "panes.json": panes, "agents.json": agents} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"result":{"workspaces":[{"workspace_id":"w1","label":"wave-dev","focused":false}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestStopReadsHerdrAgainAndForgetsIt: the 5-second memory of herdr must not
// decide a stop. A team another serve has just put away is gone, not stopped
// again; and the stop this serve makes shows in the very next read (r-b7e1).
func TestStopReadsHerdrAgainAndForgetsIt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "wave")
	dir := f.stateHerdr(t)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	if got := len(read.Running().Teams); got != 1 {
		t.Fatalf("the priming read holds %d teams, want wave", got)
	}
	// Another serve stops the team: herdr no longer shows it. The memory of the
	// read above still says it is there.
	f.writeHerdrState(t, dir, false)
	if _, err := read.Stop("wave"); err == nil {
		t.Fatal("stopping a team another serve put away succeeded")
	} else if hq, ok := errors.AsType[*web.HQError](err); !ok || hq.Status != http.StatusNotFound {
		t.Errorf("stopping a team that is gone = %v, want 404", err)
	}

	// This serve stops a team, and the next read must not show it again.
	f.writeHerdrState(t, dir, true)
	read.forgetHerdr()
	if got := len(read.Running().Teams); got != 1 {
		t.Fatalf("the second priming read holds %d teams", got)
	}
	if _, err := read.Stop("wave"); err != nil {
		t.Fatal(err)
	}
	if got := len(read.Running().Teams); got != 0 {
		t.Errorf("the read right after a stop holds %d teams, want none", got)
	}
}

// TestAStartDecisionReadsHerdrAgain: the name, the clash and the workspace a
// start decides on are read now, not from the 5-second memory (r-b7e1).
func TestAStartDecisionReadsHerdrAgain(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "wave")
	dir := f.stateHerdr(t)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	if got := len(read.Running().Teams); got != 1 {
		t.Fatalf("the priming read holds %d teams", got)
	}
	f.writeHerdrState(t, dir, false)
	if name, err := read.hq.startName(web.HQLaunchIn{Definition: "wave"}, env); err != nil || name != "wave" {
		t.Errorf("startName after another serve stopped the team = %q, %v", name, err)
	}
	options, err := read.LaunchOptions(env.ProjectRoot, "wave", "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Clash) != 0 {
		t.Errorf("the clash list holds %+v after the team went", options.Clash)
	}
	if _, err := read.hq.startWorkspace(web.HQWorkspaceIn{ID: "w1"}, env); err != nil {
		t.Errorf("the emptied workspace was refused: %v", err)
	}
}

// TestAnUnrecordedTeamBelongsToTheProjectItsPanesStandIn: a tab this machine has
// no launch record of is placed by where its named panes stand, so another
// project's team is not taken for this one's (d-73fc).
func TestAnUnrecordedTeamBelongsToTheProjectItsPanesStandIn(t *testing.T) {
	f := newFixture(t)
	dir := f.stateHerdr(t)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	other := filepath.Join(filepath.Dir(f.proj), "other")
	writeState(t, dir, "stray", other)
	if got := len(read.Running().Teams); got != 0 {
		t.Errorf("a team whose panes stand in another project is here: %d", got)
	}
	read.forgetHerdr()
	writeState(t, dir, "stray", f.proj)
	if got := len(read.Running().Teams); got != 1 {
		t.Errorf("a team whose panes stand in this project is not here: %d", got)
	}
}

// writeState writes the stateful fake's files for one team whose panes stand in
// one directory.
func writeState(t *testing.T, dir, name, cwd string) {
	t.Helper()
	tabs := `{"result":{"tabs":[{"tab_id":"t1","label":"` + name + `","workspace_id":"w1"}]}}`
	panes := `{"result":{"panes":[{"pane_id":"p1","label":"` + name + `-lead","tab_id":"t1","workspace_id":"w1","cwd":"` + cwd + `"}]}}`
	agents := `{"result":{"agents":[{"name":"` + name + `-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}]}}`
	for file, text := range map[string]string{"tabs.json": tabs, "panes.json": panes, "agents.json": agents} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAScreenPathSaysWhatToWrite: a path typed in a screen gets the words a
// screen writes, not the ones a person reading config.yaml gets (audit B3).
func TestAScreenPathSaysWhatToWrite(t *testing.T) {
	f := newFixture(t)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	initLocales(t)
	if _, err := read.AddProject("rel/x"); err == nil {
		t.Fatal("a relative path was accepted")
	} else if want := i18n.Tf("web.hq.path_relative", "rel/x"); !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal = %v, want %q", err, want)
	}
	if _, err := read.AddProject("  "); err == nil {
		t.Fatal("an empty path was accepted")
	} else if want := i18n.T("web.hq.path_empty"); !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal = %v, want %q", err, want)
	}
	_ = f
}

// TestServeNamesTheConfigFileOnABrokenProjectPath: a project path serve cannot
// read is a setting of config.yaml, and the sentence names that file, as validate
// does (audit B3).
func TestServeNamesTheConfigFileOnABrokenProjectPath(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nprojects:\n  - rel/x\n")
	_, stderr, code := run("serve", "--no-open", "--port", "0")
	if code == 0 {
		t.Fatal("serve with a relative project path started")
	}
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	if !strings.Contains(stderr, path) || !strings.Contains(stderr, "rel/x") {
		t.Errorf("the refusal does not name the file and the entry:\n%s", stderr)
	}
}
