package catalog

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
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
// the name. A role a readable definition names is a row whether or not a
// document is there — a role is born when a member names one (3.5) — and a role
// document a layer holds is a row too.
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
	for _, def := range c.readable() {
		for _, member := range def.Graph.Members {
			add(member.Role)
		}
	}
	for _, one := range c.layers() {
		for _, name := range roleNames(one.dir) {
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

// rolePlace is where one layer's document for a role stands, whether or not it
// is there.
func (c *Catalog) rolePlace(layer, name string) string {
	return filepath.Join(c.LayerDir(layer), "roles", name+".yaml")
}

// roleNames lists the role documents under one layer, by role name.
func roleNames(dir string) []string {
	matches, err := filepath.Glob(filepath.Join(dir, "roles", "*.yaml"))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, strings.TrimSuffix(filepath.Base(match), ".yaml"))
	}
	sort.Strings(names)
	return names
}

// sortRoles turns the rows built by name into a list, by name.
func sortRoles(byName map[string]*RoleEntry) []RoleEntry {
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]RoleEntry, 0, len(names))
	for _, name := range names {
		out = append(out, *byName[name])
	}
	return out
}
