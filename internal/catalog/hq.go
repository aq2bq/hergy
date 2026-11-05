package catalog

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is what the headquarters (d-73fc) reads of one project's layer: how
// many names it holds, whether a name is there, and a catalog that reads one
// layer alone. Everything else about a layer is the reading this package already
// does; these are the counts and the questions a screen asks per project.

// Counts is how many names one layer holds: team definitions, role documents and
// profiles. The counts are the layer's own, not what a name resolves to, so a
// project that shadows a global name is counted once on each side, and the
// headquarters can show every project's own size.
func (c *Catalog) Counts(layer string) (definitions, roles, profiles int) {
	dir := c.LayerDir(layer)
	definitions = len(c.teamNames(dir))
	seen := map[string]bool{}
	for _, name := range c.roleNames(dir) {
		if seen[name] {
			continue
		}
		seen[name] = true
		roles++
	}
	path := filepath.Join(dir, "profiles.yaml")
	text, ok, err := c.fileText(path)
	if err != nil || !ok {
		return definitions, roles, 0
	}
	parsed, err := profile.ParseProfiles(text, path)
	if err != nil {
		return definitions, roles, 0
	}
	return definitions, roles, len(parsed)
}

// ListedRoles is how many role rows one layer's list draws (r-929e): a row per
// name the layer holds a document for, plus, on the project's side, the names
// the list still shows without a document of their own — a legacy .yaml, and a
// name only a team member names. A name standing in both layers is two rows,
// one on each side.
func (c *Catalog) ListedRoles(layer string) int {
	n := 0
	for _, entry := range c.Roles() {
		hasProject := entry.HasDocument && !entry.Outside
		hasGlobal := entry.Outside || entry.UnusedPath != ""
		switch layer {
		case source.LayerLocal:
			if hasProject || !entry.HasDocument {
				n++
			}
		case source.LayerGlobal:
			if hasGlobal {
				n++
			}
		}
	}
	return n
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

// LayerOnly returns a catalog that reads one layer and nothing else: the other
// layer's directory is a name nothing stands at. It is how the headquarters shows
// the global rows once, without every project's shadow of them (d-73fc).
func (c *Catalog) LayerOnly(layer string) *Catalog {
	out := *c
	none := filepath.Join(c.globalDir, ".hq", "no-layer")
	if layer == source.LayerGlobal {
		out.localDir = none
	} else {
		out.globalDir = none
	}
	out.overlays = nil
	out.hidden = nil
	out.reader = c.reader
	return &out
}
