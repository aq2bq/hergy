package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// This file is the measurement of d-040e (E1, E4): every sentence hergy prints
// when it stops tells the person what happened and what to do next. The next
// step is said in Japanese with ください, in the words of a command, a screen or
// a place in a file; a sentence that only says what went wrong does not pass.
// The English sentence is held to the same keys and the same replacements, so a
// reader of either language is told the same thing.

// errorKey is the family of keys a person reads when a run stops: the errors of
// a command, and the errors a web screen shows.
var errorKey = regexp.MustCompile(`^(error|web\.error)\.`)

// errorParts are the keys that are not a sentence of their own: they are pieces
// another sentence is built with, so they cannot say a next step. Each one is
// here with the reason it is a piece, and nothing else may join them.
var errorParts = map[string]string{
	"error.prefix":         "全エラー文の前に付く接頭辞で、後ろの文が次にすることを言う",
	"error.profiles_moved": "team.yaml の読み取りエラーの後ろに添える括弧の文で、単独では出ない",
	"web.error.title":      "画面の見出しだけで、次にすることを言うのは同じ画面のほかの鍵",
}

// nextStep is how a Japanese error sentence says what to do next.
const nextStep = "ください"

// TestEveryErrorSaysTheNextStep holds every error.* and web.error.* key to
// d-040e: the Japanese sentence says the next step, the English sentence is
// there too, and both write the same replacements in the same order.
func TestEveryErrorSaysTheNextStep(t *testing.T) {
	locales := map[string]map[string]string{}
	for _, lang := range []string{"ja", "en"} {
		locales[lang] = readLocale(t, lang)
	}
	for key, why := range errorParts {
		if why == "" {
			t.Errorf("%s is exempted from saying the next step with no reason written", key)
		}
		for _, lang := range []string{"ja", "en"} {
			if _, ok := locales[lang][key]; !ok {
				t.Errorf("%s is exempted from saying the next step, but the %s locale has no such key", key, lang)
			}
		}
	}

	keys := make([]string, 0, len(locales["ja"]))
	for key := range locales["ja"] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !errorKey.MatchString(key) {
			continue
		}
		if _, ok := errorParts[key]; ok {
			continue
		}
		if !strings.Contains(locales["ja"][key], nextStep) {
			t.Errorf("%s: the Japanese sentence does not say what to do next (%q): %s", key, nextStep, locales["ja"][key])
		}
		en, ok := locales["en"][key]
		if !ok {
			t.Errorf("%s is in the ja locale but not in the en locale", key)
			continue
		}
		want, got := replacements(locales["ja"][key]), replacements(en)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: ja writes the replacements %v but en writes %v; a translation keeps the same arguments in the same order", key, want, got)
		}
	}
}

// readLocale reads one locale the binary ships.
func readLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	path := filepath.Join("internal", "i18n", "locales", lang+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var messages map[string]string
	if err := json.Unmarshal(raw, &messages); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	return messages
}

// replacements is the arguments a sentence writes, in order. %% is a percent
// sign and not an argument, and a width or an index (%[2]s, %-10v) stays with
// the argument it belongs to.
func replacements(text string) []string {
	var out []string
	for i := 0; i < len(text); i++ {
		if text[i] != '%' {
			continue
		}
		if i+1 < len(text) && text[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(text) && !unicode.IsLetter(rune(text[j])) {
			j++
		}
		if j < len(text) {
			out = append(out, text[i:j+1])
		}
		i = j
	}
	return out
}
