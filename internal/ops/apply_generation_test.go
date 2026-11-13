package ops

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"errors"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/words"
)

type plannedApplyPort struct {
	herdr.Port
	mutations []string
}

func (p *plannedApplyPort) JSON(args ...string) (map[string]any, error) {
	if len(args) > 1 {
		command := strings.Join(args[:2], " ")
		switch command {
		case "agent prompt", "agent start", "pane split", "pane close":
			p.mutations = append(p.mutations, command)
		}
	}
	return p.Port.JSON(args...)
}

func TestApplyRejectsAPlanFromBeforeTheTeamWasStartedAgain(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	writeFile(t, role, readFile(t, role)+"\nChanged instruction for the plan.\n")
	port := &plannedApplyPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR"))}
	var replacement runningstate.State
	accepted := false
	_, err := Apply(context.Background(), ApplyInput{Env: serveEnv(t), Selection: ApplySelectionInput{Names: []string{"a"}, Team: "demo"}, Timeout: 1000, Port: port,
		Accepted: func(ApplySelection) {
			accepted = true
			mustRun(t, "down", "demo")
			// The fake keeps empty tabs; use a fresh herdr session for the new run.
			f.statefulHerdr(t)
			mustRun(t, "up", "demo")
			replacement = teamState(t, "demo")
		},
	})
	if !accepted {
		t.Fatal("the operation never completed its selection")
	}
	refusal, refused := errors.AsType[*Refusal](err)
	rule, explained := errors.AsType[*words.Refusal](err)
	if !refused || refusal.Kind != "conflict" || !explained || rule.Rule != "apply.run-changed" {
		t.Errorf("old plan was not refused as a changed run: %v", err)
	}
	if len(port.mutations) != 0 {
		t.Errorf("old plan mutated the replacement: %v", port.mutations)
	}
	if after := teamState(t, "demo"); !reflect.DeepEqual(after.Members(), replacement.Members()) {
		t.Error("old plan replaced the new run's receipts")
	}
}
