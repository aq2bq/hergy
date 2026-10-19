package web

// This file is the one button that makes a document a place does not have yet
// (W8). It is shared: a definition's page, the shared agreement and a profile
// all offer it.

// makeForm is the one button that makes a document a place does not have yet
// (W8): where the write goes, the token of this run, and the hash of nothing.
func (s *Server) makeForm(kind, name, action string) *makeForm {
	place := s.cat.NewPlace(kind, name)
	if place == "" {
		return nil
	}
	return &makeForm{Href: action, Token: s.token, Hash: s.hashOf(place), Path: s.cat.Display(place)}
}

// makeForm is what the 作る button sends: where the write goes, the token of this
// run, the hash of nothing, and the place the document is made, so the person
// reads where it goes before pressing (spec/42 T1).
type makeForm struct {
	Href  string
	Token string
	Hash  string
	Path  string
}
