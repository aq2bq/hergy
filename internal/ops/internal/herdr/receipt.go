package herdr

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
)

// ConfirmPrompt owns confirmed sending (d-b4b5, r-990d): herdr must observe
// working within five seconds. Retry the identical text once on any error,
// returning the last answer. Callers decide what to do with that answer; the
// watcher gives the turn to the human, while a launch distinguishes an absent
// receipt from a failure to send (d-b804).
func (h *Herdr) ConfirmPrompt(target domainname.Target, text string) error {
	var err error
	for range 2 {
		if err = h.PromptWait(target, text, 5000); err == nil {
			return nil
		}
	}
	return err
}

// Unseen is ConfirmShownPrompt's answer when herdr saw the agent working but
// the pane never showed the text. An agent that asks a human a question while
// it starts, and that herdr does not report as blocked, takes the text's
// enter as the answer and drops the text; what it does next reads as working
// (n-6a78, measured 2026-11-13 with Codex's folder access question).
type Unseen struct{}

func (*Unseen) Error() string { return i18n.T("launcher.prompt_unseen") }

const (
	// shownTries and shownStep bound the wait for the agent to draw the text
	// it was handed after herdr saw it working: as long as herdr itself waits
	// for the agent to work.
	shownTries = 25
	shownStep  = 200 * time.Millisecond
	// probeRunes is how much of a line is looked for: short enough that no
	// harness wraps it in a pane hergy lays out, long enough to be the text.
	probeRunes    = 16
	probeMinRunes = 6
)

// ConfirmShownPrompt is ConfirmPrompt for a text handed to an agent: after
// herdr sees the agent working, the pane must also show the text. The three
// harnesses keep the end of a handed text on the screen from before they work
// until they stop, and a question that took the text never shows it (measured
// 2026-11-13, reading every 50 ms). The screen is read
// before and after, so a text the screen already showed (the last prompt of a
// member that answered in a line) is not taken for this one. pause waits
// between two readings and may end the wait. A reading that fails shows
// nothing. A text with no line to look for leaves herdr's own answer standing.
func (h *Herdr) ConfirmShownPrompt(target domainname.Target, text string, pause func(time.Duration) error) error {
	probes := promptProbes(text)
	if len(probes) == 0 {
		return h.ConfirmPrompt(target, text)
	}
	before, _ := h.ReadVisible(target)
	if err := h.ConfirmPrompt(target, text); err != nil {
		return err
	}
	for try := range shownTries {
		if after, err := h.ReadVisible(target); err == nil && showsNewly(before, after, probes) {
			return nil
		}
		if try < shownTries-1 {
			if err := pause(shownStep); err != nil {
				return err
			}
		}
	}
	return &Unseen{}
}

// ReadVisible reads what a live agent's pane shows now. Herdr hands it over
// while the agent works, which it does not do for the lines above the screen.
func (h *Herdr) ReadVisible(target domainname.Target) (string, error) {
	return h.port.Text("agent", "read", target.String(), "--source", "visible")
}

// promptProbes is the start of the text's first and last lines that are long
// enough to tell apart: the first stands at the top of what the agent draws,
// and the last is still in reach when the text is longer than herdr reads.
func promptProbes(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if utf8.RuneCountInString(line) >= probeMinRunes {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	head, tail := probeOf(lines[0]), probeOf(lines[len(lines)-1])
	if head == tail {
		return []string{head}
	}
	return []string{head, tail}
}

func probeOf(line string) string {
	runes := []rune(line)
	if len(runes) > probeRunes {
		runes = runes[:probeRunes]
	}
	return string(runes)
}

// showsNewly says the screen shows one of the probes where it did not: more
// often than before, or with another line under its last showing. The second
// is a text handed again to a member whose screen still shows the last one:
// the old showing has the member's answer under it, the new one does not.
func showsNewly(before, after string, probes []string) bool {
	for _, probe := range probes {
		was, now := strings.Count(before, probe), strings.Count(after, probe)
		if now > was || (now > 0 && lineUnder(after, probe) != lineUnder(before, probe)) {
			return true
		}
	}
	return false
}

// lineUnder is the first line with a word on it under the last showing of
// probe. A harness that frames what it draws puts a rule on every line, so a
// line of nothing but rules and spaces is not one.
func lineUnder(screen, probe string) string {
	at := strings.LastIndex(screen, probe)
	if at < 0 {
		return ""
	}
	_, rest, _ := strings.Cut(screen[at:], "\n")
	for line := range strings.SplitSeq(rest, "\n") {
		if strings.ContainsFunc(line, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
