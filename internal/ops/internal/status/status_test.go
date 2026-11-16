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

// TestRowText: one row prints its live state, "not running" when no agent is
// live, and the worker mark on a hired worker.
func TestRowText(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{"live", Row{Name: "a", State: "working", Live: true}, "working"},
		{"idle", Row{Name: "a", State: "idle", Live: true}, "idle"},
		{"absent", Row{Name: "a"}, "not running"},
		{"blocked", Row{Name: "a", State: "blocked", Live: true}, "blocked (waiting for permission)"},
		{"working unchanged", Row{Name: "a", State: "working", Live: true, UnchangedMinutes: new(7)}, "working (unchanged for 7 min)"},
		{"live worker", Row{Name: "w", State: "blocked", Live: true, Worker: true}, "blocked (waiting for permission) (worker)"},
		{"absent worker", Row{Name: "w", Worker: true}, "not running (worker)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.row.Text(); got != c.want {
				t.Fatalf("Text() = %q, want %q", got, c.want)
			}
		})
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

// TestWordsFollowTheLanguage: row wording follows the selected locale.
func TestWordsFollowTheLanguage(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	if got, want := (Row{Name: "b"}).Text(), "動いていない"; got != want {
		t.Fatalf("Japanese absent text = %q, want %q", got, want)
	}
	if got, want := (Row{Name: "a", State: "blocked", Live: true}).Text(), "blocked（許可待ち）"; got != want {
		t.Fatalf("Japanese blocked text = %q, want %q", got, want)
	}
	if got, want := (Row{Name: "a", State: "working", Live: true, UnchangedMinutes: new(9)}).Text(), "仕事中（9 分変化なし）"; got != want {
		t.Fatalf("Japanese unchanged text = %q, want %q", got, want)
	}
}
