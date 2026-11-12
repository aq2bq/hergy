package ops

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/check"
	"github.com/aq2bq/hergy/internal/ops/internal/newteam"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// ResourceProblem identifies a draft field's check result.
type ResourceProblem struct{ Sev, At, Field, Text string }

// ResourceDraft keeps the starter/copy basis separate from saved resources.
type ResourceDraft struct {
	Place, Layer                          string
	Starter, StarterRoles, StarterProfile bool
	Team, Agree                           string
	AgreementExists                       bool
}

// ResourceDraftBase is the already selected source catalog of a definition copy.
type ResourceDraftBase struct {
	Place   domainplace.Place
	Name    string
	Catalog *catalog.Catalog
}

// ResourceDraftInput is a draft's requested destination and chosen dependencies.
type ResourceDraftInput struct {
	Place                             domainplace.Place
	Name                              string
	Base                              *ResourceDraftBase
	CarryRoles, CarryProfiles         []string
	New, StarterRoles, StarterProfile bool
}
type t2Carry struct {
	Source          *catalog.Catalog
	Layer           string
	Roles, Profiles []string
}
type t2Chosen struct{ Roles, Profiles map[string]bool }

var memberField = regexp.MustCompile(`^members\[(\d+)\]`)
var t2WorkerField = regexp.MustCompile(`^workers\((\d+)\)$`)

func (s ResourceScope) resourceGlobalCatalog() *catalog.Catalog {
	if s.Global != nil {
		return s.Global
	}
	return s.Catalog
}
func (s ResourceScope) t2NameProblems(in ResourceDraftInput) []ResourceProblem {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return []ResourceProblem{{Sev: "err", At: "name", Field: "name", Text: i18n.T("error.team_name_needed")}}
	}
	if _, err := domainname.NewDefinitionName(name); err != nil {
		return []ResourceProblem{{Sev: "err", At: "name", Field: "name", Text: i18n.Tf("error.new_name_pattern", name)}}
	}
	dir := filepath.Join(s.Catalog.LayerDir(in.Place.Layer()), "teams", name)
	if _, err := os.Stat(dir); err == nil {
		return []ResourceProblem{{Sev: "err", At: "name", Field: "name", Text: i18n.Tf("error.new_exists", name, s.Catalog.Display(dir))}}
	}
	return nil
}

func (s ResourceScope) t2DraftWrites(row ResourceDraft, in ResourceDraftInput, name, team string) []string {
	// The team.yaml text is the row with its ops applied, so the starter roles
	// and profile the save would write follow the members the row now has: a
	// starter name no member uses is not one the person chose (r-54b8, d-86b1).
	files := []scaffold.File{{Path: "team.yaml", Data: []byte(team)}}
	if row.AgreementExists {
		files = append(files, scaffold.File{Path: "agreement.md"})
	}
	written, err := newteam.Plan(newteam.In{
		Catalog: s.Catalog, Layer: row.Layer, Name: name, Lang: i18n.Language(),
		Files: files, Starter: row.Starter, StarterRoles: row.StarterRoles, StarterProfile: row.StarterProfile,
		Carry: s.carryIn(in),
	})
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(written.Files)+len(written.Roles)+2)
	for _, paths := range [][]string{written.Files, written.Roles, written.CarryRoles} {
		for _, path := range paths {
			out = append(out, s.Catalog.Display(path))
		}
	}
	if written.Profile != "" {
		out = append(out, s.Catalog.Display(written.Profile))
	}
	if written.CarryProfileFile != "" {
		out = append(out, s.Catalog.Display(written.CarryProfileFile))
	}
	return out
}

func (s ResourceScope) t2StarterFindings(row ResourceDraft, cat *catalog.Catalog, def *definition.Definition, findings []check.Finding) []check.Finding {
	roles := t2RoleNames(cat)
	profiles := t2ProfileNames(cat)
	out := make([]check.Finding, 0, len(findings))
	for _, finding := range findings {
		if member := t2MemberAt(def, finding.Field); member != nil {
			if finding.Kind == check.KindPrompt && strings.HasSuffix(finding.Field, ".role") {
				if _, held := cat.Role(member.Role); !held {
					if _, markdown := cat.LegacyRole(member.Role); !markdown {
						if row.StarterRoles && slices.Contains(scaffold.RoleNames, member.Role+".md") {
							// The save writes this role document; it is not missing.
							continue
						}
						finding.Text = t2ChooseRole(member.Name.String(), roles)
					}
				}
			}
			if finding.Stops && strings.HasSuffix(finding.Field, ".profile") {
				if row.StarterProfile && member.Profile == scaffold.ProfileName {
					// The save adds the starter profile; it is not missing.
					continue
				}
				// The person picks a profile where the member stands, so the
				// sentence says which profiles are there rather than how to
				// write the file (d-86b1).
				finding.Text = t2ChooseProfile(member.Name.String(), profiles)
			}
		}
		out = append(out, finding)
	}
	return out
}

func t2RoleNames(cat *catalog.Catalog) []string {
	var out []string
	for _, role := range cat.Roles() {
		if role.HasDocument {
			out = append(out, role.Name)
		}
	}
	return out
}

func t2ProfileNames(cat *catalog.Catalog) []string {
	profiles, _, err := cat.Profiles()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func t2ChooseRole(member string, roles []string) string {
	if len(roles) == 0 {
		return i18n.Tf("error.role_to_choose_none", member)
	}
	return i18n.Tf("error.role_to_choose", member, strings.Join(roles, i18n.T("list.separator")))
}

func t2ChooseProfile(member string, profiles []string) string {
	if len(profiles) == 0 {
		return i18n.Tf("error.profile_to_choose_none", member)
	}
	return i18n.Tf("error.profile_to_choose", member, strings.Join(profiles, i18n.T("list.separator")))
}

func t2MemberAt(def *definition.Definition, field string) *definition.MemberNode {
	if def == nil || def.Graph == nil {
		return nil
	}
	m := memberField.FindStringSubmatch(field)
	if m == nil {
		return nil
	}
	i, err := strconv.Atoi(m[1])
	if err != nil || i < 0 || i >= len(def.Graph.Members) {
		return nil
	}
	return &def.Graph.Members[i]
}

func t2ChosenOf(in ResourceDraftInput) t2Chosen {
	out := t2Chosen{Roles: map[string]bool{}, Profiles: map[string]bool{}}
	for _, name := range in.CarryRoles {
		out.Roles[name] = true
	}
	for _, name := range in.CarryProfiles {
		out.Profiles[name] = true
	}
	return out
}

func t2IsCopyToGlobal(in ResourceDraftInput, row ResourceDraft) bool {
	return in.Base != nil && row.Layer == source.LayerGlobal && in.Base.Place.Layer() == source.LayerLocal
}

func (s ResourceScope) t2CarryOf(in ResourceDraftInput, row ResourceDraft, def *definition.Definition) *t2Carry {
	if !t2IsCopyToGlobal(in, row) {
		return nil
	}
	if def == nil || def.Graph == nil {
		return nil
	}
	from := in.Base.Catalog
	srcProfiles, err := from.LayerProfiles(source.LayerLocal)
	if err != nil {
		return nil
	}
	destProfiles, err := s.Catalog.LayerProfiles(source.LayerGlobal)
	if err != nil {
		return nil
	}
	out := &t2Carry{Source: from, Layer: source.LayerLocal, Roles: []string{}, Profiles: []string{}}
	roles, profiles := map[string]bool{}, map[string]bool{}
	role := func(name string) {
		if roles[name] {
			return
		}
		roles[name] = true
		if _, held := s.Catalog.LayerRole(source.LayerGlobal, name); held {
			return
		}
		if _, there := from.LayerRole(source.LayerLocal, name); !there {
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

func (c *t2Carry) holdsRole(name string) bool    { return slices.Contains(c.Roles, name) }
func (c *t2Carry) holdsProfile(name string) bool { return slices.Contains(c.Profiles, name) }
func (c *t2Carry) display() string               { return c.Source.Display(c.Source.LayerDir(c.Layer)) }
func t2WorkerIndex(field string) (int, bool) {
	m := t2WorkerField.FindStringSubmatch(field)
	if m == nil {
		return 0, false
	}
	i, err := strconv.Atoi(m[1])
	return i, err == nil
}

func (s ResourceScope) carryIn(in ResourceDraftInput) *newteam.Carry {
	chosen := t2ChosenOf(in)
	if in.Base == nil || len(chosen.Roles)+len(chosen.Profiles) == 0 {
		return nil
	}
	if in.Base.Place.Layer() != source.LayerLocal || in.Place.Layer() != source.LayerGlobal {
		return nil
	}
	from := in.Base.Catalog
	return &newteam.Carry{
		Catalog: from, Layer: source.LayerLocal,
		Roles: in.CarryRoles, Profiles: in.CarryProfiles,
	}
}

func (s ResourceScope) carryFindings(carry *t2Carry, chosen t2Chosen, def *definition.Definition, findings []check.Finding) []check.Finding {
	if carry == nil || def == nil || def.Graph == nil {
		return findings
	}
	place := i18n.T("place.global")
	dir := carry.display()
	out := make([]check.Finding, 0, len(findings))
	for _, finding := range findings {
		if member := t2MemberAt(def, finding.Field); member != nil {
			if strings.HasSuffix(finding.Field, ".role") && carry.holdsRole(member.Role) {
				if chosen.Roles[member.Role] {
					continue
				}
				finding.Text = i18n.Tf("error.role_to_carry", member.Name.String(), member.Role, place, dir, place)
			}
			if strings.HasSuffix(finding.Field, ".profile") && carry.holdsProfile(member.Profile) {
				if chosen.Profiles[member.Profile] {
					continue
				}
				finding.Text = i18n.Tf("error.profile_to_carry", member.Name.String(), member.Profile, place, dir, place)
			}
		}
		if i, ok := t2WorkerIndex(finding.Field); ok && i < len(def.Graph.Workers) {
			worker := def.Graph.Workers[i]
			if carry.holdsProfile(worker.Profile) {
				if chosen.Profiles[worker.Profile] {
					continue
				}
				finding.Text = i18n.Tf("error.workers_profile_to_carry", worker.From, worker.Purpose, worker.Profile, place, dir, place)
			}
		}
		out = append(out, finding)
	}
	return out
}

// ResourceDraftChecked contains the folder check and planned writes.
type ResourceDraftChecked struct {
	Row                   ResourceDraft
	Findings              []check.Finding
	NameProblems          []ResourceProblem
	Definition            *definition.Definition
	Draft, Display        string
	Writes                []string
	HasRoles, HasProfiles bool
	Carry                 *ResourceCopyCandidates
}

// ResourceCopyCandidates lists dependencies a definition copy can bring.
type ResourceCopyCandidates struct{ Roles, Profiles []string }

// CheckDefinitionDraft inspects the folder and its chosen dependencies together.
func (s ResourceScope) CheckDefinitionDraft(in ResourceDraftInput, ops []definition.Op) (ResourceDraftChecked, *ResourceFailure) {
	row, bad := s.ReadResourceDraft(in)
	if bad != nil {
		return ResourceDraftChecked{}, bad
	}
	return s.checkDefinitionDraft(in, row, ops)
}

func (s ResourceScope) checkDefinitionDraft(in ResourceDraftInput, row ResourceDraft, ops []definition.Op) (ResourceDraftChecked, *ResourceFailure) {
	name := in.Name
	if name == "" {
		name = "t2draft"
	}
	base := s.Catalog
	if t2IsCopyToGlobal(in, row) {
		base = s.resourceGlobalCatalog()
	}
	path := base.TeamYAML(name, row.Layer)
	display := base.Display(base.TeamYAML(in.Name, row.Layer))
	cat := base.With(path, row.Team).With(filepath.Join(filepath.Dir(path), "agreement.md"), row.Agree)
	report := check.Team(check.TeamInput{Catalog: cat, Name: name, Layer: row.Layer, Ops: ops})
	def, _ := cat.Definition(name, row.Layer)
	findings := report.Problems
	if row.Starter {
		findings = s.t2StarterFindings(row, cat, def, findings)
	}
	carry := s.t2CarryOf(in, row, def)
	findings = s.carryFindings(carry, t2ChosenOf(in), def, findings)
	draft, err := editDefinitionText(row.Team, display, ops)
	if err != nil {
		return ResourceDraftChecked{}, &ResourceFailure{Text: err.Error()}
	}
	hasProfiles, _ := newteam.HasProfiles(cat)
	out := ResourceDraftChecked{Row: row, Findings: findings, NameProblems: s.t2NameProblems(in), Definition: def, Draft: draft, Display: display, Writes: s.t2DraftWrites(row, in, in.Name, draft), HasRoles: newteam.HasRoles(cat), HasProfiles: hasProfiles}
	if carry != nil {
		out.Carry = &ResourceCopyCandidates{Roles: carry.Roles, Profiles: carry.Profiles}
	}
	return out, nil
}

// SaveDefinitionDraft saves the checked destination through the folder sequence.
func (s ResourceScope) SaveDefinitionDraft(ref domainplace.Ref, in ResourceDraftInput, ops []definition.Op, agreement *string) (ResourceSaved, *ResourceFailure) {
	if ref.Place() != in.Place || ref.Name() != in.Name {
		return ResourceSaved{}, &ResourceFailure{Text: "draft and destination identify different resources"}
	}
	row, bad := s.ReadResourceDraft(in)
	if bad != nil {
		return ResourceSaved{}, bad
	}
	return s.createDefinition(ref, definitionFolder{Draft: row, Input: in, Ops: ops, Agreement: agreement})
}

// ReadResourceDraft reads the authoritative starter or copy basis.
func (s ResourceScope) ReadResourceDraft(in ResourceDraftInput) (ResourceDraft, *ResourceFailure) {
	out := ResourceDraft{Place: in.Place.API(), Layer: in.Place.Layer(), Starter: in.New, StarterRoles: in.StarterRoles, StarterProfile: in.StarterProfile}
	if in.New {
		files, err := scaffold.Team(i18n.Language())
		if err != nil {
			return out, &ResourceFailure{Text: err.Error()}
		}
		out.AgreementExists = true
		for _, file := range files {
			switch file.Path {
			case "team.yaml":
				out.Team = string(file.Data)
			case "agreement.md":
				out.Agree = string(file.Data)
			}
		}
		return out, nil
	}
	if in.Base == nil {
		return out, &ResourceFailure{Status: 400, Text: "team: a new definition needs new or base"}
	}
	from, err := ObserveResource(domainplace.Definition, in.Base.Place, in.Base.Name)
	if err != nil {
		return out, &ResourceFailure{Text: err.Error()}
	}
	draft, bad := ReadDefinitionDraft(in.Base.Catalog, ExistingResource(from))
	if bad != nil {
		return out, bad
	}
	out.Team, out.Agree, out.AgreementExists = draft.Team, draft.Agreement, draft.AgreementExists
	return out, nil
}

// CheckDefinitionDraftAgreement checks the rules beside the same copy/starter basis.
func (s ResourceScope) CheckDefinitionDraftAgreement(in ResourceDraftInput, text string) (check.DocReport, *ResourceFailure) {
	row, bad := s.ReadResourceDraft(in)
	if bad != nil {
		return check.DocReport{}, bad
	}
	return s.checkDefinitionDraftAgreement(in, row, text), nil
}

func (s ResourceScope) checkDefinitionDraftAgreement(in ResourceDraftInput, row ResourceDraft, text string) check.DocReport {
	name := in.Name
	if name == "" {
		name = "t2draft"
	}
	path := s.Catalog.TeamYAML(name, row.Layer)
	cat := s.Catalog.With(path, row.Team).With(filepath.Join(filepath.Dir(path), "agreement.md"), row.Agree)
	return check.Agreement(check.AgreementInput{Catalog: cat, Name: name, Place: row.Layer, Text: text})
}
