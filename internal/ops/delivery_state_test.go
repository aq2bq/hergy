package ops

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/source"
)

func TestRepromptDoesNotHideAProcessThatNeedsRestart(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	before := teamState(t, "demo").Members()[0]
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, changed]\n")
	mustRun(t, "reprompt", "a", "--team", "demo")
	after := teamState(t, "demo").Members()[0]
	if after.Start != before.Start || after.Env != before.Env {
		t.Errorf("reprompt rewrote the running process: before=%s/%s after=%s/%s", before.Start, before.Env, after.Start, after.Env)
	}
	out := mustRun(t, "apply", "a", "--team", "demo", "--dry", "--json")
	if !strings.Contains(out, `"restart"`) {
		t.Fatalf("reprompt hid the required restart: %s", out)
	}
}

// captureDeliveryPort observes the prompt at the external boundary without
// affecting the order in which the operation records confirmed receipt.
type captureDeliveryPort struct {
	herdr.Port
	prompts []string
	before  func()
}

func (p *captureDeliveryPort) JSON(args ...string) (map[string]any, error) {
	if len(args) >= 4 && args[0] == "agent" && args[1] == "prompt" {
		p.prompts = append(p.prompts, args[3])
		if p.before != nil {
			p.before()
			p.before = nil
		}
	}
	return p.Port.JSON(args...)
}

func TestReceiptKeepsTheRulesThatBuiltTheDeliveredPrompt(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	delivered := readFile(t, role) + "\nRule handed to this member.\n"
	writeFile(t, role, delivered)
	port := &captureDeliveryPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR")), before: func() { writeFile(t, role, "# Later rules\nNot handed to the member.\n") }}
	_, err := Reprompt(context.Background(), MemberInput{Env: serveEnv(t), Team: downInput("demo"), Member: "a", Port: port, Timeout: 1000})
	if err != nil {
		t.Fatal(err)
	}
	receipt := teamState(t, "demo").Members()[0]
	if !strings.Contains(receipt.Prompt, delivered) {
		t.Fatal("prompt did not contain its built rules")
	}
	for _, doc := range receipt.Docs {
		if doc.Path == role {
			if doc.Text != delivered {
				t.Fatalf("recorded later rules: %q", doc.Text)
			}
			return
		}
	}
	t.Fatal("missing rule contents")
}

func TestDeliveryReadingsAndSendsSurviveDeletedLaunchHistory(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	env := serveEnv(t)
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	writeFile(t, role, readFile(t, role)+"\nA changed instruction for delivery.\n")
	beforeCLI := mustRun(t, "apply", "--dry", "--json", "--team", "demo")
	beforeRead := newActivityRead(env).Apply()
	history, err := newActivityRead(env).PromptHistory(env.ProjectRoot, "demo", "project", "a", "demo")
	if err != nil || history.Recorded == nil {
		t.Fatalf("history before deletion: %+v %v", history, err)
	}
	plan, err := memberPlan(MemberInput{Env: env, Team: downInput("demo"), Member: "a"}, "reprompt", false)
	if err != nil {
		t.Fatal(err)
	}
	expected := promptsToResend(env.Catalog(source.Disk{}), env.ProjectRoot, plan.Resolved, plan.Members)
	b, err := plan.Resolved.SelectStarts("b")
	if err != nil {
		t.Fatal(err)
	}
	expectedB := promptsToResend(env.Catalog(source.Disk{}), env.ProjectRoot, plan.Resolved, b)
	if err := os.RemoveAll(filepath.Join(f.state, "hergy", "launches")); err != nil {
		t.Fatal(err)
	}
	if after := mustRun(t, "apply", "--dry", "--json", "--team", "demo"); after != beforeCLI {
		t.Fatalf("dry changed after deletion:\n%s\n%s", beforeCLI, after)
	}
	if after := newActivityRead(env).Apply(); !reflect.DeepEqual(beforeRead, after) {
		t.Fatalf("sheet changed: %+v / %+v", beforeRead, after)
	}
	if after, err := newActivityRead(env).PromptHistory(env.ProjectRoot, "demo", "project", "a", "demo"); err != nil || !reflect.DeepEqual(history, after) {
		t.Fatalf("history changed: %+v / %+v err=%v", history, after, err)
	}
	port := &captureDeliveryPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR"))}
	if _, err := Reprompt(context.Background(), MemberInput{Env: env, Team: downInput("demo"), Member: "a", Port: port, Timeout: 1000}); err != nil {
		t.Fatal(err)
	}
	if len(port.prompts) != 1 || port.prompts[0] != expected[0].Prompt {
		t.Fatalf("reprompt lost preamble: %+v", port.prompts)
	}
	// Reprompt recreated the independent history. Removing it again must not
	// change the pending apply for the other two members.
	if err := os.RemoveAll(filepath.Join(f.state, "hergy", "launches")); err != nil {
		t.Fatal(err)
	}
	port.prompts = nil
	result, err := Apply(context.Background(), ApplyInput{Env: env, Selection: ApplySelectionInput{Names: []string{"b"}, Team: "demo"}, Port: port, Timeout: 1000})
	if err != nil || result.FinalError != nil {
		t.Fatalf("apply: %v / %v", err, result.FinalError)
	}
	if len(port.prompts) != 1 || port.prompts[0] != expectedB[0].Prompt {
		t.Fatalf("apply lost preamble: %+v", port.prompts)
	}
	after := newActivityRead(env).Apply()
	for _, team := range after.Teams {
		for _, member := range team.Members {
			if member.Short == "b" {
				t.Fatalf("apply did not record b: %+v", member)
			}
		}
	}
}

func TestDownThenUpDoesNotCompareAnEarlierMembersReceipt(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	if len(teamState(t, "demo").Members()) != 3 {
		t.Fatal("first run did not record its members")
	}
	mustRun(t, "down", "demo")
	// The fake retains empty tabs just as down does; start in a fresh fake session.
	f.statefulHerdr(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), "version: 1\ndescription: second run\nmembers:\n  - {name: a, role: lead, profile: claude-default}\nturns:\n  - human -> a\n  - a -> human\n")
	mustRun(t, "up", "demo")
	// An unrelated agent happens to reuse a former member's name. Only this run's
	// receipts can make it a removed member in apply.
	writeFile(t, filepath.Join(f.cfg, "agents.txt"), readFile(t, filepath.Join(f.cfg, "agents.txt"))+"t1 demo-b\n")
	if got := teamState(t, "demo").Members(); len(got) != 1 || got[0].Short != "a" {
		t.Fatalf("old recipients: %+v", got)
	}
	out := mustRun(t, "apply", "--team", "demo", "--dry", "--json")
	if strings.Contains(out, "demo-b") || strings.Contains(out, "demo-c") {
		t.Fatalf("earlier run leaked into comparison: %s", out)
	}
}

func TestRefreshKeepsThePaneEnvironmentAndItsApplyDrift(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	before := teamState(t, "demo").Members()[0]
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, changed]\n    env: {MODE: changed}\n")
	mustRun(t, "reprompt", "a", "--team", "demo")
	resent := teamState(t, "demo").Members()[0]
	if before.Env != resent.Env || before.Start != resent.Start {
		t.Fatal("reprompt replaced process fingerprints")
	}
	_, _, code := run("apply", "a", "--team", "demo", "--dry", "--json")
	if code != 1 {
		t.Fatal("changed env was accepted after reprompt")
	}
	receiptHerdr(t, f, "unused", "", false)
	mustRun(t, "refresh", "a", "--team", "demo")
	refreshed := teamState(t, "demo").Members()[0]
	if refreshed.Start == before.Start || refreshed.Env != before.Env {
		t.Errorf("refresh did not keep its pane environment: before=%+v after=%+v", before, refreshed)
	}
	out, _, code := run("apply", "a", "--team", "demo", "--dry", "--json")
	if code != 1 || !strings.Contains(out, `"env"`) {
		t.Fatalf("refresh hid the unchanged pane environment: code=%d %s", code, out)
	}
}

func TestApplyRecordsTheEnvironmentOnlyForANewMembersPane(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	text := readFile(t, path)
	text = strings.Replace(text, "workers:\n", "  - {name: d, role: lead, profile: added}\nworkers:\n", 1)
	text = strings.Replace(text, "  - c -> human", "  - c -> d\n  - d -> human", 1)
	writeFile(t, path, text)
	profiles := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	writeFile(t, profiles, readFile(t, profiles)+"  added:\n    kind: claude\n    env: {MODE: new-pane}\n")
	// The normal startup fake has no pane-list answer; an apply addition needs
	// the named existing panes as split sources.
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	script = strings.Replace(script, `  "pane list") echo '{"result":{"panes":[]}}' ;;`, `  "pane list") echo '{"result":{"panes":[{"pane_id":"p-demo-a","label":"demo-a","tab_id":"t1"}]}}' ;;`, 1)
	f.herdr(t, script)
	mustRun(t, "apply", "d", "--team", "demo")
	for _, receipt := range teamState(t, "demo").Members() {
		if receipt.Short == "d" {
			want := runningstate.EnvFingerprint(map[string]string{"MODE": "new-pane", "GY_ACTOR": "demo-d"})
			if receipt.Env != want {
				t.Fatalf("new pane environment: got %s want %s", receipt.Env, want)
			}
			return
		}
	}
	t.Fatal("apply did not record the new member")
}
