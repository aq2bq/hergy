package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

func TestPromptComparisonCurrentMatchesMemberAndNeverWrites(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	before := readFile(t, f.teamYAML())
	in := promptComparisonIn{apiIn: apiIn{Name: "demo", Place: "project", Hash: source.Hash(before), Doc: json.RawMessage(`{"ops":[]}`)}, Member: "lead"}
	out, failure := s.comparePrompt(in)
	if failure != nil {
		t.Fatal(failure)
	}
	def, err := s.cat.Definition("demo", "local")
	if err != nil {
		t.Fatal(err)
	}
	node, _ := def.Graph.Member("lead")
	built, err := prompt.Member(def, &node, prompt.Run{Team: "demo", Base: s.cat.Base(), Executable: promptExecutable(), Reader: s.cat.Reader(), Sources: s.cat, Display: s.cat.Display})
	if err != nil {
		t.Fatal(err)
	}
	if out.Current == nil || *out.Current != built.Text || out.Recorded != nil {
		t.Fatalf("%+v", out)
	}
	in.Doc = json.RawMessage(`{"ops":[{"kind":"addTurn","from":"lead","to":"human"}]}`)
	out, failure = s.comparePrompt(in)
	if failure != nil || out.Current == nil || !out.Draft || *out.Current == built.Text {
		t.Fatalf("out=%+v failure=%+v", out, failure)
	}
	if readFile(t, f.teamYAML()) != before {
		t.Fatal("comparison saved the draft")
	}
	in.Hash = "other"
	if _, failure = s.comparePrompt(in); failure == nil || failure.status != 409 {
		t.Fatalf("hash: %+v", failure)
	}
}
func TestPromptComparisonInvalidDraftAndRoute(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	text := readFile(t, f.teamYAML())
	payload := map[string]any{"name": "demo", "place": "project", "hash": source.Hash(text), "member": "lead", "doc": map[string]any{"ops": []any{}}}
	response, body := f.api(t, s, "POST", "/api/team/prompt-comparison", payload)
	if response.StatusCode != 200 || !strings.Contains(body, `"current":`) {
		t.Fatalf("%d %s", response.StatusCode, body)
	}
	f.write(t, "teams/demo/team.yaml", strings.ReplaceAll(text, "role: lead", "role: missing"))
	payload["hash"] = source.Hash(readFile(t, f.teamYAML()))
	response, body = f.api(t, s, "POST", "/api/team/prompt-comparison", payload)
	var out promptComparison
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || out.Current != nil || len(out.Problems) == 0 {
		t.Fatalf("%d %s", response.StatusCode, body)
	}
}

type promptReaderFixture struct {
	Activity
	history PromptHistory
}

func (f promptReaderFixture) PromptHistory(_, _, _, _, _ string) (PromptHistory, error) {
	return f.history, nil
}
func TestPromptComparisonPreservesHistoryForInvalidDraft(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	s.act = promptReaderFixture{history: PromptHistory{Runs: []PromptRun{}, Status: "recorded", Recorded: &RecordedPrompt{Text: "old\r\n \t", Command: "up", Time: "2026-11-04T00:00:00Z"}}}
	text := readFile(t, f.teamYAML())
	text = strings.ReplaceAll(text, "role: lead", "role: absent")
	f.write(t, "teams/demo/team.yaml", text)
	out, failure := s.comparePrompt(promptComparisonIn{apiIn: apiIn{Name: "demo", Place: "project", Hash: source.Hash(text), Doc: json.RawMessage(`{"ops":[]}`)}, Member: "lead"})
	if failure != nil || out.Current != nil || out.Recorded == nil || out.Recorded.Text != "old\r\n \t" {
		t.Fatalf("%+v %+v", out, failure)
	}
}
func TestPromptComparisonRejectsTraversalAndUnknownTargets(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	for _, name := range []string{"../demo", "/demo", ""} {
		response, _ := f.api(t, s, "POST", "/api/team/prompt-comparison", map[string]any{"place": "project", "name": name, "member": "lead", "doc": map[string]any{"ops": []any{}}})
		if response.StatusCode != 422 {
			t.Fatal(response.StatusCode, name)
		}
	}
}
