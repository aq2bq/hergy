package web

import (
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// apiDangerLoss is GET /api/danger/{kind}/{name}: what deleting one thing takes
// away, in the words the confirmation shows (r-ccf8, d-2cce).
type apiDangerLoss struct {
	Lines []string `json:"lines"`
}

// apiDanger answers the loss of one kind.
func (s *Server) apiDanger(w http.ResponseWriter, r *http.Request) {
	scoped, refusal := s.withProjectInput(projectQuery(r))
	if refusal != nil {
		apiAnswer(w, nil, refusal)
		return
	}
	name := r.PathValue("name")
	place := r.URL.Query().Get("place")
	switch r.PathValue("kind") {
	case "activity":
		lines, refusal := scoped.dangerLossActivity(name)
		apiAnswer(w, apiDangerLoss{Lines: lines}, refusal)
	case "team":
		lines, refusal := scoped.dangerLossTeam(place, name)
		apiAnswer(w, apiDangerLoss{Lines: lines}, refusal)
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
func (s *Server) profileTargets(place, name string) (display string, users int, found bool) {
	target, bad := s.resourceSeen(domainplace.Profile, place, name)
	if bad != nil {
		return "", 0, false
	}
	loss, failure := s.resources().ResourceLoss(ops.ExistingResource(target))
	if failure != nil || len(loss.Files) == 0 {
		return "", 0, false
	}
	return s.cat.Display(loss.Files[0]), loss.Users, loss.Found
}

// dangerLossProfile is the profile loss in the confirmation's words.
func (s *Server) dangerLossProfile(place, name string) ([]string, *apiErr) {
	display, users, found := s.profileTargets(place, name)
	if !found {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	lines := []string{tf("danger.profile_item", display, name), t("danger.file_stays")}
	if users > 0 {
		lines = append(lines, tf("danger.profile_users", users))
	}
	lines = append(lines, t("danger.irreversible"))
	return lines, nil
}

// roleTargets is what deleting one role takes: its document, and the people
// who name it. Delete and the loss answer both take it from here (ac-ddb6).
func (s *Server) roleTargets(place, name string) (path string, readers int, found bool) {
	target, bad := s.resourceSeen(domainplace.Role, place, name)
	if bad != nil {
		return "", 0, false
	}
	loss, failure := s.resources().ResourceLoss(ops.ExistingResource(target))
	if failure != nil || len(loss.Files) == 0 {
		return "", 0, false
	}
	return loss.Files[0], loss.Users, loss.Found
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
	lines = append(lines, t("danger.irreversible"))
	return lines, nil
}

// teamTargets is what deleting one definition takes: its team.yaml and the
// agreement.md when one stands there, whether the folder goes (nothing else in
// it), and what stays in the folder (ac-ddb6).
func (s *Server) teamTargets(place, name string) (files, leftovers []string, dirRemoved, found bool) {
	target, bad := s.resourceSeen(domainplace.Definition, place, name)
	if bad != nil {
		return nil, nil, false, false
	}
	loss, failure := s.resources().ResourceLoss(ops.ExistingResource(target))
	if failure != nil {
		return nil, nil, false, false
	}
	return loss.Files, loss.Leftovers, loss.RemoveDirectory, loss.Found
}

// dangerLossTeam is the team loss in the confirmation's words.
func (s *Server) dangerLossTeam(place, name string) ([]string, *apiErr) {
	files, leftovers, dirRemoved, found := s.teamTargets(place, name)
	if !found {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	shown := make([]string, 0, len(files))
	for _, path := range files {
		shown = append(shown, s.cat.Display(path))
	}
	lines := []string{tf("danger.team_files", strings.Join(shown, "、"))}
	for _, item := range s.activity().List().Items {
		if item.Summary.Definition == name {
			lines = append(lines, tf("danger.team_running", item.Name))
		}
	}
	if dirRemoved {
		lines = append(lines, t("danger.team_folder"))
	}
	if len(leftovers) > 0 {
		lines = append(lines, tf("danger.team_left", strings.Join(leftovers, "、")))
	}
	lines = append(lines, t("danger.irreversible"))
	return lines, nil
}

// dangerLossActivity is the team stop's loss: the members and workers whose
// panes close, the workspace that stays, and the words of the confirmation
// (r-ccf8). It reads the same activity answer the stop closes from.
func (s *Server) dangerLossActivity(team string) ([]string, *apiErr) {
	list := s.activity().List()
	var item *ActivityItem
	for i := range list.Items {
		if list.Items[i].Name == team {
			item = &list.Items[i]
			break
		}
	}
	if item == nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	members, workers, ok := s.activity().StopTargets(team)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	lines := []string{tf("danger.stop_members", len(members), strings.Join(members, "、"))}
	if workers > 0 {
		lines = append(lines, tf("danger.stop_workers", workers))
	}
	if label := item.Workspace.Label; label != "" {
		lines = append(lines, tf("danger.stop_workspace", label))
	}
	lines = append(lines, t("danger.irreversible"))
	lines = append(lines, t("danger.stop_talk"))
	return lines, nil
}
