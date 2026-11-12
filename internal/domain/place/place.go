// Package place exposes checked values; raw observations stay in ops/internal.
package place

import (
	"github.com/aq2bq/hergy/internal/domain/name"
	inner "github.com/aq2bq/hergy/internal/ops/model/place"
)

// Project is the shared domain value.
type Project = inner.Project

// NewProject uses the domain constructor or derivation.
func NewProject(path string) (Project, error) { return inner.NewProject(path) }

// Comparison is the shared domain value.
type Comparison = inner.Comparison

// Unknown identifies the domain case.
const Unknown = inner.Unknown

// Same identifies the domain case.
const Same = inner.Same

// Different identifies the domain case.
const Different = inner.Different

// Seen is the shared domain value.
type Seen = inner.Seen

// Place is the shared domain value.
type Place = inner.Place

// Global uses the domain constructor or derivation.
func Global() Place { return inner.Global() }

// In uses the domain constructor or derivation.
func In(project Project) Place { return inner.In(project) }

// Selection is the shared domain value.
type Selection = inner.Selection

// Kind is the shared domain value.
type Kind = inner.Kind

// Definition identifies the domain case.
const Definition = inner.Definition

// Role identifies the domain case.
const Role = inner.Role

// Profile identifies the domain case.
const Profile = inner.Profile

// Ref is the shared domain value.
type Ref = inner.Ref

// DefinitionRef uses the domain constructor or derivation.
func DefinitionRef(at Place, n name.DefinitionName) Ref { return inner.DefinitionRef(at, n) }

// RoleRef uses the domain constructor or derivation.
func RoleRef(at Place, n name.RoleName) Ref { return inner.RoleRef(at, n) }

// ProfileRef uses the domain constructor or derivation.
func ProfileRef(at Place, n name.ProfileName) Ref { return inner.ProfileRef(at, n) }

// Input is the shared domain value.
type Input = inner.Input
