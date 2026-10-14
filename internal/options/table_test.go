package options

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// wantText fails when a Text is missing a language hergy ships, or has one it
// does not, or a language says nothing.
func wantText(t *testing.T, where string, tx Text) {
	t.Helper()
	for _, lang := range i18n.Available {
		got, ok := tx[lang]
		switch {
		case !ok:
			t.Errorf("%s has no %s text", where, lang)
		case strings.TrimSpace(got) == "":
			t.Errorf("%s: the %s text is empty", where, lang)
		case got != strings.TrimSpace(got):
			t.Errorf("%s: the %s text has spaces at an end: %q", where, lang, got)
		}
	}
	for lang := range tx {
		known := false
		for _, l := range i18n.Available {
			known = known || l == lang
		}
		if !known {
			t.Errorf("%s has text for %q, which is not a language hergy ships (%v)", where, lang, i18n.Available)
		}
	}
}

// TestEveryKindHasARow: a command kind that is not in the table is a command
// with no flags, no summary and no --help.
func TestEveryKindHasARow(t *testing.T) {
	for kind := Kind(0); kind < kindEnd; kind++ {
		if kind == Help {
			continue // not a command: it is what --help and no command turn into
		}
		if specOf(kind) == nil {
			t.Errorf("command kind %d has no row in the table (spec.go)", kind)
		}
	}
	if len(Kinds) != len(specs) {
		t.Errorf("Kinds lists %d commands, the table has %d", len(Kinds), len(specs))
	}
	names := map[string]bool{}
	for _, s := range specs {
		if s.Name == "" {
			t.Errorf("a row of the table has no command word (kind %d)", s.Kind)
		}
		if names[s.Name] {
			t.Errorf("two rows of the table are the command %q", s.Name)
		}
		names[s.Name] = true
		if specNamed(s.Name).Kind != s.Kind {
			t.Errorf("the command %q is found as another kind", s.Name)
		}
	}
}

// TestTableHasEveryDescription: every command and every flag a command accepts
// is described, in every language. Add a flag to a command, or a command to the
// table, and write the words, or this fails and says which.
func TestTableHasEveryDescription(t *testing.T) {
	inAll := map[*Flag]bool{}
	for _, f := range allFlags {
		inAll[f] = true
		if f.Value != nil {
			wantText(t, "the value of --"+f.Name, f.Value)
		}
		if f.Set == nil {
			t.Errorf("--%s sets nothing", f.Name)
		}
	}
	for _, s := range specs {
		wantText(t, "hergy "+s.Name+": the summary", s.Summary)
		if s.Detail != nil {
			wantText(t, "hergy "+s.Name+": the detail", s.Detail)
		}
		if s.Bare && (len(s.Flags) > 0 || len(s.Args) > 0) {
			t.Errorf("hergy %s answers before any flag is read, so it cannot list flags or arguments", s.Name)
		}
		for _, a := range s.Args {
			where := "hergy " + s.Name + ": the argument <" + a.Name["en"] + ">"
			wantText(t, where+" name", a.Name)
			wantText(t, where+" description", a.Desc)
			if a.Or != nil && !has(s, a.Or) {
				t.Errorf("%s stands in for --%s, which the command does not accept", where, a.Or.Name)
			}
		}
		seen := map[*Flag]bool{}
		for _, u := range s.Flags {
			if u.Flag == nil {
				t.Errorf("hergy %s has a flag entry with no flag", s.Name)
				continue
			}
			where := "hergy " + s.Name + " --" + u.Flag.Name
			wantText(t, where+": the description", u.Desc)
			if !inAll[u.Flag] {
				t.Errorf("%s is not in allFlags, so the parser cannot read it", where)
			}
			if seen[u.Flag] {
				t.Errorf("%s is listed twice", where)
			}
			seen[u.Flag] = true
			if u.Flag == flagLang || u.Flag == flagHelp {
				t.Errorf("%s belongs in common, not in the command's own flags", where)
			}
		}
	}
	for _, u := range common {
		wantText(t, "--"+u.Flag.Name+" (common)", u.Desc)
	}
}

// TestSummaryIsInTheFullList: a command's summary is the sentence of its row in
// hergy --help, so the list and the command's own page do not say it two ways.
func TestSummaryIsInTheFullList(t *testing.T) {
	for _, lang := range i18n.Available {
		useLanguage(t, lang)
		list := Usage()
		for _, s := range specs {
			if !strings.Contains(list, "  "+s.Summary[lang]+"\n") {
				t.Errorf("[%s] the summary of hergy %s, %q, is not a row of the full list (hergy --help)", lang, s.Name, s.Summary[lang])
			}
		}
	}
}
