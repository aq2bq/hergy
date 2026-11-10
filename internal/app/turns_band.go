package app

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/sys/unix"

	"github.com/aq2bq/hergy/internal/turns"
)

// The band is what the watcher shows in its own pane (r-4e04, d-1a54ca): three
// lines, redrawn in place, so a person reads the relay at a glance and the pane
// stays small. The sentences the watcher used to print there go to the team's
// log file instead (turns.LogPath), stamped with the time; this file only reads
// the state and the actions and draws.
//
//	 sig   lead ○   dev ●   spec ○  │ turn: dev (from lead) │ waiting: lead → dev
//	 21:28:47 lead→dev →   21:29:11 dev→human ✓
//	 ? asking dev (lead waiting)
//
// Line 1: the team, each member's state (● working, ○ stopped, ◐ blocked,
// · unknown), who holds the turn and from whom, who waits for whose answer.
// Line 2: the latest handoffs, oldest left, as many as fit; each is the time,
// from→to and a mark (→ handed, ✓ to the human, ? asked, ⟲ rested, ✗ not
// delivered, unclear or silent). Line 3: only while hergy asks or waits, or
// when a read failed.

const (
	// bandLines is how many lines the band draws; hergy up cuts the watcher's
	// pane to bandRows (the lines and one spare, so writing the last line never
	// scrolls the first away).
	bandLines = 3
	// bandEvents is how many handoffs the band remembers; fewer may fit.
	bandEvents = 12
	// bandDefaultWidth is used when the pane's width cannot be read.
	bandDefaultWidth = 100
)

// bandEvent is one handoff as the band shows it.
type bandEvent struct {
	At       time.Time
	From, To string
	Mark     string
}

// band is the watcher's picture of the relay.
type band struct {
	team    string
	members []turns.Agent
	events  []bandEvent
	problem string // the last read error, until the next good read
	width   func() int
}

func newBand(team string) *band {
	return &band{team: team, width: terminalWidth}
}

// seen takes the agents as the watcher read them this tick.
func (b *band) seen(now []turns.Agent) {
	b.members = now
	b.problem = ""
}

// failed notes a read error for the third line.
func (b *band) failed(text string) { b.problem = strings.TrimSpace(text) }

// note turns one of the watcher's actions into an event, or nothing.
func (b *band) note(action turns.Action, at time.Time) {
	var ev bandEvent
	switch action.Kind {
	case turns.Told, turns.Deliver, turns.Box, turns.Started:
		if action.To == turns.Human {
			return // the turn's end says it, with ✓
		}
		ev = bandEvent{From: action.From, To: action.To, Mark: "→"}
	case turns.ToHuman:
		mark := "✓"
		if action.Why != "" {
			mark = "✗"
		}
		ev = bandEvent{From: action.From, To: turns.Human, Mark: mark}
	case turns.Ask:
		ev = bandEvent{To: action.To, Mark: "?"}
	case turns.Rested:
		ev = bandEvent{From: action.From, Mark: "⟲"}
	case turns.Dropped:
		ev = bandEvent{From: action.From, Mark: "✗"}
	default:
		return
	}
	ev.At = at
	b.events = append(b.events, ev)
	if len(b.events) > bandEvents {
		b.events = b.events[len(b.events)-bandEvents:]
	}
}

// render draws the band as bandLines plain lines, each at most width cells.
func (b *band) render(st *turns.State, width int) []string {
	lines := []string{b.statusLine(st), b.eventsLine(width), b.pendingLine(st)}
	for i := range lines {
		lines[i] = clip(lines[i], width)
	}
	return lines
}

func (b *band) statusLine(st *turns.State) string {
	var sb strings.Builder
	sb.WriteString(" " + b.team + "  ")
	for _, m := range b.members {
		sb.WriteString(" " + m.Name + " " + stateMark(m.Status) + " ")
	}
	sb.WriteString(" │ " + "turn: " + bandParty(st.Holder))
	if st.From != "" && st.From != st.Holder {
		sb.WriteString(" (from " + bandParty(st.From) + ")")
	}
	if waits := st.Waits(); len(waits) > 0 {
		sb.WriteString(" │ " + "waiting: " + strings.Join(waits, ", "))
	}
	return sb.String()
}

// eventsLine fits the latest events in from the right: the newest is always
// shown, older ones as the width allows.
func (b *band) eventsLine(width int) string {
	var parts []string
	used := 1
	for _, v := range slices.Backward(b.events) {
		part := eventText(v)
		w := cells(part)
		if len(parts) > 0 && used+3+w > width {
			break
		}
		if len(parts) == 0 {
			used += w
		} else {
			used += 3 + w
		}
		parts = append([]string{part}, parts...)
	}
	return " " + strings.Join(parts, "   ")
}

func (b *band) pendingLine(st *turns.State) string {
	if b.problem != "" {
		return " ! " + b.problem
	}
	for _, name := range sortedKeys(st.Asked) {
		q := st.Asked[name]
		return " " + "? asking " + name + " (" + strings.Join(q.Waiting, ", ") + " waiting)"
	}
	if len(st.Pending) > 0 {
		return " " + "boxed for: " + strings.Join(sortedKeys(st.Pending), ", ")
	}
	return ""
}

func eventText(ev bandEvent) string {
	who := ""
	switch {
	case ev.From != "" && ev.To != "":
		who = bandParty(ev.From) + "→" + bandParty(ev.To)
	case ev.From != "":
		who = bandParty(ev.From)
	default:
		who = bandParty(ev.To)
	}
	return ev.At.Format("15:04:05") + " " + who + " " + ev.Mark
}

func stateMark(status string) string {
	switch status {
	case "working":
		return "●"
	case "blocked":
		return "◐"
	case "idle", "done":
		return "○"
	}
	return "·"
}

// bandParty is a name as the band writes it. The band's own words (turn,
// waiting, human, asking) are English in every language: they are labels a
// person reads at a glance, and a translated label reads worse than the word
// they already know (d-1a54ca's follow-up, 2026-11-10).
func bandParty(name string) string { return name }

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

// cells is how many terminal cells a string takes: East Asian wide runes two,
// the rest one. Close enough for a band that is clipped, not aligned.
func cells(s string) int {
	n := 0
	for _, r := range s {
		n += runeCells(r)
	}
	return n
}

func runeCells(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x3000 && r <= 0x30FF: // CJK punctuation, hiragana, katakana (the prolonged sound mark among them)
		return 2
	case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hangul, r):
		return 2
	case r >= 0xFF00 && r <= 0xFF60, r >= 0xFFE0 && r <= 0xFFE6: // fullwidth forms
		return 2
	}
	return 1 // box drawing and the marks ● ○ ◐ ✓ ✗ ⟲: one cell in most terminals
}

// clip cuts a string to width cells.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	n := 0
	for i, r := range s {
		w := runeCells(r)
		if n+w > width {
			return s[:i]
		}
		n += w
	}
	return s
}

// terminalWidth is the width of the terminal the watcher runs in, or the default.
func terminalWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return bandDefaultWidth
	}
	return int(ws.Col)
}

// screen draws the band in place: it clears the pane once, then rewrites the
// same lines from the top on every draw, and never ends the last line, so the
// pane does not scroll.
type screen struct {
	out   io.Writer
	begun bool
}

func (s *screen) draw(lines []string) {
	if !s.begun {
		fmt.Fprint(s.out, "\x1b[?25l\x1b[2J")
		s.begun = true
	}
	var sb strings.Builder
	sb.WriteString("\x1b[H")
	for i, line := range lines {
		sb.WriteString(styled(line) + "\x1b[K")
		if i < len(lines)-1 {
			sb.WriteString("\r\n")
		}
	}
	fmt.Fprint(s.out, sb.String())
}

// end shows the cursor again when the watcher stops.
func (s *screen) end() {
	if s.begun {
		fmt.Fprint(s.out, "\x1b[?25h\r\n")
	}
}

// styled colours the marks and dims the separators; the text itself is left
// as it is, so the plain line is what render measured.
func styled(line string) string {
	r := strings.NewReplacer(
		"●", "\x1b[32m●\x1b[0m",
		"◐", "\x1b[33m◐\x1b[0m",
		"│", "\x1b[2m│\x1b[0m",
		" ? ", " \x1b[1;33m?\x1b[0m ",
		" ✗", " \x1b[31m✗\x1b[0m",
		" ! ", " \x1b[31m!\x1b[0m ",
	)
	return r.Replace(line)
}

// stampWriter writes the watcher's sentences to the log with the time each
// line began.
type stampWriter struct {
	out     io.Writer
	clock   func() time.Time
	midline bool
}

func (s *stampWriter) Write(p []byte) (int, error) {
	text := string(p)
	var sb strings.Builder
	for len(text) > 0 {
		if !s.midline {
			sb.WriteString(s.clock().Format("15:04:05") + " ")
			s.midline = true
		}
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			sb.WriteString(text)
			break
		}
		sb.WriteString(text[:i+1])
		s.midline = false
		text = text[i+1:]
	}
	if _, err := io.WriteString(s.out, sb.String()); err != nil {
		return 0, err
	}
	return len(p), nil
}
