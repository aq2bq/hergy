package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

type wireApplyList struct {
	Herdr wireReadNote    `json:"herdr"`
	Count int             `json:"count"`
	Teams []wireApplyTeam `json:"teams"`
}

type wireApplyTeam struct {
	Team        string              `json:"team"`
	Definition  string              `json:"definition"`
	Project     domainplace.Project `json:"project"`
	ProjectName string              `json:"project_name"`
	// Error is why the definition cannot be built, in the words hergy status
	// uses. Such a team has no member to apply.
	Error   string            `json:"error,omitempty"`
	Notes   []string          `json:"notes"`
	Members []wireApplyMember `json:"members"`
}

type wireApplyMember struct {
	Name             string   `json:"name"`
	Short            string   `json:"short"`
	Role             string   `json:"role"`
	State            string   `json:"state"`
	How              ApplyHow `json:"how"`
	Changes          []Change `json:"changes"`
	CanApply         bool     `json:"can_apply"`
	WhyNot           string   `json:"why_not,omitempty"`
	ConversationLost bool     `json:"conversation_lost"`
}
type wireReadNote struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}
