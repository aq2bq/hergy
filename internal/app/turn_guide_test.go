package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// The contact guide's lines, exactly as they stand in the locale (r-bc26,
// ac-2d71): a member writes in its own pane and stops, hergy's watcher hands it
// on, and nobody sends a turn by hand. A test names the whole line, not a word:
// a shorter rewording must fail.
var turnGuide = map[string][]string{
	"ja": {
		"- 結果と、次に誰かにしてほしいことを、自分のペインに書いて止まる。hergy の見張りが、チーム定義の turns が名指す相手へ届ける（相手が作業中なら、止まった瞬間に届く）。",
		"- 渡し先を変えたいときだけ、最後の行に「→ <名前>」か「→ 人間」と書く。人間に読んでほしい文も、同じように「→ 人間」と書く。",
		"- `herdr agent prompt` は使わない。使うと hergy の見張りと二重になる。",
	},
	"en": {
		"- Write your result, and what you need someone to do next, in your own pane and stop. hergy's watcher hands it to the party the definition's turns name (one that is working gets it the moment it stops).",
		"- Only to change where it goes, write \"→ <name>\" or \"→ human\" as the last line. Text for the person goes the same way: \"→ human\".",
		"- Do not use `herdr agent prompt`: it would double what the watcher hands on.",
	},
}

// workerTurnGuide is the contact guide a hired worker reads: its pane is handed
// to the member that hired it, questions go the same way, and it adds no
// teammates (r-dee4, r-bc26).
var workerTurnGuide = map[string][]string{
	"ja": {
		"- 結果を自分のペインに書いて止まる。hergy の見張りが、あなたを追加したメンバーへ届ける。",
		"- 途中で聞きたいことがあるときも、自分のペインに書いて止まる。同じ道で届く。",
		"- `herdr agent prompt` は使わない。使うと hergy の見張りと二重になる。",
	},
	"en": {
		"- Write your result in your own pane and stop. hergy's watcher hands it to the member that hired you.",
		"- To ask something before you are done, write it in your own pane and stop the same way.",
		"- Do not use `herdr agent prompt`: it would double what the watcher hands on.",
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

// TestWorkerPromptWritesTheResultInItsPane: a hired worker is told to write its
// result in its own pane and stop, and that hergy's watcher hands it to the
// member that hired it (r-dee4, d-de32, r-bc26). The lines that told it to send
// with herdr agent prompt, and that its pane does not reach the member, are gone.
func TestWorkerPromptWritesTheResultInItsPane(t *testing.T) {
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
			},
			wanted: []string{
				"終わったら、結果を自分のペインに書いて止まる。hergy の見張りが dev-lead へ届ける。",
				"途中で聞きたいことがあるときも、自分のペインに書いて止まる。同じ道で届く。",
			},
		},
		{
			lang: "en",
			gone: []string{
				"herdr agent prompt dev-lead",
				"keeps waiting",
				"Writing in your own pane and stopping hands the turn to the human",
			},
			wanted: []string{
				"When you are done, write the result in your own pane and stop. hergy's watcher hands it to dev-lead.",
				"To ask something before you are done, write it in your own pane and stop the same way.",
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
