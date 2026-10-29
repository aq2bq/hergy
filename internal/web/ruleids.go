package web

import (
	"github.com/aq2bq/hergy/internal/ruleid"
)

// A screen's save is a write, so the rules it writes get their ids (d-4aab): the
// screen writes none of its own, and the answer it reads back carries what the
// files now hold. The ids are chosen against every rule of this project's two
// layers and the launch record, so a name that was used and taken out is not
// used again.
//
// The save writes the person's lines first and gives the ids after it, so the
// id lines are the only lines a save that changed nothing else moves: a comment
// inside a rule, a key hergy does not read, and the text as typed all stay.
func (s *Server) assignRuleIDs() *apiErr {
	if _, err := ruleid.Assign(s.cat); err != nil {
		return refuse(500, err.Error())
	}
	return nil
}
