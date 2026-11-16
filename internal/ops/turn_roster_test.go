package ops

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func TestWatcherLoadForgetsOldWorkersAndSaves(t *testing.T) {
	f := newTurnsFixture(t)
	st := &turns.State{Team: fixtureSeen("demo"), Holder: "old", Opened: map[string][]turns.Opening{"old": {{By: "lead", At: f.now}}, "builder": {{By: "lead", At: f.now}}}}
	if err := turns.Save(f.w.path, st); err != nil {
		t.Fatal(err)
	}
	if err := f.w.load(); err != nil {
		t.Fatal(err)
	}
	saved, err := turns.Load(f.w.path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Holder != turns.Human || saved.Why != turns.WhyUnanswered || !reflect.DeepEqual(saved.Waits(), []string{"lead → builder"}) {
		t.Fatalf("stale turn survived loading: %+v", saved)
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

func TestStopWorkerForgetsItsWaitWithBrokenDefinition(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "definition", true: "broken definition"}[broken], func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "wave")
			log := f.stopHerdr(t, true)
			if err := teststate.Hire("wave", teststate.Worker{Name: "wave-deps", From: "lead"}); err != nil {
				t.Fatal(err)
			}
			env := serveEnv(t)
			name, _ := domainname.ParseTeamName("wave")
			run, _ := currentTeamState(name)
			path := turns.Path(filepath.Join(f.state, "hergy"), "wave")
			st := &turns.State{Team: fixtureSeen("wave"), Holder: "deps", Opened: map[string][]turns.Opening{"deps": {{By: "lead"}}, "builder": {{By: "lead"}}, "old": {{By: "lead"}}}}
			if err := turns.Save(path, st); err != nil {
				t.Fatal(err)
			}
			if broken {
				writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "wave", "team.yaml"), "version: [broken")
			}
			result, err := StopWorker(context.Background(), StopInput{Env: env, Target: StopTarget{
				team:   team.Running{Team: fixtureRunningTeam("wave"), State: run, Project: env.ProjectRoot},
				worker: &downWorker{Team: fixtureSeen("wave"), Short: fixtureSeen("deps"), Herdr: fixtureSeen("wave-deps"), Label: fixtureSeen("worker:wave/deps")},
			}, Now: func() time.Time { return activityNow }})
			if err != nil {
				t.Fatal(err)
			}
			if result.Closed.Workers != 1 || !strings.Contains(calls(t, log), "pane close p3") {
				t.Fatal("worker was not closed")
			}
			saved, err := turns.Load(path, fixtureSeen("wave"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"lead → builder"}
			if broken {
				want = append(want, "lead → old")
			}
			if !reflect.DeepEqual(saved.Waits(), want) || saved.Holder != turns.Human || saved.Why != turns.WhyUnanswered {
				t.Fatalf("waits after stop: %+v, want %v", saved, want)
			}
		})
	}
}

func TestOlderTeamTakeoverForgetsPreviousWorkers(t *testing.T) {
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
	saved, err := turns.Load(path, fixtureSeen("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Holder != turns.Human || !reflect.DeepEqual(saved.Waits(), []string{"b → a"}) {
		t.Fatalf("previous worker survived takeover: %+v", saved)
	}
}
