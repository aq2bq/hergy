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
func (f fakeHQ) RefreshMembers(in HQRefreshIn) (HQLaunch, error) {
	return HQLaunch{ID: "l-9", Action: ActionRefresh, Team: in.Team, Members: in.Members, Phase: "starting"}, nil
}
func (f fakeHQ) LaunchState(string) (HQLaunch, bool)  { return HQLaunch{}, false }
func (f fakeHQ) CancelLaunch(string) (HQLaunch, bool) { return HQLaunch{}, false }
func (f fakeHQ) Stop(string) (HQStopped, error)       { return HQStopped{}, nil }

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

// TestHQRefreshRaisesTheNamedMembers: POST /api/hq/refresh takes the team and the
// members to raise, and answers 202 with a start's own state, so the sheet reads
// its progress and its startup question at /api/hq/launches/{id} (r-5d1a).
func TestHQRefreshRaisesTheNamedMembers(t *testing.T) {
	f := newFixture(t)
	server := f.hqServer(t)
	recorder := &raiseHQ{fakeHQ: server.hq.(fakeHQ)}
	server.hq = recorder
	resp, answer := f.apiJSON(t, server, http.MethodPost, "/api/hq/refresh",
		map[string]any{"team": "core", "members": []string{"builder"}})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /api/hq/refresh answers %d, want 202: %+v", resp.StatusCode, answer)
	}
	if recorder.in.Team != "core" || len(recorder.in.Members) != 1 || recorder.in.Members[0] != "builder" {
		t.Errorf("the raise read %+v, want core's builder", recorder.in)
	}
	launch, ok := answer["launch"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no launch: %+v", answer)
	}
	if launch["action"] != ActionRefresh || launch["id"] != "l-9" || launch["phase"] != "starting" {
		t.Errorf("the launch = %+v, want a refresh's own start state", launch)
	}
}

// raiseHQ records what a raise names.
type raiseHQ struct {
	fakeHQ
	in HQRefreshIn
}

func (r *raiseHQ) RefreshMembers(in HQRefreshIn) (HQLaunch, error) {
	r.in = in
	return r.fakeHQ.RefreshMembers(in)
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
	if err := os.WriteFile(filepath.Join(global, "team.yaml"), []byte("version: 1\nmission: global docs\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(team, "team.yaml"), []byte("version: 1\nmission: ledger\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n"), 0o644); err != nil {
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
