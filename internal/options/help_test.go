package options

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// useLanguage loads the locales and switches to one of them for a test.
func useLanguage(t *testing.T, lang string) {
	t.Helper()
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
	if err := i18n.Use(lang); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
}

// helpOf is what hergy <command> <flag> prints, read through Parse the way the
// program reads it.
func helpOf(t *testing.T, command, flag string) string {
	t.Helper()
	got, err := Parse([]string{command, flag})
	if err != nil {
		t.Fatalf("hergy %s %s: %v", command, flag, err)
	}
	if got.Kind != Help {
		t.Fatalf("hergy %s %s: Kind = %d, want Help", command, flag, got.Kind)
	}
	return got.HelpText
}

// flagRow finds a line of the flag table: two spaces, then --name, or -C, --name.
var flagRow = regexp.MustCompile(`^  (?:-[A-Za-z], )?--([a-z][a-z0-9-]*)(?: [^ ].*?)?(?:  +(\S.*))?$`)

// TestCommandHelpListsExactlyTheFlags: hergy <command> --help and -h print the
// same text, and the flags it lists, each with a description, are the ones the
// command accepts: its own, and --lang and --help. A flag the command refuses is
// not listed.
func TestCommandHelpListsExactlyTheFlags(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		for i := range specs {
			s := &specs[i]
			if s.Bare {
				continue
			}
			long, short := helpOf(t, s.Name, "--help"), helpOf(t, s.Name, "-h")
			if long != short {
				t.Errorf("[%s] hergy %s: -h and --help print different text:\n%s\n---\n%s", lang, s.Name, short, long)
			}

			var listed []string
			for line := range strings.SplitSeq(long, "\n") {
				m := flagRow.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				if m[2] == "" {
					t.Errorf("[%s] hergy %s --help: --%s has no description: %q", lang, s.Name, m[1], line)
				}
				listed = append(listed, m[1])
			}
			sort.Strings(listed)
			want := Flags(s.Kind)
			if strings.Join(listed, " ") != strings.Join(want, " ") {
				t.Errorf("[%s] hergy %s --help lists --%s, but the command accepts --%s:\n%s",
					lang, s.Name, strings.Join(listed, " --"), strings.Join(want, " --"), long)
			}

			if first, _, _ := strings.Cut(long, "\n"); first != "hergy "+s.Name+" - "+s.Summary[lang] {
				t.Errorf("[%s] hergy %s --help opens with %q, want its name and summary", lang, s.Name, first)
			}
			if !strings.Contains(long, i18n.T("help.usage_label")+" "+usageLine(s, lang)+"\n") {
				t.Errorf("[%s] hergy %s --help has no usage line %q", lang, s.Name, usageLine(s, lang))
			}
			for _, a := range s.Args {
				if a.Word {
					continue // typed as it stands: the usage line holds it
				}
				if !strings.Contains(long, "  <"+a.Name[lang]+">") && !strings.Contains(long, "  ["+a.Name[lang]+"]") {
					t.Errorf("[%s] hergy %s --help does not list the argument %q", lang, s.Name, a.Name[lang])
				}
			}
		}
	}
}

// maxHelpWidth is the widest a --help line may be, in terminal columns.
const maxHelpWidth = 100

// secondColumn is where the second cell of a table row starts, in terminal
// columns, or -1 for a line that is not a row: indented two spaces, then a cell,
// then a gap of two or more spaces. A line indented further continues a cell.
func secondColumn(line string) int {
	if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
		return -1
	}
	body := line[2:]
	gap := strings.Index(body, "  ")
	if gap < 0 {
		return -1
	}
	rest := strings.TrimLeft(body[gap:], " ")
	if rest == "" {
		return -1
	}
	return i18n.Width(line[:len(line)-len(rest)])
}

// TestCommandHelpColumnsLineUp: in every command's --help, the second column
// starts at one place, whether it is the arguments or the flags, a full-width
// character counting as two columns, and no line is wider than maxHelpWidth.
func TestCommandHelpColumnsLineUp(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		for i := range specs {
			s := &specs[i]
			if s.Bare {
				continue
			}
			want := -1
			for line := range strings.SplitSeq(helpOf(t, s.Name, "--help"), "\n") {
				if w := i18n.Width(line); w > maxHelpWidth {
					t.Errorf("[%s] hergy %s --help: a line is %d columns wide, over %d:\n%s", lang, s.Name, w, maxHelpWidth, line)
				}
				col := secondColumn(line)
				switch {
				case col < 0:
				case want < 0:
					want = col
				case col != want:
					t.Errorf("[%s] hergy %s --help: the second column starts at %d, want %d:\n%s", lang, s.Name, col, want, line)
				}
			}
		}
	}
}

// TestHelpWithoutACommandIsTheFullList: hergy, hergy --help, hergy -h and
// hergy help print the list of every command, not one command's page.
func TestHelpWithoutACommandIsTheFullList(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		for _, argv := range [][]string{nil, {"--help"}, {"-h"}, {"help"}} {
			got, err := Parse(argv)
			if err != nil {
				t.Fatalf("Parse(%v): %v", argv, err)
			}
			if got.Kind != Help || got.HelpText != Usage() {
				t.Errorf("[%s] Parse(%v) is not the full list:\n%s", lang, argv, got.HelpText)
			}
		}
	}
}

// TestEveryCommandHasItsOwnHelp: no two commands print the same help, and none
// prints the full list.
func TestEveryCommandHasItsOwnHelp(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		seen := map[string]string{}
		for i := range specs {
			s := &specs[i]
			if s.Bare {
				continue
			}
			text := helpOf(t, s.Name, "--help")
			if text == Usage() {
				t.Errorf("[%s] hergy %s --help prints the full list", lang, s.Name)
			}
			if other, ok := seen[text]; ok {
				t.Errorf("[%s] hergy %s --help and hergy %s --help print the same text", lang, s.Name, other)
			}
			seen[text] = s.Name
		}
	}
}

// TestHelpStopsTheParseWhereItStands: --help is answered when it is read, as
// before: an error ahead of it still wins, and one behind it is never read.
func TestHelpStopsTheParseWhereItStands(t *testing.T) {
	useLanguage(t, "en")
	if got, err := Parse([]string{"new", "--help", "--bogus"}); err != nil || got.Kind != Help {
		t.Errorf("hergy new --help --bogus = %+v, %v; want the help", got, err)
	}
	if _, err := Parse([]string{"new", "--bogus", "--help"}); err == nil {
		t.Error("hergy new --bogus --help printed help; the unknown option comes first and is an error")
	}
	if got, err := Parse([]string{"version", "--help"}); err != nil || got.Kind != Version {
		t.Errorf("hergy version --help = %+v, %v; want the version, as before", got, err)
	}
}

// TestSynopsis: the command line the cheatsheet for an agent prints for a
// command, written out for the shapes: optional argument, required flags, and a
// flag standing in for an argument.
func TestSynopsis(t *testing.T) {
	tests := map[Kind]string{
		New:     "hergy new <definition> [-C <dir>]",
		Up:      "hergy up [definition] [--as <team>] [--dry] [--json] [--timeout <ms>] [-C <dir>]",
		Refresh: "hergy refresh <member>|--all [--team <team>] [--dry] [--json] [--timeout <ms>] [-C <dir>]",
		Hire: "hergy hire <name> --purpose <purpose> --task <task> [--from <member>] [--team <team>] " +
			"[--timeout <ms>] [--dry] [--json] [-C <dir>]",
		Version: "hergy version",
	}
	for kind, want := range tests {
		if got := Synopsis(kind, "en"); got != want {
			t.Errorf("Synopsis(%s) =\n  %s\nwant\n  %s", commandName(kind), got, want)
		}
	}
}

// TestOptionErrorsPointAtTheCommandsHelp: an unknown option, and a flag that
// belongs to another command, are refused with a pointer to the page of the
// command typed, hergy <command> --help, where that command's flags are listed.
// An unknown command has no page to point at, so it still points at hergy help.
func TestOptionErrorsPointAtTheCommandsHelp(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		for i := range specs {
			s := &specs[i]
			if s.Bare {
				continue
			}
			want := "hergy " + s.Name + " --help"

			_, err := Parse(append(argvOf(s, len(s.Args), nil), "--bogus"))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("[%s] hergy %s --bogus: the error is %v, want it to point at %q", lang, s.Name, err, want)
			}

			for _, f := range allFlags {
				if accepts(s, f) {
					continue
				}
				if f == flagGlobal || f == flagLocal || f == flagProfile {
					// --global, --local and --profile are gone (段 3, 段 6b):
					// they are refused with the sentence that says so, not with
					// "not allowed here". Those sentences are checked below.
					continue
				}
				_, err := Parse(append(argvOf(s, len(s.Args), nil), flagArgs(f)...))
				if err == nil || !strings.Contains(err.Error(), "--"+f.Name) || !strings.Contains(err.Error(), want) {
					t.Errorf("[%s] hergy %s --%s: the error is %v, want it to name the flag and point at %q", lang, s.Name, f.Name, err, want)
				}
			}
		}

		// --global and --local are refused everywhere with the one sentence
		// (段 3 の設計), whatever command they are typed on; --profile with the
		// sentence that says where the profile comes from now (段 6b). The
		// sentence names the flag twice, so both places must be filled: a
		// missing one shows as %!s(MISSING) and the reader cannot tell what to
		// take out (n-60fd).
		for _, f := range []*Flag{flagGlobal, flagLocal} {
			_, err := Parse(append(argvOf(&specs[0], len(specs[0].Args), nil), "--"+f.Name))
			want := i18n.Tf("error.layer_flag_gone", f.Name, f.Name)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("[%s] hergy %s --%s: the error is %v, want the sentence that says the flag is gone", lang, specs[0].Name, f.Name, err)
				continue
			}
			if strings.Contains(err.Error(), "%!") {
				t.Errorf("[%s] hergy %s --%s: the error is %v, want no unfilled placeholder", lang, specs[0].Name, f.Name, err)
			}
		}
		if _, err := Parse([]string{"hire", "w", "--profile", "dev"}); err == nil || !strings.Contains(err.Error(), i18n.T("error.profile_flag_gone")) {
			t.Errorf("[%s] hergy hire --profile: the error is %v, want the sentence that says the flag is gone", lang, err)
		}

		_, err := Parse([]string{"frobnicate"})
		wantList := map[string]string{"en": "`hergy help`", "ja": "hergy --help"}[lang]
		if err == nil || !strings.Contains(err.Error(), wantList) {
			t.Errorf("[%s] hergy frobnicate: the error is %v, want it to point at %s", lang, err, wantList)
		}
	}
}

// TestWrapDetail fixes where a description breaks: at the last space that fits,
// inside a word only when the word alone is too long, and never on an empty
// paragraph.
func TestWrapDetail(t *testing.T) {
	for _, c := range []struct {
		name, text string
		width      int
		want       string
	}{
		{"fits exactly", "aaaaa bbbb", 10, "aaaaa bbbb"},
		{"breaks at the last space that fits", "aaa bbb ccc ddd", 10, "aaa bbb\nccc ddd"},
		{"a word longer than the line is cut", "aaaaaaaaaaaa", 5, "aaaaa\naaaaa\naa"},
		{"a long word after a short one", "ab cccccccc", 5, "ab\nccccc\nccc"},
		{"an empty paragraph adds no line", "aa\n\nbb", 10, "aa\nbb"},
		{"wide characters count two columns", "ああああ", 6, "あああ\nあ"},
		{"a character wider than the line still moves on", "ああ", 1, "あ\nあ"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := wrapDetail(c.text, c.width); got != c.want {
				t.Errorf("wrapDetail(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
			}
		})
	}
}

// TestWriteRowsWrapsUnderTheSecondColumn fixes the table's shape: a description
// is cut so the line ends by column 100, and what is cut off carries on under
// the second column.
func TestWriteRowsWrapsUnderTheSecondColumn(t *testing.T) {
	const col = 10
	fits := strings.Repeat("a", 43) + " " + strings.Repeat("b", 42) // 86 = 100 - col - 4
	over := fits + "b"
	var b strings.Builder
	writeRows(&b, []row{{"--one", fits}, {"--two", over}}, col)
	want := "  --one       " + fits + "\n" +
		"  --two       " + strings.Repeat("a", 43) + "\n" +
		strings.Repeat(" ", 14) + strings.Repeat("b", 43) + "\n"
	if got := b.String(); got != want {
		t.Errorf("rows:\n%q\nwant:\n%q", got, want)
	}
}

// TestCommandHelpNamesArgumentsOnlyWhenThereAreSome keeps the arguments heading
// off the page of a command that takes none.
func TestCommandHelpNamesArgumentsOnlyWhenThereAreSome(t *testing.T) {
	useLanguage(t, "en")
	heading := "\n" + i18n.T("help.arguments") + "\n"
	with, without := 0, 0
	for _, s := range specs {
		if s.Bare {
			continue
		}
		got := strings.Contains(CommandHelp(s.Kind), heading)
		if want := len(s.Args) > 0; got != want {
			t.Errorf("hergy %s --help: arguments heading %t, want %t", s.Name, got, want)
		}
		if len(s.Args) > 0 {
			with++
		} else {
			without++
		}
	}
	if with == 0 || without == 0 {
		t.Fatalf("the table has %d commands with arguments and %d without: both are needed here", with, without)
	}
}

// TestHasFindsOnlyTheCommandsOwnFlags reads one command's flags against another's.
func TestHasFindsOnlyTheCommandsOwnFlags(t *testing.T) {
	var owner, other *Spec
	for i := range specs {
		if len(specs[i].Flags) > 0 && owner == nil {
			owner = &specs[i]
		}
	}
	if owner == nil {
		t.Fatal("no command with a flag of its own")
	}
	own := owner.Flags[0].Flag
	for i := range specs {
		lacks := true
		for _, u := range specs[i].Flags {
			lacks = lacks && u.Flag != own
		}
		if lacks {
			other = &specs[i]
			break
		}
	}
	if other == nil {
		t.Fatal("every command takes the same flag")
	}
	if !has(*owner, own) {
		t.Errorf("hergy %s: has() misses its own flag", owner.Name)
	}
	if has(*other, own) {
		t.Errorf("hergy %s: has() finds a flag it does not take", other.Name)
	}
}

// TestAWordArgumentIsWrittenBare: hergy projects is typed with the word add, so
// the usage line writes the word without brackets and --help gives it no row.
func TestAWordArgumentIsWrittenBare(t *testing.T) {
	useLanguage(t, "en")
	if got, want := Synopsis(Projects, "en"), "hergy projects add <directory>"; got != want {
		t.Errorf("the synopsis reads %q, want %q", got, want)
	}
	help := CommandHelp(Projects)
	if !strings.Contains(help, "hergy projects add <directory> ") || strings.Contains(help, "\n  add ") {
		t.Errorf("hergy projects --help:\n%s", help)
	}
	if got, err := Parse([]string{"projects", "add", "/x"}); err != nil || got.Path != "/x" {
		t.Errorf("hergy projects add /x: %+v, %v", got, err)
	}
	for _, argv := range [][]string{{"projects"}, {"projects", "add"}, {"projects", "remove", "/x"}, {"projects", "add", "/x", "/y"}} {
		if _, err := Parse(argv); err == nil {
			t.Errorf("hergy %v is accepted", argv)
		}
	}
}
