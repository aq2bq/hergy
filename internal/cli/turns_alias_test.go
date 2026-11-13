package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops"
)

// ac-9a3d: inspect the team after tab creation, before any member pane is named
// or any initial prompt is received. No launch history exists yet.
func TestAliasDuringStartupResolvesTurnsTellAndStatus(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	env, err := ops.NewEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := domainname.ParseDefinitionName("demo")
	observed := false
	_, err = ops.StartTeam(context.Background(), ops.StartInput{Env: env, Definition: definition, Name: "alpha", Workspace: ops.WorkspaceInput{Current: true}, Timeout: 1000,
		Now: func() time.Time { return time.Unix(1, 0) }, Observe: func(event ops.Event) {
			if event.Progress == nil || event.Progress.CreatedTab == "" {
				return
			}
			observed = true
			for _, args := range [][]string{{"turns", "alpha", "--log"}, {"tell", "human", "--team", "alpha", "--from", "lead"}, {"status", "alpha", "--json"}} {
				stdout, stderr, code := run(args...)
				if code != 0 {
					t.Fatalf("startup %v: %d %s", args, code, stderr)
				}
				if args[0] == "status" && (!strings.Contains(stdout, "alpha-lead") || !strings.Contains(stdout, "alpha-builder")) {
					t.Fatalf("status used wrong definition: %s", stdout)
				}
			}
		}})
	if err != nil || !observed {
		t.Fatalf("start: observed=%t err=%v", observed, err)
	}
	calls := readFile(t, log)
	if strings.Contains(calls, "--definition") || !strings.Contains(calls, " turns alpha -C "+f.proj+" --lang ") {
		t.Fatalf("watcher command: %s", calls)
	}
}
