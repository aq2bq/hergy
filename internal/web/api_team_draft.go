package web

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the part of the team kind that is about a definition that is not
// there yet: a row added from the scaffold, a duplicate of another definition,
// or a copy of one into the other place (spec-teams B1-B3, B6, C8). The row
// stands on a base text rather than on a saved file, so its checks read the base
// with the row's own ops laid over it, and its save writes the whole folder.

// apiTeamPreview is the topology tab's YAML preview (spec-teams B7): the lines
// the row would write, each with what it stands for, and how they differ from
// the text the row stands on. The lines are the text a save would write, comments
// and all; the screen draws them and marks nothing of its own.
type apiTeamPreview struct {
	Lines []definition.YAMLLine `json:"lines"`
	Diff  check.RoleDiff        `json:"diff"`
}

// t2DraftRow is a definition that is not there yet, as the server reads it: the
// text its two files stand on, the place and layer it would go into, whether the
// starter role documents and profile go with it, and whether the person chose
// them (r-54b8).
type t2DraftRow = ops.ResourceDraft

// t2DraftRowOf reads the base a row stands on: the starter files for a row added
// from the scaffold (spec-teams B2), and the named definition's own files for a
// duplicate or a copy (spec-teams B3).
func (s *Server) t2DraftRowOf(in apiIn) (t2DraftRow, *apiErr) {
	query, bad := s.resourceDraftInput(in)
	if bad != nil {
		return t2DraftRow{}, bad
	}
	row, failure := s.resources().ReadResourceDraft(query)
	return row, s.resourceFailure(failure, nil, false)
}

// t2LineTexts is the text of a preview's lines, for a diff by line.
func t2LineTexts(lines []definition.YAMLLine) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.Text
	}
	return out
}

// t2AgreeOfText is one layer's agreement read from a text rather than from the
// file: what a definition that is not there yet would read (spec-teams B2).
func (s *Server) t2AgreeOfText(name, layer, text string) *apiTeamAgree {
	path := s.t2AgreeFile(name, layer)
	out := &apiTeamAgree{Path: s.cat.Display(path)}
	if text == "" {
		return out
	}
	out.Has, out.Hash = true, source.Hash(text)
	out.Doc = r2DocOf(text, path)
	return out
}

// checkDraft answers what is wrong with a definition that is not there yet, and
// carries what the screen draws it from (spec-teams B2, B7). The base's two
// files are laid over the catalog at the paths the row would be written to, so
// the check reads the draft as if it were saved, and a row named after a
// definition that is already there is checked as its own draft.
func (k apiTeam) checkDraft(in apiIn) (any, *apiErr) {
	s := k.s
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	query, bad := s.resourceDraftInput(in)
	if bad != nil {
		return nil, bad
	}
	result, failure := s.resources().CheckDefinitionDraft(query, body.ops)
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	row := result.Row
	problems := make([]apiProblem, 0, len(result.NameProblems)+len(result.Findings))
	for _, p := range result.NameProblems {
		problems = append(problems, apiProblem{Sev: p.Sev, At: p.At, Field: p.Field, Text: p.Text})
	}
	problems = append(problems, t2FigureProblems(result.Findings, result.Display)...)
	var carry *apiCheckCarry
	if result.Carry != nil {
		carry = &apiCheckCarry{Roles: result.Carry.Roles, Profiles: result.Carry.Profiles}
	}
	return apiCheck{Problems: problems, Doc: s.t2DocWithAgree(result.Definition, s.t2AgreeOfText(in.Name, row.Layer, row.Agree)), Draft: &apiCheckDraft{Name: in.Name, Place: in.Place.String(), Starter: row.Starter, Path: result.Display, Hash: source.Hash(""), Writes: result.Writes, HasRoles: result.HasRoles, HasProfiles: result.HasProfiles, Carry: carry}, Preview: apiTeamPreview{Lines: definition.YAMLLines(result.Draft), Diff: check.LineDiff(t2LineTexts(definition.YAMLLines(row.Team)), t2LineTexts(definition.YAMLLines(result.Draft)))}}, nil
}

// checkDraftAgreement answers what is wrong with the rules of a definition that
// is not there yet: the same check the saved row's rules get (spec-teams C6),
// with the base team.yaml laid over the catalog so its arrows are judged against
// the draft's turns.
func (k apiTeam) checkDraftAgreement(in apiIn) (any, *apiErr) {
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	if body.in.Doc == nil {
		return nil, refuse(422, "agreement: one document is needed")
	}
	query, bad := k.s.resourceDraftInput(in)
	if bad != nil {
		return nil, bad
	}
	checked, failure := k.s.resources().CheckDefinitionDraftAgreement(query, body.in.Doc.Text)
	if failure != nil {
		return nil, k.s.resourceFailure(failure, nil, false)
	}
	return r2CheckAnswer(checked, false), nil
}

// saveFolder writes a definition that is not there yet, whole (spec-teams B2,
// B6, C8): its team.yaml (the base with the row's ops), its agreement.md (the
// base with the draft's document), and, for one made from the scaffold, the
// layer's role documents and the starter profile. A name problem, or a folder
// that has appeared since the check, stops it before anything is written.
func (k apiTeam) saveFolder(name string, in apiIn) (any, *apiErr) {
	s := k.s
	ref, bad := s.resourceRef(domainplace.Definition, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	body, bad := t2DecodeOps(in)
	if bad != nil {
		return nil, bad
	}
	query, bad := s.resourceDraftInput(in)
	if bad != nil {
		return nil, bad
	}
	var agreement *string
	if body.in.Doc != nil {
		agreement = &body.in.Doc.Text
	}
	_, failure := s.resources().SaveDefinitionDraft(ref, query, body.ops, agreement)
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	return k.savedAnswer(name, in.Place.String()), nil
}
