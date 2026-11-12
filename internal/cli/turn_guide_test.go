package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// The contact guide's lines, exactly as they stand in the locale (r-103d,
// d-8b4e, d-e601): a member hands its result on with hergy tell and stops, text
// for the person is written in the pane and marked with hergy tell 人間, a stop
// without a tell is asked about, and
// nobody uses herdr agent prompt. A test names the whole line, not a word: a
// shorter rewording must fail.
var turnGuide = map[string][]string{
	"ja": {
		"- 結果と、次に誰かにしてほしいことは、`hergy tell <名前> \"<本文>\"` で渡してから止まる。名前は名簿の短い名前（lead など）。",
		"- 人間に読んでほしい文は、自分のペインに書き、`hergy tell 人間` と渡してから止まる。",
		"- 誰にも渡さずに止まると、hergy が「誰に渡しますか」と聞く。名前 1 語か「人間」か「誰にも」で答える。",
		"- `herdr agent prompt` は使わない。hergy が誰のターンか分からなくなる。",
	},
	"en": {
		"- Hand your result, and what you need someone to do next, on with `hergy tell <name> \"<text>\"`, then stop. The name is the short one in the roster (lead, for example).",
		"- Text for the person: write it in your own pane, hand it on with `hergy tell human`, then stop.",
		"- If you stop without handing on, hergy asks who it is for. Answer with one name, \"human\" or \"nobody\".",
		"- Do not use `herdr agent prompt`: hergy would lose whose turn it is.",
	},
}

// workerTurnGuide is the contact guide a hired worker reads: it hands its
// result to the member that hired it with hergy tell, questions go the same
// way, and it adds no teammates (r-dee4, r-103d).
var workerTurnGuide = map[string][]string{
	"ja": {
		"- 結果は `hergy tell <追加したメンバー> \"<本文>\"` で渡してから止まる。途中で聞きたいことも、同じ道で渡す。",
		"- 渡さずに止まると、hergy が「誰に渡しますか」と聞く。名前 1 語で答える。",
		"- `herdr agent prompt` は使わない。hergy が誰のターンか分からなくなる。",
	},
	"en": {
		"- Hand your result on with `hergy tell <the member that hired you> \"<text>\"`, then stop. A question before you are done goes the same way.",
		"- If you stop without handing on, hergy asks who it is for. Answer with one name.",
		"- Do not use `herdr agent prompt`: hergy would lose whose turn it is.",
	},
}

// TestBootstrapPromptsCarryTheTurnGuide: every member's up --dry prompt and the
// worker's hire --dry prompt carry the turn lines of their own, in both languages
// (r-e54b, ac-a516, r-dee4).
func TestBootstrapPromptsCarryTheTurnGuide(t *testing.T) {
	for _, lang := range []string{"ja", "en"} {
		t.Run(lang, func(t *testing.T) {
			workersProject(t, newFixture(t))
			up := mustRun(t, "up", "dev", "--dry", "--json", "--lang", lang)
			var plan struct {
				Members []struct {
					Name   string `json:"name"`
					Prompt string `json:"prompt"`
				} `json:"members"`
			}
			if err := json.Unmarshal([]byte(up), &plan); err != nil {
				t.Fatalf("up --dry --json is not JSON: %v\n%s", err, up)
			}
			if len(plan.Members) == 0 {
				t.Fatal("up --dry --json has no member")
			}
			for _, member := range plan.Members {
				for _, line := range turnGuide[lang] {
					if !hasPromptLine(member.Prompt, line) {
						t.Errorf("%s's prompt does not carry the turn line %q:\n%s", member.Name, line, member.Prompt)
					}
				}
			}

			hire := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry", "--json", "--lang", lang)
			var worker struct {
				Members []struct {
					Name   string `json:"name"`
					Prompt string `json:"prompt"`
				} `json:"members"`
			}
			if err := json.Unmarshal([]byte(hire), &worker); err != nil {
				t.Fatalf("hire --dry --json is not JSON: %v\n%s", err, hire)
			}
			if len(worker.Members) != 1 {
				t.Fatalf("hire --dry --json has %d members, want one", len(worker.Members))
			}
			for _, line := range workerTurnGuide[lang] {
				if !hasPromptLine(worker.Members[0].Prompt, line) {
					t.Errorf("the worker's prompt does not carry the turn line %q:\n%s", line, worker.Members[0].Prompt)
				}
			}
		})
	}
}

// hasPromptLine says one whole line of a prompt is line, not merely a substring.
func hasPromptLine(prompt, line string) bool {
	return slices.Contains(strings.Split(prompt, "\n"), line)
}

// TestWorkerPromptHandsTheResultOnWithTell: a hired worker is told to hand its
// result to the member that hired it with hergy tell and stop (r-dee4, d-de32,
// r-103d). The lines that told it to send with herdr agent prompt, that its
// pane does not reach the member, and that the watcher hands its pane on are
// gone.
func TestWorkerPromptHandsTheResultOnWithTell(t *testing.T) {
	tests := []struct {
		lang   string
		gone   []string
		wanted []string
	}{
		{
			lang: "ja",
			gone: []string{
				"herdr agent prompt dev-lead",
				"待ち続ける",
				"自分のペインに書いて止まることは、人間にターンを渡すこと",
				"hergy の見張りが dev-lead へ届ける",
			},
			wanted: []string{
				"終わったら、結果を `hergy tell dev-lead \"<本文>\"` で渡して止まる。",
				"- 結果は `hergy tell <追加したメンバー> \"<本文>\"` で渡してから止まる。途中で聞きたいことも、同じ道で渡す。",
			},
		},
		{
			lang: "en",
			gone: []string{
				"herdr agent prompt dev-lead",
				"keeps waiting",
				"Writing in your own pane and stopping hands the turn to the human",
				"hergy's watcher hands it to dev-lead",
			},
			wanted: []string{
				"When you are done, hand the result on with `hergy tell dev-lead \"<text>\"` and stop.",
				"- Hand your result on with `hergy tell <the member that hired you> \"<text>\"`, then stop. A question before you are done goes the same way.",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.lang, func(t *testing.T) {
			workersProject(t, newFixture(t))
			got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry", "--json", "--lang", tc.lang)
			var plan struct {
				Members []struct {
					Prompt string `json:"prompt"`
				} `json:"members"`
			}
			if err := json.Unmarshal([]byte(got), &plan); err != nil {
				t.Fatalf("hire --dry --json is not JSON: %v\n%s", err, got)
			}
			if len(plan.Members) != 1 {
				t.Fatalf("hire --dry --json has %d members, want one", len(plan.Members))
			}
			prompt := plan.Members[0].Prompt
			for _, gone := range tc.gone {
				if strings.Contains(prompt, gone) {
					t.Errorf("the worker's prompt still says %q:\n%s", gone, prompt)
				}
			}
			for _, want := range tc.wanted {
				if !strings.Contains(prompt, want) {
					t.Errorf("the worker's prompt does not say %q:\n%s", want, prompt)
				}
			}
		})
	}
}
