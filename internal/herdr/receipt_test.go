package herdr

import (
	"slices"
	"testing"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/protocol"
)

func TestConfirmPrompt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers []string
		calls   int
		failure string
	}{
		{"first", []string{""}, 1, ""},
		{"retry", []string{"agent_prompt_stalled", ""}, 2, ""},
		{"stalled", []string{"agent_prompt_stalled", "agent_prompt_stalled"}, 2, "agent_prompt_stalled"},
		{"blocked", []string{"agent_blocked", "agent_blocked"}, 2, "agent_blocked"},
		{"timeout", []string{"timeout", "timeout"}, 2, "timeout"},
		{"send-failure", []string{"send_failed", "send_failed"}, 2, "send_failed"},
		{"last-answer", []string{"agent_prompt_stalled", "send_failed"}, 2, "send_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := &fakePort{}
			port.reply = func(n int, _ []string) (map[string]any, error) {
				code := tc.answers[n-1]
				if code != "" {
					return nil, herdrFailure(code)
				}
				return map[string]any{}, nil
			}
			err := New(port).ConfirmPrompt(domainname.Observe("recipient"), "identical text")
			if protocol.ErrorCode(err) != tc.failure {
				t.Fatalf("last answer=%v, want %s", err, tc.failure)
			}
			if (err == nil) != (tc.failure == "") {
				t.Fatalf("error=%v", err)
			}
			if len(port.calls) != tc.calls {
				t.Fatalf("calls=%q", port.calls)
			}
			for _, args := range port.calls {
				if !slices.Equal(args, []string{"agent", "prompt", "recipient", "identical text", "--wait", "--until", "working", "--timeout", "5000"}) {
					t.Fatalf("wrong receipt request: %q", args)
				}
			}
		})
	}
}
