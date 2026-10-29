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
	for _, want := range []string{`neither a running team nor a worker`, "next: status"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("down nowhere does not say %q:\n%s", want, stderr)
		}
	}
}

// workerHerdr writes a fake herdr with the team draft running and two workers
// hired into tabs of their own. withBenchTeam adds a running team named bench, so
// a test can make one name a team's and a worker's at once. It records every call
// in <cfg>/herdr.log and returns the log.
func (f *fixture) workerHerdr(t *testing.T, withBenchTeam bool) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	tabs := `{"tab_id":"t1","label":"draft"},{"tab_id":"t2","label":"worker:draft/bench"},{"tab_id":"t3","label":"worker:draft/scout"}`
	agents := `{"name":"draft-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"},{"name":"draft-bench","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2"},{"name":"draft-scout","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"t3"}`
	panes := `{"pane_id":"p1","label":"draft-lead","tab_id":"t1"},{"pane_id":"p2","label":"draft-bench","tab_id":"t2"},{"pane_id":"p3","label":"draft-scout","tab_id":"t3"}`
	if withBenchTeam {
		tabs += `,{"tab_id":"t4","label":"bench"}`
		agents += `,{"name":"bench-lead","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t4"}`
		panes += `,{"pane_id":"p4","label":"bench-lead","tab_id":"t4"}`
	}
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
  "agent list") echo '{"result":{"agents":[`+agents+`]}}' ;;
  "tab list")   echo '{"result":{"tabs":[`+tabs+`]}}' ;;
  "pane list")  echo '{"result":{"panes":[`+panes+`]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// TestDownClosesOneWorkerAndLeavesTheTeamAndTheOtherWorker: hergy down <worker>
// closes the worker's pane (and its empty tab) and nothing else; the member and
// the other worker stay (r-27ca, d-fa05). The worker is found by its short name
// and by its herdr name.
func TestDownClosesOneWorkerAndLeavesTheTeamAndTheOtherWorker(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	log := f.workerHerdr(t, false)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	want := "ワーカー: draft-bench\n" +
		"  チーム: draft\n" +
		"draft-bench: 閉じます（ペイン p2、ワーカー）\n" +
		"draft-bench: タブを閉じます（ほかにペインが無いため）\n" +
		"閉じた数: 1\n" +
		"ペインが無かった数: 0\n"
	if got := mustRun(t, "down", "bench", "--lang", "ja"); got != want {
		t.Fatalf("down bench printed:\n%s\nwant:\n%s", got, want)
	}
	closes := calls(t, log)
	if !strings.Contains(closes, "pane close p2") || !strings.Contains(closes, "tab close t2") {
		t.Fatalf("down bench did not close the worker's pane and tab:\n%s", closes)
	}
	for _, untouched := range []string{"pane close p1", "pane close p3", "tab close t1", "tab close t3"} {
		if strings.Contains(closes, untouched) {
			t.Fatalf("down bench ran %q, but it must close only bench:\n%s", untouched, closes)
		}
	}

	// The herdr name finds the worker too.
	if got := mustRun(t, "down", "draft-scout", "--lang", "ja"); !strings.Contains(got, "draft-scout: 閉じます（ペイン p3、ワーカー）") {
		t.Fatalf("down draft-scout printed:\n%s", got)
	}
	closes = calls(t, log)
	if !strings.Contains(closes, "pane close p3") || strings.Contains(closes, "pane close p1") {
		t.Fatalf("down draft-scout closed the wrong pane:\n%s", closes)
	}
}

// TestDownRefusesANameThatIsBothATeamAndAWorker: one name that is a running team
// and a worker of another is refused, naming both; the worker's herdr name still
// picks the worker (d-fa05).
func TestDownRefusesANameThatIsBothATeamAndAWorker(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	mustRun(t, "new", "bench")
	log := f.workerHerdr(t, true)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	_, stderr, code := run("down", "bench", "--lang", "ja")
	if code == 0 {
		t.Fatalf("down bench accepted a name that is a team and a worker")
	}
	for _, want := range []string{"チーム名にもワーカー名にも当たります", "チーム bench", "ワーカー draft-bench"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, stderr)
		}
	}
	if closes := calls(t, log); strings.Contains(closes, "pane close") {
		t.Fatalf("the refusal closed something:\n%s", closes)
	}

	// The herdr name names the worker alone.
	mustRun(t, "down", "draft-bench", "--lang", "ja")
	closes := calls(t, log)
	if !strings.Contains(closes, "pane close p2") || strings.Contains(closes, "pane close p4") {
		t.Fatalf("down draft-bench did not close only the worker:\n%s", closes)
	}
}

// TestDownDryOfAWorkerPlansTheWorkerAndItsTab: --dry names the worker's pane and
// the tab that would go with it, closes nothing, and names the command that would
// carry it out (r-27ca).
func TestDownDryOfAWorkerPlansTheWorkerAndItsTab(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	log := f.workerHerdr(t, false)

	want := "ワーカー: draft-bench\n" +
		"  チーム: draft\n" +
		"draft-bench: 閉じる予定（ペイン p2、ワーカー）\n" +
		"draft-bench: 閉じる予定のタブ（ほかにペインが無いため）\n" +
		"閉じる予定の数: 1\n" +
		"閉じない数: 0\n" +
		"next: down draft-bench\n"
	if got := mustRun(t, "down", "bench", "--dry", "--lang", "ja"); got != want {
		t.Fatalf("down bench --dry printed:\n%s\nwant:\n%s", got, want)
	}
	if closes := calls(t, log); strings.Contains(closes, "pane close") || strings.Contains(closes, "tab close") {
		t.Fatalf("a dry run closed something:\n%s", closes)
	}
}

// TestDownRefusesAWorkerNameTwoTeamsHave: a worker short name two running teams
// carry is not guessed at; the refusal names the herdr names that pick one
// (r-27ca, d-fa05).
func TestDownRefusesAWorkerNameTwoTeamsHave(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	mustRun(t, "new", "ops")
	f.herdr(t, `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"draft"},{"tab_id":"t2","label":"worker:draft/bench"},{"tab_id":"t3","label":"ops"},{"tab_id":"t4","label":"worker:ops/bench"}]}}' ;;
  "agent list") echo '{"result":{"agents":[{"name":"draft-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"},{"name":"draft-bench","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2"},{"name":"ops-lead","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"t3"},{"name":"ops-bench","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t4"}]}}' ;;
  "pane list") echo '{"result":{"panes":[{"pane_id":"p1","label":"draft-lead","tab_id":"t1"},{"pane_id":"p2","label":"draft-bench","tab_id":"t2"},{"pane_id":"p3","label":"ops-lead","tab_id":"t3"},{"pane_id":"p4","label":"ops-bench","tab_id":"t4"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	_, stderr, code := run("down", "bench", "--lang", "ja")
	if code == 0 {
		t.Fatalf("down bench guessed between two teams' workers")
	}
	for _, want := range []string{"複数のチームにいます", "draft-bench", "ops-bench"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, stderr)
		}
	}
}
