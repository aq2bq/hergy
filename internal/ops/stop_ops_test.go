package ops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/internal/turns" // stopBoundaryPort controls execution failures independently of the initial
	"github.com/aq2bq/hergy/internal/source"
	// snapshot, so cleanup is measured after exactly the failing operation.
)

type stopBoundaryPort struct {
	*stopReleasePort
	failure     string
	beforeClose func()
}

func (p *stopBoundaryPort) JSON(args ...string) (map[string]any, error) {
	command := strings.Join(args[:2], " ")
	if command == "pane close" && p.beforeClose != nil {
		p.beforeClose()
	}
	if command == "pane close" && p.failure == "close" {
		return nil, fmt.Errorf("close refused")
	}
	if command == "agent get" && p.failure == "read" {
		return nil, fmt.Errorf("name read refused")
	}
	return p.stopReleasePort.JSON(args...)
}
func stopFixture(t *testing.T) (*fixture, Environment, *Resolved, *stopBoundaryPort, *hqRead) {
	t.Helper()
	f := newFixture(t)
	receiptTeam(t, f)
	seedRunningTeam(t, f.proj, "demo", "demo")
	useJapanese(t)
	env := serveEnv(t)
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	port := &stopBoundaryPort{stopReleasePort: &stopReleasePort{reads: map[string]int{}}}
	port.snapshot.Workspaces = []herdr.Workspace{{ID: "w1", Label: "demo"}}
	port.snapshot.Tabs = []protocol.Tab{{ID: "t1", Label: fixtureSeen("demo"), WorkspaceID: "w1"}}
	for i, start := range r.Starts {
		pane := fixturePane(fmt.Sprintf("old-%d", i))
		port.snapshot.Panes = append(port.snapshot.Panes, protocol.Pane{ID: pane, Label: start.Name.Observation(), TabID: "t1", Cwd: env.ProjectRoot.Seen()})
		port.snapshot.Agents = append(port.snapshot.Agents, protocol.Agent{Name: start.Name.Observation(), Kind: "claude", Status: "idle", PaneID: pane, TabID: "t1"})
	}
	a := newActivityRead(env)
	a.port = port
	elapsed := time.Duration(0)
	a.hq.stopNow = func() time.Time { return time.Unix(0, 0).Add(elapsed) }

	a.hq.stopPause = func(time.Duration) { elapsed = 30 * time.Second }
	return f, env, r, port, a.hq
}
func seedRunTurn(t *testing.T, f *fixture) []string {
	t.Helper()
	dir := filepath.Join(f.state, "hergy")
	path := turns.Path(dir, "demo")
	if err := turns.Save(path, &turns.State{Team: fixtureSeen("demo"), Holder: "a"}); err != nil {
		t.Fatal(err)
	}
	signal, tell := turns.SignalDir(dir, "demo"), turns.TellDir(dir, "demo")
	writeFile(t, filepath.Join(signal, "previous.json"), "{}")
	writeFile(t, filepath.Join(tell, "previous.json"), "{}")
	writeFile(t, turns.LogPath(dir, "demo"), "previous history\n")
	return []string{path, signal, tell}
}

// ac-e091 / n-7f40: a successful screen stop, followed by the screen's start of
// the same name, cannot restore the previous run's state, signals or tells.
func TestScreenStopSameNameStartDoesNotInheritTurns(t *testing.T) {
	f, env, r, _, h := stopFixture(t)
	paths := seedRunTurn(t, f)
	cancelled := 0
	pending := h.newOp(HQLaunch{Team: "demo", Phase: "awaiting", Waiting: &HQWaiting{}})
	pending.cancel = func() { cancelled++ }
	if _, err := h.Stop("demo"); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("previous run remains at %s: %v", path, err)
		}
	}
	if cancelled != 1 || pending.snapshot().Phase != "cancelled" || pending.snapshot().Waiting != nil {
		t.Fatalf("pending launch was not cancelled: %d, %+v", cancelled, pending.snapshot())
	}
	next := h.newOp(HQLaunch{Team: "demo", Phase: "starting"})
	h.runStart(context.Background(), next, env, r, HQLaunchIn{Definition: "demo"}, HQWorkspace{ID: "w1"})
	if got := next.snapshot(); got.Phase != "done" {
		t.Fatalf("same-name start: %+v", got)
	}
	turn, err := turns.Load(paths[0], fixtureSeen("demo"))
	if err != nil || turn.Holder != turns.Human {
		t.Fatalf("new run inherited turn: %+v, %v", turn, err)
	}
	for _, path := range paths[1:] {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("new run inherited pending data at %s: %v", path, err)
		}
	}
	if got := readFile(t, turns.LogPath(filepath.Join(f.state, "hergy"), "demo")); got != "previous history\n" {
		t.Fatalf("turn history changed: %q", got)
	}
}

// The CLI's existing TestTeamDownCleanupAfterNameWait covers these outcomes
// through runDown; this table drives the same StopTeam through the screen.
func TestScreenStopUsesTeamCleanupBoundary(t *testing.T) {
	for _, outcome := range []string{"success", "timeout", "close", "read"} {
		t.Run(outcome, func(t *testing.T) {
			f, _, _, port, h := stopFixture(t)
			paths := seedRunTurn(t, f)
			port.failure = outcome
			if outcome == "timeout" {
				port.survive = 1000
			}
			cancelled := 0
			pending := h.newOp(HQLaunch{Team: "demo", Phase: "starting"})
			pending.cancel = func() { cancelled++ }
			other := h.newOp(HQLaunch{Team: "other", Phase: "starting"})
			done := h.newOp(HQLaunch{Team: "demo", Phase: "done"})
			_, err := h.Stop("demo")
			if (err == nil) != (outcome == "success") {
				t.Fatalf("Stop: %v", err)
			}
			cleanup := outcome == "success" || outcome == "timeout"
			for _, path := range paths {
				_, statErr := os.Stat(path)
				if cleanup && !errors.Is(statErr, os.ErrNotExist) {
					t.Errorf("turn remains at %s: %v", path, statErr)
				}
				if !cleanup && statErr != nil {
					t.Errorf("turn removed at %s: %v", path, statErr)
				}
			}
			if (cancelled == 1) != cleanup {
				t.Errorf("cancelled %d, cleanup %t", cancelled, cleanup)
			}
			if other.snapshot().Phase != "starting" || done.snapshot().Phase != "done" {
				t.Fatal("another or completed launch changed")
			}
		})
	}
}
func TestScreenStopCountsActualMemberAndWorkerPanes(t *testing.T) {
	_, _, _, port, h := stopFixture(t)

	port.snapshot.Agents = port.snapshot.Agents[:1]
	port.snapshot.Panes = port.snapshot.Panes[:1]
	port.snapshot.Tabs = append(port.snapshot.Tabs, protocol.Tab{ID: "tw", Label: fixtureSeen("worker:demo/deps"), WorkspaceID: "w1"})
	port.snapshot.Panes = append(port.snapshot.Panes,
		protocol.Pane{ID: "watch", Label: fixtureSeen("demo-turns"), TabID: "t1"},
		protocol.Pane{ID: "worker", Label: fixtureSeen("demo-deps"), TabID: "tw"})
	port.snapshot.Agents = append(port.snapshot.Agents, protocol.Agent{Name: fixtureSeen("demo-deps"), Kind: "claude", Status: "idle", PaneID: "worker", TabID: "tw"})
	result, err := h.Stop("demo")
	if err != nil {
		t.Fatal(err)
	}
	if result.Closed.Members != 1 || result.Closed.Workers != 1 || len(port.closed) != 3 {
		t.Fatalf("closed %+v; actual %v", result.Closed, port.closed)
	}
}
func TestScreenStopAcceptsEmptyTeamTab(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(fmt.Sprintf("recorded=%t", recorded), func(t *testing.T) {
			f, env, r, port, h := stopFixture(t)
			log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
			if recorded {
				if err := writeLaunch(log, env, r, launchlog.CommandUp, launchlog.Workspace{}); err != nil {
					t.Fatal(err)
				}
			}
			paths := seedRunTurn(t, f)
			port.snapshot.Agents, port.snapshot.Panes = nil, nil
			result, err := h.Stop("demo")
			if err != nil {
				t.Fatal(err)
			}
			if result.Closed.Members != 0 || result.Closed.Workers != 0 || len(port.closed) != 0 {
				t.Fatalf("empty tab: %+v, %v", result.Closed, port.closed)
			}
			if len(port.snapshot.Tabs) != 1 {
				t.Fatal("the empty tab itself was closed")
			}
			for _, path := range paths {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("turn remains at %s: %v", path, err)
				}
			}
		})
	}
}
func TestStopOperationKeepsWatcherCountSeparate(t *testing.T) {
	_, env, _, port, _ := stopFixture(t)
	port.snapshot.Panes = append(port.snapshot.Panes, protocol.Pane{ID: "watch", Label: fixtureSeen("demo-turns"), TabID: "t1"})
	tabs := teamTabs(port.snapshot.Tabs, port.snapshot.Agents, port.snapshot.Panes)
	target := teamStopTarget(team.Running{Team: fixtureRunningTeam("demo"), Definition: "demo", Project: env.ProjectRoot}, tabs)
	result, err := StopTeam(context.Background(), StopInput{Env: env, Target: target, Port: port, Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	if result.Closed != (StopClosed{Members: 3, Watchers: 1}) {
		t.Fatalf("closed counts: %+v", result.Closed)
	}
}

// Both entry points receive identical herdr answers. Their actual close order
// and the new run's initial turn must match the shared operation's contract.
func TestCLIAndScreenUseSameStopOperation(t *testing.T) {
	for _, entry := range []string{"cli", "screen"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "wave")
			log := f.stopHerdr(t, true)
			t.Setenv("HERDR_ENV", "1")
			t.Setenv("HERDR_PANE_ID", "outside")
			dir := filepath.Join(f.state, "hergy")
			path := turns.Path(dir, "wave")
			if err := turns.Save(path, &turns.State{Team: fixtureSeen("wave"), Holder: "builder"}); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(turns.SignalDir(dir, "wave"), "old.json"), "{}")
			writeFile(t, filepath.Join(turns.TellDir(dir, "wave"), "old.json"), "{}")
			switch entry {
			case "cli":
				var out, stderr bytes.Buffer
				if code := Run([]string{"down", "wave"}, &out, &stderr); code != 0 {
					t.Fatalf("down exit %d: %s", code, &stderr)
				}
			case "screen":
				result, err := newActivityRead(serveEnv(t)).Stop("wave")
				if err != nil {
					t.Fatal(err)
				}
				if result.Closed.Members != 2 || result.Closed.Workers != 1 {
					t.Fatalf("closed: %+v", result.Closed)
				}
			}
			var closed []string
			for line := range strings.SplitSeq(calls(t, log), "\n") {
				if strings.HasPrefix(line, "pane close ") {
					closed = append(closed, line)
				}
			}
			if !reflect.DeepEqual(closed, []string{"pane close p1", "pane close p2", "pane close p3"}) {
				t.Fatalf("close order: %v", closed)
			}
			state, err := turns.Load(path, fixtureSeen("wave"))
			if err != nil || state.Holder != turns.Human {
				t.Fatalf("next run's turn: %+v, %v", state, err)
			}
			for _, path := range []string{path, turns.SignalDir(dir, "wave"), turns.TellDir(dir, "wave")} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("previous run remains at %s: %v", path, err)
				}
			}
		})
	}
}
func TestStopCancellationKeepsTurnAndEmitsBeforeClosing(t *testing.T) {
	f, env, _, port, _ := stopFixture(t)
	paths := seedRunTurn(t, f)
	tabs := teamTabs(port.snapshot.Tabs, port.snapshot.Agents, port.snapshot.Panes)
	target := teamStopTarget(team.Running{Team: fixtureRunningTeam("demo"), Definition: "demo", Project: env.ProjectRoot}, tabs)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	described, text := false, ""
	port.beforeClose = func() {
		if !described || !strings.Contains(text, "old-0") {
			t.Errorf("close preceded its output: described=%t, text=%q", described, text)
		}
		cancel()
	}
	cancelledLaunch := false
	result, err := StopTeam(ctx, StopInput{Env: env, Target: target, Port: port, Timeout: 0,
		PendingLaunches: func(func(PendingLaunch)) { cancelledLaunch = true },
		Observe: func(event StopEvent) {
			if event.Plan != nil {
				described = true
			}
			text += event.Text
		}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %+v, %v", result, err)
	}
	if cancelledLaunch || len(port.closed) != 1 {
		t.Fatalf("cancel performed cleanup or further closes: %t, %v", cancelledLaunch, port.closed)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("cancel removed previous run at %s: %v", path, err)
		}
	}
}
