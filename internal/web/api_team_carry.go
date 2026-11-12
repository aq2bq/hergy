package web

import (
	"github.com/aq2bq/hergy/internal/ops"
)

func (s *Server) resourceDraftInput(in apiIn) (ops.ResourceDraftInput, *apiErr) {
	out := ops.ResourceDraftInput{Place: in.Place.At(s.cat.Base()), Name: in.Name, CarryRoles: in.CarryRoles, CarryProfiles: in.CarryProfiles, New: in.New, StarterRoles: in.StarterRoles, StarterProfile: in.StarterProfile}
	if in.Base != nil {
		from, bad := s.withProjectInput(in.Base.Project)
		if bad != nil {
			return out, bad
		}
		out.Base = &ops.ResourceDraftBase{Catalog: from.cat, Place: in.Base.Place.At(from.cat.Base()), Name: in.Base.Name}
	}
	return out, nil
}
