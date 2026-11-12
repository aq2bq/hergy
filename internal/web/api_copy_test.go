package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the copy of a document that stands under a listed project's head
// (r-a028): the row the headquarters draws for another project's document,
// copied into the global layer, reads that project's file. Reading the project
// serve was started in left a team copy without a base, and its save could not
// be pressed at all, and it wrote a role copy from another project's lines.

// copyFixture is a fixture with a second listed project: the serve's own project
// holds one definition, and the other project holds one of its own.
type copyFixture struct {
	*fixture
	other string
}

func newCopyFixture(t *testing.T) copyFixture {
	t.Helper()
	f := newFixture(t)
	other := filepath.Join(f.root, "other")
	dir := filepath.Join(other, ".hergy", "teams", "other-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "version: 1\ndescription: \"ほかのプロジェクトの定義\"\nmembers:\n  - name: lead\n    role: lead\n    profile: planner\nturns:\n  - human -> lead\n  - lead -> human\n"
	if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	agree := "- ほかのプロジェクトのチームのルール。\n"
	if err := os.WriteFile(filepath.Join(dir, "agreement.md"), []byte(agree), 0o644); err != nil {
		t.Fatal(err)
	}
	// The other project holds a role document and a profile of its own: what a
	// copy of its definition can bring into the global layer (r-7e4a).
	writeUnder(t, other, ".hergy/roles/lead.md", "# 仕切る\n\n- ほかのプロジェクトの lead。\n")
	writeUnder(t, other, ".hergy/profiles.yaml", "version: 1\nprofiles:\n  planner:\n    kind: codex\n")

	// The other project's builder says something this project's builder does
	// not: a copy that reads the wrong file writes the copy from the wrong
	// lines.
	roles := filepath.Join(other, ".hergy", "roles")
	if err := os.MkdirAll(roles, 0o755); err != nil {
		t.Fatal(err)
	}
	builder := "# 作る\n\n## 作り方\n\n- ほかのプロジェクトでは、作る前に見積もりを書く。\n"
	if err := os.WriteFile(filepath.Join(roles, "builder.yaml"), []byte(builder), 0o644); err != nil {
		t.Fatal(err)
	}
	return copyFixture{fixture: f, other: other}
}

// twoProjects is the headquarters reading that lists both projects: the serve's
// own, and the other one.
func (c copyFixture) twoProjects(t *testing.T) *Server {
	t.Helper()
	server := c.server(t)
	otherCat := catalog.New(c.global, filepath.Join(c.other, ".hergy"), projectPath(c.other), source.Disk{}, func(path string) string { return path }).InRepo(true)
	hq := fakeHQ{
		projects: HQProjects{
			Mode:    "listed",
			Current: HQProject{Path: projectPath(c.proj), Name: "proj", Listed: true},
			Projects: []HQProject{
				{Path: projectPath(c.proj), Name: "proj", Listed: true},
				{Path: projectPath(c.other), Name: "other", Listed: true},
			},
			Global: HQGlobal{Path: c.global, Display: "~/.config/hergy"},
		},
		cats:   map[domainplace.Project]*catalog.Catalog{projectPath(c.proj): c.catalog(), projectPath(c.other): otherCat},
		global: catalog.New(c.global, filepath.Join(c.global, "no-local"), projectPath(c.proj), source.Disk{}, func(p string) string { return p }),
	}
	server.hq = hq
	server.projects = hq
	return server
}

// post sends one write with the server's own token, and answers its status and
// what it said.
func post(t *testing.T, server *Server, target, body string) (int, string) {
	t.Helper()
	server.addr = "127.0.0.1:8787"
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hergy-Token", server.token)
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	said, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, string(said)
}

// TestCopyReadsTheBaseFromItsOwnProject: a copy of another listed project's
// definition answers the draft its save would write, and that draft stands in
// the global layer rather than in the project serve was started in.
func TestCopyReadsTheBaseFromItsOwnProject(t *testing.T) {
	c := newCopyFixture(t)
	server := c.twoProjects(t)
	body := `{"place":"global","name":"other-dev","part":"team","doc":{"ops":[]},"base":{"place":"project","name":"other-dev","project":` +
		strconvQuote(c.other) + `}}`
	status, said := post(t, server, "/api/team/check", body)
	if status != http.StatusOK {
		t.Fatalf("check of a copy answers %d: %s", status, said)
	}
	var got struct {
		Problems []apiProblem   `json:"problems"`
		Draft    *apiCheckDraft `json:"draft"`
		Doc      *apiTeamDoc    `json:"doc"`
	}
	if err := json.Unmarshal([]byte(said), &got); err != nil {
		t.Fatal(err)
	}
	if got.Draft == nil {
		t.Fatal("the copy answered no draft, so its save could not be pressed")
	}
	if want := filepath.Join(c.global, "teams", "other-dev", "team.yaml"); got.Draft.Path != want {
		t.Errorf("the copy would write %q, want %q", got.Draft.Path, want)
	}
	// The row stands on the other project's description, which is the file the
	// base names.
	if got.Doc == nil || got.Doc.Description != "ほかのプロジェクトの定義" {
		t.Errorf("the copy stands on %+v, want the other project's definition", got.Doc)
	}
	// The copy is read as the global layer holds it: the role and the profile
	// the project holds are what it can bring, and the sentences say where they
	// are rather than that there is none (r-7e4a).
	if got.Draft.Carry == nil {
		t.Fatal("the copy of a project's definition answers no carry")
	}
	if want := []string{"lead"}; !slices.Equal(got.Draft.Carry.Roles, want) {
		t.Errorf("the copy can bring roles %v, want %v", got.Draft.Carry.Roles, want)
	}
	if want := []string{"planner"}; !slices.Equal(got.Draft.Carry.Profiles, want) {
		t.Errorf("the copy can bring profiles %v, want %v", got.Draft.Carry.Profiles, want)
	}
	for _, problem := range got.Problems {
		if strings.Contains(problem.Text, "というプロファイルはありません") || strings.Contains(problem.Text, "文書が見つかりません") {
			t.Errorf("the problem does not say where the name is: %q", problem.Text)
		}
	}
}

// TestCopyWithoutItsProjectSaysWhy: a copy whose base cannot be read answers the
// sentence that says what happened and how to fix it, rather than the sentence
// about a missing screen (r-a028, ac-456e).
func TestCopyWithoutItsProjectSaysWhy(t *testing.T) {
	c := newCopyFixture(t)
	server := c.twoProjects(t)
	// No project: the base is read from serve's own project, which holds no
	// other-dev.
	body := `{"place":"global","name":"other-dev","part":"team","doc":{"ops":[]},"base":{"place":"project","name":"other-dev"}}`
	status, said := post(t, server, "/api/team/check", body)
	if status != http.StatusNotFound {
		t.Fatalf("check of a copy without its project answers %d: %s", status, said)
	}
	var refused struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(said), &refused); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(refused.Text, "元にしたチーム定義の") || !strings.Contains(refused.Text, "もう一度") {
		t.Errorf("the refusal says %q, want what could not be read and how to fix it", refused.Text)
	}
	// The file is named once, by its path: the sentence does not say team.yaml
	// twice (r-a028).
	if n := strings.Count(refused.Text, "team.yaml"); n > 1 {
		t.Errorf("the refusal names team.yaml %d times: %q", n, refused.Text)
	}
	if strings.Contains(refused.Text, "その画面はありません") {
		t.Errorf("the refusal tells the person about a screen that has nothing to do with it: %q", refused.Text)
	}
}

// strconvQuote is one string as a JSON string, for a body a test writes.
func strconvQuote(text string) string {
	one, _ := json.Marshal(text)
	return string(one)
}

// put sends one save with the server's own token, and answers its status and
// what it said.
func put(t *testing.T, server *Server, target, body string) (int, string) {
	t.Helper()
	server.addr = "127.0.0.1:8787"
	req := httptest.NewRequest(http.MethodPut, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hergy-Token", server.token)
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	said, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, string(said)
}

// TestRoleCopyReadsTheBaseFromItsOwnProject: a copy of another listed project's
// role stands on that project's document, so the lines and the comments the
// person there wrote are the ones the copy keeps (r-a028).
func TestRoleCopyReadsTheBaseFromItsOwnProject(t *testing.T) {
	c := newCopyFixture(t)
	server := c.twoProjects(t)
	text := "# 作る\n\nほかのプロジェクトだけが書いているメモ\nほかのプロジェクトでは、作る前に見積もりを書く。\n"
	doc := `{"name":"builder","title":"作る","text":` + strconvQuote(text) + `}`
	body := `{"place":"global","name":"builder","doc":` + doc + `,"base":{"place":"project","name":"builder","project":` + strconvQuote(c.other) + `}}`
	status, said := put(t, server, "/api/role/builder", body)
	if status != http.StatusOK {
		t.Fatalf("save of a role copy answers %d: %s", status, said)
	}
	written := readFile(t, filepath.Join(c.global, "roles", "builder.md"))
	if !strings.Contains(written, "ほかのプロジェクトだけが書いているメモ") {
		t.Errorf("the copy lost the line the source carries, so it was written from another project's document:\n%s", written)
	}
	if !strings.Contains(written, "ほかのプロジェクトでは、作る前に見積もりを書く。") {
		t.Errorf("the copy does not hold the source's rule:\n%s", written)
	}
}

// TestCopyCarriesOnlyWhatThePersonChose: a copy of a project's definition into
// the global layer writes the definition alone; a profile or a role the person
// chose is written with it, as the project it came from holds it, and a name the
// global layer already holds is not written over (r-7e4a, d-dc81).
func TestCopyCarriesOnlyWhatThePersonChose(t *testing.T) {
	c := newCopyFixture(t)
	server := c.twoProjects(t)

	// The other project's definition names its own profile and the role only it
	// holds: both are what the person may bring to the global layer.
	builder := "# 作る\n\n## 作り方\n\n- ほかのプロジェクトでは、作る前に見積もりを書く。\n"
	profiles := "version: 1\nprofiles:\n  other-fast:\n    kind: claude\n    args: [\"--model\", \"claude-haiku-5-5\"]\n    env:\n      A: one\n"
	// The definition names the two, so the copy is what may bring them.
	team := "version: 1\ndescription: \"ほかのプロジェクトの定義\"\nmembers:\n  - name: lead\n    role: builder\n    profile: other-fast\nturns:\n  - human -> lead\n  - lead -> human\n"
	writeUnder(t, c.other, ".hergy/roles/builder.md", builder)
	writeUnder(t, c.other, ".hergy/profiles.yaml", profiles)
	writeUnder(t, c.other, ".hergy/teams/other-dev/team.yaml", team)

	base := `"base":{"place":"project","name":"other-dev","project":` + strconvQuote(c.other) + `}`
	check := `{"place":"global","name":"other-dev","part":"team","doc":{"ops":[]},` + base + `}`
	status, said := post(t, server, "/api/team/check", check)
	if status != http.StatusOK {
		t.Fatalf("check of a copy answers %d: %s", status, said)
	}
	var got struct {
		Problems []apiProblem `json:"problems"`
		Draft    struct {
			Carry *apiCheckCarry `json:"carry"`
		} `json:"draft"`
	}
	if err := json.Unmarshal([]byte(said), &got); err != nil {
		t.Fatal(err)
	}
	if got.Draft.Carry == nil {
		t.Fatal("the copy of a project's definition answers no carry")
	}
	if want := []string{"builder"}; !slices.Equal(got.Draft.Carry.Roles, want) {
		t.Errorf("the copy can bring roles %v, want %v", got.Draft.Carry.Roles, want)
	}
	if want := []string{"other-fast"}; !slices.Equal(got.Draft.Carry.Profiles, want) {
		t.Errorf("the copy can bring profiles %v, want %v", got.Draft.Carry.Profiles, want)
	}
	// The problem says where the profile is, rather than that there is none.
	text := problemTextOf(got.Problems, "members[0].profile")
	if !strings.Contains(text, "グローバルにありません") || !strings.Contains(text, ".hergy にあります") {
		t.Errorf("the profile's sentence says %q, want the global layer and the project named", text)
	}

	// Nothing chosen: the global layer's profiles and roles are as they were.
	globalProfiles := filepath.Join(c.global, "profiles.yaml")
	globalRole := filepath.Join(c.global, "roles", "builder.md")
	wasProfiles, hadProfiles := readFileIfThere(t, globalProfiles)
	save := `{"place":"global","name":"other-dev","part":"folder","doc":{"ops":[]},` + base + `}`
	if status, said := put(t, server, "/api/team/other-dev", save); status != http.StatusUnprocessableEntity {
		t.Fatalf("copy with unchosen required references answers %d, want 422: %s", status, said)
	}
	if now, _ := readFileIfThere(t, globalProfiles); now != wasProfiles {
		t.Errorf("a copy a person chose nothing for wrote profiles.yaml:\n%s", now)
	}
	if _, there := readFileIfThere(t, globalRole); there {
		t.Error("a copy a person chose nothing for wrote the role document")
	}
	if _, err := os.Stat(filepath.Join(c.global, "teams", "other-dev", "team.yaml")); !os.IsNotExist(err) {
		t.Errorf("refused copy wrote a definition: %v", err)
	}
	_ = hadProfiles

	// The same copy again, with the two names chosen: they are written, the
	// definition alone is replaced, and the profile keeps the project's own words.
	if err := os.RemoveAll(filepath.Join(c.global, "teams", "other-dev")); err != nil {
		t.Fatal(err)
	}
	saveCarry := `{"place":"global","name":"other-dev","part":"folder","carry_roles":["builder"],"carry_profiles":["other-fast"],"doc":{"ops":[]},` + base + `}`
	if status, said := put(t, server, "/api/team/other-dev", saveCarry); status != http.StatusOK {
		t.Fatalf("save of a copy with what the person chose answers %d: %s", status, said)
	}
	written, _ := readFileIfThere(t, globalProfiles)
	if !strings.Contains(written, "other-fast:") || !strings.Contains(written, "claude-haiku-5-5") || !strings.Contains(written, "A: one") {
		t.Errorf("the carried profile is not the project's own:\n%s", written)
	}
	if role, _ := readFileIfThere(t, globalRole); role != builder {
		t.Errorf("the carried role document is %q, want the project's own", role)
	}

	// A name the global layer already holds is not written over.
	writeUnder(t, c.global, "roles/builder.md", "version: 1\ntitle: グローバルのもの\nsections: []\n")
	if err := os.RemoveAll(filepath.Join(c.global, "teams", "other-dev")); err != nil {
		t.Fatal(err)
	}
	if status, said := put(t, server, "/api/team/other-dev", saveCarry); status != http.StatusOK {
		t.Fatalf("save over a name the global layer holds answers %d: %s", status, said)
	}
	if role, _ := readFileIfThere(t, globalRole); role != "version: 1\ntitle: グローバルのもの\nsections: []\n" {
		t.Errorf("the copy wrote over the global layer's own role document: %q", role)
	}
}

// writeUnder writes one file under a project's .hergy.
func writeUnder(t *testing.T, project, rel, text string) {
	t.Helper()
	path := filepath.Join(project, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readFileIfThere is one file as written, and whether it is there.
func readFileIfThere(t *testing.T, path string) (string, bool) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(text), true
}

// problemTextOf is the sentence one problem carries, by where it stands.
func problemTextOf(problems []apiProblem, at string) string {
	for _, one := range problems {
		if one.At == at {
			return one.Text
		}
	}
	return ""
}
