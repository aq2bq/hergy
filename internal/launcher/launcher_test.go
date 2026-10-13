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

// fakePort is a Herdr that reports the given live agents and records every pane
// it is asked to close.
type fakePort struct {
	live     []protocol.Agent
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
			})
		}
		return map[string]any{"agents": agents}, nil
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
	seats := make([]config.Member, len(members))
	for i, name := range members {
		seats[i] = config.Member{Name: name, Role: "role", Kind: "claude"}
	}
	cfg := &config.Config{Team: "team", Version: 1, Members: seats}
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

	if err := l.Down(members, "own", false); err != nil {
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

	if err := l.Down(members, "", false); err != nil {
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

	if err := l.Down(members, "p1", true); err != nil {
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

	if err := l.Down(members, "", false); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(port.closed) != 0 {
		t.Fatalf("closed %v, want nothing", port.closed)
	}
	if !strings.Contains(out.String(), "nothing to close") {
		t.Fatalf("output does not say nothing was closed:\n%s", out.String())
	}
}
