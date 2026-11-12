// Package place owns canonical project identity and resource locations.
// Paths are made absolute, cleaned, then resolved through existing symlinks.
// A zero Project is unknown; it never means the current working directory.
package place

import (
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/domain/name"
)

// Project identifies one normalized absolute directory. Compare its known values with ==.
type Project struct{ path string }

// NewProject is the only constructor from an input path. Empty input is unknown.
// A missing path retains its cleaned absolute spelling.
func NewProject(path string) (Project, error) {
	if path == "" {
		return Project{}, nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	return Project{path: absolute}, nil
}

// String returns the normalized absolute path.
func (p Project) String() string { return p.path }

// IsZero reports unknown project identity.
func (p Project) IsZero() bool { return p.path == "" }

// MarshalText writes the canonical path as a JSON or text string.
func (p Project) MarshalText() ([]byte, error) { return []byte(p.path), nil }

// UnmarshalText is the JSON/record boundary, not an unchecked cast.
func (p *Project) UnmarshalText(text []byte) error {
	value, err := NewProject(string(text))
	if err == nil {
		*p = value
	}
	return err
}

// Comparison distinguishes unknown identity from known inequality.
type Comparison uint8

// Results of comparing two project identities.
const (
	Unknown Comparison = iota
	Same
	Different
)

// Compare uses normalized identity. Unknown is interpreted explicitly by each caller.
func (p Project) Compare(other Project) Comparison {
	if p.IsZero() || other.IsZero() {
		return Unknown
	}
	if p == other {
		return Same
	}
	return Different
}

// Contains reports whether another canonical directory is this project or below it.
func (p Project) Contains(dir Project) bool {
	if p.IsZero() || dir.IsZero() {
		return false
	}
	rel, err := filepath.Rel(p.path, dir.path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Seen preserves an external path's spelling, including an empty observation,
// while computing its identity exactly once when it enters the program.
type Seen struct {
	raw     string
	project Project
}

// Observe belongs only at protocol, environment and disk read boundaries.
func Observe(raw string) Seen {
	project, _ := NewProject(raw)
	return Seen{raw: raw, project: project}
}

// String returns the external spelling unchanged.
func (p Seen) String() string { return p.raw }

// Project returns the identity computed at observation.
func (p Seen) Project() Project { return p.project }

// MarshalText preserves an existing record's spelling.
func (p Seen) MarshalText() ([]byte, error) { return []byte(p.raw), nil }

// Seen returns a canonical model path ready for a new record.
func (p Project) Seen() Seen { return Seen{raw: p.path, project: p} }

// Place has precisely two branches: the shared global location, or a project.
// Its zero value is the global location.
type Place struct {
	project Project
	local   bool
}

// Global is the shared configuration location.
func Global() Place { return Place{} }

// In is the configuration location of one project.
func In(project Project) Place { return Place{project: project, local: true} }

// Project returns the project branch, or an unknown value for global.
func (p Place) Project() Project { return p.project }

// IsGlobal reports the global branch.
func (p Place) IsGlobal() bool { return !p.local }

// API returns the HTTP spelling of this location.
func (p Place) API() string {
	if p.IsGlobal() {
		return "global"
	}
	return "project"
}

// Layer returns the internal spelling of this location.
func (p Place) Layer() string {
	if p.IsGlobal() {
		return "global"
	}
	return "local"
}

// Selection is an unresolved boundary spelling, not a resource's Place.
// Both API and internal spellings are interpreted only here.
type Selection string

// String preserves an unresolved selector for the existing operation adapters.
func (s Selection) String() string { return string(s) }

// At resolves the selector in one project. Unknown selectors retain the local default.
func (s Selection) At(project Project) Place {
	if s == "global" {
		return Global()
	}
	return In(project)
}

// Layer adapts a boundary selector to the existing layer-based operations.
func (s Selection) Layer() string {
	if s == "global" {
		return "global"
	}
	return "local"
}

// API adapts an internal selector to the HTTP spelling.
func (s Selection) API() string {
	if s == "global" {
		return "global"
	}
	return "project"
}

// Kind is the kind of resource identified by a Ref.
type Kind uint8

// Kinds of resource addressed by a Ref.
const (
	Definition Kind = iota
	Role
	Profile
)

// Ref identifies a resource by kind, place and a name validated for that kind.
type Ref struct {
	kind       Kind
	place      Place
	definition name.DefinitionName
	role       name.RoleName
	profile    name.ProfileName
}

// DefinitionRef identifies a definition with its typed name.
func DefinitionRef(at Place, n name.DefinitionName) Ref {
	return Ref{kind: Definition, place: at, definition: n}
}

// RoleRef identifies a role with its typed name.
func RoleRef(at Place, n name.RoleName) Ref { return Ref{kind: Role, place: at, role: n} }

// ProfileRef identifies a profile with its typed name.
func ProfileRef(at Place, n name.ProfileName) Ref { return Ref{kind: Profile, place: at, profile: n} }

// Kind returns the resource kind.
func (r Ref) Kind() Kind { return r.kind }

// Place returns the resource location.
func (r Ref) Place() Place { return r.place }

// Name returns the name of the selected resource kind.
func (r Ref) Name() string {
	switch r.kind {
	case Role:
		return r.role.String()
	case Profile:
		return r.profile.String()
	default:
		return r.definition.String()
	}
}

// Input is a path still written as the caller supplied it. It has no identity
// until the CLI, HTTP or configuration boundary resolves it to a Project.
type Input string

// UnmarshalText constructs a canonical path. Preserving raw record spelling is
// reserved for private readers using Observe.
func (p *Seen) UnmarshalText(raw []byte) error {
	checked, err := NewProject(string(raw))
	if err != nil {
		return err
	}
	*p = checked.Seen()
	return nil
}
