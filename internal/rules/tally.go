package rules

// Tally counts the rule files one person's teams read, and the rules in them.
// A file is counted once however many teams read it, because the same document
// reaches every member of every team that lists it.
type Tally struct {
	Files int // the rule files counted
	Total int // the rules in them
}

// Add counts one document.
func (t *Tally) Add(doc *Document) {
	t.Files++
	t.Total += len(doc.Rules())
}
