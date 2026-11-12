package launcher

import (
	"bytes"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/protocol"
)

// receiptPort fixes the order of herdr's answers, without a running harness.
type receiptPort struct {
	*fakePort
	answers []string
	sent    [][]string
}

func (p *receiptPort) JSON(args ...string) (map[string]any, error) {
	if len(args) >= 4 && args[0] == "agent" && args[1] == "prompt" {
		p.sent = append(p.sent, slices.Clone(args))
		code := ""
		if len(p.answers) > 0 {
			code, p.answers = p.answers[0], p.answers[1:]
		}
		if code != "" {
			_, err := protocol.Result(args, fmt.Sprintf(`{"error":{"code":%q}}`, code), "", 1)
			return nil, err
		}
		return map[string]any{}, nil
	}
	switch strings.Join(args[:2], " ") {
	case "pane current":
		return map[string]any{"pane": map[string]any{"workspace_id": "w1"}}, nil
	case "workspace list":
		return map[string]any{"workspaces": []any{}}, nil
	case "tab create":
		return map[string]any{"tab": map[string]any{"tab_id": "t1"}, "root_pane": map[string]any{"pane_id": "p1"}}, nil
	}
	return p.fakePort.JSON(args...)
}

func TestBootstrapReceipt(t *testing.T) {
	inLanguage(t, "ja")
	for _, operation := range []string{"up", "hire", "refresh", "reprompt"} {
		for _, tc := range []struct {
			name    string
			answers []string
			calls   int
			failed  bool
		}{
			{"first", []string{""}, 1, false},
			{"retry", []string{"agent_prompt_stalled", ""}, 2, false},
			{"stalled", []string{"agent_prompt_stalled", "agent_prompt_stalled"}, 2, true},
			{"blocked", []string{"agent_blocked", "agent_blocked"}, 2, true},
			{"timeout", []string{"timeout", "timeout"}, 2, true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				base := &fakePort{agents: map[string]protocol.Agent{}, gone: map[string]bool{}, tabs: []protocol.Tab{{ID: "t1", Label: fixtureSeen("team")}}}
				port := &receiptPort{fakePort: base, answers: slices.Clone(tc.answers)}
				members := []definition.Start{{Name: fixtureHerdrName("team-a"), Short: fixtureShortName("a"), Kind: "claude", Cwd: t.TempDir(), Prompt: "the same request"}}
				if operation == "refresh" || operation == "reprompt" {
					base.agents["team-a"] = protocol.Agent{Name: fixtureSeen("team-a"), Kind: "claude", Status: "idle", PaneID: "p1", TabID: "t1"}
				}
				var out bytes.Buffer
				l := New(members, fixtureSeen("team"), t.TempDir(), herdr.New(port), 1000, &out)
				l.kill = func(int) error { base.gone["team-a"] = true; return nil }
				var err error
				switch operation {
				case "up":
					err = l.Boot("hergy up team")
				case "hire":
					err = l.Hire(&members[0], "task")
				case "refresh":
					err = l.Refresh(members, "")
				case "reprompt":
					err = l.Reprompt(members)
				}
				if (err != nil) != tc.failed {
					t.Fatalf("error = %v, output = %s", err, &out)
				}
				if len(port.sent) != tc.calls {
					t.Fatalf("sent %q, want %d calls", port.sent, tc.calls)
				}
				for _, args := range port.sent {
					if args[3] != members[0].Prompt || !slices.Equal(args[4:], []string{"--wait", "--until", "working", "--timeout", "5000"}) {
						t.Fatalf("unchecked or changed prompt: %q", args)
					}
				}
				if tc.failed {
					if strings.Contains(out.String(), "送りました") || strings.Contains(out.String(), "送り直しました") {
						t.Fatalf("false success: %s", &out)
					}
					if !strings.Contains(fmt.Sprint(err), "届いていません: team-a（ペイン p1）") || !strings.Contains(fmt.Sprint(err), "hergy ") {
						t.Fatalf("missing recovery: %v", err)
					}
					if len(base.closed) != 0 {
						t.Fatalf("closed panes: %q", base.closed)
					}
				}
			})
		}
	}
}

// TestBootstrapContinuesOnlyAfterMissingReceipt fixes a failure in the middle:
// the third member is contacted after an absent receipt, never after a send
// failure. Each operation must preserve the same distinction.
func TestBootstrapContinuesOnlyAfterMissingReceipt(t *testing.T) {
	inLanguage(t, "ja")
	for _, operation := range []string{"up", "refresh", "reprompt"} {
		for _, code := range []string{"agent_prompt_stalled", "agent_blocked", "timeout", "send_failed"} {
			t.Run(operation+"/"+code, func(t *testing.T) {
				base := &fakePort{agents: map[string]protocol.Agent{}, gone: map[string]bool{}, process: map[string][2]int{}, tabs: []protocol.Tab{{ID: "t1", Label: fixtureSeen("team")}}}
				port := &receiptPort{fakePort: base, answers: []string{"", code, code, ""}}
				var members []definition.Start
				for i, short := range []string{"a", "b", "c"} {
					name, pane := "team-"+short, fmt.Sprintf("p%d", i+1)
					members = append(members, definition.Start{Name: fixtureHerdrName(name), Short: fixtureShortName(short), Kind: "claude", Cwd: t.TempDir(), Prompt: "request for " + name})
					base.process[pane] = [2]int{100 + i, 900}
					if operation != "up" {
						base.agents[name] = protocol.Agent{Name: fixtureSeen(name), Kind: "claude", Status: "idle", PaneID: fixturePane(pane), TabID: "t1"}
					}
				}
				var out bytes.Buffer
				l := New(members, fixtureSeen("team"), t.TempDir(), herdr.New(port), 1000, &out)
				l.kill = func(pgid int) error {
					for name, agent := range base.agents {
						if base.process[agent.PaneID.String()][0] == pgid {
							base.gone[name] = true
						}
					}
					return nil
				}
				var err error
				switch operation {
				case "up":
					err = l.Boot("hergy up team")
				case "refresh":
					err = l.Refresh(members, "")
				case "reprompt":
					err = l.Reprompt(members)
				}
				if err == nil {
					t.Fatal("succeeded after missing receipt")
				}
				want := 4
				if code == "send_failed" {
					want = 3
				}
				if len(port.sent) != want {
					t.Fatalf("calls=%q, error=%v", port.sent, err)
				}
				if l.Received("team-b") || l.Received("team-c") != (code != "send_failed") {
					t.Fatalf("receipt states: %+v", l.progress.States)
				}
				if code != "send_failed" {
					if !IsUndelivered(err) || !strings.Contains(err.Error(), "team-b") {
						t.Fatalf("missing summary: %v", err)
					}
					for line := range strings.SplitSeq(out.String(), "\n") {
						if strings.Contains(line, "team-b") && (strings.Contains(line, "送りました") || strings.Contains(line, "送り直しました") || strings.Contains(line, "起動し直しました")) {
							t.Fatalf("false success: %s", line)
						}
					}
				} else if IsUndelivered(err) {
					t.Fatalf("send failure became absent receipt: %v", err)
				}
			})
		}
	}
}

// The close reply precedes name release. Down must observe that release before
// returning, so its advertised recovery command can hire the same name again.
func TestWorkerDownWaitsForNameBeforeRehire(t *testing.T) {
	agent := protocol.Agent{Name: fixtureSeen("team-worker"), Kind: "claude", Status: "idle", PaneID: "old", TabID: "worker-tab"}
	base := &fakePort{agents: map[string]protocol.Agent{agent.Name.String(): agent}, gone: map[string]bool{}, live: []protocol.Agent{agent}, tabs: []protocol.Tab{{ID: "worker-tab", Label: fixtureSeen("worker:team/worker")}}, panes: []protocol.Pane{{ID: "old", Label: agent.Name, TabID: "worker-tab"}}}
	port := &receiptPort{fakePort: base}
	l := New(nil, fixtureSeen("team"), t.TempDir(), herdr.New(port), 1000, &bytes.Buffer{})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	pauses := 0
	l.pause = func(d time.Duration) { pauses++; now = now.Add(d) }
	reads := 0
	base.onAgentGet = func(name string) {
		if name != agent.Name.String() {
			return
		}
		reads++
		if reads == 2 {
			delete(base.agents, name)
			base.live = nil
		}
	}
	if _, err := l.DownWorker(fixtureSeen("worker:team/worker"), "", false); err != nil {
		t.Fatal(err)
	}
	if reads != 2 || pauses != 1 || !slices.Equal(base.closed, []string{"old"}) {
		t.Fatalf("reads=%d pauses=%d closed=%v", reads, pauses, base.closed)
	}
	worker := definition.Start{Name: fixtureHerdrName(agent.Name.String()), Short: fixtureShortName("worker"), Kind: "claude", Cwd: t.TempDir(), Prompt: "task", From: fixtureMemberName("lead"), Purpose: "research"}
	if err := l.Hire(&worker, "task"); err != nil {
		t.Fatalf("rehire before release: %v", err)
	}
}

func TestWorkerDownNameReleaseTimeout(t *testing.T) {
	inLanguage(t, "ja")
	agent := protocol.Agent{Name: fixtureSeen("team-worker"), Kind: "claude", Status: "idle", PaneID: "old", TabID: "worker-tab"}
	base := &fakePort{agents: map[string]protocol.Agent{agent.Name.String(): agent}, live: []protocol.Agent{agent}, tabs: []protocol.Tab{{ID: "worker-tab", Label: fixtureSeen("worker:team/worker")}}}
	var out bytes.Buffer
	l := New(nil, fixtureSeen("team"), t.TempDir(), herdr.New(base), 1, &out)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	l.pause = func(d time.Duration) { now = now.Add(d) }
	_, err := l.DownWorker(fixtureSeen("worker:team/worker"), "", false)
	if err == nil || !strings.Contains(err.Error(), "ワーカーの名前が消えません") || strings.Contains(err.Error(), "refresh") {
		t.Fatalf("timeout=%v", err)
	}
	if strings.Contains(out.String(), "閉じました:") {
		t.Fatalf("false completion: %s", &out)
	}
}

func TestHireRetryPreservesTaskAsOneShellArgument(t *testing.T) {
	worker := definition.Start{Name: fixtureHerdrName("team-worker"), Short: fixtureShortName("worker"), From: fixtureMemberName("lead"), Purpose: "read 'quoted' docs"}
	task := "line one\n'quote' `touch forbidden` $(touch forbidden) $VALUE \\end\r\n"
	command := hireRetry("/project", "team", &worker, task)
	if strings.ContainsAny(command, "\r\n") {
		t.Fatalf("recovery is multiple lines: %q", command)
	}
	// Run only printf with the quoted task: no hergy, herdr or harness is started.
	setup, word := recoveryWord(task, "task")
	shells := []string{"/bin/sh", "bash"}
	if dash, err := exec.LookPath("dash"); err == nil {
		shells = append(shells, dash)
	}
	for _, shell := range shells {
		cmd := exec.Command(shell, "-c", setup+"printf '%s' "+word)
		cmd.Dir = t.TempDir()
		got, err := cmd.Output()
		if err != nil || string(got) != task {
			t.Fatalf("%s changed task: %q, %v", shell, got, err)
		}
	}
	if !strings.Contains(command, "hergy down team-worker -C '/project' && hergy hire worker --team team -C '/project' --purpose ") || !strings.HasSuffix(command, " --from lead") {
		t.Fatalf("incomplete recovery: %s", command)
	}
}
