package web

// The activity screen's answer (spec-activity 3). The screen draws every row,
// every sentence in it and every count from these shapes, and it writes no time
// and no length of its own: the reading behind them is internal/app's, which is
// the only layer that touches herdr and gy (6 A1, V1).

// Activity is how the screen reads the teams running in this project. The
// reading is handed in, so the screen and its tests do not start herdr: hergy
// serve builds one in internal/app and hands it to New.
type Activity interface {
	// List is GET /api/activity: the running teams, in the order of
	// activity.Sort, and the band's two numbers.
	List() ActivityList
	// One is GET /api/activity/{team}: one team's opened part. ok is false for
	// a team that is not running, which the handler answers as a 404.
	One(name string) (ActivityDoc, bool)
}

// noActivity is the reading of a server built without one: nothing is running
// and nothing is wrong, so the screen says there is no team running (2.7). It is
// what a test that is not about the activity gets, and it is the same answer as
// a herdr that reports no team.
type noActivity struct{}

func (noActivity) List() ActivityList {
	return ActivityList{Items: []ActivityItem{}, Herdr: ActivityNote{OK: true}}
}

func (noActivity) One(string) (ActivityDoc, bool) { return ActivityDoc{}, false }

// ActivityList is GET /api/activity's answer.
type ActivityList struct {
	Items   []ActivityItem `json:"items"`
	Running int            `json:"running"`
	Waiting int            `json:"waiting"`
	Herdr   ActivityNote   `json:"herdr"`
}

// ActivityItem is one running team tile in the list.
type ActivityItem struct {
	Name    string          `json:"name"`
	Summary ActivitySummary `json:"summary"`
	// Error is why this team's definition cannot be read, as hergy status
	// writes it (2.2). Empty when it reads.
	Error string `json:"error"`
	// Project is the listed project the team belongs to, and ProjectName is
	// what a person reads beside it (d-73fc). Workspace is the herdr workspace
	// its tab stands in.
	Project     string        `json:"project"`
	ProjectName string        `json:"project_name"`
	Workspace   ActivitySpace `json:"workspace"`
}

// ActivitySpace is one herdr workspace as the activity names it.
type ActivitySpace struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ActivitySummary is the tile's definition, state and how long that state has
// lasted.
type ActivitySummary struct {
	// Definition is the team definition the team was booted from, which the row
	// writes under the team's name when the two differ (2.2).
	Definition string `json:"definition"`
	// State is answer, turn, stalled or working (activity.State).
	State string `json:"state"`
	// Since is how long the tile has been in this state, and when it began. It is
	// left out for a working team and for one with no live agent (2.2).
	Since *ActivitySince `json:"since,omitempty"`
}

// ActivitySince is a length and the moment it began, in the server's own words
// (6 A4).
type ActivitySince struct {
	Len   string `json:"len"`
	At    string `json:"at"`
	Lower bool   `json:"lower"`
}

// ActivityNote is one reader that could not be read: whether it could, and the
// sentence a person reads when it could not.
type ActivityNote struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}

// ActivityDoc is GET /api/activity/{team}'s members and workers for its tile.
type ActivityDoc struct {
	// Nodes are the definition's members with their live state, the one waiting
	// for the human first and, within one state, the one that stopped last
	// first.
	Nodes []ActivityNode `json:"nodes"`
	// Hired are the team's workers, in the same order.
	Hired []ActivityNode `json:"hired"`
	// Question is the confirmation line from the main member's pane the waiting row shows.
	// Empty when the member has no pane or no readable text.
	Question string `json:"question"`
}

// ActivityNode is one member or one worker of the right-hand column: the name,
// the agent, its state, and the moment that state began (2.5).
type ActivityNode struct {
	// Name is the member's short name, and a worker's name with the team's own
	// prefix taken off ("deps" for the agent "core-deps", X1).
	Name string `json:"name"`
	// Herdr is the agent's name as herdr knows it, for the row's own mark of
	// the member the line is about.
	Herdr string `json:"herdr"`
	// State is blocked, handed, active, idle or absent (activity.NodeState).
	State string `json:"state"`
	// At is when the state began, as HH:MM, and empty for a working member and
	// for one with no agent (2.5).
	At string `json:"at"`
	// SinceAt is the same moment as the row's いつから reads it: "15:31 から",
	// or "15:31 より前" when the watching began after it (d-b4b9 audit 2). Empty
	// when At is.
	SinceAt string `json:"since_at,omitempty"`
	// Doing is what the agent's terminal says it is doing, exactly as herdr
	// strips its own marks from the title (d-b4b9). It is shown whatever the
	// state, and left out when the title is empty.
	Doing string `json:"doing,omitempty"`
	// From is the short name of the member that hired this worker, read from the
	// hire line of the launch record (d-b4b9). Empty for a worker hired before
	// that line was written, and always empty for a member.
	From string `json:"from,omitempty"`
}
