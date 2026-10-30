package catalog

import (
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// Entry is one row of the team definition list a screen shows (C1): what a
// person reads about a name without being told which layer it came from (I11).
type Entry struct {
	// Name is the definition's name: the directory under teams/.
	Name string
	// Path is the team.yaml hergy reads for this name.
	Path string
	// Outside says the definition stands outside the project (its file is under
	// the global directory), so a screen marks the row with that path.
	Outside bool
	// UnusedPath is the path of the same name that is not used: the other
	// layer's team.yaml when both hold the name, and "" when they do not.
	UnusedPath string
	// Description is the one line saying which case the team is meant for. It
	// is "" when the definition does not read, and Err says why.
	Description string
	// Err is why the definition does not read, and nil when it does.
	Err error
}

// Entries is every team definition a name resolves to, as a list shows it (C1,
// I11): one row per name, the project's own where both layers hold the name. A
// name that is there but does not read is a row too, with Err set: the list does
// not drop a definition because its file is broken (spec/31 D1).
func (c *Catalog) Entries() []Entry {
	names := c.candidateNames()
	out := make([]Entry, 0, len(names))
	for _, name := range names {
		local := c.TeamYAML(name, source.LayerLocal)
		global := c.TeamYAML(name, source.LayerGlobal)
		entry := Entry{Name: name}
		if c.hasFile(local) {
			entry.Path = local
			if c.hasFile(global) {
				entry.UnusedPath = global
			}
		} else {
			entry.Path = global
			entry.Outside = true
		}
		def, err := c.Read(name)
		if err != nil {
			entry.Err = err
		} else {
			entry.Description = def.Description
		}
		out = append(out, entry)
	}
	return out
}

// Read reads one team definition by name, the one way a name is resolved (K1,
// I11): the project's own first, then the one outside it. A screen reads by name
// alone through this, so no screen is handed a layer; validate and the screens
// that show one layer take Definition.
func (c *Catalog) Read(name string) (*definition.Definition, error) {
	return c.Definition(name, "")
}
