package ops

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/teststate"
)

// A live worker tab can survive in a fake snapshot across End/Begin. Neither it
// nor retained history may restore an earlier run's hirer, member name or time.
func TestWorkerReadersKeepOnlyThisRunsFacts(t *testing.T) {
	f, env, r, port, _ := stopFixture(t)
	name := fixtureDemoTeam()
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	begin := func(start time.Time) {
		t.Helper()
		if err := teststate.Write(teststate.Team{Name: "demo", Definition: "demo", Project: f.proj, StartedAt: start}); err != nil {
			t.Fatal(err)
		}
	}
	begin(at)
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	hire := func(from string, hired time.Time) {
		t.Helper()
		if err := teststate.Hire("demo", teststate.Worker{Name: "demo-scroll", From: from, Tab: "worker:demo/scroll", Time: hired}); err != nil {
			t.Fatal(err)
		}
	}
	hire("a", at.Add(time.Minute))
	state, _ := currentTeamState(name)
	if err := store.Receive(name, state.Generation(), []runningstate.Receipt{{Name: "demo-former", Short: "former"}}); err != nil {
		t.Fatal(err)
	}
	port.snapshot.Tabs = append(port.snapshot.Tabs, protocol.Tab{ID: "tw", Label: fixtureSeen("worker:demo/scroll")})
	port.snapshot.Agents = append(port.snapshot.Agents, protocol.Agent{Name: fixtureSeen("demo-scroll"), TabID: "tw", Status: "idle"})
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{Command: launchlog.CommandHire, Team: name.Observation(), Project: observedplace.Observe(f.proj), Time: at.Add(3 * time.Hour).Format(time.RFC3339), Hire: &launchlog.Hire{Worker: fixtureSeen("demo-scroll"), From: "stale", Tab: fixtureSeen("worker:demo/scroll")}}); err != nil {
		t.Fatal(err)
	}
	check := func(start, hired time.Time, from string) {
		t.Helper()
		hirers, names := hirersOf(name)
		state, _ := currentTeamState(name)
		sources := hireSources(state)
		parties := tellParties(r, TellInput{MemberInput: MemberInput{Port: port}})
		if from == "" {
			if len(hirers) != 0 || len(sources) != 0 || names["scroll"].String() != "" || slices.Contains(parties, fixtureSeen("demo-scroll")) {
				t.Fatalf("previous hire survived: hirers=%v names=%v sources=%v parties=%v", hirers, names, sources, parties)
			}
		} else {
			if !reflect.DeepEqual(hirers, map[string]string{"scroll": from}) || names["scroll"].String() != "demo-scroll" || !reflect.DeepEqual(sources, map[string]string{"demo-scroll": from}) || !slices.Contains(parties, fixtureSeen("demo-scroll")) {
				t.Fatalf("current hire missing: hirers=%v names=%v sources=%v parties=%v", hirers, names, sources, parties)
			}
		}
		if got, known := teamStartedAt(env.ProjectRoot, "demo"); !known || !got.Equal(start) {
			t.Fatalf("team time=%v known=%t want=%v", got, known, start)
		}
		if got, known := workerHiredAt(env.ProjectRoot, "demo", "demo-scroll"); known != !hired.IsZero() || !got.Equal(hired) {
			t.Fatalf("worker time=%v known=%t want=%v", got, known, hired)
		}
	}
	check(at, at.Add(time.Minute), "a")
	if err := store.End(name); err != nil {
		t.Fatal(err)
	}
	next := at.Add(24 * time.Hour)
	begin(next)
	check(next, time.Time{}, "")
	_, names := hirersOf(name)
	if names["former"].String() != "" {
		t.Fatal("previous member name survived Begin")
	}
	hire("b", next.Add(time.Minute))
	check(next, next.Add(time.Minute), "b")
	if err := os.Remove(filepath.Join(f.state, "hergy", "launches", "log.jsonl")); err != nil {
		t.Fatal(err)
	}
	check(next, next.Add(time.Minute), "b")
	// A project with the same team spelling cannot borrow these times.
	other, _ := domainplace.NewProject(t.TempDir())
	if _, known := teamStartedAt(other, "demo"); known {
		t.Fatal("another project borrowed the run time")
	}
	if _, known := workerHiredAt(other, "demo", "demo-scroll"); known {
		t.Fatal("another project borrowed the hire time")
	}
	// A legacy invalid name is never a state filename, and missing workers have no fallback.
	for _, raw := range []string{"Upper/a", "absent"} {
		if _, known := teamStartedAt(env.ProjectRoot, raw); known {
			t.Fatalf("unregistered run %q has a time", raw)
		}
	}
	if _, known := workerHiredAt(env.ProjectRoot, "demo", "demo-absent"); known {
		t.Fatal("missing worker inherited team start time")
	}
}

func TestWorkerMetadataDoesNotTrustUnreadableState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	project, _ := domainplace.NewProject(t.TempDir())
	name, _ := domainname.ParseTeamName("demo")
	if err := teststate.Write(teststate.Team{Name: "demo", Definition: "demo", Project: project.String()}); err != nil {
		t.Fatal(err)
	}
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(name), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if hirers, names := hirersOf(name); len(hirers) != 0 || len(names) != 0 {
		t.Fatalf("unreadable run supplied metadata: %v %v", hirers, names)
	}
	if _, known := teamStartedAt(project, "demo"); known {
		t.Fatal("unreadable run supplied a start time")
	}
	if _, known := workerHiredAt(project, "demo", "demo-scroll"); known {
		t.Fatal("unreadable run supplied a hire time")
	}
}
