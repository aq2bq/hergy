package ops

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/resourcewrite"
	"github.com/aq2bq/hergy/internal/source"
)

// DefinitionRenamePreview is the shared resource model.
type DefinitionRenamePreview struct {
	From, To string
	Notes    []string
}

// PreviewDefinitionRename reads or changes the selected resource.
func (s ResourceScope) PreviewDefinitionRename(old SeenResource, next domainplace.Ref) (*DefinitionRenamePreview, *ResourceFailure) {
	if domainname.CheckRenameSource(old.Name()) != nil || old.Name() == next.Name() {
		return nil, &ResourceFailure{Field: "name", Text: i18n.T("web.rename.name")}
	}
	if _, err := domainname.NewDefinitionName(next.Name()); err != nil {
		return nil, &ResourceFailure{Field: "name", Text: i18n.T("web.rename.name")}
	}
	cat := s.Catalog
	if next.Place().IsGlobal() && s.Global != nil {
		cat = s.Global
	}
	from := cat.TeamYAML(old.Name(), old.Place().Layer())
	to := cat.TeamYAML(next.Name(), next.Place().Layer())
	if _, err := os.Lstat(filepath.Dir(to)); err == nil {
		return nil, &ResourceFailure{Status: 409, Text: i18n.T("web.rename.taken")}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, &ResourceFailure{Internal: true, Text: err.Error()}
	}
	out := &DefinitionRenamePreview{From: cat.Display(from), To: cat.Display(to), Notes: []string{i18n.T("web.rename.history")}}
	if !next.Place().IsGlobal() {
		if cat.HoldsDefinition(old.Name(), source.LayerGlobal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.reveals", old.Name()))
		}
		if cat.HoldsDefinition(next.Name(), source.LayerGlobal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.hides", next.Name()))
		}
		return out, nil
	}
	out.Notes = append(out.Notes, i18n.T("web.rename.global"))
	if !s.Registered || s.Projects == nil {
		return nil, &ResourceFailure{Status: 503, Text: i18n.T("web.rename.unavailable")}
	}
	cats, err := s.Projects()
	if err != nil {
		return nil, &ResourceFailure{Status: 503, Text: err.Error()}
	}
	for _, one := range cats {
		if one.HoldsDefinition(next.Name(), source.LayerLocal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.shadowed", filepath.Base(one.Base().String()), next.Name()))
		}
	}
	return out, nil
}

// DefinitionRename is the shared resource model.
type DefinitionRename struct {
	Hash, AgreementHash string
	AgreementExists     bool
}

// RenameDefinition reads or changes the selected resource.
func (s ResourceScope) RenameDefinition(old SeenResource, next domainplace.Ref, in DefinitionRename) (*DefinitionRenamePreview, *ResourceFailure) {
	if _, bad := s.PreviewDefinitionRename(old, next); bad != nil {
		return nil, bad
	}
	cat := s.Catalog
	if next.Place().IsGlobal() && s.Global != nil {
		cat = s.Global
	}
	oldPath, newPath := cat.TeamYAML(old.Name(), old.Place().Layer()), cat.TeamYAML(next.Name(), next.Place().Layer())
	release, err := source.LockTeams(oldPath, newPath)
	if err != nil {
		return nil, &ResourceFailure{Status: 409, Text: i18n.T("web.rename.busy")}
	}
	defer release()
	preview, bad := s.PreviewDefinitionRename(old, next)
	if bad != nil {
		return nil, bad
	}
	text, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, &ResourceFailure{Status: 409, Text: i18n.T("web.rename.read_again")}
	}
	agree, agreeErr := os.ReadFile(filepath.Join(filepath.Dir(oldPath), "agreement.md"))
	if agreeErr != nil && !errors.Is(agreeErr, os.ErrNotExist) {
		return nil, &ResourceFailure{Internal: true, Text: agreeErr.Error()}
	}
	if source.Hash(string(text)) != in.Hash || source.Hash(string(agree)) != in.AgreementHash || (agreeErr == nil) != in.AgreementExists {
		return nil, &ResourceFailure{Status: 409, Text: i18n.T("web.rename.read_again")}
	}
	if s.RenameChecker == nil {
		return nil, &ResourceFailure{Status: 503, Text: i18n.T("web.rename.unavailable")}
	}
	if err := s.RenameChecker(cat.TeamPath(old.Name(), old.Place().Layer())); err != nil {
		if hq, ok := errors.AsType[*HQError](err); ok {
			return nil, &ResourceFailure{Status: hq.Status, Text: hq.Text}
		}
		return nil, &ResourceFailure{Status: 503, Text: err.Error()}
	}
	edited, err := definition.RenameYAML(text, cat.Display(oldPath), next.Name())
	if err != nil {
		return nil, &ResourceFailure{Text: err.Error()}
	}
	trial := cat.With(newPath, string(edited)).Without(oldPath)
	if agreeErr == nil {
		trial = trial.With(filepath.Join(filepath.Dir(newPath), "agreement.md"), string(agree))
	}
	def, err := trial.Definition(next.Name(), next.Place().Layer())
	if err != nil {
		return nil, &ResourceFailure{Text: err.Error()}
	}
	renamed, _ := domainname.NewDefinitionName(next.Name())
	names := domainname.DefinitionNames(renamed)
	for _, m := range def.Graph.Members {
		if _, err := names.MemberTarget(m.Name); err != nil {
			return nil, &ResourceFailure{Field: "name", Text: i18n.Tf("error.herdr_name_pattern", names.Member(m.Name))}
		}
	}
	if err := moveDefinition(oldPath, newPath, in.Hash, text, edited); err != nil {
		return nil, &ResourceFailure{Internal: true, Text: err.Error()}
	}
	return preview, nil
}
func moveDefinition(oldPath, newPath, hash string, before, after []byte) error {
	if err := resourcewrite.WriteIfUnchanged(oldPath, hash, string(after)); err != nil {
		return err
	}
	if err := resourcewrite.RenameDirectory(filepath.Dir(oldPath), filepath.Dir(newPath)); err != nil {
		return errors.Join(err, resourcewrite.WriteIfUnchanged(oldPath, source.Hash(string(after)), string(before)))
	}
	return nil
}
