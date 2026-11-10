package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/turns"
)

// TestBandRendersStateHandoffsAndWhatIsAsked: the three lines, whose labels
// are English even in a Japanese run,
// from a state in which lead told builder, builder stopped unhanded and is
// being asked (r-4e04, ac-3d07).
func TestBandRendersStateHandoffsAndWhatIsAsked(t *testing.T) {
	inJapanese(t)
	b := newBand("demo")
	b.seen([]turns.Agent{{Name: "lead", Status: "idle"}, {Name: "builder", Status: "working"}})
	at := time.Date(2026, 11, 9, 21, 28, 47, 0, time.Local)
	b.note(turns.Action{Kind: turns.Told, From: "lead", To: "builder"}, at)
	b.note(turns.Action{Kind: turns.Ask, To: "builder", Waiting: []string{"lead"}}, at.Add(24*time.Second))
	st := &turns.State{
		Team: "demo", Holder: "builder", From: "lead",
		Opened: map[string][]turns.Opening{"builder": {{By: "lead", At: at}}},
		Asked:  map[string]turns.Question{"builder": {Waiting: []string{"lead"}}},
	}
	lines := b.render(st, 120)
	if len(lines) != bandLines {
		t.Fatalf("render = %d lines, want %d", len(lines), bandLines)
	}
	for _, want := range []string{" demo  ", " lead ○ ", " builder ● ", "│ turn: builder (from lead)", "│ waiting: lead → builder"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line 1 lacks %q: %q", want, lines[0])
		}
	}
	if want := " 21:28:47 lead→builder →   21:29:11 builder ?"; lines[1] != want {
		t.Errorf("line 2 = %q, want %q", lines[1], want)
	}
	if want := " ? asking builder (lead waiting)"; lines[2] != want {
		t.Errorf("line 3 = %q, want %q", lines[2], want)
	}
	// The human holds it, nothing asked: the third line is empty.
	rest := &turns.State{Team: "demo", Holder: turns.Human, From: "builder"}
	b.note(turns.Action{Kind: turns.ToHuman, From: "builder"}, at.Add(time.Minute))
	lines = b.render(rest, 120)
	if !strings.Contains(lines[0], "turn: human (from builder)") || lines[2] != "" {
		t.Errorf("render = %q, want the human holding it from builder and an empty third line", lines)
	}
	if !strings.HasSuffix(lines[1], "21:29:47 builder→human ✓") {
		t.Errorf("line 2 = %q, want it to end with the handoff to the human", lines[1])
	}
}

// TestBandKeepsTheNewestHandoffsThatFit: the second line drops the oldest
// handoffs first, and always shows the newest; every line is clipped to the
// width in cells (Japanese counts two).
func TestBandKeepsTheNewestHandoffsThatFit(t *testing.T) {
	inJapanese(t)
	b := newBand("demo")
	at := time.Date(2026, 11, 9, 9, 0, 0, 0, time.Local)
	for i := range 5 {
		b.note(turns.Action{Kind: turns.Told, From: "lead", To: "dev"}, at.Add(time.Duration(i)*time.Minute))
	}
	st := &turns.State{Team: "demo", Holder: turns.Human}
	one := " 09:04:00 lead→dev →"
	if got := b.render(st, 30)[1]; got != one {
		t.Errorf("at width 30 line 2 = %q, want the newest alone %q", got, one)
	}
	if got := b.render(st, 50)[1]; got != " 09:03:00 lead→dev →   09:04:00 lead→dev →" {
		t.Errorf("at width 50 line 2 = %q, want the newest two", got)
	}
	if got := b.render(st, 8)[0]; got != " demo   " {
		t.Errorf("at width 8 line 1 = %q, want it clipped to the team", got)
	}
	if cells("ターン: 人間") != 12 || cells(" lead ○ ") != 8 {
		t.Errorf("cells counts wrong: %d, %d", cells("ターン: 人間"), cells(" lead ○ "))
	}
}

// TestStampWriterDatesEachLine: the log gets the time a line began, once per
// line, whatever the writes were cut into.
func TestStampWriterDatesEachLine(t *testing.T) {
	var out bytes.Buffer
	at := time.Date(2026, 11, 9, 21, 29, 11, 0, time.Local)
	w := &stampWriter{out: &out, clock: func() time.Time { return at }}
	_, _ = w.Write([]byte("dev → "))
	at = at.Add(time.Second)
	_, _ = w.Write([]byte("人間: 渡りました\nlead は"))
	at = at.Add(time.Second)
	_, _ = w.Write([]byte("止まりました\n"))
	if want := "21:29:11 dev → 人間: 渡りました\n21:29:12 lead は止まりました\n"; out.String() != want {
		t.Errorf("log = %q, want %q", out.String(), want)
	}
}

// TestScreenDrawsInPlace: the first draw clears the pane and hides the cursor;
// every draw starts at the top, clears each line's tail, and does not end the
// last line, so the pane never scrolls; end shows the cursor again.
func TestScreenDrawsInPlace(t *testing.T) {
	var out bytes.Buffer
	s := &screen{out: &out}
	s.draw([]string{"a", "b", "c"})
	if got := out.String(); got != "\x1b[?25l\x1b[2J\x1b[Ha\x1b[K\r\nb\x1b[K\r\nc\x1b[K" {
		t.Errorf("first draw = %q", got)
	}
	out.Reset()
	s.draw([]string{"x lead ●", "", ""})
	if got := out.String(); strings.Contains(got, "\x1b[2J") || !strings.HasPrefix(got, "\x1b[H") || strings.HasSuffix(got, "\n") || !strings.Contains(got, "\x1b[32m●\x1b[0m") {
		t.Errorf("second draw = %q, want no clear, from the top, the mark coloured, no final newline", got)
	}
	s.end()
	if !strings.HasSuffix(out.String(), "\x1b[?25h\r\n") {
		t.Errorf("end = %q, want the cursor shown", out.String())
	}
}

// TestPrintTurnsLogPrintsTheFileOrSaysThereIsNone covers hergy turns --log.
func TestPrintTurnsLogPrintsTheFileOrSaysThereIsNone(t *testing.T) {
	inJapanese(t)
	path := t.TempDir() + "/demo.log"
	var out bytes.Buffer
	if err := printTurnsLog("demo", path, &out); err != nil || !strings.Contains(out.String(), "見張りのログはまだありません") {
		t.Fatalf("missing log: %q, %v", out.String(), err)
	}
	out.Reset()
	f, err := openTurnsLog(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("21:29:11 dev → 人間\n")
	_ = f.Close()
	if err := printTurnsLog("demo", path, &out); err != nil || out.String() != "21:29:11 dev → 人間\n" {
		t.Fatalf("log = %q, %v", out.String(), err)
	}
}
