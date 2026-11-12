package cli

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"
)

type jsonApplyList struct {
	Herdr jsonReadNote    `json:"herdr"`
	Count int             `json:"count"`
	Teams []jsonApplyTeam `json:"teams"`
}

type jsonApplyTeam struct {
	Team        string              `json:"team"`
	Definition  string              `json:"definition"`
	Project     domainplace.Project `json:"project"`
	ProjectName string              `json:"project_name"`
	// Error is why the definition cannot be built, in the words hergy status
	// uses. Such a team has no member to apply.
	Error   string            `json:"error,omitempty"`
	Notes   []string          `json:"notes"`
	Members []jsonApplyMember `json:"members"`
}

type jsonApplyMember struct {
	Name             string       `json:"name"`
	Short            string       `json:"short"`
	Role             string       `json:"role"`
	State            string       `json:"state"`
	How              string       `json:"how"`
	Changes          []ops.Change `json:"changes"`
	CanApply         bool         `json:"can_apply"`
	WhyNot           string       `json:"why_not,omitempty"`
	ConversationLost bool         `json:"conversation_lost"`
}
type jsonReadNote struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}
