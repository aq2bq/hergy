package rules

import (
	"slices"
	"testing"
)

// TestModelWordsFindsTheNamesAsTheyAreWritten: a rule's text that names the
// engine or the model is reported, whatever the case, at a word boundary, once
// each and in the written order (C8).
func TestModelWordsFindsTheNamesAsTheyAreWritten(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"an engine", "設計は claude に渡す", []string{"claude"}},
		{"a model family", "設計は opus に書かせる", []string{"opus"}},
		{"two names in the written order", "設計は opus、実装は sonnet", []string{"opus", "sonnet"}},
		{"a version after the family", "GPT-4 と gpt-4o", []string{"GPT-4", "gpt-4o"}},
		{"a family herdr offers", "qwen で書く", []string{"qwen"}},
		{"a family with a dot", "deepseek-v4.1 を使う", []string{"deepseek-v4.1"}},
		{"a name a sentence ends on", "claude. に頼む", []string{"claude"}},
		{"the same name twice, whatever the case", "Claude と claude", []string{"Claude"}},
		{"a word the family only begins", "gptx はモデルではない", nil},
		{"a word that contains an engine", "clause は別の語", nil},
		{"a digit after a family", "opus2x はモデルではない", nil},
		{"an engine inside a word", "pine は木", nil},
		{"nothing", "何かをする。", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ModelWords(tc.text); !slices.Equal(got, tc.want) {
				t.Errorf("ModelWords(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
