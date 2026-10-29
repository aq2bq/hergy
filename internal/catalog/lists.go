package catalog

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// RoleEntry is one row of the roles list a screen shows (3.5, spec/38 T2), in
// the shape a team definition's row has (C1): a role is a name in a layer, so a
// person reads it without being told which layer it came from (I11).
type RoleEntry struct {
	// Name is the role's name, as a member's role writes it.
	Name string
	// Path is the document hergy reads for this role: the project's layer
	// first, then the global one. It is "" when no layer holds one, and
	// HasDocument says which.
	Path string
	// Outside says the document that is there stands outside the project (it
	// is under the global directory), so a screen marks the row with that
	// path.
	Outside bool
	// UnusedPath is the same-named document that is not used: the other
	// layer's when both hold one, and "" when they do not.
	UnusedPath string
	// HasDocument says a document is there to read. A role a definition names
	// may have none; the row stays, and a screen offers to make one (3.5, W8).
	HasDocument bool
}

// Roles is every role a name resolves to, as the roles list shows it (3.5,
// spec/38 T2): one row per name, the project's document where both layers hold
// the name. A role a team definition names is a row whether or not a document is
// there — a role is born when a member names one (3.5), and a definition that
// does not read still names its members' roles (spec-roles 1.2.10) — and a role
// document a layer holds is a row too. The order is the project's document names
// first, then the names only the shared layer holds, then the names no document
// holds (spec-roles 1.2「並び」).
func (c *Catalog) Roles() []RoleEntry {
	byName := map[string]*RoleEntry{}
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := byName[name]; !ok {
			byName[name] = &RoleEntry{Name: name}
		}
	}
	for _, role := range c.namedRoles() {
		add(role)
	}
	for _, one := range c.layers() {
		for _, name := range c.roleNames(one.dir) {
			add(name)
		}
	}
	for name, row := range byName {
		local, global := c.rolePlace(source.LayerLocal, name), c.rolePlace(source.LayerGlobal, name)
		switch {
		case c.hasFile(local):
			row.Path, row.HasDocument = local, true
			if c.hasFile(global) {
				row.UnusedPath = global
			}
		case c.hasFile(global):
			row.Path, row.Outside, row.HasDocument = global, true, true
		}
	}
	return sortRoles(byName)
}

// ProfileEntry is one row of the profiles list a screen shows (3.7, spec/38 T2),
// in the shape a team definition's row has (C1). A profile has no row of its own
// for "a document is not there": a row is born where a profiles.yaml defines the
// name, so there is always one to read.
type ProfileEntry struct {
	// Name is the profile's name, the key under profiles:.
	Name string
	// Kind is the kind the profile runs (hergy keeps no closed list), read from
	// the same file the name resolves to.
	Kind string
	// Path is the profiles.yaml hergy reads this profile from: the project's
	// layer first, then the global one (K1).
	Path string
	// Outside says the profile stands outside the project (its profiles.yaml
	// is under the global directory).
	Outside bool
	// UnusedPath is the other layer's profiles.yaml when it defines the same
	// name, and "" when it does not.
	UnusedPath string
}

// ProfileEntries is every profile a name resolves to, as the profiles list shows
// it (3.7, spec/38 T2): one row per name, the project's profiles.yaml where both
// layers define the name.
//
// A layer whose profiles.yaml does not read contributes no rows: what cannot be
// read has no names to show, and the definitions that name one of them say so
// where they are read.
func (c *Catalog) ProfileEntries() []ProfileEntry {
	byName := map[string]*ProfileEntry{}
	for _, one := range c.layers() { // the project's layer first (K1)
		path := filepath.Join(one.dir, "profiles.yaml")
		text, ok, err := c.fileText(path)
		if err != nil || !ok {
			continue
		}
		profiles, err := profile.ParseProfiles(text, path)
		if err != nil {
			continue
		}
		for name, value := range profiles {
			row, ok := byName[name]
			if !ok {
				row = &ProfileEntry{Name: name}
				byName[name] = row
			}
			if row.Path == "" {
				row.Path = path
				row.Kind = value.Kind
				row.Outside = one.name == source.LayerGlobal
				continue
			}
			row.UnusedPath = path
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ProfileEntry, 0, len(names))
	for _, name := range names {
		out = append(out, *byName[name])
	}
	return out
}

// namedRoles is every role the team definitions name, read from each file as it
// is: a definition that does not read still names the roles its members play, so
// the role screen shows a row for a role a member points at (spec-roles 1.2.10).
func (c *Catalog) namedRoles() []string {
	var out []string
	for _, listing := range c.Definitions() {
		path := listing.GlobalPath
		if listing.Local {
			path = listing.LocalPath
		}
		text, ok, err := c.fileText(path)
		if err != nil || !ok {
			continue
		}
		var raw struct {
			Members []struct {
				Role string `yaml:"role"`
			} `yaml:"members"`
		}
		if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
			continue
		}
		for _, member := range raw.Members {
			out = append(out, member.Role)
		}
	}
	return out
}

// rolePlace is where one layer's document for a role stands, whether or not it
// is there.
func (c *Catalog) rolePlace(layer, name string) string {
	return filepath.Join(c.LayerDir(layer), "roles", name+".yaml")
}

// roleNames lists the role documents under one layer, by role name: the .yaml
// hergy reads, and a .md still left as Markdown, which the screen shows as a row
// that says so (spec-roles 1.2.10).
// roleNames is one layer's role names, under the version of its roles/
// directory (d-e00b).
func (c *Catalog) roleNames(dir string) []string {
	return c.dirNames(filepath.Join(dir, "roles"), func() []string { return roleNames(dir) })
}

func roleNames(dir string) []string {
	var names []string
	for _, ext := range []string{"yaml", "md"} {
		matches, err := filepath.Glob(filepath.Join(dir, "roles", "*."+ext))
		if err != nil {
			continue
		}
		for _, match := range matches {
			names = append(names, strings.TrimSuffix(filepath.Base(match), "."+ext))
		}
	}
	return names
}

// sortRoles turns the rows built by name into a list: the project's documents by
// name, then the names only the shared layer holds by name, then the names no
// document holds by name (spec-roles 1.2「並び」).
func sortRoles(byName map[string]*RoleEntry) []RoleEntry {
	rows := make([]RoleEntry, 0, len(byName))
	for _, row := range byName {
		rows = append(rows, *row)
	}
	rank := func(row RoleEntry) int {
		switch {
		case row.HasDocument && !row.Outside:
			return 0
		case row.HasDocument:
			return 1
		default:
			return 2
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := rank(rows[i]), rank(rows[j])
		if ri != rj {
			return ri < rj
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}
