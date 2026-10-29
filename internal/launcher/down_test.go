package launcher

import (
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/protocol"
)

// TestDownClosesThePanesNoAgentStandsIn: a boot that stopped at a startup question
// leaves the panes hergy made with no agent in them. down closes them by the names
// hergy gave, and leaves a pane hergy did not name open: that one is a person's
// own (d-da92).
func TestDownClosesThePanesNoAgentStandsIn(t *testing.T) {
	port := &fakePort{
		tabs: []protocol.Tab{{ID: "t1", Label: "team"}},
		panes: []protocol.Pane{
			{ID: "p1", Label: "team-lead", TabID: "t1"},
			{ID: "p2", Label: "team-builder", TabID: "t1"},
			{ID: "p3", TabID: "t1"},
			{ID: "p4", Label: "other-lead", TabID: "t9"},
		},
	}
	l, out := downLauncher(t, port, "team-lead", "team-builder")

	if _, err := l.Down(l.starts, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if want := []string{"p1", "p2"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	for _, line := range []string{
		"team-lead: closing (pane p1, no agent running)",
		"team-builder: closing (pane p2, no agent running)",
		"closed: 2\n", "absent: 0\n",
	} {
		if !strings.Contains(out.String(), line) {
			t.Fatalf("down does not say %q:\n%s", line, out.String())
		}
	}
	if strings.Contains(out.String(), "other-lead") {
		t.Fatalf("down reports another team's pane:\n%s", out.String())
	}
}

// TestDownStoppedClosesWhatHergyMadeForAStoppedTeam: a team no agent runs in is
// still hergy's: its tab carries the team's name, and the panes hergy named after
// it are closed, workers and all. A pane hergy did not name is left alone, and so
// is a tab another team owns (d-da92).
func TestDownStoppedClosesWhatHergyMadeForAStoppedTeam(t *testing.T) {
	port := &fakePort{
		tabs: []protocol.Tab{
			{ID: "t1", Label: "team"},
			{ID: "t2", Label: "worker:team/w1"},
			{ID: "t3", Label: "team-x"},
		},
		panes: []protocol.Pane{
			{ID: "p1", Label: "team-lead", TabID: "t1"},
			{ID: "p2", TabID: "t1"},
			{ID: "p5", Label: "team-w1", TabID: "t2"},
			{ID: "p6", TabID: "t2"},
			{ID: "p7", Label: "team-x-lead", TabID: "t3"},
		},
	}
	l, out := downLauncher(t, port, "team-lead")

	closed, err := l.DownStopped("team", "", false)
	if err != nil {
		t.Fatalf("DownStopped: %v", err)
	}
	if want := []string{"p1", "p5"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if closed != 2 {
		t.Fatalf("DownStopped returned %d, want 2", closed)
	}
	// No agent runs in the worker's pane either, and that is what down says: the
	// pane is closed, and the member marker is not the point.
	if !strings.Contains(out.String(), "team-w1: closing (pane p5, no agent running)") {
		t.Fatalf("down does not say no agent runs in the worker's pane:\n%s", out.String())
	}
	if strings.Contains(out.String(), "team-x-lead") {
		t.Fatalf("down closed another team's pane:\n%s", out.String())
	}
}

// TestDownStoppedWithNoTabOfThatTeamClosesNothing: a name no tab carries is not a
// stopped team, and nothing is closed.
func TestDownStoppedWithNoTabOfThatTeamClosesNothing(t *testing.T) {
	port := &fakePort{
		tabs:  []protocol.Tab{{ID: "t9", Label: "other"}},
		panes: []protocol.Pane{{ID: "p1", Label: "team-lead", TabID: "t9"}},
	}
	l, out := downLauncher(t, port, "team-lead")

	if _, err := l.DownStopped("team", "", false); err != nil {
		t.Fatalf("DownStopped: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("closed %v, want nothing", port.closed)
	}
	if !strings.Contains(out.String(), "nothing to close") {
		t.Fatalf("down does not say nothing was closed:\n%s", out.String())
	}
	if slices.Contains(commandTrail(port.calls), "pane close") {
		t.Fatalf("down closed a pane in a tab that is not the team's:\n%v", port.calls)
	}
}

// TestDownWorkerClosesOnlyThatWorkerAndItsTab: hergy down <worker> closes the
// pane the worker stands in, and the worker's own tab when that was the last pane
// in it. The member and the other worker are left as they are (d-fa05).
func TestDownWorkerClosesOnlyThatWorkerAndItsTab(t *testing.T) {
	port := &fakePort{
		tabs: []protocol.Tab{
			{ID: "t1", Label: "team"},
			{ID: "t2", Label: "worker:team/w1"},
			{ID: "t3", Label: "worker:team/w2"},
		},
		panes: []protocol.Pane{
			{ID: "p1", Label: "team-lead", TabID: "t1"},
			{ID: "p2", Label: "team-w1", TabID: "t2"},
			{ID: "p3", Label: "team-w2", TabID: "t3"},
		},
		live: []protocol.Agent{
			{Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1", TabID: "t1"},
			{Name: "team-w1", Kind: "claude", Status: "idle", PaneID: "p2", TabID: "t2"},
			{Name: "team-w2", Kind: "claude", Status: "idle", PaneID: "p3", TabID: "t3"},
		},
	}
	l, out := downLauncher(t, port, "team-lead")

	closed, err := l.DownWorker("worker:team/w1", "", false)
	if err != nil {
		t.Fatalf("DownWorker: %v", err)
	}
	if want := []string{"p2"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if closed != 1 {
		t.Fatalf("DownWorker returned %d, want 1", closed)
	}
	if want := []string{"t2"}; !slices.Equal(port.closedTabs, want) {
		t.Fatalf("closed tabs %v, want %v", port.closedTabs, want)
	}
	for _, call := range port.calls {
		if call == "tab close t1" || call == "tab close t3" || call == "pane close p1" || call == "pane close p3" {
			t.Fatalf("down <worker> touched another pane or team:\n%s", strings.Join(port.calls, "\n"))
		}
	}
	for _, line := range []string{
		"team-w1: closing (pane p2, worker)",
		"team-w1: closing the tab too (no other pane there)",
		"closed: 1\n", "absent: 0\n",
	} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("down <worker> does not say %q:\n%s", line, out.String())
		}
	}
}

// TestDownWorkerDryPlansThePaneAndItsTab: --dry names the worker's pane and the
// tab that would go with it, and closes nothing.
func TestDownWorkerDryPlansThePaneAndItsTab(t *testing.T) {
	port := &fakePort{
		tabs:  []protocol.Tab{{ID: "t2", Label: "worker:team/w1"}},
		panes: []protocol.Pane{{ID: "p2", Label: "team-w1", TabID: "t2"}},
		live:  []protocol.Agent{{Name: "team-w1", Kind: "claude", Status: "idle", PaneID: "p2", TabID: "t2"}},
	}
	l, out := downLauncher(t, port, "team-lead")

	closed, err := l.DownWorker("worker:team/w1", "", true)
	if err != nil {
		t.Fatalf("DownWorker --dry: %v", err)
	}
	if closed != 1 {
		t.Fatalf("DownWorker --dry returned %d, want 1", closed)
	}
	if len(port.closed) != 0 || len(port.closedTabs) != 0 {
		t.Fatalf("a dry run closed something:\n%s", strings.Join(port.calls, "\n"))
	}
	for _, line := range []string{
		"team-w1: would close (pane p2, worker)",
		"team-w1: would close the tab too (no other pane there)",
		"would close: 1\n", "would skip: 0\n",
	} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("down <worker> --dry does not say %q:\n%s", line, out.String())
		}
	}
}

// TestDownWorkerKeepsATabThatStillHoldsAPane: a person's own pane in the worker's
// tab is not hergy's to close, and the tab stays with it.
func TestDownWorkerKeepsATabThatStillHoldsAPane(t *testing.T) {
	port := &fakePort{
		tabs: []protocol.Tab{{ID: "t2", Label: "worker:team/w1"}},
		panes: []protocol.Pane{
			{ID: "p2", Label: "team-w1", TabID: "t2"},
			{ID: "p4", TabID: "t2"},
		},
		live: []protocol.Agent{{Name: "team-w1", Kind: "claude", Status: "idle", PaneID: "p2", TabID: "t2"}},
	}
	l, _ := downLauncher(t, port, "team-lead")

	if _, err := l.DownWorker("worker:team/w1", "", false); err != nil {
		t.Fatalf("DownWorker: %v", err)
	}
	if want := []string{"p2"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want only the worker's pane %v", port.closed, want)
	}
	if len(port.closedTabs) != 0 {
		t.Fatalf("the worker's tab was closed while a pane still stands in it: %v", port.closedTabs)
	}
}
