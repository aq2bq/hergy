package ops

import (
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/team"
)

type runningTeam = RunningTeam
type projectView = ProjectView
type launchRecords = LaunchRecords

func sessionOf(read herdrNow) Session {
	return Session{Tabs: read.tabs, Agents: read.agents, AgentsRead: read.agentsRead, Workspaces: read.spaces, Seen: read.seen, Err: read.err}
}

func readRunningTeamsIn(views []projectView, read herdrNow, lenient bool) []runningTeam {
	return ReadRunningTeamsIn(views, sessionOf(read), lenient)
}
func projectOf(one team.Running, records launchRecords, views []projectView, tabs []team.Tab, lenient bool) (projectView, launchlog.Record, bool) {
	return ProjectOf(one, records, views, tabs, lenient)
}

func latestLaunches() launchRecords                                  { return LatestLaunches() }
func teamTab(tabs []team.Tab, name domainname.Seen) (team.Tab, bool) { return TeamTab(tabs, name) }
func workspaceLabel(spaces []herdr.Workspace, id string) string {
	return WorkspaceLabel(spaces, id)
}
func judgeOf(one runningTeam, read herdrNow, now time.Time) activity.Team {
	return JudgeOf(one, sessionOf(read), now)
}
func agentsOf(read herdrNow, name domainname.Seen) []activity.Agent {
	return AgentsOf(sessionOf(read), name)
}

func nameProjects(views []projectView) []projectView { return NameProjects(views) }
