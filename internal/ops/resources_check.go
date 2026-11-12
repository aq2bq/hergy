package ops

import (
	"github.com/aq2bq/hergy/internal/definition"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/check"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// ResourceInspection accepts draft names: checking never creates a model name.
type ResourceInspection struct {
	Kind                 domainplace.Kind
	Place                domainplace.Place
	Name, Original, Text string
	WholeText            bool
	Ops                  []definition.Op
	Effect               *definition.Op
	Profile              *profile.Draft
	Part                 ResourcePart
}

// ResourceChecked is the checker's complete result before transport projection.
type ResourceChecked struct {
	Team                 check.TeamReport
	Document             check.DocReport
	Profiles             []profile.Problem
	Effect               *check.EffectReport
	Definition           *definition.Definition
	Before, Edited, Path string
	Size, Limit          int
	Catalog              *catalog.Catalog
}

// CheckResource inspects a complete draft with the same kind-specific validators
// saving uses. Input names can be incomplete while a person is typing.
func (s ResourceScope) CheckResource(in ResourceInspection) (ResourceChecked, *ResourceFailure) {
	switch in.Kind {
	case domainplace.Role:
		return ResourceChecked{Document: check.Role(check.RoleInput{Catalog: s.Catalog, Place: in.Place.Layer(), Name: in.Name, Original: in.Original, Text: in.Text})}, nil
	case domainplace.Profile:
		path := s.Catalog.ProjectProfileFile()
		if in.Place.IsGlobal() {
			path = s.Catalog.SharedProfileFile()
		}
		var problems []profile.Problem
		if in.Profile != nil {
			text, _ := s.Catalog.Reader().Read(source.Doc{Path: path})
			entries, _ := profile.ReadAll(text, s.Catalog.Display(path))
			problems = profile.CheckDraft(*in.Profile, entries, in.Original)
		} else {
			_, problems = profile.ReadAll(in.Text, s.Catalog.Display(path))
		}
		return ResourceChecked{Profiles: problems, Path: path}, nil
	case domainplace.Definition:
		return s.checkDefinition(in)
	}
	return ResourceChecked{}, &ResourceFailure{Text: "unknown resource kind"}
}
func (s ResourceScope) checkDefinition(in ResourceInspection) (ResourceChecked, *ResourceFailure) {
	path := s.Catalog.TeamYAML(in.Name, in.Place.Layer())
	out := ResourceChecked{Path: path, Limit: check.TeamCharLimit}
	text, readErr := s.Catalog.Reader().Read(source.Doc{Path: path})
	if readErr != nil && in.Part == ResourceAgreement && !in.WholeText {
		return out, &ResourceFailure{Text: readErr.Error()}
	}
	out.Before = text
	edited, err := editDefinitionText(text, s.Catalog.Display(path), in.Ops)
	if err != nil {
		return out, &ResourceFailure{Text: err.Error()}
	}
	cat := s.Catalog.With(path, edited)
	out.Edited = edited
	out.Catalog = cat
	if in.Part == ResourceAgreement {
		return ResourceChecked{Document: check.Agreement(check.AgreementInput{Catalog: cat, Name: in.Name, Place: in.Place.Layer(), Text: in.Text}), Catalog: cat}, nil
	}
	if in.WholeText {
		cat = s.Catalog.With(path, in.Text)
		out.Edited = in.Text
		out.Catalog = cat
	}
	out.Team = check.Team(check.TeamInput{Catalog: cat, Name: in.Name, Layer: in.Place.Layer()})
	out.Definition, _ = cat.Definition(in.Name, in.Place.Layer())
	if out.Definition != nil {
		out.Size = check.TeamSizeChars(cat, out.Definition)
	}
	if in.Effect != nil {
		report := check.TeamEffect(check.TeamInput{Catalog: s.Catalog, Name: in.Name, Layer: in.Place.Layer(), Ops: in.Ops}, *in.Effect)
		out.Effect = &report
	}
	return out, nil
}
func editDefinitionText(text, display string, ops []definition.Op) (string, error) {
	for _, op := range ops {
		out, err := definition.EditYAML([]byte(text), display, op)
		if err != nil {
			return "", err
		}
		text = string(out)
	}
	return text, nil
}
