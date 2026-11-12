package app

import (
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/team"
)

type runningTeam = ops.RunningTeam
type projectView = ops.ProjectView
type launchRecords = ops.LaunchRecords

func sessionOf(read herdrNow) ops.Session {
	return ops.Session{Tabs: read.tabs, Agents: read.agents, AgentsRead: read.agentsRead, Workspaces: read.spaces, Seen: read.seen, Err: read.err}
}

func readRunningTeamsIn(views []projectView, read herdrNow, lenient bool) []runningTeam {
	return ops.ReadRunningTeamsIn(views, sessionOf(read), lenient)
}
func projectOf(one team.Running, records launchRecords, views []projectView, tabs []team.Tab, lenient bool) (projectView, launchlog.Record, bool) {
	return ops.ProjectOf(one, records, views, tabs, lenient)
}

func latestLaunches() launchRecords                                  { return ops.LatestLaunches() }
func teamTab(tabs []team.Tab, name domainname.Seen) (team.Tab, bool) { return ops.TeamTab(tabs, name) }
func workspaceLabel(spaces []herdr.Workspace, id string) string {
	return ops.WorkspaceLabel(spaces, id)
}
func judgeOf(one runningTeam, read herdrNow, now time.Time) activity.Team {
	return ops.JudgeOf(one, sessionOf(read), now)
}
func agentsOf(read herdrNow, name domainname.Seen) []activity.Agent {
	return ops.AgentsOf(sessionOf(read), name)
}
func teamAgents(tabs []team.Tab, agents []protocol.Agent, name domainname.Seen) []protocol.Agent {
	return ops.TeamAgents(tabs, agents, name)
}

func nameProjects(views []projectView) []projectView { return ops.NameProjects(views) }
