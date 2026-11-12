package web

import (
	"errors"
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"
)

func (s *Server) resources() ops.ResourceScope {
	return ops.ResourceScope{Catalog: s.cat, Global: s.globalCatalog(), Registered: s.hq != nil, RenameChecker: func(target domainplace.Project) error {
		checker, ok := s.act.(interface {
			CheckDefinitionRename(domainplace.Project) error
		})
		if !ok {
			return &ops.HQError{Status: 503, Text: t("web.rename.unavailable")}
		}
		err := checker.CheckDefinitionRename(target)
		if bad, ok := errors.AsType[*HQError](err); ok {
			return &ops.HQError{Status: bad.Status, Text: bad.Text}
		}
		return err
	}, Projects: func() ([]*catalog.Catalog, error) {
		if s.hq == nil {
			return []*catalog.Catalog{s.cat}, nil
		}
		projects, err := s.hq.Projects()
		if err != nil {
			return nil, err
		}
		cats := make([]*catalog.Catalog, 0, len(projects.Projects))
		for _, p := range hqShownProjects(projects) {
			cat, bad := s.catalogAt(p.Path)
			if bad != nil {
				return nil, errors.New(bad.Text)
			}
			cats = append(cats, cat)
		}
		return cats, nil
	}}
}
func (s *Server) resourceRef(kind domainplace.Kind, place, name string) (domainplace.Ref, *apiErr) {
	ref, err := ops.NewResourceRef(kind, domainplace.Selection(place).At(s.cat.Base()), name)
	if err != nil {
		return ref, refuseAt("name", err.Error())
	}
	return ref, nil
}
func (s *Server) resourceSeen(kind domainplace.Kind, place, name string) (ops.SeenResource, *apiErr) {
	ref, err := ops.ObserveResource(kind, domainplace.Selection(place).At(s.cat.Base()), name)
	if err != nil {
		return ref, refuseAt("name", err.Error())
	}
	return ref, nil
}
func (s *Server) resourceFailure(bad *ops.ResourceFailure, doc func(string) any, deleting bool) *apiErr {
	if bad == nil {
		return nil
	}
	if bad.Conflict && !bad.Internal {
		return s.refuseChanged(s.cat.Display(bad.Path), bad.Path, doc, deleting)
	}
	if bad.Field != "" {
		return refuseAt(bad.Field, bad.Text)
	}
	status := http.StatusUnprocessableEntity
	if bad.Status != 0 {
		status = bad.Status
	}
	if bad.Internal {
		status = http.StatusInternalServerError
	}
	if bad.Missing {
		status = http.StatusNotFound
	}
	return refuse(status, bad.Text)
}
