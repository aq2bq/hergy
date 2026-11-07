package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/turns"
)

// TestTurnDoneWritesTheHarnessesSignal: Claude Code's Stop hook JSON on stdin
// becomes one signal in the team's signal directory, with the turn's last
// reply and which harness spoke (r-378e).
func TestTurnDoneWritesTheHarnessesSignal(t *testing.T) {
	f := newFixture(t)
	stdin := strings.NewReader(`{"session_id":"s","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"積んだ。\n→ spec"}`)
	if got := runWithStdin(t, stdin, "turn-done", "demo", "lead"); strings.TrimSpace(got) != "" {
		t.Errorf("turn-done printed %q, want nothing", got)
	}
	dir := turns.SignalDir(filepath.Join(f.state, "hergy"), "demo")
	sigs, err := turns.TakeSignals(dir)
	if err != nil || len(sigs) != 1 {
		t.Fatalf("TakeSignals = %+v, %v; want one signal", sigs, err)
	}
	if sigs[0].Member != "lead" || sigs[0].Text != "積んだ。\n→ spec" || sigs[0].Source != "claude" || time.Since(sigs[0].At) > time.Minute {
		t.Errorf("signal = %+v", sigs[0])
	}
	// Nothing on stdin and no argument: the harness handed nothing.
	if _, err := runExpectingError(t, strings.NewReader(""), "turn-done", "demo", "lead"); err == nil || !strings.Contains(err.Error(), "turn-done") {
		t.Errorf("an empty payload was taken: %v", err)
	}
}

// TestTurnDoneCallsThePersonsCodexNotifyAfterwards: Codex's JSON comes as one
// argument; the person's own notify (HERGY_NOTIFY_THEN, from their
// config.toml) is called with the same argument (d-a70c).
func TestTurnDoneCallsThePersonsCodexNotifyAfterwards(t *testing.T) {
	f := newFixture(t)
	called := filepath.Join(f.cfg, "notified.txt")
	script := filepath.Join(f.cfg, "notify.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s|%s' \"$1\" \"$2\" > '"+called+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_NOTIFY_THEN", `["`+script+`","turn-ended"]`)
	payload := `{"type":"agent-turn-complete","thread-id":"t","last-assistant-message":"pong"}`
	runWithStdin(t, strings.NewReader(""), "turn-done", "demo", "dev", payload)
	sigs, err := turns.TakeSignals(turns.SignalDir(filepath.Join(f.state, "hergy"), "demo"))
	if err != nil || len(sigs) != 1 || sigs[0].Source != "codex" || sigs[0].Text != "pong" || sigs[0].Member != "dev" {
		t.Fatalf("TakeSignals = %+v, %v", sigs, err)
	}
	var got []byte
	for range 50 {
		if got, err = os.ReadFile(called); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(got) != "turn-ended|"+payload {
		t.Errorf("the person's notify got %q, want turn-ended and the same JSON", got)
	}
}

// runWithStdin runs one command with the given stdin, as a hook would call it.
func runWithStdin(t *testing.T, stdin io.Reader, argv ...string) string {
	t.Helper()
	stdout, stderr, code := runStdin(t, stdin, argv...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstderr:\n%s", argv, code, stderr)
	}
	return stdout
}

func runExpectingError(t *testing.T, stdin io.Reader, argv ...string) (string, error) {
	t.Helper()
	stdout, stderr, code := runStdin(t, stdin, argv...)
	if code == 0 {
		return stdout, nil
	}
	return stdout, errors.New(stderr)
}

func runStdin(t *testing.T, stdin io.Reader, argv ...string) (string, string, int) {
	t.Helper()
	raw, err := io.ReadAll(stdin)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	was := os.Stdin
	os.Stdin = file
	defer func() { os.Stdin = was }()
	return run(argv...)
}
