package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
)

func TestScreenApplyDeduplicatesRecipientsBeforeSending(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	writeFile(t, role, readFile(t, role)+"\nChanged instruction.\n")
	log := receiptHerdr(t, f, "unused", "", false)
	h := newActivityRead(serveEnv(t)).hq
	result, err := Apply(context.Background(), h.applyInput([]string{"demo-a", "demo-a", "demo-b", "demo-b"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Selection.Selected) != 2 {
		t.Fatalf("recipients: %+v", result.Selection)
	}
	text := readFile(t, log)
	if strings.Count(text, "agent prompt p-demo-a ") != 1 || strings.Count(text, "agent prompt p-demo-b ") != 1 {
		t.Fatalf("duplicate sends:\n%s", text)
	}
}

type applyCallbackPort struct {
	herdr.Port
	onPrompt func()
	called   bool
}

func (p *applyCallbackPort) JSON(args ...string) (map[string]any, error) {
	if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" && !p.called {
		p.called = true
		p.onPrompt()
	}
	return p.Port.JSON(args...)
}
func TestScreenApplyLocksAndCancelsEverySelectedTeam(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	mustRun(t, "up", "demo", "--as", "second")
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	writeFile(t, role, readFile(t, role)+"\nChanged instruction.\n")
	log := receiptHerdr(t, f, "unused", "", false)
	a := newActivityRead(serveEnv(t))
	h := a.hq
	port := &applyCallbackPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR"))}
	a.port = port
	selected, refusals := applySelection(a.plan(a.clock()), []string{"demo-a", "second-a"})
	if len(refusals) > 0 {
		t.Fatal(refusals)
	}
	op := h.newOp(applyLaunch(selected))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op.cancel = cancel
	port.onPrompt = func() {
		for _, name := range []string{"demo", "second"} {
			if id, active := h.activeLaunch(name); !active || id != op.launch.ID {
				t.Errorf("%s missing from the pending apply: %q %t", name, id, active)
			}
		}
		_, err := h.ApplyMembers([]string{"second-a"})
		refusal, ok := errors.AsType[*HQError](err)
		if !ok || refusal.Status != 409 {
			t.Errorf("second team was not locked: %v", err)
		}

		stopped, err := h.Stop("second")
		if err != nil || stopped.Closed.Members != 3 {
			t.Errorf("second-team stop: %+v, %v", stopped, err)
		}

	}
	h.runApply(ctx, op, selected)
	if !port.called || op.snapshot().Phase != "cancelled" {
		t.Fatalf("apply state: %+v", op.snapshot())
	}
	text := readFile(t, log)
	if strings.Contains(text, "agent prompt p-second-a ") {
		t.Fatalf("the cancelled operation sent to the second team:\n%s", text)
	}
}
func TestApplyReturnsTypedStartupFailureForBothAdapters(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	profile := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	writeFile(t, profile, strings.Replace(readFile(t, profile), "kind: claude", "kind: claude\n    args: [--model, test]", 1))
	receiptHerdr(t, f, "unused", "", false)
	fake := os.Getenv("HERGY_HERDR")
	script := strings.Replace(readFile(t, fake), `case "$g $s" in`, `case "$g $s" in`+"\n"+`  "agent start")
    if [ "$3" = demo-b ]; then echo '{"error":{"code":"start_failed","message":"refused start"}}'; exit 1; fi
    ;;`, 1)

	script = strings.Replace(script, `  "agent start")
    if [ "$3" = demo-b ]; then echo '{"error":{"code":"start_failed","message":"refused start"}}'; exit 1; fi
    ;;`, `  "agent start")
    if [ "$3" = demo-b ]; then echo '{"error":{"code":"start_failed","message":"refused start"}}'; exit 1; fi
    name="$3"; tab=$(tail -n 1 "$tabs" | cut -d' ' -f1); printf '%s %s\n' "$tab" "$name" >> "$state"; echo '{"result":{}}' ;;`, 1)
	f.herdr(t, script)
	h := newActivityRead(serveEnv(t)).hq
	_, err := Apply(context.Background(), h.applyInput([]string{"demo-a", "demo-b", "demo-c"}, nil))
	failure, ok := errors.AsType[*LaunchFailure](err)
	if !ok || failure.Failed.Member != "demo-b" || len(failure.Failed.Started) != 1 || failure.Failed.Started[0] != "demo-a" || failure.Failed.Panes == 0 || !strings.Contains(failure.Command, "apply") {
		t.Fatalf("typed failure: %+v, %v", failure, err)
	}
	for _, want := range []string{"demo-a", "demo-b", "refused start", "hergy apply"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure message lacks %q: %v", want, err)
		}
	}
}
