package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	agree := "version: 1\nsections:\n  - rules:\n      - id: OTHER-1\n        text: ほかのプロジェクトのチームのルール。\n"
	if err := os.WriteFile(filepath.Join(dir, "agreement.yaml"), []byte(agree), 0o644); err != nil {
		t.Fatal(err)
	}
	// The other project's builder says something this project's builder does
	// not: a copy that reads the wrong file writes the copy from the wrong
	// lines.
	roles := filepath.Join(other, ".hergy", "roles")
	if err := os.MkdirAll(roles, 0o755); err != nil {
		t.Fatal(err)
	}
	builder := "# ほかのプロジェクトだけが書いているメモ\nversion: 1\ntitle: 作る\nsections:\n  - heading: 作り方\n    rules:\n      - id: DAAA\n        text: ほかのプロジェクトでは、作る前に見積もりを書く。\n"
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
	otherCat := catalog.New(c.global, filepath.Join(c.other, ".hergy"), c.other, source.Disk{}, func(path string) string { return path }).InRepo(true)
	hq := fakeHQ{
		projects: HQProjects{
			Mode:    "listed",
			Current: HQProject{Path: c.proj, Name: "proj", Listed: true},
			Projects: []HQProject{
				{Path: c.proj, Name: "proj", Listed: true},
				{Path: c.other, Name: "other", Listed: true},
			},
			Global: HQGlobal{Path: c.global, Display: "~/.config/hergy"},
		},
		cats:   map[string]*catalog.Catalog{c.proj: c.catalog(), c.other: otherCat},
		global: catalog.New(c.global, filepath.Join(c.global, "no-local"), c.proj, source.Disk{}, func(p string) string { return p }),
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
	if len(got.Problems) != 0 {
		t.Errorf("the copy of a sound definition has problems: %+v", got.Problems)
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
	doc := `{"name":"builder","title":"作る","sections":[{"heading":"作り方","level":2,"intro":"",` +
		`"rules":[{"from":{"section":0,"rule":0},"id":"DAAA","text":"ほかのプロジェクトでは、作る前に見積もりを書く。","in_prompt":true,"note":""}]}]}`
	body := `{"place":"global","name":"builder","doc":` + doc + `,"base":{"place":"project","name":"builder","project":` + strconvQuote(c.other) + `}}`
	status, said := put(t, server, "/api/role/builder", body)
	if status != http.StatusOK {
		t.Fatalf("save of a role copy answers %d: %s", status, said)
	}
	written := readFile(t, filepath.Join(c.global, "roles", "builder.yaml"))
	if !strings.Contains(written, "ほかのプロジェクトだけが書いているメモ") {
		t.Errorf("the copy lost the line the source carries, so it was written from another project's document:\n%s", written)
	}
	if !strings.Contains(written, "ほかのプロジェクトでは、作る前に見積もりを書く。") {
		t.Errorf("the copy does not hold the source's rule:\n%s", written)
	}
}
