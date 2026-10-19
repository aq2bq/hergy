package catalog

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/source"
)

// This file is where the two words a screen uses for a layer meet the layer
// names hergy keeps inside itself (I11). A screen says "project" for the
// project's own layer and "global" for the shared one; hergy's own names are
// local and global. One mapping, here, so a screen that writes a file and the
// check that reads it cannot disagree about which layer a place is.

// The two words a screen uses for a layer.
const (
	// PlaceProject is the project's own layer.
	PlaceProject = "project"
	// PlaceGlobal is the shared layer.
	PlaceGlobal = "global"
)

// Layer is the layer a screen's place names. Anything that is not the global
// place is the project's own, so a place hergy does not read is still a layer
// hergy can write.
func Layer(place string) string {
	if place == PlaceGlobal {
		return source.LayerGlobal
	}
	return source.LayerLocal
}

// RolePath is where a role document stands in one place: roles/<name>.yaml in
// that layer. A screen names the place, not the directory.
func (c *Catalog) RolePath(place, name string) string {
	return filepath.Join(c.LayerDir(Layer(place)), "roles", name+".yaml")
}
