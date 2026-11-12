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
	return team, Seen{s: raw.s[len(team.s)+1:]}, true
}

// ObservedWorker is read from a single worker tab label. Its parts cannot be
// supplied independently, so deriving its agent cannot concatenate arbitrary observations.
type ObservedWorker struct{ label, team, short Seen }

// Team is the team part read from the tab.
func (n ObservedWorker) Team() Seen { return n.team }

// Short is the worker part read from the tab.
func (n ObservedWorker) Short() Seen { return n.short }

// Label retains the complete observed tab label.
func (n ObservedWorker) Label() Seen { return n.label }

// Agent derives the recipient witnessed by this one worker tab.
func (n ObservedWorker) Agent() Seen { return Seen{s: targetSpelling(n.team.s, n.short.s)} }

// WorkerTab reads a worker tab without discarding legacy names or slash suffixes.
func (n Seen) WorkerTab() (ObservedWorker, bool) {
	rest, ok := strings.CutPrefix(n.s, "worker:")
	if !ok {
		return ObservedWorker{}, false
	}
	a, b, ok := strings.Cut(rest, "/")
	if !ok || a == "" || b == "" {
		return ObservedWorker{}, false
	}
	return ObservedWorker{label: n, team: Seen{s: a}, short: Seen{s: b}}, true
}

// Team is the fallible conversion from an existing definition to a run name.
func (n DefinitionName) Team() (TeamName, error) { return ParseTeamName(n.s) }

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

// WorkerShort reads the suffix of one known team's worker label. Empty suffixes
// remain distinguishable from a label of a different team.
func WorkerShort(label, team Seen) (Seen, bool) {
	s, ok := strings.CutPrefix(label.s, workerLabel(team.s, ""))
	return Seen{s: s}, ok
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
func (n TeamRef) Observation() Seen {
	if n.model.s != "" {
		return n.model.Observation()
	}
	return n.legacy
}

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

// TeamLabel reads the legacy team label, keeping its spelling as an observation.
func (n Seen) TeamLabel() (team Seen, definition string, ok bool) {
	rest, older := strings.CutPrefix(n.s, "team:")
	if !older {
		return n, n.s, n.s != ""
	}
	before, after, split := strings.Cut(rest, "@")
	if split {
		if before == "" || after == "" {
			return Seen{}, "", false
		}
		return Seen{s: before}, after, true
	}
	return Seen{s: rest}, rest, rest != ""
}

// Observation retains an already observed spelling.
func (n Seen) Observation() Seen { return n }

// PaneID identifies a pane, independently of the spelling of any agent name.
type PaneID string

func (id PaneID) String() string { return string(id) }
func (PaneID) herdrTarget()      {}

// WorkerIn reads a worker label already scoped to a known team. Unlike the
// unscoped tab reader it retains legacy team names containing a slash. The
// scope may be an unresolved typed command word: it becomes an observation
// only after matching bytes in this one witnessed label.
func (n Seen) WorkerIn(team interface{ String() string }) (ObservedWorker, bool) {
	scoped := Seen{s: team.String()}
	short, ok := WorkerShort(n, scoped)
	if !ok || short.s == "" {
		return ObservedWorker{}, false
	}
	return ObservedWorker{label: n, team: scoped, short: short}, true
}

// UnmarshalText constructs a checked spelling. Legacy observation replay is
// deliberately available only to private record and protocol readers.
func (n *Seen) UnmarshalText(raw []byte) error {
	if len(raw) == 0 {
		*n = Seen{}
		return nil
	}
	var checked HerdrName
	if err := checked.UnmarshalText(raw); err != nil {
		return err
	}
	*n = checked.Observation()
	return nil
}

// UnmarshalText restores checked team names only. Private readers retain
// legacy observations with Observe and TeamFromObservation.
func (n *TeamRef) UnmarshalText(raw []byte) error {
	if len(raw) == 0 {
		*n = TeamRef{}
		return nil
	}
	checked, err := ParseTeamName(string(raw))
	if err != nil {
		return err
	}
	*n = checked.Ref()
	return nil
}
