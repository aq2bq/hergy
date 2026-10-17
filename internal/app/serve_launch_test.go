package app

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
)

// otherProject writes a second project with its own definition, the way two
// projects sit side by side on one machine (W16).
func otherProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  coder:\n    kind: claude\n")
	writeFile(t, filepath.Join(root, ".hergy", "teams", "agile", "team.yaml"),
		"version: 1\nmission: crm の使命\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> human\n")
}

// launchRecord writes one launch record for a team, as a hergy up in that
// project would have.
func launchRecord(t *testing.T, project, definition, team string) {
	t.Helper()
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{
		Time:       "2026-10-17T09:12:00Z",
		Command:    launchlog.CommandUp,
		Project:    project,
		Definition: definition,
		Team:       team,
		TeamYAML:   filepath.Join(project, ".hergy", "teams", definition, "team.yaml"),
		Members:    []launchlog.Member{{Name: team + "-lead", Short: "lead", Prompt: "abc"}},
	}); err != nil {
		t.Fatal(err)
	}
}

// TestThePortReadsAnotherProjectsTeam: the launch record says which project a
// team was booted from, and the port reads that definition through that
// project's own files, without writing any (W16).
func TestThePortReadsAnotherProjectsTeam(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	other := filepath.Join(filepath.Dir(f.proj), "other")
	otherProject(t, other)
	launchRecord(t, other, "agile", "crm")

	env := serveEnv(t)
	port := &servePort{env: env, cat: env.Catalog(source.Disk{})}

	def, err := port.LaunchedDefinition("crm")
	if err != nil {
		t.Fatalf("the team's own project could not be read: %v", err)
	}
	if def.Name != "agile" {
		t.Errorf("read the definition %q, want agile", def.Name)
	}

	launch, ok := port.Launched("crm")
	if !ok {
		t.Fatalf("the launch record was not found")
	}
	if want := time.Date(2026, 10, 17, 9, 12, 0, 0, time.UTC); !launch.At.Equal(want) {
		t.Errorf("the launch time is %s, want %s", launch.At, want)
	}
	if launch.Prompts["lead"] != "abc" {
		t.Errorf("the member's prompt hash is %q, want abc", launch.Prompts["lead"])
	}

	// A team this machine holds no record of answers nothing, and the screen
	// keeps its one line.
	if _, ok := port.Launched("nosuch"); ok {
		t.Errorf("a team with no record answered one")
	}
	if _, err := port.LaunchedDefinition("nosuch"); err == nil {
		t.Errorf("a team with no record gave back a definition")
	}
}

// TestActivityJudgesATeamFromAnotherProject is W16 end to end: the served
// activity reads a team booted from another project's definition through that
// project, judges it and counts it.
func TestActivityJudgesATeamFromAnotherProject(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	// The words the assertions read are Japanese, so this one test asks for
	// them and puts the language back when it is done.
	was := i18n.Language()
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Use(was) })
	other := filepath.Join(filepath.Dir(f.proj), "other")
	otherProject(t, other)
	launchRecord(t, other, "agile", "crm")
	f.runningHerdr(t, `{"tab_id":"t1","label":"crm"}`,
		`{"name":"crm-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1","state_change_seq":5}`)

	url, stop := startServeForTest(t, f)
	defer stop()

	resp, err := http.Get(url + "/activity")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	page := string(body)
	if !strings.Contains(page, "人間を待っているチーム 1・動いているチーム 1") {
		t.Errorf("the team from another project is not judged and counted:\n%s", page)
	}
	if strings.Contains(page, "この場所にチーム定義") {
		t.Errorf("the team was not read from its own project:\n%s", page)
	}
}

// startServeForTest runs hergy serve with --no-open on a working XDG, and
// returns the URL it prints and a stop function.
func startServeForTest(t *testing.T, _ *fixture) (string, func()) {
	t.Helper()
	env := serveEnv(t)
	opts := options.Options{Port: 0, NoOpen: true}
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, env, &opts, urlSink{urls}, io.Discard)
	}()
	select {
	case url := <-urls:
		return url, func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("serve did not stop")
			}
		}
	case err := <-done:
		cancel()
		t.Fatalf("serve stopped before it answered: %v", err)
	case <-time.After(20 * time.Second):
		cancel()
		t.Fatal("serve printed no URL")
	}
	return "", func() {}
}

// urlSink picks the URL out of what runServe prints.
type urlSink struct{ urls chan string }

func (w urlSink) Write(p []byte) (int, error) {
	for line := range strings.SplitSeq(string(p), "\n") {
		if i := strings.Index(line, "http://"); i >= 0 {
			select {
			case w.urls <- strings.TrimSpace(line[i:]):
			default:
			}
		}
	}
	return len(p), nil
}
