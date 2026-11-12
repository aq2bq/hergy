package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/i18n"
)

type apiRenamePreview struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Notes []string `json:"notes"`
}

func (s *Server) renamePreview(old, next, place string) (*apiRenamePreview, *apiErr) {
	if place != "project" && place != "global" {
		return nil, refuseAt("place", i18n.T("web.rename.place"))
	}
	from, bad := s.resourceSeen(domainplace.Definition, place, old)
	if bad != nil {
		return nil, bad
	}
	to, bad := s.resourceRef(domainplace.Definition, place, next)
	if bad != nil {
		return nil, refuseAt("name", i18n.T("web.rename.name"))
	}
	out, failure := s.resources().PreviewDefinitionRename(from, to)
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	return &apiRenamePreview{From: out.From, To: out.To, Notes: out.Notes}, nil
}

func (s *Server) apiTeamRename(w http.ResponseWriter, r *http.Request) {
	in, ok := s.apiRead(w, r)
	if !ok {
		return
	}
	scoped, refusal := s.withProjectInput(in.Project)
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
	if in.Place != "project" && in.Place != "global" {
		return nil, refuseAt("place", i18n.T("web.rename.place"))
	}
	from, bad := s.resourceSeen(domainplace.Definition, in.Place.String(), old)
	if bad != nil {
		return nil, bad
	}
	to, bad := s.resourceRef(domainplace.Definition, in.Place.String(), *in.RenameTo)
	if bad != nil {
		return nil, refuseAt("name", i18n.T("web.rename.name"))
	}
	preview, failure := s.resources().RenameDefinition(from, to, ops.DefinitionRename{Hash: in.Hash, AgreementHash: in.AgreementHash, AgreementExists: *in.AgreementExists})
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	return struct {
		apiSaved
		Name   string            `json:"name"`
		Rename *apiRenamePreview `json:"rename"`
	}{apiTeam{s: s}.savedAnswer(*in.RenameTo, in.Place.String()), *in.RenameTo, &apiRenamePreview{From: preview.From, To: preview.To, Notes: preview.Notes}}, nil
}
