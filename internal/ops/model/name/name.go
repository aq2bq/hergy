// Package name exposes checked values; raw observations stay in ops/internal.
package name

import inner "github.com/aq2bq/hergy/internal/ops/internal/name"

// Kind is the shared domain value.
type Kind = inner.Kind

// Team identifies the domain case.
const Team = inner.Team

// Definition identifies the domain case.
const Definition = inner.Definition

// Member identifies the domain case.
const Member = inner.Member

// Worker identifies the domain case.
const Worker = inner.Worker

// Role identifies the domain case.
const Role = inner.Role

// Profile identifies the domain case.
const Profile = inner.Profile

// PurposeKind identifies the domain case.
const PurposeKind = inner.PurposeKind

// Herdr identifies the domain case.
const Herdr = inner.Herdr

// Rule is the shared domain value.
type Rule = inner.Rule

// EmptyPurpose identifies the domain case.
const EmptyPurpose = inner.EmptyPurpose

// MultiplePurposes identifies the domain case.
const MultiplePurposes = inner.MultiplePurposes

// Problem is the shared domain value.
type Problem = inner.Problem

// TeamName is the shared domain value.
type TeamName = inner.TeamName

// NewTeamName uses the domain constructor or derivation.
func NewTeamName(s string) (TeamName, error) { return inner.NewTeamName(s) }

// ParseTeamName uses the domain constructor or derivation.
func ParseTeamName(s string) (TeamName, error) { return inner.ParseTeamName(s) }

// DefinitionName is the shared domain value.
type DefinitionName = inner.DefinitionName

// NewDefinitionName uses the domain constructor or derivation.
func NewDefinitionName(s string) (DefinitionName, error) { return inner.NewDefinitionName(s) }

// ParseDefinitionName uses the domain constructor or derivation.
func ParseDefinitionName(s string) (DefinitionName, error) { return inner.ParseDefinitionName(s) }

// MemberName is the shared domain value.
type MemberName = inner.MemberName

// ParseMemberName uses the domain constructor or derivation.
func ParseMemberName(s string) (MemberName, error) { return inner.ParseMemberName(s) }

// WorkerName is the shared domain value.
type WorkerName = inner.WorkerName

// ParseWorkerName uses the domain constructor or derivation.
func ParseWorkerName(s string) (WorkerName, error) { return inner.ParseWorkerName(s) }

// RoleName is the shared domain value.
type RoleName = inner.RoleName

// ParseRoleName uses the domain constructor or derivation.
func ParseRoleName(s string) (RoleName, error) { return inner.ParseRoleName(s) }

// ProfileName is the shared domain value.
type ProfileName = inner.ProfileName

// ParseProfileName uses the domain constructor or derivation.
func ParseProfileName(s string) (ProfileName, error) { return inner.ParseProfileName(s) }

// Purpose is the shared domain value.
type Purpose = inner.Purpose

// ParsePurpose uses the domain constructor or derivation.
func ParsePurpose(s string) (Purpose, error) { return inner.ParsePurpose(s) }

// HerdrName is the shared domain value.
type HerdrName = inner.HerdrName

// ParseWorkerTab uses the domain constructor or derivation.
func ParseWorkerTab(label string) (TeamName, WorkerName, bool) { return inner.ParseWorkerTab(label) }

// Naming is the shared domain value.
type Naming = inner.Naming

// RunningNames uses the domain constructor or derivation.
func RunningNames(team TeamRef) Naming { return inner.RunningNames(team) }

// DefinitionNames uses the domain constructor or derivation.
func DefinitionNames(definition DefinitionName) Naming { return inner.DefinitionNames(definition) }

// NamingSet is the shared domain value.
type NamingSet = inner.NamingSet

// Target is the shared domain value.
type Target = inner.Target

// Seen is the shared domain value.
type Seen = inner.Seen

// PaneName is the shared domain value.
type PaneName = inner.PaneName

// Resolve uses the domain constructor or derivation.
func Resolve(raw Seen, teams []Seen) (team, short Seen, ok bool) { return inner.Resolve(raw, teams) }

// ObservedWorker is the shared domain value.
type ObservedWorker = inner.ObservedWorker

// PaneLabel is the shared domain value.
type PaneLabel = inner.PaneLabel

// ShortIn uses the domain constructor or derivation.
func ShortIn(raw, team Seen) Seen { return inner.ShortIn(raw, team) }

// BelongsTo uses the domain constructor or derivation.
func BelongsTo(raw, team Seen) bool { return inner.BelongsTo(raw, team) }

// IsWorkerTab uses the domain constructor or derivation.
func IsWorkerTab(label, team Seen) bool { return inner.IsWorkerTab(label, team) }

// TeamRef is the shared domain value.
type TeamRef = inner.TeamRef

// TeamFromObservation uses the domain constructor or derivation.
func TeamFromObservation(seen Seen) TeamRef { return inner.TeamFromObservation(seen) }

// ShortName is the shared domain value.
type ShortName = inner.ShortName

// CheckRenameSource uses the domain constructor or derivation.
func CheckRenameSource(raw string) error { return inner.CheckRenameSource(raw) }

// PaneID is the shared domain value.
type PaneID = inner.PaneID
