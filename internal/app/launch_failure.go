package app

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
)

// startFailureText is what a start that stopped partway says, the same on the
// command line and on a screen (d-5f02): the member herdr refused, herdr's own
// reason in words, who started and what remains, and the command that goes on,
// with the project it is typed in. project is the project's name, and command the
// way on, as the command line writes it.
func startFailureText(f launcher.StartFailure, project, command string) string {
	reason := strings.TrimRight(f.Reason, "。. ")
	lines := make([]string, 0, 5)
	if f.Member != "" {
		lines = append(lines, i18n.Tf("launch.failed_member", f.Member, reason))
	} else {
		lines = append(lines, i18n.Tf("launch.failed_no_member", reason))
	}
	if len(f.Started) > 0 {
		lines = append(lines, i18n.Tf("launch.failed_started", strings.Join(f.Started, i18n.T("list.separator"))))
	} else {
		lines = append(lines, i18n.T("launch.failed_none_started"))
	}
	// A start that made no pane left no team to put away: it does not count as
	// running, and the way on is the start again rather than a stop first.
	if f.Panes > 0 {
		lines = append(lines, i18n.Tf("launch.failed_left", f.Tab, f.Panes), i18n.T("launch.failed_running"))
	} else {
		lines = append(lines, i18n.T("launch.failed_left_none"))
	}
	lines = append(lines, i18n.Tf("launch.failed_next", project, command))
	return strings.Join(lines, i18n.T("launch.failed_gap"))
}

// applyFixCommand is the command that tries the members of one apply again, as
// the command line writes them.
func applyFixCommand(group []plannedMember) string {
	names := make([]string, 0, len(group))
	for _, member := range group {
		names = append(names, member.start.Short)
	}
	return i18n.Tf("web.apply.command", strings.Join(names, " "))
}

// failedStartCommand is the command that goes on after a start stopped partway:
// a start that left panes behind is put away first, and one that left nothing is
// started again.
func failedStartCommand(f launcher.StartFailure, team, up string) string {
	if f.Panes == 0 {
		return up
	}
	return i18n.Tf("launch.fix_boot", team, up)
}
