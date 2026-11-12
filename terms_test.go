package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/words"
)

// Canonical names live in internal/words; bans guard handwritten text only.

// ban is a word that must not name a term. lang is the language of the ban,
// which is also the language whose sources are checked and the name the fix is
// printed in.
type ban struct {
	lang string
	word string
	// instead is the word to write, when it is not a glossary term. The master
	// rules the wording; a ban with no instead says the word is out and leaves
	// the way to say it to whoever rewrites the sentence.
	instead string
	// except is a locale key prefix left unchecked. It is how a refusal may
	// name the old key it tells the person to remove (error.mission_gone).
	except string
	// whole says the word is checked as a whole word, not as any part of a
	// longer one: mission: is a ban that must not catch "permission".
	whole bool
}

type term struct {
	id                         string
	english, japanese, meaning string
	banned                     []ban
}

var glossary = buildGlossary()

func buildGlossary() []term {
	bans := []term{
		{id: "team-name"},
		{id: "team-definition-name"},
		{id: "team-definition"},
		{id: "team"},
		{id: "description", banned: []ban{
			// mission: was the old key on team.yaml and the old word for this
			// line (d-5f55): a definition that still writes it is stopped with
			// the sentence that says how to rewrite it, and no other sentence
			// names it. The refusal's own message is the one place the old key
			// is written, because it is what a person has to remove.
			{lang: "en", word: "mission", except: "error.mission_gone", whole: true},
			{lang: "ja", word: "使命", except: "error.mission_gone"},
		}},
		{id: "member", banned: []ban{
			// "seat" and 担当 were a third and fourth word for the same thing
			// (d-e425).
			{lang: "en", word: "seat"},
			{lang: "ja", word: "担当"},
		}},
		{id: "member-name"},
		{id: "herdr-name"},
		{id: "worker"},
		{id: "layer", banned: []ban{
			// The layer is hergy's own word. A person reads the place by its
			// path, never by this name (I11, 段 7). The Web UI is checked too:
			// its rebuilt screens name no layer (設計 6 節).
			{lang: "ja", word: "層"},
			{lang: "en", word: "layer"},
		}},
		{id: "global", banned: []ban{
			// 共通 was the older Japanese name for the place (d-04d1).
			{lang: "ja", word: "共通", instead: "グローバル"},
		}},
		{id: "place", banned: []ban{
			// ロケーション was the master's temporary name for the two places;
			// the screen, the API's promises and the glossary say 置き場所 (d-e77c).
			{lang: "ja", word: "ロケーション", instead: "置き場所"},
		}},
		{id: "preferred", banned: []ban{
			{lang: "ja", word: "効いている", instead: "優先"},
		}},
		{id: "inactive", banned: []ban{
			{lang: "ja", word: "隠れている", instead: "無効"},
		}},
		{id: "profile"},
		{id: "profile-name"},
		{id: "kind"},
		{id: "role", banned: []ban{
			// 役割 was the old Japanese name for a role (合意 7, d-0ccd).
			{lang: "ja", word: "役割"},
		}},
		{id: "role-document", banned: []ban{
			// 役割ファイル was the old Japanese name for a role document.
			{lang: "ja", word: "役割ファイル"},
		}},
		{id: "role-name"},
		{id: "team-agreement"},
		{id: "shared-agreement"},
		{id: "human"},
		{id: "hands-the-turn-to", banned: []ban{
			// 手番 and 話しかけてよい were the old Japanese words for this
			// (合意 7, d-0ccd).
			{lang: "ja", word: "手番", instead: "ターン"},
			{lang: "ja", word: "話しかけてよい"},
		}},
		{id: "node"},
		{id: "edge"},
		{id: "purpose"},
		{id: "bootstrap-prompt"},
		{id: "apply", banned: []ban{
			// 変更を伝える and 伝えていない were the screens' and the prose's
			// words for 反映, and 伝える is tell's (d-3d30, n-f894).
			{lang: "ja", word: "変更を伝え", instead: "反映し"},
			{lang: "ja", word: "伝えていない", instead: "反映していない"},
		}},
		{id: "tell"},
		{id: "start", banned: []ban{
			// 起こす was the screens' own word for starting a member, and the
			// glossary and the command line had called it 起動 all along
			// (r-5d1a's follow-up). 起こる is another word and is left alone.
			{lang: "ja", word: "起こす", instead: "起動する"},
			{lang: "ja", word: "起こし", instead: "起動し"},
			{lang: "ja", word: "起こせ", instead: "起動でき"},
		}},
		{id: "down", banned: []ban{
			{lang: "ja", word: "チームを止める", instead: "チームを終了する"},
		}},
		{id: "stop"},
		{id: "refresh"},
		{id: "reprompt", banned: []ban{
			// 再投入 was the old Japanese name for this (d-3d30).
			{lang: "ja", word: "再投入", instead: "送り直し"},
		}},
		{id: "status"},
		{id: "waiting-for-permission", banned: []ban{
			// 承認待ち and 確認待ち were the other words for the same state, and
			// ブロック is herdr's word for it: hergy writes the state by what the
			// member is doing, and the question itself is 確認.
			{lang: "ja", word: "承認待ち"},
			{lang: "ja", word: "確認待ち"},
			{lang: "ja", word: "ブロック"},
		}},
		{id: "handed", banned: []ban{
			// 人間に渡した is the past event, and 人間の番 brings a second word
			// for the turn: neither reads as who is waiting now (d-7b45).
			{lang: "ja", word: "人間に渡した", instead: "人間の返事待ち"},
			{lang: "ja", word: "人間の番", instead: "人間の返事待ち"},
		}},
		{id: "cheatsheet", banned: []ban{
			{lang: "ja", word: "早見"},
		}},
		{id: "flag", banned: []ban{
			{lang: "ja", word: "旗"},
		}},
		{id: "rule", banned: []ban{{lang: "ja", word: "散文"}}},
		{id: "rule-note"},
		{id: "origin"},
		{id: "reason"},
		{id: "in-prompt", banned: []ban{
			// メンバーに渡す was the older name for this switch (d-30a3).
			{lang: "ja", word: "メンバーに渡す"},
			{lang: "ja", word: "渡さない"},
		}},
		{id: "check", banned: []ban{
			{lang: "ja", word: "確かめる手段"},
			{lang: "ja", word: "確かめ方"},
		}},
		{id: "report-shape"},
		{id: "incident", banned: []ban{
			{lang: "ja", word: "事故"},
		}},
		{id: "activity", banned: []ban{
			{lang: "ja", word: "活動", instead: "アクティビティ"},
		}},
		{id: "launch-record"},
		{id: "undo"},
		{id: "revert"},
		{id: "settings-check"},
		{id: "revision-number"},
		{id: "size-guide"},
		{id: "running-column"},
		{id: "find", banned: []ban{
			// 検索 was the other word for this field; the screens say 探す.
			{lang: "ja", word: "検索", instead: "探す"},
		}},
		{id: "waiting-for-the-human"},
		{id: "headquarters"},
	}
	byID := map[string][]ban{}
	for _, t := range bans {
		byID[t.id] = t.banned
	}
	var out []term
	for _, t := range words.Terms() {
		out = append(out, term{id: string(t.ID), english: t.English, japanese: t.Japanese, meaning: t.Meaning, banned: byID[string(t.ID)]})
	}
	return out
}

// bannedWords are words hergy does not use for anything, ruled out by the
// master. They stand for no term of their own, so they live away from the
// table: instead, when set, is the word to write, and without it the sentence
// is written another way.
var bannedWords = []ban{
	{lang: "ja", word: "台帳"},
	{lang: "ja", word: "つながり"},
	{lang: "ja", word: "棚卸し"},
	// 約束 was the old Japanese name for a team's or a layer's rules; the
	// rules themselves are 規則, and the place they are gathered is ルール.
	{lang: "ja", word: "約束", instead: "ルール"},
	{lang: "ja", word: "版", instead: "バージョン"},
	// role file was the old English name for a role document (段 7, spec/28 H3).
	{lang: "en", word: "role file", instead: "role document"},
	// 開き直して was how a refused save asked the person to start over (r-3c77,
	// d-a34a): the screen keeps what they wrote, so no sentence sends them back
	// to a fresh page.
	{lang: "ja", word: "開き直して"},
	// ディスク was the word the same sentences used for the file (r-3c77, d-a34a):
	// a person reads and writes ファイル, and hergy does not say which medium it
	// stands on.
	{lang: "ja", word: "ディスク", instead: "ファイル"},
	// 起動… was the start button's word (r-df61, d-b023): the dots are the
	// convention that a press asks first, and a person who does not know it
	// cannot read what the button does. The button names its act.
	{lang: "ja", word: "起動…"},
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
		for _, b := range append(append([]ban{}, tm.banned...), bannedWords...) {
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
			if other, ok := names[b.word]; ok && other != tm.id {
				t.Errorf("term %q forbids %q, but it is the name of %q", tm.id, b.word, other)
			}
		}
	}
}

// banHits reports whether one ban finds its word in a text. A whole-word ban
// looks for the word with no ASCII letter on either side, so mission: does not
// catch "permission"; every other ban is a plain part of the text.
func banHits(text string, b ban) bool {
	if !b.whole {
		return strings.Contains(text, b.word)
	}
	for i := 0; i+len(b.word) <= len(text); {
		at := strings.Index(text[i:], b.word)
		if at < 0 {
			return false
		}
		from, to := i+at, i+at+len(b.word)
		if (from == 0 || !asciiLetter(text[from-1])) && (to == len(text) || !asciiLetter(text[to])) {
			return true
		}
		i = from + 1
	}
	return false
}

// asciiLetter reports whether one byte is an ASCII letter.
func asciiLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// TestGlossaryBannedWordsStayOut fails when a forbidden word turns up in the
// strings a person or an agent reads. Each hit names the file and key (or line),
// the word, and the term to write instead.
func TestGlossaryBannedWordsStayOut(t *testing.T) {
	type hit struct {
		where string
		word  string
		term  string // the term it stands for, empty when it stands for none
		want  string // the word to write; empty when the sentence is written another way
	}
	var hits []hit

	add := func(where, lang, text, termID, termName string, b ban) {
		if b.lang != lang || !banHits(text, b) {
			return
		}
		want := b.instead
		if want == "" {
			want = termName
		}
		hits = append(hits, hit{where: where, word: b.word, term: termID, want: want})
	}

	check := func(where, key, lang, text string) {
		for _, tm := range glossary {
			name := tm.english
			if lang == "ja" {
				name = tm.japanese
			}
			for _, b := range tm.banned {
				if b.except != "" && strings.HasPrefix(key, b.except) {
					continue
				}
				add(where, lang, text, tm.id, name, b)
			}
		}
		for _, b := range bannedWords {
			if b.except != "" && strings.HasPrefix(key, b.except) {
				continue
			}
			add(where, lang, text, "", "", b)
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
			if !words.GeneratedLocaleKey(key) {
				check(path+":"+key, key, lang, messages[key])
			}
		}
	}

	// The rules: internal/words is where the sentences of help, both cheats and
	// the generated parts of README and SKILL are written, once. Those outputs
	// are not read again below, so a banned word is caught here, at the one
	// place a person writes it, and the hit names the rule (n-42bb).
	for _, rule := range words.Rules() {
		for _, lang := range []string{"en", "ja"} {
			where := "internal/words rule " + string(rule.ID)
			check(where, "", lang, words.Describe([]words.RuleID{rule.ID}, lang))
			check(where+" (statement)", "", lang, rule.Statement[lang])
			check(where+" (recovery)", "", lang, rule.Recovery[lang])
		}
	}

	// The prose and the bundled skills, line by line so a hit has a line number.
	for _, src := range []struct{ path, lang string }{
		{"README.md", "en"},
		{"README.ja.md", "ja"},
		{"skills/hergy/SKILL.md", "en"},
		// The API's written promise carries Japanese descriptions too, so the
		// place words a person reads there stay in the glossary (r-dfe5).
		{"internal/apicontract/openapi.yaml", "ja"},
	} {
		raw, err := os.ReadFile(src.path)
		if err != nil {
			t.Fatalf("read %s: %v", src.path, err)
		}
		for i, line := range strings.Split(words.Handwritten(string(raw)), "\n") {
			check(fmt.Sprintf("%s:%d", src.path, i+1), "", src.lang, line)
		}
	}

	// The screens' own words: their modules, styles and templates write Japanese
	// too, and a word that stays out of the locales must stay out of them
	// (r-5d1a's follow-up). The screens are written in src/ and built into
	// static/, so their words are read there, once (r-893d).
	for _, dir := range screensSources {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if path == "internal/web/src/lib/terms.generated.ts" || !screenSource(path) {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(raw), "\n") {
				check(fmt.Sprintf("%s:%d", path, i+1), "", "ja", line)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
	}

	if len(hits) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d banned word(s) found:\n", len(hits))
	for _, h := range hits {
		switch {
		case h.want == "":
			fmt.Fprintf(&b, "  %s: %q is a banned word; write it another way\n", h.where, h.word)
		case h.term == "":
			fmt.Fprintf(&b, "  %s: %q is a banned word; write %q\n", h.where, h.word, h.want)
		default:
			fmt.Fprintf(&b, "  %s: %q is banned for the term %q; write %q\n", h.where, h.word, h.term, h.want)
		}
	}
	t.Fatal(b.String())
}

// screensSources is where the screens' own words stand: their modules under
// src/, the styles beside them in static/ (which the build does not write), and
// the templates. The scripts under static/ are what webbuild makes of src/, so
// their words are read at their source and not twice here.
var screensSources = []string{
	"internal/web/src",
	"internal/web/static",
	"internal/web/templates",
}

// screenSource reports whether one file holds the screens' own words: a module
// under src/, or a style or a template beside it. The scripts static/ holds are
// the build's, and their words are read at their source.
func screenSource(path string) bool {
	switch filepath.Ext(path) {
	case ".css", ".html":
		return true
	}
	if !strings.HasPrefix(filepath.ToSlash(path), "internal/web/src/") {
		return false
	}
	switch filepath.Ext(path) {
	case ".js", ".jsx", ".ts", ".tsx":
		return true
	}
	return false
}

// TestScreensNameTheirNameFields: a screen writes the name of the thing it names
// with the glossary's word, not 「名前」 alone (ac-ae5f). A field or a column is
// the element's own text (「ロール名」「チーム定義名」「メンバー名」「プロファイ
// ル名」), and a search or a label is the same word; the walk over the screens'
// sources catches the bare word, so one word cannot drift back.
func TestScreensNameTheirNameFields(t *testing.T) {
	bare := regexp.MustCompile(`>名前<|placeholder="名前"|aria-label="名前"`)
	var hits []string
	for _, dir := range screensSources {
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() != "." && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if path == "internal/web/src/lib/terms.generated.ts" || !screenSource(path) {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(raw), "\n") {
				if bare.MatchString(line) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
	}
	if len(hits) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("a screen writes 「名前」 alone; write the glossary's word (ロール名・チーム定義名・メンバー名・プロファイル名):\n")
	for _, hit := range hits {
		b.WriteString("  " + hit + "\n")
	}
	t.Fatal(b.String())
}
