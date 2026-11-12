package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/team"
	// hqHerdr is a fake herdr with two workspaces and two teams, one in each. It
	// answers the calls the headquarters makes: the tabs with their workspace, the
	// panes hergy named, the agents, the workspaces, and the calls a raise makes (a
	// member with no pane is given one in the team's tab). Every call is written to
	// the fixture's herdr.log, so a test can read what a raise did.
)

const hqHerdr = `#!/bin/sh
log="${HERDR_TEST_LOG:-}"
[ -n "$log" ] && printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[
    {"tab_id":"t1","label":"core","workspace_id":"w3"},
    {"tab_id":"t2","label":"ledger","workspace_id":"w2"}]}}' ;;
  "agent list") echo '{"result":{"agents":[
    {"name":"core-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1","state_change_seq":2},
    {"name":"ledger-lead","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2","state_change_seq":1}]}}' ;;
  "agent get")
    case "$3" in
      core-lead|p1) echo '{"result":{"agent":{"name":"core-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1","state_change_seq":2}}}' ;;
      ledger-lead|p2) echo '{"result":{"agent":{"name":"ledger-lead","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2","state_change_seq":1}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac ;;
  "pane list") echo '{"result":{"panes":[
    {"pane_id":"p1","label":"core-lead","tab_id":"t1","workspace_id":"w3"},
    {"pane_id":"p2","label":"ledger-lead","tab_id":"t2","workspace_id":"w2"}]}}' ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p9"}}}' ;;
  "workspace list") echo '{"result":{"workspaces":[
    {"workspace_id":"w1","label":"proj-dev","focused":false},
    {"workspace_id":"w2","label":"ledger-dev","focused":false},
    {"workspace_id":"w3","label":"scratch","focused":false}]}}' ;;
  "workspace get") echo '{"result":{"workspace":{"workspace_id":"'"$3"'","label":"proj-dev"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// hqFixture is the fixture with another project listed beside the current one,
// and a launch record of the team in each.
func hqFixture(t *testing.T) (*fixture, team.Env, string) {
	t.Helper()
	f := newFixture(t)
	mustRun(t, "new", "core")
	other := filepath.Join(filepath.Dir(f.proj), "other")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(filepath.Join(f.proj, ".hergy"), filepath.Join(other, ".hergy")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(other, ".hergy", "teams", "core"), filepath.Join(other, ".hergy", "teams", "ledger")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\nprojects:\n  - "+f.proj+"\n  - "+other+"\n")
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	for _, one := range []struct{ teamName, project string }{
		{"core", f.proj}, {"ledger", other},
	} {
		if err := log.Append(launchlog.Record{
			Time: "2026-10-24T09:00:00+09:00", Command: launchlog.CommandUp,
			Project: domainplace.Observe(one.project), Layer: "local", Definition: one.teamName, Team: fixtureSeen(one.teamName),
			TeamYAML:    domainplace.Observe(filepath.Join(one.project, ".hergy", "teams", one.teamName, "team.yaml")),
			Workspace:   map[string]string{f.proj: "proj-dev", other: "ledger-dev"}[one.project],
			WorkspaceID: map[string]string{f.proj: "w1", other: "w2"}[one.project],
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.herdr(t, hqHerdr)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return f, env, other
}

// stateMatches says the state the activity judged is one the running members can
// show: a team with a blocked member is answer, and one with no working member is
// stalled.

// catGlobalDefs is how many team definitions the global layer holds, for a test
// that does not want to write the number down.

// copyTree lays one tree over another, for a test that needs a second project.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

// stateHerdr is a fake herdr whose answers are files a test may rewrite, so a
// test can make a team appear and disappear the way another serve's stop does.
// A pane close empties the tabs, panes and agents: what herdr reports after a
// team is put away.

// writeHerdrState writes the files the stateful fake answers: one team, or
// nothing.

// writeState writes the stateful fake's files for one team whose panes stand in
// one directory.

// teamNamed is one team of a running reading, by name.
