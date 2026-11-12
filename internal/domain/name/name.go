// Package name owns the shape and composition of hergy's names. It depends only
// on the standard library; callers translate Problem into their own sentences.
package name

import (
	"fmt"
	"strings"
)

// Kind identifies which naming rule refused a value.
type Kind string

// The kinds of name whose validation can fail.
const (
	Team        Kind = "team"
	Definition  Kind = "definition"
	Member      Kind = "member"
	Worker      Kind = "worker"
	Role        Kind = "role"
	Profile     Kind = "profile"
	PurposeKind Kind = "purpose"
	Herdr       Kind = "herdr"
)

// Rule identifies a purpose error without embedding a translated sentence.
type Rule string

// The reasons a purpose can be refused.
const (
	EmptyPurpose     Rule = "empty-purpose"
	MultiplePurposes Rule = "multiple-purposes"
)

// Problem reports a refused value without choosing a language or a message.
type Problem struct {
	Kind  Kind
	Value string
	Max   int
	Rule  Rule
}

func (p *Problem) Error() string { return fmt.Sprintf("invalid %s name %q", p.Kind, p.Value) }

func plain(s string, limit int, kind Kind) error {
	if len(s) == 0 || len(s) > limit || s[0] < 'a' || s[0] > 'z' {
		return &Problem{Kind: kind, Value: s, Max: limit}
	}
	for _, c := range s[1:] {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return &Problem{Kind: kind, Value: s, Max: limit}
		}
	}
	return nil
}

// TeamName is a validated team name. Its zero value denotes no name.
type TeamName struct{ s string }

// NewTeamName checks a newly assigned name (at most 25 characters).
func NewTeamName(s string) (TeamName, error) {
	if err := plain(s, 25, Team); err != nil {
		return TeamName{}, err
	}
	return TeamName{s: s}, nil
}

// ParseTeamName reads an existing name (at most 32 characters).
func ParseTeamName(s string) (TeamName, error) {
	if err := plain(s, 32, Team); err != nil {
		return TeamName{}, err
	}
	return TeamName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n TeamName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n TeamName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *TeamName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = TeamName{}
		return nil
	}
	parsed, err := ParseTeamName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// DefinitionName is a validated definition name. Its zero value denotes no name.
type DefinitionName struct{ s string }

// NewDefinitionName checks a newly assigned name (at most 25 characters).
func NewDefinitionName(s string) (DefinitionName, error) {
	if err := plain(s, 25, Definition); err != nil {
		return DefinitionName{}, err
	}
	return DefinitionName{s: s}, nil
}

// ParseDefinitionName reads an existing name (at most 32 characters).
func ParseDefinitionName(s string) (DefinitionName, error) {
	if err := plain(s, 32, Definition); err != nil {
		return DefinitionName{}, err
	}
	return DefinitionName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n DefinitionName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n DefinitionName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *DefinitionName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = DefinitionName{}
		return nil
	}
	parsed, err := ParseDefinitionName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// MemberName is a validated member name. Its zero value denotes no name.
type MemberName struct{ s string }

// ParseMemberName reads an existing name (at most 32 characters).
func ParseMemberName(s string) (MemberName, error) {
	if err := plain(s, 32, Member); err != nil {
		return MemberName{}, err
	}
	return MemberName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n MemberName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n MemberName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *MemberName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = MemberName{}
		return nil
	}
	parsed, err := ParseMemberName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// WorkerName is a validated worker name. Its zero value denotes no name.
type WorkerName struct{ s string }

// ParseWorkerName reads an existing name (at most 32 characters).
func ParseWorkerName(s string) (WorkerName, error) {
	if err := plain(s, 32, Worker); err != nil {
		return WorkerName{}, err
	}
	return WorkerName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n WorkerName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n WorkerName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *WorkerName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = WorkerName{}
		return nil
	}
	parsed, err := ParseWorkerName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// RoleName is a validated role name. Its zero value denotes no name.
type RoleName struct{ s string }

// ParseRoleName reads an existing name (at most 32 characters).
func ParseRoleName(s string) (RoleName, error) {
	if err := plain(s, 32, Role); err != nil {
		return RoleName{}, err
	}
	return RoleName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n RoleName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n RoleName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *RoleName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = RoleName{}
		return nil
	}
	parsed, err := ParseRoleName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// ProfileName is a validated profile name. Its zero value denotes no name.
type ProfileName struct{ s string }

// ParseProfileName reads an existing name (at most 32 characters).
func ParseProfileName(s string) (ProfileName, error) {
	if err := plain(s, 32, Profile); err != nil {
		return ProfileName{}, err
	}
	return ProfileName{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n ProfileName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n ProfileName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *ProfileName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = ProfileName{}
		return nil
	}
	parsed, err := ParseProfileName(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// Purpose is a validated purposekind name. Its zero value denotes no name.
type Purpose struct{ s string }

// ParsePurpose reads an existing name.
func ParsePurpose(s string) (Purpose, error) {
	if err := purposeProblem(s); err != nil {
		return Purpose{}, err
	}
	return Purpose{s: s}, nil
}

// String returns the unchanged stored spelling.
func (n Purpose) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n Purpose) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *Purpose) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = Purpose{}
		return nil
	}
	parsed, err := ParsePurpose(s)
	if err != nil {
		return err
	}
	*n = parsed
	return nil
}

// HerdrName is a validated herdr name. Its zero value denotes no name.
type HerdrName struct{ s string }

// String returns the unchanged stored spelling.
func (n HerdrName) String() string { return n.s }

// MarshalText preserves the string representation in saved records and JSON.
func (n HerdrName) MarshalText() ([]byte, error) { return []byte(n.s), nil }

// UnmarshalText reads the stored spelling. An empty stored value denotes absence.
func (n *HerdrName) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" {
		*n = HerdrName{}
		return nil
	}
	if err := plain(s, 32, Herdr); err != nil {
		return err
	}
	*n = HerdrName{s: s}
	return nil
}

// A purpose keeps its spelling, including spaces. Commas mean more than one
// purpose (d-37ac, n-b7de); the definition reader has always refused those.
func purposeProblem(s string) error {
	if strings.TrimSpace(s) == "" {
		return &Problem{Kind: PurposeKind, Value: s, Rule: EmptyPurpose}
	}
	if strings.ContainsAny(s, ",、") {
		return &Problem{Kind: PurposeKind, Value: s, Rule: MultiplePurposes}
	}
	return nil
}

// Member constructs the herdr name of a member, checking the composed length.
func (n TeamName) Member(member MemberName) (HerdrName, error) {
	return n.compose(member.s)
}

// Worker constructs the herdr name of a hired worker.
func (n TeamName) Worker(worker WorkerName) (HerdrName, error) {
	return n.compose(worker.s)
}

// Watcher constructs the name of the team's turn watcher.
func (n TeamName) Watcher() (HerdrName, error) { return n.compose("turns") }

func (n TeamName) compose(short string) (HerdrName, error) {
	s := n.s + "-" + short
	if err := plain(s, 32, Herdr); err != nil {
		return HerdrName{}, err
	}
	if n.s == "" || short == "" {
		return HerdrName{}, &Problem{Kind: Herdr, Value: s, Max: 32}
	}
	return HerdrName{s: s}, nil
}

// WorkerTab labels the separate tab of one hired worker.
func (n TeamName) WorkerTab(worker WorkerName) string { return "worker:" + n.s + "/" + worker.s }

// ParseWorkerTab reads a complete worker tab label, rather than a prefix.
func ParseWorkerTab(label string) (TeamName, WorkerName, bool) {
	rest, ok := strings.CutPrefix(label, "worker:")
	if !ok {
		return TeamName{}, WorkerName{}, false
	}
	team, worker, ok := strings.Cut(rest, "/")
	if !ok {
		return TeamName{}, WorkerName{}, false
	}
	t, err := ParseTeamName(team)
	if err != nil {
		return TeamName{}, WorkerName{}, false
	}
	w, err := ParseWorkerName(worker)
	if err != nil {
		return TeamName{}, WorkerName{}, false
	}
	return t, w, true
}

// ResolveHerdr resolves an observed herdr name against the running teams. The
// longest matching team wins: a-b-lead belongs to a-b rather than a. Both
// components may contain dashes, so there is no context-free inverse. This reads
// observations without constructing a validated HerdrName. If no team matches,
// the second result keeps raw unchanged for callers that display observations.
func ResolveHerdr(raw string, teams []TeamName) (TeamName, string, bool) {
	var best TeamName
	for _, team := range teams {
		if team.s != "" && len(team.s) > len(best.s) && strings.HasPrefix(raw, team.s+"-") {
			best = team
		}
	}
	if best.s == "" {
		return TeamName{}, raw, false
	}
	return best, raw[len(best.s)+1:], true
}
