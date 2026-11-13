package ops

import (
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
)

// recordReceivedLaunch owns which confirmed recipients enter a launch and when
// an empty launch is kept (d-8e2f). History keeps an up even when nobody
// received a prompt. Other operations keep no new record without a receipt.
func recordReceivedLaunch(env Environment, r *Resolved, command string, members []definition.Start, l *launcher.Launcher, workspace launchlog.Workspace, now func() time.Time) string {
	var received []definition.Start
	for _, member := range members {
		if l.Received(member.Name.String()) {
			received = append(received, member)
		}
	}
	if command != launchlog.CommandUp && len(received) == 0 {
		return ""
	}
	stateNote := runningStateWarning(receiveRunningState(r, command, received, now, l.Started, l.CreatedPane))
	if note := recordLaunchIn(env, r, command, workspace); note != "" {
		return note
	}
	return stateNote
}

// recordLaunchIn writes one launch, naming the workspace its tab was made in
// (d-73fc). A launch that makes no tab passes the zero workspace.
func recordLaunchIn(env Environment, r *Resolved, command string, workspace launchlog.Workspace) string {
	log, err := launchlog.Open()
	if err == nil {
		err = writeLaunch(log, env, r, command, workspace)
	}
	if err != nil {
		return i18n.Tf("launch.not_recorded", err)
	}
	return ""
}

// writeLaunch appends only launch identity and workspace history. Receipt
// bodies, documents and fingerprints have already been saved in runningstate.
func writeLaunch(log launchlog.Log, env Environment, r *Resolved, command string, workspace launchlog.Workspace) error {
	return log.Append(launchlog.Record{
		Time: time.Now().Format(time.RFC3339), Command: command,
		Project: env.ProjectRoot.Seen(), Team: r.Team.Name.Observation(),
		TeamYAML:  r.Team.Definition.TeamYAML.Project().Seen(),
		Workspace: workspace.Label, WorkspaceID: workspace.ID,
	})
}
