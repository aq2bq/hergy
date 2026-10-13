package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
)

// This file is the one place hergy decides what each part of itself is called.
// The glossary holds the English name, the Japanese name, a short definition,
// and the words that must not stand in for the term. The tests below hold every
// string a person or an agent reads to that table, so the same thing is not
// called one word in --help and another in the cheatsheet.

// ban is a word that must not name a term. lang is the language of the ban,
// which is also the language whose sources are checked and the name the fix is
// printed in.
type ban struct {
	lang string
	word string
}

// term is one thing hergy names, in both languages.
type term struct {
	id       string
	english  string
	japanese string
	meaning  string
	banned   []ban
}

// glossary is the table. An empty banned list is fine: a term with no word to
// forbid yet needs none.
var glossary = []term{
	{
		id:       "team-name",
		english:  "team name",
		japanese: "チーム名",
		meaning:  "the name a team runs under; --as changes it, and it is the first half of the herdr name",
	},
	{
		id:       "settings-name",
		english:  "settings name",
		japanese: "設定名",
		meaning:  "the directory name under teams/ that holds team.yaml; the default team name",
	},
	{
		id:       "seat",
		english:  "seat",
		japanese: "担当",
		meaning:  "one member written in team.yaml's members[].name; a short name",
		banned: []ban{
			// The short name in a placeholder or after --from is a seat. The
			// plain word "member" is left alone: it is the YAML key "members"
			// and the ordinary sense of a person on the team.
			{lang: "en", word: "<member>"},
			{lang: "en", word: "--from member"},
			// "member name" drifted in once the bans above were in place (d-5a13, drift 1).
			{lang: "en", word: "member name"},
		},
	},
	{
		id:       "herdr-name",
		english:  "herdr name",
		japanese: "herdr 名",
		meaning:  "the name herdr and the agents see: <team>-<seat>",
	},
	{
		id:       "worker",
		english:  "worker",
		japanese: "ワーカー",
		meaning:  "an agent a member hired with hergy hire, outside the roster",
	},
	{
		id:       "layer",
		english:  "layer",
		japanese: "層",
		meaning:  "one of the two settings roots: local (<project>/.hergy) and global ($XDG_CONFIG_HOME/hergy)",
	},
	{
		id:       "profile",
		english:  "profile",
		japanese: "プロファイル",
		meaning:  "a named engine (kind, args, env) in profiles.yaml",
	},
	{
		id:       "bootstrap-prompt",
		english:  "bootstrap prompt",
		japanese: "初期プロンプト",
		meaning:  "the first prompt hergy hands a member or a worker",
	},
	{
		id:       "cheatsheet",
		english:  "cheatsheet",
		japanese: "チートシート",
		meaning:  "hergy cheat's text; --ai prints the agent's one",
		banned: []ban{
			{lang: "ja", word: "早見"},
		},
	},
	{
		id:       "flag",
		english:  "flag",
		japanese: "オプション",
		meaning:  "a --word on the command line",
		banned: []ban{
			{lang: "ja", word: "旗"},
		},
	},
}

// TestGlossaryIsConsistent fails when the table contradicts itself: one name on
// two terms, or a name that is another term's forbidden word.
func TestGlossaryIsConsistent(t *testing.T) {
	byEnglish := map[string]string{}
	byJapanese := map[string]string{}
	byID := map[string]bool{}
	for _, tm := range glossary {
		if tm.id == "" || tm.english == "" || tm.japanese == "" || tm.meaning == "" {
			t.Errorf("term is incomplete (id %q): %+v", tm.id, tm)
		}
		if byID[tm.id] {
			t.Errorf("two terms share the id %q", tm.id)
		}
		byID[tm.id] = true
		if prev, ok := byEnglish[tm.english]; ok {
			t.Errorf("English name %q is on both %q and %q", tm.english, prev, tm.id)
		} else {
			byEnglish[tm.english] = tm.id
		}
		if prev, ok := byJapanese[tm.japanese]; ok {
			t.Errorf("Japanese name %q is on both %q and %q", tm.japanese, prev, tm.id)
		} else {
			byJapanese[tm.japanese] = tm.id
		}
	}

	for _, tm := range glossary {
		for _, b := range tm.banned {
			var names map[string]string
			switch b.lang {
			case "en":
				names = byEnglish
			case "ja":
				names = byJapanese
			default:
				t.Errorf("term %q bans %q in the unknown language %q", tm.id, b.word, b.lang)
				continue
			}
			if other, ok := names[b.word]; ok {
				t.Errorf("term %q forbids %q, but it is the name of %q", tm.id, b.word, other)
			}
		}
	}
}

// TestGlossaryBannedWordsStayOut fails when a forbidden word turns up in the
// strings a person or an agent reads. Each hit names the file and key (or line),
// the word, and the term to write instead.
func TestGlossaryBannedWordsStayOut(t *testing.T) {
	type hit struct {
		where string
		word  string
		term  term
		want  string
	}
	var hits []hit

	check := func(where, lang, text string) {
		for _, tm := range glossary {
			for _, b := range tm.banned {
				if b.lang != lang || !strings.Contains(text, b.word) {
					continue
				}
				want := tm.english
				if b.lang == "ja" {
					want = tm.japanese
				}
				hits = append(hits, hit{where: where, word: b.word, term: tm, want: want})
			}
		}
	}

	// The locales: every string a run can print.
	for _, lang := range []string{"en", "ja"} {
		path := "internal/i18n/locales/" + lang + ".json"
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var messages map[string]string
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("%s is not valid JSON: %v", path, err)
		}
		keys := make([]string, 0, len(messages))
		for key := range messages {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			check(path+":"+key, lang, messages[key])
		}
	}

	// The prose and the bundled skill, line by line so a hit has a line number.
	for _, src := range []struct{ path, lang string }{
		{"README.md", "en"},
		{"README.ja.md", "ja"},
		{"skills/hergy/SKILL.md", "en"},
	} {
		raw, err := os.ReadFile(src.path)
		if err != nil {
			t.Fatalf("read %s: %v", src.path, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			check(fmt.Sprintf("%s:%d", src.path, i+1), src.lang, line)
		}
	}

	// The agent cheatsheet is a Go constant, so it comes from the package.
	for i, line := range strings.Split(cheat.Machine, "\n") {
		check(fmt.Sprintf("internal/cheat/cheat.go:Machine:%d", i+1), "en", line)
	}

	if len(hits) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d banned word(s) found:\n", len(hits))
	for _, h := range hits {
		fmt.Fprintf(&b, "  %s: %q is banned for the term %q; write %q\n", h.where, h.word, h.term.id, h.want)
	}
	t.Fatal(b.String())
}
