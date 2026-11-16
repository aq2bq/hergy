package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpTakesOverATeamAnOlderHergyStarted: a team started before hergy kept a
// state for each running team has its tab and its agents in herdr and no state,
// so status and hire say it is not running. hergy up, under the same name, takes
// it over as it stands: it starts no agent, and the team is running again for
// every command.
func TestUpTakesOverATeamAnOlderHergyStarted(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	state := filepath.Join(f.state, "hergy", "teams", "demo.json")
	if err := os.Remove(state); err != nil {
		t.Fatalf("the started team has no state to take away: %v", err)
	}
	if _, stderr, code := run("status", "demo"); code == 0 {
		t.Fatalf("status finds the team without its state:\n%s", stderr)
	}
	calls := func() [4]int {
		record := readFile(t, log)
		return [4]int{strings.Count(record, "agent start "), strings.Count(record, "agent prompt "), strings.Count(record, "pane close "), strings.Count(record, "tab create ")}
	}
	before := calls()

	stdout, stderr, code := run("up", "demo")
	if code != 0 {
		t.Fatalf("up refuses the team an older hergy started: exit %d\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "took over: demo") {
		t.Errorf("up does not say it took the team over:\n%s%s", stdout, stderr)
	}
	if got := calls(); got != before {
		t.Errorf("taking the team over started, prompted, closed or made something: %v, was %v", got, before)
	}
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("the team has no state after up: %v", err)
	}
	if out := mustRun(t, "status", "demo"); !strings.Contains(out, "demo-lead") {
		t.Errorf("status after the take-over:\n%s", out)
	}

	// A second up is the ordinary refusal: the team is running.
	if _, stderr, code := run("up", "demo"); code == 0 || !strings.Contains(stderr, "already running") {
		t.Errorf("a second up: exit %d\n%s", code, stderr)
	}
}

// TestUpDoesNotTakeOverAHalfTeam: with one member's agent gone, the tab is not
// the team the definition describes, and up refuses as it always has.
func TestUpDoesNotTakeOverAHalfTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	state := filepath.Join(f.state, "hergy", "teams", "demo.json")
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(f.cfg, "agents.txt")
	kept := ""
	for line := range strings.SplitSeq(readFile(t, agents), "\n") {
		if line != "" && !strings.HasSuffix(line, " demo-builder") {
			kept += line + "\n"
		}
	}
	writeFile(t, agents, kept)
	if stdout, stderr, code := run("up", "demo"); code == 0 {
		t.Fatalf("up took over a team with a member missing:\n%s%s", stdout, stderr)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Errorf("the refused up wrote a state: %v", err)
	}
}
