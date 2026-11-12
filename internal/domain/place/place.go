// Package place owns canonical project identity and resource locations.
// Paths are made absolute, cleaned, then resolved through existing symlinks.
// A zero Project is unknown; it never means the current working directory.
package place

import (
	"path/filepath"

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

func (p Project) String() string               { return p.path }
func (p Project) IsZero() bool                 { return p.path == "" }
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
	return err == nil && rel != ".." && !startsParent(rel)
}
func startsParent(path string) bool {
	return len(path) > 3 && path[:3] == ".."+string(filepath.Separator)
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
func (p Seen) String() string                   { return p.raw }
func (p Seen) Project() Project                 { return p.project }
func (p Seen) MarshalText() ([]byte, error)     { return []byte(p.raw), nil }
func (p *Seen) UnmarshalText(text []byte) error { *p = Observe(string(text)); return nil }

// Seen returns a canonical model path ready for a new record.
func (p Project) Seen() Seen { return Seen{raw: p.path, project: p} }

// Place has precisely two branches: the shared global location, or a project.
// Its zero value is the global location.
type Place struct {
	project Project
	local   bool
}

func Global() Place              { return Place{} }
func In(project Project) Place   { return Place{project: project, local: true} }
func (p Place) Project() Project { return p.project }
func (p Place) IsGlobal() bool   { return !p.local }
func (p Place) API() string {
	if p.IsGlobal() {
		return "global"
	}
	return "project"
}
func (p Place) Layer() string {
	if p.IsGlobal() {
		return "global"
	}
	return "local"
}

// Selection is an unresolved boundary spelling, not a resource's Place.
// Both API and internal spellings are interpreted only here.
type Selection string

func (s Selection) At(project Project) Place {
	if s == "global" {
		return Global()
	}
	return In(project)
}
func (s Selection) Layer() string {
	if s == "global" {
		return "global"
	}
	return "local"
}
func (s Selection) API() string {
	if s == "global" {
		return "global"
	}
	return "project"
}

// Kind is the kind of resource identified by a Ref.
type Kind uint8

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

func DefinitionRef(at Place, n name.DefinitionName) Ref {
	return Ref{kind: Definition, place: at, definition: n}
}
func RoleRef(at Place, n name.RoleName) Ref       { return Ref{kind: Role, place: at, role: n} }
func ProfileRef(at Place, n name.ProfileName) Ref { return Ref{kind: Profile, place: at, profile: n} }
func (r Ref) Kind() Kind                          { return r.kind }
func (r Ref) Place() Place                        { return r.place }
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
