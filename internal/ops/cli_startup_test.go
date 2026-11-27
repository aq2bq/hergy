package ops

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher" // TestBootInterruptedWhileWaitingOnAQuestion: Ctrl-C while hergy waits on a
	// member's question stops with 130, says where it stopped, writes the member's
	// state as waiting for permission, and ends with the one line that says how to go
	// on: the person answers in that pane, then runs hergy down and the same hergy up.
)

func TestBootInterruptedWhileWaitingOnAQuestion(t *testing.T) {
	f := newFixture(t)
	inJapanese(t)
	log := f.blockedStartHerdr(t)
	starts := []definition.Start{{Name: fixtureHerdrName("demo-lead"), Short: fixtureShortName("lead"), Role: "lead", Kind: "claude", Prompt: "read the role document", Cwd: f.proj}}
	var out lockedBuffer
	var errOut bytes.Buffer
	l := launcher.New(starts, fixtureSeen("demo"), projectPath(f.proj), herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))), 30000, &out)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runOperation(ctx, &errOut, l.ReportFailure, l.ReportInterrupted, func() error { defer close(finished); return l.Boot("hergy up demo") })
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "waiting for permission: demo-lead") {
		if time.Now().After(deadline) {
			t.Fatalf("the boot never said it waits for the question:\n%s", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		var reported reportedError
		if !errors.As(err, &reported) || reported.code != 130 {
			t.Fatalf("Ctrl-C returned %v, want an interrupt with exit code 130", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl-C did not stop the boot")
	}
	said := errOut.String()
	for _, want := range []string{
		"interrupted\n",
		"stopped: waiting for someone to answer demo-lead's question",
		"tab: t1",
		"member: demo-lead (pane p1, waiting for permission",
		"to go on: answer the question in demo-lead's pane (tab demo (t1)), then run hergy down demo and then hergy up demo\n",
	} {
		if !strings.Contains(said, want) {
			t.Fatalf("the interrupt does not say %q:\n%s", want, said)
		}
	}

	last := strings.TrimSuffix(strings.TrimSpace(said), "\n")
	if i := strings.LastIndex(last, "\n"); i >= 0 {
		last = last[i+1:]
	}
	if !strings.HasPrefix(last, "to go on: ") {
		t.Fatalf("the interrupt ends with %q, not with the way on:\n%s", last, said)
	}
	if strings.Contains(said, "agent_not_ready") {
		t.Fatalf("the interrupt shows herdr's raw error:\n%s", said)
	}

	if err := os.WriteFile(filepath.Join(f.cfg, "answered"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the boot did not finish after the question was answered")
	}
	if !strings.Contains(calls(t, log), "agent prompt demo-lead") {
		t.Fatalf("the boot never submitted the bootstrap prompt after the answer:\n%s", calls(t, log))
	}
}
