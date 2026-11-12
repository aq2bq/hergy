package ops

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// HQError is a refusal from the headquarters reading: the status, the sentence a
// person reads, and, for a start that has to say what else could be done, the
// name hergy suggests and the thing that is in the way.
type HQError struct {
	Status        int
	Text          string
	At            string
	SuggestedName string
}

// HQProjects is GET /api/hq/projects: the current project, the listed ones, the
// launch record's candidates, and the global layer's counts.
type HQProjects struct {
	// RemovedAt is where a delete took the project out (d-2cce). It is a
	// pointer so that the first position (0) is written in the answer: a plain
	// int with omitempty would drop it and the screen could not put the project
	// back at the head of the list.
	RemovedAt *int
	Config    string
	// Mode is "listed" (config.yaml names projects:) or "current" (it does not,
	// and serve shows the project it was started in, as it always did).
	Mode     string
	Current  HQProject
	Projects []HQProject
	Global   HQGlobal
}

// HQProject is one project the headquarters knows: where it is, what it is
// called, and what it holds. Path is the identity (d-73fc); Name is what a person
// reads, with the parent added only when two names would look alike.
type HQProject struct {
	Path          domainplace.Project
	Name          string
	Display       string
	Listed        bool
	Current       bool
	Exists        bool
	HasHergy      bool
	Defs          int
	Roles         int
	Profiles      int
	Running       int
	LastLaunch    string
	LastWorkspace *HQWorkspace
	// Source says why a project is here without being listed: "launch" is the
	// machine's launch record, "current" is the project serve was started in.
	Source string
}

// HQGlobal is the global layer's own counts: shown once, after every project.
type HQGlobal struct {
	Path     string
	Display  string
	Defs     int
	Roles    int
	Profiles int
}

// HQRef is one project as a group names it, and as the workspace and launch
// answers name the project a team belongs to.
type HQRef struct {
	Path    domainplace.Project
	Name    string
	Display string
}

// HQRunning is GET /api/hq/running: the running teams of the listed projects,
// read from the same activity reading the activity screen uses.
type HQRunning struct {
	Herdr ActivityNote
	Teams []HQRunningTeam
}

// HQRunningTeam is one running team with the project and workspace it runs in.
type HQRunningTeam struct {
	Team        domainname.TeamRef
	Definition  string
	Place       string
	Project     domainplace.Project
	ProjectName string
	Workspace   HQWorkspace
	Tab         HQTab
	Starting    bool
	Launch      string
	Members     []HQMember
	Workers     []HQWorker
	Waiting     []string
	// NotUp is the members the definition has that no agent of the team is live
	// for, by short name: the ones a screen offers 起動 for (r-5d1a).
	NotUp []string
	// Error is why the definition cannot be built, in the words hergy status uses
	// (2.2). A definition that does not build has no member to offer 起動 for, so
	// a screen shows this sentence instead of leaving the column silent (r-fc16).
	Error string
}

// HQWorkspace is one herdr workspace: its id, its label, and the team that is in
// it (one team to a workspace, d-73fc).
type HQWorkspace struct {
	ID          string
	Label       string
	Focused     bool
	Team        string
	TeamProject domainplace.Project
	LastTeam    string
}

// HQTab is one team's tab.
type HQTab struct {
	ID    string
	Label string
}

// HQMember is one member of a running team: its short name, the name herdr knows
// it by, whether an agent is live there, and its state.
type HQMember struct {
	Name  string
	Herdr string
	Live  bool
	State string
}

// HQWorker is one worker tab.
type HQWorker struct {
	Name string
	Tab  string
	Live bool
}

// HQWorkspaces is GET /api/hq/workspaces.
type HQWorkspaces struct {
	Workspaces []HQWorkspace
}

// HQLaunchOptions is GET /api/hq/launch/options: everything the start sheet
// draws, with the workspace order of spec 5 章 and the name hygy suggests when
// the definition's name is taken.
type HQLaunchOptions struct {
	Definition       string
	Description      string
	Place            string
	Project          *HQRef
	Members          []HQMember
	Projects         []HQLaunchProject
	Clash            []HQClash
	SuggestedName    string
	LastWorkspace    *HQWorkspace
	NewWorkspaceName string
	Workspaces       []HQWorkspaceChoice
}

// HQLaunchProject is one project a global definition can be started in. A
// project that defines the same name itself cannot be chosen (spec 5 章).
type HQLaunchProject struct {
	Path      domainplace.Project
	Name      string
	Available bool
	Reason    string
}

// HQClash is a running team already using the definition's name.
type HQClash struct {
	Team    string
	Project domainplace.Project
}

// HQWorkspaceChoice is one workspace in the start sheet: why it is where it is in
// the order, and whether it can be chosen.
type HQWorkspaceChoice struct {
	HQWorkspace
	Selectable bool
	Why        string
}

// HQLaunchIn is POST /api/hq/launch's body.
type HQLaunchIn struct {
	Project    domainplace.Input
	Definition string
	Place      domainplace.Selection
	Name       string
	Workspace  HQWorkspaceIn
}

// HQWorkspaceIn is the workspace a start asks for: one that is there, or one to
// make with this label.
type HQWorkspaceIn struct {
	ID  string
	New string
}

// HQLaunch is one start, as it is now.
type HQLaunch struct {
	ID          string
	Action      string
	Team        string
	Definition  string
	Place       string
	Project     domainplace.Project
	ProjectName string
	Workspace   HQWorkspace
	Tab         HQTab
	Phase       string
	Steps       []HQLaunchStep
	Waiting     *HQWaiting
	Error       string
	// Members are the members a refresh starts, by short name: empty for a start,
	// whose members the start sheet reads from the definition.
	Members []string
}

// HQLaunchStep is one line of the start's progress.
type HQLaunchStep struct {
	At   string
	Kind string
	Text string
}

// HQWaiting is where a person answers the question an agent asks while it starts
// (d-d589).
type HQWaiting struct {
	Member    string
	Herdr     string
	Workspace string
	Tab       string
	Pane      string
	Command   string
}

// HQStopped is POST /api/hq/stop's answer.
type HQStopped struct {
	Team       string
	Definition string
	Project    domainplace.Project
	Workspace  HQWorkspace
	Closed     HQClosed
	StoppedAt  string
	// Leftovers is what the closed panes left in git, for the stop sheet's result
	// (r-b65e).
	Leftovers HQLeftovers
}

// HQLeftovers is what a stopped team left in git: the worktrees that came while
// it ran (other than the project's own root), and the project's stash count.
type HQLeftovers struct {
	Worktrees []HQWorktree
	Stashes   int
}

// HQWorktree is one worktree of the stopped team's project that came while the
// team ran, with what removing it would lose.
type HQWorktree struct {
	Path   domainplace.Project
	Branch string
	Dirty  bool
	Ahead  int
	Base   string
	// PaneCwd says a closed pane's working directory was this worktree (d-970c).
	PaneCwd bool
}

// HQClosed is how many members and workers a stop put away.
type HQClosed struct {
	Members int
	Workers int
}

// ActivityList is GET /api/activity's answer.
type ActivityList struct {
	Items   []ActivityItem
	Running int
	Waiting int
	Herdr   ActivityNote
}

// ActivityItem is one running team tile in the list.
type ActivityItem struct {
	Name    string
	Summary ActivitySummary
	// Error is why this team's definition cannot be read, as hergy status
	// writes it (2.2). Empty when it reads.
	Error string
	// Place is where the team's launch definition was read: project or global.
	Place string
	// Project is the listed project the team belongs to, and ProjectName is
	// what a person reads beside it (d-73fc). Workspace is the herdr workspace
	// its tab stands in.
	Project     domainplace.Project
	ProjectName string
	Workspace   ActivitySpace
}

// ActivitySpace is one herdr workspace as the activity names it.
type ActivitySpace struct {
	ID    string
	Label string
}

// ActivitySummary is the tile's definition, state and how long that state has
// lasted.
type ActivitySummary struct {
	// Definition is the team definition the team was booted from, which the row
	// writes under the team's name when the two differ (2.2).
	Definition string
	// State is answer, turn, stalled or working (activity.State).
	State string
	// Since is how long the tile has been in this state, and when it began. It is
	// left out for a working team and for one with no live agent (2.2).
	Since *ActivitySince
	// Turn is who holds the turn, as hergy's watcher keeps it (r-bc26). It is
	// left out for a team the watcher has not seen.
	Turn *ActivityTurn
}

// ActivityTurn is the turn's holder as the tile writes it: a member's or
// worker's short name or "human", who handed it to them, and, when hergy handed
// it to the human, why (silent, undelivered, no-edge; empty along an edge).
type ActivityTurn struct {
	Holder string
	From   string
	Why    string
	// Text is the line the tile prints, in the server's words.
	Text string
}

// ActivitySince is a length and the moment it began, in the server's own words
// (6 A4).
type ActivitySince struct {
	Len   string
	At    string
	Lower bool
}

// ActivityNote is one reader that could not be read: whether it could, and the
// sentence a person reads when it could not.
type ActivityNote struct {
	OK   bool
	Text string
}

// ActivityDoc is GET /api/activity/{team}'s members and workers for its tile.
type ActivityDoc struct {
	// Nodes are the definition's members with their live state, the one waiting
	// for the human first and, within one state, the one that stopped last
	// first.
	Nodes []ActivityNode
	// Hired are the team's workers, in the same order.
	Hired []ActivityNode
	// Question is the confirmation line from the main member's pane the waiting row shows.
	// Empty when the member has no pane or no readable text.
	Question string
}

// ActivityNode is one member or one worker of the right-hand column: the name,
// the agent, its state, and the moment that state began (2.5).
type ActivityNode struct {
	// Question is this member’s question or recognized approval block, not terminal history.
	Question string
	// Name is the member's short name, and a worker's name with the team's own
	// prefix taken off ("deps" for the agent "core-deps", X1).
	Name string
	// Herdr is the agent's name as herdr knows it, for the row's own mark of
	// the member the line is about.
	Herdr domainname.Seen
	// State is blocked, handed, active, idle or absent (activity.NodeState).
	State string
	// At is when the state began, as HH:MM, and empty for a working member and
	// for one with no agent (2.5).
	At string
	// SinceAt is the same moment as the row's いつから reads it: "15:31 から",
	// or "15:31 より前" when the watching began after it (d-b4b9 audit 2). Empty
	// when At is.
	SinceAt string
	// Doing is what the agent's terminal says it is doing, exactly as herdr
	// strips its own marks from the title (d-b4b9). It is shown whatever the
	// state, and left out when the title is empty.
	Doing string
	// From is the short name of the member that hired this worker, read from the
	// hire line of the launch record (d-b4b9). Empty for a worker hired before
	// that line was written, and always empty for a member.
	From string
}

// PromptRun identifies a live instance and its execution project.
type PromptRun struct {
	Team    domainname.TeamRef
	Project domainplace.Project
}

// RecordedPrompt is the exact blob body, without a sending preamble.
type RecordedPrompt struct {
	Text    string
	Command string
	Time    string
}

// PromptHistory distinguishes absent, unreadable and recorded histories.
type PromptHistory struct {
	Runs     []PromptRun
	Selected *PromptRun
	Status   string
	Reason   string
	Recorded *RecordedPrompt
}

func (e *HQError) Error() string { return e.Text }

// Actions identify the operation whose progress is being read.
const (
	ActionLaunch  = "launch"
	ActionRefresh = "refresh"
	ActionApply   = "apply"
)
