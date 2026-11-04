package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
)

// rbFixtureRoles copies the role screen's real-scale fixture (testdata/roles-v2)
// into a working project and an outside settings directory, the way a person's
// project stands, and answers a fixture whose server reads it. It is this
// file's own copier so two tests in the package cannot fight over one name.
func rbFixtureRoles(t *testing.T) *fixture {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "roles-v2")
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config")
	copyTree(t, filepath.Join(src, "project", ".hergy"), filepath.Join(proj, ".hergy"))
	copyTree(t, filepath.Join(src, "config", "hergy"), global)
	return &fixture{root: root, proj: proj, global: global}
}

// rbRoleList is the role list answer as these tests read it: one row per name,
// the places behind it (each with the path a person reads and the document a
// screen edits), and the columns only a document can answer.
type rbRoleList struct {
	Items []struct {
		Name    string                 `json:"name"`
		Places  map[string]rbRolePlace `json:"places"`
		Summary rbRoleSummary          `json:"summary"`
	} `json:"items"`
}

// rbRolePlace is one place's reading of one role row.
type rbRolePlace struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Doc  struct {
		Text     string `json:"text"`
		Sections []struct {
			Heading string `json:"heading"`
			Rules   []struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			} `json:"rules"`
		} `json:"sections"`
	} `json:"doc"`
}

// rbRoleSummary is the row's own columns: the sentence a Markdown-only role
// carries, and whether the row is one no document holds yet.
type rbRoleSummary struct {
	Markdown string `json:"markdown"`
	NoDoc    bool   `json:"noDoc"`
	IsNew    bool   `json:"isNew"`
}

// rbRoleRow finds one row of the list answer by name.
func (l rbRoleList) row(name string) (int, bool) {
	for i, item := range l.Items {
		if item.Name == name {
			return i, true
		}
	}
	return -1, false
}

// TestROLES_P16_TheAssignmentSaveTouchesOneRoleLineAndRefusesTheUndocumented: a
// save from the 割り当て tab that changes the docs team's checker role writes
// only that role: line of team.yaml, and a save that names a role no layer holds
// a document for is refused with error.no_role_doc and writes nothing
// (ROLES-P16).
func TestROLES_P16_TheAssignmentSaveTouchesOneRoleLineAndRefusesTheUndocumented(t *testing.T) {
	f := rbFixtureRoles(t)
	server := f.server(t)
	cat := f.catalog()
	path := filepath.Join(f.proj, ".hergy", "teams", "docs", "team.yaml")
	before := readFile(t, path)

	// reviewer is documented (config/hergy/roles/reviewer.md), so the save
	// goes through and moves the checker's role: line alone.
	resp, body := f.api(t, server, http.MethodPut, "/api/team/docs", map[string]any{
		"place": "project",
		"hash":  source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "memberRole", "member": "checker", "role": "writer"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/docs answers %d, want 200: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	beforeLines, afterLines := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("the save moved lines around: %d lines became %d", len(beforeLines), len(afterLines))
	}
	var moved []int
	for i := range beforeLines {
		if beforeLines[i] != afterLines[i] {
			moved = append(moved, i)
		}
	}
	if len(moved) != 1 {
		t.Fatalf("the save changed %d lines, want only the checker's role: line: %v", len(moved), moved)
	}
	line := moved[0]
	if beforeLines[line] != "    role: reviewer" || afterLines[line] != "    role: writer" {
		t.Errorf("the changed line is %q -> %q, want the checker's role: line", beforeLines[line], afterLines[line])
	}

	// nosuch is documented nowhere: the save is refused with the sentence that
	// names both the member and the places the role was looked for.
	resp, body = f.api(t, server, http.MethodPut, "/api/team/docs", map[string]any{
		"place": "project",
		"hash":  source.Hash(after),
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "memberRole", "member": "checker", "role": "nosuch"},
		}},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a save that names a role with no document answers %d, want 422: %s", resp.StatusCode, body)
	}
	places := make([]string, 0, 2)
	for _, place := range cat.RolePlaces("nosuch") {
		places = append(places, cat.Display(place))
	}
	want := tf("error.no_role_doc", "checker", "nosuch", strings.Join(places, i18n.T("list.separator")))
	if !strings.Contains(body, want) {
		t.Errorf("the refusal does not say %q:\n%s", want, body)
	}
	if got := readFile(t, path); got != after {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// rbHomeServer builds the fixture's working project with the shared settings
// under the home directory (a fake home and XDG_CONFIG_HOME), and the server the
// app would build for it, so a path reads the way a person sees it (X1). The
// machine's own config is never read.
func rbHomeServer(t *testing.T) (*Server, *fixture) {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "roles-v2")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	proj := filepath.Join(root, "project")
	global := filepath.Join(home, ".config", "hergy")
	copyTree(t, filepath.Join(src, "project", ".hergy"), filepath.Join(proj, ".hergy"))
	copyTree(t, filepath.Join(src, "config", "hergy"), global)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Chdir(proj)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env.ProjectRoot != proj {
		t.Fatalf("the environment's project root is %q, want the working project %q", env.ProjectRoot, proj)
	}
	server, err := New(env.Catalog(source.Disk{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	return server, &fixture{root: root, proj: proj, global: global}
}

// rbNoAbsolutePath fails while one answer writes one of the working
// directories' own absolute paths.
func rbNoAbsolutePath(t *testing.T, what, body string, abs ...string) {
	t.Helper()
	for _, one := range abs {
		if one != "" && strings.Contains(body, one) {
			t.Errorf("%s writes the absolute path %q:\n%s", what, one, body)
		}
	}
}

// TestROLES_V6_ThePathsReadAsTheProjectAndTheHome: with the shared settings
// under ~/.config/hergy, the list, one document and a check's answer write every
// file as .hergy/… or ~/.config/hergy/…, and the working directory's absolute
// path is nowhere (ROLES-V6).
func TestROLES_V6_ThePathsReadAsTheProjectAndTheHome(t *testing.T) {
	server, f := rbHomeServer(t)

	// 一覧: one row per name, the project's document relative to the project and
	// the shared one after ~/.
	resp, body := f.api(t, server, http.MethodGet, "/api/role", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/role answers %d: %s", resp.StatusCode, body)
	}
	var list rbRoleList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("the list is not JSON: %v: %s", err, body)
	}
	at, ok := list.row("builder")
	if !ok {
		t.Fatalf("the list does not hold builder: %s", body)
	}
	if got := list.Items[at].Places["project"].Path; got != ".hergy/roles/builder.md" {
		t.Errorf("builder's document reads %q, want .hergy/roles/builder.md", got)
	}
	at, ok = list.row("reviewer")
	if !ok {
		t.Fatalf("the list does not hold reviewer: %s", body)
	}
	if got := list.Items[at].Places["global"].Path; got != "~/.config/hergy/roles/reviewer.md" {
		t.Errorf("reviewer's document reads %q, want ~/.config/hergy/roles/reviewer.md", got)
	}
	for _, item := range list.Items {
		for _, place := range item.Places {
			if !strings.HasPrefix(place.Path, ".hergy/") && !strings.HasPrefix(place.Path, "~/.config/hergy/") {
				t.Errorf("the list writes %q, want a .hergy/… or ~/.config/hergy/… path", place.Path)
			}
		}
	}
	rbNoAbsolutePath(t, "the list", body, f.proj, f.global, f.root)

	// 1 件: one document's own reading names its file the same way.
	resp, body = f.api(t, server, http.MethodGet, "/api/role/builder?place=project", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET one role answers %d: %s", resp.StatusCode, body)
	}
	var one struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(body), &one); err != nil {
		t.Fatalf("the one role is not JSON: %v: %s", err, body)
	}
	if one.Path != ".hergy/roles/builder.md" {
		t.Errorf("one role's file reads %q, want .hergy/roles/builder.md", one.Path)
	}
	rbNoAbsolutePath(t, "the one role", body, f.proj, f.global, f.root)

}

// TestROLES_E1_ARowIsReturnedForAMarkdownOnlyRoleAndForANoDocumentRole: a role
// whose document is still Markdown, and a role a team member names and no layer
// holds a document for, are both rows of the list: the first carries the
// sentence that says it is Markdown and no document a screen can edit, and the
// second is offered as 新規 with one empty rule (ROLES-E1).
func TestROLES_E1_ARowIsReturnedForAMarkdownOnlyRoleAndForANoDocumentRole(t *testing.T) {
	f := rbFixtureRoles(t)
	// A role document still left as Markdown, and a definition that names a
	// role no layer holds a document for.
	f.write(t, filepath.Join("roles", "mdonly.md"), "# mdonly\n\n- 何かをする。\n")
	f.write(t, filepath.Join("teams", "ghost", "team.yaml"),
		"version: 1\ndescription: 幽霊の確認\nmembers:\n  - name: ghost\n    role: nosuch\n    profile: sonnet-fast\nturns:\n  - human -> ghost\n")
	server := f.server(t)

	resp, body := f.api(t, server, http.MethodGet, "/api/role", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/role answers %d: %s", resp.StatusCode, body)
	}
	var list rbRoleList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("the list is not JSON: %v: %s", err, body)
	}

	at, ok := list.row("mdonly")
	if !ok {
		t.Fatalf("the list does not hold the Markdown-only role: %s", body)
	}
	markdown := list.Items[at]
	if markdown.Summary.Markdown != "" || markdown.Places["project"].Doc.Text == "" {
		t.Fatalf("Markdown role is not editable: %+v", markdown)
	}

	at, ok = list.row("nosuch")
	if !ok {
		t.Fatalf("the list does not hold the role with no document: %s", body)
	}
	fresh := list.Items[at]
	if !fresh.Summary.NoDoc || !fresh.Summary.IsNew {
		t.Errorf("the role with no document is not offered as new: %+v", fresh.Summary)
	}
	place, ok := fresh.Places["project"]
	if !ok {
		t.Fatalf("the new role does not stand in the project: %+v", fresh.Places)
	}
	if !strings.Contains(place.Doc.Text, "## 責務") {
		t.Fatalf("new role: %+v", place.Doc)
	}
}
