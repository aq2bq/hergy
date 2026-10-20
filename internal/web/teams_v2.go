package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the team-definition screen's own answers (r-0f8c): the page the
// two tabs are drawn on, the checks a panel sends while it is edited, and the
// writes of team.yaml, agreement.yaml and the YAML tab. Every check is
// check.Team or check.Agreement (spec-teams P1, X1): the screen makes no rule of
// its own. Every write carries the hash of the text the page read.

// t2Page draws one tab of the team screen. The client script draws the tabs,
// the rows, the figure and the agreement from the data island.
func (s *Server) t2Page(w http.ResponseWriter, r *http.Request, tab string) {
	raw, err := json.Marshal(s.t2IslandOf())
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data := t2PageData{
		Lang: i18n.Language(), Nav: s.navOf("teams"), Tab: tab,
		Token: s.token, Data: template.JS(raw), //nolint:gosec // raw is json.Marshal output, which escapes <, > and &, so it cannot close the script element that carries it
	}
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "t2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// t2LayerDir stops a write that would make a layer that is not there: a project
// whose .hergy is gone has no place to put a team definition (spec 1.7, C5).
func (s *Server) t2LayerDir(place string) error {
	dir := s.cat.LayerDir(catalog.Layer(place))
	if _, err := os.Stat(dir); err != nil {
		return i18n.Errorf("error.teams_no_layer", s.cat.Display(dir))
	}
	return nil
}

// t2AgreePath is a team's own agreement.yaml, as a person reads the path.
func (s *Server) t2AgreePath(name, layer string) string {
	return s.cat.Display(filepath.Join(filepath.Dir(s.cat.TeamYAML(name, layer)), "agreement.yaml"))
}

// t2Problems maps check's findings to the shape a panel shows.
func t2Problems(findings []check.Finding) []t2Finding {
	out := make([]t2Finding, 0, len(findings))
	for _, finding := range findings {
		sev := "warn"
		if finding.Stops {
			sev = "err"
		}
		out = append(out, t2Finding{Sev: sev, Field: finding.Field, Worker: -1, Text: finding.Text})
	}
	return out
}

// t2CheckIn is what /teams/check carries.
type t2CheckIn struct {
	Definition string `json:"definition"`
	Place      string `json:"place"`
	Ops        []t2Op `json:"ops"`
}

// t2Check answers what is wrong with the team definition a panel holds: the
// saved text with the panel's ops applied, checked as if it were saved (C2).
func (s *Server) t2Check(r *http.Request) p2Answer {
	var in t2CheckIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	ops, err := t2Ops(in.Ops)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	team := check.Team(check.TeamInput{Catalog: s.cat, Name: in.Definition, Layer: layer, Ops: ops})
	findings, turnsFindings := t2Findings(team.Graph, team.Problems, s.t2AgreePath(in.Definition, layer))
	return p2JSONAnswer(map[string]any{
		"findings": findings, "turnsFindings": turnsFindings,
		"graph": team.Graph, "counts": team.Counts,
	})
}

// t2EffectIn is what /teams/effect carries: the ops a panel holds, and the one
// more op to try (spec 2, C4).
type t2EffectIn struct {
	Definition string `json:"definition"`
	Place      string `json:"place"`
	Ops        []t2Op `json:"ops"`
	Op         t2Op   `json:"op"`
}

// t2Effect answers what one more op would do: the lines the prompt moves, and
// the problems it would add (spec 1.4.4, X3).
func (s *Server) t2Effect(r *http.Request) p2Answer {
	var in t2EffectIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	ops, err := t2Ops(in.Ops)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	op, err := in.Op.op()
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	report := check.TeamEffect(check.TeamInput{Catalog: s.cat, Name: in.Definition, Layer: layer, Ops: ops}, op)
	return p2JSONAnswer(map[string]any{"lines": report.Lines, "problems": t2Problems(report.Problems)})
}

// t2SaveIn is what /teams/save carries.
type t2SaveIn struct {
	Definition string `json:"definition"`
	Place      string `json:"place"`
	Ops        []t2Op `json:"ops"`
	Hash       string `json:"hash"`
}

// t2Save writes a panel's ops into its team.yaml, in one pass: every line the
// ops do not touch comes back byte for byte. It is refused only when a problem
// the saved file did not hold has appeared (spec 1.3, X1).
func (s *Server) t2Save(r *http.Request) p2Answer {
	var in t2SaveIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := s.t2LayerDir(in.Place); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	ops, err := t2Ops(in.Ops)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	path := s.cat.TeamYAML(in.Definition, layer)
	display := s.cat.Display(path)
	text, readErr := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if readErr == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	before := check.Team(check.TeamInput{Catalog: s.cat, Name: in.Definition, Layer: layer})
	after := check.Team(check.TeamInput{Catalog: s.cat, Name: in.Definition, Layer: layer, Ops: ops})
	known := map[string]bool{}
	for _, finding := range before.Problems {
		if finding.Stops {
			known[finding.Text] = true
		}
	}
	for _, finding := range after.Problems {
		if finding.Stops && !known[finding.Text] {
			return p2TextAnswer(http.StatusUnprocessableEntity, finding.Text)
		}
	}
	edited := text
	for _, op := range ops {
		next, err := definition.EditYAML([]byte(edited), display, op)
		if err != nil {
			return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
		}
		edited = string(next)
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return r2WriteError(err, display)
	}
	findings, turnsFindings := t2Findings(after.Graph, after.Problems, s.t2AgreePath(in.Definition, layer))
	return p2JSONAnswer(map[string]any{
		"ok": true, "hash": source.Hash(edited), "text": edited,
		"findings": findings, "turnsFindings": turnsFindings,
		"graph": after.Graph, "counts": after.Counts,
	})
}

// t2AgreeCheckIn is what /teams/agreement/check carries: the ops the figure
// holds, and the agreement document as the panel has it.
type t2AgreeCheckIn struct {
	Definition string `json:"definition"`
	Place      string `json:"place"`
	Ops        []t2Op `json:"ops"`
	Doc        r2Doc  `json:"doc"`
}

// t2AgreeCheck answers what is wrong with a team's agreement, checked with the
// figure's draft laid over the team.yaml (C6, spec 1.5).
func (s *Server) t2AgreeCheck(r *http.Request) p2Answer {
	var in t2AgreeCheckIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	teamText, err := s.t2ApplyOps(in.Definition, in.Place, in.Ops)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	overlay := s.cat.With(s.cat.TeamYAML(in.Definition, layer), teamText)
	report := check.Agreement(check.AgreementInput{Catalog: overlay, Name: in.Definition, Place: layer, Draft: in.Doc.document()})
	return p2JSONAnswer(report)
}

// t2AgreeSaveIn is what /teams/agreement/save carries.
type t2AgreeSaveIn struct {
	Definition string `json:"definition"`
	Place      string `json:"place"`
	Doc        r2Doc  `json:"doc"`
	Hash       string `json:"hash"`
}

// t2AgreeSave writes a team's agreement.yaml as the panel has it, with the same
// one pass a role document's save uses, so an untouched line travels.
func (s *Server) t2AgreeSave(r *http.Request) p2Answer {
	var in t2AgreeSaveIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := s.t2LayerDir(in.Place); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(in.Place)
	path := filepath.Join(s.cat.LayerDir(layer), "teams", in.Definition, "agreement.yaml")
	display := s.cat.Display(path)
	text, readErr := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if readErr == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	draft := in.Doc.document()
	var edited []byte
	var err error
	if readErr != nil {
		edited, err = emitNewAgreement(draft)
	} else {
		saved, _ := rules.ReadAll(text, display)
		if saved == nil {
			saved = &rules.Document{Path: path}
		}
		edited, err = rules.Rewrite([]byte(text), saved, rules.Draft{Doc: draft, From: in.Doc.froms(), SectionFrom: in.Doc.sectionFroms()})
	}
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := source.WriteIfUnchanged(path, hash, string(edited)); err != nil {
		return r2WriteError(err, display)
	}
	doc, _ := rules.ReadAll(string(edited), display)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	return p2JSONAnswer(map[string]any{
		"ok": true, "hash": source.Hash(string(edited)), "text": string(edited),
		"rules": len(doc.Rules()), "chars": rules.CharCount(doc.Markdown()),
		"doc": r2DocOf(doc, source.Hash(string(edited)), ""),
	})
}

// emitNewAgreement writes a team's new agreement.yaml from the draft alone.
func emitNewAgreement(doc *rules.Document) ([]byte, error) {
	text, err := rules.Emit(doc)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

// t2YamlIn is what the YAML tab's checks and saves carry.
type t2YamlIn struct {
	File string `json:"file"`
	Text string `json:"text"`
	Hash string `json:"hash"`
}

// t2SplitFile reads a YAML tab file name: "<name>.<project|global>.<team|agreement>".
func t2SplitFile(file string) (name, place, kind string, ok bool) {
	i := strings.LastIndexByte(file, '.')
	if i < 0 {
		return "", "", "", false
	}
	kind, rest := file[i+1:], file[:i]
	j := strings.LastIndexByte(rest, '.')
	if j < 0 {
		return "", "", "", false
	}
	place, name = rest[j+1:], rest[:j]
	if name == "" || (kind != "team" && kind != "agreement") || (place != catalog.PlaceProject && place != catalog.PlaceGlobal) {
		return "", "", "", false
	}
	return name, place, kind, true
}

// t2YamlPath is the file a YAML tab name stands for.
func (s *Server) t2YamlPath(name, place, kind string) string {
	path := s.cat.TeamYAML(name, catalog.Layer(place))
	if kind == "agreement" {
		path = filepath.Join(filepath.Dir(path), "agreement.yaml")
	}
	return path
}

// t2YamlCheck answers what is wrong with the YAML a person is typing: the same
// check, read from the text (spec 1.8).
func (s *Server) t2YamlCheck(r *http.Request) p2Answer {
	var in t2YamlIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	name, place, kind, ok := t2SplitFile(in.File)
	if !ok {
		return p2TextAnswer(http.StatusNotFound, t("web.error.not_found"))
	}
	layer := catalog.Layer(place)
	var problems []t2YamlProblem
	if kind == "team" {
		path := s.cat.TeamYAML(name, layer)
		findings := check.Definition(check.Input{Catalog: s.cat.With(path, in.Text), Name: name, Layer: layer})
		for _, finding := range findings {
			sev := "warn"
			if finding.Stops {
				sev = "err"
			}
			problems = append(problems, t2YamlProblem{Sev: sev, Field: finding.Field, Text: finding.Text})
		}
	} else {
		report := check.Agreement(check.AgreementInput{Catalog: s.cat, Name: name, Place: layer, Text: in.Text})
		for _, problem := range report.Problems {
			problems = append(problems, t2YamlProblem{Sev: problem.Sev, Field: problem.Field, Line: problem.Line, Text: problem.Text})
		}
	}
	return p2JSONAnswer(map[string]any{"problems": problems})
}

// t2YamlSave writes one file's text as the person wrote it. It is refused only
// when a problem the saved file did not have has appeared.
func (s *Server) t2YamlSave(r *http.Request) p2Answer {
	var in t2YamlIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	name, place, kind, ok := t2SplitFile(in.File)
	if !ok {
		return p2TextAnswer(http.StatusNotFound, t("web.error.not_found"))
	}
	if err := s.t2LayerDir(place); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	layer := catalog.Layer(place)
	path := s.t2YamlPath(name, place, kind)
	display := s.cat.Display(path)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if err == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	if kind == "team" {
		if newError := t2NewTeamError(s.cat, path, name, layer, text, in.Text); newError != "" {
			return p2TextAnswer(http.StatusUnprocessableEntity, newError)
		}
	} else if newError := t2NewRuleError(display, text, in.Text); newError != "" {
		return p2TextAnswer(http.StatusUnprocessableEntity, newError)
	}
	if err := source.WriteIfUnchanged(path, hash, in.Text); err != nil {
		return r2WriteError(err, display)
	}
	return p2JSONAnswer(map[string]any{"ok": true, "hash": source.Hash(in.Text)})
}

// t2NewTeamError is the first error the typed team.yaml holds that the saved
// one did not, or "".
func t2NewTeamError(cat *catalog.Catalog, path, name, layer, before, after string) string {
	known := map[string]bool{}
	for _, finding := range check.Definition(check.Input{Catalog: cat.With(path, before), Name: name, Layer: layer}) {
		if finding.Stops {
			known[finding.Text] = true
		}
	}
	for _, finding := range check.Definition(check.Input{Catalog: cat.With(path, after), Name: name, Layer: layer}) {
		if finding.Stops && !known[finding.Text] {
			return finding.Text
		}
	}
	return ""
}

// t2NewRuleError is the first rule file error the typed text holds that the
// saved one did not, or "".
func t2NewRuleError(display, before, after string) string {
	_, saved := rules.ReadAll(before, display)
	known := map[string]bool{}
	for _, problem := range saved {
		if problem.Sev == rules.SevError {
			known[problem.Text] = true
		}
	}
	_, typed := rules.ReadAll(after, display)
	for _, problem := range typed {
		if problem.Sev == rules.SevError && !known[problem.Text] {
			return problem.Text
		}
	}
	return ""
}
