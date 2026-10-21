package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/scaffold"
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
	text := "version: 1\nmission: demo\nturns:\n  - human -> lead\n  - lead -> human\nmembers:\n  - name: lead\n    role: lead\n    profile: nosuch\n"
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
	if check.Problems[0].Line != 9 {
		t.Errorf("the missing profile stands on line %d, want the profile's line 9", check.Problems[0].Line)
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

// t2TestDraft is what a check of a definition that is not there yet answers
// (spec-teams B2, B7), as the tests look at it.
type t2TestDraft struct {
	Name     string          `json:"name"`
	Place    string          `json:"place"`
	Starter  bool            `json:"starter"`
	Path     string          `json:"path"`
	Writes   []string        `json:"writes"`
	Problems []t2TestProblem `json:"problems"`
	Doc      struct {
		Mission string `json:"mission"`
		Members []struct {
			Name string `json:"name"`
		} `json:"members"`
		Agreement struct {
			Path string `json:"path"`
			Has  bool   `json:"has"`
		} `json:"agreement"`
	} `json:"doc"`
	Preview struct {
		Lines []struct {
			Text string `json:"text"`
			At   string `json:"at"`
		} `json:"lines"`
		Diff struct {
			Added         []int `json:"added"`
			RemovedBefore []int `json:"removedBefore"`
			RemovedAtEnd  bool  `json:"removedAtEnd"`
		} `json:"diff"`
	} `json:"preview"`
}

// teamsDraft reads one check answer of a definition that is not there yet.
func teamsDraft(t *testing.T, body string) t2TestDraft {
	t.Helper()
	var out t2TestDraft
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the answer does not read: %v\n%s", err, body)
	}
	return out
}

// TestTeamsDraftCheckAnswersTheNameAndTheWrites: a definition that is not there
// yet is checked as if it were saved, and the answer carries its name problems,
// where it would stand, what a save would write, and the topology tab's own
// preview lines (spec-teams B2, B7, C8).
func TestTeamsDraftCheckAnswersTheNameAndTheWrites(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	checkOne := func(name string) t2TestDraft {
		t.Helper()
		resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
			"place": "project", "name": name, "part": "team", "new": true,
			"doc": map[string]any{"ops": []any{}},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/team/check answers %d, want 200: %s", resp.StatusCode, body)
		}
		return teamsDraft(t, body)
	}

	// An empty name is the name's problem, and it is about the name; the
	// figure's own sentences stand beside it all the same — a row that was just
	// added shows the mission's warning while the name is being typed (B2, N1).
	empty := checkOne("")
	if len(empty.Problems) != 2 || empty.Problems[0].At != "name" || empty.Problems[1].At != "mission" {
		t.Fatalf("an empty name answers %+v, want the name's problem and the mission's warning", empty.Problems)
	}
	if want := i18n.T("error.team_name_needed"); empty.Problems[0].Text != want {
		t.Errorf("the empty name's problem reads %q, want %q", empty.Problems[0].Text, want)
	}

	// A name the place already holds is a name problem too, in the words the
	// scaffold's own check uses (B2, Y2).
	taken := checkOne("demo")
	if len(taken.Problems) == 0 || taken.Problems[0].At != "name" {
		t.Fatalf("a taken name answers %+v, want one problem about the name", taken.Problems)
	}
	if want := i18n.Tf("error.new_exists", "demo", filepath.Join(f.proj, ".hergy", "teams", "demo")); taken.Problems[0].Text != want {
		t.Errorf("the taken name's problem reads %q, want %q", taken.Problems[0].Text, want)
	}

	// A name hergy new accepts is no problem, and the row says where it stands
	// and what a save would write (B2, C8, Y3).
	draft := checkOne("ops")
	for _, p := range draft.Problems {
		if p.At == "name" {
			t.Errorf("the name %q still answers %q", draft.Name, p.Text)
		}
	}
	if draft.Name != "ops" || draft.Place != "project" || !draft.Starter {
		t.Errorf("the draft reads name %q place %q starter %v, want ops / project / true", draft.Name, draft.Place, draft.Starter)
	}
	dir := filepath.Join(f.proj, ".hergy", "teams", "ops")
	for _, want := range []string{
		filepath.Join(dir, "team.yaml"),
		filepath.Join(dir, "agreement.yaml"),
		filepath.Join(f.proj, ".hergy", "profiles.yaml"),
	} {
		if !slices.Contains(draft.Writes, want) {
			t.Errorf("the writes read %v, want %s among them", draft.Writes, want)
		}
	}
	// The starter's own figure: two members, and the mission placeholder
	// validate warns about (B2).
	if len(draft.Doc.Members) != 2 {
		t.Errorf("the starter's figure holds %d members, want 2", len(draft.Doc.Members))
	}
	if draft.Doc.Mission == "" {
		t.Error("the starter's figure carries no mission")
	}
	if !draft.Doc.Agreement.Has {
		t.Error("the starter's own agreement is not read")
	}
	// The preview is the text a save would write, line by line, and the lines
	// say what they stand for (B7, Y4).
	if len(draft.Preview.Lines) == 0 {
		t.Fatal("the draft answers no preview lines")
	}
	var atMission, atNode bool
	for _, line := range draft.Preview.Lines {
		if line.At == "mission" {
			atMission = true
		}
		if strings.HasPrefix(line.At, "node:") {
			atNode = true
		}
	}
	if !atMission || !atNode {
		t.Errorf("the preview lines name no mission (%v) or member (%v): %+v", atMission, atNode, draft.Preview.Lines)
	}
	// No ops: nothing the row writes differs from the base it stands on.
	if len(draft.Preview.Diff.Added) != 0 || len(draft.Preview.Diff.RemovedBefore) != 0 || draft.Preview.Diff.RemovedAtEnd {
		t.Errorf("a draft with no ops answers a diff: %+v", draft.Preview.Diff)
	}
}

// TestTeamsDraftSaveWritesTheStarterFolder: saving a definition that is not there
// yet writes hergy new's own files, so the screen and the command cannot drift
// (spec-teams B2, C8, N3).
func TestTeamsDraftSaveWritesTheStarterFolder(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/ops", map[string]any{
		"place": "project", "name": "ops", "part": "folder", "new": true,
		"doc": map[string]any{"ops": []any{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/ops answers %d, want 200: %s", resp.StatusCode, body)
	}
	files, err := scaffold.Team(i18n.Language())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, file := range files {
		want[file.Path] = string(file.Data)
	}
	dir := filepath.Join(f.proj, ".hergy", "teams", "ops")
	for _, name := range []string{"team.yaml", "agreement.yaml"} {
		got := readFile(t, filepath.Join(dir, name))
		if got != want[name] {
			t.Errorf("the saved %s reads\n%s\nwant hergy new's own\n%s", name, got, want[name])
		}
	}
	// The layer's role documents the starter reads were already there, so the
	// save left them as they are (K5).
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")); !strings.Contains(got, "LEAD-1") {
		t.Errorf("the layer's own lead.yaml was written over:\n%s", got)
	}
	// The starter profile went into the layer's profiles.yaml.
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml")); !strings.Contains(got, scaffold.ProfileName) {
		t.Errorf("profiles.yaml does not hold the starter profile:\n%s", got)
	}
	// The definition is in the list now, and the answer carries it (B4).
	if !strings.Contains(body, `"ops"`) {
		t.Errorf("the save's answer does not carry the new definition:\n%s", body)
	}
}

// TestTeamsDraftSaveRefusesANameProblem: a name that is empty, not one hergy new
// accepts, or already in the place stops the write before anything is written
// (spec-teams B2, N2).
func TestTeamsDraftSaveRefusesAnUnwritableName(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	for _, tc := range []struct{ name, want string }{
		{"Bad_Name", i18n.Tf("error.new_name_pattern", "Bad_Name")},
		{"demo", i18n.Tf("error.new_exists", "demo", filepath.Join(f.proj, ".hergy", "teams", "demo"))},
	} {
		resp, body := f.api(t, server, http.MethodPut, "/api/team/"+tc.name, map[string]any{
			"place": "project", "name": tc.name, "part": "folder", "new": true,
			"doc": map[string]any{"ops": []any{}},
		})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PUT /api/team/%s answers %d, want 422: %s", tc.name, resp.StatusCode, body)
		}
		var refusal struct {
			Text string `json:"text"`
			At   string `json:"at"`
		}
		if err := json.Unmarshal([]byte(body), &refusal); err != nil {
			t.Fatalf("the refusal does not read: %v\n%s", err, body)
		}
		if refusal.Text != tc.want || refusal.At != "name" {
			t.Errorf("the refusal for %q reads %q at %q, want %q at name", tc.name, refusal.Text, refusal.At, tc.want)
		}
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "Bad_Name")); !os.IsNotExist(err) {
		t.Error("the refused name was written")
	}
}

// TestTeamsDraftDuplicateWritesTheSameFiles: a duplicate stands on the named
// definition's own files and writes only its own two, byte for byte (spec-teams
// B3, C8, N4, N5).
func TestTeamsDraftDuplicateWritesTheSameFiles(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	base := map[string]any{"place": "project", "name": "demo"}
	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo-2", "part": "team", "base": base,
		"doc": map[string]any{"ops": []any{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d, want 200: %s", resp.StatusCode, body)
	}
	draft := teamsDraft(t, body)
	if draft.Starter {
		t.Error("a duplicate is not a starter, so it writes no role documents")
	}
	if len(draft.Writes) != 2 {
		t.Errorf("a duplicate writes %v, want its own two files", draft.Writes)
	}
	resp, body = f.api(t, server, http.MethodPut, "/api/team/demo-2", map[string]any{
		"place": "project", "name": "demo-2", "part": "folder", "base": base,
		"doc": map[string]any{"ops": []any{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/demo-2 answers %d, want 200: %s", resp.StatusCode, body)
	}
	one := filepath.Join(f.proj, ".hergy", "teams", "demo")
	two := filepath.Join(f.proj, ".hergy", "teams", "demo-2")
	for _, name := range []string{"team.yaml", "agreement.yaml"} {
		if got, want := readFile(t, filepath.Join(two, name)), readFile(t, filepath.Join(one, name)); got != want {
			t.Errorf("the duplicate's %s reads\n%s\nwant the original's\n%s", name, got, want)
		}
	}
}

// TestTeamsCopyAndDeleteTakeOnePlace: a copy stands on the other place's
// definition and goes into this place under the same name, and taking one place's
// folder away leaves the other's (spec-teams B3, B6, N6, N7).
func TestTeamsCopyAndDeleteTakeOnePlace(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "global", "name": "demo", "part": "folder",
		"base": map[string]any{"place": "project", "name": "demo"},
		"doc":  map[string]any{"ops": []any{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/team/demo answers %d, want 200: %s", resp.StatusCode, body)
	}
	global := filepath.Join(f.global, "teams", "demo")
	if got, want := readFile(t, filepath.Join(global, "team.yaml")), readFile(t, f.teamYAML()); got != want {
		t.Errorf("the copy reads\n%s\nwant the original's\n%s", got, want)
	}
	// Both places hold it now, so the row shows one in effect and one hidden
	// (spec-teams 1.4.5).
	if !strings.Contains(body, `"global"`) || !strings.Contains(body, `"project"`) {
		t.Errorf("the answer names neither place:\n%s", body)
	}

	// A hash that moved under the screen is refused, and nothing is removed
	// (B6).
	stale, staleBody := f.api(t, server, http.MethodDelete, "/api/team/demo?place=project&hash=nope", nil)
	if stale.StatusCode != http.StatusConflict {
		t.Fatalf("DELETE with a stale hash answers %d, want 409: %s", stale.StatusCode, staleBody)
	}
	if _, err := os.Stat(f.teamYAML()); err != nil {
		t.Error("the refused delete removed the project's team.yaml")
	}

	// The delete takes the whole folder, and the directory with it (B6, N7).
	agreePath := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	q := url.Values{}
	q.Set("place", "project")
	q.Set("hash", source.Hash(readFile(t, f.teamYAML())))
	q.Set("agreement_hash", source.Hash(readFile(t, agreePath)))
	resp, body = f.api(t, server, http.MethodDelete, "/api/team/demo?"+q.Encode(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/team/demo answers %d, want 200: %s", resp.StatusCode, body)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "demo")); !os.IsNotExist(err) {
		t.Error("the project's definition's directory is still there")
	}
	if _, err := os.Stat(filepath.Join(global, "team.yaml")); err != nil {
		t.Error("the copy in the other place went away with it")
	}
}
