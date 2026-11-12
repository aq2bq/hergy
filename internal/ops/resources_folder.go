package ops

import (
	"errors"
	"os"
	"path/filepath"
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
	Draft     ResourceDraft
	Input     ResourceDraftInput
	Ops       []definition.Op
	Agreement *string
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
			text, err := editDefinitionText(in.Draft.Team, s.Catalog.Display(before.Path), in.Ops)
			if err != nil {
				return "", &ResourceFailure{Text: err.Error()}
			}
			return text, nil
		},
		check: func(_ ResourceFile, text string) *ResourceFailure {
			var bad *ResourceFailure
			written, bad = s.checkDefinitionFolder(ref, in, text)
			return bad
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

func (s ResourceScope) checkDefinitionFolder(ref domainplace.Ref, in definitionFolder, text string) (newteam.In, *ResourceFailure) {
	agree := in.Draft.Agree
	if in.Agreement != nil {
		agree = *in.Agreement
	}
	files := []scaffold.File{{Path: "team.yaml", Data: []byte(text)}}
	if in.Draft.AgreementExists || in.Agreement != nil {
		files = append(files, scaffold.File{Path: "agreement.md", Data: []byte(agree)})
	}
	written := newteam.In{Catalog: s.Catalog, Layer: ref.Place().Layer(), Name: ref.Name(), Lang: i18n.Language(), Files: files, Starter: in.Draft.Starter, StarterRoles: in.Draft.StarterRoles, StarterProfile: in.Draft.StarterProfile, Carry: s.carryIn(in.Input)}
	planned, err := newteam.Overlay(written)
	if err != nil {
		return written, &ResourceFailure{Text: err.Error()}
	}
	// The screen compares the checked starter/copy basis with its edits.
	// Keep its existing errors, including missing unchosen dependencies;
	// only a new stopping sentence outside node: closes the save.
	baseline, bad := s.checkDefinitionDraft(in.Input, in.Draft, nil)
	if bad != nil {
		return written, bad
	}
	edited, bad := s.checkDefinitionDraft(in.Input, in.Draft, in.Ops)
	if bad != nil {
		return written, bad
	}
	known := map[string]bool{}
	for _, finding := range baseline.Findings {
		if finding.Stops {
			known[finding.Text] = true
		}
	}
	for _, finding := range edited.Findings {
		if finding.Stops && !strings.HasPrefix(finding.Field, "node:") && !known[finding.Text] {
			return written, &ResourceFailure{Text: finding.Text}
		}
	}
	for _, problem := range check.Agreement(check.AgreementInput{Catalog: planned, Name: ref.Name(), Place: ref.Place().Layer(), Text: agree}).Problems {
		if problem.Sev == "err" {
			return written, &ResourceFailure{Text: problem.Text}
		}
	}
	return written, nil
}
