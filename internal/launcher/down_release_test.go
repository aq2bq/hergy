package launcher

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/protocol"
)

// Names survive a close for a fixed number of readings, and remain in list
// until get reports their disappearance. No herdr or harness is started.
func releaseLauncher(t *testing.T, survive int) (*Launcher, *fakePort, *time.Duration) {
	t.Helper()
	base := &fakePort{agents: map[string]protocol.Agent{}, gone: map[string]bool{}, tabs: []protocol.Tab{{ID: "t1", Label: fixtureSeen("demo")}}}
	starts := []definition.Start{}
	for i, name := range []string{"demo-a", "demo-b"} {
		pane := []string{"p1", "p2"}[i]
		a := protocol.Agent{Name: fixtureSeen(name), PaneID: fixturePane(pane), TabID: "t1", Status: "idle", Kind: "claude"}
		base.agents[name] = a
		base.live = append(base.live, a)
		base.panes = append(base.panes, protocol.Pane{ID: fixturePane(pane), TabID: "t1", Label: fixtureSeen(name)})
		starts = append(starts, definition.Start{Name: fixtureHerdrName(name), Short: fixtureShortName(strings.TrimPrefix(name, "demo-")), Kind: "claude", Cwd: t.TempDir(), Prompt: "hello"})
	}
	reads := map[string]int{}
	base.onAgentGet = func(name string) {
		agent, ok := base.agents[name]
		if !ok || !slices.Contains(base.closed, agent.PaneID.String()) {
			return
		}
		reads[name]++
		if reads[name] <= survive {
			return
		}
		delete(base.agents, name)
		base.live = slices.DeleteFunc(base.live, func(a protocol.Agent) bool { return a.Name.String() == name })
	}
	l := New(starts, fixtureSeen("demo"), t.TempDir(), herdr.New(&receiptPort{fakePort: base}), 400, io.Discard)
	elapsed := new(time.Duration)
	l.now = func() time.Time { return time.Unix(0, 0).Add(*elapsed) }
	l.pause = func(d time.Duration) { *elapsed += d }
	return l, base, elapsed
}

func downPath(l *Launcher, path, self string, dry bool) (int, error) {
	switch path {
	case "Down":
		return l.Down(l.starts, fixturePane(self), dry)
	case "DownByName":
		return l.DownByName(fixtureSeenList([]string{"demo-a", "demo-b"}), fixturePane(self), dry)
	default:
		return l.DownStopped(fixtureSeen("demo"), fixturePane(self), dry)
	}
}

func TestTeamDownWaitsBeforeBoot(t *testing.T) {
	for _, path := range []string{"Down", "DownByName", "DownStopped"} {
		t.Run(path, func(t *testing.T) {
			l, p, elapsed := releaseLauncher(t, 1)
			n, err := downPath(l, path, "", false)
			if err != nil || n != 2 {
				t.Fatalf("down = %d, %v", n, err)
			}
			if *elapsed != 200*time.Millisecond {
				t.Fatalf("wait = %s", *elapsed)
			}
			firstGet := slices.Index(p.calls, "agent get demo-a")
			if firstGet < slices.Index(p.calls, "pane close p2") {
				t.Fatalf("wait before all closes: %v", p.calls)
			}
			if err := l.Boot(""); err != nil {
				t.Fatalf("up after down: %v", err)
			}
		})
	}
}

func TestTeamDownTimeout(t *testing.T) {
	inLanguage(t, "ja")
	for _, path := range []string{"Down", "DownByName", "DownStopped"} {
		for _, self := range []string{"", "p2"} {
			t.Run(path+"/"+self, func(t *testing.T) {
				l, p, elapsed := releaseLauncher(t, 100)
				_, err := downPath(l, path, self, false)
				if err == nil {
					t.Fatal("down succeeded before name release")
				}
				for _, want := range []string{"400", "demo-a", "hergy status"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q lacks %q", err, want)
					}
				}
				if *elapsed != 400*time.Millisecond {
					t.Errorf("one shared deadline: %s", *elapsed)
				}
				if self != "" {
					if slices.Contains(p.closed, self) {
						t.Fatal("own pane closed on timeout")
					}
					for _, want := range []string{"自分のペインは残しました", "hergy down demo"} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("%q lacks %q", err, want)
						}
					}
				} else if !strings.Contains(err.Error(), "demo-b") {
					t.Errorf("missing remaining name: %v", err)
				}
			})
		}
	}
}

func TestTeamDownDryDoesNotWait(t *testing.T) {
	for _, path := range []string{"Down", "DownByName", "DownStopped"} {
		t.Run(path, func(t *testing.T) {
			l, p, elapsed := releaseLauncher(t, 100)
			if _, err := downPath(l, path, "p2", true); err != nil {
				t.Fatal(err)
			}
			if len(p.closed) != 0 || *elapsed != 0 || hasCall(p, "agent get ") {
				t.Fatalf("dry closed or waited: %v", p.calls)
			}
		})
	}
}

func TestTeamDownWaitsBeforeClosingOwnPane(t *testing.T) {
	l, p, _ := releaseLauncher(t, 1)
	if _, err := l.Down(l.starts, "p2", false); err != nil {
		t.Fatal(err)
	}
	lastGet := -1
	for i, call := range p.calls {
		if call == "agent get demo-a" {
			lastGet = i
		}
	}
	if lastGet < 0 || lastGet > slices.Index(p.calls, "pane close p2") || hasCall(p, "agent get demo-b") {
		t.Fatalf("own pane ordering: %v", p.calls)
	}
}

func TestTeamDownWaitsForWorkersAndNamedPanes(t *testing.T) {
	for _, path := range []string{"Down", "DownByName", "DownStopped"} {
		t.Run(path, func(t *testing.T) {
			l, p, elapsed := releaseLauncher(t, 1)
			p.tabs = append(p.tabs, protocol.Tab{ID: "tw", Label: fixtureSeen("worker:demo/research")})
			worker := protocol.Agent{Name: fixtureSeen("demo-research"), PaneID: "pw", TabID: "tw", Status: "idle", Kind: "claude"}
			p.agents[worker.Name.String()] = worker
			p.live = append(p.live, worker)
			p.panes = append(p.panes, protocol.Pane{ID: "pw", TabID: "tw", Label: worker.Name}, protocol.Pane{ID: "orphan", TabID: "t1", Label: fixtureSeen("demo-ended")})
			if _, err := downPath(l, path, "", false); err != nil {
				t.Fatal(err)
			}
			if *elapsed != 200*time.Millisecond || !hasCall(p, "agent get demo-research") || !hasCall(p, "agent get demo-ended") || len(p.closed) != 4 {
				t.Fatalf("closed=%v, waited=%s, calls=%v", p.closed, *elapsed, p.calls)
			}
		})
	}
}

func TestTeamDownReportsOnlyNamesStillPresent(t *testing.T) {
	inLanguage(t, "ja")
	l, p, elapsed := releaseLauncher(t, 100)
	original := p.onAgentGet
	p.onAgentGet = func(name string) {
		if name == "demo-a" && *elapsed >= 200*time.Millisecond {
			delete(p.agents, name)
			p.live = slices.DeleteFunc(p.live, func(a protocol.Agent) bool { return a.Name.String() == name })
		}
		original(name)
	}
	_, err := l.Down(l.starts, "", false)
	if err == nil || strings.Contains(err.Error(), "demo-a") || !strings.Contains(err.Error(), "demo-b") {
		t.Fatalf("remaining names: %v", err)
	}
	if *elapsed != 400*time.Millisecond {
		t.Fatalf("deadline: %s", *elapsed)
	}
}
