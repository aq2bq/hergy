package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/team" // serveEnv is the environment the screens read, built the way Run builds it.
	// fixture is one temporary project with a global config directory outside the
	// home directory, so paths inside the project go relative and paths under the
	// config directory stay absolute.
)

func serveEnv(t *testing.T) team.Env {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

type fixture struct {
	proj  string
	cfg   string
	state string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{proj: filepath.Join(root, "proj"), cfg: filepath.Join(root, "cfg")}
	f.state = filepath.Join(root, "state")
	for _, dir := range []string{f.proj, f.cfg} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", f.proj, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Chdir(f.proj)
	t.Setenv("XDG_CONFIG_HOME", f.cfg)

	t.Setenv("XDG_STATE_HOME", f.state)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("HERDR_ENV", "")
	t.Setenv("HERDR_PANE_ID", "")
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-herdr"))

	t.Setenv("HERGY_GY", filepath.Join(f.cfg, "no-gy"))

	t.Setenv("CODEX_HOME", "")
	return f
}

// herdr writes a fake herdr and points HERGY_HERDR at it.
func (f *fixture) herdr(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(f.cfg, "herdr")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_HERDR", path)
}

// statefulHerdr writes a fake herdr that remembers the agents it started and the
// tabs it created, so a second boot under the same names collides in preflight
// the way real herdr would, and a team command can find the team's tab. It
// records every call it gets in <cfg>/herdr.log and returns that path, so a test
// can check a tab label or a closed pane.
func (f *fixture) statefulHerdr(t *testing.T) string {
	t.Helper()
	state := filepath.Join(f.cfg, "agents.txt")
	tabs := filepath.Join(f.cfg, "tabs.txt")
	log := filepath.Join(f.cfg, "herdr.log")
	for _, path := range []string{state, tabs} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
state='` + state + `'
tabs='` + tabs + `'
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "workspace list") echo '{"result":{"workspaces":[{"workspace_id":"w1","label":"workspace"}]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r tab n; do
      [ -z "$n" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"idle","pane_id":"p-%s","tab_id":"%s"}' "$n" "$n" "$tab"
    done < "$state"
    printf ']}}'
    ;;
  "agent start")
    name="$3"
    if grep -q " $name$" "$state"; then
      echo '{"error":{"code":"name_in_use"}}'
      exit 1
    fi
    tab=$(tail -n 1 "$tabs" | cut -d' ' -f1)
    printf '%s %s\n' "$tab" "$name" >> "$state"
    echo '{"result":{}}'
    ;;
  "agent get")
    name="$3"
    case "$3" in
      p-*) name="${3#p-}" ;;
    esac
    tab=$(awk -v n="$name" '$2 == n { print $1; exit }' "$state")
    if [ -n "$tab" ]; then
      printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"idle","pane_id":"p-%s","tab_id":"%s"}}}\n' "$name" "$name" "$tab"
    else
      echo '{"error":{"code":"agent_not_found"}}'
    fi
    ;;
  "tab list")
    printf '{"result":{"tabs":['
    first=1
    while read -r id label; do
      [ -z "$id" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"tab_id":"%s","label":"%s","workspace_id":"w1"}' "$id" "$label"
    done < "$tabs"
    printf ']}}'
    ;;
  "pane close")
    name="${3#p-}"
    awk -v n="$name" '$2 != n' "$state" > "$state.tmp"; mv "$state.tmp" "$state"
    echo '{"result":{}}' ;;
  "pane list") echo '{"result":{"panes":[]}}' ;;
  "tab create")
    label=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = "--label" ]; then label="$a"; fi
      prev="$a"
    done
    n=$(wc -l < "$tabs" | tr -d ' ')
    n=$((n + 1))
    printf 't%s %s\n' "$n" "$label" >> "$tabs"
    printf '{"result":{"tab":{"tab_id":"t%s"},"root_pane":{"pane_id":"p1"}}}' "$n"
    ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "pane get") echo '{"result":{"pane":{"pane_id":"p2","scroll":{"viewport_rows":20}}}}' ;;
  "pane process-info") echo '{"result":{"process_info":{"foreground_process_group_id":900,"shell_pid":900,"pane_id":"p1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// runningHerdr writes a fake herdr that reports the given team tabs and live
// agents, so a team command can find the team definition behind a team name.
// tabs and agents are JSON object fragments (no brackets). It also answers the
// calls a status or a down needs.
func (f *fixture) runningHerdr(t *testing.T, agents string) {
	tabs := `{"tab_id":"t1","label":"demo"}`
	t.Helper()
	script := `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list")     echo '{"result":{"tabs":[` + tabs + `]}}' ;;
  "agent list")   echo '{"result":{"agents":[` + agents + `]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
}

// readFile returns a file's contents, failing the test when it is missing.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeFile writes a file, creating its directories.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mustRun runs a command that is expected to succeed.
func mustRun(t *testing.T, argv ...string) string {
	t.Helper()
	stdout, stderr, code := run(argv...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstderr:\n%s", argv, code, stderr)
	}
	return stdout
}

// launchLog is the record the fixture's commands write, under the temporary root
// that XDG_STATE_HOME points at.
func launchLog(t *testing.T, f *fixture) (launchlog.Log, []launchlog.Record) {
	t.Helper()
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	records, err := log.Read()
	if err != nil {
		t.Fatal(err)
	}
	return log, records
}

// profilesYAML is one profiles.yaml with a used profile (fast, read by the dev
// definition's lead) and one nobody reads (slow), each with the args given.
func profilesYAML(fast, slow string) string {
	return "version: 1\nprofiles:\n" +
		"  fast:\n    kind: claude\n    args: [" + fast + "]\n" +
		"  slow:\n    kind: codex\n    args: [" + slow + "]\n"
}
func projectPath(raw string) domainplace.Project {
	p, err := domainplace.NewProject(raw)
	if err != nil {
		panic(err)
	}
	return p
}
func seenPaths(raw []string) []domainplace.Seen {
	out := make([]domainplace.Seen, len(raw))
	for i, path := range raw {
		out[i] = domainplace.Observe(path)
	}
	return out
}

// initLocales loads the locales for the tests that call the pieces of a run
// rather than Run itself: a team's built-in prompt template comes from the
// locale, so without it the members read no document and count no rule.
func initLocales(t *testing.T) {
	t.Helper()
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
}
