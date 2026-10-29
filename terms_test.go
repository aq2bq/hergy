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
	// instead is the word to write, when it is not a glossary term. The master
	// rules the wording; a ban with no instead says the word is out and leaves
	// the way to say it to whoever rewrites the sentence.
	instead string
	// except is a locale key prefix left unchecked. Nothing uses it now: the
	// Web UI's own strings (web.*) name no layer either (設計 6 節).
	except string
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
		meaning:  "the name a running team goes by; --as changes it, and it is the first half of the herdr name",
	},
	{
		id:       "team-definition-name",
		english:  "team definition name",
		japanese: "チーム定義名",
		meaning:  "the directory name under teams/ that holds team.yaml; the default team name",
	},
	{
		id:       "team-definition",
		english:  "team definition",
		japanese: "チーム定義",
		meaning:  "how one team is built: the folder under teams/ that holds team.yaml; of two definitions of a name, the project's own is the one used",
	},
	{
		id:       "team",
		english:  "team",
		japanese: "チーム",
		meaning:  "one group started from a team definition; it goes by the team name, and it is what runs on herdr",
	},
	{
		id:       "mission",
		english:  "mission",
		japanese: "使命",
		meaning:  "what team.yaml's mission: says the team is for: one line, shown first by the commands and the screens",
	},
	{
		id:       "member",
		english:  "member",
		japanese: "メンバー",
		meaning:  "one entry of team.yaml's members; its short name is the member name",
		banned: []ban{
			// "seat" and 担当 were a third and fourth word for the same thing
			// (d-e425).
			{lang: "en", word: "seat"},
			{lang: "ja", word: "担当"},
		},
	},
	{
		id:       "member-name",
		english:  "member name",
		japanese: "メンバー名",
		meaning:  "the short name written in members[].name (lead); hergy puts the team name in front of it",
	},
	{
		id:       "herdr-name",
		english:  "herdr name",
		japanese: "herdr 名",
		meaning:  "the name herdr and the agents see: <team>-<member>",
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
		meaning:  "one of the two settings roots: the project (<project>/.hergy) and outside it ($XDG_CONFIG_HOME/hergy). It stays inside hergy: it is not a word a person reads (I11)",
		banned: []ban{
			// The layer is hergy's own word. A person reads the place by its
			// path, never by this name (I11, 段 7). The Web UI is checked too:
			// its rebuilt screens name no layer (設計 6 節).
			{lang: "ja", word: "層"},
			{lang: "en", word: "layer"},
		},
	},
	{
		id:       "global",
		english:  "global",
		japanese: "グローバル",
		meaning:  "one of the two places a settings file stands in: プロジェクト (<project>/.hergy) or グローバル (~/.config/hergy). A screen writes it beside プロジェクト",
		banned: []ban{
			// 共通 was the older Japanese name for the place (d-04d1).
			{lang: "ja", word: "共通", instead: "グローバル"},
		},
	},
	{
		id:       "profile",
		english:  "profile",
		japanese: "プロファイル",
		meaning:  "a named engine (kind, args, env) in profiles.yaml",
	},
	{
		id:       "profile-name",
		english:  "profile name",
		japanese: "プロファイル名",
		meaning:  "the name under profiles: in profiles.yaml; what a member's or worker's profile: names",
	},
	{
		id:       "kind",
		english:  "kind",
		japanese: "kind",
		meaning:  "the kind of agent (エージェントの種類) a profile starts, for example claude, codex or opencode (hergy keeps no closed list); written in a profile, not on a member, and the Japanese text is kind too",
	},
	{
		id:       "role",
		english:  "role",
		japanese: "ロール",
		meaning:  "what a member acts as; the member reads that role's document",
		banned: []ban{
			// 役割 was the old Japanese name for a role (合意 7, d-0ccd).
			{lang: "ja", word: "役割"},
		},
	},
	{
		id:       "role-document",
		english:  "role document",
		japanese: "ロールの文書",
		meaning:  "one role's rule document, roles/<role>.yaml; every member of that role reads it",
		banned: []ban{
			// 役割ファイル was the old Japanese name for a role document.
			{lang: "ja", word: "役割ファイル"},
		},
	},
	{
		id:       "role-name",
		english:  "role name",
		japanese: "ロール名",
		meaning:  "the file name under roles/ without .yaml; what team.yaml's role: names",
	},
	{
		id:       "team-agreement",
		english:  "team agreement",
		japanese: "チームのルール",
		meaning:  "the rules a team definition gives every member of that team, written in the definition's own folder",
	},
	{
		id:       "shared-agreement",
		english:  "shared agreement",
		japanese: "全チームのルール",
		meaning:  "the rules hergy used to read for every team of one place, kept in agreement.yaml beside the teams; it is not read any more (d-31ad), and what is left stops the commands that build a prompt",
	},
	{
		id:       "human",
		english:  "human",
		japanese: "人間",
		meaning:  "the one person outside every team; a team definition's turns write it human, and a Japanese rule text writes it 人間",
	},
	{
		id:       "hands-the-turn-to",
		english:  "hands the turn to",
		japanese: "ターンを渡す",
		meaning:  "the directed relation between two nodes that says which one passes the turn on; a team definition's turns write it <a> -> <b>, and a rule text still writes <a> → <b>",
		banned: []ban{
			// 手番 and 話しかけてよい were the old Japanese words for this
			// (合意 7, d-0ccd).
			{lang: "ja", word: "手番", instead: "ターン"},
			{lang: "ja", word: "話しかけてよい"},
		},
	},
	{
		id:       "node",
		english:  "node",
		japanese: "ノード",
		meaning:  "who receives and hands on the turn; today the human, a member, or a worker, and more kinds can be added",
	},
	{
		id:       "edge",
		english:  "edge",
		japanese: "エッジ",
		meaning:  "one node handing the turn to another; a member -> worker edge also carries the purpose and the profile",
	},
	{
		id:       "purpose",
		english:  "purpose",
		japanese: "目的",
		meaning:  "what a worker is hired for; workers: pairs each purpose with a member and a profile",
	},
	{
		id:       "bootstrap-prompt",
		english:  "bootstrap prompt",
		japanese: "初期プロンプト",
		meaning:  "the first prompt hergy hands a member or a worker",
	},
	{
		id:       "apply",
		english:  "apply",
		japanese: "反映",
		meaning:  "put a saved definition's drift into the teams running from it: hergy apply, the screens' 反映, POST /api/apply. hergy decides the way for each member: 送り直す (resend), 起動する (start), 起動し直す (restart), or 反映できない (cannot), where the reason and down・up are named",
		banned: []ban{
			// 変更を伝える and 伝えていない were the screens' and the prose's
			// words for 反映, and 伝える is tell's (d-3d30, n-f894).
			{lang: "ja", word: "変更を伝え", instead: "反映し"},
			{lang: "ja", word: "伝えていない", instead: "反映していない"},
		},
	},
	{
		id:       "tell",
		english:  "tell",
		japanese: "伝える",
		meaning:  "send one member a message from a person or another member: hergy tell. It is not the word for 反映",
	},
	{
		id:       "start",
		english:  "start",
		japanese: "起動",
		meaning:  "begin a team, or a member of one: hergy up starts a team, and hergy apply starts a member the definition has and no agent is live for; 起動し直し (refresh) is starting a member again",
		banned: []ban{
			// 起こす was the screens' own word for starting a member, and the
			// glossary and the command line had called it 起動 all along
			// (r-5d1a's follow-up). 起こる is another word and is left alone.
			{lang: "ja", word: "起こす", instead: "起動する"},
			{lang: "ja", word: "起こし", instead: "起動し"},
			{lang: "ja", word: "起こせ", instead: "起動でき"},
		},
	},
	{
		id:       "down",
		english:  "down",
		japanese: "終了する",
		meaning:  "finish a running team and close its panes with hergy down; do not call this チームを止める",
		banned: []ban{
			{lang: "ja", word: "チームを止める", instead: "チームを終了する"},
		},
	},
	{
		id:       "stop",
		english:  "stop",
		japanese: "止める",
		meaning:  "end one member's current turn with hergy stop; use this for a member's turn, not a team",
	},
	{
		id:       "refresh",
		english:  "refresh",
		japanese: "起動し直し",
		meaning:  "end a member's agent and start it again in the same pane, so its conversation starts empty; adding a member to a running team is apply",
	},
	{
		id:       "reprompt",
		english:  "reprompt",
		japanese: "送り直し",
		meaning:  "send the bootstrap prompt to a member again, keeping its conversation, with a preamble saying what changed when the definition moved",
		banned: []ban{
			// 再投入 was the old Japanese name for this (d-3d30).
			{lang: "ja", word: "再投入", instead: "送り直し"},
		},
	},
	{
		id:       "status",
		english:  "status",
		japanese: "状態",
		meaning:  "hergy status: one line per member and worker with its live agent state, then who is waiting on the human",
	},
	{
		id:       "waiting-for-permission",
		english:  "waiting for permission",
		japanese: "許可待ち",
		meaning:  "herdr's blocked: an agent is asking the human a question (a permission, or a question while it starts, such as whether to trust a folder) and waits for the answer",
		banned: []ban{
			// 承認待ち and 確認待ち were the other words for the same state, and
			// ブロック is herdr's word for it: hergy writes the state by what the
			// member is doing, and the question itself is 確認.
			{lang: "ja", word: "承認待ち"},
			{lang: "ja", word: "確認待ち"},
			{lang: "ja", word: "ブロック"},
		},
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
	{
		id:       "rule",
		english:  "rule",
		japanese: "規則",
		meaning:  "one rule of a team's agreement or role document: its id, its text, whether it is on, the day it was switched off, and a note for people",
	},
	{
		id:       "rule-note",
		english:  "note",
		japanese: "メモ",
		meaning:  "a record a person leaves with a rule: why it was written and what was seen; hergy never sends it to a member",
	},
	{
		id:       "origin",
		english:  "origin",
		japanese: "由来",
		meaning:  "where a rule came from: an incident, an instruction, or another rule it was derived from",
	},
	{
		id:       "reason",
		english:  "reason",
		japanese: "理由",
		meaning:  "why a rule is there; written as why, and sent unless send_why is false",
	},
	{
		id:       "in-prompt",
		english:  "in the prompt",
		japanese: "プロンプトに含める",
		meaning:  "whether a rule goes into a member's prompt; written in the file as in_prompt: false, and left unsaid when the rule goes in. A rule that is not in the prompt is a rule still: it keeps its id, its origin and its checks, and it is not counted in the amount a member is handed",
		banned: []ban{
			// メンバーに渡す was the older name for this switch (d-30a3).
			{lang: "ja", word: "メンバーに渡す"},
			{lang: "ja", word: "渡さない"},
		},
	},
	{
		id:       "check",
		english:  "check",
		japanese: "検証方法",
		meaning:  "how a rule is shown to have been kept: a report field or a test",
		banned: []ban{
			{lang: "ja", word: "確かめる手段"},
			{lang: "ja", word: "確かめ方"},
		},
	},
	{
		id:       "report-shape",
		english:  "report shape",
		japanese: "報告の形",
		meaning:  "the fields of one report, written under reports",
	},
	{
		id:       "incident",
		english:  "incident",
		japanese: "インシデント",
		meaning:  "something that went wrong and put a rule there; one kind of origin",
		banned: []ban{
			{lang: "ja", word: "事故"},
		},
	},
	{
		id:       "activity",
		english:  "activity",
		japanese: "アクティビティ",
		meaning:  "how a running team stands now: who is waiting on the human, what needs deciding, and why it is stopped",
		banned: []ban{
			{lang: "ja", word: "活動", instead: "アクティビティ"},
		},
	},
	{
		id:       "launch-record",
		english:  "launch record",
		japanese: "起動記録",
		meaning:  "what a running team was started from: the definition and the hash of its content at that time",
	},
	{
		id:       "undo",
		english:  "undo",
		japanese: "取り消す",
		meaning:  "take back the one operation just made (the notice's 取り消す and ⌘Z); a change already saved is not brought back by it",
	},
	{
		id:       "revert",
		english:  "revert",
		japanese: "元に戻す",
		meaning:  "throw away every change not saved in this panel and write the file's own text again (the save band's 元に戻す)",
	},
	{
		id:       "running-column",
		english:  "Running",
		japanese: "動き",
		meaning:  "the column of the team definitions list that says whether a team of that definition is running, and starts or stops it",
	},
	{
		id:       "find",
		english:  "Find",
		japanese: "探す",
		meaning:  "the field in the band that finds running teams, team definitions, workspaces, members, roles, rules and profiles across the listed projects and the global settings",
		banned: []ban{
			// 検索 was the other word for this field; the screens say 探す.
			{lang: "ja", word: "検索", instead: "探す"},
		},
	},
	{
		id:       "waiting-for-the-human",
		english:  "waiting for the human",
		japanese: "人間を待っている",
		meaning:  "the group at the top of the activity screen: members waiting for permission and members that handed the turn to the human",
	},
	{
		id:       "headquarters",
		english:  "headquarters",
		japanese: "本部",
		meaning:  "hergy serve reading every project listed under projects: in the global config.yaml (d-73fc). The screens use it in the sheet's title 本部に並べるプロジェクト / Projects in the headquarters and in the notice about an unlisted project, and nowhere else",
	},
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
		if b.lang != lang || !strings.Contains(text, b.word) {
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
			check(path+":"+key, key, lang, messages[key])
		}
	}

	// The prose and the bundled skills, line by line so a hit has a line number.
	for _, src := range []struct{ path, lang string }{
		{"README.md", "en"},
		{"README.ja.md", "ja"},
		{"skills/hergy/SKILL.md", "en"},
		{"skills/hergy-rules/SKILL.md", "en"},
		// The API's written promise carries Japanese descriptions too, so the
		// place words a person reads there stay in the glossary (r-dfe5).
		{"internal/apicontract/openapi.yaml", "ja"},
	} {
		raw, err := os.ReadFile(src.path)
		if err != nil {
			t.Fatalf("read %s: %v", src.path, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			check(fmt.Sprintf("%s:%d", src.path, i+1), "", src.lang, line)
		}
	}

	// The agent cheatsheet is a Go constant, so it comes from the package.
	for i, line := range strings.Split(cheat.Machine, "\n") {
		check(fmt.Sprintf("internal/cheat/cheat.go:Machine:%d", i+1), "", "en", line)
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
				return nil
			}
			if !screenSource(path) {
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
				return nil
			}
			if !screenSource(path) {
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
