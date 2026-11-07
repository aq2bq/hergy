package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/turns"
)

// TestUpStartsTheWatcherUnderTheMembers: a boot cuts a strip off the bottom of
// the team's tab, names it <team>-turns and runs hergy turns <team> there, before
// the members are laid out above it (r-bc26, d-90e0, d-5b60).
func TestUpStartsTheWatcherUnderTheMembers(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo")

	record := readFile(t, log)
	lines := strings.Split(record, "\n")
	split, rename, run := -1, -1, -1
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "pane split p1 --direction down --ratio 0.85"):
			split = i
		case strings.HasSuffix(line, " demo-turns") && strings.HasPrefix(line, "pane rename"):
			rename = i
		case strings.HasPrefix(line, "pane run ") && strings.Contains(line, " turns demo -C "+f.proj+" --lang "):
			run = i
		}
	}
	if split < 0 || rename < 0 || run < 0 {
		t.Fatalf("up did not cut, name and run the watcher's strip (split %d, rename %d, run %d):\n%s", split, rename, run, record)
	}
	if split >= rename || rename >= run {
		t.Errorf("the strip is cut, then named, then run; got split %d, rename %d, run %d", split, rename, run)
	}
	for i, line := range lines {
		if strings.HasPrefix(line, "agent start ") && i < run {
			t.Errorf("a member was started before the watcher's strip was cut:\n%s", record)
			break
		}
	}
	// Each Claude member is started with hergy's Stop hook, so its harness
	// calls hergy turn-done when a turn ends (r-378e); the profile's own
	// arguments come first.
	var started string
	for _, line := range lines {
		if strings.HasPrefix(line, "agent start demo-lead ") {
			started = line
		}
	}
	for _, want := range []string{"--kind claude", "-- ", "--settings {\"hooks\":{\"Stop\":[", "'turn-done' 'demo' 'lead'"} {
		if !strings.Contains(started, want) {
			t.Errorf("demo-lead's start lacks %q:\n%s", want, started)
		}
	}
	if strings.Contains(record, "signal") {
		t.Errorf("up noted a member without a signal, but every starter member has one:\n%s", record)
	}
}

// TestStatusSaysWhoHoldsTheTurn: status reads the turn's state the watcher keeps
// and says who holds it and from whom; with no watcher pane live it says how to
// start one.
func TestStatusSaysWhoHoldsTheTurn(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, liveHerdr)
	path := turns.Path(filepath.Join(f.state, "hergy"), "demo")
	if err := turns.Save(path, &turns.State{Team: "demo", Holder: "lead", From: "builder", Since: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got := mustRun(t, "status", "demo")
	for _, line := range []string{"turn: lead (handed by builder)", "no watcher is running; hergy turns demo starts it"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status is missing %q:\n%s", line, got)
		}
	}
	if err := turns.Save(path, &turns.State{Team: "demo", Holder: turns.Human, From: "builder", Why: turns.WhySilent, Since: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got = mustRun(t, "status", "demo")
	if !strings.Contains(got, "turn: the human (from builder): it stopped without writing a line\n") {
		t.Fatalf("status does not say why the human holds the turn:\n%s", got)
	}
	// A team the watcher never saw has no turn line, only the way to start one.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	got = mustRun(t, "status", "demo")
	if strings.Contains(got, "turn:") {
		t.Fatalf("status names a turn nobody has seen:\n%s", got)
	}
}
