package options

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// exclusive are flags that cannot be given together. A command line writes such
// a pair as one group: [--global|--local].
var exclusive = [][2]*Flag{{flagGlobal, flagLocal}}

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
		for _, pair := range exclusive {
			if pair[0] == u.Flag && i+1 < len(s.Flags) && s.Flags[i+1].Flag == pair[1] {
				word = "[--" + pair[0].Name + "|--" + pair[1].Name + "]"
				i++
				break
			}
		}
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
			label := "--" + u.Flag.Name
			if u.Flag.Short != "" {
				label = "-" + u.Flag.Short + ", " + label
			}
			flagRows = append(flagRows, row{withValue(label, u.Flag, lang), u.Desc[lang]})
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
