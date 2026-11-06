package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// apiDangerLoss is GET /api/danger/{kind}/{name}: what deleting one thing takes
// away, in the words the confirmation shows (r-ccf8, d-2cce).
type apiDangerLoss struct {
	Lines []string `json:"lines"`
}

// apiDanger answers the loss of one kind.
func (s *Server) apiDanger(w http.ResponseWriter, r *http.Request) {
	scoped, refusal := s.withProject(projectQuery(r))
	if refusal != nil {
		apiAnswer(w, nil, refusal)
		return
	}
	name := r.PathValue("name")
	place := r.URL.Query().Get("place")
	switch r.PathValue("kind") {
	case "role":
		lines, refusal := scoped.dangerLossRole(place, name)
		apiAnswer(w, apiDangerLoss{Lines: lines}, refusal)
	case "profile":
		lines, refusal := scoped.dangerLossProfile(place, name)
		apiAnswer(w, apiDangerLoss{Lines: lines}, refusal)
	default:
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
	}
}

// profileTargets is what deleting one profile takes: its file and entry, and the
// people its name reaches. Delete and the loss answer both take it from here
// (ac-ddb6).
func (s *Server) profileTargets(place, name string) (path, display string, users int, found bool) {
	project, shared, defs := s.p2Read()
	file := project
	if place == p2Shared {
		file = shared
	}
	if _, ok := p2Find(file, name); !ok {
		return file.path, file.display, 0, false
	}
	return file.path, file.display, len(p2Users(p2Chips(defs), name)), true
}

// dangerLossProfile is the profile loss in the confirmation's words.
func (s *Server) dangerLossProfile(place, name string) ([]string, *apiErr) {
	_, display, users, found := s.profileTargets(place, name)
	if !found {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	lines := []string{tf("danger.profile_item", display, name), t("danger.file_stays")}
	if users > 0 {
		lines = append(lines, tf("danger.profile_users", users))
	}
	return lines, nil
}

// roleTargets is what deleting one role takes: its document, and the people
// who name it. Delete and the loss answer both take it from here (ac-ddb6).
func (s *Server) roleTargets(place, name string) (path string, readers int, found bool) {
	path = s.cat.RolePath(place, name)
	there, ok, _ := s.cat.FileState(path)
	if !there || !ok {
		return path, 0, false
	}
	for _, row := range s.cat.Definitions() {
		teamPath := row.LocalPath
		if place == catalog.PlaceGlobal {
			teamPath = row.GlobalPath
		}
		text, err := s.cat.Reader().Read(source.Doc{Path: teamPath})
		if err != nil {
			continue
		}
		var raw struct {
			Members []struct {
				Role string `yaml:"role"`
			} `yaml:"members"`
		}
		if yaml.Unmarshal([]byte(text), &raw) != nil {
			continue
		}
		for _, m := range raw.Members {
			if m.Role == name {
				readers++
			}
		}
	}
	return path, readers, true
}

// dangerLossRole is the role loss in the confirmation's words.
func (s *Server) dangerLossRole(place, name string) ([]string, *apiErr) {
	path, readers, found := s.roleTargets(place, name)
	if !found {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	lines := []string{tf("danger.role_doc", s.cat.Display(path))}
	if readers > 0 {
		lines = append(lines, tf("danger.role_used", readers))
	}
	return lines, nil
}
