package check

import (
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// EffectLine is one member's part that a change moves, as the lines the prompt
// writes it with (spec-teams 1.4.4, X3).
type EffectLine struct {
	Member  string   `json:"member"` // the herdr name "<team>-<member>"
	Part    string   `json:"part"`   // "turns" or "workers"
	Removed []string `json:"removed"`
	Added   []string `json:"added"`
}

// EffectReport is what one tried change does (C4).
type EffectReport struct {
	Lines    []EffectLine `json:"lines"`
	Problems []Finding    `json:"problems"`
}

// TeamEffect is what one more op would do, on top of the ops already in in.Ops
// (C4): the before/after effect.Of of the two views, written as the prompt's own
// lines, and the problems the op would add.
//
// A definition either view cannot read gives an empty report: there is no prompt
// to compare, and a screen shows the op's own refusal when it tries to save it.
func TeamEffect(in TeamInput, op definition.Op) EffectReport {
	cat := in.Catalog
	if cat == nil {
		return EffectReport{}
	}
	before, err := teamView(cat, in)
	if err != nil {
		return EffectReport{}
	}
	afterIn := in
	afterIn.Ops = append(append([]definition.Op(nil), in.Ops...), op)
	after, err := teamView(cat, afterIn)
	if err != nil {
		return EffectReport{}
	}

	return EffectReport{
		Lines:    effectLines(in.Name, before, after, effect.Of(before, after)),
		Problems: addedProblems(Team(in), Team(afterIn)),
	}
}

// teamView is one read state of a team definition, as effect.Of wants it: the
// ops applied, laid over the catalog, with the profiles and the facts a prompt
// is built with. A definition that does not read is an error.
func teamView(cat *catalog.Catalog, in TeamInput) (effect.View, error) {
	path := cat.TeamYAML(in.Name, in.Layer)
	text, err := cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return effect.View{}, err
	}
	text, err = applyTeamOps(cat, path, text, in.Ops)
	if err != nil {
		return effect.View{}, err
	}
	overlay := cat.With(path, text)
	def, err := overlay.Definition(in.Name, in.Layer)
	if err != nil {
		return effect.View{}, err
	}
	profiles, _, err := cat.Profiles()
	if err != nil {
		return effect.View{}, err
	}
	return effect.View{
		Def: def, Profiles: profiles,
		Run: prompt.Run{
			Team: in.Name, Base: cat.Base(),
			Reader: overlay.Reader(), Sources: overlay, Display: overlay.Display,
		},
	}, nil
}

// effectLines is the changes grouped by member and part: one line per member and
// part, holding what left and what came in.
func effectLines(team string, before, after effect.View, changes []effect.Change) []EffectLine {
	var lines []EffectLine
	index := map[[2]string]int{}
	add := func(member, part string, removed, added []string) {
		if len(removed) == 0 && len(added) == 0 {
			return
		}
		key := [2]string{member, part}
		if at, ok := index[key]; ok {
			lines[at].Removed = append(lines[at].Removed, removed...)
			lines[at].Added = append(lines[at].Added, added...)
			return
		}
		index[key] = len(lines)
		lines = append(lines, EffectLine{Member: member, Part: part, Removed: removed, Added: added})
	}
	for _, change := range changes {
		member := team + "-" + change.Member
		for _, part := range change.Parts {
			switch part.Part {
			case effect.Turns:
				add(member, "turns", turnEffectLines(team, part.Removed), turnEffectLines(team, part.Added))
			case effect.Workers:
				removed := workerEffectLines(change.Member, part.Removed, before)
				added := workerEffectLines(change.Member, part.Added, after)
				for _, changed := range part.Changed {
					removed = append(removed, workerItemLine(change.Member, changed.Name, before))
					added = append(added, workerItemLine(change.Member, changed.Name, after))
				}
				add(member, "workers", removed, added)
			case effect.Roster, effect.RoleDocument, effect.Agreements, effect.Launch, effect.Env:
				// A part the team screen shows no line for: the figure's cards
				// only carry the turns and the workers (spec-teams X3).
			}
		}
	}
	return lines
}

// turnEffectLines is the "ターンを渡す相手" lines of a change, exactly the prompt's
// own: the human's one line, or a member's line naming its herdr name and role.
func turnEffectLines(team string, names []effect.Named) []string {
	var out []string
	for _, named := range names {
		if named.Role == "" {
			out = append(out, i18n.T("turns.human_item"))
			continue
		}
		out = append(out, i18n.Tf("turns.member_item", herdrName(team, named.Name), named.Role))
	}
	return out
}

// herdrName is a member's herdr name from the short name effect names it by. A
// name that already carries the team's prefix stands as it is.
func herdrName(team, member string) string {
	if strings.HasPrefix(member, team+"-") {
		return member
	}
	return team + "-" + member
}

// workerEffectLines is the "ワーカーの追加" lines of a change, exactly the prompt's
// own "<purpose> → <profile>". A name with a role is a member party, not a
// worker, and is left out.
func workerEffectLines(from string, names []effect.Named, view effect.View) []string {
	var out []string
	for _, named := range names {
		if named.Role != "" {
			continue
		}
		out = append(out, workerItemLine(from, named.Name, view))
	}
	return out
}

// workerItemLine is one worker's line, with the profile read from the definition
// the name's side stands for: the before definition for one taken away, the
// after definition for one added.
func workerItemLine(from, purpose string, view effect.View) string {
	return i18n.Tf("workers.allowed_item", purpose, workerProfile(view, from, purpose))
}

// workerProfile is the profile a member hires a worker for, from the definition a
// view holds, or "" when the definition does not hold that worker.
func workerProfile(view effect.View, from, purpose string) string {
	if view.Def == nil {
		return ""
	}
	for _, worker := range view.Def.Graph.Workers {
		if worker.From == from && worker.Purpose == purpose {
			return worker.Profile
		}
	}
	return ""
}

// addedProblems is the findings the after check has that the before one does
// not, compared by sentence, so a screen shows only what the change would add.
func addedProblems(before, after TeamReport) []Finding {
	seen := map[string]bool{}
	for _, finding := range before.Problems {
		seen[finding.Text] = true
	}
	var out []Finding
	for _, finding := range after.Problems {
		if seen[finding.Text] {
			continue
		}
		out = append(out, finding)
	}
	return out
}
