package web

import (
	"github.com/aq2bq/hergy/internal/ruleid"
	"github.com/aq2bq/hergy/internal/source"
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

// writtenText is the text a save answers with: the file as the save left it on
// the disk, read again. A write is not alone — hergy gives the rules their ids
// after it, and the id lines are lines the answer would miss if it carried the
// text the save sent. The screen's next save is held to the file that is there,
// so the answer must be that file, or the person's own next save is refused as a
// change another place made (r-b41c). text is what the save wrote, answered when
// the file cannot be read again.
func (s *Server) writtenText(path, text string) string {
	if read, err := s.cat.Reader().Read(source.Doc{Path: path}); err == nil {
		return read
	}
	return text
}
