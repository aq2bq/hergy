package launcher

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// TestMain loads the messages, so the printed lines a test reads are the ones a
// human sees.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// fakePort is a Herdr that reports the given live agents and tabs and records
// every pane it is asked to close.
type fakePort struct {
	live     []protocol.Agent
	tabs     []protocol.Tab
	closed   []string
	verified []string
}

func (p *fakePort) Verify(mode string) error {
	p.verified = append(p.verified, mode)
	return nil
}

func (p *fakePort) JSON(args ...string) (map[string]any, error) {
	switch {
	case slices.Equal(args, []string{"agent", "list"}):
		agents := make([]any, 0, len(p.live))
		for _, agent := range p.live {
			agents = append(agents, map[string]any{
				"name": agent.Name, "agent": agent.Kind,
				"agent_status": agent.Status, "pane_id": agent.PaneID,
				"tab_id": agent.TabID,
			})
		}
		return map[string]any{"agents": agents}, nil
	case slices.Equal(args, []string{"tab", "list"}):
		tabs := make([]any, 0, len(p.tabs))
		for _, tab := range p.tabs {
			tabs = append(tabs, map[string]any{"tab_id": tab.ID, "label": tab.Label})
		}
		return map[string]any{"tabs": tabs}, nil
	case len(args) == 3 && args[0] == "pane" && args[1] == "close":
		p.closed = append(p.closed, args[2])
		return map[string]any{}, nil
	}
	return nil, fmt.Errorf("unexpected herdr command: %v", args)
}

// downLauncher builds a team of the named members and a launcher over the fake
// Herdr. The returned buffer collects what Down prints.
func downLauncher(t *testing.T, port *fakePort, members ...string) (*Launcher, *bytes.Buffer) {
	t.Helper()
	return downLauncherFor(t, port, "team", members...)
}

// downLauncherFor is downLauncher with the team named, so a test can set the
// name the worker tabs are labelled with.
func downLauncherFor(t *testing.T, port *fakePort, team string, members ...string) (*Launcher, *bytes.Buffer) {
	t.Helper()
	seats := make([]config.Member, len(members))
	for i, name := range members {
		seats[i] = config.Member{Name: name, Seat: name, Role: "role", Kind: "claude"}
	}
	cfg := &config.Config{Team: team, Version: 1, Members: seats}
	out := &bytes.Buffer{}
	return New(cfg, herdr.New(port), 30000, out), out
}

func TestDownClosesItsOwnPaneLast(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
		{Name: "b", Kind: "claude", Status: "idle", PaneID: "own"},
		{Name: "c", Kind: "claude", Status: "idle", PaneID: "p3"},
	}}
	l, _ := downLauncher(t, port, "a", "b", "c")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "own", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	want := []string{"p1", "p3", "own"}
	if !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if !slices.Equal(port.verified, []string{protocol.ModeDown}) {
		t.Fatalf("Verify called with %v, want [%s]", port.verified, protocol.ModeDown)
	}
}

func TestDownSkipsAMemberThatIsNotLive(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
	}}
	l, out := downLauncher(t, port, "a", "b")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if !slices.Equal(port.closed, []string{"p1"}) {
		t.Fatalf("closed %v, want only [p1]", port.closed)
	}
	if !strings.Contains(out.String(), "b: not present") {
		t.Fatalf("output does not report b as absent:\n%s", out.String())
	}
}

func TestDownDryClosesNothing(t *testing.T) {
	port := &fakePort{live: []protocol.Agent{
		{Name: "a", Kind: "claude", Status: "idle", PaneID: "p1"},
		{Name: "b", Kind: "claude", Status: "idle", PaneID: "p2"},
	}}
	l, out := downLauncher(t, port, "a", "b")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "p1", true); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("dry run closed %v, want nothing", port.closed)
	}
	if len(port.verified) != 0 {
		t.Fatalf("dry run verified %v, want no Herdr check", port.verified)
	}
	if !strings.Contains(out.String(), "would close") {
		t.Fatalf("output shows no plan:\n%s", out.String())
	}
}

func TestDownWithNobodyLiveClosesNothing(t *testing.T) {
	port := &fakePort{}
	l, out := downLauncher(t, port, "a", "b")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("closed %v, want nothing", port.closed)
	}
	if !strings.Contains(out.String(), "nothing to close") {
		t.Fatalf("output does not say nothing was closed:\n%s", out.String())
	}
}

// TestDownClosesTheTeamsHiredWorkers: down closes the team's members and the
// workers hired into tabs labelled "worker:<team>/...", and leaves another
// team's worker alone.
func TestDownClosesTheTeamsHiredWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-worker", Kind: "claude", Status: "idle", PaneID: "p-worker", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-w1", TabID: "t-w1"},
			{Name: "dev-w2", Kind: "claude", Status: "idle", PaneID: "p-w2", TabID: "t-w2"},
			{Name: "dev-x-w1", Kind: "claude", Status: "idle", PaneID: "p-x1", TabID: "t-x1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
			{ID: "t-w2", Label: "worker:dev/w2"},
			{ID: "t-x1", Label: "worker:dev-x/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead", "dev-worker")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	want := []string{"p-lead", "p-worker", "p-w1", "p-w2"}
	if !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v", port.closed, want)
	}
	if strings.Contains(out.String(), "dev-x") {
		t.Fatalf("down dev reports dev-x's worker:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "dev-w1: closing (pane p-w1, worker)\n") {
		t.Fatalf("down does not mark dev-w1 as a worker:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "closed: 4\n") {
		t.Fatalf("down does not count the workers:\n%s", out.String())
	}
}

// TestDownAsClosesOnlyThatTeamsWorkers: with --as, the team name in the label
// decides; only "worker:<as>/..." tabs close, not the settings name's workers.
func TestDownAsClosesOnlyThatTeamsWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-dev", TabID: "t-dev"},
			{Name: "dev2-w1", Kind: "claude", Status: "idle", PaneID: "p-dev2", TabID: "t-dev2"},
		},
		tabs: []protocol.Tab{
			{ID: "t-dev", Label: "worker:dev/w1"},
			{ID: "t-dev2", Label: "worker:dev2/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev2", "dev2-lead", "dev2-worker")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if !slices.Equal(port.closed, []string{"p-dev2"}) {
		t.Fatalf("closed %v, want only dev2's worker", port.closed)
	}
	if !strings.Contains(out.String(), "closed: 1\n") {
		t.Fatalf("down does not count dev2's worker:\n%s", out.String())
	}
}

// TestDownDryPlansTheWorkers: --dry prints the members and the workers and
// closes nothing.
func TestDownDryPlansTheWorkers(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "p-w1", TabID: "t-w1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead", "dev-worker")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "", true); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("dry run closed %v, want nothing", port.closed)
	}
	if len(port.verified) != 0 {
		t.Fatalf("dry run verified %v, want no Herdr check", port.verified)
	}
	for _, want := range []string{
		"dev-lead: would close (pane p-lead)\n",
		"dev-w1: would close (pane p-w1, worker)\n",
		"would close: 2\n",
		"would skip: 1\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("dry plan is missing %q:\n%s", want, out.String())
		}
	}
}

// TestDownClosesAWorkersOwnPaneLast: a worker can be running in hergy's own
// pane; it is still closed, and closed last.
func TestDownClosesAWorkersOwnPaneLast(t *testing.T) {
	port := &fakePort{
		live: []protocol.Agent{
			{Name: "dev-lead", Kind: "claude", Status: "idle", PaneID: "p-lead", TabID: "t-team"},
			{Name: "dev-w1", Kind: "claude", Status: "idle", PaneID: "own", TabID: "t-w1"},
		},
		tabs: []protocol.Tab{
			{ID: "t-team", Label: "team:dev"},
			{ID: "t-w1", Label: "worker:dev/w1"},
		},
	}
	l, out := downLauncherFor(t, port, "dev", "dev-lead")
	members, err := l.cfg.Select("")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := l.Down(members, "own", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if want := []string{"p-lead", "own"}; !slices.Equal(port.closed, want) {
		t.Fatalf("closed %v, want %v (own pane last)", port.closed, want)
	}
	if !strings.Contains(out.String(), "dev-w1: closing (pane own, worker, own pane; last)\n") {
		t.Fatalf("down does not report the worker's own pane last:\n%s", out.String())
	}
}
