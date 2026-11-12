package app

import (
	"fmt"
	"io"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/team"
)

func launchWarning(out io.Writer, note string) {
	if note != "" {
		fmt.Fprintln(out, note)
	}
}
func recordReceivedLaunch(out io.Writer, env team.Env, r *ops.Resolved, command string, members []definition.Start, l *launcher.Launcher) {
	launchWarning(out, ops.RecordReceivedLaunch(env, r, command, members, l, launchlog.Workspace{}))
}
func recordHire(out io.Writer, env team.Env, r *ops.Resolved, worker definition.Start) {
	launchWarning(out, ops.RecordHire(env, r, worker))
}
