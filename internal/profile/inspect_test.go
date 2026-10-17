package profile

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestReadAllCollectsEveryProblemAndItsLine: one pass reads the entries in the
// file's order and gathers every problem with the line it stands on, rather
// than stopping at the first (r-c683).
func TestReadAllCollectsEveryProblemAndItsLine(t *testing.T) {
	text := "version: 1\n" +
		"# 手で書いたファイル\n" +
		"profiles:\n" +
		"  codex:\n" +
		"    kind: codex\n" +
		"  Bad Name:\n" +
		"    kind: \"\"\n" +
		"    args: [1, --ok]\n" +
		"    env:\n" +
		"      A: one\n" +
		"      A: two\n" +
		"      B: 3\n"
	entries, problems := ReadAll(text, "profiles.yaml")
	if len(entries) != 2 {
		t.Fatalf("the file reads as %d entries, want 2", len(entries))
	}
	if entries[0].Name != "codex" || entries[0].Line != 4 {
		t.Errorf("the first entry is %q on line %d", entries[0].Name, entries[0].Line)
	}
	if entries[1].Args[0] != "1" || entries[1].Env[0].Key != "A" {
		t.Errorf("the second entry reads as %+v", entries[1])
	}
	want := []struct {
		text string
		line int
	}{
		{i18n.Tf("error.profile_name_pattern", "profiles.Bad Name"), 6},
		{i18n.Tf("error.not_plain_kind", "profiles.Bad Name.kind"), 7},
		{i18n.Tf("error.arg_not_string", "profiles.Bad Name.args", 0), 8},
		{i18n.Tf("error.env_key_twice", "profiles.Bad Name", "A"), 11},
		{i18n.Tf("error.env_not_string", "profiles.Bad Name.env", "B"), 12},
		{i18n.Tf("validate.profile_named_after_kind", "codex"), 4},
	}
	for _, one := range want {
		found := false
		for _, problem := range problems {
			if problem.Text == one.text && problem.Line == one.line {
				found = true
			}
		}
		if !found {
			t.Errorf("no %q on line %d: %+v", one.text, one.line, problems)
		}
	}
	var warnings int
	for _, problem := range problems {
		if problem.Sev == Warn {
			warnings++
		}
	}
	if warnings != 1 {
		t.Errorf("the file has %d warnings, want the one name that is its kind", warnings)
	}
}

// TestReadAllNamesTheFileProblems: a file that does not read at all says why,
// on the line the reading stopped.
func TestReadAllNamesTheFileProblems(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"", i18n.Tf("error.file_empty", "profiles.yaml", "profiles.yaml")},
		{"profiles:\n  a:\n    kind: claude\n", i18n.Tf("error.version_missing", "profiles.yaml", "profiles.yaml")},
		{"version: 2\nprofiles:\n", i18n.Tf("error.version_wrong", "profiles.yaml", 2, "profiles.yaml")},
		{"version: 1\nprofiles:\n  a:\n    kind: claude\n    colour: red\n", "colour not found"},
	} {
		entries, problems := ReadAll(test.text, "profiles.yaml")
		if len(entries) != 0 && !strings.Contains(test.want, "colour") {
			t.Errorf("%q read as %d entries", test.text, len(entries))
		}
		if len(problems) == 0 || !strings.Contains(problems[0].Text, test.want) {
			t.Errorf("%q reads as %+v, want %q", test.text, problems, test.want)
		}
	}
}

// TestCheckDraftSaysTheScreensSentences: a draft is checked with the same
// sentences the file's own reading uses, including the new one for a variable
// written twice.
func TestCheckDraftSaysTheScreensSentences(t *testing.T) {
	saved := []Entry{{Name: "coder", Kind: "claude"}}
	problems := CheckDraft(Draft{
		Name: "planner", Kind: "codex",
		Env: []EnvPair{{Key: "A", Value: "1"}, {Key: "A", Value: "2"}},
	}, saved, "")
	want := map[string]bool{i18n.Tf("error.env_key_twice", "profiles.planner", "A"): false}
	for _, problem := range problems {
		if problem.Field == "env.1.key" && problem.Text == i18n.Tf("error.env_key_twice", "profiles.planner", "A") {
			want[i18n.Tf("error.env_key_twice", "profiles.planner", "A")] = true
		}
	}
	if !want[i18n.Tf("error.env_key_twice", "profiles.planner", "A")] {
		t.Errorf("the twice-written variable is not named: %+v", problems)
	}
	if got := CheckDraft(Draft{Name: "coder", Kind: "claude"}, saved, "coder"); len(got) != 0 {
		t.Errorf("a draft that only keeps its name is refused: %+v", got)
	}
}

// TestRenameKeepsEveryOtherLine: a rename writes the key's own line, and the
// comment above it, the profile under it and the next profile stay as they
// were.
func TestRenameKeepsEveryOtherLine(t *testing.T) {
	text := "version: 1\nprofiles:\n  # 計画\n  planner:\n    kind: claude\n    args: [--x]  # 重い\n  coder:\n    kind: codex\n"
	out, err := EditYAML([]byte(text), "profiles.yaml", Rename("planner", "plan"))
	if err != nil {
		t.Fatalf("the rename was refused: %v", err)
	}
	want := strings.Replace(text, "  planner:", "  plan:", 1)
	if string(out) != want {
		t.Errorf("the rename wrote:\n%s\nwant:\n%s", out, want)
	}
	if _, err := EditYAML([]byte(text), "profiles.yaml", Rename("coder", "planner")); err == nil {
		t.Errorf("a rename onto a name that is there was allowed")
	}
	if _, err := EditYAML([]byte(text), "profiles.yaml", Rename("nosuch", "x")); err == nil {
		t.Errorf("a rename of a profile that is not there was allowed")
	}
}
