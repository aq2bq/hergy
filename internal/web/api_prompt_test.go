package web

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
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
	if out.Current == nil || *out.Current != built.Text || out.Recorded != nil || !reflect.DeepEqual(out.Segments, built.Segments) {
		t.Fatalf("%+v", out)
	}
	in.Doc = json.RawMessage(`{"ops":[{"kind":"addTurn","from":"lead","to":"human"}]}`)
	out, failure = s.comparePrompt(in)
	if failure != nil || out.Current == nil || !out.Draft || *out.Current == built.Text {
		t.Fatalf("out=%+v failure=%+v", out, failure)
	}
	var joined strings.Builder
	for _, segment := range out.Segments {
		joined.WriteString(segment.Text)
	}
	if joined.String() != *out.Current {
		t.Fatal("draft segments differ from current")
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
	if response.StatusCode != 200 || out.Current != nil || len(out.Segments) != 0 || len(out.Problems) == 0 {
		t.Fatalf("%d %s", response.StatusCode, body)
	}
}

type promptReaderFixture struct {
	Activity
	history PromptHistory
	runs    []PromptRun
	runsErr error
}

func (f promptReaderFixture) PromptRuns(_, _, _ string) ([]PromptRun, error) {
	return f.runs, f.runsErr
}
func (f promptReaderFixture) PromptHistory(_, _, _, _, _ string) (PromptHistory, error) {
	return f.history, nil
}
func TestPromptComparisonPreservesHistoryForInvalidDraft(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	s.act = promptReaderFixture{
		runs:    []PromptRun{{Team: "demo", Project: ""}},
		history: PromptHistory{Status: "recorded", Recorded: &RecordedPrompt{Text: "old\r\n \t", Command: "up", Time: "2026-11-04T00:00:00Z"}},
	}
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

func TestPromptComparisonSegmentsUseRunningAlias(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	s.act = promptReaderFixture{
		runs:    []PromptRun{{Team: "sample-run", Project: ""}},
		history: PromptHistory{Status: "missing"},
	}
	out, failure := s.comparePrompt(promptComparisonIn{apiIn: apiIn{Name: "demo", Place: "project", Hash: source.Hash(readFile(t, f.teamYAML())), Doc: json.RawMessage(`{"ops":[]}`)}, Member: "lead"})
	if failure != nil || out.Current == nil {
		t.Fatalf("%+v %+v", out, failure)
	}
	var joined strings.Builder
	for _, segment := range out.Segments {
		joined.WriteString(segment.Text)
	}
	if joined.String() != *out.Current || !strings.Contains(joined.String(), "sample-run-lead") {
		t.Fatal("segments must describe the running alias, not the definition preview")
	}
}

// TestPromptsForChoosesTheName: ⑤ r-ba9b — the one name rule. The text carries
// the name the answer names, and says why when the live reading fails.
func TestPromptsForChoosesTheName(t *testing.T) {
	f := newFixture(t)
	text := readFile(t, f.teamYAML())
	base := promptComparisonIn{apiIn: apiIn{Name: "demo", Place: "project", Hash: source.Hash(text), Doc: json.RawMessage(`{"ops":[]}`)}, Member: "lead"}
	cases := []struct {
		name    string
		runs    []PromptRun
		runsErr error
		picked  string
		team    string
		kind    string
		status  string
	}{
		{"none running", nil, nil, "", "demo", "definition", "none"},
		{"one running", []PromptRun{{Team: "demo", Project: f.proj}}, nil, "", "demo", "run", "single"},
		{"one alias", []PromptRun{{Team: "demo-hotfix", Project: f.proj}}, nil, "", "demo-hotfix", "run", "single"},
		{"two unchosen", []PromptRun{{Team: "demo", Project: f.proj}, {Team: "demo-hotfix", Project: f.proj}}, nil, "", "demo", "definition", "choose"},
		{"two chosen", []PromptRun{{Team: "demo", Project: f.proj}, {Team: "demo-hotfix", Project: f.proj}}, nil, "demo-hotfix", "demo-hotfix", "run", "picked"},
		{"pick stopped", []PromptRun{{Team: "demo", Project: f.proj}}, nil, "demo-old", "demo", "definition", "choose"},
		{"unreadable", nil, errors.New("herdr is not there"), "", "demo", "definition", "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := f.server(t)
			s.act = promptReaderFixture{runs: tc.runs, runsErr: tc.runsErr}
			in := base
			in.RunningTeam = tc.picked
			built, failure := s.promptsFor(promptAsk{In: in, WithRuns: true})
			if failure != nil {
				t.Fatal(failure)
			}
			if built.Names.Team != tc.team || built.Names.Kind != tc.kind || built.Names.Status != tc.status {
				t.Fatalf("names = %+v, want %s/%s/%s", built.Names, tc.team, tc.kind, tc.status)
			}
			if tc.status == "error" && built.Names.Reason == "" {
				t.Error("an unreadable live reading says no reason")
			}
			if len(built.Built) != 1 {
				t.Fatalf("built %d texts, want the one member", len(built.Built))
			}
			var joined strings.Builder
			for _, segment := range built.Built[0].Segments {
				joined.WriteString(segment.Text)
			}
			if !strings.Contains(joined.String(), tc.team+"-lead") {
				t.Errorf("the text does not name %s-lead: %s", tc.team, joined.String())
			}
		})
	}
}

// TestWorkerPreviewUsesTheRunningNameAndTheDraft: ⑤ r-ba9b — the worker's
// preview is the text hire --dry builds: the running team's name when one runs,
// and the draft's unsaved edits.
func TestWorkerPreviewUsesTheRunningNameAndTheDraft(t *testing.T) {
	f := newFixture(t)
	hash := source.Hash(readFile(t, f.teamYAML()))
	ask := func(doc string) apiIn {
		return apiIn{Name: "demo", Place: "project", Hash: hash, Doc: json.RawMessage(doc)}
	}
	preview := func(t *testing.T, s *Server, doc string) string {
		t.Helper()
		answer, refusal := (apiTeam{s: s}).checkWorkerPrompt(ask(doc))
		if refusal != nil {
			t.Fatal(refusal)
		}
		var text strings.Builder
		for _, segment := range answer.(apiCheck).Prompt {
			text.WriteString(segment.Text)
		}
		return text.String()
	}
	want := func(t *testing.T, team, leadRole, purpose string) string {
		t.Helper()
		built, err := prompt.Worker(prompt.WorkerInput{
			Team: team, Lead: team + "-lead", LeadRole: leadRole,
			Purpose: purpose, Name: team + "-helper", Task: "作る",
		})
		if err != nil {
			t.Fatal(err)
		}
		return built.Text
	}
	const plain = `{"name":"helper","purpose":"試作","task":"作る","from":"lead","preview":true}`
	cases := []struct {
		name string
		runs []PromptRun
		team string
	}{
		{"no running team", nil, "demo"},
		{"one running team", []PromptRun{{Team: "demo", Project: ""}}, "demo"},
		{"an alias", []PromptRun{{Team: "demo-hotfix", Project: ""}}, "demo-hotfix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := f.server(t)
			s.act = promptReaderFixture{runs: tc.runs}
			if got, expected := preview(t, s, plain), want(t, tc.team, "lead", "試作"); got != expected {
				t.Errorf("the preview is not the hire's text for %s", tc.team)
			}
		})
	}
	t.Run("unsaved edits", func(t *testing.T) {
		s := f.server(t)
		const draft = `{"name":"helper","purpose":"新しい目的","task":"作る","from":"lead","preview":true,` +
			`"ops":[{"kind":"memberRole","member":"lead","role":"builder"},{"kind":"addWorker","from":"lead","purpose":"新しい目的","profile":"coder"}]}`
		if got, expected := preview(t, s, draft), want(t, "demo", "builder", "新しい目的"); got != expected {
			t.Error("the preview does not carry the draft's role and purpose")
		}
	})
}

func TestWorkerPromptChecksShortNameOnlyOutsidePreview(t *testing.T) {
	f := newFixture(t)
	for _, preview := range []bool{false, true} {
		doc, _ := json.Marshal(map[string]any{"name": "1", "purpose": "試作", "task": "作る", "from": "lead", "preview": preview})
		_, refusal := (apiTeam{s: f.server(t)}).checkWorkerPrompt(apiIn{Name: "demo", Place: "project", Hash: source.Hash(readFile(t, f.teamYAML())), Doc: doc})
		if preview && refusal != nil {
			t.Fatalf("preview: %v", refusal)
		}
		if !preview && (refusal == nil || refusal.Text != i18n.T("error.worker_name_pattern")) {
			t.Fatalf("normal: %+v", refusal)
		}
	}
}
