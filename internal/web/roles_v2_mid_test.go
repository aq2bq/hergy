package web

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

var regexpOffSince = regexp.MustCompile(`(?m)^        off_since: .*\n`)

// This file holds the 小・中 checks of the role screen that are held without a
// browser: the preview a member's prompt carries (ROLES-M1) and the writes the
// role screen sends (ROLES-F1, F2, F4, F5, F6, F7). The fixture is the
// real-scale one (testdata/roles-v2), the same one the mock and the e2e walks
// were drawn on.
//
// Every helper here is this file's own: another worker's test file holds its own
// copier and its own block readers, and the names never meet.

// ruFixtureRoles copies the real-scale roles fixture into a working project and
// an outside settings directory, and answers a fixture whose server reads it.
func ruFixtureRoles(t *testing.T) *fixture {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "roles-v2")
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config")
	copyTree(t, filepath.Join(src, "project"), proj)
	copyTree(t, filepath.Join(src, "config", "hergy"), global)
	return &fixture{root: root, proj: proj, global: global}
}

// ruRolePath is where one layer keeps one role document, the way the catalog
// reads it.
func ruRolePath(f *fixture, place, name string) string {
	if place == catalog.PlaceGlobal {
		return filepath.Join(f.global, "roles", name+".yaml")
	}
	return filepath.Join(f.proj, ".hergy", "roles", name+".yaml")
}

// ruDraftOf is one parsed document as the draft a panel sends: every section and
// every rule carries where it stood in the saved document, so the server can
// write the changed lines and carry a moved rule or section whole (the from the
// page's own toDoc adds).
func ruDraftOf(t *testing.T, doc *rules.Document, hash, text string) *r2Doc {
	t.Helper()
	out := r2DocOf(doc, hash, text)
	for si := range out.Sections {
		at := si
		out.Sections[si].From = &at
		for ri := range out.Sections[si].Rules {
			out.Sections[si].Rules[ri].From = &r2From{Section: si, Rule: ri}
		}
	}
	return out
}

// ruLoadDraft reads one role file and answers the draft a panel would send for
// it, with the text it was read from.
func ruLoadDraft(t *testing.T, path string) (*r2Doc, string) {
	t.Helper()
	text := readFile(t, path)
	parsed, err := rules.Parse(text, path)
	if err != nil {
		t.Fatal(err)
	}
	return ruDraftOf(t, parsed, source.Hash(text), text), text
}

// ruRuleAt finds one rule of a document by its id.
func ruRuleAt(t *testing.T, doc *rules.Document, id string) (int, int) {
	t.Helper()
	for si := range doc.Sections {
		for ri := range doc.Sections[si].Rules {
			if doc.Sections[si].Rules[ri].ID == id {
				return si, ri
			}
		}
	}
	t.Fatalf("the document holds no rule %s", id)
	return -1, -1
}

// ruChangedLines is the 1-based numbers of the lines that differ between two
// texts, with the line each became — how a save is held to the one line it was
// meant to write.
func ruChangedLines(before, after string) map[int]string {
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	out := map[int]string{}
	n := max(len(a), len(b))
	for i := range n {
		l, r := "", ""
		if i < len(b) {
			l = b[i]
		}
		if i < len(a) {
			r = a[i]
		}
		if l != r {
			out[i+1] = r
		}
	}
	return out
}

// ruSameLines holds a save to moving lines about rather than changing them: the
// file comes back with the same lines in another order.
func ruSameLines(t *testing.T, before, after string) {
	t.Helper()
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(b) != len(a) {
		t.Fatalf("the file went from %d lines to %d:\n%s", len(b), len(a), after)
	}
	sb, sa := append([]string(nil), b...), append([]string(nil), a...)
	sort.Strings(sb)
	sort.Strings(sa)
	if strings.Join(sb, "\n") != strings.Join(sa, "\n") {
		t.Fatalf("the file's lines changed, not only their order:\n%s", after)
	}
}

// ruItemBlock is one rule's own lines: its "- id:" line and the keys under it, up
// to the next line that is indented less.
func ruItemBlock(t *testing.T, text, id string) string {
	t.Helper()
	return ruBlock(t, text, "- id: "+id, "        ")
}

// ruSectionBlock is one section's own lines: its "- heading:" line and the lines
// under it.
func ruSectionBlock(t *testing.T, text, heading string) string {
	t.Helper()
	return ruBlock(t, text, "- heading: "+heading, "    ")
}

// ruBlock is the lines from the one a heading stands on, down to the first line
// that is not indented deeper than it.
func ruBlock(t *testing.T, text, head, deeper string) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == head {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the file no longer holds %q:\n%s", head, text)
	}
	end := start + 1
	for end < len(lines) && strings.HasPrefix(lines[end], deeper) {
		end++
	}
	return strings.Join(lines[start:end], "\n")
}

// ruMemberPrompt builds one member's bootstrap prompt for the definition that
// first plays a role: the initial prompt the role document's own part stands in
// (ROLES-M1).
func ruMemberPrompt(t *testing.T, cat *catalog.Catalog, role string) (*prompt.Built, error) {
	t.Helper()
	users := cat.RoleUsers(role)
	if len(users) == 0 {
		t.Fatalf("no definition plays the role %s", role)
	}
	def, err := cat.Definition(users[0].Definition, "")
	if err != nil {
		t.Fatalf("the definition %s does not read: %v", users[0].Definition, err)
	}
	for i := range def.Graph.Members {
		if def.Graph.Members[i].Role != role {
			continue
		}
		node := &def.Graph.Members[i]
		return prompt.Member(def, node, prompt.Run{
			Team: def.Name, Reader: cat.Reader(), Sources: cat, Display: cat.Display, Base: cat.Base(),
		})
	}
	t.Fatalf("no member of %s plays the role %s", def.Name, role)
	return nil, nil
}

// ruRoleDoc reads one role document through the catalog and parses it under the
// path a person reads it by.
func ruRoleDoc(t *testing.T, cat *catalog.Catalog, role string) (source.Doc, *rules.Document) {
	t.Helper()
	doc, ok := cat.Role(role)
	if !ok {
		t.Fatalf("no layer holds a document for %s", role)
	}
	text, err := cat.Reader().Read(doc)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := rules.Parse(text, cat.Display(doc.Path))
	if err != nil {
		t.Fatal(err)
	}
	return doc, parsed
}

// ruNumbered is the Markdown preview's "N 行 · N 文字" as a check answers it: the
// lines that carry a number (a rule kept out of the prompt carries none) and the
// characters of the text a member reads.
func ruNumbered(preview map[string]any) (int, int) {
	lines, _ := preview["lines"].([]any)
	n := 0
	for _, l := range lines {
		one, _ := l.(map[string]any)
		if one["part"] != rules.PartExcluded {
			n++
		}
	}
	chars, _ := preview["chars"].(float64)
	return n, int(chars)
}

// ruIssues is a check's problems as the sentences a screen shows, in order.
func ruIssues(problems []any) []string {
	out := make([]string, 0, len(problems))
	for _, p := range problems {
		one, _ := p.(map[string]any)
		text, _ := one["text"].(string)
		out = append(out, text)
	}
	return out
}

// ruErrorTexts is a check's error problems' sentences.
func ruErrorTexts(problems []any) map[string]bool {
	out := map[string]bool{}
	for _, p := range problems {
		one, _ := p.(map[string]any)
		if one["sev"] != rules.SevError {
			continue
		}
		text, _ := one["text"].(string)
		out[text] = true
	}
	return out
}

// TestROLES_M1_ThePreviewIsThePrompt'sRolePart: the Markdown a role document's
// preview draws is the very part the member's initial prompt carries, one
// character not differing, and a reason kept out of the prompt (send_why: false)
// and a rule kept out of the prompt (in_prompt: false) stand as struck lines
// with no number, inside neither the "N 行 · N 文字" nor the amount a member
// reads.
func TestROLES_M1_ThePreviewIsThePromptsRolePart(t *testing.T) {
	cat := ruFixtureRoles(t).catalog()

	// The six roles the fixture holds. The preview's numbered lines, joined with
	// a line break, are exactly the role-document part of the prompt: the same
	// text, with no line a member does not read and no line missing.
	for _, role := range []string{"analyst", "builder", "lead", "scout", "reviewer", "writer"} {
		t.Run(role, func(t *testing.T) {
			doc, parsed := ruRoleDoc(t, cat, role)
			var numbered []string
			for _, line := range parsed.PreviewLines() {
				if line.Part != rules.PartExcluded {
					numbered = append(numbered, line.Text)
				}
			}
			region := strings.Join(numbered, "\n")
			built, err := ruMemberPrompt(t, cat, role)
			if err != nil {
				// builder's document carries the persisted problem (BUILD-4's
				// derived origin points at REQ-5, which no member reads): the
				// prompt's cross-document check refuses before it carries the
				// role document. That is builder's own problem, not a preview
				// that drifts, so the preview is still the document a prompt
				// would carry.
				if role != "builder" {
					t.Fatalf("the prompt of %s did not build: %v", role, err)
				}
				if !strings.Contains(err.Error(), "BUILD-4") {
					t.Fatalf("builder's prompt refused with %v, want BUILD-4's persisted problem", err)
				}
				if region == "" {
					t.Error("builder's preview draws nothing")
				}
				return
			}
			if built.Role.Path != doc.Path {
				t.Errorf("the prompt read the role document %s, the preview %s", built.Role.Path, doc.Path)
			}
			if !strings.Contains(built.Text, "\n\n"+region+"\n") {
				t.Errorf("the prompt does not carry the preview's lines of %s verbatim:\n--- preview ---\n%s\n--- prompt ---\n%s", role, region, built.Text)
			}
		})
	}

	// A rule kept out of the prompt (in_prompt: false) is one struck line with no
	// number: the numbered count and the character count fall, and the prompt
	// carries no LEAD-9. No per-member amount is part of the answer (d-b87a).
	t.Run("a rule kept out of the prompt", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		path := ruRolePath(f, catalog.PlaceProject, "lead")
		draft, _ := ruLoadDraft(t, path)

		_, before := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
			"place": catalog.PlaceProject, "original": "lead", "doc": draft,
		})
		beforePreview, _ := before["preview"].(map[string]any)
		beforeLines, beforeChars := ruNumbered(beforePreview)

		si, ri := ruRuleAt(t, draftRuleDoc(t, draft, "lead"), "LEAD-9")
		draft.Sections[si].Rules[ri].InPrompt = false
		_, after := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
			"place": catalog.PlaceProject, "original": "lead", "doc": draft,
		})
		afterPreview, _ := after["preview"].(map[string]any)
		afterLines, afterChars := ruNumbered(afterPreview)
		excluded := 0
		lines, _ := afterPreview["lines"].([]any)
		for _, l := range lines {
			one, _ := l.(map[string]any)
			if one["part"] == rules.PartExcluded {
				excluded++
				if !strings.HasPrefix(one["text"].(string), "LEAD-9 ") {
					t.Errorf("the excluded line reads %q, want LEAD-9's own", one["text"])
				}
			}
		}
		if excluded != 1 {
			t.Errorf("the preview draws %d excluded lines, want LEAD-9's one", excluded)
		}
		if afterLines != beforeLines-1 {
			t.Errorf("the preview reads %d 行 after the switch, want %d", afterLines, beforeLines-1)
		}
		if afterChars >= beforeChars {
			t.Errorf("the preview reads %d 文字 after the switch, want below %d", afterChars, beforeChars)
		}

		// The prompt carries the document without the rule: the same text the
		// preview draws as one struck line.
		draftText, err := rules.Emit(draft.document())
		if err != nil {
			t.Fatal(err)
		}
		overlay := f.catalog().With(path, draftText)
		built, err := ruMemberPrompt(t, overlay, "lead")
		if err != nil {
			t.Fatalf("the prompt of the switched lead did not build: %v", err)
		}
		if strings.Contains(built.Text, "LEAD-9") {
			t.Errorf("the prompt still carries the rule kept out of it")
		}
	})
}

// draftRuleDoc is one draft's document in the rules package's words, so a test
// can look a rule up by its id.
func draftRuleDoc(t *testing.T, draft *r2Doc, name string) *rules.Document {
	t.Helper()
	doc := draft.document()
	if doc == nil {
		t.Fatalf("%s's draft is empty", name)
	}
	return doc
}

// TestROLES_F1_ASaveWritesTheChangedLineAlone: one save that changes one value
// writes the line that value stands on and nothing else — a rule's body, a
// section's heading, a section's depth (which adds the one level: line) and a
// rule's in_prompt (which adds the one in_prompt: false line).
func TestROLES_F1_ASaveWritesTheChangedLineAlone(t *testing.T) {
	cases := []struct {
		name string
		role string
		edit func(t *testing.T, draft *r2Doc)
		want string // the one line the save writes, or "" when it is added
	}{
		{
			name: "a rule's body",
			role: "builder",
			edit: func(t *testing.T, draft *r2Doc) {
				si, ri := ruRuleAt(t, draftRuleDoc(t, draft, "builder"), "BUILD-5")
				draft.Sections[si].Rules[ri].Text = "書き換えた本文。"
			},
			want: "        text: 書き換えた本文。",
		},
		{
			name: "a section's heading",
			role: "builder",
			edit: func(_ *testing.T, draft *r2Doc) {
				draft.Sections[0].Heading = "作り方の見直し"
			},
			want: "  - heading: 作り方の見直し",
		},
		{
			name: "a section's depth",
			role: "builder",
			edit: func(_ *testing.T, draft *r2Doc) {
				draft.Sections[0].Level = 3
			},
			want: "    level: 3",
		},
		{
			name: "a rule kept out of the prompt",
			role: "lead",
			edit: func(t *testing.T, draft *r2Doc) {
				si, ri := ruRuleAt(t, draftRuleDoc(t, draft, "lead"), "LEAD-9")
				draft.Sections[si].Rules[ri].InPrompt = false
			},
			want: "        in_prompt: false",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := ruFixtureRoles(t)
			server := f.server(t)
			path := ruRolePath(f, catalog.PlaceProject, test.role)
			before := readFile(t, path)
			draft, text := ruLoadDraft(t, path)
			test.edit(t, draft)

			resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/"+test.role, map[string]any{
				"place": catalog.PlaceProject, "original": test.role, "hash": source.Hash(text), "doc": draft,
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("the save answered %d", resp.StatusCode)
			}
			after := readFile(t, path)
			changed := ruChangedLines(before, after)
			switch {
			case test.want == "        in_prompt: false":
				// Turning a rule off writes in_prompt and the day hergy switched
				// it off (d-9f30); everything else comes back byte for byte.
				if !strings.Contains(after, "        in_prompt: false\n") || !strings.Contains(after, "        off_since: ") {
					t.Errorf("the save did not add in_prompt and off_since:\n%s", after)
				}
				without := strings.Replace(after, "        in_prompt: false\n", "", 1)
				without = regexpOffSince.ReplaceAllString(without, "")
				if without != before {
					t.Errorf("the save changed more than the switch:\n%s", after)
				}
				if n := strings.Count(after, "\n") - strings.Count(before, "\n"); n != 2 {
					t.Errorf("the save added %d lines, want in_prompt and off_since", n)
				}
				_ = changed
			case strings.HasPrefix(test.want, "    level"):
				// An added line: everything else comes back byte for byte.
				if strings.Replace(after, test.want+"\n", "", 1) != before {
					t.Errorf("the save did not add the one %q line and nothing else:\n%s", test.want, after)
				}
				if n := strings.Count(after, "\n") - strings.Count(before, "\n"); n != 1 {
					t.Errorf("the save added %d lines, want the one %q", n, test.want)
				}
				_ = changed
			default:
				if len(changed) != 1 {
					t.Fatalf("the save changed %d lines, want the one it wrote: %v", len(changed), changed)
				}
				for _, got := range changed {
					if got != test.want {
						t.Errorf("the changed line reads %q, want %q", got, test.want)
					}
				}
			}
		})
	}
}

// TestROLES_F2_AMoveKeepsTheMovedItemsLines: one save that moves rules (BUILD-2
// down, BUILD-1 below BUILD-3) and a whole section (確かめ方 up) comes back with
// the same lines in another order: every rule's item is byte for byte the one
// that stood before, and the section that moved carries its own lines whole.
func TestROLES_F2_AMoveKeepsTheMovedItemsLines(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	before := readFile(t, path)
	draft, text := ruLoadDraft(t, path)

	// 作り方's rules take the order a person leaves them in: BUILD-3, BUILD-1,
	// BUILD-2, BUILD-4 (BUILD-2 one down, BUILD-1 below BUILD-3).
	sec := draft.Sections[0]
	byID := map[string]r2Rule{}
	for _, rule := range sec.Rules {
		byID[rule.ID] = rule
	}
	sec.Rules = nil
	for _, id := range []string{"BUILD-3", "BUILD-1", "BUILD-2", "BUILD-4"} {
		sec.Rules = append(sec.Rules, byID[id])
	}
	draft.Sections[0] = sec
	// 確かめ方 moves above 作り方.
	draft.Sections[0], draft.Sections[1] = draft.Sections[1], draft.Sections[0]

	resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(text), "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}
	after := readFile(t, path)

	ruSameLines(t, before, after)
	for _, id := range []string{"BUILD-1", "BUILD-2", "BUILD-3", "BUILD-4", "BUILD-5", "BUILD-6", "BUILD-7"} {
		if got, want := ruItemBlock(t, after, id), ruItemBlock(t, before, id); got != want {
			t.Errorf("%s's own lines changed:\n%s\nwant\n%s", id, got, want)
		}
	}
	if got, want := ruSectionBlock(t, after, "確かめ方"), ruSectionBlock(t, before, "確かめ方"); got != want {
		t.Errorf("the moved section's own lines changed:\n%s\nwant\n%s", got, want)
	}
	if strings.Index(after, "  - heading: 確かめ方") > strings.Index(after, "  - heading: 作り方") {
		t.Errorf("the moved section did not land first:\n%s", after)
	}
	at := 0
	for _, id := range []string{"BUILD-3", "BUILD-1", "BUILD-2", "BUILD-4"} {
		i := strings.Index(after, "- id: "+id)
		if i < 0 {
			t.Fatalf("%s left the file:\n%s", id, after)
		}
		if i < at {
			t.Errorf("the file puts %s above the rule before it:\n%s", id, after)
		}
		at = i
	}
}

// TestROLES_F4_RenamingWritesTheReadingTeamsRoleLine: a save that renames a role
// and rewrites the definitions that read it changes the file's name, leaves the
// new file byte for byte the old one, and changes only the role: lines of the
// team.yaml files whose members played the old name.
func TestROLES_F4_RenamingWritesTheReadingTeamsRoleLine(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	old := ruRolePath(f, catalog.PlaceProject, "scout")
	before := readFile(t, old)
	teams := map[string]string{}
	for _, name := range []string{"core", "docs", "migrate", "research"} {
		teams[name] = readFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml"))
	}
	sharedScout := ruRolePath(f, catalog.PlaceGlobal, "scout")
	sharedBefore := readFile(t, sharedScout)

	draft, text := ruLoadDraft(t, old)
	draft.Name = "scout-3"
	resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/scout", map[string]any{
		"place": catalog.PlaceProject, "original": "scout", "hash": source.Hash(text),
		"rename_users": true, "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the old file is still there: %v", err)
	}
	next := ruRolePath(f, catalog.PlaceProject, "scout-3")
	if got := readFile(t, next); got != before {
		t.Errorf("the renamed file is not the old one byte for byte:\n%s", got)
	}
	research := filepath.Join(f.proj, ".hergy", "teams", "research", "team.yaml")
	want := strings.ReplaceAll(teams["research"], "    role: scout\n", "    role: scout-3\n")
	if got := readFile(t, research); got != want {
		t.Errorf("research's team.yaml is not the fixture with only the role: lines changed:\n%s\nwant\n%s", got, want)
	}
	if n := strings.Count(teams["research"], "    role: scout\n"); n != 2 {
		t.Fatalf("research reads scav's role on %d lines, want 2", n)
	}
	for _, name := range []string{"core", "docs", "migrate"} {
		path := filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml")
		if got := readFile(t, path); got != teams[name] {
			t.Errorf("%s's team.yaml changed, and no member of it plays scout:\n%s", name, got)
		}
	}
	if got := readFile(t, sharedScout); got != sharedBefore {
		t.Errorf("renaming the project's scout changed the shared one:\n%s", got)
	}
}

// TestROLES_F5_AStaleHashIsRefused: a save whose page read a text another place
// has since changed is refused with error.rules_file_changed, and writes
// nothing.
func TestROLES_F5_AStaleHashIsRefused(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	draft, _ := ruLoadDraft(t, path)

	changed := read + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(read), "doc": draft,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the save answered %d, want 409", resp.StatusCode)
	}
	if want := i18n.Tf("error.rules_file_changed", path); decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if got := readFile(t, path); got != changed {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// TestROLES_F6_APersistedProblemBlocksOnlyTheNewOne: with builder's note on
// BUILD-4 standing, a save that fixes another rule is written; a save that
// overlaps a rule's id is refused. The screen refuses by
// the check it reads (ruledoc.js disables 保存 while a new error stands; the
// server's own write has no rule-id check), so the refusal is the check's new
// error — the fix save carries no new error and is written.
func TestROLES_F6_APersistedProblemBlocksOnlyTheNewOne(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	before := readFile(t, path)
	draft, text := ruLoadDraft(t, path)

	_, savedCheck := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "doc": draft,
	})
	known := ruErrorTexts(mustProblems(t, savedCheck))

	// The fix save: change BUILD-5's body, and the persisted problem stays.
	si, ri := ruRuleAt(t, draftRuleDoc(t, draft, "builder"), "BUILD-5")
	draft.Sections[si].Rules[ri].Text = "直した本文。"
	_, fixCheck := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "doc": draft,
	})
	fixErrors := ruErrorTexts(mustProblems(t, fixCheck))
	for text := range fixErrors {
		if known[text] {
			continue
		}
		t.Errorf("the fix save brought a new error the screen would refuse on: %q", text)
	}
	resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(text), "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the fix save answered %d, want it written", resp.StatusCode)
	}
	after := readFile(t, path)
	if !strings.Contains(after, "note: '由来: derived  REQ-5'") {
		t.Errorf("the fix save lost BUILD-4's note:\n%s", after)
	}
	if changed := ruChangedLines(before, after); len(changed) != 1 {
		t.Errorf("the fix save changed %d lines, want BUILD-5's text alone: %v", len(changed), changed)
	}

	// The overlapping save: BUILD-5 takes BUILD-4's id. The check reports a new
	// error on the rule's id field, which is what the screen refuses the save on.
	over, _ := ruLoadDraft(t, path)
	osi, ori := ruRuleAt(t, draftRuleDoc(t, over, "builder"), "BUILD-5")
	over.Sections[osi].Rules[ori].ID = "BUILD-4"
	_, dupCheck := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "doc": over,
	})
	found := false
	for _, one := range mustProblems(t, dupCheck) {
		problem, _ := one.(map[string]any)
		if problem["sev"] != rules.SevError || problem["field"] != "id" {
			continue
		}
		text, _ := problem["text"].(string)
		if strings.Contains(text, "BUILD-4") && !known[text] {
			found = true
		}
	}
	if !found {
		t.Errorf("the overlapping save brought no new error on the id field, so the screen would not refuse it: %v", ruIssues(mustProblems(t, dupCheck)))
	}
}

// mustProblems is one check answer's problems, or a test failure.
func mustProblems(t *testing.T, decoded map[string]any) []any {
	t.Helper()
	problems, ok := decoded["problems"].([]any)
	if !ok {
		t.Fatalf("the check answered no problems: %v", decoded)
	}
	return problems
}

// TestROLES_F7_TheFiveWrites: a role added, a duplicate and a copy to the shared
// place write new files (the duplicate and the copy byte for byte from the
// document they came from), a delete takes that one file away, and a role no
// layer holds a document for gets a new one.
func TestROLES_F7_TheFiveWrites(t *testing.T) {
	t.Run("a role added", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		draft := r2NewRole("ra-new")
		draft.Sections[0].Rules[0].ID = "NEW-1"
		draft.Sections[0].Rules[0].Text = "新しく足したロールの規則。"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
			"place": catalog.PlaceProject, "hash": "", "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		written := readFile(t, ruRolePath(f, catalog.PlaceProject, "ra-new"))
		for _, want := range []string{"id: NEW-1", "新しく足したロールの規則。"} {
			if !strings.Contains(written, want) {
				t.Errorf("the added file does not hold %q:\n%s", want, written)
			}
		}
	})

	t.Run("builder duplicated", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		draft.Name = "builder-2"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder-2", map[string]any{
			"place": catalog.PlaceProject, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceProject, "builder-2")); got != before {
			t.Errorf("the duplicate does not keep the original's lines:\n%s", got)
		}
		if got := readFile(t, src); got != before {
			t.Errorf("duplicating changed the original:\n%s", got)
		}
	})

	t.Run("copied to the shared place", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != before {
			t.Errorf("the copy does not keep the original's lines:\n%s", got)
		}
		if got := readFile(t, src); got != before {
			t.Errorf("copying changed the original:\n%s", got)
		}
	})

	t.Run("a role taken away", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		path := ruRolePath(f, catalog.PlaceProject, "builder")
		hash := source.Hash(readFile(t, path))
		resp, _ := f.api(t, server, http.MethodDelete, "/api/role/builder?place="+catalog.PlaceProject+"&hash="+url.QueryEscape(hash), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the delete answered %d", resp.StatusCode)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the file is still there: %v", err)
		}
		if _, err := os.Stat(ruRolePath(f, catalog.PlaceProject, "lead")); err != nil {
			t.Errorf("the delete took another file away: %v", err)
		}
		if _, err := os.Stat(ruRolePath(f, catalog.PlaceGlobal, "reviewer")); err != nil {
			t.Errorf("the delete took the shared reviewer away: %v", err)
		}
	})

	t.Run("a role no layer holds", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		dir := filepath.Join(f.proj, ".hergy", "teams", "ghost")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		team := "version: 1\nmission: \"文書の無いロールを読む\"\n" +
			"members:\n  - name: lead\n    role: lead\n    profile: opus-deep\n" +
			"  - name: ghost-writer\n    role: ghost\n    profile: gemini-pro\n" +
			"turns:\n  - human -> lead\n  - lead -> ghost-writer\n  - ghost-writer -> lead\n"
		if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte(team), 0o644); err != nil {
			t.Fatal(err)
		}

		_, list := f.apiJSON(t, server, http.MethodGet, "/api/role", nil)
		found := false
		for _, item := range list["items"].([]any) {
			one, _ := item.(map[string]any)
			if one["name"] != "ghost" {
				continue
			}
			summary, _ := one["summary"].(map[string]any)
			found = summary["noDoc"] == true
		}
		if !found {
			t.Fatalf("the list has no 文書の無い row for ghost: %v", list["items"])
		}

		draft := r2NewRole("ghost")
		draft.Sections[0].Rules[0].ID = "GHOST-1"
		draft.Sections[0].Rules[0].Text = "文書の無いロールに足した規則。"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/ghost", map[string]any{
			"place": catalog.PlaceProject, "hash": "", "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		written := readFile(t, ruRolePath(f, catalog.PlaceProject, "ghost"))
		for _, want := range []string{"id: GHOST-1", "文書の無いロールに足した規則。"} {
			if !strings.Contains(written, want) {
				t.Errorf("the new file does not hold %q:\n%s", want, written)
			}
		}
	})
}

// TestROLES_F8_ALayerWithoutRolesGetsOne: a layer that is there but holds no
// roles/ yet gets one on the first save, whether the save adds a role, copies one
// to the shared place or writes the YAML tab's text; a layer that is not there is
// still refused with the sentence that names it (n-86c7, d-4cba).
func TestROLES_F8_ALayerWithoutRolesGetsOne(t *testing.T) {
	// noRoles answers a fixture whose one layer has lost its roles/.
	noRoles := func(t *testing.T, place string) *fixture {
		t.Helper()
		f := ruFixtureRoles(t)
		if err := os.RemoveAll(filepath.Dir(ruRolePath(f, place, "x"))); err != nil {
			t.Fatal(err)
		}
		return f
	}

	t.Run("a role added", func(t *testing.T) {
		for _, place := range []string{catalog.PlaceProject, catalog.PlaceGlobal} {
			f := noRoles(t, place)
			server := f.server(t)
			draft := r2NewRole("ra-new")
			draft.Sections[0].Rules[0].Text = "roles/ の無い層に足したロールの規則。"
			resp, body := f.api(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
				"place": place, "hash": "", "doc": draft,
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: the save answered %d: %s", place, resp.StatusCode, body)
			}
			if got := readFile(t, ruRolePath(f, place, "ra-new")); !strings.Contains(got, "roles/ の無い層に足したロールの規則。") {
				t.Errorf("%s: the added file does not hold the rule:\n%s", place, got)
			}
		}
	})

	t.Run("copied to the shared place", func(t *testing.T) {
		f := noRoles(t, catalog.PlaceGlobal)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		resp, body := f.api(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != before {
			t.Errorf("the copy does not keep the original's lines:\n%s", got)
		}
	})

	t.Run("the YAML tab's text", func(t *testing.T) {
		f := noRoles(t, catalog.PlaceGlobal)
		server := f.server(t)
		text := readFile(t, ruRolePath(f, catalog.PlaceProject, "builder"))
		resp, body := f.api(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "name": "builder", "hash": source.Hash(""), "text": text,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != text {
			t.Errorf("the save wrote %q, want the typed text", got)
		}
	})

	t.Run("a layer that is not there", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		if err := os.RemoveAll(f.global); err != nil {
			t.Fatal(err)
		}
		resp, body := f.api(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "", "doc": r2NewRole("ra-new"),
		})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("the save answered %d, want 422: %s", resp.StatusCode, body)
		}
		if !strings.Contains(body, "roles/") {
			t.Errorf("the refusal does not name roles/: %s", body)
		}
		if _, err := os.Stat(f.global); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the refused save made the layer: %v", err)
		}
	})
}
