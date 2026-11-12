package web

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/newteam"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is what a copy of a project's team definition into the global layer
// brings with it (r-7e4a, d-dc81): the definition's two files alone, and, only
// when a person chooses them, the role documents and profiles the project has and
// the global layer does not. The copy is the one place a person adds to the
// global layer by choosing, so the names are settled here and written by
// newteam, in one place each (spec-teams B2, C8).

// t2Carry is one copy's own: the layer its definition stands in, and the names
// that layer holds and the global layer does not (r-7e4a).
type t2Carry struct {
	Source   *catalog.Catalog
	Layer    string
	Roles    []string
	Profiles []string
}

// t2Chosen is what the person chose to bring: the names a save writes with the
// definition (r-7e4a). A name not here is not written.
type t2Chosen struct {
	Roles    map[string]bool
	Profiles map[string]bool
}

// t2ChosenOf reads the names a check or a save carries.
func t2ChosenOf(in apiIn) t2Chosen {
	out := t2Chosen{Roles: map[string]bool{}, Profiles: map[string]bool{}}
	for _, name := range in.CarryRoles {
		out.Roles[name] = true
	}
	for _, name := range in.CarryProfiles {
		out.Profiles[name] = true
	}
	return out
}

// t2IsCopyToGlobal says this row copies a project's definition into the global
// layer: the one copy that carries nothing by itself, because the definition it
// stands on may name a role document and a profile only its project holds
// (r-7e4a, d-dc81).
func t2IsCopyToGlobal(in apiIn, row t2DraftRow) bool {
	return in.Base != nil && row.Layer == source.LayerGlobal && in.Base.Place.Layer() == source.LayerLocal
}

// t2CarryOf settles what one copy brings with it: the role documents and
// profiles the definition's members and workers name that the global layer does
// not hold and the project it copies from does. It answers nil for a row that is
// not a copy of a project's definition into the global layer — the one copy that
// carries nothing by itself — and for a row whose definition does not read, which
// names nothing to carry.
func (s *Server) t2CarryOf(in apiIn, row t2DraftRow, def *definition.Definition) *t2Carry {
	if !t2IsCopyToGlobal(in, row) {
		return nil
	}
	if def == nil || def.Graph == nil {
		return nil
	}
	from, refusal := s.withProjectInput(in.Base.Project)
	if refusal != nil {
		return nil
	}
	srcProfiles, err := from.cat.LayerProfiles(source.LayerLocal)
	if err != nil {
		return nil
	}
	destProfiles, err := s.cat.LayerProfiles(source.LayerGlobal)
	if err != nil {
		return nil
	}
	out := &t2Carry{Source: from.cat, Layer: source.LayerLocal, Roles: []string{}, Profiles: []string{}}
	roles, profiles := map[string]bool{}, map[string]bool{}
	role := func(name string) {
		if roles[name] {
			return
		}
		roles[name] = true
		if _, held := s.cat.LayerRole(source.LayerGlobal, name); held {
			return
		}
		if _, there := from.cat.LayerRole(source.LayerLocal, name); !there {
			return
		}
		out.Roles = append(out.Roles, name)
	}
	profile := func(name string) {
		if profiles[name] {
			return
		}
		profiles[name] = true
		if _, held := destProfiles[name]; held {
			return
		}
		if _, there := srcProfiles[name]; !there {
			return
		}
		out.Profiles = append(out.Profiles, name)
	}
	for i := range def.Graph.Members {
		role(def.Graph.Members[i].Role)
		profile(def.Graph.Members[i].Profile)
	}
	for i := range def.Graph.Workers {
		profile(def.Graph.Workers[i].Profile)
	}
	return out
}

// holdsRole is whether this copy can bring one role document.
func (c *t2Carry) holdsRole(name string) bool { return slices.Contains(c.Roles, name) }

// holdsProfile is whether this copy can bring one profile.
func (c *t2Carry) holdsProfile(name string) bool { return slices.Contains(c.Profiles, name) }

// t2WorkerField names one worker: "workers[<index>]".
var t2WorkerField = regexp.MustCompile(`^workers\((\d+)\)$`)

// t2WorkerIndex is the worker one finding is about, when it is about a worker.
func t2WorkerIndex(field string) (int, bool) {
	m := t2WorkerField.FindStringSubmatch(field)
	if m == nil {
		return 0, false
	}
	i, err := strconv.Atoi(m[1])
	return i, err == nil
}

// display is the directory the copied definition stands in, as a person reads it:
// the sentence about a name the global layer does not hold says where it is.
func (c *t2Carry) display() string { return c.Source.Display(c.Source.LayerDir(c.Layer)) }

// answer is this copy as the screen reads it: the names it can bring, which the
// person chooses from (r-7e4a).
func (c *t2Carry) answer() *apiCheckCarry {
	return &apiCheckCarry{Roles: c.Roles, Profiles: c.Profiles}
}

// carryIn is what a save passes to newteam: the names the person chose, read
// from the layer the definition the row stands on reads. Nothing chosen is no
// carry at all, so a copy a person saved as it stands writes the definition
// alone (d-dc81).
func (s *Server) carryIn(in apiIn) *newteam.Carry {
	chosen := t2ChosenOf(in)
	if in.Base == nil || len(chosen.Roles)+len(chosen.Profiles) == 0 {
		return nil
	}
	if in.Base.Place.Layer() != source.LayerLocal || in.Place.Layer() != source.LayerGlobal {
		return nil
	}
	from, refusal := s.withProjectInput(in.Base.Project)
	if refusal != nil {
		return nil
	}
	return &newteam.Carry{
		Catalog: from.cat, Layer: source.LayerLocal,
		Roles: in.CarryRoles, Profiles: in.CarryProfiles,
	}
}

// carryFindings is the draft's findings with the names this copy can bring said
// as what they are (r-7e4a): a name the global layer does not hold and the
// project does is not missing — it is in the project the row copies from, and
// choosing it writes it into the global layer. A name the person chose is not
// reported at all, because the save writes it.
func (s *Server) carryFindings(carry *t2Carry, chosen t2Chosen, def *definition.Definition, findings []check.Finding) []check.Finding {
	if carry == nil || def == nil || def.Graph == nil {
		return findings
	}
	place := t("place.global")
	dir := carry.display()
	out := make([]check.Finding, 0, len(findings))
	for _, finding := range findings {
		if member := t2MemberAt(def, finding.Field); member != nil {
			if strings.HasSuffix(finding.Field, ".role") && carry.holdsRole(member.Role) {
				if chosen.Roles[member.Role] {
					continue
				}
				finding.Text = tf("error.role_to_carry", member.Name.String(), member.Role, place, dir, place)
			}
			if strings.HasSuffix(finding.Field, ".profile") && carry.holdsProfile(member.Profile) {
				if chosen.Profiles[member.Profile] {
					continue
				}
				finding.Text = tf("error.profile_to_carry", member.Name.String(), member.Profile, place, dir, place)
			}
		}
		if i, ok := t2WorkerIndex(finding.Field); ok && i < len(def.Graph.Workers) {
			worker := def.Graph.Workers[i]
			if carry.holdsProfile(worker.Profile) {
				if chosen.Profiles[worker.Profile] {
					continue
				}
				finding.Text = tf("error.workers_profile_to_carry", worker.From, worker.Purpose, worker.Profile, place, dir, place)
			}
		}
		out = append(out, finding)
	}
	return out
}
