package web

import (
	"github.com/aq2bq/hergy/internal/source"
)

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
