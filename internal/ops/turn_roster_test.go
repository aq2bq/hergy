package ops

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func TestWatcherFirstTickForgetsOldWorkersAndSaves(t *testing.T) {
	f := newTurnsFixture(t)
	st := &turns.State{Team: fixtureSeen("demo"), Holder: "old", Opened: map[string][]turns.Opening{"old": {{By: "lead", At: f.now}}, "builder": {{By: "lead", At: f.now}}}}
	if err := turns.Save(f.w.path, st); err != nil {
		t.Fatal(err)
	}
	if err := f.w.load(); err != nil {
		t.Fatal(err)
	}
	if f.w.state.Holder != "old" {
		t.Fatal("load forgot a party before reading herdr")
	}
	f.w.tick()
	saved, err := turns.Load(f.w.path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Holder != turns.Human || saved.Why != turns.WhyUnanswered || !reflect.DeepEqual(saved.Waits(), []string{"lead → builder"}) {
		t.Fatalf("stale turn survived loading: %+v", saved)
	}
}

func TestWatcherKeepsLiveWorkerBeforeRecordingHire(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(map[bool]string{false: "older team", true: "unrecorded hire"}[registered], func(t *testing.T) {
			f := newTurnsFixture(t)
			if registered {
				if err := teststate.Write(teststate.Team{Name: "demo", Definition: "demo", Project: t.TempDir()}); err != nil {
					t.Fatal(err)
				}
			}
			f.w.read = func() ([]team.Tab, []protocol.Agent, bool, error) {
				return []team.Tab{{ID: "tw", Label: fixtureSeen("worker:demo/worker"), Agents: fixtureSeenList([]string{"demo-worker"})}}, f.agents, true, nil
			}
			f.set("demo-worker", "working", 1)
			f.tell(t, "lead", "worker")
			f.w.tick()
			f.w.tick()
			if f.w.state.Holder != "worker" || !reflect.DeepEqual(f.w.state.Waits(), []string{"lead → worker"}) {
				t.Fatalf("live worker's tell was lost: %+v", f.w.state)
			}
			f.agents = nil
			f.w.tick()
			if f.w.state.Holder != turns.Human || len(f.w.state.Waits()) != 0 {
				t.Fatalf("departed worker survived: %+v", f.w.state)
			}
		})
	}
}

func TestWatcherForgetsStoppedWorkerWithoutAnotherTurn(t *testing.T) {
	f := newTurnsFixture(t)
	if err := teststate.Write(teststate.Team{Name: "demo", Definition: "demo", Project: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := teststate.Hire("demo", teststate.Worker{Name: "demo-worker", From: "lead"}); err != nil {
		t.Fatal(err)
	}
	f.tell(t, "lead", "worker")
	f.w.tick()
	if len(f.w.state.Waits()) != 1 {
		t.Fatal("worker's opening missing before stop")
	}
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := store.Read(fixtureDemoTeam())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveWorker(fixtureDemoTeam(), run.Generation(), "demo-worker"); err != nil {
		t.Fatal(err)
	}
	f.w.tick()
	saved, err := turns.Load(f.w.path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.w.state.Waits()) != 0 || len(saved.Waits()) != 0 {
		t.Fatalf("stopped worker still awaited: %+v / %+v", f.w.state, saved)
	}
}

func TestWatcherKeepsTellAndStateWhenRunCannotBeRead(t *testing.T) {
	f := newTurnsFixture(t)
	f.w.state.Opened = map[string][]turns.Opening{"old": {{By: "lead", At: f.now}}}
	f.w.state.Started = map[string]time.Time{"old": f.now}
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, store.Path(fixtureDemoTeam()), "{broken")
	f.tell(t, "lead", "builder")
	f.w.tick()
	if f.w.state.Holder != "builder" || !reflect.DeepEqual(f.w.state.Waits(), []string{"lead → builder", "lead → old"}) || !f.w.state.Started["old"].Equal(f.now) {
		t.Fatalf("unreadable run lost the tell or forgot state: %+v", f.w.state)
	}
	saved, err := turns.Load(f.w.path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Holder != "builder" || !reflect.DeepEqual(saved.Waits(), f.w.state.Waits()) {
		t.Fatalf("tell was not saved: %+v", saved)
	}
}

func TestStopWorkerForgetsOnlyItsWaitWithBrokenDefinition(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "definition", true: "broken definition"}[broken], func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "draft")
			log := f.workerHerdr(t, false)
			if err := teststate.Hire("draft", teststate.Worker{Name: "draft-bench", From: "lead"}); err != nil {
				t.Fatal(err)
			}
			env := serveEnv(t)
			name, _ := domainname.ParseTeamName("draft")
			run, _ := currentTeamState(name)
			path := turns.Path(filepath.Join(f.state, "hergy"), "draft")
			st := &turns.State{Team: fixtureSeen("draft"), Holder: "bench", Opened: map[string][]turns.Opening{"bench": {{By: "lead"}}, "builder": {{By: "lead"}}, "scout": {{By: "lead"}}}}
			if err := turns.Save(path, st); err != nil {
				t.Fatal(err)
			}
			if broken {
				writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "draft", "team.yaml"), "version: [broken")
			}
			result, err := StopWorker(context.Background(), StopInput{Env: env, Target: StopTarget{
				team:   team.Running{Team: fixtureRunningTeam("draft"), State: run, Project: env.ProjectRoot},
				worker: &downWorker{Team: fixtureSeen("draft"), Short: fixtureSeen("bench"), Herdr: fixtureSeen("draft-bench"), Label: fixtureSeen("worker:draft/bench")},
			}, Now: func() time.Time { return activityNow }})
			if err != nil {
				t.Fatal(err)
			}
			if result.Closed.Workers != 1 || !strings.Contains(calls(t, log), "pane close p2") || strings.Contains(calls(t, log), "pane close p3") {
				t.Fatal("worker was not closed")
			}
			saved, err := turns.Load(path, fixtureSeen("draft"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"lead → builder", "lead → scout"}
			if !reflect.DeepEqual(saved.Waits(), want) || saved.Holder != turns.Human || saved.Why != turns.WhyUnanswered {
				t.Fatalf("waits after stop: %+v, want %v", saved, want)
			}
		})
	}
}

func TestOlderTeamTakeoverFirstTickForgetsPreviousWorkers(t *testing.T) {
	f, env, r, port, _ := stopFixture(t)
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.End(fixtureDemoTeam()); err != nil {
		t.Fatal(err)
	}
	path := turns.Path(filepath.Join(f.state, "hergy"), "demo")
	if err := turns.Save(path, &turns.State{Team: fixtureSeen("demo"), Holder: "old", Opened: map[string][]turns.Opening{"old": {{By: "a"}}, "a": {{By: "b"}}}}); err != nil {
		t.Fatal(err)
	}
	took, err := takeOverOlderTeam(StartInput{Env: env, Port: port, Now: func() time.Time { return activityNow }}, r)
	if err != nil || !took {
		t.Fatalf("takeover = %v, %v", took, err)
	}
	w := newTurnWatcher(fixtureDemoTeam(), r.Team.Definition.Graph, nil, path, io.Discard)
	w.clock = func() time.Time { return activityNow }
	w.read = func() ([]team.Tab, []protocol.Agent, bool, error) { return nil, nil, true, nil }
	if err := w.load(); err != nil {
		t.Fatal(err)
	}
	w.tick()
	saved, err := turns.Load(path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Holder != turns.Human || !reflect.DeepEqual(saved.Waits(), []string{"b → a"}) {
		t.Fatalf("previous worker survived takeover: %+v", saved)
	}
}
