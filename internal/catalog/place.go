package catalog

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/source"
)

// The kinds of thing NewPlace places. One name per thing hergy makes, so the
// caller says what it is making rather than writing a path of its own.
const (
	// NewTeam is a team definition: teams/<name>/ in the layer, the directory
	// hergy new writes team.yaml and agreement.yaml into.
	NewTeam = "team"
	// NewRole is a role document: roles/<name>.yaml in the layer. A role
	// document is the layer's, shared by every team of it (K5).
	NewRole = "role"
	// NewProfile is a layer's profiles.yaml. A profile is one key inside that
	// file, so adding one writes there; the file is the layer's, as every other
	// name is looked up (K1).
	NewProfile = "profile"
)

// NewPlace is where something new of one kind goes (K2): inside a git
// repository, the project's own layer, and outside one the global layer. It is
// one function, so hergy new and a screen that makes a role document or adds a
// profile write in the same place. What it returns is the thing itself: the
// definition's directory, the role document's file, the profiles.yaml.
//
// A kind it does not know answers "", and so does a catalog that was never told
// whether the project is a repository (InRepo): the caller asked for a thing
// hergy does not make, or the catalog cannot know where it goes, and to write
// outside the project by default would be worse than no answer (spec/38 T3).
func (c *Catalog) NewPlace(kind, name string) string {
	if !c.inRepoKnown {
		return ""
	}
	dir := c.LayerDir(c.newLayer())
	switch kind {
	case NewTeam:
		return filepath.Join(dir, "teams", name)
	case NewRole:
		return filepath.Join(dir, "roles", name+".yaml")
	case NewProfile:
		return filepath.Join(dir, "profiles.yaml")
	default:
		return ""
	}
}

// newLayer is the layer something new goes into (K2): the project's own inside
// a git repository, the global one outside it.
func (c *Catalog) newLayer() string {
	if c.inRepo {
		return source.LayerLocal
	}
	return source.LayerGlobal
}
