package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/turns"
	"github.com/aq2bq/hergy/internal/web"
)

// receiptHerdr augments the existing fake binary with ordered prompt answers.
// Only the middle recipient fails; later recipients must still be contacted.
func receiptHerdr(t *testing.T, f *fixture, target, code string, retry bool) string {
	t.Helper()
	path := os.Getenv("HERGY_HERDR")
	script := readFile(t, path)
	count := filepath.Join(f.cfg, "receipt-count")
	_ = os.Remove(count)
	handler := `  "agent prompt")
    case "$3" in ` + target + `|p-` + target + `)
      n=0; [ -f '` + count + `' ] && n=$(cat '` + count + `')
      n=$((n+1)); printf '%s' "$n" > '` + count + `'
      if [ "$n" -le ` + map[bool]string{true: "1", false: "2"}[retry] + ` ] && [ -n '` + code + `' ]; then
        echo '{"error":{"code":"` + code + `","message":"receipt test"}}'; exit 1
      fi ;;
    esac
    echo '{"result":{}}' ;;
`
	script = strings.Replace(script, "case \"$g $s\" in\n", "case \"$g $s\" in\n"+handler, 1)
	// A refresh of a shell drops that agent from the fake's registry immediately.
	script = strings.Replace(script, `  "pane process-info") echo`, `  "pane process-info")
    name="${4#p-}"; awk -v n="$name" '$2 != n' "$state" > "$state.tmp"; mv "$state.tmp" "$state"
    echo`, 1)
	f.herdr(t, script)
	log := filepath.Join(f.cfg, "herdr.log")
	writeFile(t, log, "")
	return log
}

func receiptTeam(t *testing.T, f *fixture) {
	t.Helper()
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
description: receipt test
members:
  - {name: a, role: lead, profile: claude-default}
  - {name: b, role: lead, profile: claude-default}
  - {name: c, role: lead, profile: claude-default}
workers:
  - {from: a, purpose: research, profile: claude-default}
turns:
  - human -> a
  - a -> b
  - b -> c
  - c -> human
`)
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
}

func TestInitialPromptReceiptOperations(t *testing.T) {
	for _, operation := range []string{"up", "hire", "refresh", "reprompt", "screen-start", "apply-cli", "apply-screen"} {
		for _, tc := range []struct {
			name, code           string
			retry, failed, fatal bool
		}{
			{name: "first"},
			{name: "retry", code: "agent_prompt_stalled", retry: true},
			{name: "stalled", code: "agent_prompt_stalled", failed: true},
			{name: "blocked", code: "agent_blocked", failed: true},
			{name: "timeout", code: "timeout", failed: true},
			{name: "send-failure", code: "send_failed", failed: true, fatal: true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				f := newFixture(t)
				receiptTeam(t, f)
				if operation != "up" && operation != "screen-start" {
					mustRun(t, "up", "demo")
				}
				target := "demo-b"
				if operation == "hire" {
					target = "demo-worker"
				}
				log := receiptHerdr(t, f, target, tc.code, tc.retry)
				_, beforeRecords := launchLog(t, f)
				var stdout, stderr bytes.Buffer
				exit := 0
				var result web.HQLaunch
				switch operation {
				case "up":
					exit = Run([]string{"up", "demo", "--lang", "ja"}, &stdout, &stderr)
				case "hire":
					exit = Run([]string{"hire", "worker", "--team", "demo", "--purpose", "research", "--task", "read docs", "--from", "a", "--lang", "ja"}, &stdout, &stderr)
				case "refresh", "reprompt":
					exit = Run([]string{operation, "--all", "--team", "demo", "--lang", "ja"}, &stdout, &stderr)
				case "screen-start":
					useJapanese(t)
					env := serveEnv(t)
					r, err := readDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo")
					if err != nil {
						t.Fatal(err)
					}
					h := newActivityRead(env).hq
					op := h.newOp(web.HQLaunch{Team: "demo", Phase: "starting"})
					h.runStart(context.Background(), op, env, r, web.HQLaunchIn{Definition: "demo"}, web.HQWorkspace{ID: "w1"})
					result = op.snapshot()
				case "apply-cli", "apply-screen":
					role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
					writeFile(t, role, readFile(t, role)+"\nReceipt test instruction.\n")
					if operation == "apply-cli" {
						exit = Run([]string{"apply", "--team", "demo", "--lang", "ja"}, &stdout, &stderr)
					} else {
						useJapanese(t)
						a := newActivityRead(serveEnv(t))
						plan := a.plan(a.clock())
						selected, refusals := applySelection(plan, []string{"demo-a", "demo-b", "demo-c"})
						if len(refusals) > 0 {
							t.Fatal(refusals)
						}
						op := a.hq.newOp(applyLaunch(selected))
						a.hq.runApply(context.Background(), op, selected)
						result = op.snapshot()
					}
				}
				if result.Phase != "" {
					if result.Phase == "failed" {
						exit = 1
					}
					stderr.WriteString(result.Error)
					for _, step := range result.Steps {
						stdout.WriteString(step.Text + "\n")
					}
				}
				if (exit != 0) != tc.failed {
					t.Fatalf("exit=%d result=%+v stdout=%s stderr=%s", exit, result, &stdout, &stderr)
				}
				if !tc.failed && !strings.Contains(stdout.String(), "送りました") && !strings.Contains(stdout.String(), "送り直しました") {
					t.Fatalf("missing confirmed success: %s", &stdout)
				}
				calls := readFile(t, log)
				prompts := strings.Split(calls, "agent prompt ")[1:]
				want := 3
				if operation == "hire" {
					want = 1
				}
				if tc.code != "" {
					want++
				}
				if tc.fatal && operation != "hire" {
					want--
				}
				if len(prompts) != want {
					t.Fatalf("%d prompts, want %d: %s", len(prompts), want, calls)
				}
				for _, call := range prompts {
					if !strings.Contains(call, "--wait --until working --timeout 5000") {
						t.Fatalf("unchecked send: %s", call)
					}
				}
				if tc.failed && !tc.fatal {
					pane := "p2"
					if operation == "hire" || operation == "reprompt" || operation == "refresh" || strings.HasPrefix(operation, "apply-") {
						pane = "p-" + target
					}
					if operation == "hire" {
						pane = "p1"
					}
					if !strings.Contains(stderr.String(), "届いていません: "+target) || !strings.Contains(stderr.String(), "ペイン "+pane) {
						t.Fatalf("missing recipient/pane: %s", &stderr)
					}
					if !strings.Contains(stderr.String(), "hergy ") {
						t.Fatalf("missing recovery: %s", &stderr)
					}
					for line := range strings.SplitSeq(stdout.String(), "\n") {
						if (strings.Contains(line, target) || strings.HasPrefix(line, "b:")) && (strings.Contains(line, "送りました") || strings.Contains(line, "送り直しました")) {
							t.Fatalf("false success: %s", line)
						}
					}
					if operation == "hire" {
						if !strings.Contains(stderr.String(), "hergy down demo-worker -C '"+f.proj+"' && hergy hire worker --team demo -C '"+f.proj+"' --purpose 'research' --task 'read docs' --from a") {
							t.Fatalf("wrong recovery: %s", &stderr)
						}
						_, records := launchLog(t, f)
						last := records[len(records)-1]
						if !last.IsHire() || last.Hire.Worker != target || last.Hire.From != "a" {
							t.Fatalf("hire not recorded: %+v", last)
						}
						if strings.Contains(calls, "pane close") {
							t.Fatalf("worker closed: %s", calls)
						}
					}
					if operation == "apply-cli" || operation == "apply-screen" {
						_, records := launchLog(t, f)
						last := records[len(records)-1]
						if last.Command != launchlog.CommandApply || len(last.Members) != 2 {
							t.Fatalf("successful members not recorded: %+v", last)
						}
						for _, m := range last.Members {
							if m.Name == target {
								t.Fatalf("undelivered member recorded: %+v", m)
							}
						}
					}
				}
				if tc.fatal {
					_, records := launchLog(t, f)
					if len(records) != len(beforeRecords) {
						t.Fatalf("sending failure recorded a launch: %+v", records)
					}
				}
				if tc.fatal && strings.Contains(stderr.String(), "届いていません") {
					t.Fatalf("send failure reclassified: %s", &stderr)
				}
			})
		}
	}
}

// The watcher uses the same confirmed sending, but still escalates every final
// error to the human, including errors that stop a launch (d-b804).
type watcherReceiptPort struct {
	herdr.Port
	answers []string
	calls   [][]string
}

func (p *watcherReceiptPort) JSON(args ...string) (map[string]any, error) {
	if len(args) < 4 || args[0] != "agent" || args[1] != "prompt" {
		return p.Port.JSON(args...)
	}
	p.calls = append(p.calls, slices.Clone(args))
	code := p.answers[len(p.calls)-1]
	if code != "" {
		_, err := protocol.Result(args, `{"error":{"code":"`+code+`"}}`, "", 1)
		return nil, err
	}
	return map[string]any{}, nil
}

func TestWatcherConfirmedDelivery(t *testing.T) {
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
		{"send-failure", []string{"send_failed", "send_failed"}, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTurnsFixture(t)
			port := &watcherReceiptPort{Port: f.port, answers: tc.answers}
			f.w.herdr = herdr.New(port)
			f.w.deliver(turns.Action{From: "builder", To: "lead", Messages: []turns.Message{{From: "builder", Text: "answer"}}}, "")
			if len(port.calls) != tc.calls {
				t.Fatalf("calls=%q", port.calls)
			}
			for _, args := range port.calls {
				if !slices.Equal(args[4:], []string{"--wait", "--until", "working", "--timeout", "5000"}) {
					t.Fatalf("unchecked delivery: %q", args)
				}
			}
			if tc.calls == 2 && !slices.Equal(port.calls[0], port.calls[1]) {
				t.Fatalf("changed retry: %q", port.calls)
			}
			if tc.failed && (f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhyUndelivered) {
				t.Fatalf("state=%+v", f.w.state)
			}
			if !tc.failed && !strings.Contains(f.out.String(), "渡しました") {
				t.Fatalf("missing delivery: %s", f.out)
			}
		})
	}
}

func TestApplyContinuesAcrossDeliveryKinds(t *testing.T) {
	for _, screen := range []bool{false, true} {
		for _, mode := range []string{"restart", "mixed"} {
			t.Run(fmt.Sprintf("screen=%t/%s", screen, mode), func(t *testing.T) {
				f := newFixture(t)
				receiptTeam(t, f)
				mustRun(t, "up", "demo")
				if mode == "restart" {
					file := filepath.Join(f.proj, ".hergy", "profiles.yaml")
					writeFile(t, file, strings.Replace(readFile(t, file), "kind: claude", "kind: claude\n    args: [--model, test]", 1))
				} else {
					file := filepath.Join(f.cfg, "agents.txt")
					writeFile(t, file, "t1 demo-a\n") // b and c need new panes; a needs a resend.
					file = filepath.Join(f.proj, ".hergy", "roles", "lead.md")
					writeFile(t, file, readFile(t, file)+"\nChanged role.\n")
				}
				log := receiptHerdr(t, f, "demo-b", "agent_prompt_stalled", false)
				if mode == "mixed" {
					fake := os.Getenv("HERGY_HERDR")
					script := strings.Replace(readFile(t, fake), `"panes":[]`, `"panes":[{"pane_id":"p-demo-a","label":"demo-a","tab_id":"t1"}]`, 1)
					f.herdr(t, script)
				}
				var out, stderr bytes.Buffer
				if screen {
					useJapanese(t)
					a := newActivityRead(serveEnv(t))
					selected, refusals := applySelection(a.plan(a.clock()), []string{"demo-a", "demo-b", "demo-c"})
					if len(refusals) > 0 {
						t.Fatal(refusals)
					}
					op := a.hq.newOp(applyLaunch(selected))
					a.hq.runApply(context.Background(), op, selected)
					result := op.snapshot()
					if result.Phase != "failed" || !strings.Contains(result.Error, "demo-b") {
						t.Fatalf("result=%+v", result)
					}
					for _, step := range result.Steps {
						out.WriteString(step.Text + "\n")
					}
				} else {
					if exit := Run([]string{"apply", "--team", "demo", "--lang", "ja"}, &out, &stderr); exit != 1 {
						t.Fatalf("exit=%d: %s %s", exit, &out, &stderr)
					}
				}
				calls := readFile(t, log)
				if got := strings.Count(calls, "agent prompt "); got != 4 {
					t.Fatalf("%d prompts: %s", got, calls)
				}
				_, records := launchLog(t, f)
				last := records[len(records)-1]
				if last.Command != launchlog.CommandApply || len(last.Members) != 2 {
					t.Fatalf("record=%+v", last)
				}
				for _, member := range last.Members {
					if member.Name == "demo-b" {
						t.Fatalf("unreceived prompt recorded: %+v", member)
					}
				}
				for line := range strings.SplitSeq(out.String(), "\n") {
					if (strings.HasPrefix(line, "b:") || strings.Contains(line, "demo-b")) && (strings.Contains(line, "起動しました") || strings.Contains(line, "起動し直しました")) {
						t.Fatalf("false success: %s", line)
					}
				}
			})
		}
	}
}

func TestReceiptRecoveryKeepsProjectFromDirOption(t *testing.T) {
	for _, operation := range []string{"hire", "reprompt"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			receiptTeam(t, f)
			mustRun(t, "up", "demo")
			target := "demo-b"
			if operation == "hire" {
				target = "demo-worker"
			}
			receiptHerdr(t, f, target, "agent_prompt_stalled", false)
			caller := t.TempDir()
			relative, err := filepath.Rel(caller, f.proj)
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(caller)
			args := []string{operation, "--team", "demo", "-C", relative, "--lang", "ja"}
			if operation == "hire" {
				args = append(args, "worker", "--purpose", "research", "--task", "same task", "--from", "a")
			} else {
				args = append(args, "b")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 1 {
				t.Fatalf("code=%d: %s %s", code, &stdout, &stderr)
			}
			want := "hergy reprompt demo-b --team demo -C '" + f.proj + "'"
			if operation == "hire" {
				want = "hergy down demo-worker -C '" + f.proj + "' && hergy hire worker --team demo -C '" + f.proj + "' --purpose 'research' --task 'same task' --from a"
			}
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("wrong project or recipient: %s", &stderr)
			}
		})
	}
}

// d-8e2f separates the team's existence from its confirmed recipients.
func TestReceiptLaunchRecords(t *testing.T) {
	for _, operation := range []string{"up", "screen-start", "refresh", "reprompt", "apply-cli", "apply-screen"} {
		for _, allMissing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/all-missing=%t", operation, allMissing), func(t *testing.T) {
				f := newFixture(t)
				receiptTeam(t, f)
				if operation != "up" && operation != "screen-start" {
					mustRun(t, "up", "demo")
				}
				_, before := launchLog(t, f)
				target := "demo-b"
				if allMissing {
					target = "*"
				}
				receiptHerdr(t, f, target, "agent_prompt_stalled", false)
				if allMissing {
					file := os.Getenv("HERGY_HERDR")
					f.herdr(t, strings.Replace(readFile(t, file), `[ "$n" -le 2 ]`, `true`, 1))
				}
				var out, stderr bytes.Buffer
				switch operation {
				case "up":
					if code := Run([]string{"up", "demo"}, &out, &stderr); code != 1 {
						t.Fatalf("code=%d: %s", code, &stderr)
					}
				case "refresh", "reprompt":
					if code := Run([]string{operation, "--all", "--team", "demo"}, &out, &stderr); code != 1 {
						t.Fatalf("code=%d: %s", code, &stderr)
					}
				case "screen-start":
					env := serveEnv(t)
					r, err := readDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo")
					if err != nil {
						t.Fatal(err)
					}
					h := newActivityRead(env).hq
					op := h.newOp(web.HQLaunch{Team: "demo", Phase: "starting"})
					h.runStart(context.Background(), op, env, r, web.HQLaunchIn{Definition: "demo"}, web.HQWorkspace{ID: "w1", Label: "workspace"})
					if got := op.snapshot(); got.Phase != "failed" {
						t.Fatalf("result=%+v", got)
					}
				case "apply-cli", "apply-screen":
					file := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
					writeFile(t, file, readFile(t, file)+"\nChanged role.\n")
					if operation == "apply-cli" {
						if code := Run([]string{"apply", "--team", "demo"}, &out, &stderr); code != 1 {
							t.Fatalf("code=%d: %s", code, &stderr)
						}
					} else {
						a := newActivityRead(serveEnv(t))
						selected, refused := applySelection(a.plan(a.clock()), []string{"demo-a", "demo-b", "demo-c"})
						if len(refused) > 0 {
							t.Fatal(refused)
						}
						op := a.hq.newOp(applyLaunch(selected))
						a.hq.runApply(context.Background(), op, selected)
						if got := op.snapshot(); got.Phase != "failed" {
							t.Fatalf("result=%+v", got)
						}
					}
				}
				_, records := launchLog(t, f)
				creates := operation == "up" || operation == "screen-start"
				wantRecords := len(before) + 1
				if allMissing && !creates {
					wantRecords = len(before)
				}
				if len(records) != wantRecords {
					t.Fatalf("%d records, want %d", len(records), wantRecords)
				}
				if wantRecords == len(before) {
					return
				}
				last := records[len(records)-1]
				wantMembers := 2
				if allMissing {
					wantMembers = 0
				}
				if last.Team != "demo" || last.Definition != "demo" || last.Project != f.proj || len(last.Members) != wantMembers {
					t.Fatalf("record=%+v", last)
				}
				for _, m := range last.Members {
					if m.Name == "demo-b" {
						t.Fatalf("unreceived member recorded: %+v", last)
					}
				}
				if creates {
					if last.WorkspaceID != "w1" {
						t.Fatalf("missing workspace: %+v", last)
					}
					out.Reset()
					stderr.Reset()
					if code := Run([]string{"status", "demo"}, &out, &stderr); code != 0 {
						t.Fatalf("status code=%d: %s", code, &stderr)
					}
					a := newActivityRead(serveEnv(t))
					list := a.List()
					if len(list.Items) != 1 || list.Items[0].Name != "demo" {
						t.Fatalf("activity=%+v", list)
					}
					running := a.Running()
					if len(running.Teams) != 1 || running.Teams[0].Definition != "demo" {
						t.Fatalf("running=%+v", running)
					}
				}
			})
		}
	}
}

func TestUndeliveredAliasCanReprompt(t *testing.T) {
	for _, allMissing := range []bool{false, true} {
		t.Run(fmt.Sprintf("all-missing=%t", allMissing), func(t *testing.T) {
			f := newFixture(t)
			receiptTeam(t, f)
			file := os.Getenv("HERGY_HERDR")
			original := readFile(t, file)
			target := "alias-b"
			if allMissing {
				target = "*"
			}
			receiptHerdr(t, f, target, "agent_prompt_stalled", false)
			if allMissing {
				f.herdr(t, strings.Replace(readFile(t, file), `[ "$n" -le 2 ]`, `true`, 1))
			}
			var out, stderr bytes.Buffer
			if code := Run([]string{"up", "demo", "--as", "alias", "--lang", "ja"}, &out, &stderr); code != 1 {
				t.Fatalf("code=%d: %s", code, &stderr)
			}
			if !strings.Contains(stderr.String(), "hergy reprompt alias-b --team alias -C '"+f.proj+"'") {
				t.Fatalf("missing recovery: %s", &stderr)
			}
			_, records := launchLog(t, f)
			if len(records) != 1 || records[0].Team != "alias" || records[0].Definition != "demo" {
				t.Fatalf("alias missing: %+v", records)
			}
			f.herdr(t, original) // The same fake agents now accept the recovery prompt.
			out.Reset()
			stderr.Reset()
			if code := Run([]string{"reprompt", "alias-b", "--team", "alias", "-C", f.proj}, &out, &stderr); code != 0 {
				t.Fatalf("recovery code=%d: %s", code, &stderr)
			}
			_, records = launchLog(t, f)
			last := records[len(records)-1]
			if last.Command != launchlog.CommandReprompt || last.Definition != "demo" || len(last.Members) != 1 || last.Members[0].Name != "alias-b" {
				t.Fatalf("recovery=%+v", last)
			}
		})
	}
}
