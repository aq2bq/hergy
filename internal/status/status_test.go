package status

import (
	"fmt"
	"os"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestMain loads the messages, so the lines a test reads are the ones a person
// sees.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestRowText: one row prints its live state, "not present" when no agent is
// live, and the worker mark on a hired worker.
func TestRowText(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{"live", Row{Name: "a", State: "working", Live: true}, "working"},
		{"idle", Row{Name: "a", State: "idle", Live: true}, "idle"},
		{"absent", Row{Name: "a"}, "not present"},
		{"blocked", Row{Name: "a", State: "blocked", Live: true}, "blocked (waiting for permission)"},
		{"working unchanged", Row{Name: "a", State: "working", Live: true, UnchangedMinutes: new(7)}, "working (unchanged for 7 min)"},
		{"live worker", Row{Name: "w", State: "blocked", Live: true, Worker: true}, "blocked (waiting for permission) (worker)"},
		{"absent worker", Row{Name: "w", Worker: true}, "not present (worker)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.row.Text(); got != c.want {
				t.Fatalf("Text() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestJudgeText: the judgement is the names waiting on the human, else that
// nobody is working, else nothing when the team is working with nobody blocked.
func TestJudgeText(t *testing.T) {
	cases := []struct {
		name string
		rows []Row
		want string
	}{
		{
			"waiting",
			[]Row{{Name: "a", State: "blocked", Live: true}, {Name: "b", State: "working", Live: true}},
			"waiting for a human: a",
		},
		{
			"waiting in order",
			[]Row{{Name: "b", State: "blocked", Live: true}, {Name: "a", State: "blocked", Live: true}},
			"waiting for a human: b, a",
		},
		{
			"nobody working",
			[]Row{{Name: "a", State: "idle", Live: true}, {Name: "b", Live: false}},
			"nobody is working",
		},
		{
			"working with nobody blocked",
			[]Row{{Name: "a", State: "working", Live: true}, {Name: "b", State: "done", Live: true}},
			"",
		},
		{
			"no rows",
			nil,
			"nobody is working",
		},
		{
			"a blocked member with no live agent does not count",
			[]Row{{Name: "a", State: "blocked", Live: false}, {Name: "b", State: "working", Live: true}},
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := Judge(c.rows)
			if got := v.Text(); got != c.want {
				t.Fatalf("Judge(%+v).Text() = %q, want %q", c.rows, got, c.want)
			}
		})
	}
}

// TestJudgeCountsWorkingAndWaiting: Judge reports both the waiting names and how
// many are working, so a caller can show more than the one line.
func TestJudgeCountsWorkingAndWaiting(t *testing.T) {
	rows := []Row{
		{Name: "a", State: "working", Live: true},
		{Name: "b", State: "working", Live: true},
		{Name: "c", State: "blocked", Live: true},
		{Name: "d", State: "idle", Live: true},
		{Name: "e", Live: false},
	}
	got := Judge(rows)
	if got.Working != 2 {
		t.Fatalf("Working = %d, want 2", got.Working)
	}
	if len(got.Waiting) != 1 || got.Waiting[0] != "c" {
		t.Fatalf("Waiting = %v, want [c]", got.Waiting)
	}
}

// TestCountsSeparatesMembersAndWorkers: workers are counted apart from members,
// live or not.
func TestCountsSeparatesMembersAndWorkers(t *testing.T) {
	rows := []Row{
		{Name: "a", Live: true},
		{Name: "b", Worker: true},
		{Name: "c", Live: true, Worker: true},
	}
	members, workers := Counts(rows)
	if members != 1 || workers != 2 {
		t.Fatalf("Counts = %d members, %d workers, want 1 and 2", members, workers)
	}
}

// TestWordsFollowTheLanguage: the words are the locales', so a Japanese run
// prints the Japanese judgement.
func TestWordsFollowTheLanguage(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	rows := []Row{{Name: "a", State: "blocked", Live: true}}
	if got, want := Judge(rows).Text(), "人間の返事待ち: a"; got != want {
		t.Fatalf("Japanese judgement = %q, want %q", got, want)
	}
	if got, want := (Row{Name: "b"}).Text(), "動いていない"; got != want {
		t.Fatalf("Japanese absent text = %q, want %q", got, want)
	}
	if got, want := (Row{Name: "a", State: "blocked", Live: true}).Text(), "blocked（許可待ち）"; got != want {
		t.Fatalf("Japanese blocked text = %q, want %q", got, want)
	}
	if got, want := (Row{Name: "a", State: "working", Live: true, UnchangedMinutes: new(9)}).Text(), "作業中（9 分変化なし）"; got != want {
		t.Fatalf("Japanese unchanged text = %q, want %q", got, want)
	}
}
