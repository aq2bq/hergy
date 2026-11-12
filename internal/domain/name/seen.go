package name

import "strings"

// Target is an agent name accepted by herdr. Only a constructed HerdrName or
// an observation can be sent back to herdr; a bare string cannot be a target.
type Target interface {
	String() string
	herdrTarget()
}

func (HerdrName) herdrTarget() {}
func (Seen) herdrTarget()      {}

// Seen preserves a name read from herdr, including legacy and foreign names.
// It carries no promise about the name's shape.
type Seen struct{ s string }

// Observe marks the boundary where a name is read from an external source.
func Observe(s string) Seen   { return Seen{s: s} }
func (n Seen) String() string { return n.s }

// MarshalText preserves an observed spelling in records and JSON.
func (n Seen) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText restores an observation without validating its shape.
func (n *Seen) UnmarshalText(text []byte) error { n.s = string(text); return nil }

// PaneName is a constructed pane label, which has no agent-name length limit.
type PaneName struct{ s string }

func (n PaneName) String() string { return n.s }

// Resolve finds the longest known team prefix. Observations with an empty short
// name are not members. Neither component is validated: old tabs remain visible.
func Resolve(raw Seen, teams []Seen) (team, short Seen, ok bool) {
	for _, candidate := range teams {
		if candidate.s != "" && len(raw.s) > len(candidate.s)+1 && len(candidate.s) > len(team.s) && strings.HasPrefix(raw.s, candidate.s+"-") {
			team = candidate
		}
	}
	if team.s == "" {
		return Seen{}, raw, false
	}
	return team, Observe(raw.s[len(team.s)+1:]), true
}

// WithShort reconstructs an observed name from an observed team and short name.
// It is used for legacy worker tab labels, which are not model names.
func (n Seen) WithShort(short Seen) Seen { return Observe(targetSpelling(n.s, short.s)) }

// WorkerTab reads a worker tab without discarding legacy names or slash suffixes.
func (n Seen) WorkerTab() (team, short Seen, ok bool) {
	rest, ok := strings.CutPrefix(n.s, "worker:")
	if !ok {
		return Seen{}, Seen{}, false
	}
	a, b, ok := strings.Cut(rest, "/")
	if !ok || a == "" || b == "" {
		return Seen{}, Seen{}, false
	}
	return Observe(a), Observe(b), true
}

// PreviewName is a display value, never an agent target. Preview placeholders
// and existing definition directory names need not satisfy model naming rules.
func PreviewName(team, short string) string { return targetSpelling(team, short) }

// Team is the fallible conversion from an existing definition to a run name.
func (n DefinitionName) Team() (TeamName, error) { return ParseTeamName(n.s) }

// MemberForInput resolves a run spelling at the legacy input boundary. Failures
// carry the composed spelling so callers preserve the existing diagnostic.
func MemberForInput(team string, member MemberName) (HerdrName, error) {
	definition, _ := ParseDefinitionName(team)
	t, err := definition.Team()
	if err != nil {
		return HerdrName{}, &Problem{Kind: Herdr, Value: PreviewName(team, member.String()), Max: 32}
	}
	return t.Member(member)
}

// WorkerForInput resolves a run spelling at the legacy hire boundary.
func WorkerForInput(team string, worker WorkerName) (HerdrName, error) {
	t, err := ParseTeamName(team)
	if err != nil {
		return HerdrName{}, &Problem{Kind: Herdr, Value: PreviewName(team, worker.String()), Max: 32}
	}
	return t.Worker(worker)
}

// PaneLabel is either a model agent label, a watcher label, or a label read from
// herdr. It cannot be implemented outside this package.
type PaneLabel interface {
	String() string
	paneLabel()
}

func (HerdrName) paneLabel() {}
func (PaneName) paneLabel()  {}
func (Seen) paneLabel()      {}

// Watcher preserves the pane label of an observed legacy team.
func (n Seen) Watcher() PaneName { return PaneName{s: targetSpelling(n.s, "turns")} }

// ShortIn reads a short observed name in a tab or prompt already scoped to a
// single team. Unmatched observations keep their spelling.
func ShortIn(raw, team Seen) Seen { _, short, _ := Resolve(raw, []Seen{team}); return short }

// BelongsTo tests membership in an already scoped team using Resolve's rule.
func BelongsTo(raw, team Seen) bool { _, _, ok := Resolve(raw, []Seen{team}); return ok }

// WorkerLabel reconstructs an observed worker's tab label without validating it.
func (n Seen) WorkerLabel(short Seen) Seen { return Observe(workerLabel(n.s, short.s)) }

// WorkerShort reads the suffix of one known team's worker label. Empty suffixes
// remain distinguishable from a label of a different team.
func WorkerShort(label, team Seen) (Seen, bool) {
	s, ok := strings.CutPrefix(label.s, workerLabel(team.s, ""))
	return Observe(s), ok
}

// IsWorkerTab reports whether a tab is in an already scoped team's worker tabs.
func IsWorkerTab(label, team Seen) bool { _, ok := WorkerShort(label, team); return ok }

// TeamRef is a running team's identity: a model team name when the spelling
// fits, otherwise the original legacy observation. Neither branch is dropped.
type TeamRef struct {
	model  TeamName
	legacy Seen
}

// TeamFromObservation keeps invalid legacy labels as observations.
func TeamFromObservation(seen Seen) TeamRef {
	if model, err := ParseTeamName(seen.String()); err == nil {
		return TeamRef{model: model}
	}
	return TeamRef{legacy: seen}
}
func (n TeamRef) String() string {
	if n.model.s != "" {
		return n.model.String()
	}
	return n.legacy.String()
}

// Model returns the checked name, if this row has one.
func (n TeamRef) Model() (TeamName, bool) { return n.model, n.model.s != "" }

// Observation returns the original spelling for read-only and close operations.
func (n TeamRef) Observation() Seen { return Observe(n.String()) }

// ShortName is the short part of a constructed member or worker target.
type ShortName struct{ s string }

func (n ShortName) String() string { return n.s }

// Short is this member's short target identity.
func (n MemberName) Short() ShortName { return ShortName(n) }

// Short is this worker's short target identity.
func (n WorkerName) Short() ShortName { return ShortName(n) }

// CheckRenameSource preserves the rename endpoint's existing source-name
// restriction. Existing definition readers themselves accept directory names.
func CheckRenameSource(raw string) error { return plain(raw, 32, Definition) }

func targetSpelling(team, short string) string { return team + "-" + short }
func workerLabel(team, short string) string    { return "worker:" + team + "/" + short }

// MarshalText preserves a short name's spelling in records and JSON.
func (n ShortName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText restores the same short-name rule as members and workers.
func (n *ShortName) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*n = ShortName{}
		return nil
	}
	member, err := ParseMemberName(string(text))
	if err != nil {
		return err
	}
	*n = member.Short()
	return nil
}

// MarshalText preserves either branch of a running team identity.
func (n TeamRef) MarshalText() ([]byte, error) { return []byte(n.String()), nil }

// UnmarshalText restores the checked or legacy branch without dropping a row.
func (n *TeamRef) UnmarshalText(text []byte) error {
	*n = TeamFromObservation(Observe(string(text)))
	return nil
}
