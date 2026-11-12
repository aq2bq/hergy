package catalog

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/profile"
)

// This file is what the headquarters (d-73fc) reads of one project's layer: how
// many names it holds, whether a name is there, and a catalog that reads one
// layer alone. Everything else about a layer is the reading this package already
// does; these are the counts and the questions a screen asks per project.

// CountRows is the band's three numbers for one layer: the rows that layer's lists
// draw (listedrows.go owns the rule). A number is a number of rows, never a
// separate count of files (r-ea17, d-4d88).
func CountRows(c *Catalog, layer string) (definitions, roles, profiles int) {
	return len(c.shownRows(KindTeam, layer)), len(c.shownRows(KindRole, layer)), len(c.shownRows(KindProfile, layer))
}

// HoldsDefinition reports whether one layer holds a team definition of this name.
func (c *Catalog) HoldsDefinition(name, layer string) bool {
	return c.hasFile(c.TeamYAML(name, layer))
}

// HoldsRole reports whether one layer holds a role document of this name: the
// .md hergy reads, or a legacy .yaml awaiting explicit migration.
func (c *Catalog) HoldsRole(name, layer string) bool {
	dir := c.LayerDir(layer)
	return c.hasFile(filepath.Join(dir, "roles", name+".yaml")) ||
		c.hasFile(filepath.Join(dir, "roles", name+".md"))
}

// HoldsProfile reports whether one layer's profiles.yaml defines this name.
func (c *Catalog) HoldsProfile(name, layer string) bool {
	for _, one := range c.layers() {
		if one.name != layer {
			continue
		}
		text, ok, err := c.fileText(filepath.Join(one.dir, "profiles.yaml"))
		if err != nil || !ok {
			return false
		}
		parsed, err := profile.ParseProfiles(text, one.dir)
		if err != nil {
			return false
		}
		_, found := parsed[name]
		return found
	}
	return false
}
