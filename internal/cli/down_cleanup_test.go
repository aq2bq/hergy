package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/options"
)

// Each route receives the same close/get answers. The zero test limit makes
// a retained name time out on its first reading, without sleeping or a herdr.
func TestTeamDownCleanupAfterNameWait(t *testing.T) {
	for _, path := range []string{"Down", "DownByName", "DownStopped"} {
		for _, outcome := range []string{"success", "timeout", "timeout-own", "close-error", "read-error"} {
			t.Run(path+"/"+outcome, func(t *testing.T) {
				f := newFixture(t)
				mustRun(t, "new", "wave")
				useJapanese(t)
				f.gitBase(t)
				at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				recordStart(t, f, "wave", at)
				worktree := f.lane(t, "left", at.Add(time.Hour))
				log := f.stopHerdr(t, false)
				if path == "DownByName" {
					writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "wave", "team.yaml"), "invalid: [")
				}
				if path == "DownStopped" {
					writeFile(t, filepath.Join(f.cfg, "stop-herdr", "agents.json"), `{"result":{"agents":[]}}`)
				}
				script := readFile(t, os.Getenv("HERGY_HERDR"))
				answer := `echo '{"error":{"code":"agent_not_found"}}'`
				if strings.HasPrefix(outcome, "timeout") {
					answer = `printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"done","pane_id":"p1","tab_id":"t1"}}}' "$3"`
				} else if outcome == "read-error" {
					answer = `echo '{"error":{"code":"timeout","message":"name reading timed out"}}'; exit 1`
				}
				script = strings.Replace(script, `"agent get") echo '{"error":{"code":"agent_not_found"}}' ;;`, `"agent get") `+answer+` ;;`, 1)
				if outcome == "close-error" {
					script = strings.Replace(script, `case "$g $s" in`, `case "$g $s" in`+"\n"+`  "pane close") echo '{"error":{"code":"close_failed","message":"close refused"}}'; exit 1 ;;`, 1)
				}
				f.herdr(t, script)
				t.Setenv("HERDR_ENV", "1")
				t.Setenv("HERDR_PANE_ID", "outside")
				if outcome == "timeout-own" {
					t.Setenv("HERDR_PANE_ID", "p2")
				}
				stateDir := filepath.Join(f.state, "hergy")
				state := filepath.Join(stateDir, "turns", "wave.json")
				signals := filepath.Join(stateDir, "turns", "wave.signals")
				tells := filepath.Join(stateDir, "turns", "wave.tells")
				writeFile(t, state, `{"team":"wave","holder":"lead"}`)
				writeFile(t, filepath.Join(signals, "pending.json"), "{}")
				writeFile(t, filepath.Join(tells, "pending.json"), "{}")
				opts, err := options.Parse([]string{"down", "wave"})
				if err != nil {
					t.Fatal(err)
				}
				opts.Timeout = 0
				var out, stderr bytes.Buffer
				err = runDown(context.Background(), serveEnv(t), opts, &out, &stderr)
				if outcome == "success" {
					if err != nil {
						t.Fatalf("down: %v; %s", err, &stderr)
					}
				} else {
					reported, ok := errors.AsType[reportedError](err)
					if !ok || reported.code != 1 {
						t.Fatalf("down must fail: %v; %s", err, &stderr)
					}
				}
				_, timedOut := errors.AsType[*ops.NameReleaseTimeout](err)
				if timedOut != strings.HasPrefix(outcome, "timeout") {
					t.Errorf("name-release timeout type = %t, error: %v", timedOut, err)
				}
				cleanup := outcome == "success" || strings.HasPrefix(outcome, "timeout")
				if got := strings.Contains(out.String(), "git worktree remove "+worktree); got != cleanup {
					t.Errorf("leftover list = %t, want %t:\n%s", got, cleanup, &out)
				}
				for _, path := range []string{state, signals, tells} {
					_, err := os.Stat(path)
					if cleanup && !errors.Is(err, os.ErrNotExist) {
						t.Errorf("turn remains at %s: %v", path, err)
					}
					if !cleanup && err != nil {
						t.Errorf("turn removed at %s: %v", path, err)
					}
				}
				if outcome == "timeout-own" {
					if strings.Contains(calls(t, log), "pane close p2") || !strings.Contains(stderr.String(), "自分のペインは残しました") {
						t.Errorf("own pane was not retained: %s; %s", calls(t, log), &stderr)
					}
				}
			})
		}
	}
}
