package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
)

// stopReleasePort orders the close acknowledgement before registry release.
// The same registry serves list, get and the next screen start's preflight.
type stopReleasePort struct {
	snapshot herdr.SessionSnapshot
	closed   []string
	reads    map[string]int
	survive  int
	seq      int
}

func (p *stopReleasePort) Verify(string) error                             { return nil }
func (p *stopReleasePort) Text(...string) (string, error)                  { return "", nil }
func (p *stopReleasePort) SessionSnapshot() (herdr.SessionSnapshot, error) { return p.snapshot, nil }
func (p *stopReleasePort) JSON(args ...string) (map[string]any, error) {
	switch strings.Join(args[:2], " ") {
	case "agent list":
		agents := []any{}
		for _, a := range p.snapshot.Agents {
			agents = append(agents, stopAgent(a))
		}
		return map[string]any{"agents": agents}, nil
	case "tab list":
		tabs := []any{}
		for _, tab := range p.snapshot.Tabs {
			tabs = append(tabs, map[string]any{"tab_id": tab.ID, "label": tab.Label.String()})
		}
		return map[string]any{"tabs": tabs}, nil
	case "pane list":
		panes := []any{}
		for _, pane := range p.snapshot.Panes {
			panes = append(panes, map[string]any{"pane_id": pane.ID.String(), "label": pane.Label.String(), "tab_id": pane.TabID})
		}
		return map[string]any{"panes": panes}, nil
	case "pane close":
		p.closed = append(p.closed, args[2])
		return map[string]any{}, nil
	case "agent get":
		for _, a := range p.snapshot.Agents {
			if a.Name.String() != args[2] {
				continue
			}
			if slices.Contains(p.closed, a.PaneID.String()) {
				p.reads[a.Name.String()]++
				if p.reads[a.Name.String()] > p.survive {
					p.snapshot.Agents = slices.DeleteFunc(p.snapshot.Agents, func(b protocol.Agent) bool { return b.Name.String() == a.Name.String() })
					break
				}
			}
			return map[string]any{"agent": stopAgent(a)}, nil
		}
		_, err := protocol.Result(args, `{"error":{"code":"agent_not_found"}}`, "", 1)
		return nil, err
	case "workspace get":
		return map[string]any{"workspace": map[string]any{"workspace_id": "w1", "label": "demo"}}, nil
	case "tab create":
		return map[string]any{"tab": map[string]any{"tab_id": "new"}, "root_pane": map[string]any{"pane_id": "new-root"}}, nil
	case "pane split":
		p.seq++
		return map[string]any{"pane": map[string]any{"pane_id": fmt.Sprintf("new-%d", p.seq)}}, nil
	case "agent start":
		for _, a := range p.snapshot.Agents {
			if a.Name.String() == args[2] {
				return nil, fmt.Errorf("agent_name_taken: %s", args[2])
			}
		}
		p.snapshot.Agents = append(p.snapshot.Agents, protocol.Agent{Name: fixtureSeen(args[2]), Kind: "claude", Status: "idle", PaneID: "new-root", TabID: "new"})
		return map[string]any{}, nil
	}
	return map[string]any{}, nil
}
func stopAgent(a protocol.Agent) map[string]any {
	return map[string]any{"name": a.Name.String(), "agent": a.Kind, "agent_status": a.Status, "pane_id": a.PaneID.String(), "tab_id": a.TabID}
}

func TestScreenStopWaitsForNameRelease(t *testing.T) {
	for _, broken := range []bool{false, true} {
		for _, outcome := range []string{"released", "timeout", "timeout-own"} {
			t.Run(fmt.Sprintf("broken=%t/%s", broken, outcome), func(t *testing.T) {
				f := newFixture(t)
				receiptTeam(t, f)
				useJapanese(t)
				env := serveEnv(t)
				r, err := readDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo")
				if err != nil {
					t.Fatal(err)
				}
				p := &stopReleasePort{reads: map[string]int{}, survive: 1}
				p.snapshot.Workspaces = []herdr.Workspace{{ID: "w1", Label: "demo"}}
				p.snapshot.Tabs = []protocol.Tab{{ID: "t1", Label: fixtureSeen("demo"), WorkspaceID: "w1"}}
				for i, start := range r.starts {
					pane := fmt.Sprintf("old-%d", i)
					p.snapshot.Panes = append(p.snapshot.Panes, protocol.Pane{ID: fixturePane(pane), Label: fixtureSeen(start.Name.String()), TabID: "t1"})
					p.snapshot.Agents = append(p.snapshot.Agents, protocol.Agent{Name: fixtureSeen(start.Name.String()), Kind: "claude", Status: "idle", PaneID: fixturePane(pane), TabID: "t1"})
				}
				if outcome != "released" {
					p.survive = 1000
				}
				if outcome == "timeout-own" {
					t.Setenv("HERDR_PANE_ID", "old-2")
				}
				a := newActivityRead(env)
				a.port = p
				h := a.hq
				elapsed := time.Duration(0)
				h.stopNow = func() time.Time { return time.Unix(0, 0).Add(elapsed) }
				h.stopPause = func(d time.Duration) { elapsed += d }
				teamPath := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
				original := readFile(t, teamPath)
				if broken {
					writeFile(t, teamPath, "invalid: [")
				}
				_, err = h.Stop("demo")
				if outcome != "released" {
					if err == nil {
						t.Fatal("stop succeeded while names remain")
					}
					for _, want := range []string{"30000", "demo-a", "hergy status"} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("%q lacks %q", err, want)
						}
					}
					if elapsed != 30*time.Second {
						t.Fatalf("shared deadline: %s", elapsed)
					}
					if outcome == "timeout-own" && (slices.Contains(p.closed, "old-2") || !strings.Contains(err.Error(), "自分のペインは残しました")) {
						t.Fatalf("own pane: %v, %v", p.closed, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if elapsed != 200*time.Millisecond {
					t.Fatalf("wait: %s", elapsed)
				}
				if broken {
					writeFile(t, teamPath, original)
				}
				op := h.newOp(web.HQLaunch{Team: "demo", Phase: "starting"})
				h.runStart(context.Background(), op, env, r, web.HQLaunchIn{Definition: "demo"}, web.HQWorkspace{ID: "w1"})
				if got := op.snapshot(); got.Phase != "done" {
					t.Fatalf("screen start after stop: %+v", got)
				}
			})
		}
	}
}
