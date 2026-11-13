package cli

import (
	"strings"
	"testing"
)

// TestWatcherOfATeamNotYetRecordedFindsItsDefinition: hergy up runs the watcher
// before the launch is recorded, so the watcher of a team started under a name
// this machine never recorded (--as) cannot look its team definition up. up
// tells it which definition the team runs from, and the watcher reads that one
// instead of taking the team's name for a definition's (r-ee91).
func TestWatcherOfATeamNotYetRecordedFindsItsDefinition(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	// The state the watcher starts in: no launch record names alpha.
	stdout, stderr, code := run("turns", "alpha", "--definition", "demo", "--log")
	if code != 0 {
		t.Fatalf("turns alpha --definition demo --log: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "alpha") {
		t.Errorf("the watcher did not answer for the team alpha:\n%s", stdout)
	}

	mustRun(t, "up", "demo", "--as", "alpha")
	want := " turns alpha --definition demo -C " + f.proj + " --lang "
	ran := false
	for line := range strings.SplitSeq(readFile(t, log), "\n") {
		if strings.HasPrefix(line, "pane run ") && strings.Contains(line, want) {
			ran = true
		}
	}
	if !ran {
		t.Errorf("up --as alpha did not run the watcher with %q:\n%s", want, readFile(t, log))
	}
}
