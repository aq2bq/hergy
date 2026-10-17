package effect

import (
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// Undelivered is what a running team still has to take up: the members whose
// prompt reads differently now from the prompt the launch record holds (C9,
// 設計 10.2, W9). launched is the record's hash of each member's prompt, by the
// member's short name, and now is the definition as it reads at this moment; the
// View's Run must carry the team's own name, because a prompt names each member
// by its herdr name.
//
// The record keeps no prompt text, only its hash, so no part of the prompt can
// be named: a Change holds the member and its definition and no Parts, and the
// one line a screen shows is UndeliveredLines. A member the record does not
// name is left out: it was added since the launch (W7).
func Undelivered(launched map[string]string, now View) []Change {
	if len(launched) == 0 || now.Def == nil {
		return nil
	}
	team := now.Run.Team
	if team == "" {
		team = now.Def.Name
	}
	now.Run.Team = team
	var out []Change
	for i := range now.Def.Graph.Members {
		member := &now.Def.Graph.Members[i]
		was, ok := launched[member.Name]
		if !ok {
			continue
		}
		if promptHash(now.Def, member, now.Run) == was {
			continue
		}
		out = append(out, Change{Member: member.Name, Definition: now.Def.Name})
	}
	return out
}

// promptHash is the hash of one member's prompt as it reads now, or "" when it
// cannot be built: a prompt that does not build has moved.
func promptHash(def *definition.Definition, member *definition.MemberNode, run prompt.Run) string {
	built, err := prompt.Member(def, member, run)
	if err != nil {
		return ""
	}
	return source.Hash(built.Text)
}

// UndeliveredLines is the one line each undelivered member gets: which member's
// prompt changed since the launch, and nothing of the difference (W9). The
// handover beside it comes from Commands, which says a reprompt for a member
// with no parts.
func UndeliveredLines(changes []Change) []string {
	out := make([]string, 0, len(changes))
	for _, change := range changes {
		out = append(out, i18n.Tf("effect.undelivered.member", change.Member))
	}
	return out
}
