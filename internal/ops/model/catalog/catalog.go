// Package catalog exposes read models; implementations are private to ops.
package catalog

import inner "github.com/aq2bq/hergy/internal/ops/internal/catalog"

// Catalog is the shared resource model.
type Catalog = inner.Catalog

// Entry is the shared resource model.
type Entry = inner.Entry

// ListedRow is the shared resource model.
type ListedRow = inner.ListedRow

// Listing is the shared resource model.
type Listing = inner.Listing

// MemberRef is the shared resource model.
type MemberRef = inner.MemberRef

// ProfileEntry is the shared resource model.
type ProfileEntry = inner.ProfileEntry

// ReadError is the shared resource model.
type ReadError = inner.ReadError

// RoleEntry is the shared resource model.
type RoleEntry = inner.RoleEntry

// KindProfile is the shared resource model.
const KindProfile = inner.KindProfile

// KindRole is the shared resource model.
const KindRole = inner.KindRole

// KindTeam is the shared resource model.
const KindTeam = inner.KindTeam

// NewProfile is the shared resource model.
const NewProfile = inner.NewProfile

// NewRole is the shared resource model.
const NewRole = inner.NewRole

// NewTeam is the shared resource model.
const NewTeam = inner.NewTeam

// PlaceGlobal is the shared resource model.
const PlaceGlobal = inner.PlaceGlobal

// PlaceProject is the shared resource model.
const PlaceProject = inner.PlaceProject

// StateUnreadable is the shared resource model.
const StateUnreadable = inner.StateUnreadable

// ErrNotFound is the shared resource model.
var ErrNotFound = inner.ErrNotFound

// New is the shared resource model.
var New = inner.New

// RoleNameProblem is the shared resource model.
var RoleNameProblem = inner.RoleNameProblem
