package ops

import (
	"github.com/aq2bq/hergy/internal/deliver"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// ApplyHow is the closed set of operations selected by definition drift.
type ApplyHow string

// Apply operations selected by drift.
const (
	ApplyResend  ApplyHow = "resend"
	ApplyStart   ApplyHow = "start"
	ApplyRestart ApplyHow = "restart"
	ApplyCannot  ApplyHow = "cannot"
)

// ReadNote records whether a read is available; a missing read is not empty state.
type ReadNote struct {
	OK   bool
	Text string
}

// ApplyList is the operation's read model, projected into DTOs by adapters.
type ApplyList struct {
	Herdr ReadNote
	Count int
	Teams []ApplyTeam
}

// ApplyTeam is one running team's changes and team-wide notes.
type ApplyTeam struct {
	Team, Definition   string
	Project            domainplace.Project
	ProjectName, Error string
	Notes              []string
	Members            []ApplyMember
}

// ApplyMember is one changed member and the selected operation.
type ApplyMember struct {
	Name, Short, Role, State string
	How                      ApplyHow
	Changes                  []deliver.Change
	CanApply                 bool
	WhyNot                   string
	ConversationLost         bool
}
