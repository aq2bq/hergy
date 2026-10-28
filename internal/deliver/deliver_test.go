package deliver

import (
	"os"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	if err := i18n.Use("ja"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCompareReadsRulesTurnsAndWorkers(t *testing.T) {
	oldDoc := "version: 1\nsections:\n  - rules:\n      - id: C1\n        text: 古い本文。\n"
	nowDoc := "version: 1\nsections:\n  - rules:\n      - id: C1\n        text: 新しい本文。\n      - id: C2\n        text: 足した本文。\n"
	oldPrompt := "## Team agreements\n\n" + oldDoc + "\n## ターンを渡す相手\nあなたがターンを渡す相手:\n- demo-reviewer（ロール reviewer）\n"
	current := definition.Start{
		Name: "demo-lead", Short: "lead", Role: "lead", Prompt: "new prompt",
		Segments: []definition.PromptSegment{
			{Kind: "team_rules", Text: "\n" + nowDoc, Origin: definition.PromptOrigin{Path: "demo/agreement.yaml"}},
			{Kind: "turns", Text: "\n## ターンを渡す相手\nあなたがターンを渡す相手:\n- demo-builder（ロール builder）\n"},
			{Kind: "workers", Text: "\n## ワーカーの追加\n必要なときは、次の「目的 → プロファイル」の組でワーカーを追加してよい: 校正 → cheap\n"},
		},
		Turns:   []string{"demo-builder"},
		Workers: []definition.WorkerNode{{From: "lead", Purpose: "校正", Profile: "cheap"}},
	}
	changes := Compare(oldPrompt, map[string]string{
		".hergy/teams/demo/agreement.yaml": oldDoc,
	}, current, []Doc{{
		Path: ".hergy/teams/demo/agreement.yaml", Display: "demo/agreement.yaml", Text: nowDoc,
	}})
	var kinds []string
	for _, change := range changes {
		kinds = append(kinds, change.Kind+":"+change.Rule+change.Turn+change.Worker)
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{"rule:C1", "rule:C2", "turn:lead → builder", "worker:校正"} {
		if !strings.Contains(joined, want) {
			t.Errorf("changes = %+v, want %s", changes, want)
		}
	}
	for _, change := range changes {
		if change.Text == "" {
			t.Errorf("change %+v has no sentence", change)
		}
	}
}

func TestCompareIsEmptyForTheSamePrompt(t *testing.T) {
	current := definition.Start{Prompt: "same"}
	if changes := Compare("same", nil, current, nil); len(changes) != 0 {
		t.Errorf("changes = %+v, want none", changes)
	}
	if changes := Compare("", nil, current, nil); len(changes) != 1 || changes[0].Kind != "no_record" {
		t.Errorf("no record = %+v, want the no_record change", changes)
	}
}

// TestPreambleSaysWhatChangedAndWhatToDo: a send with changes is headed with the
// reason, each change, and the words that make the text the new bootstrap prompt
// without restarting the work. A send with no change gets no preamble.
func TestPreambleSaysWhatChangedAndWhatToDo(t *testing.T) {
	if got := Preamble(nil); got != "" {
		t.Errorf("a send with no change is headed with %q, want nothing", got)
	}
	got := Preamble([]Change{{Kind: "rule", Text: "L1 を直した"}, {Kind: "roster", Text: "名簿が変わった"}})
	for _, want := range []string{
		i18n.T("apply.preamble_lead"),
		"- L1 を直した",
		"- 名簿が変わった",
		i18n.T("apply.preamble_close"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("preamble = %q, want %q", got, want)
		}
	}
}

// TestAChangedWorkerProfileIsNotAnAddedWorker: a purpose that already stood in
// the old note with another profile is a profile change, so a person is told
// that, not that the worker was added (d-36ef).
func TestAChangedWorkerProfileIsNotAnAddedWorker(t *testing.T) {
	segment := "\n## ワーカーの追加\n必要なときは、次の「目的 → プロファイル」の組でワーカーを追加してよい: 校正 → cheap\n"
	old := "## ワーカーの追加\n必要なときは、次の「目的 → プロファイル」の組でワーカーを追加してよい: 校正 → cheap\n"
	current := definition.Start{Workers: []definition.WorkerNode{{Purpose: "校正", Profile: "good"}}}
	changes := workerChanges(current, segment, old)
	if len(changes) != 1 || changes[0].Kind != "worker_profile" {
		t.Fatalf("changes = %+v, want one worker_profile change", changes)
	}
	if !strings.Contains(changes[0].Text, "校正") {
		t.Errorf("the text = %q, want the purpose named", changes[0].Text)
	}
}
