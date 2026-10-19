package herdr

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/protocol"
)

// fakePort answers JSON from a script and counts the calls, so a test can see
// how often the launcher asked Herdr to start an agent.
type fakePort struct {
	calls [][]string
	reply func(n int, args []string) (map[string]any, error)
}

func (p *fakePort) Verify(string) error { return nil }

func (p *fakePort) JSON(args ...string) (map[string]any, error) {
	p.calls = append(p.calls, args)
	return p.reply(len(p.calls), args)
}

func (p *fakePort) Text(args ...string) (string, error) {
	p.calls = append(p.calls, args)
	return "", nil
}

// herdrFailure is the error herdrcli builds from a failed herdr reply. The code
// is what Start reads to tell a busy pane from a real failure.
func herdrFailure(code string) error {
	_, err := protocol.Result([]string{"agent", "start"}, "",
		fmt.Sprintf(`{"error":{"code":%q}}`, code), 1)
	return err
}

func alwaysReply(err error) func(int, []string) (map[string]any, error) {
	return func(int, []string) (map[string]any, error) {
		if err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}
}

func TestStartRetriesWhileThePaneIsBusy(t *testing.T) {
	port := &fakePort{}
	port.reply = func(n int, _ []string) (map[string]any, error) {
		if n <= 2 {
			return nil, herdrFailure("agent_pane_busy")
		}
		return map[string]any{}, nil
	}
	member := &definition.Start{Name: "worker", Kind: "claude", Args: []string{"--flag"}}

	if err := New(port).Start(member, "p1", 30000); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(port.calls) != 3 {
		t.Fatalf("Start called herdr %d times, want 3", len(port.calls))
	}
	want := []string{"agent", "start", "worker", "--kind", "claude", "--pane", "p1",
		"--timeout", "30000", "--", "--flag"}
	if !slices.Equal(port.calls[0], want) {
		t.Fatalf("Start called %q, want %q", port.calls[0], want)
	}
}

func TestStartGivesUpAfterTheTimeout(t *testing.T) {
	const timeoutMS = 250
	port := &fakePort{reply: alwaysReply(herdrFailure("agent_pane_busy"))}

	start := time.Now()
	err := New(port).Start(&definition.Start{Name: "worker", Kind: "claude"}, "p1", timeoutMS)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Start returned no error while the pane stayed busy")
	}
	if got := protocol.ErrorCode(err); got != "agent_pane_busy" {
		t.Fatalf("ErrorCode = %q, want agent_pane_busy (err: %v)", got, err)
	}
	if elapsed < timeoutMS*time.Millisecond {
		t.Fatalf("Start stopped after %v, before the %dms timeout", elapsed, timeoutMS)
	}
	if len(port.calls) < 2 {
		t.Fatalf("Start called herdr %d times, want it to retry at least once", len(port.calls))
	}
	if len(port.calls) > 10 {
		t.Fatalf("Start called herdr %d times; it should stop at the timeout", len(port.calls))
	}
}

func TestStartDoesNotRetryAnotherError(t *testing.T) {
	port := &fakePort{reply: alwaysReply(herdrFailure("agent_kind_unknown"))}

	err := New(port).Start(&definition.Start{Name: "worker", Kind: "claude"}, "p1", 30000)

	if err == nil {
		t.Fatal("Start returned no error")
	}
	if got := protocol.ErrorCode(err); got != "agent_kind_unknown" {
		t.Fatalf("ErrorCode = %q, want agent_kind_unknown (err: %v)", got, err)
	}
	if len(port.calls) != 1 {
		t.Fatalf("Start called herdr %d times, want 1", len(port.calls))
	}
}

// TestFindPaneReadsTheLabel: the name hergy gives a pane with pane rename comes
// back as "label" in pane list; a label no pane carries returns empty.
func TestFindPaneReadsTheLabel(t *testing.T) {
	port := &fakePort{reply: func(_ int, args []string) (map[string]any, error) {
		if want := []string{"pane", "list"}; !slices.Equal(args, want) {
			return nil, fmt.Errorf("unexpected args %q, want %q", args, want)
		}
		return map[string]any{"panes": []any{
			map[string]any{"pane_id": "p1", "label": "team-lead"},
			map[string]any{"pane_id": "p2"},
		}}, nil
	}}

	got, err := New(port).FindPane("team-lead")
	if err != nil {
		t.Fatalf("FindPane: %v", err)
	}
	if got != "p1" {
		t.Fatalf("FindPane = %q, want p1", got)
	}

	got, err = New(port).FindPane("nobody")
	if err != nil {
		t.Fatalf("FindPane(nobody): %v", err)
	}
	if got != "" {
		t.Fatalf("FindPane(nobody) = %q, want empty", got)
	}
}

// TestPaneProcessInfoReadsTheGroupAndShell: the pane's foreground group and
// shell pid come from result.process_info, and a reply without a number fails.
func TestPaneProcessInfoReadsTheGroupAndShell(t *testing.T) {
	port := &fakePort{reply: func(_ int, args []string) (map[string]any, error) {
		if want := []string{"pane", "process-info", "--pane", "p1"}; !slices.Equal(args, want) {
			return nil, fmt.Errorf("unexpected args %q, want %q", args, want)
		}
		return map[string]any{"process_info": map[string]any{
			"foreground_process_group_id": float64(4242),
			"shell_pid":                   float64(900),
		}}, nil
	}}

	info, err := New(port).PaneProcessInfo("p1")
	if err != nil {
		t.Fatalf("PaneProcessInfo: %v", err)
	}
	if info.Pgid != 4242 || info.ShellPID != 900 {
		t.Fatalf("PaneProcessInfo = %+v, want {Pgid:4242 ShellPID:900}", info)
	}

	port.reply = alwaysReply(nil)
	if _, err := New(port).PaneProcessInfo("p1"); err == nil {
		t.Fatal("PaneProcessInfo accepted a reply with no process_info")
	}
}

// TestFocusSendsTheTarget: focus asks herdr to move to the target, and reports
// herdr's own error when it refuses.
func TestFocusSendsTheTarget(t *testing.T) {
	port := &fakePort{}
	port.reply = alwaysReply(nil)
	if err := New(port).Focus("p-lead"); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	if want := []string{"agent", "focus", "p-lead"}; !slices.Equal(port.calls[0], want) {
		t.Fatalf("Focus called %v, want %v", port.calls[0], want)
	}

	port.reply = func(int, []string) (map[string]any, error) {
		return nil, herdrFailure("agent_not_found")
	}
	err := New(port).Focus("gone")
	if protocol.ErrorCode(err) != "agent_not_found" {
		t.Fatalf("Focus error = %v, want agent_not_found", err)
	}
}
