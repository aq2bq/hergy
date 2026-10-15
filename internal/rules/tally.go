package rules

// Tally counts what the rules of the files one person's teams read carry: where
// each rule came from, why it is there, and whether something shows it was kept.
// hergy validate prints it after the per-file lines, so the rules that came from
// nothing show up instead of staying in the file unnoticed.
//
// A file is counted once however many teams read it, because the same document
// reaches every member of every team that lists it.
type Tally struct {
	Files       int // the rule files counted
	Total       int // the rules in them
	WithOrigin  int
	Incident    int // rules with an origin of that kind, at least once
	Instruction int
	Derived     int
	WithWhy     int // a reason, whether or not it is sent
	Report      int // a rule that names a report field
	Tests       int // a rule that names a test
	WithCheck   int // a report field or a test
	Missing     []MissingOrigin
}

// MissingOrigin is the rules of one file that carry no origin.
type MissingOrigin struct {
	Path string
	IDs  []string
}

// Add counts one document.
func (t *Tally) Add(doc *Document) {
	t.Files++
	var missing []string
	for _, rule := range doc.Rules() {
		t.count(rule)
		if len(rule.Origin) == 0 {
			missing = append(missing, rule.ID)
		}
	}
	if len(missing) > 0 {
		t.Missing = append(t.Missing, MissingOrigin{Path: doc.Path, IDs: missing})
	}
}

// count adds one rule. note is not a reason (01_規則の形式と変換.md 4 節), and a
// reason counts whether or not the file sends it.
func (t *Tally) count(rule Rule) {
	t.Total++
	if len(rule.Origin) > 0 {
		t.WithOrigin++
	}
	kinds := map[string]bool{}
	for _, origin := range rule.Origin {
		kinds[origin.Kind] = true
	}
	if kinds[KindIncident] {
		t.Incident++
	}
	if kinds[KindInstruction] {
		t.Instruction++
	}
	if kinds[KindDerived] {
		t.Derived++
	}
	if rule.Why != "" {
		t.WithWhy++
	}
	if rule.Report != "" {
		t.Report++
	}
	if len(rule.Tests) > 0 {
		t.Tests++
	}
	if rule.Report != "" || len(rule.Tests) > 0 {
		t.WithCheck++
	}
}
