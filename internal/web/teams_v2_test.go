package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// teamsIsland reads the data island out of a teams page.
func teamsIsland(t *testing.T, body string) t2Island {
	t.Helper()
	const open = `<script id="teams-data" type="application/json">`
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatalf("the teams page carries no island:\n%s", body)
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "</script>")
	if j < 0 {
		t.Fatalf("the island is not closed:\n%s", body)
	}
	var out t2Island
	if err := json.Unmarshal([]byte(rest[:j]), &out); err != nil {
		t.Fatalf("the island does not read: %v", err)
	}
	return out
}

// teamsAnswer reads one POST answer.
func teamsAnswer(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the answer does not read: %v\n%s", err, body)
	}
	return out
}

// TestTeamsListDrawsTheIsland: the list page carries the definition with its
// shape, its two tabs' data, the figure, the prompt's own lines and the
// agreement, all of it from check.Team and catalog (spec-teams 1.1, 1.4).
func TestTeamsListDrawsTheIsland(t *testing.T) {
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
	if len(island.Defs) != 1 || island.Defs[0].Name != "demo" {
		t.Fatalf("the island's definitions read %+v, want the one demo", island.Defs)
	}
	def := island.Defs[0]
	if def.Broken {
		t.Errorf("the fixture's demo definition reads as broken")
	}
	if def.Mission != "demo の使命" {
		t.Errorf("the mission reads %q", def.Mission)
	}
	if len(def.Team.Members) != 2 || len(def.Team.Workers) != 3 || len(def.Team.Turns) != 3 {
		t.Errorf("the shape reads %d members, %d workers, %d turns", len(def.Team.Members), len(def.Team.Workers), len(def.Team.Turns))
	}
	if !def.HasAgree || def.Rules != 1 {
		t.Errorf("the agreement reads hasAgree=%v rules=%d, want one rule", def.HasAgree, def.Rules)
	}
	if def.AgreeReport == nil || len(def.AgreeReport.Markdown.Lines) == 0 {
		t.Errorf("the agreement check carries no Markdown")
	}
	// The figure's node order is the human first, then the members.
	if len(def.Findings) > 0 {
		t.Logf("the fixture's findings: %+v", def.Findings)
	}
	// The prompt's own line for what lead hands to builder (X3): the screen
	// never builds this string itself.
	want := tf("turns.member_item", "demo-builder", "builder")
	if !containsLine(def.Prompt.Turns["lead"], want) {
		t.Errorf("lead's turn lines read %v, want %q", def.Prompt.Turns["lead"], want)
	}
}

// containsLine is whether lines holds one line, exactly.
func containsLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

// TestTeamsCheckAndEffect: a check applies the panel's ops and answers the
// figure's data; an effect answers the prompt's lines and the problems the one
// more op would add (spec-teams 2, C2, C4).
func TestTeamsCheckAndEffect(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)

	resp, body := f.post(t, server, "/teams/check", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[{"kind":"removeTurn","from":"human","to":"lead"}]}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/check answers %d: %s", resp.StatusCode, body)
	}
	answer := teamsAnswer(t, body)
	var findings []t2Finding
	if err := json.Unmarshal(answer["findings"], &findings); err != nil {
		t.Fatal(err)
	}
	// The human no longer hands the turn on: an error that stops (I12a).
	if !hasErr(findings) {
		t.Errorf("removing the human's turn found no error: %+v", findings)
	}
	var graph struct {
		Nodes      []struct{ ID string } `json:"nodes"`
		SavedTurns [][2]string           `json:"savedTurns"`
	}
	if err := json.Unmarshal(answer["graph"], &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 0 {
		t.Errorf("a graph the draft broke reads %d nodes, want none", len(graph.Nodes))
	}
	if len(graph.SavedTurns) != 3 {
		t.Errorf("the saved turns read %d, want the three the file writes", len(graph.SavedTurns))
	}

	// An op that keeps the graph valid still answers the figure's nodes.
	resp, body = f.post(t, server, "/teams/check", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[{"kind":"memberProfile","member":"builder","profile":"planner"}]}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/check answers %d: %s", resp.StatusCode, body)
	}
	answer = teamsAnswer(t, body)
	if err := json.Unmarshal(answer["graph"], &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 3 || graph.Nodes[0].ID != "human" || graph.Nodes[2].ID != "builder" {
		t.Errorf("the figure's nodes read %+v, want the human first", graph.Nodes)
	}

	resp, body = f.post(t, server, "/teams/effect", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[],"op":{"kind":"removeTurn","from":"lead","to":"builder"}}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/effect answers %d: %s", resp.StatusCode, body)
	}
	var effectAnswer struct {
		Lines []struct {
			Member  string   `json:"member"`
			Part    string   `json:"part"`
			Removed []string `json:"removed"`
		} `json:"lines"`
	}
	if err := json.Unmarshal([]byte(body), &effectAnswer); err != nil {
		t.Fatal(err)
	}
	if len(effectAnswer.Lines) == 0 {
		t.Fatalf("the effect of removing lead -> builder found no line:\n%s", body)
	}
	if effectAnswer.Lines[0].Member != "demo-lead" || effectAnswer.Lines[0].Part != "turns" {
		t.Errorf("the effect's line reads %+v, want demo-lead's turns", effectAnswer.Lines[0])
	}
	if want := tf("turns.member_item", "demo-builder", "builder"); !containsLine(effectAnswer.Lines[0].Removed, want) {
		t.Errorf("the removed lines read %v, want %q", effectAnswer.Lines[0].Removed, want)
	}
}

// hasErr is whether a finding stops the definition.
func hasErr(findings []t2Finding) bool {
	for _, finding := range findings {
		if finding.Sev == "err" {
			return true
		}
	}
	return false
}

// TestTeamsSaveWritesTheOneLine: a save writes the ops with byte-exact edits,
// so every line a person did not touch, a comment among them, comes back as it
// was (spec-teams 1.3, X1).
func TestTeamsSaveWritesTheOneLine(t *testing.T) {
	f := newFixture(t)
	f.write(t, "teams/demo/team.yaml", "version: 1\n# a comment a person wrote\nmission: demo の使命\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\n  - name: builder\n    role: builder\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\nworkers:\n  - from: lead\n    purpose: 実装\n    profile: coder\n  - from: lead\n    purpose: 試作\n    profile: coder\n  - from: lead\n    purpose: 調査\n    profile: planner\n")
	server := f.server(t)
	_, page := f.get(t, server, "/teams")
	hash := teamsIsland(t, page).Defs[0].Hash
	before := readFile(t, f.teamYAML())

	resp, body := f.post(t, server, "/teams/save", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[{"kind":"mission","text":"新しい使命"}],"hash":"` + hash + `"}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/save answers %d: %s", resp.StatusCode, body)
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

// TestTeamsAgreementCheckSharesThePart: the agreement's check answers the same
// DocReport shape the role screen reads (spec-teams 7, C6).
func TestTeamsAgreementCheckSharesThePart(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	doc := `{"name":"","title":"demo チームの約束","revision":1,"sections":[{"heading":"使命","rules":[{"id":"TEAM-1","text":"このチームの約束。","why":"だから。","send_why":true,"in_prompt":true,"note":"","report":"","tests":[],"origin":[]}]}]}`
	resp, body := f.post(t, server, "/teams/agreement/check", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[],"doc":` + doc + `}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/agreement/check answers %d: %s", resp.StatusCode, body)
	}
	var report struct {
		Markdown struct {
			Lines []struct {
				Text string `json:"text"`
			} `json:"lines"`
			Chars int `json:"chars"`
		} `json:"markdown"`
		Readers []struct {
			Member string `json:"member"`
		} `json:"readers"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Markdown.Lines) == 0 || report.Markdown.Chars == 0 {
		t.Errorf("the agreement's Markdown is empty:\n%s", body)
	}
	if len(report.Readers) != 2 {
		t.Errorf("the readers read %d, want the two members", len(report.Readers))
	}
}

// TestTeamsYamlCheckAndSplit: the YAML tab's file name reads, and a check finds
// a profile that is not there (spec-teams 1.8, 2).
func TestTeamsYamlCheckAndSplit(t *testing.T) {
	name, place, kind, ok := t2SplitFile("demo.project.team")
	if !ok || name != "demo" || place != "project" || kind != "team" {
		t.Errorf("demo.project.team reads %q %q %q %v", name, place, kind, ok)
	}
	if _, _, _, ok := t2SplitFile("demo.nope.team"); ok {
		t.Errorf("a file with an unknown place reads as valid")
	}
	if _, _, _, ok := t2SplitFile("nosuch"); ok {
		t.Errorf("a file with no dots reads as valid")
	}

	f := newFixture(t)
	server := f.server(t)
	text := "version: 1\nmission: demo\nturns:\n  - human -> lead\nmembers:\n  - name: lead\n    role: lead\n    profile: nosuch\n"
	resp, body := f.post(t, server, "/teams/yaml/check", url.Values{
		"payload": {`{"file":"demo.project.team","text":` + jsonString(text) + `}`},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /teams/yaml/check answers %d: %s", resp.StatusCode, body)
	}
	var answer struct {
		Problems []t2YamlProblem `json:"problems"`
	}
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		t.Fatal(err)
	}
	if len(answer.Problems) == 0 || answer.Problems[0].Sev != "err" {
		t.Errorf("a member with a missing profile found no error: %+v", answer.Problems)
	}
}

// jsonString is one Go string as a JSON string.
func jsonString(s string) string {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// TestTeamsNoLayerStopsAWrite: a project whose .hergy is gone has no place for
// a team.yaml, and the write says so with the sentence C5 added (spec-teams
// 1.7, X6).
func TestTeamsNoLayerStopsAWrite(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	// Remove the project's own settings directory: the layer is not there.
	if err := os.RemoveAll(filepath.Join(f.proj, ".hergy")); err != nil {
		t.Fatal(err)
	}
	resp, body := f.post(t, server, "/teams/save", url.Values{
		"payload": {`{"definition":"demo","place":"project","ops":[],"hash":""}`},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST /teams/save answers %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "teams/") {
		t.Errorf("the refusal does not say teams/ cannot be written:\n%s", body)
	}
}
