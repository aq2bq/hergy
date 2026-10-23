package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// The three lines the designer wrote for r-e54b, exactly as they stand in the
// locale. A test names the whole line, not a word: a shorter rewording must fail.
var turnGuide = map[string][]string{
	"ja": {
		"- 仲間に送ることは、その相手にターンを渡すこと。自分のペインに書いて止まることは、人間にターンを渡すこと。どちらも、渡したら止まって相手からの連絡を待つ。",
		"- ターンを持っている間は、頼まれた仕事が終わるまで止まらない。渡したあとにする仕事を予告しない（「このあと〜に進みます」と書かない）。続ける仕事があるなら渡す前に済ませるか、続けてよいかを聞いて答えを待つ。",
		"- 連絡を受け取ったら、ターンは自分にある。相手が続けると書いていても、相手は待っている。次の仕事を渡すか、その仕事を終わりにする。",
	},
	"en": {
		"- Sending to a teammate hands them the turn. Writing in your own pane and stopping hands the turn to the human. Either way, once you hand it over, stop and wait for their message.",
		"- While the turn is yours, do not stop until the work you were given is done. Do not promise work for after you hand the turn over (do not write \"next I will move on to ...\"). If there is more to do, finish it before you hand over, or ask whether to go on and wait for the answer.",
		"- When a message reaches you, the turn is yours. Even if the sender wrote that they would go on, they are waiting. Hand them the next piece of work, or bring that work to an end.",
	},
}

// TestBootstrapPromptsCarryTheTurnGuide: every member's up --dry prompt and the
// worker's hire --dry prompt carry the three turn lines, in both languages
// (r-e54b, ac-a516).
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
			for _, line := range turnGuide[lang] {
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
