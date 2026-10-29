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

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
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
	Size     int             `json:"size"`
	Limit    int             `json:"limit"`
	Members  []struct {
		Name     string                     `json:"name"`
		Segments []definition.PromptSegment `json:"segments"`
	} `json:"members"`
	Prompt  []definition.PromptSegment `json:"prompt"`
	Preview struct {
		Lines []struct {
			Text string `json:"text"`
			At   string `json:"at"`
			Part string `json:"part"`
			Held string `json:"held"`
		} `json:"lines"`
		Diff struct {
			Added         []int `json:"added"`
			RemovedBefore []int `json:"removedBefore"`
			RemovedAtEnd  bool  `json:"removedAtEnd"`
		} `json:"diff"`
		Chars int `json:"chars"`
	} `json:"preview"`
	Effect struct {
		Lines []struct {
			Member  string   `json:"member"`
			Part    string   `json:"part"`
			Removed []string `json:"removed"`
			Added   []string `json:"added"`
		} `json:"lines"`
		Problems []t2TestProblem `json:"problems"`
	} `json:"effect"`
	Readers []struct {
		Definition string `json:"definition"`
		Member     string `json:"member"`
		Rules      int    `json:"rules"`
		Chars      int    `json:"chars"`
		Over       bool   `json:"over"`
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
		Workers  int `json:"workers"`
		Rules    int `json:"rules"`
		Excluded int `json:"excluded"`
		Size     int `json:"size"`
		Limit    int `json:"limit"`
	} `json:"summary"`
	Places map[string]struct {
		Text string `json:"text"`
		Hash string `json:"hash"`
		Path string `json:"path"`
		Doc  struct {
			Mission string `json:"mission"`
			Members []struct {
				Name     string                     `json:"name"`
				Role     string                     `json:"role"`
				Profile  string                     `json:"profile"`
				Segments []definition.PromptSegment `json:"segments"`
			} `json:"members"`
			Workers []any `json:"workers"`
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

// TestTEAMS_S3_ListCarriesTheTable: the list page carries one item per definition,
// with the places' readings and the columns the list shows, all of it from
// check.Team and the catalog (spec-teams 1.1, 1.2).
func TestTEAMS_S3_ListCarriesTheTable(t *testing.T) {
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
	if len(place.Doc.Members) == 0 || len(place.Doc.Members[0].Segments) == 0 {
		t.Errorf("the saved document carries no prompt segments: %+v", place.Doc.Members)
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

// TestTEAMS_ListSizeIsTheFigureCheck: the list's 大きさ column carries the same
// size and budget as the figure's check answer (r-2f83). Both count the part of
// the definition's rules and templates that goes into the prompt (d-00a9). A
// file that does not read has no size, but its budget is still known.
func TestTEAMS_ListSizeIsTheFigureCheck(t *testing.T) {
	f := newFixture(t)
	f.write(t, filepath.Join("teams", "broken", "team.yaml"), "version: 2\n")
	server := f.server(t)
	resp, body := f.get(t, server, "/teams")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /teams answers %d, want 200", resp.StatusCode)
	}
	island := teamsIsland(t, body)
	if len(island.Team.Items) != 2 {
		t.Fatalf("the list holds %d items, want the readable demo and the broken one", len(island.Team.Items))
	}
	for _, item := range island.Team.Items {
		resp, answer := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
			"place": item.Effective, "name": item.Name, "part": "team",
			"doc": map[string]any{"ops": []map[string]any{}},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the figure check of %s answers %d: %s", item.Name, resp.StatusCode, answer)
		}
		checkAnswer := teamsCheck(t, answer)
		if item.Summary.Size != checkAnswer.Size {
			t.Errorf("%s's list size reads %d, the figure answers %d", item.Name, item.Summary.Size, checkAnswer.Size)
		}
		if item.Summary.Limit != checkAnswer.Limit {
			t.Errorf("%s's list limit reads %d, the figure answers %d", item.Name, item.Summary.Limit, checkAnswer.Limit)
		}
		if item.Summary.Limit != check.TeamCharLimit {
			t.Errorf("%s's limit reads %d, want the provisional budget %d", item.Name, item.Summary.Limit, check.TeamCharLimit)
		}
		if item.Name == "demo" && item.Summary.Size == 0 {
			t.Errorf("the readable demo's size reads 0, want the agreement's part")
		}
		if item.Name == "broken" && item.Summary.Size != 0 {
			t.Errorf("the broken definition's size reads %d, want 0", item.Summary.Size)
		}
	}
}

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

// TestTEAMS_MemberOpsThroughTheAPI: the four member and worker ops the canvas
// sends reach definition.EditYAML's single-purpose edits through the panel's
// check: the preview is the team.yaml the save would write, and the other lines
// stay where they were (r-b614).
func TestTEAMS_MemberOpsThroughTheAPI(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct {
		name string
		op   map[string]any
		want func(t *testing.T, text string)
	}{
		{
			name: "addMember",
			op:   map[string]any{"kind": "addMember", "member": "ops", "role": "builder", "profile": "coder"},
			want: func(t *testing.T, text string) {
				for _, line := range []string{"- name: ops", "role: builder", "profile: coder"} {
					if !strings.Contains(text, line) {
						t.Errorf("the preview does not hold %q:\n%s", line, text)
					}
				}
			},
		},
		{
			name: "removeMember",
			op:   map[string]any{"kind": "removeMember", "member": "builder"},
			want: func(t *testing.T, text string) {
				for _, line := range []string{"name: builder", "lead -> builder", "builder -> human"} {
					if strings.Contains(text, line) {
						t.Errorf("the preview still holds %q:\n%s", line, text)
					}
				}
			},
		},
		{
			name: "renameMember",
			op:   map[string]any{"kind": "renameMember", "member": "builder", "new": "impl"},
			want: func(t *testing.T, text string) {
				for _, line := range []string{"name: impl", "lead -> impl", "impl -> human"} {
					if !strings.Contains(text, line) {
						t.Errorf("the preview does not hold %q:\n%s", line, text)
					}
				}
				if strings.Contains(text, "name: builder") || strings.Contains(text, "-> builder") || strings.Contains(text, "builder ->") {
					t.Errorf("the preview still names builder:\n%s", text)
				}
			},
		},
		{
			name: "moveWorker",
			op:   map[string]any{"kind": "moveWorker", "from": "lead", "purpose": "試作", "to": "builder"},
			want: func(t *testing.T, text string) {
				if strings.Count(text, "from: lead") != 2 {
					t.Errorf("the workers lead hires read %d, want 2:\n%s", strings.Count(text, "from: lead"), text)
				}
				if strings.Count(text, "from: builder") != 1 {
					t.Errorf("the workers builder hires read %d, want the moved one:\n%s", strings.Count(text, "from: builder"), text)
				}
				if !strings.Contains(text, "purpose: 試作") {
					t.Errorf("the moved worker's purpose is not in the preview:\n%s", text)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
				"place": "project", "name": "demo", "part": "team",
				"doc": map[string]any{"ops": []map[string]any{tc.op}},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
			}
			check := teamsCheck(t, body)
			var text strings.Builder
			for _, line := range check.Preview.Lines {
				text.WriteString(line.Text)
				text.WriteString("\n")
			}
			tc.want(t, text.String())
		})
	}
}

// TestTEAMS_PromptSegmentsThroughTheAPI: the check answers each member's prompt
// as the pieces it was built from, for the saved definition and with the panel's
// ops applied; a worker's prompt is one prompt's pieces (r-4cf9).
func TestTEAMS_PromptSegmentsThroughTheAPI(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)

	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "addMember", "member": "ops", "role": "builder", "profile": "coder"},
			{"kind": "addTurn", "from": "human", "to": "ops"},
			{"kind": "addTurn", "from": "ops", "to": "human"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
	}
	check := teamsCheck(t, body)
	var added *struct {
		Name     string                     `json:"name"`
		Segments []definition.PromptSegment `json:"segments"`
	}
	for i := range check.Members {
		if check.Members[i].Name == "ops" {
			added = &check.Members[i]
		}
	}
	if added == nil {
		t.Fatalf("the check does not answer the added member's prompt: %+v", check.Members)
	}
	if len(added.Segments) == 0 {
		t.Fatal("the added member has no prompt segments")
	}
	var roster, role string
	for _, segment := range added.Segments {
		if segment.Kind == "roster" {
			roster = segment.Text
		}
		if segment.Kind == "role_doc" {
			role = segment.Origin.Path
		}
	}
	if !strings.Contains(roster, "ops") {
		t.Errorf("the added member's roster segment does not name it: %q", roster)
	}
	if !strings.HasSuffix(role, "/.hergy/roles/builder.yaml") {
		t.Errorf("the added member's role segment reads %+v", role)
	}

	// The worker prompt: one prompt's segments, with the task it was hired for.
	resp, body = f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "worker",
		"doc": map[string]any{"name": "helper", "purpose": "試作", "task": "read the thing"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the worker check answers %d: %s", resp.StatusCode, body)
	}
	worker := teamsCheck(t, body)
	if len(worker.Prompt) == 0 {
		t.Fatal("the worker answer has no prompt")
	}
	var text strings.Builder
	var task string
	for _, segment := range worker.Prompt {
		text.WriteString(segment.Text)
		if segment.Kind == "task" {
			task = segment.Text
		}
	}
	if task != "read the thing" || !strings.Contains(text.String(), "read the thing") {
		t.Errorf("the worker's task is not its own segment: %+v", worker.Prompt)
	}
}

// TestTEAMS_F1_FigureSaveWritesTheOneLine: a save writes the ops with byte-exact
// edits, so every line a person did not touch, a comment among them, comes back
// as it was (spec-teams 1.3, X1).
func TestTEAMS_F1_FigureSaveWritesTheOneLine(t *testing.T) {
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

// TestTEAMS_F7_FigureSaveRefusesANewProblem: a save is refused only when a problem
// the saved file did not hold has appeared (spec-teams 1.3, X3).
func TestTEAMS_F7_FigureSaveRefusesANewProblem(t *testing.T) {
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
	saved, _ := rules.ReadAll(after, path)
	if len(saved.Rules()) != 1 || !rules.RuleIDPattern.MatchString(saved.Rules()[0].ID) {
		t.Errorf("the agreement's save wrote no id hergy gives:\n%s", after)
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

// TestTEAMS_P11_YamlSaveRefusesANewError: the YAML tab's save is refused only when a
// problem the saved file did not hold has appeared (spec-teams X3).
func TestTEAMS_P11_YamlSaveRefusesANewError(t *testing.T) {
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
	Draft struct {
		Name    string   `json:"name"`
		Place   string   `json:"place"`
		Starter bool     `json:"starter"`
		Path    string   `json:"path"`
		Writes  []string `json:"writes"`
	} `json:"draft"`
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
			t.Errorf("the name %q still answers %q", draft.Draft.Name, p.Text)
		}
	}
	if draft.Draft.Name != "ops" || draft.Draft.Place != "project" || !draft.Draft.Starter {
		t.Errorf("the draft reads name %q place %q starter %v, want ops / project / true", draft.Draft.Name, draft.Draft.Place, draft.Draft.Starter)
	}
	dir := filepath.Join(f.proj, ".hergy", "teams", "ops")
	for _, want := range []string{
		filepath.Join(dir, "team.yaml"),
		filepath.Join(dir, "agreement.yaml"),
		filepath.Join(f.proj, ".hergy", "profiles.yaml"),
	} {
		if !slices.Contains(draft.Draft.Writes, want) {
			t.Errorf("the writes read %v, want %s among them", draft.Draft.Writes, want)
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

// TestTEAMS_N3_DraftSaveWritesTheStarterFolder: saving a definition that is not there
// yet writes hergy new's own files, so the screen and the command cannot drift
// (spec-teams B2, C8, N3).
func TestTEAMS_N3_DraftSaveWritesTheStarterFolder(t *testing.T) {
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
		if withoutRuleIDs(got) != want[name] {
			t.Errorf("the saved %s reads\n%s\nwant hergy new's own (without the ids it gives)\n%s", name, got, want[name])
		}
		if name == "agreement.yaml" {
			doc, _ := rules.ReadAll(got, filepath.Join(dir, name))
			for _, rule := range doc.Rules() {
				if !rules.RuleIDPattern.MatchString(rule.ID) {
					t.Errorf("the saved agreement's rule %q has no id hergy writes", rule.Text)
				}
			}
		}
	}
	// The layer's role documents the starter reads were already there, so the
	// save left them as they are (K5).
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")); !strings.Contains(got, "PAAF") {
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

// TestTEAMS_N2_DraftSaveRefusesANameProblem: a name that is empty, not one hergy new
// accepts, or already in the place stops the write before anything is written
// (spec-teams B2, N2).
func TestTEAMS_N2_DraftSaveRefusesANameProblem(t *testing.T) {
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

// TestTEAMS_N5_DraftDuplicateWritesTheSameFiles: a duplicate stands on the named
// definition's own files and writes only its own two, byte for byte (spec-teams
// B3, C8, N4, N5).
func TestTEAMS_N5_DraftDuplicateWritesTheSameFiles(t *testing.T) {
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
	if draft.Draft.Starter {
		t.Error("a duplicate is not a starter, so it writes no role documents")
	}
	if len(draft.Draft.Writes) != 2 {
		t.Errorf("a duplicate writes %v, want its own two files", draft.Draft.Writes)
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
		got, want := readFile(t, filepath.Join(two, name)), readFile(t, filepath.Join(one, name))
		if withoutRuleIDs(got) != withoutRuleIDs(want) {
			t.Errorf("the duplicate's %s reads\n%s\nwant the original's (without the ids)\n%s", name, got, want)
		}
	}
	// The duplicate's rules are its own: no id is shared with the definition it
	// was copied from (d-4aab).
	original, _ := rules.ReadAll(readFile(t, filepath.Join(one, "agreement.yaml")), filepath.Join(one, "agreement.yaml"))
	copied, _ := rules.ReadAll(readFile(t, filepath.Join(two, "agreement.yaml")), filepath.Join(two, "agreement.yaml"))
	held := map[string]bool{}
	for _, rule := range original.Rules() {
		held[rule.ID] = true
	}
	for _, rule := range copied.Rules() {
		if held[rule.ID] {
			t.Errorf("the duplicate takes the id %s of the definition it copied", rule.ID)
		}
	}
}

// withoutRuleIDs is a file's lines without the id lines hergy writes: the
// template and the written file are the same shape otherwise (d-4aab).
func withoutRuleIDs(text string) string {
	var kept []string
	dash := -1
	for line := range strings.SplitSeq(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "- id: ") {
			// The id took the item's dash: the key under it gets the dash back.
			dash = len(line) - len(strings.TrimLeft(line, " "))
			continue
		}
		if strings.HasPrefix(trim, "id: ") {
			continue
		}
		if dash >= 0 {
			line = strings.Repeat(" ", dash) + "- " + trim
			dash = -1
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestTEAMS_N5_N7_CopyAndDeleteTakeOnePlace: a copy stands on the other place's
// definition and goes into this place under the same name, and taking one place's
// folder away leaves the other's (spec-teams B3, B6, N6, N7).
func TestTEAMS_N5_N7_CopyAndDeleteTakeOnePlace(t *testing.T) {
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

// TestTeamsDraftCheckReadsTheRowsOwnFiles: a definition that is not there yet is
// checked as its own draft, so a name a saved definition already holds brings
// that definition's rules with it no longer, and the paths the answer names are
// the name a person typed (spec-teams B2, 監査 R6).
func TestTeamsDraftCheckReadsTheRowsOwnFiles(t *testing.T) {
	f := newFixture(t)
	// The saved definition's own rule writes a turn the team does not hold: read
	// against the draft's base it warns, and read against the draft's own
	// agreement it is not there at all.
	f.write(t, filepath.Join("teams", "demo", "agreement.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: TEAM-1\n        text: lead → reviewer と受け渡す。\n")
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

	starter := checkOne("ops")
	for _, p := range starter.Problems {
		if strings.HasPrefix(p.At, "rule:") {
			t.Fatalf("a fresh draft answers a rule's own problem: %+v", p)
		}
	}
	taken := checkOne("demo")
	if taken.Problems[0].At != "name" {
		t.Fatalf("a name the place holds answers %+v, want the name's problem first", taken.Problems)
	}
	// The saved definition's rule is not read: the row's problems are the name's
	// and its own base's, the same as a fresh draft's.
	for _, p := range taken.Problems[1:] {
		if strings.HasPrefix(p.At, "rule:") {
			t.Errorf("a row named after a saved definition is told about that definition's rule: %+v", p)
		}
	}
	if len(taken.Problems)-1 != len(starter.Problems) {
		t.Errorf("a taken name answers %d problems beside its name's, want the %d a fresh draft answers",
			len(taken.Problems)-1, len(starter.Problems))
	}
	// The paths are the name a person typed, not the definition it collides with
	// (監査 R5).
	if want := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"); taken.Draft.Path != want {
		t.Errorf("the row says it stands at %q, want %q", taken.Draft.Path, want)
	}
	if want := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"); taken.Doc.Agreement.Path != want {
		t.Errorf("the row's agreement says it stands at %q, want %q", taken.Doc.Agreement.Path, want)
	}
}

// TestTEAMS_SaveKeepsATurnProblemForTheDefinition: a save may add a member with
// no turn (d-9abb). The save writes the member, and the definition then carries
// I12a as its own problem: the row counts it and the check says how to fix it.
func TestTEAMS_SaveKeepsATurnProblemForTheDefinition(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.teamYAML())
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "addMember", "member": "ops", "role": "builder", "profile": "coder"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a member with no turn is refused: %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(readFile(t, f.teamYAML()), "name: ops") {
		t.Fatalf("the save did not write the member:\n%s", readFile(t, f.teamYAML()))
	}

	_, body = f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	check := teamsCheck(t, body)
	want := i18n.Tf("error.graph_member_no_turn", "ops", "ops", "ops", "lead")
	found := false
	for _, problem := range check.Problems {
		if problem.Sev == "err" && strings.Contains(problem.Text, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("the definition does not carry %q: %+v", want, check.Problems)
	}
	_, page := f.get(t, server, "/teams")
	island := teamsIsland(t, page)
	if len(island.Team.Items) != 1 || island.Team.Items[0].Problems.Err != 1 {
		t.Errorf("the row's problem count reads %+v, want one", island.Team.Items)
	}
}

// TestTEAMS_SaveKeepsAHumanTurnProblemForTheDefinition: removing the member that
// handed the turn to the human is a save too; I12b stays a definition problem.
func TestTEAMS_SaveKeepsAHumanTurnProblemForTheDefinition(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.teamYAML())
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "removeMember", "member": "builder"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("removing the member that reaches the human is refused: %d: %s", resp.StatusCode, body)
	}
	if strings.Contains(readFile(t, f.teamYAML()), "name: builder") {
		t.Fatalf("the save did not remove the member:\n%s", readFile(t, f.teamYAML()))
	}

	_, body = f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	check := teamsCheck(t, body)
	want := i18n.Tf("error.graph_no_turn_reaches_human", "lead")
	found := false
	for _, problem := range check.Problems {
		if problem.Sev == "err" && strings.Contains(problem.Text, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("the definition does not carry %q: %+v", want, check.Problems)
	}
}

// TestTEAMS_Y6_ThePreviewIsTheSavedFile: the topology tab's preview is the saved
// team.yaml line for line, comments and blank lines among them, and a turn the
// panel adds stands as an added line marked with the turn it is (spec-teams B7,
// Y6, Y2). Moved from the browser walk TEAMS-Y2, which compared the drawn lines
// with the file's own text.
func TestTEAMS_Y6_ThePreviewIsTheSavedFile(t *testing.T) {
	f := newFixture(t)
	text := "version: 1\n# a comment a person wrote\nmission: demo の使命\n\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: analyst\n    role: analyst\n    profile: coder\nturns:\n  - human -> lead\n  - analyst -> lead\n"
	f.write(t, "teams/demo/team.yaml", text)
	server := f.server(t)

	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
	}
	check := teamsCheck(t, body)
	got := make([]string, len(check.Preview.Lines))
	for i, line := range check.Preview.Lines {
		got[i] = line.Text
	}
	want := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Errorf("the preview is not the saved file line for line:\n got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The turn the panel adds is an op; its line is the one the diff marks
	// added, and it stands at the end of the preview.
	resp, body = f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{
			{"kind": "addTurn", "from": "lead", "to": "analyst"},
		}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check with the turn answers %d: %s", resp.StatusCode, body)
	}
	check = teamsCheck(t, body)
	added := check.Preview.Diff.Added
	if len(added) != 1 || added[0] != len(check.Preview.Lines)-1 {
		t.Fatalf("the diff marks %v lines added, want the last line alone", added)
	}
	if line := check.Preview.Lines[added[0]]; strings.TrimSpace(line.Text) != "- lead -> analyst" {
		t.Errorf("the added line reads %q, want the turn", line.Text)
	}
}

// TestTEAMS_YamlSaveStillRefusesANewGraphError: the save gate lets I12a and I12b
// through, and nothing else: a turn to a node that is not there is still
// refused (d-9abb).
func TestTEAMS_YamlSaveStillRefusesANewGraphError(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	before := readFile(t, f.teamYAML())
	after := strings.Replace(before, "turns:\n", "turns:\n  - lead -> ghost\n", 1)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before), "text": after,
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a turn to a node that is not there is accepted: %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "ghost") {
		t.Errorf("the refusal does not name the missing node: %s", body)
	}
}

// TestTEAMS_CopyMemberThroughTheAPI: the copy the figure makes is the operations a
// person would make by hand — the member, the turns both ways and the workers'
// purposes — written in the order that reads: the member first, so the turns and
// the workers that name it are valid (r-c21b, d-48a9). The check takes them, and a
// save writes the copy into team.yaml and leaves the member it came from alone.
func TestTEAMS_CopyMemberThroughTheAPI(t *testing.T) {
	f := newFixture(t)
	f.writeTeam(t, "core", "注文 API を作り直す")
	server := f.server(t)
	ops := []map[string]any{
		{"kind": "addMember", "member": "lead-2", "role": "lead", "profile": "coder"},
		{"kind": "addTurn", "from": "human", "to": "lead-2"},
		{"kind": "addTurn", "from": "lead-2", "to": "builder"},
		{"kind": "addWorker", "from": "lead-2", "purpose": "実装", "profile": "coder"},
		{"kind": "addWorker", "from": "lead-2", "purpose": "試作", "profile": "coder"},
		{"kind": "addWorker", "from": "lead-2", "purpose": "調査", "profile": "planner"},
	}
	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "core", "part": "team",
		"doc": map[string]any{"ops": ops},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the copy's check answers %d: %s", resp.StatusCode, body)
	}
	for _, problem := range teamsCheck(t, body).Problems {
		if problem.Sev == "err" {
			t.Errorf("the copy is checked as broken: %s", problem.Text)
		}
	}

	path := filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml")
	before := readFile(t, path)
	resp, body = f.api(t, server, http.MethodPut, "/api/team/core", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": ops},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the copy's save answers %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	for _, want := range []string{
		"- name: lead-2",
		"- human -> lead-2",
		"- lead-2 -> builder",
		"- from: lead-2",
		"purpose: 実装",
		"purpose: 試作",
		"purpose: 調査",
	} {
		if !strings.Contains(after, want) {
			t.Errorf("the saved copy does not hold %q:\n%s", want, after)
		}
	}
	// The member it was copied from is as it was: its name, its turns and its
	// workers are all still there.
	for _, want := range []string{"- name: lead\n", "- human -> lead\n", "- lead -> builder\n", "purpose: 実装"} {
		if !strings.Contains(after, want) {
			t.Errorf("the save changed the member it copied: %q is gone:\n%s", want, after)
		}
	}
}

// TestTeamsSayARoleNameThatIsNotOne: a definition one of whose members plays a
// role that is not a role name cannot be built, so the definition's own check
// says it — in the figure's answer and in the list's problems — with the sentence
// the roles screen and the save use (r-fc16). The role's document is there, so
// nothing else about the definition stops.
func TestTeamsSayARoleNameThatIsNotOne(t *testing.T) {
	f := newFixture(t)
	f.write(t, filepath.Join("roles", "進行管理.yaml"), "version: 1\nsections:\n  - rules:\n      - id: SPEC-1\n        text: 何かをする。\n")
	text := "version: 1\nmission: demo\nturns:\n  - human -> lead\n  - lead -> human\nmembers:\n  - name: lead\n    role: 進行管理\n    profile: coder\n"
	f.write(t, filepath.Join("teams", "demo", "team.yaml"), text)
	server := f.server(t)

	resp, body := f.api(t, server, http.MethodPost, "/api/team/check", map[string]any{
		"place": "project", "name": "demo", "part": "team", "text": text,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/team/check answers %d: %s", resp.StatusCode, body)
	}
	answer := teamsCheck(t, body)
	found := false
	for _, problem := range answer.Problems {
		if problem.Sev == "err" && strings.Contains(problem.Text, "進行管理") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the check of a definition whose member plays 進行管理 found no error: %+v", answer.Problems)
	}

	_, body = f.get(t, server, "/teams")
	island := teamsIsland(t, body)
	for _, item := range island.Team.Items {
		if item.Name != "demo" {
			continue
		}
		if item.Problems.Err == 0 {
			t.Errorf("the list reads %+v for demo, want the role name's error", item.Problems)
		}
		return
	}
	t.Fatal("the list holds no demo")
}
