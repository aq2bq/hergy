// Package effect exposes read models; implementations are private to ops.
package effect

import inner "github.com/aq2bq/hergy/internal/ops/internal/effect"

// Change is the shared resource model.
type Change = inner.Change

// Command is the shared resource model.
type Command = inner.Command

// HumanTurn is the shared resource model.
type HumanTurn = inner.HumanTurn

// Named is the shared resource model.
type Named = inner.Named

// Part is the shared resource model.
type Part = inner.Part

// PartChange is the shared resource model.
type PartChange = inner.PartChange

// Running is the shared resource model.
type Running = inner.Running

// TeamCommands is the shared resource model.
type TeamCommands = inner.TeamCommands

// View is the shared resource model.
type View = inner.View

// Agreements is the shared resource model.
const Agreements = inner.Agreements

// Env is the shared resource model.
const Env = inner.Env

// Launch is the shared resource model.
const Launch = inner.Launch

// RoleDocument is the shared resource model.
const RoleDocument = inner.RoleDocument

// Roster is the shared resource model.
const Roster = inner.Roster

// Turns is the shared resource model.
const Turns = inner.Turns

// Workers is the shared resource model.
const Workers = inner.Workers

// Commands is the shared resource model.
var Commands = inner.Commands

// Handover is the shared resource model.
var Handover = inner.Handover

// HireLines is the shared resource model.
var HireLines = inner.HireLines

// Of is the shared resource model.
var Of = inner.Of

// Preview is the shared resource model.
var Preview = inner.Preview

// PreviewOf is the shared resource model.
var PreviewOf = inner.PreviewOf

// Undelivered is the shared resource model.
var Undelivered = inner.Undelivered

// UndeliveredLines is the shared resource model.
var UndeliveredLines = inner.UndeliveredLines
