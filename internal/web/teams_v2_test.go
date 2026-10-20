package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// These are the team kind's own promises (r-0f8c, spec-boundary 5): one kind
// under /api/team whose part says which of a definition's two documents a
// payload is about, the figure's check and effect, the agreement checked through
// the figure's draft, and the YAML tab's checks and writes.

// t2TestProblem is one problem as the boundary carries it.
type t2TestProblem struct {
	Sev  string `json:"sev"`
	At   string `json:"at"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// t2TestCheck is one check answer.
type t2TestCheck struct {
	Problems []t2TestProblem `json:"problems"`
	Preview  struct {
		Lines []struct {
			Text string `json:"text"`
		} `json:"lines"`
		Chars int `json:"chars"`
	} `json:"preview"`
	Effect struct {
		Lines []struct {
			Member  string   `json:"member"`
			Part    string   `json:"part"`
			Removed []string `json:"removed"`
		} `json:"lines"`
	} `json:"effect"`
	Readers []struct {
		Member string `json:"member"`
	} `json:"readers"`
}

// t2TestFile is one file of the team YAML tab.
type t2TestFile struct {
	Place    string          `json:"place"`
	Label    string          `json:"label"`
	Path     string          `json:"path"`
	Text     string          `json:"text"`
	Hash     string          `json:"hash"`
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Hidden   bool            `json:"hidden"`
	Problems []t2TestProblem `json:"problems"`
}

// t2TestItem is one team definition of the list.
type t2TestItem struct {
	Name      string                  `json:"name"`
	Effective string                  `json:"effective"`
	Hidden    []string                `json:"hidden"`
	Problems  struct{ Err, Warn int } `json:"problems"`
	Summary   struct {
		Mission string `json:"mission"`
		Members []struct {
			Name    string `json:"name"`
			Role    string `json:"role"`
			Profile string `json:"profile"`
		} `json:"members"`
		Workers  int                        `json:"workers"`
		Rules    int                        `json:"rules"`
		Excluded int                        `json:"excluded"`
		Amount   struct{ Rules, Chars int } `json:"amount"`
	} `json:"summary"`
	Places map[string]struct {
		Text string `json:"text"`
		Hash string `json:"hash"`
		Path string `json:"path"`
		Doc  struct {
			Mission string `json:"mission"`
			Members []any  `json:"members"`
			Workers []any  `json:"workers"`
			Turns   []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"turns"`
			Prompt struct {
				Turns   map[string][]string `json:"turns"`
				Workers map[string][]string `json:"workers"`
			} `json:"prompt"`
			Agreement struct {
				Path  string `json:"path"`
				Has   bool   `json:"has"`
				Rules int    `json:"rules"`
			} `json:"agreement"`
		} `json:"doc"`
	} `json:"places"`
}

// t2TestList is the list answer as the tests read it.
type t2TestList struct {
	Items  []t2TestItem      `json:"items"`
	Hashes map[string]string `json:"hashes"`
	Files  []t2TestFile      `json:"files"`
}

// t2TestIsland is the teams page's island.
type t2TestIsland struct {
	Team     t2TestList        `json:"team"`
	Project  string            `json:"project"`
	Places   map[string]string `json:"places"`
	Dirs     map[string]string `json:"dirs"`
	Profiles []string          `json:"profiles"`
}

// teamsIsland reads the data island out of a teams page.
func teamsIsland(t *testing.T, body string) t2TestIsland {
	t.Helper()
	const open = `<script id="teams-data" type="application/json">`
	_, after, ok := strings.Cut(body, open)
	if !ok {
		t.Fatalf("the teams page carries no island:\n%s", body)
	}
	rest := after
	before, _, ok := strings.Cut(rest, "</script>")
	if !ok {
		t.Fatalf("the island is not closed:\n%s", body)
	}
	var out t2TestIsland
	if err := json.Unmarshal([]byte(before), &out); err != nil {
		t.Fatalf("the island does not read: %v", err)
	}
	return out
}

// teamsCheck reads one check answer.
func teamsCheck(t *testing.T, body string) t2TestCheck {
	t.Helper()
	var out t2TestCheck
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the answer does not read: %v\n%s", err, body)
	}
	return out
}

// TestTeamsListCarriesTheTable: the list page carries one item per definition,
// with the places' readings and the columns the list shows, all of it from
// check.Team and the catalog (spec-teams 1.1, 1.2).
func TestTeamsListCarriesTheTable(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	resp, body := f.get(t, server, "/teams")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /teams answers %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{`data-tab="list"`, `data-screen="teams">チーム定義`} {
		if !strings.Contains(body, want) {
			t.Errorf("the teams page does not hold %q", want)
		}
	}
	island := teamsIsland(t, body)
	if len(island.Team.Items) != 1 || island.Team.Items[0].Name != "demo" {
		t.Fatalf("the island's items read %+v, want the one demo", island.Team.Items)
	}
	item := island.Team.Items[0]
	if item.Effective != "project" || len(item.Hidden) != 0 {
		t.Errorf("the item stands in %q with hidden %v, want the project", item.Effective, item.Hidden)
	}
	if item.Summary.Mission != "demo の使命" {
		t.Errorf("the mission reads %q", item.Summary.Mission)
	}
	if len(item.Summary.Members) != 2 || item.Summary.Workers != 3 {
		t.Errorf("the summary reads %d members, %d workers", len(item.Summary.Members), item.Summary.Workers)
	}
	if item.Summary.Rules != 1 {
		t.Errorf("the agreement's rules read %d, want one", item.Summary.Rules)
	}
	place := item.Places["project"]
	if place.Doc.Mission != "demo の使命" || len(place.Doc.Members) != 2 || len(place.Doc.Workers) != 3 || len(place.Doc.Turns) != 3 {
		t.Errorf("the project's reading reads %+v", place.Doc)
	}
	if !place.Doc.Agreement.Has || place.Doc.Agreement.Rules != 1 {
		t.Errorf("the agreement reads %+v, want the one rule", place.Doc.Agreement)
	}
	// The prompt's own line for what lead hands to builder (X3): the screen
	// never builds this string itself.
	want := tf("turns.member_item", "demo-builder", "builder")
	if !containsLine(place.Doc.Prompt.Turns["lead"], want) {
		t.Errorf("lead's turn lines read %v, want %q", place.Doc.Prompt.Turns["lead"], want)
	}
	// The YAML tab's files: the definition's team.yaml and its agreement.
	kinds := map[string]bool{}
	for _, file := range island.Team.Files {
		kinds[file.Place+"."+file.Kind] = true
		if file.Name != "demo" {
			t.Errorf("the file %s.%s carries name %q", file.Place, file.Kind, file.Name)
		}
	}
	if !kinds["project.team"] || !kinds["project.agreement"] {
		t.Errorf("the files read %v, want the team.yaml and the agreement.yaml", kinds)
	}
}

// containsLine is whether lines holds one line, exactly.
func containsLine(lines []string, want string) bool {
	return slices.Contains(lines, want)
}

// TestTeamsFigureCheckAndEffect: a check applies the panel's ops and answers the
// figure's problems; the one more op a card tries answers the prompt's lines it
// would move (spec-teams C2, C4).
func TestTeamsFigureCheckAndEffect(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)

	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{{"kind": "removeTurn", "from": "human", "to": "lead"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d: %s", resp.StatusCode, body)
	}
	check := teamsCheck(t, body)
	// The human no longer hands the turn on: an error that stops (I12a).
	stops := false
	for _, problem := range check.Problems {
		if problem.Sev == "err" {
			stops = true
		}
	}
	if !stops {
		t.Errorf("removing the human's turn found no error: %+v", check.Problems)
	}

	resp, body = f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{
			"ops": []map[string]any{},
			"op":  map[string]any{"kind": "removeTurn", "from": "lead", "to": "builder"},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d: %s", resp.StatusCode, body)
	}
	check = teamsCheck(t, body)
	if len(check.Effect.Lines) == 0 {
		t.Fatalf("the effect of removing lead -> builder found no line:\n%s", body)
	}
	if check.Effect.Lines[0].Member != "demo-lead" || check.Effect.Lines[0].Part != "turns" {
		t.Errorf("the effect's line reads %+v, want demo-lead's turns", check.Effect.Lines[0])
	}
	if want := tf("turns.member_item", "demo-builder", "builder"); !containsLine(check.Effect.Lines[0].Removed, want) {
		t.Errorf("the removed lines read %v, want %q", check.Effect.Lines[0].Removed, want)
	}
}

// TestTeamsFigureSaveWritesTheOneLine: a save writes the ops with byte-exact
// edits, so every line a person did not touch, a comment among them, comes back
// as it was (spec-teams 1.3, X1).
func TestTeamsFigureSaveWritesTheOneLine(t *testing.T) {
	f := newFixture(t)
	f.write(t, "teams/demo/team.yaml", "version: 1\n# a comment a person wrote\nmission: demo の使命\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\nworkers:\n  - from: lead\n    purpose: 実装\n    profile: coder\n  - from: lead\n    purpose: 試作\n    profile: coder\n  - from: lead\n    purpose: 調査\n    profile: planner\n")
	server := f.server(t)
	before := readFile(t, f.teamYAML())

	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "mission", "text": "新しい使命"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/demo answers %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, f.teamYAML())
	changed := changedLines(before, after)
	if len(changed) != 1 || !strings.Contains(after, "mission: 新しい使命") {
		t.Errorf("the save changed %d lines, want the mission's one:\n%s", len(changed), after)
	}
	if !strings.Contains(after, "# a comment a person wrote") {
		t.Errorf("the save dropped the comment:\n%s", after)
	}
}

// changedLines is the line numbers (0-based) where two texts differ.
func changedLines(before, after string) []int {
	b := strings.Split(before, "\n")
	a := strings.Split(after, "\n")
	var out []int
	for i := 0; i < len(b) || i < len(a); i++ {
		var x, y string
		if i < len(b) {
			x = b[i]
		}
		if i < len(a) {
			y = a[i]
		}
		if x != y {
			out = append(out, i)
		}
	}
	return out
}

// TestTeamsFigureSaveRefusesANewProblem: a save is refused only when a problem
// the saved file did not hold has appeared (spec-teams 1.3, X3).
func TestTeamsFigureSaveRefusesANewProblem(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.teamYAML())
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberProfile", "member": "builder", "profile": "nosuch"}}},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a save that adds a problem answers %d, want 422: %s", resp.StatusCode, body)
	}
	if got := readFile(t, f.teamYAML()); got != before {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// TestTeamsAgreementCheckSharesThePart: the agreement's check answers the same
// DocReport shape the role screen's part reads (spec-teams 1.5, C6).
func TestTeamsAgreementCheckSharesThePart(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	doc := map[string]any{
		"name": "", "title": "demo チームの約束", "revision": 1,
		"sections": []map[string]any{{"heading": "使命", "rules": []map[string]any{{
			"id": "TEAM-1", "text": "このチームの約束。", "why": "だから。",
			"send_why": true, "in_prompt": true, "note": "", "report": "", "tests": []any{}, "origin": []any{},
		}}}},
	}
	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "agreement",
		"doc": map[string]any{"ops": []map[string]any{}, "doc": doc},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d: %s", resp.StatusCode, body)
	}
	check := teamsCheck(t, body)
	if len(check.Preview.Lines) == 0 || check.Preview.Chars == 0 {
		t.Errorf("the agreement's Markdown is empty:\n%s", body)
	}
	if len(check.Readers) != 2 {
		t.Errorf("the readers read %d, want the two members", len(check.Readers))
	}
}

// TestTeamsAgreementSaveWritesTheFile: the agreement's save writes the panel's
// document into the team's own agreement.yaml (spec-teams 1.5).
func TestTeamsAgreementSaveWritesTheFile(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	doc := map[string]any{
		"name": "", "title": "demo チームの約束", "revision": 1,
		"sections": []map[string]any{{"heading": "使命", "rules": []map[string]any{{
			"id": "TEAM-1", "text": "このチームの約束を変えた。", "why": "だから。",
			"send_why": true, "in_prompt": true, "note": "", "report": "", "tests": []any{}, "origin": []any{},
		}}}},
	}
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "agreement", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{}, "doc": doc},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/demo answers %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	if !strings.Contains(after, "このチームの約束を変えた。") {
		t.Errorf("the agreement's save did not write the text:\n%s", after)
	}
	if !strings.Contains(after, "id: TEAM-1") {
		t.Errorf("the agreement's save dropped the rule's id:\n%s", after)
	}
}

// TestTeamsYamlCheckFindsTheLine: the YAML tab's check reads a whole text and
// puts each problem on the line of the key it names (spec-teams 1.8).
func TestTeamsYamlCheckFindsTheLine(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	text := "version: 1\nmission: demo\nturns:\n  - human -> lead\nmembers:\n  - name: lead\n    role: lead\n    profile: nosuch\n"
	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team", "text": text,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d: %s", resp.StatusCode, body)
	}
	check := teamsCheck(t, body)
	if len(check.Problems) == 0 || check.Problems[0].Sev != "err" {
		t.Fatalf("a member with a missing profile found no error: %+v", check.Problems)
	}
	if check.Problems[0].Line != 8 {
		t.Errorf("the missing profile stands on line %d, want the profile's line 8", check.Problems[0].Line)
	}
	if got := readFile(t, f.teamYAML()); !strings.Contains(got, "profile: coder\n") {
		t.Errorf("the check wrote the file:\n%s", got)
	}
}

// TestTeamsYamlSaveRefusesANewError: the YAML tab's save is refused only when a
// problem the saved file did not hold has appeared (spec-teams X3).
func TestTeamsYamlSaveRefusesANewError(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.teamYAML())
	bad := strings.Replace(before, "profile: coder", "profile: nosuch", 1)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before), "text": bad,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("the YAML save answers %d, want 422: %s", resp.StatusCode, body)
	}
	if got := readFile(t, f.teamYAML()); got != before {
		t.Errorf("the refused YAML save wrote the file:\n%s", got)
	}
	// A text the file's own errors do not grow is written as typed.
	ok := strings.Replace(before, "mission: demo の使命", "mission: 変えた使命", 1)
	resp, body = f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before), "text": ok,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a good YAML save answers %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, f.teamYAML()); got != ok {
		t.Errorf("the YAML save did not write the text as typed:\n%s", got)
	}
}

// TestTeamsNoLayerStopsAWrite: a project whose .hergy is gone has no place for
// a team.yaml, and the write says so with the sentence C5 added (spec-teams
// 1.7, X6).
func TestTeamsNoLayerStopsAWrite(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	if err := os.RemoveAll(filepath.Join(f.proj, ".hergy")); err != nil {
		t.Fatal(err)
	}
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": "",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("PUT /api/team/demo answers %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "teams/") {
		t.Errorf("the refusal does not say teams/ cannot be written:\n%s", body)
	}
}

// TestTeamsYamlLine: the line of a team.yaml field reads, the forms check uses
// among them (spec-teams 1.8).
func TestTeamsYamlLine(t *testing.T) {
	text := "version: 1\nmission: demo\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\nturns:\n  - human -> lead\n"
	for _, tc := range []struct {
		field string
		want  int
	}{
		{"mission", 2},
		{"members[0].profile", 6},
		{"members[1].role", 8},
		{"turns[0]", 10},
		{"node:human", 0},
		{"rule:/x/roles/lead.yaml#A", 0},
		{"workers[0]", 0},
	} {
		if got := t2YamlLine(text, tc.field); got != tc.want {
			t.Errorf("the line of %q reads %d, want %d", tc.field, got, tc.want)
		}
	}
}
