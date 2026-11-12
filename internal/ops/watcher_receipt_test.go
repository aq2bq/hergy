package ops

import (
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/turns"
	"slices"
	"strings"
	"testing"
)

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
