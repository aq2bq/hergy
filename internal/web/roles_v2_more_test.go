package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
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

	// reviewer is documented (config/hergy/roles/reviewer.yaml), so the save
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

// TestROLES_P18_TheYAMLTabWritesTheTypedTextAndRefusesANewProblem: a save of the
// YAML tab's whole text writes exactly what the person typed, even when the file
// already has a problem, and only refuses a text that adds a problem
// (ROLES-P18).
func TestROLES_P18_TheYAMLTabWritesTheTypedTextAndRefusesANewProblem(t *testing.T) {
	f := rbFixtureRoles(t)
	server := f.server(t)

	// A document that already does not read: an unknown key beside a rule's
	// text is an error rules.ReadAll — and so saveYAML — knows about.
	legacy := "version: 1\nsections:\n  - heading: 仕事\n    rules:\n      - id: LEG-1\n        text: 何かをする。\n        nope: 1\n"
	f.write(t, filepath.Join("roles", "legacy.yaml"), legacy)
	path := filepath.Join(f.proj, ".hergy", "roles", "legacy.yaml")
	if _, problems := rules.ReadAll(legacy, path); len(problems) == 0 {
		t.Fatalf("the fixture document has no problem to keep")
	}

	// The typed text still holds the same problem, so nothing new appeared and
	// the text is written as it is.
	typed := strings.Replace(legacy, "何かをする。", "別のことをする。", 1)
	resp, body := f.api(t, server, http.MethodPut, "/api/role/legacy", map[string]any{
		"place": "project", "name": "legacy", "hash": source.Hash(legacy), "text": typed,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/role/legacy answers %d, want 200: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); got != typed {
		t.Errorf("the save wrote %q, want the typed text %q", got, typed)
	}

	// A text that adds a problem is refused and leaves the file as it was.
	grown := strings.Replace(typed, "        nope: 1\n", "        nope: 1\n        wrong: 2\n", 1)
	resp, body = f.api(t, server, http.MethodPut, "/api/role/legacy", map[string]any{
		"place": "project", "name": "legacy", "hash": source.Hash(typed), "text": grown,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a text that adds a problem answers %d, want 422: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); got != typed {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// TestROLES_V4_TheCheckAnswerIsTheSentenceValidateWrites: what the screen's
// check answers for the fixture's role documents are the sentences hergy
// validate writes — the same check.Definition findings and the same size
// warnings — with only the definition's own head in front of a turn or size
// warning, so the screen adds no check of its own; and BUILD-3, BUILD-4,
// core-analyst's amount and BUILD-5 each stand on their rule and line
// (ROLES-V4).
func TestROLES_V4_TheCheckAnswerIsTheSentenceValidateWrites(t *testing.T) {
	f := rbFixtureRoles(t)
	server := f.server(t)
	cat := f.catalog()

	defs := make([]string, 0, 4)
	for _, listing := range cat.Definitions() {
		defs = append(defs, listing.Name)
	}

	// Read one answer per role row, at the place its document stands.
	type row struct {
		name, place string
		text        string
		problems    []apiProblem
		readers     []check.RoleReader
	}
	rows := []row{
		{"analyst", "project", "", nil, nil},
		{"builder", "project", "", nil, nil},
		{"lead", "project", "", nil, nil},
		{"scout", "project", "", nil, nil},
		{"reviewer", "global", "", nil, nil},
		{"writer", "global", "", nil, nil},
	}
	for i := range rows {
		dir := filepath.Join(f.proj, ".hergy", "roles")
		if rows[i].place == "global" {
			dir = filepath.Join(f.global, "roles")
		}
		rows[i].text = readFile(t, filepath.Join(dir, rows[i].name+".yaml"))
		resp, body := f.api(t, server, http.MethodPost, "/api/role/check", map[string]any{
			"place": rows[i].place, "name": rows[i].name, "text": rows[i].text,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the check of %s answers %d: %s", rows[i].name, resp.StatusCode, body)
		}
		var answer struct {
			Problems []apiProblem       `json:"problems"`
			Readers  []check.RoleReader `json:"readers"`
		}
		if err := json.Unmarshal([]byte(body), &answer); err != nil {
			t.Fatalf("the check of %s is not JSON: %v: %s", rows[i].name, err, body)
		}
		rows[i].problems, rows[i].readers = answer.Problems, answer.Readers
	}

	for _, one := range rows {
		if one.name == "analyst" {
			// The spec names core-analyst's amount, so pin the number the
			// shared check.SizeOf hands it: the sentence below is not read from
			// itself.
			read := false
			for _, reader := range one.readers {
				if reader.Definition != "core" || reader.Member != "core-analyst" {
					continue
				}
				read = true
				if reader.Rules != 21 || !reader.Over {
					t.Errorf("core-analyst is handed %d rules (over=%v), want 21, over the limit", reader.Rules, reader.Over)
				}
			}
			if !read {
				t.Errorf("the analyst document names no core-analyst reader: %+v", one.readers)
			}
		}
		// What hergy validate writes for this document: the findings of every
		// definition that reads it, and the warning of every reader over a
		// provisional limit.
		want := map[string]bool{}
		display := cat.Display(cat.RolePath(one.place, one.name))
		seen := map[string]bool{}
		for _, ref := range cat.RoleUsers(one.name) {
			if seen[ref.Definition] {
				continue
			}
			seen[ref.Definition] = true
			for _, finding := range check.Definition(check.Input{Catalog: cat, Name: ref.Definition}) {
				if strings.Contains(finding.Text, display) || strings.Contains(finding.Field, display) {
					want[rbValidateSentence(finding.Text, defs)] = true
				}
			}
		}
		for _, reader := range one.readers {
			if !reader.Over {
				continue
			}
			if reader.Rules > rules.MemberRuleLimit {
				want[rbValidateSentence(reader.Definition+": "+check.MemberRulesWarning(reader.Member, reader.Rules), defs)] = true
			}
			if reader.Chars > rules.MemberCharLimit {
				want[rbValidateSentence(reader.Definition+": "+check.MemberCharsWarning(reader.Member, reader.Chars), defs)] = true
			}
		}
		got := map[string]int{}
		for _, problem := range one.problems {
			got[rbValidateSentence(problem.Text, defs)]++
		}
		for sentence := range got {
			if !want[sentence] {
				t.Errorf("the check of %s shows a sentence validate does not write: %q", one.name, sentence)
			}
		}
		for sentence := range want {
			if got[sentence] == 0 {
				t.Errorf("the check of %s does not show validate's sentence: %q", one.name, sentence)
			}
		}
	}

	// The four the spec names, each on its rule and line.
	for _, want := range []struct {
		role, id, sev string
		section, rule int
		line          int
	}{
		{"builder", "BUILD-3", "warn", 0, 2, 21},
		{"builder", "BUILD-4", "err", 0, 3, 26},
		{"builder", "BUILD-5", "warn", 1, 0, 30},
		{"analyst", "", "warn", -1, -1, 1},
	} {
		var one row
		for _, candidate := range rows {
			if candidate.name == want.role {
				one = candidate
			}
		}
		var found *apiProblem
		for i := range one.problems {
			if one.problems[i].Section == want.section && one.problems[i].Rule == want.rule {
				found = &one.problems[i]
			}
		}
		if found == nil {
			t.Errorf("the check of %s does not stand a problem on (%d, %d)", want.role, want.section, want.rule)
			continue
		}
		if found.Sev != want.sev {
			t.Errorf("%s (%d, %d) is %q, want %q", want.role, want.section, want.rule, found.Sev, want.sev)
		}
		if found.Line != want.line {
			t.Errorf("%s (%d, %d) stands on line %d, want %d", want.role, want.section, want.rule, found.Line, want.line)
		}
		if want.id == "" {
			continue
		}
		doc, _ := rules.ReadAll(one.text, cat.Display(cat.RolePath(one.place, one.name)))
		if doc == nil || want.section >= len(doc.Sections) || want.rule >= len(doc.Sections[want.section].Rules) {
			t.Errorf("%s has no rule at (%d, %d)", want.role, want.section, want.rule)
			continue
		}
		if id := doc.Sections[want.section].Rules[want.rule].ID; id != want.id {
			t.Errorf("%s (%d, %d) is rule %s, want %s", want.role, want.section, want.rule, id, want.id)
		}
	}
}

// rbValidateSentence is one sentence as both places write it without the names a
// screen's own head already gives: the head naming the definition a turn or size
// warning is about, and any leading file path a check.Definition sentence
// carries. What is left is the sentence hergy validate writes.
func rbValidateSentence(text string, defs []string) string {
	changed := true
	for changed {
		changed = false
		for _, def := range defs {
			if after, ok := strings.CutPrefix(text, def+": "); ok {
				text, changed = after, true
			}
		}
		if head, rest, ok := strings.Cut(text, ": "); ok && strings.Contains(head, "/") {
			text, changed = rest, true
		}
	}
	return text
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
	if got := list.Items[at].Places["project"].Path; got != ".hergy/roles/builder.yaml" {
		t.Errorf("builder's document reads %q, want .hergy/roles/builder.yaml", got)
	}
	at, ok = list.row("reviewer")
	if !ok {
		t.Fatalf("the list does not hold reviewer: %s", body)
	}
	if got := list.Items[at].Places["global"].Path; got != "~/.config/hergy/roles/reviewer.yaml" {
		t.Errorf("reviewer's document reads %q, want ~/.config/hergy/roles/reviewer.yaml", got)
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
	if one.Path != ".hergy/roles/builder.yaml" {
		t.Errorf("one role's file reads %q, want .hergy/roles/builder.yaml", one.Path)
	}
	rbNoAbsolutePath(t, "the one role", body, f.proj, f.global, f.root)

	// 検査: a text that repeats one of the shared team's rule ids is told with
	// both files written as a person reads them, not as an absolute path.
	builder := readFile(t, filepath.Join(f.proj, ".hergy", "roles", "builder.yaml"))
	draft := builder + "      - id: CORE-1\n        text: 重ねて書く。\n"
	resp, body = f.api(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": "project", "name": "builder", "text": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
	}
	var answer struct {
		Problems []apiProblem `json:"problems"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		t.Fatalf("the check is not JSON: %v: %s", err, body)
	}
	told := false
	for _, problem := range answer.Problems {
		if strings.Contains(problem.Text, ".hergy/teams/core/agreement.yaml") && strings.Contains(problem.Text, ".hergy/roles/builder.yaml") {
			told = true
		}
	}
	if !told {
		t.Errorf("the check does not name both files as a person reads them: %+v", answer.Problems)
	}
	rbNoAbsolutePath(t, "the check", body, f.proj, f.global, f.root)
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
		"version: 1\nmission: 幽霊の確認\nmembers:\n  - name: ghost\n    role: nosuch\n    profile: sonnet-fast\nturns:\n  - human -> ghost\n")
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
	if want := tf("error.role_markdown", "mdonly", f.catalog().Display(filepath.Join(f.proj, ".hergy", "roles", "mdonly.md"))); markdown.Summary.Markdown != want {
		t.Errorf("the Markdown-only row says %q, want %q", markdown.Summary.Markdown, want)
	}
	if len(markdown.Places) != 0 {
		t.Errorf("the Markdown-only row carries a document a screen can edit: %+v", markdown.Places)
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
	if len(place.Doc.Sections) != 1 || place.Doc.Sections[0].Heading != "使命" {
		t.Fatalf("the new role's document is %+v, want one section 使命", place.Doc.Sections)
	}
	if n := len(place.Doc.Sections[0].Rules); n != 1 {
		t.Fatalf("the new role holds %d rules, want one empty rule", n)
	}
	if rule := place.Doc.Sections[0].Rules[0]; rule.ID != "" || rule.Text != "" {
		t.Errorf("the new role's one rule is %+v, want it empty", rule)
	}
}
