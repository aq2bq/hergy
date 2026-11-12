package app

import (
	"strings"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
)

// applyFixCommand is the command that tries the members of one apply again, as
// the command line writes them.
func applyFixCommand(group []plannedMember) string {
	names := make([]string, 0, len(group))
	for _, member := range group {
		names = append(names, member.start.Short.String())
	}
	return i18n.Tf("web.apply.command", strings.Join(names, " "))
}

func startFailureText(f launcher.StartFailure, project, command string) string {
	return ops.StartFailureText(f, project, command)
}
