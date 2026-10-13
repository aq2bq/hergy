package herdr

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/config"
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
	member := &config.Member{Name: "worker", Kind: "claude", Args: []string{"--flag"}}

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
	err := New(port).Start(&config.Member{Name: "worker", Kind: "claude"}, "p1", timeoutMS)
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

	err := New(port).Start(&config.Member{Name: "worker", Kind: "claude"}, "p1", 30000)

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
