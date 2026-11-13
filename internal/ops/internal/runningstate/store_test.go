package runningstate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

func testLaunch(t *testing.T) Launch {
	t.Helper()
	def, _ := domainname.ParseDefinitionName("definition")
	root, err := domainplace.NewProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := domainplace.NewProject(filepath.Join(root.String(), "team.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return Launch{Definition: def, Project: root, Layer: "local", TeamYAML: path, Workspace: Workspace{ID: "w1", Label: "work"},
		StartedAt: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Executable: "/bin/hergy", Lang: "en",
		Workers: []WorkerDefinition{{From: "lead", Purpose: "research", Profile: "engine", Start: "start", Env: "env"}}}
}

func testName(t *testing.T, raw string) domainname.TeamName {
	t.Helper()
	name, err := domainname.ParseTeamName(raw)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func requireState(t *testing.T, store Store, name domainname.TeamName) State {
	t.Helper()
	state, exists, err := store.Read(name)
	if err != nil || !exists {
		t.Fatalf("Read(%s): exists=%t err=%v", name, exists, err)
	}
	return state
}

func TestRunOwnsOnlyItsFacts(t *testing.T) {
	store := At(t.TempDir())
	name := testName(t, "alias")
	launch := testLaunch(t)
	if err := store.Begin(name, launch); err != nil {
		t.Fatal(err)
	}
	receipts := []Receipt{
		{Name: "alias-a", Short: "a", Prompt: "first a", Command: "up", Time: launch.StartedAt, Docs: []Doc{{Path: "/role/a", Layer: "local", Text: "rules a"}}, Start: "a-start", Env: "a-env"},
		{Name: "alias-b", Short: "b", Prompt: "first b", Command: "up", Time: launch.StartedAt},
	}
	if err := store.Receive(name, receipts); err != nil {
		t.Fatal(err)
	}
	worker := Worker{Name: "alias-one", From: "a", Purpose: "research", Tab: "worker:alias/one", Time: launch.StartedAt.Add(time.Minute)}
	if err := store.Hire(name, worker); err != nil {
		t.Fatal(err)
	}
	// A different handle represents another command, which must preserve a hire
	// when it updates one receipt and preserve the other member's last receipt.
	second := At(store.dir)
	updated := receipts[0]
	updated.Prompt = "second a"
	updated.Command = "reprompt"
	updated.Time = worker.Time
	if err := second.Receive(name, []Receipt{updated}); err != nil {
		t.Fatal(err)
	}
	got := requireState(t, store, name)
	if !reflect.DeepEqual(got.Launch(), launch) || !reflect.DeepEqual(got.Members(), []Receipt{updated, receipts[1]}) || !reflect.DeepEqual(got.Workers(), []Worker{worker}) {
		t.Fatalf("run lost facts: %+v", got.data)
	}
	// Changing any returned reading, including a nested slice, cannot change it.
	members := got.Members()
	members[0].Docs[0].Text = "caller changed it"
	members[0].Prompt = "caller changed it"
	workers := got.Workers()
	workers[0].From = "caller changed it"
	startup := got.Launch()
	startup.Workers[0].Profile = "caller changed it"
	if got.Members()[0].Docs[0].Text != "rules a" || got.Members()[0].Prompt != "second a" || got.Workers()[0].From != "a" || got.Launch().Workers[0].Profile != "engine" {
		t.Fatal("a caller mutated state")
	}
	if err := store.RemoveWorker(name, worker.Name); err != nil {
		t.Fatal(err)
	}
	got = requireState(t, store, name)
	if len(got.Workers()) != 0 || !reflect.DeepEqual(got.Members(), []Receipt{updated, receipts[1]}) {
		t.Fatal("worker removal changed member facts")
	}
	if err := store.Begin(name, launch); err != nil {
		t.Fatal(err)
	}
	got = requireState(t, store, name)
	if len(got.Members()) != 0 || len(got.Workers()) != 0 {
		t.Fatal("new run inherited an old run")
	}
	if err := store.End(name); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Read(name); exists || err != nil {
		t.Fatalf("ended state: %t %v", exists, err)
	}
	// Late receipts and hires must not resurrect a stopped team.
	if err := store.Receive(name, receipts); err != nil {
		t.Fatal(err)
	}
	if err := store.Hire(name, worker); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Read(name); exists || err != nil {
		t.Fatalf("late write resurrected a run: %t %v", exists, err)
	}
}

func TestStorePathAndAtomicFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	name := testName(t, "demo")
	if store.Path(name) != filepath.Join(root, "hergy", "teams", "demo.json") {
		t.Fatal(store.Path(name))
	}
	launch := testLaunch(t)
	if err := store.Begin(name, launch); err != nil {
		t.Fatal(err)
	}
	if err := store.Receive(name, []Receipt{{Name: "demo-a", Prompt: "complete body"}}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != ".locks" || entries[1].Name() != "demo.json" {
		t.Fatalf("state/temp files: %v", entries)
	}
	info, err := os.Stat(store.Path(name))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private prompt permissions: %v", info.Mode())
	}
	t.Setenv("XDG_STATE_HOME", "relative")
	home := t.TempDir()
	t.Setenv("HOME", home)
	fallback, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if fallback.dir != filepath.Join(home, ".local", "state", "hergy", "teams") {
		t.Fatal(fallback.dir)
	}
}

func TestListAndBrokenState(t *testing.T) {
	store := At(t.TempDir())
	if states, err := store.List(); err != nil || len(states) != 0 {
		t.Fatalf("new store: %v %v", states, err)
	}
	for _, raw := range []string{"second", "first"} {
		if err := store.Begin(testName(t, raw), testLaunch(t)); err != nil {
			t.Fatal(err)
		}
	}
	states, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].Name().String() != "first" || states[1].Name().String() != "second" {
		t.Fatalf("list: %+v", states)
	}
	name := testName(t, "first")
	for _, raw := range []string{`broken`, `{"team":"other"}`, `{"team":"first","definition":"definition","project":"relative","team_yaml":"/team.yaml"}`} {
		if err := os.WriteFile(store.Path(name), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.Read(name); err == nil {
			t.Fatalf("accepted broken state: %s", raw)
		}
		if _, err := store.List(); err == nil {
			t.Fatalf("list hid broken state: %s", raw)
		}
		if err := store.Hire(name, Worker{Name: "first-one"}); err == nil {
			t.Fatal("hire overwrote broken state")
		}
	}
	if err := store.End(name); err != nil {
		t.Fatal(err)
	}
	if err := store.End(name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path(name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestExistingNamesKeepTheirSpelling(t *testing.T) {
	store := At(t.TempDir())
	name := testName(t, "abcdefghijklmnopqrstuvwxyz")
	launch := testLaunch(t)
	launch.Definition, _ = domainname.ParseDefinitionName("Existing.Definition")
	if err := store.Begin(name, launch); err != nil {
		t.Fatal(err)
	}
	got := requireState(t, store, name)
	if got.Name() != name || got.Launch().Definition != launch.Definition {
		t.Fatalf("narrowed existing names: %+v", got.data)
	}
}
