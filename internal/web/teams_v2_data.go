package web

import (
	"path/filepath"
	"sort"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// This file reads the catalog into the team screen's island (r-0f8c). It reads
// a team.yaml and an agreement.yaml and hands them to check.Team and
// check.Agreement; it makes no sentence of its own.

// t2IslandOf reads the catalog into the page's island.
func (s *Server) t2IslandOf() t2Island {
	out := t2Island{Project: filepath.Base(s.cat.Base())}
	out.Places = map[string]string{
		catalog.PlaceProject: "プロジェクト",
		catalog.PlaceGlobal:  "共通",
	}
	out.Dirs = map[string]string{
		catalog.PlaceProject: s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceProject))) + "/teams/",
		catalog.PlaceGlobal:  s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceGlobal))) + "/teams/",
	}
	out.Texts = map[string]string{
		"separator":    t("list.separator"),
		"member_rules": t("size.member_rules"),
		"member_chars": t("size.member_chars"),
	}
	if profiles, _, err := s.cat.Profiles(); err == nil {
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		out.Profiles = names
	}
	for _, listing := range s.cat.Definitions() {
		out.Defs = append(out.Defs, s.t2DefOf(listing))
		out.Files = append(out.Files, s.t2FilesOf(listing)...)
	}
	return out
}

// t2DefOf is one team definition as the list and the two tabs show it.
func (s *Server) t2DefOf(listing catalog.Listing) t2Def {
	layer, place, teamPath := t2LayerOf(listing)
	def := t2Def{
		Name:     listing.Name,
		Place:    place,
		Places:   t2PlacesOf(listing),
		TeamPath: s.cat.Display(teamPath),
	}
	agreePath := filepath.Join(filepath.Dir(teamPath), "agreement.yaml")
	def.AgreePath = s.cat.Display(agreePath)
	if text, err := s.cat.Reader().Read(source.Doc{Path: teamPath}); err == nil {
		def.Hash = source.Hash(text)
	}
	read, err := s.cat.Read(listing.Name)
	def.Broken = err != nil
	def.Team = t2TeamOf(read)
	def.SavedTurns = t2TurnsOf(read)
	def.Mission = def.Team.Mission
	def.Prompt = t2PromptOf(read, listing.Name)
	if listing.Local && listing.Global {
		hash := ""
		if text, err := s.cat.Reader().Read(source.Doc{Path: listing.GlobalPath}); err == nil {
			hash = source.Hash(text)
		}
		def.Hidden = append(def.Hidden, t2Hidden{Place: catalog.PlaceGlobal, Path: s.cat.Display(listing.GlobalPath), Hash: hash})
	}
	agreeText := ""
	if text, err := s.cat.Reader().Read(source.Doc{Path: agreePath}); err == nil {
		agreeText = text
		def.HasAgree = true
		def.AgreeHash = source.Hash(text)
		doc, _ := rules.ReadAll(text, def.AgreePath)
		if doc != nil {
			def.Agree = r2DocOf(doc, def.AgreeHash, "")
			def.Rules = len(doc.IncludedRules())
			def.Excluded = doc.ExcludedCount()
		}
	}
	team := check.Team(check.TeamInput{Catalog: s.cat, Name: listing.Name, Layer: layer})
	def.Findings, def.TurnsFindings = t2Findings(team.Graph, team.Problems, def.AgreePath)
	// The agreement's own check reads the text on disk as if the panel had it:
	// check.Agreement builds its document from Draft or Text, so the saved text
	// is what makes the panel show the file rather than an empty one.
	report := check.Agreement(check.AgreementInput{Catalog: s.cat, Name: listing.Name, Place: layer, Text: agreeText})
	def.AgreeReport = &report
	return def
}

// t2FilesOf is every team.yaml and agreement.yaml a layer holds for one name.
func (s *Server) t2FilesOf(listing catalog.Listing) []t2File {
	var out []t2File
	add := func(place, kind, path string, hidden bool) {
		if path == "" {
			return
		}
		if _, err := s.cat.Reader().Read(source.Doc{Path: path}); err != nil {
			return
		}
		out = append(out, s.t2FileOf(place, kind, listing.Name, path, hidden))
	}
	if listing.Local {
		add(catalog.PlaceProject, "team", listing.LocalPath, false)
		add(catalog.PlaceProject, "agreement", filepath.Join(filepath.Dir(listing.LocalPath), "agreement.yaml"), false)
	}
	if listing.Global {
		hidden := listing.Local
		add(catalog.PlaceGlobal, "team", listing.GlobalPath, hidden)
		agree := filepath.Join(filepath.Dir(listing.GlobalPath), "agreement.yaml")
		if hidden {
			if _, err := s.cat.Reader().Read(source.Doc{Path: filepath.Join(filepath.Dir(listing.LocalPath), "agreement.yaml")}); err != nil {
				hidden = false
			}
		}
		add(catalog.PlaceGlobal, "agreement", agree, hidden)
	}
	return out
}

// t2FileOf is one file of the YAML tab, with the problems its own check finds.
func (s *Server) t2FileOf(place, kind, name, path string, hidden bool) t2File {
	layer := catalog.Layer(place)
	display := s.cat.Display(path)
	text, _ := s.cat.Reader().Read(source.Doc{Path: path})
	file := t2File{
		File: name + "." + place + "." + kind, Place: place, Name: name, Kind: kind,
		Path: display, Hash: source.Hash(text), Text: text, Hidden: hidden,
	}
	switch kind {
	case "team":
		findings := check.Definition(check.Input{Catalog: s.cat.With(path, text), Name: name, Layer: layer})
		for _, finding := range findings {
			sev := "warn"
			if finding.Stops {
				sev = "err"
				file.Broken = true
			}
			file.Problems = append(file.Problems, t2YamlProblem{Sev: sev, Field: finding.Field, Text: finding.Text})
		}
	default:
		report := check.Agreement(check.AgreementInput{Catalog: s.cat, Name: name, Place: layer, Text: text})
		for _, problem := range report.Problems {
			file.Problems = append(file.Problems, t2YamlProblem{Sev: problem.Sev, Field: problem.Field, Line: problem.Line, Text: problem.Text})
		}
	}
	return file
}

// t2LayerOf is the layer a listing resolves to, the word the screen uses, and
// the team.yaml it stands in.
func t2LayerOf(listing catalog.Listing) (layer, place, path string) {
	if listing.Local {
		return source.LayerLocal, catalog.PlaceProject, listing.LocalPath
	}
	return source.LayerGlobal, catalog.PlaceGlobal, listing.GlobalPath
}

// t2PlacesOf is every layer this name is in, the project's first.
func t2PlacesOf(listing catalog.Listing) []string {
	var out []string
	if listing.Local {
		out = append(out, catalog.PlaceProject)
	}
	if listing.Global {
		out = append(out, catalog.PlaceGlobal)
	}
	return out
}

// t2ApplyOps is the saved team.yaml with a panel's ops applied, for the
// catalog overlay an agreement check reads through.
func (s *Server) t2ApplyOps(name, place string, ops []t2Op) (string, error) {
	layer := catalog.Layer(place)
	path := s.cat.TeamYAML(name, layer)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return "", err
	}
	edits, err := t2Ops(ops)
	if err != nil {
		return "", err
	}
	for _, op := range edits {
		next, err := definition.EditYAML([]byte(text), s.cat.Display(path), op)
		if err != nil {
			return "", err
		}
		text = string(next)
	}
	return text, nil
}
