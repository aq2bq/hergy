package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

type apiRenamePreview struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Notes []string `json:"notes"`
}

func (s *Server) renamePreview(old, next, place string) (*apiRenamePreview, *apiErr) {
	oldErr := domainname.CheckRenameSource(old)
	_, nextErr := domainname.NewDefinitionName(next)
	if oldErr != nil || nextErr != nil || old == next {
		return nil, refuseAt("name", i18n.T("web.rename.name"))
	}
	if place != "project" && place != "global" {
		return nil, refuseAt("place", i18n.T("web.rename.place"))
	}
	cat := s.cat
	if place == "global" {
		cat = s.globalCatalog()
	}
	from := cat.TeamYAML(old, catalog.Layer(place))
	to := cat.TeamYAML(next, catalog.Layer(place))
	if _, err := os.Lstat(filepath.Dir(to)); err == nil {
		return nil, refuse(http.StatusConflict, i18n.T("web.rename.taken"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	out := &apiRenamePreview{From: cat.Display(from), To: cat.Display(to), Notes: []string{i18n.T("web.rename.history")}}
	if place == "project" {
		if cat.HoldsDefinition(old, source.LayerGlobal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.reveals", old))
		}
		if cat.HoldsDefinition(next, source.LayerGlobal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.hides", next))
		}
		return out, nil
	}
	out.Notes = append(out.Notes, i18n.T("web.rename.global"))
	if s.hq == nil {
		return nil, refuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	projects, err := s.hq.Projects()
	if err != nil {
		return nil, refuse(http.StatusServiceUnavailable, err.Error())
	}
	for _, project := range hqShownProjects(projects) {
		one, refusal := s.catalogAt(project.Path)
		if refusal != nil {
			return nil, refusal
		}
		if one.HoldsDefinition(next, source.LayerLocal) {
			out.Notes = append(out.Notes, i18n.Tf("web.rename.shadowed", project.Name, next))
		}
	}
	return out, nil
}

func (s *Server) apiTeamRename(w http.ResponseWriter, r *http.Request) {
	in, ok := s.apiRead(w, r)
	if !ok {
		return
	}
	scoped, refusal := s.withProject(in.Project)
	if refusal != nil {
		apiAnswer(w, nil, refusal)
		return
	}
	value, refusal := scoped.renameTeam(r.PathValue("name"), in)
	apiAnswer(w, value, refusal)
}

func (s *Server) renameTeam(old string, in apiIn) (any, *apiErr) {
	if in.RenameTo == nil || in.AgreementExists == nil || in.Hash == "" || in.AgreementHash == "" {
		return nil, refuseAt("name", i18n.T("web.rename.read_again"))
	}
	_, refusal := s.renamePreview(old, *in.RenameTo, in.Place)
	if refusal != nil {
		return nil, refusal
	}
	cat := s.cat
	if in.Place == "global" {
		cat = s.globalCatalog()
	}
	oldPath, newPath := cat.TeamYAML(old, catalog.Layer(in.Place)), cat.TeamYAML(*in.RenameTo, catalog.Layer(in.Place))
	release, err := source.LockTeams(oldPath, newPath)
	if err != nil {
		return nil, refuse(http.StatusConflict, i18n.T("web.rename.busy"))
	}
	defer release()
	preview, refusal := s.renamePreview(old, *in.RenameTo, in.Place)
	if refusal != nil {
		return nil, refusal
	}
	//nolint:gosec // G703: renamePreview validated both names and place; the catalog supplies the configured root.
	text, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, refuse(http.StatusConflict, i18n.T("web.rename.read_again"))
	}
	//nolint:gosec // G703: oldPath is validated above, and the second filename is constant.
	agree, agreeErr := os.ReadFile(filepath.Join(filepath.Dir(oldPath), "agreement.md"))
	if agreeErr != nil && !errors.Is(agreeErr, os.ErrNotExist) {
		return nil, refuse(http.StatusInternalServerError, agreeErr.Error())
	}
	if source.Hash(string(text)) != in.Hash || source.Hash(string(agree)) != in.AgreementHash || (agreeErr == nil) != *in.AgreementExists {
		return nil, refuse(http.StatusConflict, i18n.T("web.rename.read_again"))
	}
	checker, ok := s.act.(interface{ CheckDefinitionRename(string) error })
	if !ok {
		return nil, refuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	if err := checker.CheckDefinitionRename(oldPath); err != nil {
		if hq, yes := errors.AsType[*HQError](err); yes {
			return nil, refuse(hq.Status, hq.Text)
		}
		return nil, refuse(http.StatusServiceUnavailable, err.Error())
	}
	edited, err := definition.RenameYAML(text, cat.Display(oldPath), *in.RenameTo)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	trial := cat.With(newPath, string(edited)).Without(oldPath)
	if agreeErr == nil {
		trial = trial.With(filepath.Join(filepath.Dir(newPath), "agreement.md"), string(agree))
	}
	def, err := trial.Definition(*in.RenameTo, catalog.Layer(in.Place))
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	for i := range def.Graph.Members {
		if _, err := domainname.MemberForInput(*in.RenameTo, def.Graph.Members[i].Name); err != nil {
			return nil, refuseAt("name", i18n.Tf("error.herdr_name_pattern", domainname.PreviewName(*in.RenameTo, def.Graph.Members[i].Name.String())))
		}
	}
	if err := moveDefinition(oldPath, newPath, in.Hash, text, edited); err != nil {
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	out := apiTeam{s: s}.savedAnswer(*in.RenameTo, in.Place)
	return struct {
		apiSaved
		Name   string            `json:"name"`
		Rename *apiRenamePreview `json:"rename"`
	}{out, *in.RenameTo, preview}, nil
}

func moveDefinition(oldPath, newPath, hash string, before, after []byte) error {
	if err := source.WriteIfUnchanged(oldPath, hash, string(after)); err != nil {
		return err
	}
	if err := source.RenameDirectory(filepath.Dir(oldPath), filepath.Dir(newPath)); err != nil {
		return errors.Join(err, source.WriteIfUnchanged(oldPath, source.Hash(string(after)), string(before)))
	}
	return nil
}
