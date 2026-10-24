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
}

func (f fakeHQ) Projects() (HQProjects, error) { return f.projects, nil }
func (f fakeHQ) AddProject(path string) (HQProjects, error) {
	next := f.projects
	next.Projects = append(next.Projects, HQProject{Path: path, Listed: true})
	return next, nil
}
func (f fakeHQ) RemoveProject(string) (HQProjects, error) { return f.projects, nil }
func (f fakeHQ) Running() HQRunning                       { return HQRunning{Herdr: ActivityNote{OK: true}} }
func (f fakeHQ) Workspaces() HQWorkspaces                 { return HQWorkspaces{} }
func (f fakeHQ) LaunchOptions(string, string, string) (HQLaunchOptions, error) {
	return HQLaunchOptions{}, nil
}
func (f fakeHQ) StartLaunch(HQLaunchIn) (HQLaunch, error) { return HQLaunch{}, nil }
func (f fakeHQ) LaunchState(string) (HQLaunch, bool)      { return HQLaunch{}, false }
func (f fakeHQ) CancelLaunch(string) (HQLaunch, bool)     { return HQLaunch{}, false }
func (f fakeHQ) Stop(string) (HQStopped, error)           { return HQStopped{}, nil }

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

// TestHQTeamsGroupsTheProjectAndTheGlobalLayer: the group of a project holds only
// that project's rows, and the global group comes last with what it defines
// (d-73fc).
func TestHQTeamsGroupsTheProjectAndTheGlobalLayer(t *testing.T) {
	f := newFixture(t)
	global := filepath.Join(f.global, "teams", "docs")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "team.yaml"), []byte("version: 1\nmission: global docs\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(other, ".hergy", "teams", "ledger", "team.yaml"), []byte("version: 1\nmission: ledger\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
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

// TestHQRemoveProjectReadsPath: DELETE /api/hq/projects carries the directory in
// ?path=, as api.md writes it, and answers the project list (r-b7e1).
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
