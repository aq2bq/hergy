package app

import (
	"path/filepath"
	"slices"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/launchlog"
)

// The recorded half of the prompt reading (⑤ r-ba9b): one member's body as a
// launch recorded it. Which team the person asked for is chosen by the web
// owner (promptsFor); this file only finds that team's record. The live half
// (which teams run, and whether the member is up) is in prompt_runs.go.

// lastPromptRecord is the newest recorded body of one member of one team. The
// definition, the team's project and the exact team.yaml path are compared, so a
// project-less old record is not borrowed for another project's or another
// definition's same-named member.
func lastPromptRecord(records []launchlog.Record, project, path, definition, team, member string) (launchlog.Record, launchlog.Member, bool) {
	d, _ := domainname.ParseDefinitionName(team)
	naming := domainname.DefinitionNames(d)
	short, _ := domainname.ParseMemberName(member)
	for _, r := range slices.Backward(records) {
		if r.Team.String() != team || r.Definition != definition || !promptCommand(r.Command) {
			continue
		}
		if r.Project != "" && !samePath(r.Project, project) {
			continue
		}
		if !filepath.IsAbs(r.TeamYAML) || !samePath(r.TeamYAML, path) {
			continue
		}
		for _, m := range r.Members {
			if m.Name.String() == naming.Member(short) {
				return r, m, true
			}
		}
	}
	return launchlog.Record{}, launchlog.Member{}, false
}

// promptCommand is a command whose record holds a prompt that was handed over.
func promptCommand(command string) bool {
	switch command {
	case launchlog.CommandUp, launchlog.CommandApply, launchlog.CommandReprompt, launchlog.CommandRefresh:
		return true
	}
	return false
}
