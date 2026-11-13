package herdr

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
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
			err := New(port).ConfirmPrompt(fixtureSeen("recipient"), "identical text")
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

// shownPort is a pane as hergy reads it after it handed a text: one screen per
// reading, the last one repeated. It fixes the order of herdr's answers and the
// pane's drawing.
type shownPort struct {
	before   string
	screens  []string // "" is a reading herdr fails
	prompted bool
	reads    int
}

func (p *shownPort) Verify(string) error { return nil }

func (p *shownPort) JSON(args ...string) (map[string]any, error) {
	if len(args) >= 4 && args[0] == "agent" && args[1] == "prompt" {
		p.prompted = true
	}
	return map[string]any{}, nil
}

func (p *shownPort) Text(...string) (string, error) {
	if !p.prompted {
		return p.before, nil
	}
	screen := p.screens[min(p.reads, len(p.screens)-1)]
	p.reads++
	if screen == "" {
		return "", herdrFailure("agent_not_found")
	}
	return screen, nil
}

func TestConfirmShownPrompt(t *testing.T) {
	const (
		prompt = "You are demo-dev of the demo team.\n\n# Rules\n- hand the turn to demo-lead when done\n"
		drawn  = ">_ agent\n› " + prompt
		ready  = ">_ agent ready\n"
		asking = "Folder access\n› 1. Open restricted\n"
	)
	for _, tc := range []struct {
		name   string
		port   shownPort
		prompt string
		unseen bool
		pauses int
	}{
		{name: "drawn and kept", port: shownPort{screens: []string{drawn}}, prompt: prompt},
		{name: "drawn on the third reading", port: shownPort{screens: []string{ready, ready, drawn}}, prompt: prompt, pauses: 2},
		{name: "taken by a question herdr did not report", port: shownPort{before: asking, screens: []string{ready}}, prompt: prompt, unseen: true, pauses: 24},
		{name: "herdr does not know the agent while it takes the answer", port: shownPort{before: asking, screens: []string{"", ready}}, prompt: prompt, unseen: true, pauses: 24},
		{name: "only the last prompt, still on the screen with its answer", port: shownPort{before: drawn + "• ok\n", screens: []string{drawn + "• ok\n› Ask\n"}}, prompt: prompt, unseen: true, pauses: 24},
		{name: "drawn again under the last prompt", port: shownPort{before: drawn + "• ok\n", screens: []string{drawn + "• ok\n" + drawn}}, prompt: prompt},
		{name: "drawn again after the last prompt left the screen", port: shownPort{before: "- hand the turn to demo-lead when done\n• ok\n", screens: []string{"- hand the turn to demo-lead when done\n• Working\n"}}, prompt: prompt},
		{name: "drawn again in a frame, the last prompt gone from the screen", port: shownPort{before: "┃ - hand the turn to demo-lead when done\n┃\n  ok\n", screens: []string{"┃ - hand the turn to demo-lead when done\n┃\n┃ Build\n"}}, prompt: prompt},
		{name: "only the last prompt, in a frame", port: shownPort{before: "┃ - hand the turn to demo-lead when done\n┃\n  ok\n", screens: []string{"┃ - hand the turn to demo-lead when done\n┃\n  ok\n┃\n"}}, prompt: prompt, unseen: true, pauses: 24},
		{name: "a failed reading, then drawn", port: shownPort{screens: []string{"", drawn}}, prompt: prompt, pauses: 1},
		{name: "no line to look for", port: shownPort{screens: []string{ready}}, prompt: "ok"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pauses := 0
			err := New(&tc.port).ConfirmShownPrompt(fixtureSeen("demo-dev"), tc.prompt, func(time.Duration) error {
				pauses++
				return nil
			})
			if _, unseen := errors.AsType[*Unseen](err); unseen != tc.unseen || (err != nil) != tc.unseen {
				t.Fatalf("error=%v, want unseen=%v", err, tc.unseen)
			}
			if pauses != tc.pauses {
				t.Fatalf("pauses=%d, want %d", pauses, tc.pauses)
			}
		})
	}
}

func TestPromptProbesAreShortLineStarts(t *testing.T) {
	got := promptProbes("あなたは demo チームの demo-dev です。ロールは developer です。\n\n- a\n最後の行は人間に渡すときの決まりを書いている\n\n")
	want := []string{"あなたは demo チームの d", "最後の行は人間に渡すときの決まり"}
	if !slices.Equal(got, want) {
		t.Fatalf("probes=%q, want %q", got, want)
	}
}
