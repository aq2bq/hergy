// Package prompt builds the bootstrap prompts. A template names its parts with
// %{...}; each part is built only when the template asks for it. A bare % stays
// as it is. The wording itself lives in the locale files.
package prompt

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// MemberTemplate is the default bootstrap prompt for a team member, in one
// language; empty is the language in use.
func MemberTemplate(lang string) string { return i18n.Text(lang, "prompt.member") }

// WorkerTemplate is the bootstrap prompt for a hired worker.
func WorkerTemplate() string { return i18n.T("prompt.worker") }

// ExpandSegments says where each piece came from. A
// placeholder in parts becomes one or more segments of its own; one in simple
// is written into the template's own text; the text between placeholders is a
// template segment. Joining every segment's Text in order gives the prompt
// character for character (r-4cf9).
func ExpandSegments(template string, origin definition.PromptOrigin, simple map[string]func() string, parts map[string]func() []definition.PromptSegment) (string, []definition.PromptSegment, error) {
	var literal strings.Builder
	var segments []definition.PromptSegment
	flush := func() {
		if literal.Len() == 0 {
			return
		}
		segments = append(segments, definition.PromptSegment{
			Kind: "template", Text: literal.String(), Origin: origin,
		})
		literal.Reset()
	}
	for i := 0; i < len(template); {
		if template[i] == '%' && i+1 < len(template) && template[i+1] == '{' {
			rest := template[i+2:]
			end := strings.IndexByte(rest, '}')
			if end > 0 {
				key := rest[:end]
				if build, ok := parts[key]; ok {
					flush()
					segments = append(segments, build()...)
					i += 2 + end + 1
					continue
				}
				if build, ok := simple[key]; ok {
					literal.WriteString(build())
					i += 2 + end + 1
					continue
				}
				return "", nil, i18n.Errorf("error.unknown_placeholder", key)
			}
		}
		literal.WriteByte(template[i])
		i++
	}
	flush()
	var text strings.Builder
	for _, segment := range segments {
		text.WriteString(segment.Text)
	}
	return text.String(), segments, nil
}

// RosterEntry is one line of the roster.
type RosterEntry struct {
	Name string
	Role string
}

// Roster lists the team, marking the reader. lang is the language the prompt is
// written in; empty is the language in use.
func Roster(lang string, members []RosterEntry, me string) string {
	lines := make([]string, 0, len(members))
	for _, member := range members {
		mark := ""
		if member.Name == me {
			mark = i18n.Text(lang, "roster.mark")
		}
		lines = append(lines, fmt.Sprintf(i18n.Text(lang, "roster.line"), member.Role, member.Name, mark))
	}
	return strings.Join(lines, "\n")
}

// HerdrMember states how a member reaches the others with Herdr as it is; the
// launcher adds no messaging of its own. intro is how the member introduces
// itself. It also names the one way to add a teammate, so a member does not
// start one with herdr directly and lose it from hergy's roster.
func HerdrMember(lang, intro string) string {
	return i18n.Textf(lang, "herdr.guide_member", intro)
}

// HerdrWorker is the same contact guide for a hired worker, which adds no
// teammates of its own.
func HerdrWorker(intro string) string {
	return i18n.Tf("herdr.guide_worker", intro)
}

// WorkerPair is one purpose-to-profile pair a member may hire for.
type WorkerPair struct {
	Purpose string
	Profile string
}

// WorkersNote states what a member may hire. It is written into the prompt and
// never enforced: a hire outside it only prints a note. It lists the pairs of
// purpose and profile the member may hire for. `command` is the hire command
// line shown to the member, with the placeholders it should fill in.
func WorkersNote(lang, memberName string, workers []WorkerPair, command string) string {
	allowed := make([]string, 0, len(workers))
	for _, worker := range workers {
		allowed = append(allowed, fmt.Sprintf(i18n.Text(lang, "workers.allowed_item"), worker.Purpose, worker.Profile))
	}
	separator := i18n.Text(lang, "list.separator")
	return i18n.Textf(lang, "workers.note",
		strings.Join(allowed, separator),
		command,
		memberName)
}

// TurnPart is one node a member hands the turn to, as the member's prompt names
// it: the herdr name it reaches the party by, and its role. Role is empty for
// the human, which has none.
type TurnPart struct {
	Name string
	Role string
}

// TurnsNote lists who a member hands the turn to. It is built from the graph's
// edges out of the member and adds nothing to them (I13): taking one edge away
// takes that party out of the prompt.
func TurnsNote(lang string, turns []TurnPart) string {
	if len(turns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(turns))
	for _, turn := range turns {
		if turn.Role == "" {
			lines = append(lines, i18n.Text(lang, "turns.human_item"))
			continue
		}
		lines = append(lines, fmt.Sprintf(i18n.Text(lang, "turns.member_item"), turn.Name, turn.Role))
	}
	return i18n.Textf(lang, "turns.note", strings.Join(lines, "\n"))
}
