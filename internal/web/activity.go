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

// ActivityItem is one running team, one row of the list.
type ActivityItem struct {
	Name     string           `json:"name"`
	Summary  ActivitySummary  `json:"summary"`
	Problems ActivityProblems `json:"problems"`
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

// ActivitySummary is the row's columns: which definition it runs from, what it
// is doing, who the row is about, how long it has been that way, and who is
// working.
type ActivitySummary struct {
	// Definition is the team definition the team was booted from, which the row
	// writes under the team's name when the two differ (2.2).
	Definition string `json:"definition"`
	// State is answer, turn, stalled or working (activity.State).
	State string `json:"state"`
	// Last is the short name of the member the row is about: the one waiting for
	// the human, or the one that stopped last. Empty for a working team and for
	// one with no live agent.
	Last string `json:"last"`
	// Since is how long the row has been in this state, and when it began. It is
	// left out for a working team and for one with no live agent (2.2).
	Since *ActivitySince `json:"since,omitempty"`
	// Working is who is working, for a working team only.
	Working *ActivityWork `json:"working,omitempty"`
}

// ActivitySince is a length and the moment it began, in the server's own words
// (6 A4).
type ActivitySince struct {
	Len   string `json:"len"`
	At    string `json:"at"`
	Lower bool   `json:"lower"`
}

// ActivityWork is a working team's members and how many of its workers are
// working.
type ActivityWork struct {
	Members []string `json:"members"`
	Workers int      `json:"workers"`
}

// ActivityProblems is the row's counts. The activity's reading finds no problem
// of its own to count — a definition it cannot read is the row's error, not a
// problem here — so both are zero (spec-activity 3).
type ActivityProblems struct {
	Err  int `json:"err"`
	Warn int `json:"warn"`
}

// ActivityNote is one reader that could not be read: whether it could, and the
// sentence a person reads when it could not.
type ActivityNote struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}

// ActivityDoc is GET /api/activity/{team}'s doc: one team, opened.
type ActivityDoc struct {
	Name        string        `json:"name"`
	Definition  string        `json:"definition"`
	Project     string        `json:"project"`
	ProjectName string        `json:"project_name"`
	Workspace   ActivitySpace `json:"workspace"`
	State       string        `json:"state"`
	Last        string        `json:"last"`
	// Herdr is the row's main member as herdr knows it ("core-lead"), which is
	// the heading of the left column. Empty for a working team.
	Herdr string `json:"herdr"`
	// Role is that member's role from the team definition (2.4).
	Role  string         `json:"role"`
	Since *ActivitySince `json:"since,omitempty"`
	// Nodes are the definition's members with their live state, the one waiting
	// for the human first and, within one state, the one that stopped last
	// first (2.5, 6 A3).
	Nodes []ActivityNode `json:"nodes"`
	// Hired are the team's workers, in the same order (X1, 6 A3).
	Hired []ActivityNode `json:"hired"`
	// Questions are the open questions the row's main member asked, newest
	// first. All of them: the screen shows one and the rest in one line each
	// (2.4).
	Questions []ActivityQuestion `json:"questions"`
	Tail      ActivityTail       `json:"tail"`
	// Gy says whether the ledger could be read, and why not when it could not
	// (2.4).
	Gy ActivityNote `json:"gy"`
	// Error is why this team's definition cannot be read, for the row and for
	// the opened part (2.2).
	Error string `json:"error"`
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
	// Stale says hergy last handed this member a different prompt than its
	// current definition builds (r-a8fa). Workers are never stale.
	Stale bool `json:"stale,omitempty"`
}

// ActivityQuestion is one open question the row's main member asked.
type ActivityQuestion struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Ago     string   `json:"ago"`
	Options []string `json:"options"`
}

// ActivityTail is the last lines of the row's main member's pane, as herdr wrote
// them (2.4, 6 A6). Lines is empty when the pane could not be read.
type ActivityTail struct {
	Lines []string `json:"lines"`
	// Shown is how many of them the screen shows before 上に N 行.
	Shown int `json:"shown"`
}
