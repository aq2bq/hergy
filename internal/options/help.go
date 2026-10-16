package options

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// spelling is how a command line writes a flag: the one-letter form when it
// has one (-C), else the long form.
func spelling(f *Flag) string {
	if f.Short != "" {
		return "-" + f.Short
	}
	return "--" + f.Name
}

// withValue writes a flag with its value name, in a language.
func withValue(flag string, f *Flag, lang string) string {
	if f.Value == nil {
		return flag
	}
	return flag + " " + f.Value[lang]
}

// flagLabel is how a table of flags writes one: -C, --dir <dir>.
func flagLabel(f *Flag, lang string) string {
	label := "--" + f.Name
	if f.Short != "" {
		label = "-" + f.Short + ", " + label
	}
	return withValue(label, f, lang)
}

// argWords writes the positional arguments of a command: [definition] when it
// may be left out, <member>|--all when a flag can stand in for it.
func argWords(s *Spec, lang string) []string {
	var words []string
	for _, a := range s.Args {
		word := "<" + a.Name[lang] + ">"
		if a.Optional {
			word = "[" + a.Name[lang] + "]"
		}
		if a.Or != nil {
			word += "|--" + a.Or.Name
		}
		words = append(words, word)
	}
	return words
}

// standsIn reports whether the flag takes the place of a positional argument,
// so a command line already shows it as <member>|--all.
func standsIn(s *Spec, f *Flag) bool {
	for _, a := range s.Args {
		if a.Or == f {
			return true
		}
	}
	return false
}

// Synopsis is the command line of one command, in a language, with every flag
// it accepts: the required ones bare, the others in brackets. The cheatsheet
// for an agent prints it, so the line is read from the table and cannot fall
// behind it. --lang and --help, which every command has, are left out.
func Synopsis(kind Kind, lang string) string {
	s := specOf(kind)
	if s == nil {
		return ""
	}
	words := append([]string{"hergy " + s.Name}, argWords(s, lang)...)
	for i := 0; i < len(s.Flags); i++ {
		u := s.Flags[i]
		if standsIn(s, u.Flag) {
			continue
		}
		if u.Required {
			words = append(words, withValue(spelling(u.Flag), u.Flag, lang))
			continue
		}
		word := "[" + withValue(spelling(u.Flag), u.Flag, lang) + "]"
		words = append(words, word)
	}
	return strings.Join(words, " ")
}

// usageLine is the "how to type it" line of one command's --help: its words and
// the flags it cannot run without, then a bracket for all the others, which the
// table below spells out.
func usageLine(s *Spec, lang string) string {
	words := append([]string{"hergy " + s.Name}, argWords(s, lang)...)
	for _, u := range s.Flags {
		if u.Required {
			words = append(words, withValue(spelling(u.Flag), u.Flag, lang))
		}
	}
	return strings.Join(append(words, i18n.T("help.options_placeholder")), " ")
}

// row is one line of a table in --help: a first cell and what it means.
type row struct{ label, desc string }

// writeRows prints rows with their second column at col. A description that
// runs over more than one line carries on under the second column.
func writeRows(b *strings.Builder, rows []row, col int) {
	for _, r := range rows {
		lines := strings.Split(r.desc, "\n")
		fmt.Fprintf(b, "  %s%s  %s\n", r.label, strings.Repeat(" ", col-i18n.Width(r.label)), lines[0])
		for _, more := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", strings.Repeat(" ", 2+col+2), more)
		}
	}
}

// CommandHelp is the --help text of one command, in the language in use: what it does,
// how to type it, its arguments, and every flag it accepts with what the flag
// means for that command. It is read from the table, so a flag the parser takes
// is a flag this lists, and one it refuses is not there.
func CommandHelp(kind Kind) string {
	s := specOf(kind)
	if s == nil || s.Bare {
		return Usage()
	}
	lang := i18n.Language()

	var argRows, flagRows []row
	for _, a := range s.Args {
		label := "<" + a.Name[lang] + ">"
		if a.Optional {
			label = "[" + a.Name[lang] + "]"
		}
		argRows = append(argRows, row{label, a.Desc[lang]})
	}
	for _, list := range [][]Use{s.Flags, common} {
		for _, u := range list {
			flagRows = append(flagRows, row{flagLabel(u.Flag, lang), u.Desc[lang]})
		}
	}
	col := 0
	for _, list := range [][]row{argRows, flagRows} {
		for _, r := range list {
			col = max(col, i18n.Width(r.label))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "hergy %s - %s\n\n", s.Name, s.Summary[lang])
	fmt.Fprintf(&b, "%s %s\n", i18n.T("help.usage_label"), usageLine(s, lang))
	if detail := s.Detail[lang]; detail != "" {
		fmt.Fprintf(&b, "\n%s\n", detail)
	}
	if len(argRows) > 0 {
		fmt.Fprintf(&b, "\n%s\n", i18n.T("help.arguments"))
		writeRows(&b, argRows, col)
	}
	fmt.Fprintf(&b, "\n%s\n", i18n.T("help.options"))
	writeRows(&b, flagRows, col)
	return b.String()
}

// FlagSummary is one flag as the cheatsheet for an agent lists it.
type FlagSummary struct {
	Name     string   // "dir"
	Label    string   // "-C, --dir <dir>"
	Commands []string // the commands that take it, in the cheatsheet's order; none for a flag every command takes
}

// FlagSummaries lists every flag, in the order the cheatsheet shows them, with
// the commands that take each. --lang and --help, which every command takes,
// carry no commands.
func FlagSummaries() []FlagSummary {
	out := make([]FlagSummary, 0, len(allFlags))
	for _, f := range allFlags {
		sum := FlagSummary{Name: f.Name, Label: flagLabel(f, "en")}
		if f != flagLang && f != flagHelp {
			for _, s := range specs {
				if has(s, f) {
					sum.Commands = append(sum.Commands, s.Name)
				}
			}
			if len(sum.Commands) == 0 {
				// A flag the parser still knows but no command takes any more
				// (--global and --local, gone in 段 3): it is refused with its
				// own sentence and is not a flag of the shape.
				continue
			}
		}
		out = append(out, sum)
	}
	return out
}

func has(s Spec, f *Flag) bool {
	for _, u := range s.Flags {
		if u.Flag == f {
			return true
		}
	}
	return false
}
