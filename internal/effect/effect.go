// Package effect is what a change to a team definition does to the members'
// prompts: which part of which member's prompt moves, and what to do so a
// running team takes the change up (I13, I16). It is the projection the web UI's
// "保存すると" line and its 渡し方 line are made from, so the model's knowledge of
// what a part of a prompt is stays here, not in the screen (7 節 C4).
package effect

import (
	"maps"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// Part is one part of a member's prompt a change can move. The set is what the
// model says a definition change moves (I13, I16): the parties a member hands
// the turn to, the workers it may hire, the roster, the role document, the
// agreements, and, from the profile the member names, how it is started and the
// environment its pane is made with.
type Part int

const (
	// Turns is who a member hands the turn to (I13).
	Turns Part = iota
	// Workers is what a member may hire (I12c).
	Workers
	// Roster is the list of members and their roles, which every member reads
	// and which moves when a role moves.
	Roster
	// RoleDocument is the role document a member reads; a role change reads a
	// different one.
	RoleDocument
	// Agreements is the team's own agreement a member reads. The agreement
	// every team used to share is not read any more (d-31ad).
	Agreements
	// Launch is how a member's agent is started: its profile's kind and args.
	Launch
	// Env is the environment a member's pane is made with: its profile's env,
	// which reaches a pane only when the pane is made.
	Env
)

// Named is one thing a part names: a party a member hands the turn to, or a
// worker it may hire. Role is the role of a member party, and "" for the human;
// a worker is named by its purpose, so its Role is "" too.
type Named struct {
	Name string
	Role string
}

// PartChange is one part of one member's prompt that moves: what left and what
// came in, in the order the prompt names them, and, for a worker, the purposes
// whose profile moved (the worker itself stayed, so it is neither removed nor
// added).
type PartChange struct {
	Part    Part
	Removed []Named
	Added   []Named
	Changed []Named
}

// Change is one member whose prompt a definition change moves, and the
// definition it belongs to: a change to one definition's team.yaml is only that
// definition's teams' to take up (I16).
type Change struct {
	Member     string
	Definition string
	Parts      []PartChange
}

// View is one read state of a definition, as the effects are read from it: the
// definition itself, the profiles its members name, and the facts a prompt is
// built with (prompt.Run). Of compares two views of the same definition, before
// and after an edit.
type View struct {
	Def      *definition.Definition
	Profiles map[string]profile.Profile
	Run      prompt.Run
}

// Of is what an edit does to the members' prompts: for each member whose prompt
// moves, which parts move and what left or came in (I13, I16).
//
// The parts are read from the prompts the member would be given, before and
// after (prompt.Member for the documents, prompt.TurnsOf, prompt.WorkersOf and
// prompt.RosterOf for the rest), and the launch and env parts come from the
// profile the member names. When the two prompts read the same text, the parts
// that live in that text (the turns, the workers, the roster) are left out: a
// definition that writes no %{turns} does not move the text when a turn moves,
// so a screen should say nothing (C4). A member that reads the same on both
// sides has no Change.
func Of(before, after View) []Change {
	team := before.Run.Team
	if team == "" {
		team = before.Def.Name
	}
	before.Run.Team = team
	after.Run.Team = team

	rosterMoves := !slices.Equal(prompt.RosterOf(before.Def, team), prompt.RosterOf(after.Def, team))

	var changes []Change
	for i := range before.Def.Graph.Members {
		bm := before.Def.Graph.Members[i]
		am, ok := after.Def.Graph.Member(bm.Name)
		if !ok {
			// A member added or taken away is not this round's edit (W7); the
			// roster part still says the roster moved.
			continue
		}
		bb, bErr := prompt.Member(before.Def, &bm, before.Run)
		ab, aErr := prompt.Member(after.Def, &am, after.Run)
		var parts []PartChange
		parts = append(parts, documentParts(before, after, bb, bErr, ab, aErr, bm, am)...)
		if p := diffTurns(prompt.TurnsOf(before.Def, bm.Name, team), prompt.TurnsOf(after.Def, am.Name, team), team); p != nil {
			parts = append(parts, *p)
		}
		if p := diffWorkers(prompt.WorkersOf(before.Def, bm.Name), prompt.WorkersOf(after.Def, am.Name)); p != nil {
			parts = append(parts, *p)
		}
		if rosterMoves {
			parts = append(parts, PartChange{Part: Roster})
		}
		parts = append(parts, profileParts(before, after, bm, am)...)
		if bErr == nil && aErr == nil && bb.Text == ab.Text {
			parts = promptPartsOnly(parts)
		}
		if len(parts) > 0 {
			changes = append(changes, Change{Member: bm.Name, Definition: before.Def.Name, Parts: parts})
		}
	}
	return changes
}

// promptPartsOnly is the parts that stay when the two prompts read the same
// text: the ones that do not live in the text. The parts that do live in it —
// the turns, the workers, the roster, the role document and the agreements —
// are left out, because a change they carry reaches no member of this team
// (spec/42 N1: a document written back with the same values reads the same, so
// a team is not told to take it up). A launch or an env reaches a pane, not the
// sentences, so it is reported either way.
func promptPartsOnly(parts []PartChange) []PartChange {
	out := parts[:0:0]
	for _, part := range parts {
		switch part.Part {
		case Turns, Workers, Roster, RoleDocument, Agreements:
		default:
			out = append(out, part)
		}
	}
	return out
}

// documentParts is the role document and the agreements a member's prompt reads,
// from prompt.Member's build of that prompt. The documents are compared by the
// text each side reads, not by their paths alone: a screen that edits one rule
// of a document writes the same path, and the member's prompt moves all the same
// (段 4). A prompt that does not build (a document is missing) still compares the
// role, so a role change is not lost.
func documentParts(before, after View, bb *prompt.Built, bErr error, ab *prompt.Built, aErr error, bm, am definition.MemberNode) []PartChange {
	var parts []PartChange
	if bErr == nil && aErr == nil {
		if bb.HasRole != ab.HasRole || bb.Role != ab.Role || !sameDocText(before.Run.Reader, bb.Role, after.Run.Reader, ab.Role) {
			parts = append(parts, PartChange{Part: RoleDocument})
		}
		if !sameDocs(bb.Read, ab.Read) || !sameDocListText(before.Run.Reader, bb.Read, after.Run.Reader, ab.Read) {
			parts = append(parts, PartChange{Part: Agreements})
		}
		return parts
	}
	if bm.Role != am.Role {
		parts = append(parts, PartChange{Part: RoleDocument})
	}
	return parts
}

// sameDocText is whether two reads of one document carry the same text. A view
// with no reader is not compared: the paths stand for the reading, as they did
// before the texts were read.
func sameDocText(before source.Reader, bDoc source.Doc, after source.Reader, aDoc source.Doc) bool {
	if before == nil || after == nil {
		return true
	}
	return docText(before, bDoc) == docText(after, aDoc)
}

// sameDocListText is whether two lists of documents carry the same texts, in the
// same order.
func sameDocListText(before source.Reader, bDocs []source.Doc, after source.Reader, aDocs []source.Doc) bool {
	if before == nil || after == nil || len(bDocs) != len(aDocs) {
		return len(bDocs) == len(aDocs)
	}
	for i := range bDocs {
		if !sameDocText(before, bDocs[i], after, aDocs[i]) {
			return false
		}
	}
	return true
}

// docText is one document's text, or nothing when it cannot be read.
func docText(reader source.Reader, doc source.Doc) string {
	if doc.Path == "" {
		return ""
	}
	text, err := reader.Read(doc)
	if err != nil {
		return ""
	}
	return text
}

// sameDocs is whether two prompt builds read the same documents, in the same
// order.
func sameDocs(before, after []source.Doc) bool { return slices.Equal(before, after) }

// profileParts is the parts that come from the profile a member names: how it is
// started (kind and args) and the environment its pane is made with. A profile
// that is not there on both sides is not compared.
func profileParts(before, after View, bm, am definition.MemberNode) []PartChange {
	bp, bok := before.Profiles[bm.Profile]
	ap, aok := after.Profiles[am.Profile]
	if !bok || !aok {
		return nil
	}
	var parts []PartChange
	if bp.Kind != ap.Kind || !slices.Equal(bp.Args, ap.Args) {
		parts = append(parts, PartChange{Part: Launch})
	}
	if !maps.Equal(bp.Env, ap.Env) {
		parts = append(parts, PartChange{Part: Env})
	}
	return parts
}

// diffTurns is the parties a member hands the turn to that left or came in,
// named as a member reads them (the short member name, not the herdr name). A
// turn is its party: a role that moved under the same party is not a turn change
// (the roster part carries that), so the party's role is only read for the line.
func diffTurns(before, after []prompt.TurnPart, team string) *PartChange {
	removed := turnsOnly(before, after, team)
	added := turnsOnly(after, before, team)
	if len(removed) == 0 && len(added) == 0 {
		return nil
	}
	return &PartChange{Part: Turns, Removed: removed, Added: added}
}

func turnsOnly(from, other []prompt.TurnPart, team string) []Named {
	var out []Named
	for _, turn := range from {
		if hasTurnParty(other, turn.Name) {
			continue
		}
		out = append(out, Named{Name: shortName(turn.Name, team), Role: turn.Role})
	}
	return out
}

// hasTurnParty is whether a party is one of the turns. The human's party is the
// empty name.
func hasTurnParty(list []prompt.TurnPart, name string) bool {
	for _, turn := range list {
		if turn.Name == name {
			return true
		}
	}
	return false
}

// shortName is a member's short name, from the herdr name the prompt names it by
// ("<team>-<member>"). The human has no herdr name (its Name is "").
func shortName(herdr, team string) string {
	if herdr == "" {
		return ""
	}
	return strings.TrimPrefix(herdr, team+"-")
}

// diffWorkers is the workers a member may hire that left or came in, named by
// purpose, and the purposes whose profile moved (the worker itself stayed).
func diffWorkers(before, after []prompt.WorkerPair) *PartChange {
	removed := workersGone(before, after)
	added := workersGone(after, before)
	var changed []Named
	for _, worker := range before {
		next, ok := workerByPurpose(after, worker.Purpose)
		if ok && next.Profile != worker.Profile {
			changed = append(changed, Named{Name: worker.Purpose})
		}
	}
	if len(removed) == 0 && len(added) == 0 && len(changed) == 0 {
		return nil
	}
	return &PartChange{Part: Workers, Removed: removed, Added: added, Changed: changed}
}

func workersGone(from, other []prompt.WorkerPair) []Named {
	var out []Named
	for _, worker := range from {
		if _, ok := workerByPurpose(other, worker.Purpose); !ok {
			out = append(out, Named{Name: worker.Purpose})
		}
	}
	return out
}

// workerByPurpose is the worker a member hires for one purpose. A worker's
// identity is the member that hires it and the purpose (I12c), so the purpose is
// what one member's list is keyed by.
func workerByPurpose(list []prompt.WorkerPair, purpose string) (prompt.WorkerPair, bool) {
	for _, worker := range list {
		if worker.Purpose == purpose {
			return worker, true
		}
	}
	return prompt.WorkerPair{}, false
}
