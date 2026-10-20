package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// controlHerdr writes a fake herdr for the member commands (focus, nudge, stop,
// tell) over one running team, demo. <cfg>/status.txt holds one live agent per
// line, "<name> <status> <pane> <screen>": agent list, agent get and agent read
// answer from it. Every call is recorded in <cfg>/herdr.log, and every prompt in
// <cfg>/prompts.txt as "<target>\t<text>" with newlines turned to spaces, so a
// test can read the body a member was handed.
//
// agent send-keys counts the escs sent to each member in <cfg>/esc.txt and, when
// a test wrote a stop plan, moves the member to the state that plan gives after
// that count (<cfg>/stop-plan.txt, one "<herdr name> <esc count> <status>" line
// per step). That is how the fake herdr stands for a harness that stops on the
// first esc, one that stops on the second, and one that never stops.
func (f *fixture) controlHerdr(t *testing.T, agents string) (logPath, promptsPath string) {
	t.Helper()
	status := filepath.Join(f.cfg, "status.txt")
	logPath = filepath.Join(f.cfg, "herdr.log")
	promptsPath = filepath.Join(f.cfg, "prompts.txt")
	esc := filepath.Join(f.cfg, "esc.txt")
	plan := filepath.Join(f.cfg, "stop-plan.txt")
	if err := os.WriteFile(status, []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
status='` + status + `'
log='` + logPath + `'
prompts='` + promptsPath + `'
esc='` + esc + `'
plan='` + plan + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo"}]}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r name st pane screen; do
      [ -z "$name" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}' "$name" "$st" "$pane"
    done < "$status"
    printf ']}}'
    ;;
  "agent get")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$name" ] || [ "$pane" = "$3" ]; then
        printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}}}\n' "$n" "$st" "$pane"
        exit 0
      fi
    done < "$status"
    echo '{"error":{"code":"agent_not_found"}}'
    ;;
  "agent read")
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$3" ]; then printf '%s' "$screen"; exit 0; fi
    done < "$status"
    ;;
  "agent prompt")
    printf '%s\t' "$3" >> "$prompts"
    printf '%s' "$4" | tr '\n' ' ' >> "$prompts"
    printf '\n' >> "$prompts"
    echo '{"result":{}}'
    ;;
  "agent focus")     echo '{"result":{}}' ;;
  "agent send-keys")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    n=0
    if [ -f "$esc" ]; then n=$(cat "$esc"); fi
    n=$((n+1))
    printf '%s' "$n" > "$esc"
    if [ -f "$plan" ]; then
      new=$(awk -v n="$n" -v nm="$name" '$1==nm && $2==n {print $3}' "$plan")
      if [ -n "$new" ]; then
        awk -v nm="$name" -v st="$new" '{ if ($1==nm) $2=st; print }' "$status" > "$status.new"
        mv "$status.new" "$status"
      fi
    fi
    echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return logPath, promptsPath
}

// stopPlan tells the fake herdr when each esc ends a member's turn: one
// "<herdr name> <esc count> <status>" line per step. A plan with no line for a
// member leaves it working through every esc.
func (f *fixture) stopPlan(t *testing.T, plan string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.cfg, "stop-plan.txt"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

// logCallCount is how many times the fake herdr recorded a call whose first two
// words are group and sub.
func logCallCount(t *testing.T, path, group, sub string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == group && fields[1] == sub {
			n++
		}
	}
	return n
}

// mustFail runs a command that is expected to exit non-zero and returns what a
// person would see.
func mustFail(t *testing.T, argv ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, code := run(argv...)
	if code == 0 {
		t.Fatalf("%v: exit 0, want a failure\nstdout:\n%s", argv, stdout)
	}
	return stdout, stderr
}

// promptsRead returns the prompts a fake herdr recorded, one per line.
func promptsRead(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// logHasCall reports whether the fake herdr recorded a call whose first two
// words are group and sub.
func logHasCall(t *testing.T, path, group, sub string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == group && fields[1] == sub {
			return true
		}
	}
	return false
}

// TestFocusMovesToTheMembersPane: focus moves herdr's focus to the member's pane
// and says which member it moved to.
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

// TestNudgeSendsOnlyToAnIdleOrDoneMember: a nudge reaches an idle member, headed
// by who it is from (the human), and a working member is refused without sending.
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
		{[]string{"nudge", "lead", "--dry"}, "would send to demo-lead: From the human: Please carry on."},
		{[]string{"tell", "lead", "hello", "--from", "builder", "--dry"}, "would send to demo-lead: From demo-builder: hello"},
		{[]string{"stop", "builder", "--dry"}, "would stop demo-builder"},
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

// TestStatusJSONCarriesStateAndUnchangedMinutes: status --json prints the rows
// as an object, with the unchanged count as a number after a second look.
func TestStatusJSONCarriesStateAndUnchangedMinutes(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead working p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	setClock(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	mustRun(t, "status", "demo") // the first look records the screen
	setClock(t, time.Date(2026, 1, 1, 0, 12, 0, 0, time.UTC))

	raw := mustRun(t, "status", "demo", "--json")
	var got struct {
		Team    string `json:"team"`
		Members []struct {
			Name             string `json:"name"`
			State            string `json:"state"`
			UnchangedMinutes *int   `json:"unchanged_minutes"`
			Live             bool   `json:"live"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("status --json is not JSON: %v\n%s", err, raw)
	}
	if got.Team != "demo" {
		t.Fatalf("status --json team = %q, want demo", got.Team)
	}
	byName := map[string]int{}
	for _, m := range got.Members {
		if m.UnchangedMinutes != nil {
			byName[m.Name] = *m.UnchangedMinutes
		}
	}
	if byName["demo-lead"] != 12 {
		t.Fatalf("unchanged_minutes for demo-lead = %d, want 12 (%s)", byName["demo-lead"], raw)
	}
	if _, ok := byName["demo-builder"]; ok {
		t.Fatalf("an idle member carries unchanged_minutes: %s", raw)
	}
}

// TestStatusMarksAWorkingMemberWhoseScreenHasNotChanged: a second status, twelve
// minutes on with the same screen, says so; a screen that changed starts over.
func TestStatusMarksAWorkingMemberWhoseScreenHasNotChanged(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead working p1 screen-lead\n")

	setClock(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	first := mustRun(t, "status", "demo")
	if !strings.Contains(first, "demo-lead: working\n") {
		t.Fatalf("the first status already marks the member:\n%s", first)
	}

	setClock(t, time.Date(2026, 1, 1, 0, 12, 0, 0, time.UTC))
	second := mustRun(t, "status", "demo")
	if !strings.Contains(second, "demo-lead: working (unchanged for 12 min)\n") {
		t.Fatalf("the second status does not mark the unchanged screen:\n%s", second)
	}

	// A screen that changed is not stale any more.
	f.controlHerdr(t, "demo-lead working p1 a different screen\n")
	setClock(t, time.Date(2026, 1, 1, 0, 20, 0, 0, time.UTC))
	third := mustRun(t, "status", "demo")
	if !strings.Contains(third, "demo-lead: working\n") {
		t.Fatalf("a changed screen is still marked:\n%s", third)
	}
}

// setClock fixes the clock status stamps a screen with, for the length of one
// test.
func setClock(t *testing.T, at time.Time) {
	t.Helper()
	previous := screenClock
	screenClock = func() time.Time { return at }
	t.Cleanup(func() { screenClock = previous })
}
