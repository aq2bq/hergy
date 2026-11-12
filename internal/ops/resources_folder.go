package ops

import (
	"errors"
	"os"
	"path/filepath"

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

// DefinitionDraft is the base of a new or copied folder. AgreementExists is
// distinct from empty text, so absence never silently creates a new document.
type DefinitionDraft struct {
	Team, Agreement string
	AgreementExists bool
}

// ReadDefinitionDraft reads or changes the selected resource.
func ReadDefinitionDraft(cat *catalog.Catalog, target ResourceTarget) (DefinitionDraft, *ResourceFailure) {
	s := ResourceScope{Catalog: cat}
	team, bad := s.ReadResource(target, ResourceMain)
	if bad != nil {
		return DefinitionDraft{}, &ResourceFailure{Missing: true, Text: i18n.Tf("error.copy_base_unreadable", cat.Display(bad.Path))}
	}
	if !team.Exists {
		return DefinitionDraft{}, &ResourceFailure{Text: i18n.Tf("error.copy_base_unreadable", cat.Display(team.Path)), Missing: true}
	}
	agree, bad := s.ReadResource(target, ResourceAgreement)
	if bad != nil {
		return DefinitionDraft{}, bad
	}
	return DefinitionDraft{Team: team.Text, Agreement: agree.Text, AgreementExists: agree.Exists}, nil
}

// definitionFolder carries the kind-specific plan, never a caller-supplied write
// function. The operation validates every planned file before writing the folder.
type definitionFolder struct {
	Draft                                 DefinitionDraft
	Ops                                   []definition.Op
	Agreement                             *string
	Starter, StarterRoles, StarterProfile bool
	Carry                                 *newteam.Carry
}

// createDefinition reads or changes the selected resource.
func (s ResourceScope) createDefinition(ref domainplace.Ref, in definitionFolder) (ResourceSaved, *ResourceFailure) {
	if ref.Kind() != domainplace.Definition {
		return ResourceSaved{}, &ResourceFailure{Text: "not a definition"}
	}
	if _, err := domainname.NewDefinitionName(ref.Name()); err != nil {
		return ResourceSaved{}, &ResourceFailure{Field: "name", Text: i18n.Tf("error.new_name_pattern", ref.Name())}
	}
	dir := filepath.Dir(s.Catalog.TeamYAML(ref.Name(), ref.Place().Layer()))
	var written newteam.In
	spec := resourceDefinition{
		read: func() (ResourceFile, *ResourceFailure) {
			if _, err := os.Stat(dir); err == nil {
				return ResourceFile{}, &ResourceFailure{Field: "name", Text: i18n.Tf("error.new_exists", ref.Name(), s.Catalog.Display(dir))}
			} else if !errors.Is(err, os.ErrNotExist) {
				return ResourceFile{}, &ResourceFailure{Text: err.Error()}
			}
			if bad := s.layerReady(ref); bad != nil {
				return ResourceFile{}, bad
			}
			return ResourceFile{Path: filepath.Join(dir, "team.yaml"), Hash: source.Hash("")}, nil
		},
		edit: func(before ResourceFile) (string, *ResourceFailure) {
			text := in.Draft.Team
			for _, op := range in.Ops {
				out, err := definition.EditYAML([]byte(text), s.Catalog.Display(before.Path), op)
				if err != nil {
					return "", &ResourceFailure{Text: err.Error()}
				}
				text = string(out)
			}
			return text, nil
		},
		check: func(_ ResourceFile, text string) *ResourceFailure {
			agree := in.Draft.Agreement
			if in.Agreement != nil {
				agree = *in.Agreement
			}
			files := []scaffold.File{{Path: "team.yaml", Data: []byte(text)}}
			if in.Draft.AgreementExists || in.Agreement != nil {
				files = append(files, scaffold.File{Path: "agreement.md", Data: []byte(agree)})
			}
			written = newteam.In{Catalog: s.Catalog, Layer: ref.Place().Layer(), Name: ref.Name(), Lang: i18n.Language(), Files: files, Starter: in.Starter, StarterRoles: in.StarterRoles, StarterProfile: in.StarterProfile, Carry: in.Carry}
			planned, err := newteam.Overlay(written)
			if err != nil {
				return &ResourceFailure{Text: err.Error()}
			}
			// A newly created resource has no grandfathered stopping problems.
			for _, finding := range check.Team(check.TeamInput{Catalog: planned, Name: ref.Name(), Layer: ref.Place().Layer()}).Problems {
				if finding.Stops {
					return &ResourceFailure{Text: finding.Text}
				}
			}
			for _, problem := range check.Agreement(check.AgreementInput{Catalog: planned, Name: ref.Name(), Place: ref.Place().Layer(), Text: agree}).Problems {
				if problem.Sev == "err" {
					return &ResourceFailure{Text: problem.Text}
				}
			}
			return nil
		},
		write: func(_ ResourceFile, _ string) *ResourceFailure {
			if _, err := newteam.Write(written); err != nil {
				return &ResourceFailure{Text: err.Error(), Internal: true}
			}
			return nil
		},
	}
	return saveResource(source.Hash(""), false, spec)
}
