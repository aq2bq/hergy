package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/web"
)

func TestCLIAndScreenStartRejectSameRunningName(t *testing.T) {
	for _, entry := range []string{"cli", "screen"} {
		t.Run(entry, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "wave")
			log := f.stopHerdr(t, false)
			t.Setenv("HERDR_ENV", "1")
			t.Setenv("HERDR_PANE_ID", "outside")
			useJapanese(t)
			var message string
			if entry == "cli" {
				var out, stderr bytes.Buffer
				if code := Run([]string{"up", "wave", "--as", " wave ", "--lang", "ja"}, &out, &stderr); code != 1 {
					t.Fatalf("exit %d: %s", code, &out)
				}
				message = stderr.String()
			} else {
				_, err := newActivityRead(serveEnv(t)).StartLaunch(web.HQLaunchIn{Definition: "wave", Name: " wave ", Place: "project", Workspace: web.HQWorkspaceIn{ID: "w1"}})
				refusal, ok := errors.AsType[*web.HQError](err)
				if !ok || refusal.Status != 409 || refusal.At != "name" {
					t.Fatalf("screen refusal: %v", err)
				}
				message = err.Error()
			}
			if !strings.Contains(message, "wave という名前のチームがすでに動いています。別の名前で起動してください") {
				t.Fatalf("name rule differs: %q", message)
			}
			for _, change := range []string{"tab create", "workspace create", "agent start"} {
				if strings.Contains(calls(t, log), change) {
					t.Fatalf("refusal changed herdr: %s", calls(t, log))
				}
			}
		})
	}
}

func TestStartTeamTrimsNameAndPreservesWorkspacePolicies(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo", "--as", "occupied")
	env := serveEnv(t)
	definition, _ := domainname.ParseDefinitionName("demo")
	plan, err := ops.StartTeam(context.Background(), ops.StartInput{Env: env, Definition: definition, Name: " next ", Workspace: ops.WorkspaceInput{Current: true}, Timeout: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Resolved.Team.Name.String() != "next" {
		t.Fatalf("name: %s", plan.Resolved.Team.Name)
	}
	if !strings.Contains(readFile(t, filepath.Join(f.cfg, "tabs.txt")), " next\n") {
		t.Fatal("the normalized team tab was not created")
	}
	useJapanese(t)
	_, err = newActivityRead(env).StartLaunch(web.HQLaunchIn{Definition: "demo", Name: "free-name", Place: "project", Workspace: web.HQWorkspaceIn{ID: "w1"}})
	refusal, ok := errors.AsType[*web.HQError](err)
	if !ok || refusal.Status != 409 || !strings.Contains(err.Error(), "ワークスペース") {
		t.Fatalf("screen accepted occupied workspace: %v", err)
	}
}

type startQuestionPort struct{ herdr.Port }

func (p startQuestionPort) JSON(args ...string) (map[string]any, error) {
	switch strings.Join(args[:2], " ") {
	case "agent start":
		return protocol.Result(args, `{"error":{"code":"agent_not_ready","message":"agent demo-a is blocked during startup and is not ready for prompts"}}`, "", 1)
	case "agent get":
		return map[string]any{"agent": map[string]any{"name": args[2], "agent": "claude", "agent_status": "blocked", "pane_id": "p1", "tab_id": "t1"}}, nil
	}
	return p.Port.JSON(args...)
}

func TestStartTeamCancellationUsesSharedProgressAndWritesNoRecord(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	env := serveEnv(t)
	definition, _ := domainname.ParseDefinitionName("demo")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sawQuestion := false
	port := startQuestionPort{herdrcli.New(os.Getenv("HERGY_HERDR"))}
	_, err := ops.StartTeam(ctx, ops.StartInput{Env: env, Definition: definition, Workspace: ops.WorkspaceInput{Current: true}, Timeout: 1000, Port: port, Observe: func(event ops.Event) {
		if event.Progress != nil && event.Progress.Waiting != nil {
			sawQuestion = true
			cancel()
		}
	}})
	if !sawQuestion || !errors.Is(err, context.Canceled) {
		t.Fatalf("question=%t, err=%v", sawQuestion, err)
	}
	_, records := launchLog(t, f)
	if len(records) != 0 {
		t.Fatalf("cancelled start recorded: %+v", records)
	}
	if strings.Contains(readFile(t, filepath.Join(f.cfg, "herdr.log")), "agent prompt ") {
		t.Fatal("cancellation sent a bootstrap prompt")
	}
}
