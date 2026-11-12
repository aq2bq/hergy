package catalog

import (
	"path/filepath"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// domain/place owns API/internal spelling conversion; these constants label DTOs.

// The two words a screen uses for a layer.
const (
	// PlaceProject is the project's own layer.
	PlaceProject = "project"
	// PlaceGlobal is the shared layer.
	PlaceGlobal = "global"
)

// RolePath is where a role document stands in one place: roles/<name>.md in
// that layer. A screen names the place, not the directory.
func (c *Catalog) RolePath(place, name string) string {
	return filepath.Join(c.LayerDir(domainplace.Selection(place).Layer()), "roles", name+".md")
}
