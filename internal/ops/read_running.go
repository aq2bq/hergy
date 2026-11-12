package ops

import (
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
)

type runningTeam = RunningTeam
type projectView = ProjectView

func sessionOf(read herdrNow) session {
	return session{Tabs: read.tabs, Agents: read.agents, AgentsRead: read.agentsRead, Workspaces: read.spaces, Seen: read.seen, Err: read.err}
}

func readVisibleTeams(views []projectView, read herdrNow, lenient bool) []runningTeam {
	return readRunningTeamsIn(views, sessionOf(read), lenient)
}

func judgeRead(one runningTeam, read herdrNow, now time.Time) activity.Team {
	return judgeOf(one, sessionOf(read), now)
}
func agentsRead(read herdrNow, name domainname.Seen) []activity.Agent {
	return agentsOf(sessionOf(read), name)
}
