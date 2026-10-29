package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/web"
)

// withThirdMember gives demo a third member, writer, so a start that fails on
// the third member has two that got through before it.
func withThirdMember(t *testing.T, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	text := readFile(t, path)
	text = strings.Replace(text,
		"  - name: builder\n    role: builder\n    profile: claude-default\n",
		"  - name: builder\n    role: builder\n    profile: claude-default\n  - name: writer\n    role: builder\n    profile: claude-default\n", 1)
	text = strings.Replace(text, "  - lead -> builder\n", "  - lead -> builder\n  - lead -> writer\n", 1)
	text = strings.Replace(text, "  - builder -> lead\n", "  - builder -> lead\n  - writer -> lead\n", 1)
	writeFile(t, path, text)
}

// failingThirdHerdr writes a fake herdr whose third start, demo-writer's, is
// refused with herdr's own message. The panes it makes are numbered, so the
// count a failure reports is the count herdr was given.
func failingThirdHerdr(t *testing.T, f *fixture) {
	t.Helper()
	count := filepath.Join(f.cfg, "panes.txt")
	script := `#!/bin/sh
count='` + count + `'
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "workspace list") echo '{"result":{"workspaces":[{"workspace_id":"w1","label":"main","focused":true}]}}' ;;
  "workspace get") echo '{"result":{"workspace":{"workspace_id":"w1","label":"main"}}}' ;;
  "agent list") echo '{"result":{"agents":[]}}' ;;
  "tab list") echo '{"result":{"tabs":[]}}' ;;
  "pane list") echo '{"result":{"panes":[]}}' ;;
  "tab create") echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")
    n=$(cat "$count" 2>/dev/null || echo 1)
    n=$((n + 1))
    echo "$n" > "$count"
    printf '{"result":{"pane":{"pane_id":"p%s"}}}\n' "$n" ;;
  "pane rename") echo '{"result":{}}' ;;
  "agent start")
    if [ "$3" = "demo-writer" ]; then
      echo '{"error":{"code":"profile_unknown","message":"the profile claude-default is not installed"}}'
      exit 1
    fi
    echo '{"result":{}}' ;;
  "agent prompt") echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
}

// failedStartWants is what the screen and the command line both say about the
// start that stopped on demo-writer: the member, herdr's reason in words, who
// started, what remains, and the command that goes on, in the project.
var failedStartWants = []string{
	"demo-writer の起動が失敗しました: the profile claude-default is not installed",
	"起動できたメンバー: demo-lead、demo-builder",
	"残っているもの: タブ demo、ペイン 3 個",
	"このチームは動いている扱いです",
	"proj で hergy down demo のあと hergy up demo を実行してください",
}

func wantFailedStartText(t *testing.T, got string) {
	t.Helper()
	for _, want := range failedStartWants {
		if !strings.Contains(got, want) {
			t.Errorf("the failure text does not say %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"error"`) || strings.Contains(got, `"code"`) {
		t.Errorf("the failure text shows herdr's JSON:\n%s", got)
	}
}

// TestALaunchThatFailsPartwayNamesTheMemberAndWhatIsLeft: the screen's start
// answers with the member herdr refused, herdr's reason in words, the members
// that started, the tab and panes left, and the command to go on with, in the
// project (d-5f02, ac-713d).
func TestALaunchThatFailsPartwayNamesTheMemberAndWhatIsLeft(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	useJapanese(t)
	withThirdMember(t, f)
	failingThirdHerdr(t, f)

	env := serveEnv(t)
	read := newActivityRead(env)
	launch, err := read.StartLaunch(web.HQLaunchIn{
		Project: env.ProjectRoot, Definition: "demo", Place: "project",
		Workspace: web.HQWorkspaceIn{ID: "w1"},
	})
	if err != nil {
		t.Fatalf("StartLaunch: %v", err)
	}
	last := waitLaunch(t, read, launch.ID)
	if last.Phase != "failed" {
		t.Fatalf("the launch = %+v, want failed", last)
	}
	wantFailedStartText(t, last.Error)
}

// TestUpThatFailsPartwaySaysTheSameThing: hergy up's failure names the same
// member, reason, remains and way on as the screen's start (d-5f02).
func TestUpThatFailsPartwaySaysTheSameThing(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--lang", "ja")
	useJapanese(t)
	withThirdMember(t, f)
	failingThirdHerdr(t, f)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	_, stderr, code := run("up", "demo")
	if code == 0 {
		t.Fatalf("up did not fail:\n%s", stderr)
	}
	wantFailedStartText(t, stderr)
}
