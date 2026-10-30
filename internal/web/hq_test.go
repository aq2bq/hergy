package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

// fakeHQ is a headquarters reading a test writes: the projects it lists, and the
// catalog each one reads. It answers no start: the screens' reads are what these
// tests are about.
type fakeHQ struct {
	projects HQProjects
	cats     map[string]*catalog.Catalog
	global   *catalog.Catalog
	// running is what GET /api/hq/running answers; empty is herdr with nothing
	// running.
	running HQRunning
}

func (f fakeHQ) Projects() (HQProjects, error) { return f.projects, nil }
func (f fakeHQ) AddProject(path string) (HQProjects, error) {
	next := f.projects
	next.Projects = append(next.Projects, HQProject{Path: path, Listed: true})
	return next, nil
}
func (f fakeHQ) RemoveProject(string) (HQProjects, error) { return f.projects, nil }
func (f fakeHQ) Running() HQRunning {
	if f.running.Teams != nil {
		return f.running
	}
	return HQRunning{Herdr: ActivityNote{OK: true}}
}
func (f fakeHQ) Workspaces() HQWorkspaces { return HQWorkspaces{} }
func (f fakeHQ) LaunchOptions(string, string, string) (HQLaunchOptions, error) {
	return HQLaunchOptions{}, nil
}
func (f fakeHQ) StartLaunch(HQLaunchIn) (HQLaunch, error) { return HQLaunch{}, nil }
func (f fakeHQ) LaunchState(string) (HQLaunch, bool)      { return HQLaunch{}, false }
func (f fakeHQ) CancelLaunch(string) (HQLaunch, bool)     { return HQLaunch{}, false }
func (f fakeHQ) Stop(string) (HQStopped, error)           { return HQStopped{}, nil }

type launchHQ struct {
	fakeHQ
	started []HQLaunchIn
}

func (f *launchHQ) StartLaunch(in HQLaunchIn) (HQLaunch, error) {
	f.started = append(f.started, in)
	return HQLaunch{
		ID: "l-1", Action: ActionLaunch, Team: in.Definition, Definition: in.Definition,
		Place: in.Place, Project: in.Project, ProjectName: "proj",
		Workspace: HQWorkspace{ID: "w-1", Label: "workspace"}, Tab: HQTab{ID: "t-1", Label: in.Definition},
		Phase: "starting", Steps: []HQLaunchStep{}, Error: "",
	}, nil
}

func (f fakeHQ) ProjectCatalog(path string) (*catalog.Catalog, error) {
	if cat, ok := f.cats[path]; ok {
		return cat, nil
	}
	return nil, &HQError{Status: http.StatusUnprocessableEntity, Text: "not listed"}
}

func (f fakeHQ) GlobalCatalog() *catalog.Catalog { return f.global }

// hqServer is a server over the fixture with a headquarters that lists this
// project.
func (f *fixture) hqServer(t *testing.T) *Server {
	t.Helper()
	server := f.server(t)
	hq := fakeHQ{
		projects: HQProjects{
			Mode:    "current",
			Current: HQProject{Path: f.proj, Name: "proj", Listed: true},
			Projects: []HQProject{
				{Path: f.proj, Name: "proj", Listed: true},
			},
			Global: HQGlobal{Path: f.global, Display: "~/.config/hergy"},
		},
		cats:   map[string]*catalog.Catalog{f.proj: f.catalog()},
		global: catalog.New(f.global, filepath.Join(f.global, "no-local"), f.proj, source.Disk{}, func(p string) string { return p }),
	}
	server.hq = hq
	server.projects = hq
	return server
}

// TestHQProjectsIsServed: GET /api/hq/projects answers the headquarters reading.
func TestHQProjectsIsServed(t *testing.T) {
	f := newFixture(t)
	resp, body := f.get(t, f.hqServer(t), "/api/hq/projects")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/hq/projects answers %d: %s", resp.StatusCode, body)
	}
	var got HQProjects
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "current" || got.Current.Path != f.proj {
		t.Errorf("projects = %+v", got)
	}
}

// TestHQWithoutAReadingAnswers503: a server built without a headquarters does not
// show a made-up list.
func TestHQWithoutAReadingAnswers503(t *testing.T) {
	f := newFixture(t)
	resp, _ := f.get(t, f.server(t), "/api/hq/projects")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /api/hq/projects without a reading answers %d, want 503", resp.StatusCode)
	}
}

// TestHQLaunchAcceptsOmittedNameAndOneWorkspaceChoice: the server derives an
// omitted name from the definition, and a launch chooses either an existing or
// a new workspace (n-e3de).
func TestHQLaunchAcceptsOmittedNameAndOneWorkspaceChoice(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	hq := &launchHQ{fakeHQ: server.hq.(fakeHQ)}
	server.hq = hq

	for _, tc := range []struct {
		name      string
		workspace map[string]string
	}{
		{name: "existing workspace", workspace: map[string]string{"id": "w-1"}},
		{name: "new workspace", workspace: map[string]string{"new": "fresh"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"project": f.proj, "definition": "demo", "place": "project", "workspace": tc.workspace,
			}
			resp, body := f.api(t, server, http.MethodPost, "/api/hq/launch", payload)
			if resp.StatusCode != http.StatusAccepted {
				t.Fatalf("POST /api/hq/launch answers %d: %s", resp.StatusCode, body)
			}
		})
	}
	if len(hq.started) != 2 {
		t.Fatalf("the headquarters received %d starts, want 2", len(hq.started))
	}
	for _, in := range hq.started {
		if in.Name != "" {
			t.Errorf("omitted name decoded as %q, want empty for the default", in.Name)
		}
	}
	if got := hq.started[0].Workspace; got.ID != "w-1" || got.New != "" {
		t.Errorf("existing workspace request = %+v", got)
	}
	if got := hq.started[1].Workspace; got.New != "fresh" || got.ID != "" {
		t.Errorf("new workspace request = %+v", got)
	}
}

// TestHQTeamsGroupsTheProjectAndTheGlobalLayer: the group of a project holds only
// that project's rows, and the global group comes last with what it defines
// (d-73fc).
func TestHQTeamsGroupsTheProjectAndTheGlobalLayer(t *testing.T) {
	f := newFixture(t)
	global := filepath.Join(f.global, "teams", "docs")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "team.yaml"), []byte("version: 1\ndescription: global docs\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, body := f.get(t, f.hqServer(t), "/api/hq/teams")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/hq/teams answers %d: %s", resp.StatusCode, body)
	}
	var got HQGroups
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Groups) != 2 {
		t.Fatalf("groups = %+v", got.Groups)
	}
	project, globalGroup := got.Groups[0], got.Groups[1]
	if project.Project == nil || project.Project.Path != f.proj || len(project.List.Items) != 1 {
		t.Errorf("the project's group = %+v", project)
	}
	if !globalGroup.Global || len(globalGroup.List.Items) != 1 || globalGroup.List.Items[0].Name != "docs" {
		t.Errorf("the global group = %+v", globalGroup)
	}
	for _, item := range project.List.Items {
		if _, ok := item.Places["project"]; !ok {
			t.Errorf("a project row carries %v", item.Places)
		}
	}
}

// TestAScreenReadsOneProjectWithProject: a GET with project= reads that project's
// catalog, which is how the UI reads another listed project's row.
func TestAScreenReadsOneProjectWithProject(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(f.root, "other")
	if err := os.MkdirAll(filepath.Join(other, ".hergy", "teams", "ledger"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".hergy", "teams", "ledger", "team.yaml"), []byte("version: 1\ndescription: ledger\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := f.hqServer(t)
	hq := server.hq.(fakeHQ)
	hq.cats[other] = catalog.New(f.global, filepath.Join(other, ".hergy"), other, source.Disk{}, func(p string) string { return p })
	server.hq = hq
	server.projects = hq

	resp, body := f.get(t, server, "/api/team?project="+other)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/team?project= answers %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"ledger"`) {
		t.Fatalf("the other project's definition is not in the answer: %s", body)
	}
}

// TestAHQWriteNeedsTheToken: the headquarters' writes keep the boundary's
// promises (spec-boundary 5).
func TestAHQWriteNeedsTheToken(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	server.addr = "127.0.0.1:8787"
	req := httptest.NewRequest(http.MethodPost, "/api/hq/projects", strings.NewReader(`{"path":"/tmp"}`))
	req.Host = "127.0.0.1:8787"
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /api/hq/projects without a token answers %d, want 403", rec.Code)
	}
}

// removalHQ records the path a DELETE names.
type removalHQ struct {
	fakeHQ
	path string
}

func (r *removalHQ) RemoveProject(path string) (HQProjects, error) {
	r.path = path
	return r.Projects()
}

// TestHQRemoveProjectReadsPath: DELETE /api/hq/projects carries the directory in// ?path=, as api.md writes it, and answers the project list (r-b7e1).
func TestHQRemoveProjectReadsPath(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	base := server.hq.(fakeHQ)
	recorder := &removalHQ{fakeHQ: base}
	server.hq = recorder
	server.projects = recorder
	server.addr = "127.0.0.1:8787"
	target := filepath.Join(f.root, "ledger")
	req := httptest.NewRequest(http.MethodDelete, "/api/hq/projects?path="+url.QueryEscape(target), nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("X-Hergy-Token", server.token)
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/hq/projects answers %d: %s", rec.Code, rec.Body.String())
	}
	if recorder.path != target {
		t.Errorf("the handler removed %q, want %q", recorder.path, target)
	}
	if !strings.Contains(rec.Body.String(), `"mode"`) {
		t.Errorf("the answer is not the project list: %s", rec.Body.String())
	}
}

// TestHQBandCountsEveryGroup: the band's three numbers are the listed projects'
// and the global layer's, not the one project serve was started in (spec 2 章).
func TestHQBandCountsEveryGroup(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(f.root, "other")
	writeOtherTeam(t, other)
	global := filepath.Join(f.global, "teams", "docs")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "team.yaml"), []byte("version: 1\ndescription: global docs\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := f.hqServer(t)
	hq := server.hq.(fakeHQ)
	hq.projects = HQProjects{
		Mode:    "listed",
		Current: HQProject{Path: f.proj, Name: "proj", Listed: true},
		Projects: []HQProject{
			{Path: f.proj, Name: "proj", Listed: true, Defs: 1, Roles: 2, Profiles: 2},
			{Path: other, Name: "other", Listed: true, Defs: 1, Roles: 1, Profiles: 0},
		},
		Global: HQGlobal{Path: f.global, Display: "~/.config/hergy", Defs: 1, Roles: 3, Profiles: 2},
	}
	hq.cats[other] = catalog.New(f.global, filepath.Join(other, ".hergy"), other, source.Disk{}, func(p string) string { return p })
	server.hq, server.projects = hq, hq

	nav := server.navOf("teams")
	if nav.HQ == nil || nav.HQ.Mode != "listed" || nav.HQ.N != 2 {
		t.Fatalf("the band's headquarters part = %+v", nav.HQ)
	}
	if nav.Teams != 3 || nav.Roles != 6 || nav.Profiles != 4 {
		t.Errorf("the band's counts = %d/%d/%d, want the two projects' and the global layer's", nav.Teams, nav.Roles, nav.Profiles)
	}
}

// writeOtherTeam writes one project with a team definition of its own.
func writeOtherTeam(t *testing.T, dir string) {
	t.Helper()
	team := filepath.Join(dir, ".hergy", "teams", "ledger-dev")
	if err := os.MkdirAll(team, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(team, "team.yaml"), []byte("version: 1\ndescription: ledger\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAScreenCarriesTheProjectGroups: the three list screens carry the groups of
// every listed project and the global layer, so the rows stand under their
// project (spec 4 章).
func TestAScreenCarriesTheProjectGroups(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	for _, target := range []string{"/teams", "/roles", "/profiles"} {
		resp, body := f.get(t, server, target)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s answers %d: %s", target, resp.StatusCode, body)
		}
		if !strings.Contains(body, `"hq":{"projects"`) {
			t.Errorf("GET %s carries no headquarters groups: %s", target, body)
		}
	}
}

// TestTheBandSaysHowManyProjects: the band's crumb names the number of listed
// projects; without projects: it keeps the one project and the way to list more
// (spec 2 章).
func TestTheBandSaysHowManyProjects(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	_, body := f.get(t, server, "/teams")
	if !strings.Contains(body, "ほかのプロジェクトも並べる") {
		t.Errorf("a serve without projects: carries no way to list more: %s", body)
	}
	if !strings.Contains(body, `id="hq-q"`) {
		t.Errorf("the band carries no search field: %s", body)
	}
}

// TestHQRunningCarriesWhyADefinitionCannotBeBuilt: a running team whose
// definition does not build answers its reason in error (r-fc16), and that
// answer goes through the API, so openapi.yaml holds the field (n-e3de).
func TestHQRunningCarriesWhyADefinitionCannotBeBuilt(t *testing.T) {
	f := newFixture(t)
	hq := fakeHQ{running: HQRunning{Herdr: ActivityNote{OK: true}, Teams: []HQRunningTeam{{
		Team: "core", Definition: "core", Place: "project", Project: f.proj, ProjectName: "proj",
		Members: []HQMember{{Name: "lead", Herdr: "core-lead", Live: true, State: "idle"}},
		Workers: []HQWorker{}, Waiting: []string{}, NotUp: []string{},
		Error: "ロール名「進行管理」は使えません",
	}}}}
	server := f.server(t)
	server.hq = hq
	resp, body := f.api(t, server, http.MethodGet, "/api/hq/running", nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "進行管理") {
		t.Fatalf("GET /api/hq/running = %d %s", resp.StatusCode, body)
	}
}
