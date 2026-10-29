package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/deliver"
)

// This file is GET /api/apply: the drift between a saved definition and the
// teams running from it, member by member, and the way hergy would put each
// member right (d-36ef, d-3d30). A person reads it, ticks the members to apply,
// and does not decide what kind of operation a change needs.

// The ways hergy can put a member right. They are the wire's words, so a screen
// writes them and never guesses one from what changed.
const (
	// ApplyResend hands the current prompt again; the conversation stays.
	ApplyResend = "resend"
	// ApplyStart starts a member the definition has and no agent is live for.
	ApplyStart = "start"
	// ApplyRestart ends the agent and starts it again; the conversation goes.
	ApplyRestart = "restart"
	// ApplyCannot says hergy cannot apply this drift: why_not says why.
	ApplyCannot = "cannot"
)

// Applier is the reading behind the band's count and the sheet that applies a
// saved definition to the teams running from it. The app's activity reading
// implements it; a server built without one answers nothing to apply.
type Applier interface {
	// Apply is the drift of every running team, by member, and the count the
	// band shows.
	Apply() ApplyList
	// ApplyCount is the band's number alone.
	ApplyCount() int
	// ApplyMembers applies the drift to the named members, by herdr name, and
	// answers the start whose progress the sheet reads.
	ApplyMembers(members []string) (HQLaunch, error)
}

// ApplyIn is POST /api/apply's body: the members to apply, by herdr name.
type ApplyIn struct {
	Members []string `json:"members"`
}

// ApplyList is GET /api/apply: the running teams that differ from their saved
// definition, and the band's count. Only teams and members with drift are here.
type ApplyList struct {
	Herdr ActivityNote `json:"herdr"`
	Count int          `json:"count"`
	Teams []ApplyTeam  `json:"teams"`
}

// ApplyTeam is one running team's drift, and the notes that are the team's
// rather than a member's (a worker profile change takes effect from the next
// worker, so it is not applied to anyone).
type ApplyTeam struct {
	Team        string `json:"team"`
	Definition  string `json:"definition"`
	Project     string `json:"project"`
	ProjectName string `json:"project_name"`
	// Error is why the definition cannot be built, in the words hergy status
	// uses. Such a team has no member to apply.
	Error   string        `json:"error,omitempty"`
	Notes   []string      `json:"notes"`
	Members []ApplyMember `json:"members"`
}

// ApplyMember is one member of a running team that differs from its saved
// definition, and how hergy would put it right. State is the activity's own word
// (idle, handed, active, blocked, absent). CanApply says hergy can apply it now;
// WhyNot says why it cannot when it cannot. ConversationLost says applying this
// member ends the conversation it holds.
type ApplyMember struct {
	Name             string           `json:"name"`
	Short            string           `json:"short"`
	Role             string           `json:"role"`
	State            string           `json:"state"`
	How              string           `json:"how"`
	Changes          []deliver.Change `json:"changes"`
	CanApply         bool             `json:"can_apply"`
	WhyNot           string           `json:"why_not,omitempty"`
	ConversationLost bool             `json:"conversation_lost"`
}

// apiApply is POST /api/apply: the drift is applied to the selected members. The
// work goes on after the answer, and its progress and startup question are read
// from GET /api/hq/launches/{id}, the way a start's are.
func (s *Server) apiApply(w http.ResponseWriter, r *http.Request) {
	if s.apply == nil {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	if !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[ApplyIn](w, r)
	if !ok {
		return
	}
	launch, err := s.apply.ApplyMembers(in.Members)
	if err != nil {
		answerHQ(w, nil, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"launch": launch})
}

// emptyApply is what a server with no applier answers: nothing to apply, and a
// herdr that could be read.
func emptyApply() ApplyList {
	return ApplyList{Herdr: ActivityNote{OK: true}, Teams: []ApplyTeam{}}
}

// apiApplyList answers GET /api/apply.
func (s *Server) apiApplyList(w http.ResponseWriter, _ *http.Request) {
	answer := emptyApply()
	if s.apply != nil {
		answer = s.apply.Apply()
	}
	writeJSON(w, http.StatusOK, answer)
}

// apiApplyCount answers GET /api/apply/count: the band's light number.
func (s *Server) apiApplyCount(w http.ResponseWriter, _ *http.Request) {
	count := 0
	if s.apply != nil {
		count = s.apply.ApplyCount()
	}
	writeJSON(w, http.StatusOK, struct {
		Count int `json:"count"`
	}{Count: count})
}
