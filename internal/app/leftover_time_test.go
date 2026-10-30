package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/launchlog"
)

// recordStart writes one up line of the launch record for a team in the
// fixture's project, at the time a test says, the way up writes it after a boot.
func recordStart(t *testing.T, f *fixture, teamName string, at time.Time) {
	t.Helper()
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	if err := log.Append(launchlog.Record{
		Time: at.Format(time.RFC3339), Command: launchlog.CommandUp,
		Project: f.proj, Team: teamName, Definition: teamName,
	}); err != nil {
		t.Fatal(err)
	}
}

// recordHireLine writes one hire line for a worker of a team in the fixture's
// project, at the time a test says, the way hire writes it.
func recordHireLine(t *testing.T, f *fixture, teamName, worker string, at time.Time) {
	t.Helper()
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	if err := log.Append(launchlog.Record{
		Time: at.Format(time.RFC3339), Command: launchlog.CommandHire,
		Project: f.proj, Team: teamName, Definition: teamName,
		Hire: &launchlog.Hire{Worker: worker},
	}); err != nil {
		t.Fatal(err)
	}
}

// lane adds a worktree of the fixture's project on its own branch, made at the
// time a test says, and returns its path. git writes the time into
// .git/worktrees/<name>/gitdir when it makes the worktree, and hergy reads that
// file (d-970c): a test puts it back to lay out a worktree that was there before
// a time.
func (f *fixture) lane(t *testing.T, name string, made time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	gitIn(t, f.proj, "worktree", "add", "-q", "-b", name, path, "main")
	if !made.IsZero() {
		at := filepath.Join(f.proj, ".git", "worktrees", name, "gitdir")
		if err := os.Chtimes(at, made, made); err != nil {
			t.Fatal(err)
		}
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

// gitBase lays out the one commit of the fixture's project that a worktree is
// branched from, on main.
func (f *fixture) gitBase(t *testing.T) {
	t.Helper()
	gitIn(t, f.proj, "config", "user.email", "test@example.com")
	gitIn(t, f.proj, "config", "user.name", "test")
	writeFile(t, filepath.Join(f.proj, "base.txt"), "base\n")
	gitIn(t, f.proj, "add", "base.txt")
	gitIn(t, f.proj, "commit", "-q", "-m", "base")
	gitIn(t, f.proj, "branch", "-M", "main")
}

// TestDownListsTheWorktreesMadeWhileTheTeamRan: an agent keeps its pane in the
// project root and cds into a worktree for each command, so hergy down cannot
// find the team's worktrees by where the panes stand. It reads when the team was
// started from the launch record instead, and lists the worktrees of the project
// that came after the start, leaving out the one that was there before it
// (d-970c).
func TestDownListsTheWorktreesMadeWhileTheTeamRan(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	f.gitBase(t)
	started := time.Now().Add(-time.Hour)
	theirs := f.lane(t, "theirs", started.Add(-time.Hour))
	mine := f.lane(t, "mine", started.Add(time.Minute))
	recordStart(t, f, "draft", started)
	// Every pane stays in the project root: the agent cds into the worktree.
	f.leftoversHerdr(t, f.proj)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	got := mustRun(t, "down", "draft", "--lang", "ja")
	if !strings.Contains(got, "draft が動いている間にできたワークツリー（git）:") {
		t.Errorf("down does not say when the worktrees were made:\n%s", got)
	}
	if !strings.Contains(got, "ワークツリー: "+mine) {
		t.Errorf("down does not list the worktree made while the team ran:\n%s", got)
	}
	if strings.Contains(got, theirs) {
		t.Errorf("down lists the worktree that was there before the team started:\n%s", got)
	}
	// The panes stayed in the project root, so no worktree carries their mark.
	if strings.Contains(got, "（ペインの作業ディレクトリがここにあった）") {
		t.Errorf("down marks a worktree no pane stood in:\n%s", got)
	}

	// --dry plans the same list without closing anything.
	got = mustRun(t, "down", "draft", "--dry", "--lang", "ja")
	if !strings.Contains(got, "draft が動いている間にできたワークツリー（git、閉じたあとに残る予定）:") ||
		!strings.Contains(got, "ワークツリー: "+mine) || strings.Contains(got, theirs) {
		t.Errorf("down --dry does not plan the same list:\n%s", got)
	}
}

// TestDownALoneWorkerListsTheWorktreesMadeAfterItWasHired: closing one worker
// lists the worktrees that came after that worker was hired, from its hire line
// in the launch record — not the ones that came after the team was started, and
// not the ones that were there before the hire (d-970c).
func TestDownALoneWorkerListsTheWorktreesMadeAfterItWasHired(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "draft")
	f.gitBase(t)
	hired := time.Now().Add(-time.Hour)
	beforeHire := f.lane(t, "before-hire", hired.Add(-time.Hour))
	afterHire := f.lane(t, "after-hire", hired.Add(time.Minute))
	recordStart(t, f, "draft", hired.Add(-2*time.Hour))
	recordHireLine(t, f, "draft", "draft-bench", hired)
	// Every pane stays in the project root, and the worker has a tab of its own.
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
  "agent list") echo '{"result":{"agents":[{"name":"draft-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"},{"name":"draft-bench","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"draft"},{"tab_id":"t2","label":"worker:draft/bench"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"draft-lead","tab_id":"t1","cwd":"`+f.proj+`"},{"pane_id":"p2","label":"draft-bench","tab_id":"t2","cwd":"`+f.proj+`"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	got := mustRun(t, "down", "bench", "--lang", "ja")
	if !strings.Contains(got, "draft-bench を雇ったあとにできたワークツリー（git）:") {
		t.Errorf("down does not say when the worktrees were made:\n%s", got)
	}
	if !strings.Contains(got, "ワークツリー: "+afterHire) {
		t.Errorf("down does not list the worktree made after the worker was hired:\n%s", got)
	}
	if strings.Contains(got, beforeHire) {
		t.Errorf("down lists the worktree made before the worker was hired:\n%s", got)
	}
}
