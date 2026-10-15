package rulesview

import (
	"strings"

	"github.com/aq2bq/hergy/internal/rules"
)

// The paths this screen answers on. They are here so the handlers, the forms
// and the links agree on one spelling; the templates ask for them by name with
// the p function.
const (
	PathRules = "/rules"
	// PathRulesList is the rules list alone, without the frame: what the search
	// box swaps into #rules-list while a person types (試作 rulesTab の検索).
	PathRulesList = "/rules/list"
	PathPanel     = "/rules/panel"
	PathPanelEdit = "/rules/panel/edit"
	PathPreview   = "/rules/preview"
	PathSave      = "/rules/save"
	PathDelete    = "/rules/delete"
	PathReport    = "/rules/report"
	PathBump      = "/rules/bump"
)

// EditRule is one rule as the edit panel's form holds it: the values a person
// types, and the place the rule stands at. A new rule has Section set and Index
// -1, so a save knows it is adding rather than replacing.
type EditRule struct {
	Section int
	Index   int
	ID      string
	Text    string
	Why     string
	Send    bool
	Note    string
	Report  string
	Tests   string // the ids as typed, separated by commas
	Origins []OriginRow
}

// OriginRow is one origin as the form holds it.
type OriginRow struct {
	Kind string
	Date string
	Refs string // the references as typed, separated by commas
	Note string
}

// Derived says whether this origin is the kind that points at other rules.
func (o OriginRow) Derived() bool { return o.Kind == rules.KindDerived }

// NewEditRule reads a rule into the form's shape.
func NewEditRule(section, index int, rule rules.Rule) EditRule {
	out := EditRule{
		Section: section,
		Index:   index,
		ID:      rule.ID,
		Text:    rule.Text,
		Why:     rule.Why,
		Send:    rule.SendWhy == nil || *rule.SendWhy,
		Note:    rule.Note,
		Report:  rule.Report,
		Tests:   strings.Join(rule.Tests, ", "),
	}
	for _, origin := range rule.Origin {
		out.Origins = append(out.Origins, OriginRow{
			Kind: origin.Kind,
			Date: origin.Date,
			Refs: strings.Join(origin.Refs, ", "),
			Note: origin.Note,
		})
	}
	return out
}

// Rule turns the form's shape back into a rule. A ticked "send the reason" is
// the default and stays unwritten; the rest of the fields are carried as typed.
func (e EditRule) Rule() rules.Rule {
	rule := rules.Rule{
		ID:     strings.TrimSpace(e.ID),
		Text:   e.Text,
		Why:    e.Why,
		Note:   e.Note,
		Report: strings.TrimSpace(e.Report),
		Tests:  SplitList(e.Tests),
	}
	if !e.Send {
		send := false
		rule.SendWhy = &send
	}
	for _, row := range e.Origins {
		origin := rules.Origin{
			Kind: strings.TrimSpace(row.Kind),
			Date: strings.TrimSpace(row.Date),
			Refs: SplitList(row.Refs),
			Note: row.Note,
		}
		if origin.Kind == "" {
			continue // a row with no kind chosen is not an origin
		}
		rule.Origin = append(rule.Origin, origin)
	}
	return rule
}

// SplitList reads a field a person typed as a comma-separated list.
func SplitList(value string) []string {
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ReportOption is one report field a rule may be pointed at: a field of one of
// the documents the same member reads (d-0c48), labelled with its document so
// two documents with the same report name are told apart.
type ReportOption struct {
	Ref   string
	Label string
}

// ReportOptions lists the report fields of the documents, in order.
func ReportOptions(docs []Doc) []ReportOption {
	var out []ReportOption
	for _, doc := range docs {
		if doc.Document == nil {
			continue
		}
		for _, report := range doc.Document.Reports {
			for _, field := range report.Fields {
				ref := report.Name + "." + field.Name
				out = append(out, ReportOption{Ref: ref, Label: ref + " (" + doc.Label() + ")"})
			}
		}
	}
	return out
}

// Panel is the right panel with one rule open for editing, or with a new one
// (New). Preview is the same rule as a member would read it, which the screen
// refreshes while a person types.
//
// Hash and OrigID travel with the form and are handed back at save time: Hash is
// the version of the file the panel was drawn from, and OrigID is the id the
// rule at that place had then. A panel drawn again from the form (an origin
// added) carries the two as they were, so a file that changed under the screen
// is refused rather than written into another rule's place (K1).
type Panel struct {
	Layer string
	Doc   Doc
	// View is the document the body was showing, which every link and form of
	// the panel carries as ?view= so a save comes back to that document and not
	// the one the panel edits (a related rule may stand in another document).
	View    string
	Edit    EditRule
	Reports []ReportOption
	// Related is the rules the panel lists beside the open one: what its text
	// points at, what points at it, and what was derived from it.
	Related Related
	// AskAI is the prompt the panel offers to copy: what to ask an AI to find
	// out where this rule came from.
	AskAI   string
	Preview Preview
	Hash    string
	OrigID  string
	Global  bool
	New     bool
	Token   string
	// Notice is one line under the form: what a save did, or why it refused.
	Notice string
	// Broken says the preview found a break, so the rule cannot be saved.
	Broken bool
	// AskingDelete says the form is asking whether to delete the rule.
	AskingDelete bool
	// SectionLabel is the section the rule stands in, as the crumb names it:
	// its heading, or that the section has none.
	SectionLabel string
	// Badges are the three marks above the form, for the rule as the form
	// holds it.
	Badges Badges
}

// Badges is the three marks above the edit form (the prototype's evBadges):
// whether the rule as it stands carries an origin, a reason and a check.
type Badges struct {
	Origin bool
	Reason bool
	Check  bool
}

// BadgesOf reads the three marks off the form's rule.
func BadgesOf(edit EditRule) Badges {
	return Badges{
		Origin: len(edit.Origins) > 0,
		Reason: strings.TrimSpace(edit.Why) != "",
		Check:  strings.TrimSpace(edit.Report) != "" || strings.TrimSpace(edit.Tests) != "",
	}
}
