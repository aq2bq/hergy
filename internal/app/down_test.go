package app

import (
	"path/filepath"
	"strings"
	"testing"
)

// stoppedTeamHerdr writes a fake herdr of a team that stopped before any agent
// ran: the tab hergy labelled with the team's name, the panes it named after the
// members, one pane a person opened (no name), and another team's tab and pane.
// No agent is live. It records every call in <cfg>/herdr.log and returns the log.
func (f *fixture) stoppedTeamHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	f.herdr(t, `#!/bin/sh
log='`+log+`'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"turnwalk"},{"tab_id":"t2","label":"other"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"turnwalk-lead","tab_id":"t1"},{"pane_id":"p2","label":"turnwalk-builder","tab_id":"t1"},{"pane_id":"p3","tab_id":"t1"},{"pane_id":"p9","label":"other-lead","tab_id":"t2"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// TestDownClosesATeamThatStoppedBeforeAnyAgentRan: a boot cut short at a startup
// question leaves hergy's tab and panes with no agent in them, so the team is not
// a running team. hergy down <team> still puts it away, by the names hergy gave:
// the panes named after the team are closed, and the pane a person opened is not
// (d-da92, ac-1b17). A pane that stands with no agent in it is told apart from a
// member that has no pane at all, in both languages.
func TestDownClosesATeamThatStoppedBeforeAnyAgentRan(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{
			lang: "ja",
			want: "チーム: turnwalk\n" +
				"turnwalk-lead: 閉じます（ペイン p1、エージェントは動いていない）\n" +
				"turnwalk-builder: 閉じます（ペイン p2、エージェントは動いていない）\n" +
				"閉じた数: 2\n" +
				"ペインが無かった数: 0\n",
		},
		{
			lang: "en",
			want: "team: turnwalk\n" +
				"turnwalk-lead: closing (pane p1, no agent running)\n" +
				"turnwalk-builder: closing (pane p2, no agent running)\n" +
				"closed: 2\n" +
				"absent: 0\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.lang, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "turnwalk")
			log := f.stoppedTeamHerdr(t)
			t.Setenv("HERDR_ENV", "1")
			t.Setenv("HERDR_PANE_ID", "p9")

			if got := mustRun(t, "down", "turnwalk", "--lang", tc.lang); got != tc.want {
				t.Fatalf("down turnwalk (--lang %s) printed:\n%s\nwant:\n%s", tc.lang, got, tc.want)
			}
			closes := calls(t, log)
			if !strings.Contains(closes, "pane close p1") || !strings.Contains(closes, "pane close p2") {
				t.Fatalf("down did not close the panes hergy named:\n%s", closes)
			}
			if strings.Contains(closes, "pane close p3") {
				t.Fatalf("down closed a pane hergy did not name:\n%s", closes)
			}
			if strings.Contains(closes, "pane close p9") {
				t.Fatalf("down closed another team's pane:\n%s", closes)
			}
		})
	}
}

// TestDownDryOfAStoppedTeamPlansTheSame: --dry names the panes it would close,
// says which of them have no agent in them, and closes nothing.
func TestDownDryOfAStoppedTeamPlansTheSame(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "turnwalk")
	log := f.stoppedTeamHerdr(t)

	want := "チーム: turnwalk\n" +
		"turnwalk-lead: 閉じる予定（ペイン p1、エージェントは動いていない）\n" +
		"turnwalk-builder: 閉じる予定（ペイン p2、エージェントは動いていない）\n" +
		"閉じる予定の数: 2\n" +
		"閉じない数: 0\n" +
		"next: down turnwalk\n"
	if got := mustRun(t, "down", "turnwalk", "--dry", "--lang", "ja"); got != want {
		t.Fatalf("down turnwalk --dry printed:\n%s\nwant:\n%s", got, want)
	}
	if closes := calls(t, log); strings.Contains(closes, "pane close") {
		t.Fatalf("a dry run closed panes:\n%s", closes)
	}
}

// TestDownOfATeamWithNoTabKeepsTheStop: a name no tab carries is not a stopped
// team: down keeps the stop that says there is no running team of that name.
func TestDownOfATeamWithNoTabKeepsTheStop(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "turnwalk")
	f.stoppedTeamHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	stdout, stderr, code := run("down", "nowhere")
	if code == 0 {
		t.Fatalf("down nowhere succeeded:\n%s", stdout)
	}
	for _, want := range []string{`no running team "nowhere"`, "next: status"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("down nowhere does not say %q:\n%s", want, stderr)
		}
	}
}
