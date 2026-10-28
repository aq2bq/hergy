package app

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/web"
)

// This file holds raising a member of a running team from the screens: the
// reading that names the members no agent is live for, the API that raises one
// through the launcher's own refresh, and its refusals (r-5d1a, ac-d090).

// waitLaunch waits for one launch to end, as the screens' polling does, and
// answers its last state.
func waitLaunch(t *testing.T, read *activityRead, id string) web.HQLaunch {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		launch, ok := read.LaunchState(id)
		if !ok {
			t.Fatalf("launch %s is not there", id)
		}
		if launch.Phase != "starting" && launch.Phase != "awaiting" {
			return launch
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("launch %s did not end: %+v", id, launch)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// teamNamed is one team of a running reading, by name.
func teamNamed(running web.HQRunning, name string) web.HQRunningTeam {
	for _, one := range running.Teams {
		if one.Team == name {
			return one
		}
	}
	return web.HQRunningTeam{}
}

// TestRunningNamesTheMembersNoAgentIsLiveFor: GET /api/hq/running names, for each
// running team, the members its definition has that no agent is live for: the
// ones a screen offers 起動 for (r-5d1a). The members that are running are not
// in that list, and both are still in members.
func TestRunningNamesTheMembersNoAgentIsLiveFor(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	core := teamNamed(read.Running(), "core")
	if len(core.Members) != 2 {
		t.Fatalf("core's members = %+v, want the definition's two", core.Members)
	}
	if !slices.Equal(core.NotUp, []string{"builder"}) {
		t.Fatalf("core's not_up = %v, want only builder (the lead is live)", core.NotUp)
	}
}

// TestRefreshMembersRaisesTheMemberThroughRefresh: POST /api/hq/refresh gives a
// member the definition has and no agent is live for a pane in the team's tab and
// starts it there — the launcher's refresh, the same road `hergy refresh
// <member>` takes. The answer is a start's, so the progress and the startup
// question are read at /api/hq/launches/{id} (r-5d1a).
func TestRefreshMembersRaisesTheMemberThroughRefresh(t *testing.T) {
	f, env, _ := hqFixture(t)
	log := filepath.Join(f.cfg, "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	launch, err := read.RefreshMembers(web.HQRefreshIn{Team: "core", Members: []string{"builder"}})
	if err != nil {
		t.Fatalf("RefreshMembers: %v", err)
	}
	if launch.Action != web.ActionRefresh || launch.Team != "core" || launch.Project != env.ProjectRoot {
		t.Fatalf("the answer = %+v, want a refresh of core in this project", launch)
	}
	if !slices.Equal(launch.Members, []string{"builder"}) {
		t.Fatalf("the answer names %v, want the member it raises", launch.Members)
	}
	if launch.Phase != "starting" || launch.ID == "" {
		t.Fatalf("the answer is %+v, want a start's state with an id to read", launch)
	}

	done := waitLaunch(t, read, launch.ID)
	if done.Phase != "done" {
		t.Fatalf("the raise ended %q: %s", done.Phase, done.Error)
	}
	// It is the launcher's refresh, not a boot: no tab is made, the pane is split
	// from the team's own, named after the member, and the agent is started there
	// and handed its bootstrap prompt.
	record := readFile(t, log)
	for _, want := range []string{
		"pane split p1 --direction right",
		"pane rename p9 core-builder",
		"agent start core-builder --kind claude --pane p9",
		"agent prompt core-builder",
	} {
		if !strings.Contains(record, want) {
			t.Errorf("the raise did not run %q:\n%s", want, record)
		}
	}
	if strings.Contains(record, "tab create") {
		t.Errorf("the raise made a tab:\n%s", record)
	}
	// The progress says what was done: the member was started, and the raise
	// ended.
	joined := stepTexts(done)
	for _, want := range []string{"Started core-builder", "Started: builder"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the progress does not say %q:\n%s", want, joined)
		}
	}
	// The raise is in the launch record: the band and the activity screen read
	// what a member was last handed from it.
	_, records := launchLog(t, f)
	last := records[len(records)-1]
	if last.Command != "refresh" || len(last.Members) != 1 || last.Members[0].Name != "core-builder" {
		t.Errorf("the last record = %+v, want a refresh of core-builder", last)
	}
}

// stepTexts is a launch's steps as one text, for a test that reads whether a line
// was written.
func stepTexts(launch web.HQLaunch) string {
	var out strings.Builder
	for _, step := range launch.Steps {
		out.WriteString(step.Text)
		out.WriteString("\n")
	}
	return out.String()
}

// TestRefreshMembersRefusesATeamThatIsNotRunning: a team with no tab of its own
// cannot take a member, and the screens are told so by name (r-5d1a).
func TestRefreshMembersRefusesATeamThatIsNotRunning(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	_, err := read.RefreshMembers(web.HQRefreshIn{Team: "gone", Members: []string{"builder"}})
	hq, ok := errors.AsType[*web.HQError](err)
	if !ok || hq.Status != http.StatusNotFound {
		t.Fatalf("raising in a team that is not running = %v, want 404", err)
	}
	if !strings.Contains(hq.Text, "gone") {
		t.Errorf("the refusal does not name the team: %q", hq.Text)
	}
}

// TestRefreshMembersRefusesAMemberThatIsRunning: a member that is live is not
// raised: starting it again would end the conversation it holds, and nobody asked
// for that. Nothing is started then (r-5d1a).
func TestRefreshMembersRefusesAMemberThatIsRunning(t *testing.T) {
	f, env, _ := hqFixture(t)
	log := filepath.Join(f.cfg, "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }

	_, err := read.RefreshMembers(web.HQRefreshIn{Team: "core", Members: []string{"lead"}})
	hq, ok := errors.AsType[*web.HQError](err)
	if !ok || hq.Status != http.StatusConflict {
		t.Fatalf("raising a member that is running = %v, want 409", err)
	}
	if !strings.Contains(hq.Text, "lead") {
		t.Errorf("the refusal does not name the member: %q", hq.Text)
	}
	if record, err := os.ReadFile(log); err == nil && strings.Contains(string(record), "agent start") {
		t.Errorf("a refused raise started an agent:\n%s", record)
	}
	// The live member's agent is still the blocked one it was.
	if got := teamNamed(read.Running(), "core"); len(got.Waiting) != 1 {
		t.Errorf("core's waiting members = %v, want the lead", got.Waiting)
	}
}

// TestRunningSaysWhyTheDefinitionCannotBeBuilt: a team whose definition cannot be
// built is read as the live agents alone, so it names no member it has and has
// nothing to offer 起動 for. The reading carries the reason in the words hergy
// status uses — with the way to fix it — instead of leaving the 動き column
// silent (r-fc16, 2.2).
func TestRunningSaysWhyTheDefinitionCannotBeBuilt(t *testing.T) {
	f, env, _ := hqFixture(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(raw), "    role: builder\n", "    role: 進行管理\n", 1))

	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	core := teamNamed(read.Running(), "core")
	if core.Error == "" {
		t.Fatalf("core's reading = %+v, want the reason its definition cannot be built", core)
	}
	if !strings.Contains(core.Error, "進行管理") {
		t.Errorf("the reason %q does not name the role that stops the build", core.Error)
	}
	if len(core.NotUp) != 0 {
		t.Errorf("not_up = %v, want none: the definition names no member it can start", core.NotUp)
	}
}
