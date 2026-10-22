package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/newteam"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/scaffold"
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

// apiTeamDraftAnswer is what a check of a definition that is not there yet
// answers (spec-teams B2, B7): the draft's own document, the files a save would
// write, what is wrong with it, where it would stand, and the topology tab's YAML
// preview. The screen draws the row from it, so it builds no document of its own.
type apiTeamDraftAnswer struct {
	Name     string         `json:"name"`
	Place    string         `json:"place"`
	Starter  bool           `json:"starter"`
	Path     string         `json:"path"`
	Hash     string         `json:"hash"`
	Writes   []string       `json:"writes"`
	Problems []apiProblem   `json:"problems"`
	Doc      apiTeamDoc     `json:"doc"`
	Preview  apiTeamPreview `json:"preview"`
}

// t2DraftRow is a definition that is not there yet, as the server reads it: the
// text its two files stand on, the place and layer it would go into, and whether
// the starter role documents and profile go with it.
type t2DraftRow struct {
	Place   string
	Layer   string
	Starter bool
	Team    string
	Agree   string
}

// t2DraftRowOf reads the base a row stands on: the starter files for a row added
// from the scaffold (spec-teams B2), and the named definition's own files for a
// duplicate or a copy (spec-teams B3).
func (s *Server) t2DraftRowOf(in apiIn) (t2DraftRow, *apiErr) {
	out := t2DraftRow{Place: in.Place, Layer: catalog.Layer(in.Place)}
	if in.New {
		files, err := scaffold.Team(i18n.Language())
		if err != nil {
			return t2DraftRow{}, refuse(http.StatusUnprocessableEntity, err.Error())
		}
		out.Starter = true
		for _, file := range files {
			switch file.Path {
			case "team.yaml":
				out.Team = string(file.Data)
			case "agreement.yaml":
				out.Agree = string(file.Data)
			}
		}
		return out, nil
	}
	if in.Base == nil {
		return t2DraftRow{}, refuse(http.StatusBadRequest, "team: a new definition needs new or base")
	}
	src := catalog.Layer(in.Base.Place)
	text, err := s.cat.Reader().Read(source.Doc{Path: s.cat.TeamYAML(in.Base.Name, src)})
	if err != nil {
		return t2DraftRow{}, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	out.Team = text
	out.Agree, _ = s.cat.Reader().Read(source.Doc{Path: s.t2AgreeFile(in.Base.Name, src)})
	return out, nil
}

// t2CheckName is the name a row's own team.yaml is read under while the row is
// checked: the catalog reads a definition's file by its name, and a row that was
// just added has no name yet — its name is what the person is typing. The check
// stands the row's text at a name of its own so the figure's sentences are read
// all the same (spec-teams B2, N1: a new row shows the mission's warning). The
// name is the server's own; nothing a person reads names it.
const t2CheckName = "t2draft"

// t2DraftCheckName is the name a draft check reads the row's team.yaml under.
func t2DraftCheckName(name string) string {
	if name == "" {
		return t2CheckName
	}
	return name
}

// t2NameProblems is what is wrong with a new definition's name, in the order the
// screen shows them (spec-teams B2): it is empty, it is not a name hergy new
// accepts, or the place already has a definition of it.
func (s *Server) t2NameProblems(in apiIn) []apiProblem {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return []apiProblem{{Sev: "err", At: "name", Field: "name", Text: t("error.team_name_needed")}}
	}
	if !scaffold.NamePattern.MatchString(name) {
		return []apiProblem{{Sev: "err", At: "name", Field: "name", Text: tf("error.new_name_pattern", name)}}
	}
	dir := filepath.Join(s.cat.LayerDir(catalog.Layer(in.Place)), "teams", name)
	if _, err := os.Stat(dir); err == nil {
		return []apiProblem{{Sev: "err", At: "name", Field: "name", Text: tf("error.new_exists", name, s.cat.Display(dir))}}
	}
	return nil
}

// t2DraftWrites is the files a save of this row would write, in the order it
// writes them (spec-teams B2): the definition's own two, the layer's role
// documents a starter definition reads, and the starter profile when the layer
// cannot resolve it. The screen shows these; it decides none of them.
func (s *Server) t2DraftWrites(row t2DraftRow, name string) []string {
	files := []scaffold.File{{Path: "team.yaml"}, {Path: "agreement.yaml"}}
	written, err := newteam.Plan(newteam.In{
		Catalog: s.cat, Layer: row.Layer, Name: name, Lang: i18n.Language(),
		Files: files, Starter: row.Starter,
	})
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(written.Files)+len(written.Roles)+1)
	for _, path := range written.Files {
		out = append(out, s.cat.Display(path))
	}
	for _, path := range written.Roles {
		out = append(out, s.cat.Display(path))
	}
	if written.Profile != "" {
		out = append(out, s.cat.Display(written.Profile))
	}
	return out
}

// t2WithoutStarter drops the findings the write itself clears (spec-teams B2): a
// starter definition's role documents and the starter profile are not in the
// layer yet, and the save writes them, so a row that has not been saved is not
// told they are missing.
func t2WithoutStarter(findings []check.Finding) []check.Finding {
	out := make([]check.Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Kind == check.KindPrompt {
			continue
		}
		if finding.Stops && strings.Contains(finding.Text, scaffold.ProfileName) {
			continue
		}
		out = append(out, finding)
	}
	return out
}

// t2ApplyOps is the text a row's own team.yaml stands for: the base text with the
// row's ops applied in order.
func t2ApplyOps(base, display string, ops []definition.Op) (string, error) {
	text := base
	for _, op := range ops {
		next, err := definition.EditYAML([]byte(text), display, op)
		if err != nil {
			return "", err
		}
		text = string(next)
	}
	return text, nil
}

// t2AgreementText is the text a row's agreement.yaml stands for: the base text
// with the draft's own document laid over it (spec-teams B2). A row whose rules
// were not touched writes the base text as it is.
func t2AgreementText(display, base string, draft *r2Doc) (string, error) {
	if draft == nil {
		return base, nil
	}
	if base == "" {
		return rules.Emit(draft.document())
	}
	saved, _ := rules.ReadAll(base, display)
	if saved == nil {
		saved = &rules.Document{Path: display}
	}
	written, err := rules.Rewrite([]byte(base), saved, rules.Draft{
		Doc: draft.document(), From: draft.froms(), SectionFrom: draft.sectionFroms(),
		SaveFrom: rules.SaveFromTeamDefinition,
	})
	if err != nil {
		return "", err
	}
	return string(written), nil
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
	doc, _ := rules.ReadAll(text, out.Path)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	out.Rules = len(doc.IncludedRules()) + doc.ExcludedCount()
	out.Excluded = doc.ExcludedCount()
	out.Doc = r2DocOf(doc, out.Hash, "")
	return out
}

// checkDraft answers what is wrong with a definition that is not there yet, and
// carries what the screen draws it from (spec-teams B2, B7). The base's two
// files are laid over the catalog at the paths the row would be written to, so
// the check reads the draft as if it were saved, and a row named after a
// definition that is already there is checked as its own draft.
func (k apiTeam) checkDraft(in apiIn) (any, *apiErr) {
	s := k.s
	row, refusal := s.t2DraftRowOf(in)
	if refusal != nil {
		return nil, refusal
	}
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	name := t2DraftCheckName(in.Name)
	path := s.cat.TeamYAML(name, row.Layer)
	display := s.cat.Display(s.cat.TeamYAML(in.Name, row.Layer))
	// The row's own agreement is laid over beside its team.yaml, so the check
	// reads the draft's rules rather than the rules of a definition that already
	// holds the name: a row named after a saved definition is not told about
	// that definition's rules (spec-teams B2, 監査 R6).
	cat := s.cat.With(path, row.Team).With(s.t2AgreeFile(name, row.Layer), row.Agree)
	report := check.Team(check.TeamInput{Catalog: cat, Name: name, Layer: row.Layer, Ops: body.ops})
	findings := report.Problems
	if row.Starter {
		findings = t2WithoutStarter(findings)
	}
	def, _ := cat.Definition(name, row.Layer)
	draft, err := t2ApplyOps(row.Team, display, body.ops)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	problems := append(s.t2NameProblems(in), t2FigureProblems(findings, display)...)
	return apiTeamDraftAnswer{
		Name: in.Name, Place: in.Place, Starter: row.Starter, Path: display, Hash: source.Hash(""),
		Writes:   s.t2DraftWrites(row, in.Name),
		Problems: problems,
		Doc:      s.t2DocWithAgree(in.Name, def, s.t2AgreeOfText(in.Name, row.Layer, row.Agree)),
		Preview: apiTeamPreview{
			Lines: definition.YAMLLines(draft),
			Diff:  check.LineDiff(t2LineTexts(definition.YAMLLines(row.Team)), t2LineTexts(definition.YAMLLines(draft))),
		},
	}, nil
}

// checkDraftAgreement answers what is wrong with the rules of a definition that
// is not there yet: the same check the saved row's rules get (spec-teams C6),
// with the base team.yaml laid over the catalog so its arrows are judged against
// the draft's turns.
func (k apiTeam) checkDraftAgreement(in apiIn) (any, *apiErr) {
	s := k.s
	row, refusal := s.t2DraftRowOf(in)
	if refusal != nil {
		return nil, refusal
	}
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	if body.in.Doc == nil {
		return nil, refuse(http.StatusUnprocessableEntity, "agreement: one document is needed")
	}
	// The base's two files are laid over, so the arrows are judged against the
	// draft's turns and the diff stands against the row's own base rather than
	// the file of a definition that already holds the name (spec-teams B2, 監査
	// R6).
	cat := s.cat.With(s.cat.TeamYAML(t2DraftCheckName(in.Name), row.Layer), row.Team).
		With(s.t2AgreeFile(t2DraftCheckName(in.Name), row.Layer), row.Agree)
	report := check.Agreement(check.AgreementInput{
		Catalog: cat, Name: t2DraftCheckName(in.Name), Place: row.Layer, Draft: body.in.Doc.document(),
	})
	return r2CheckAnswer(report, false), nil
}

// saveFolder writes a definition that is not there yet, whole (spec-teams B2,
// B6, C8): its team.yaml (the base with the row's ops), its agreement.yaml (the
// base with the draft's document), and, for one made from the scaffold, the
// layer's role documents and the starter profile. A name problem, or a folder
// that has appeared since the check, stops it before anything is written.
func (k apiTeam) saveFolder(name string, in apiIn) (any, *apiErr) {
	s := k.s
	if problems := s.t2NameProblems(in); len(problems) > 0 {
		return nil, refuseAt(http.StatusUnprocessableEntity, "name", problems[0].Text)
	}
	if err := s.t2LayerDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	row, refusal := s.t2DraftRowOf(in)
	if refusal != nil {
		return nil, refusal
	}
	body, refusal := t2DecodeOps(in)
	if refusal != nil {
		return nil, refusal
	}
	display := s.cat.Display(s.cat.TeamYAML(name, row.Layer))
	team, err := t2ApplyOps(row.Team, display, body.ops)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	agree, err := t2AgreementText(s.cat.Display(s.t2AgreeFile(name, row.Layer)), row.Agree, body.in.Doc)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	files := []scaffold.File{
		{Path: "team.yaml", Data: []byte(team)},
		{Path: "agreement.yaml", Data: []byte(agree)},
	}
	if _, err := newteam.Write(newteam.In{
		Catalog: s.cat, Layer: row.Layer, Name: name, Lang: i18n.Language(),
		Files: files, Starter: row.Starter,
	}); err != nil {
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	return k.savedAnswer(name, in.Place), nil
}
