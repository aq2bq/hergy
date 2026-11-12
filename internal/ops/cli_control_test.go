package ops

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/internal/turns" // TestFocusMovesToTheMembersPane: focus moves herdr's focus to the member's pane
	// and says which member it moved to.
	// TestNudgeSendsOnlyToAnIdleOrDoneMember: a nudge reaches an idle member, headed
	// by who it is from (the human), and a working member is refused without sending.
)

func TestFocusMovesToTheMembersPane(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	logPath, _ := f.controlHerdr(t, "demo-lead idle p1 screen-lead\n")
	got := mustRun(t, "focus", "lead")
	if got != "focused demo-lead\n" {
		t.Fatalf("focus printed %q, want \"focused demo-lead\\n\"", got)
	}
	if !logHasCall(t, logPath, "agent", "focus") {
		t.Fatalf("focus did not call agent focus:\n%s", readFile(t, logPath))
	}
}

func TestNudgeSendsOnlyToAnIdleOrDoneMember(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder working p2 screen-builder\n")
	if got := mustRun(t, "nudge", "lead"); got != "sent to demo-lead: From the human: Please carry on.\n" {
		t.Fatalf("nudge printed %q", got)
	}
	want := []string{"demo-lead\tFrom the human: Please carry on."}
	if got := promptsRead(t, promptsPath); !slices.Equal(got, want) {
		t.Fatalf("prompts = %q, want %q", got, want)
	}
	_, stderr := mustFail(t, "nudge", "builder")
	if !strings.Contains(stderr, "demo-builder") {
		t.Fatalf("nudge refusal does not name the member:\n%s", stderr)
	}
	if got := promptsRead(t, promptsPath); len(got) != 1 {
		t.Fatalf("nudge of a working member sent something: %q", got)
	}
}

// TestNudgeTakesTheBodyAndThePaneItWasTypedIn: the body, when given, is what is
// sent, and a member that types the nudge is named as the sender.
func TestNudgeTakesTheBodyAndThePaneItWasTypedIn(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")
	t.Setenv("HERDR_PANE_ID", "p1")

	mustRun(t, "nudge", "builder", "the report is late", "--team", "demo")
	want := []string{"demo-builder\tFrom demo-lead: the report is late"}
	if got := promptsRead(t, promptsPath); !slices.Equal(got, want) {
		t.Fatalf("prompts = %q, want %q", got, want)
	}
}

// TestStopSendsEscOnceToAHarnessThatStopsAtOnce: stop ends a working member's
// turn with one esc and names refresh as the next step; a harness that stops on
// the first esc is never sent a second, because on some harnesses the second esc
// does something else. A member that is not working is left alone.
func TestStopSendsEscOnceToAHarnessThatStopsAtOnce(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	logPath, _ := f.controlHerdr(t, "demo-lead working p1 screen-lead\ndemo-builder idle p2 screen-builder\n")
	f.stopPlan(t, "demo-lead 1 idle\n")

	got := mustRun(t, "stop", "lead")
	for _, line := range []string{"stopped demo-lead\n", "next: refresh demo-lead --team demo\n"} {
		if !strings.Contains(got, line) {
			t.Fatalf("stop is missing %q:\n%s", line, got)
		}
	}
	if !logHasCall(t, logPath, "agent", "send-keys") {
		t.Fatalf("stop did not call agent send-keys:\n%s", readFile(t, logPath))
	}
	if n := logCallCount(t, logPath, "agent", "send-keys"); n != 1 {
		t.Fatalf("stop sent esc %d times to a member that stopped at once, want 1:\n%s", n, readFile(t, logPath))
	}

	before := readFile(t, logPath)
	got = mustRun(t, "stop", "builder")
	if !strings.Contains(got, "demo-builder") || strings.Contains(got, "next:") {
		t.Fatalf("stop of a non-working member printed %q", got)
	}
	if strings.Contains(readFile(t, logPath), "send-keys demo-builder") {
		t.Fatalf("stop of a non-working member sent esc:\n%s", readFile(t, logPath))
	}
	if !strings.Contains(before, "send-keys demo-lead") {
		t.Fatalf("stop of a working member did not send esc:\n%s", before)
	}
}

// TestStopSendsASecondEscWhenTheMemberKeepsWorking: a harness whose first esc
// only asks it to interrupt (opencode) is sent a second esc, and the stop is
// reported as done, with refresh still the next step.
func TestStopSendsASecondEscWhenTheMemberKeepsWorking(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	logPath, _ := f.controlHerdr(t, "demo-lead working p1 screen-lead\n")
	f.stopPlan(t, "demo-lead 2 idle\n")

	got := mustRun(t, "stop", "lead")
	for _, line := range []string{"stopped demo-lead\n", "next: refresh demo-lead --team demo\n"} {
		if !strings.Contains(got, line) {
			t.Fatalf("stop is missing %q:\n%s", line, got)
		}
	}
	if n := logCallCount(t, logPath, "agent", "send-keys"); n != 2 {
		t.Fatalf("stop sent esc %d times to a member that needed two, want 2:\n%s", n, readFile(t, logPath))
	}
}

// TestStopSaysWhenTwoEscsDidNotStop: a member that keeps working through both
// escs is reported as not stopped, the command fails, and refresh is named as
// the next step, because starting the member again is what is left to do.
func TestStopSaysWhenTwoEscsDidNotStop(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	logPath, _ := f.controlHerdr(t, "demo-lead working p1 screen-lead\n")

	stdout, stderr := mustFail(t, "stop", "lead")
	if !strings.Contains(stderr, "demo-lead") {
		t.Fatalf("stop of a member that did not stop does not name it:\n%s", stderr)
	}
	if !strings.Contains(stdout, "next: refresh demo-lead --team demo\n") {
		t.Fatalf("stop that did not stop does not name refresh:\n%s", stdout)
	}
	if strings.Contains(stdout, "stopped demo-lead") {
		t.Fatalf("stop of a member that did not stop says it stopped:\n%s", stdout)
	}
	if n := logCallCount(t, logPath, "agent", "send-keys"); n != 2 {
		t.Fatalf("stop sent esc %d times, want 2 before it gives up:\n%s", n, readFile(t, logPath))
	}
}

// TestStopRefusesTheMembersOwnPane: a member running in hergy's own pane cannot
// stop itself, the same reason refresh refuses it.
func TestStopRefusesTheMembersOwnPane(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead working p1 screen-lead\n")
	t.Setenv("HERDR_PANE_ID", "p1")

	_, stderr := mustFail(t, "stop", "lead")
	if !strings.Contains(stderr, "demo-lead") {
		t.Fatalf("stop refusal does not name the member:\n%s", stderr)
	}
}

// TestTellHeadsTheMessageWithItsSender: tell sends a message headed by --from,
// and the pane's member when --from is left out.
func TestTellHeadsTheMessageWithItsSender(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	mustRun(t, "tell", "builder", "please look at #42", "--from", "lead")
	want := []string{"demo-builder\tFrom demo-lead: please look at #42"}
	if got := promptsRead(t, promptsPath); !slices.Equal(got, want) {
		t.Fatalf("prompts = %q, want %q", got, want)
	}
}

// TestTellSenderComesFromThePaneElseTheHuman: with no --from, the member whose
// pane hergy runs in sends it; from any other pane it is the human.
func TestTellSenderComesFromThePaneElseTheHuman(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	t.Setenv("HERDR_PANE_ID", "p1")
	mustRun(t, "tell", "builder", "hi")
	t.Setenv("HERDR_PANE_ID", "")
	mustRun(t, "tell", "builder", "hi again")

	want := []string{
		"demo-builder\tFrom demo-lead: hi",
		"demo-builder\tFrom the human: hi again",
	}
	if got := promptsRead(t, promptsPath); !slices.Equal(got, want) {
		t.Fatalf("prompts = %q, want %q", got, want)
	}

	tells, err := turns.TakeTells(turns.TellDir(filepath.Join(f.state, "hergy"), "demo"))
	if err != nil || len(tells) != 2 || tells[0].From != "lead" || tells[0].To != "builder" || tells[1].From != "" || tells[1].To != "builder" {
		t.Fatalf("TakeTells = %+v, %v; want lead's then the person's, both to builder", tells, err)
	}
}

// TestTellToTheHumanRecordsAndSendsNothing: from a member's pane, hergy tell
// 人間 sends no prompt and records a tell to the human, so the watcher knows
// the member's text is the person's (d-e601). A body is not needed.
func TestTellToTheHumanRecordsAndSendsNothing(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	t.Setenv("HERDR_PANE_ID", "p1")
	stdout := mustRun(t, "tell", "人間")
	if !strings.Contains(stdout, "handed to the human") {
		t.Errorf("stdout = %q, want it to say the text was handed to the human", stdout)
	}
	mustRun(t, "tell", "human", "--from", "builder")
	if got := promptsRead(t, promptsPath); len(got) != 0 {
		t.Fatalf("prompts = %q, want none: nothing is sent to the human", got)
	}
	tells, err := turns.TakeTells(turns.TellDir(filepath.Join(f.state, "hergy"), "demo"))
	if err != nil || len(tells) != 2 || tells[0].From != "lead" || tells[0].To != turns.Human || tells[1].From != "builder" || tells[1].To != turns.Human {
		t.Fatalf("TakeTells = %+v, %v; want lead's then builder's tell to the human", tells, err)
	}
}

// TestTellToTheHumanFromNoMembersPaneIsRefused: the person has nothing to mark
// as their own; hergy says so and records nothing. A tell to a member still
// needs its text.
func TestTellToTheHumanFromNoMembersPaneIsRefused(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	t.Setenv("HERDR_PANE_ID", "")
	_, stderr := mustFail(t, "tell", "人間")
	if !strings.Contains(stderr, "typed in its own pane") {
		t.Errorf("stderr = %q, want the refusal to say a member types it in its own pane", stderr)
	}
	_, stderr = mustFail(t, "tell", "builder")
	if !strings.Contains(stderr, "<text>") {
		t.Errorf("stderr = %q, want the refusal to ask for the text", stderr)
	}
	tells, err := turns.TakeTells(turns.TellDir(filepath.Join(f.state, "hergy"), "demo"))
	if err != nil || len(tells) != 0 {
		t.Fatalf("TakeTells = %+v, %v; want nothing recorded", tells, err)
	}
}

// TestTellRefusesAFromOutsideTheTeam: --from names a member of the team.
func TestTellRefusesAFromOutsideTheTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	_, stderr := mustFail(t, "tell", "builder", "hi", "--from", "stranger")
	if !strings.Contains(stderr, "stranger") {
		t.Fatalf("tell refusal does not name --from:\n%s", stderr)
	}
	if got := promptsRead(t, promptsPath); got != nil {
		t.Fatalf("tell sent something for a bad --from: %q", got)
	}
}

// TestDryPrintsWhatWouldBeSentAndSendsNothing: --dry names the target and the
// body, and never reads or drives herdr.
func TestDryPrintsWhatWouldBeSentAndSendsNothing(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	_, promptsPath := f.controlHerdr(t, "demo-lead idle p1 screen-lead\ndemo-builder working p2 screen-builder\n")

	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"nudge", "lead", "--team", "demo", "--dry"}, "would send to demo-lead: From the human: Please carry on."},
		{[]string{"tell", "lead", "hello", "--from", "builder", "--team", "demo", "--dry"}, "would send to demo-lead: From demo-builder: hello"},
		{[]string{"stop", "builder", "--team", "demo", "--dry"}, "would stop demo-builder"},
	}
	for _, c := range cases {
		got := mustRun(t, c.argv...)
		if !strings.Contains(got, c.want) {
			t.Fatalf("%v printed %q, want it to contain %q", c.argv, got, c.want)
		}
	}
	if got := promptsRead(t, promptsPath); got != nil {
		t.Fatalf("--dry sent something: %q", got)
	}
}

// TestFocusRejectsDry: focus has no --dry, so it is refused at the command line
// rather than silently doing nothing.
func TestFocusRejectsDry(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead idle p1 screen-lead\n")

	_, stderr := mustFail(t, "focus", "lead", "--dry")
	if !strings.Contains(stderr, "--dry") {
		t.Fatalf("focus --dry is not refused with a flag message:\n%s", stderr)
	}
}

// TestTeamOfAgentNamesTheTeamAMemberBelongsTo: a member that types a team
// command while several teams run does not name its team; its herdr name does
// (r-103d).
func TestTeamOfAgentNamesTheTeamAMemberBelongsTo(t *testing.T) {
	running := []team.Running{{Team: fixtureRunningTeam("sig")}, {Team: fixtureRunningTeam("hergy-dev")}}
	if got := teamOfAgent(running, fixtureSeen("hergy-dev-spec")); got.String() != "hergy-dev" {
		t.Errorf("hergy-dev-spec: %q, want hergy-dev", got)
	}
	if got := teamOfAgent(running, fixtureSeen("sig-dev")); got.String() != "sig" {
		t.Errorf("sig-dev: %q, want sig", got)
	}
	if got := teamOfAgent(running, fixtureSeen("other-lead")); got.String() != "" {
		t.Errorf("other-lead: %q, want none", got)
	}
}
func TestTeamOfAgentChoosesLongestNameRegardlessOfOrder(t *testing.T) {
	for _, running := range [][]team.Running{
		{{Team: fixtureRunningTeam("a")}, {Team: fixtureRunningTeam("a-b")}},
		{{Team: fixtureRunningTeam("a-b")}, {Team: fixtureRunningTeam("a")}},
	} {
		if got := teamOfAgent(running, fixtureSeen("a-b-c-d")); got.String() != "a-b" {
			t.Fatalf("got %q", got)
		}
	}
}
