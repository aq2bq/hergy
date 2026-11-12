package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/turns"
)

// ac-92c2: a live hired worker and its hirer tell one another in their own
// names; a pane outside the roster still speaks as the human.
func TestTellResolvesWorkersAndHumanPanes(t *testing.T) {
	for _, tc := range []struct{ name, pane, to, from, target string }{
		{"worker to hirer", "pw", "lead", "demo-scroll", "demo-lead"},
		{"hirer to worker", "p1", "scroll", "demo-lead", "demo-scroll"},
		{"human to member", "outside", "lead", "human", "demo-lead"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "demo")
			_, prompts := f.controlHerdr(t, "demo-lead idle p1 screen\ndemo-builder idle p2 screen\ndemo-scroll idle pw screen\n")
			fake := os.Getenv("HERGY_HERDR")
			script := strings.Replace(readFile(t, fake), `{"tab_id":"t1","label":"team:demo"}`, `{"tab_id":"t1","label":"team:demo"},{"tab_id":"tw","label":"worker:demo/scroll"}`, 1)
			f.herdr(t, script)
			log, err := launchlog.Open()
			if err != nil {
				t.Fatal(err)
			}
			err = log.Append(launchlog.Record{Command: launchlog.CommandHire, Team: fixtureSeen("demo"), Hire: &launchlog.Hire{Worker: fixtureSeen("demo-scroll"), From: "lead", Tab: fixtureSeen("worker:demo/scroll")}})
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("HERDR_PANE_ID", tc.pane)
			mustRun(t, "tell", tc.to, "answer", "--team", "demo")
			sent := readFile(t, prompts)
			sender := tc.from
			if sender == "human" {
				sender = "the human"
			}
			if !strings.Contains(sent, tc.target+"\tFrom "+sender+": answer") {
				t.Fatalf("sent=%q; want %s from %s", sent, tc.target, tc.from)
			}

			tells, err := turns.TakeTells(turns.TellDir(filepath.Join(f.state, "hergy"), "demo"))
			if err != nil || len(tells) != 1 {
				t.Fatalf("tells=%+v %v", tells, err)
			}
			from := strings.TrimPrefix(tc.from, "demo-")
			if from == "human" {
				from = ""
			}
			if tells[0].From != from || tells[0].To != tc.to {
				t.Fatalf("tells=%+v", tells)
			}
		})
	}
}
