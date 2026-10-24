package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/launcher"
)

// blockedStartHerdr writes a fake herdr whose start is refused because the agent
// asks a person a question while it starts, and whose agent stays blocked until
// <cfg>/answered exists: a person answering in the pane is that file appearing.
// It records every call in <cfg>/herdr.log and returns the log's path.
func (f *fixture) blockedStartHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	answered := filepath.Join(f.cfg, "answered")
	script := `#!/bin/sh
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "agent start")  echo '{"error":{"code":"agent_not_ready","message":"agent demo-lead is blocked during startup and is not ready for prompts"}}' ;;
  "agent get")
    if [ -f '` + answered + `' ]; then
      echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}}}'
    else
      echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1"}}}'
    fi
    ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// lockedBuffer is a buffer one goroutine may read while another writes. The boot
// runs in its own goroutine, as it does under hergy, and the test reads what it
// printed while it waits for the person's answer.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestBootInterruptedWhileWaitingOnAQuestion: Ctrl-C while hergy waits on a
// member's question stops with 130, says where it stopped, writes the member's
// state as waiting for permission, and ends with the one line that says how to go
// on: the person answers in that pane, then runs hergy down and the same hergy up.
func TestBootInterruptedWhileWaitingOnAQuestion(t *testing.T) {
	f := newFixture(t)
	inJapanese(t)
	log := f.blockedStartHerdr(t)
	starts := []definition.Start{{
		Name: "demo-lead", Short: "lead", Role: "lead", Kind: "claude",
		Prompt: "read the role document", Cwd: f.proj,
	}}
	var out lockedBuffer
	var errOut bytes.Buffer
	l := launcher.New(starts, "demo", herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))), 30000, &out)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runOperation(ctx, &errOut, l.ReportFailure, l.ReportInterrupted, func() error {
			defer close(finished)
			return l.Boot("hergy up demo")
		})
	}()
	// Wait for the boot to hand the turn to the person: the waiting line is what
	// says so, and it is printed once, before the wait.
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "許可待ち: demo-lead") {
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
		"中断しました\n",
		"止まったところ: demo-lead が確認に答えてもらうのを待っています",
		"タブ: t1",
		"メンバー: demo-lead（ペイン p1、許可待ち（起動の途中の確認に答えを待っている））",
		"続けるには: タブ demo（t1）の demo-lead のペインで確認に答えてから、hergy down demo のあと hergy up demo を実行してください\n",
	} {
		if !strings.Contains(said, want) {
			t.Fatalf("the interrupt does not say %q:\n%s", want, said)
		}
	}
	// The line that says how to go on is the last one: it is read after the
	// panes that were kept.
	last := strings.TrimSuffix(strings.TrimSpace(said), "\n")
	if i := strings.LastIndex(last, "\n"); i >= 0 {
		last = last[i+1:]
	}
	if !strings.HasPrefix(last, "続けるには: ") {
		t.Fatalf("the interrupt ends with %q, not with the way on:\n%s", last, said)
	}
	if strings.Contains(said, "agent_not_ready") {
		t.Fatalf("the interrupt shows herdr's raw error:\n%s", said)
	}
	// Let the abandoned boot finish, so nothing keeps polling after the test:
	// the person answers, and the boot submits the prompt and ends.
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
