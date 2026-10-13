// Package prompt builds the bootstrap prompts. A template names its parts with
// %{...}; each part is built only when the template asks for it. A bare % stays
// as it is. The wording itself lives in the locale files.
package prompt

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// MemberTemplate is the default bootstrap prompt for a team member.
func MemberTemplate() string { return i18n.T("prompt.member") }

// WorkerTemplate is the bootstrap prompt for a hired worker.
func WorkerTemplate() string { return i18n.T("prompt.worker") }

// Expand replaces every %{name} with the value its builder returns. An unknown
// placeholder is an error; %{ with no closing } and %{} stay literal.
func Expand(template string, parts map[string]func() string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(template); {
		if template[i] == '%' && i+1 < len(template) && template[i+1] == '{' {
			rest := template[i+2:]
			end := strings.IndexByte(rest, '}')
			if end > 0 {
				key := rest[:end]
				build, ok := parts[key]
				if !ok {
					return "", i18n.Errorf("error.unknown_placeholder", key)
				}
				b.WriteString(build())
				i += 2 + end + 1
				continue
			}
		}
		b.WriteByte(template[i])
		i++
	}
	return b.String(), nil
}

// RosterEntry is one line of the roster.
type RosterEntry struct {
	Name string
	Role string
}

// Roster lists the team, marking the reader.
func Roster(members []RosterEntry, me string) string {
	lines := make([]string, 0, len(members))
	for _, member := range members {
		mark := ""
		if member.Name == me {
			mark = i18n.T("roster.mark")
		}
		lines = append(lines, fmt.Sprintf(i18n.T("roster.line"), member.Role, member.Name, mark))
	}
	return strings.Join(lines, "\n")
}

// HerdrMember states how a member reaches the others with Herdr as it is; the
// launcher adds no messaging of its own. intro is how the member introduces
// itself. It also names the one way to add a teammate, so a member does not
// start one with herdr directly and lose it from hergy's roster.
func HerdrMember(intro string) string {
	return i18n.Tf("herdr.guide_member", intro)
}

// HerdrWorker is the same contact guide for a hired worker, which adds no
// teammates of its own.
func HerdrWorker(intro string) string {
	return i18n.Tf("herdr.guide_worker", intro)
}

// SpawnNote states what a member may spawn. It is written into the prompt and
// never enforced: a hire outside it only prints a note. `command` is the hire
// command line shown to the member, with the placeholders it should fill in.
func SpawnNote(memberName string, purposes, profileNames []string, profileKinds map[string]string, command string) string {
	allowed := make([]string, 0, len(profileNames))
	for _, name := range profileNames {
		allowed = append(allowed, fmt.Sprintf(i18n.T("spawn.allowed_item"), name, profileKinds[name]))
	}
	separator := i18n.T("list.separator")
	return i18n.Tf("spawn.note",
		strings.Join(purposes, separator),
		strings.Join(allowed, separator),
		command,
		memberName)
}
