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
