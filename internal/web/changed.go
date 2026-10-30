package web

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is what a save answers when the file moved under the screen
// (r-3c77, d-a34a): the file as it is now, and the text the save would have
// written, so the screen keeps the words a person wrote and lets them choose
// between reading the file again, writing over it, and going back — without
// reading the file a second time or knowing its format.
//
// The screen does not open the page again. It holds the draft it always held,
// draws this beside it, and sends one of the three back: the same save with the
// hash read here (overwrite), or the file's own reading (go back).

// apiChanged is one file a refusal is about: where it stands, what it holds now,
// when it was written, and the text the save would have put there.
type apiChanged struct {
	// Path is the file as the screen names it (the same words the sentence uses).
	Path string `json:"path"`
	// Exists says the file is there; when it is false, Text and Hash are empty
	// and Removed is why: the person can still write the draft as a new file.
	Exists bool `json:"exists"`
	// Hash is the text the file holds now: an overwrite carries this one, so a
	// file that moved again is refused the same way.
	Hash string `json:"hash,omitempty"`
	// Text is the file as it is now: the left side of 見比べる.
	Text string `json:"text,omitempty"`
	// Changed is when the file was written, as a person reads the time (10:42,
	// or 1/2 10:42 for another day). Empty when the time cannot be read.
	Changed string `json:"changed,omitempty"`
	// Doc is the file as the kind itself reads it, in the shape the screen keeps
	// its draft in, so 元に戻す puts the file's own reading in place without the
	// screen knowing the file's format (r-3c77). Empty for a file whose kind has
	// no such reading (a whole-text save).
	Doc any `json:"doc,omitempty"`
}

// refuseChanged is the 409 a save answers when the file is not the one the
// screen read: the file as it is now, its hash and the hour it was written, so
// the screen compares it with the text it read and keeps the draft it holds.
// deleting says the save was a delete, which offers the person other words and
// no overwrite.
//
// A file that is gone is answered the same way: Exists false, and the sentence
// says it was deleted, because the draft can still be written as a new file.
func (s *Server) refuseChanged(display, path string, doc func(text string) any, deleting bool) *apiErr {
	out := &apiChanged{Path: display}
	if text, err := s.cat.Reader().Read(source.Doc{Path: path}); err == nil {
		out.Exists, out.Text, out.Hash = true, text, source.Hash(text)
		out.Changed = changedClock(writtenAt(path))
		if doc != nil {
			out.Doc = doc(text)
		}
	}
	switch {
	case deleting && out.Exists:
		return &apiErr{status: http.StatusConflict, Text: tf("error.file_changed_delete", display), File: out}
	case deleting:
		return &apiErr{status: http.StatusConflict, Text: tf("error.file_gone_delete", display), File: out}
	case !out.Exists:
		return &apiErr{status: http.StatusConflict, Text: tf("error.file_gone", display), File: out}
	}
	return &apiErr{status: http.StatusConflict, Text: tf("error.file_changed", display, out.Changed), File: out}
}

// writtenAt is when one file was last written, or the zero time when it cannot
// be read: the sentence then leaves the hour out rather than making one up.
func writtenAt(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// changedClock is one moment as a person reads it: the hour for today, and the
// day with the hour for another day (10:42 / 1/2 10:42). The zero time answers
// the empty string, which leaves the hour out of the sentence.
func changedClock(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	now := time.Now()
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}
	return fmt.Sprintf("%d/%d %s", int(at.Month()), at.Day(), at.Format("15:04"))
}

// ruleDoc is the file as a rule document in the shape the role screen and the
// team's rules keep their draft in: what 元に戻す puts in place (r-3c77).
func ruleDoc(display string) func(text string) any {
	return func(text string) any {
		doc, _ := rules.ReadAll(text, display)
		if doc == nil {
			return nil
		}
		return r2DocOf(doc)
	}
}

// teamDoc is the team.yaml as the team screen keeps its draft in (the description,
// the members, the workers and the turns), with the team's own rules under it.
func (s *Server) teamDoc(name, layer string) func(text string) any {
	return func(string) any {
		def, _ := s.cat.Definition(name, layer)
		if def == nil {
			return nil
		}
		return s.t2DocOf(name, layer, def)
	}
}

// profileDoc is one profile of profiles.yaml as the profile screen keeps its
// draft in: the name, the kind, the arguments and the environment.
func profileDoc(name, display string) func(text string) any {
	return func(text string) any {
		entries, _ := profile.ReadAll(text, display)
		entry, ok := p2FindEntry(entries, name)
		if !ok {
			return nil
		}
		return p2DocOf(entry)
	}
}
