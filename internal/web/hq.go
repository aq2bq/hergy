package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the headquarters (d-73fc, r-b7e1): the projects a global
// config.yaml lists, the teams, roles and profiles of every listed project and of
// the global layer, the workspaces of herdr, and starting and stopping a team
// from the screens.
//
// The reading is internal/app's: it reads the config, herdr and the launch record,
// because only that layer starts herdr (6 A1). The server only puts the answer in
// the boundary's shape. A server built without one answers 503: the screens that
// ask for it are the headquarters, and a hergy without a headquarters reading is
// not a state a person can reach.

// HQ is how the headquarters reads and acts.
type HQ interface {
	Projects() (HQProjects, error)
	AddProject(path string) (HQProjects, error)
	RemoveProject(path string) (HQProjects, error)
	Running() HQRunning
	Workspaces() HQWorkspaces
	LaunchOptions(project, definition, place string) (HQLaunchOptions, error)
	StartLaunch(in HQLaunchIn) (HQLaunch, error)
	LaunchState(id string) (HQLaunch, bool)
	CancelLaunch(id string) (HQLaunch, bool)
	Stop(team string) (HQStopped, error)
}

// HQError is a refusal from the headquarters reading: the status, the sentence a
// person reads, and, for a start that has to say what else could be done, the
// name hergy suggests and the thing that is in the way.
type HQError struct {
	Status        int
	Text          string
	SuggestedName string
}

func (e *HQError) Error() string { return e.Text }

// HQProjects is GET /api/hq/projects: the current project, the listed ones, the
// launch record's candidates, and the global layer's counts.
type HQProjects struct {
	Config string `json:"config"`
	// Mode is "listed" (config.yaml names projects:) or "current" (it does not,
	// and serve shows the project it was started in, as it always did).
	Mode     string      `json:"mode"`
	Current  HQProject   `json:"current"`
	Projects []HQProject `json:"projects"`
	Global   HQGlobal    `json:"global"`
}

// HQProject is one project the headquarters knows: where it is, what it is
// called, and what it holds. Path is the identity (d-73fc); Name is what a person
// reads, with the parent added only when two names would look alike.
type HQProject struct {
	Path          string       `json:"path"`
	Name          string       `json:"name"`
	Display       string       `json:"display"`
	Listed        bool         `json:"listed"`
	Current       bool         `json:"current"`
	Exists        bool         `json:"exists"`
	HasHergy      bool         `json:"has_hergy"`
	Defs          int          `json:"definitions"`
	Roles         int          `json:"roles"`
	Profiles      int          `json:"profiles"`
	Running       int          `json:"running"`
	LastLaunch    string       `json:"last_launch,omitempty"`
	LastWorkspace *HQWorkspace `json:"last_workspace,omitempty"`
	// Source says why a project is here without being listed: "launch" is the
	// machine's launch record, "current" is the project serve was started in.
	Source string `json:"source,omitempty"`
}

// HQGlobal is the global layer's own counts: shown once, after every project.
type HQGlobal struct {
	Path     string `json:"path"`
	Display  string `json:"display"`
	Defs     int    `json:"definitions"`
	Roles    int    `json:"roles"`
	Profiles int    `json:"profiles"`
}

// HQGroup is one project's (or the global layer's) slice of a list screen.
type HQGroup struct {
	Project *HQRef  `json:"project,omitempty"`
	Global  bool    `json:"global,omitempty"`
	Name    string  `json:"name,omitempty"`
	Path    string  `json:"path,omitempty"`
	Display string  `json:"display,omitempty"`
	List    apiList `json:"list"`
	// OverriddenBy is the global group's: the names a listed project defines
	// itself, and the projects that do (spec 4 章).
	OverriddenBy map[string][]string `json:"overridden_by,omitempty"`
}

// HQRef is one project as a group names it, and as the workspace and launch
// answers name the project a team belongs to.
type HQRef struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Display string `json:"display"`
}

// HQGroups is GET /api/hq/teams, /roles and /profiles: the projects' groups in
// the config's order, then the global layer once.
type HQGroups struct {
	Groups []HQGroup `json:"groups"`
}

// HQRunning is GET /api/hq/running: the running teams of the listed projects,
// read from the same activity reading the activity screen uses.
type HQRunning struct {
	Herdr ActivityNote    `json:"herdr"`
	Teams []HQRunningTeam `json:"teams"`
}

// HQRunningTeam is one running team with the project and workspace it runs in.
type HQRunningTeam struct {
	Team        string      `json:"team"`
	Definition  string      `json:"definition"`
	Place       string      `json:"place"`
	Project     string      `json:"project"`
	ProjectName string      `json:"project_name"`
	Workspace   HQWorkspace `json:"workspace"`
	Tab         HQTab       `json:"tab"`
	Starting    bool        `json:"starting"`
	Launch      string      `json:"launch,omitempty"`
	Members     []HQMember  `json:"members"`
	Workers     []HQWorker  `json:"workers"`
	Waiting     []string    `json:"waiting"`
	// NotUp is the members the definition has that no agent of the team is live
	// for, by short name: the ones a screen offers 起動 for (r-5d1a).
	NotUp []string `json:"not_up"`
	// Error is why the definition cannot be built, in the words hergy status uses
	// (2.2). A definition that does not build has no member to offer 起動 for, so
	// a screen shows this sentence instead of leaving the column silent (r-fc16).
	Error string `json:"error,omitempty"`
}

// HQWorkspace is one herdr workspace: its id, its label, and the team that is in
// it (one team to a workspace, d-73fc).
type HQWorkspace struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused,omitempty"`
	Team        string `json:"team,omitempty"`
	TeamProject string `json:"team_project,omitempty"`
	LastTeam    string `json:"last_team,omitempty"`
}

// HQTab is one team's tab.
type HQTab struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// HQMember is one member of a running team: its short name, the name herdr knows
// it by, whether an agent is live there, and its state.
type HQMember struct {
	Name  string `json:"name"`
	Herdr string `json:"herdr"`
	Live  bool   `json:"live"`
	State string `json:"state"`
}

// HQWorker is one worker tab.
type HQWorker struct {
	Name string `json:"name"`
	Tab  string `json:"tab"`
	Live bool   `json:"live"`
}

// HQWorkspaces is GET /api/hq/workspaces.
type HQWorkspaces struct {
	Workspaces []HQWorkspace `json:"workspaces"`
}

// HQLaunchOptions is GET /api/hq/launch/options: everything the start sheet
// draws, with the workspace order of spec 5 章 and the name hygy suggests when
// the definition's name is taken.
type HQLaunchOptions struct {
	Definition       string              `json:"definition"`
	Place            string              `json:"place"`
	Project          *HQRef              `json:"project,omitempty"`
	Members          []HQMember          `json:"members"`
	Projects         []HQLaunchProject   `json:"projects,omitempty"`
	Clash            []HQClash           `json:"clash"`
	SuggestedName    string              `json:"suggested_name"`
	LastWorkspace    *HQWorkspace        `json:"last_workspace,omitempty"`
	NewWorkspaceName string              `json:"new_workspace_name"`
	Workspaces       []HQWorkspaceChoice `json:"workspaces"`
}

// HQLaunchProject is one project a global definition can be started in. A
// project that defines the same name itself cannot be chosen (spec 5 章).
type HQLaunchProject struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// HQClash is a running team already using the definition's name.
type HQClash struct {
	Team    string `json:"team"`
	Project string `json:"project"`
}

// HQWorkspaceChoice is one workspace in the start sheet: why it is where it is in
// the order, and whether it can be chosen.
type HQWorkspaceChoice struct {
	HQWorkspace
	Selectable bool   `json:"selectable"`
	Why        string `json:"why"`
}

// HQLaunchIn is POST /api/hq/launch's body.
type HQLaunchIn struct {
	Project    string        `json:"project"`
	Definition string        `json:"definition"`
	Place      string        `json:"place"`
	Name       string        `json:"name"`
	Workspace  HQWorkspaceIn `json:"workspace"`
}

// The actions a launch answer is about: a team starting, or the members of a
// running team being started. A screen words its progress from this.
const (
	ActionLaunch  = "launch"
	ActionRefresh = "refresh"
	// ActionApply is the drift of one or more running teams applied at once
	// (d-36ef). Its progress is read the way a start's is.
	ActionApply = "apply"
)

// HQWorkspaceIn is the workspace a start asks for: one that is there, or one to
// make with this label.
type HQWorkspaceIn struct {
	ID  string `json:"id"`
	New string `json:"new"`
}

// HQLaunch is one start, as it is now.
type HQLaunch struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"`
	Team        string         `json:"team"`
	Definition  string         `json:"definition"`
	Place       string         `json:"place"`
	Project     string         `json:"project"`
	ProjectName string         `json:"project_name"`
	Workspace   HQWorkspace    `json:"workspace"`
	Tab         HQTab          `json:"tab"`
	Phase       string         `json:"phase"`
	Steps       []HQLaunchStep `json:"steps"`
	Waiting     *HQWaiting     `json:"waiting"`
	Error       string         `json:"error"`
	// Members are the members a refresh starts, by short name: empty for a start,
	// whose members the start sheet reads from the definition.
	Members []string `json:"members,omitempty"`
}

// HQLaunchStep is one line of the start's progress.
type HQLaunchStep struct {
	At   string `json:"at"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// HQWaiting is where a person answers the question an agent asks while it starts
// (d-d589).
type HQWaiting struct {
	Member    string `json:"member"`
	Herdr     string `json:"herdr"`
	Workspace string `json:"workspace"`
	Tab       string `json:"tab"`
	Pane      string `json:"pane"`
	Command   string `json:"command"`
}

// HQStopped is POST /api/hq/stop's answer.
type HQStopped struct {
	Team       string      `json:"team"`
	Definition string      `json:"definition,omitempty"`
	Project    string      `json:"project"`
	Workspace  HQWorkspace `json:"workspace"`
	Closed     HQClosed    `json:"closed"`
	StoppedAt  string      `json:"stopped_at"`
	// Leftovers is what the closed panes left in git, for the stop sheet's result
	// (r-b65e).
	Leftovers HQLeftovers `json:"leftovers"`
}

// HQLeftovers is what a stopped team left in git: the worktrees that came while
// it ran (other than the project's own root), and the project's stash count.
type HQLeftovers struct {
	Worktrees []HQWorktree `json:"worktrees"`
	Stashes   int          `json:"stashes"`
}

// HQWorktree is one worktree of the stopped team's project that came while the
// team ran, with what removing it would lose.
type HQWorktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
	Ahead  int    `json:"ahead"`
	Base   string `json:"base"`
	// PaneCwd says a closed pane's working directory was this worktree (d-970c).
	PaneCwd bool `json:"pane_cwd"`
}

// HQClosed is how many members and workers a stop put away.
type HQClosed struct {
	Members int `json:"members"`
	Workers int `json:"workers"`
}

// CatalogProvider builds the catalog of one listed project, for the reads and
// writes a screen makes with a project beside a name.
type CatalogProvider interface {
	ProjectCatalog(path string) (*catalog.Catalog, error)
	GlobalCatalog() *catalog.Catalog
}

// SetCatalogProvider tells the server how to read a listed project's layer. It
// is set by internal/app, which is the only layer that knows where the projects
// are; a server without one scopes nothing by project.
func (s *Server) SetCatalogProvider(p CatalogProvider) { s.projects = p }

// globalCatalog is the global layer read once: through the provider when there
// is one (so its paths read the same in every serve), and through this server's
// own catalog otherwise.
func (s *Server) globalCatalog() *catalog.Catalog {
	if s.projects != nil {
		return s.projects.GlobalCatalog()
	}
	return s.cat.LayerOnly(source.LayerGlobal)
}

// catalogAt is the catalog of one project, or this server's own for an empty
// path.
func (s *Server) catalogAt(project string) (*catalog.Catalog, *apiErr) {
	if project == "" {
		return s.cat, nil
	}
	if s.projects == nil {
		return nil, refuse(http.StatusUnprocessableEntity, t("web.error.project_unknown"))
	}
	cat, err := s.projects.ProjectCatalog(project)
	if err != nil {
		if hq, ok := errors.AsType[*HQError](err); ok {
			return nil, refuse(hq.Status, hq.Text)
		}
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	return cat, nil
}

// withProject is this server reading one project's layer. Everything a kind does
// reads through s.cat, so a copy with the catalog swapped is the same kind over
// another project.
func (s *Server) withProject(project string) (*Server, *apiErr) {
	cat, refusal := s.catalogAt(project)
	if refusal != nil {
		return nil, refusal
	}
	if cat == s.cat {
		return s, nil
	}
	out := *s
	out.cat = cat
	return &out, nil
}

// hq answers 503 when the server was built without a headquarters reading.
func (s *Server) hqOrFail(w http.ResponseWriter) bool {
	if s.hq != nil {
		return true
	}
	writeJSON(w, http.StatusServiceUnavailable, apiErr{status: http.StatusServiceUnavailable, Text: t("web.error.no_hq")})
	return false
}

// answerHQ writes one headquarters answer, or its refusal.
func answerHQ(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	if hq, ok := errors.AsType[*HQError](err); ok {
		body := map[string]any{"text": hq.Text}
		if hq.SuggestedName != "" {
			body["suggested_name"] = hq.SuggestedName
		}
		writeJSON(w, hq.Status, body)
		return
	}
	writeJSON(w, http.StatusInternalServerError, apiErr{status: http.StatusInternalServerError, Text: err.Error()})
}

// readHQIn decodes one headquarters write's body.
func readHQIn[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var in T
	r.Body = http.MaxBytesReader(w, r.Body, maxWrite)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiErr{status: http.StatusUnprocessableEntity, Text: t("web.error.token")})
		return in, false
	}
	return in, true
}

// projectQuery is the project one read or write names.
func projectQuery(r *http.Request) string { return r.URL.Query().Get("project") }

// hqScreenIsland is the headquarters part of a list screen's data island: the
// projects reading, and the groups of each kind the screen draws (spec 4 章).
// The screen builds its rows from the same answers GET /api/hq/{kind} gives, so a
// page and an API read cannot show two different lists. It is nil when the
// server has no headquarters reading: the screen then draws the one project's
// rows as it always did.
type hqScreenIsland struct {
	Projects HQProjects `json:"projects"`
	Teams    []HQGroup  `json:"teams,omitempty"`
	Roles    []HQGroup  `json:"roles,omitempty"`
	Profiles []HQGroup  `json:"profiles,omitempty"`
	// Error is why the headquarters' lists could not be read. A screen that gets
	// it says the sentence rather than drawing the one-project lists as though
	// the projects were not there (r-1db7 L1).
	Error string `json:"error,omitempty"`
}

// hqScreenIsland reads the groups one screen draws, in the order they stand on
// it: every listed project's own, then the global layer's once. It answers nil
// when this server has no headquarters reading at all, and an island carrying
// the refusal's sentence when the reading failed: a screen says why rather than
// falling back to the one project it was started in (r-1db7 L1).
func (s *Server) hqScreenIsland(kindNames ...string) *hqScreenIsland {
	if s.hq == nil {
		return nil
	}
	projects, err := s.hq.Projects()
	if err != nil {
		return &hqScreenIsland{Error: hqRefusal(err).Text}
	}
	out := &hqScreenIsland{Projects: projects}
	for _, kindName := range kindNames {
		groups, refusal := s.hqGroupsOf(projects, kindName)
		if refusal != nil {
			return &hqScreenIsland{Projects: projects, Error: refusal.Text}
		}
		switch kindName {
		case apiTeamKind:
			out.Teams = groups.Groups
		case apiRoleKind:
			out.Roles = groups.Groups
		case apiProfileKind:
			out.Profiles = groups.Groups
		}
	}
	return out
}

// hqProjectGroups is GET /api/hq/{kind}: every listed project's own names, then
// the global layer's once.
func (s *Server) hqProjectGroups(kindName string) (HQGroups, *apiErr) {
	projects, err := s.hq.Projects()
	if err != nil {
		return HQGroups{}, hqRefusal(err)
	}
	return s.hqGroupsOf(projects, kindName)
}

// hqGroupsOf is hqProjectGroups over a projects reading the caller already made,
// so one page does not read every project's catalogs twice.
func (s *Server) hqGroupsOf(projects HQProjects, kindName string) (HQGroups, *apiErr) {
	out := HQGroups{Groups: []HQGroup{}}
	owners := map[string][]string{}
	for _, project := range hqShownProjects(projects) {
		group, refusal := s.hqKindGroup(kindName, project)
		if refusal != nil {
			return HQGroups{}, refusal
		}
		for _, item := range group.List.Items {
			owners[item.Name] = append(owners[item.Name], project.Path)
		}
		out.Groups = append(out.Groups, group)
	}
	global, refusal := s.hqGlobalGroup(kindName, projects)
	if refusal != nil {
		return HQGroups{}, refusal
	}
	global.OverriddenBy = overrideNames(global.List, owners)
	out.Groups = append(out.Groups, global)
	return out, nil
}

// hqShownProjects is the projects the headquarters shows: the listed ones, or
// the project serve was started in when config.yaml names no projects.
func hqShownProjects(projects HQProjects) []HQProject {
	if projects.Mode == "current" {
		return []HQProject{projects.Current}
	}
	shown := make([]HQProject, 0, len(projects.Projects))
	for _, project := range projects.Projects {
		if project.Listed {
			shown = append(shown, project)
		}
	}
	return shown
}

// hqKindGroup is one project's own names of one kind.
func (s *Server) hqKindGroup(kindName string, project HQProject) (HQGroup, *apiErr) {
	scoped, refusal := s.withProject(project.Path)
	if refusal != nil {
		return HQGroup{}, refusal
	}
	kind, ok := scoped.apiKindOf(kindName)
	if !ok {
		return HQGroup{}, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	value, refusal := kind.List()
	if refusal != nil {
		return HQGroup{}, refusal
	}
	list, ok := value.(apiList)
	if !ok {
		return HQGroup{}, refuse(http.StatusInternalServerError, tf("web.error.internal", "list"))
	}
	ref := &HQRef{Path: project.Path, Name: project.Name, Display: project.Display}
	return HQGroup{Project: ref, Name: project.Name, Path: project.Path, Display: project.Display, List: filterPlace(list, catalog.PlaceProject)}, nil
}

// hqGlobalGroup is the global layer's own names of one kind, read once.
func (s *Server) hqGlobalGroup(kindName string, projects HQProjects) (HQGroup, *apiErr) {
	global := s.globalCatalog()
	scoped := *s
	scoped.cat = global
	kind, ok := scoped.apiKindOf(kindName)
	if !ok {
		return HQGroup{}, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	value, refusal := kind.List()
	if refusal != nil {
		return HQGroup{}, refusal
	}
	list, ok := value.(apiList)
	if !ok {
		return HQGroup{}, refuse(http.StatusInternalServerError, tf("web.error.internal", "list"))
	}
	return HQGroup{Global: true, Name: t("web.hq.global"), Path: projects.Global.Path, Display: projects.Global.Display, List: filterPlace(list, catalog.PlaceGlobal)}, nil
}

// filterPlace keeps the items and files that stand in one place, and makes that
// place the item's only one: a group is one layer's rows, not a name's effective
// reading across two.
func filterPlace(list apiList, place string) apiList {
	out := apiList{Items: []apiItem{}, Hashes: map[string]string{}}
	for _, item := range list.Items {
		one, ok := item.Places[place]
		if !ok {
			continue
		}
		item.Places = map[string]apiPlace{place: one}
		item.Effective = place
		item.Hidden = nil
		out.Items = append(out.Items, item)
	}
	for _, file := range list.Files {
		if file.Place == place {
			out.Files = append(out.Files, file)
		}
	}
	for key, hash := range list.Hashes {
		if key == place || strings.HasPrefix(key, place+".") {
			out.Hashes[key] = hash
		}
	}
	return out
}

// overrideNames is the global group's overridden_by: which listed project
// defines each global name itself (spec 4 章). owners is every project name the
// project groups held, with the projects that hold it.
func overrideNames(global apiList, owners map[string][]string) map[string][]string {
	out := map[string][]string{}
	for _, item := range global.Items {
		if projects := owners[item.Name]; len(projects) > 0 {
			out[item.Name] = projects
		}
	}
	return out
}

// hqProjects is GET /api/hq/projects: the projects and the global layer.
func (s *Server) hqProjects(w http.ResponseWriter, _ *http.Request) {
	if !s.hqOrFail(w) {
		return
	}
	projects, err := s.hq.Projects()
	answerHQ(w, projects, err)
}

// hqAddProject is POST /api/hq/projects: list one directory.
func (s *Server) hqAddProject(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) || !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[struct {
		Path string `json:"path"`
	}](w, r)
	if !ok {
		return
	}
	projects, err := s.hq.AddProject(in.Path)
	answerHQ(w, projects, err)
}

// hqRemoveProject is DELETE /api/hq/projects?path=: unlist one directory. The
// project's files are left as they are (d-73fc).
func (s *Server) hqRemoveProject(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) || !s.apiGuard(w, r) {
		return
	}
	projects, err := s.hq.RemoveProject(r.URL.Query().Get("path"))
	answerHQ(w, projects, err)
}

// hqTeams, hqRoles and hqProfiles are GET /api/hq/{kind}: the same lists,
// grouped by project and ending with the global layer.
func (s *Server) hqGroups(kindName string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !s.hqOrFail(w) {
			return
		}
		groups, refusal := s.hqProjectGroups(kindName)
		apiAnswer(w, groups, refusal)
	}
}

// hqRunning is GET /api/hq/running.
func (s *Server) hqRunning(w http.ResponseWriter, _ *http.Request) {
	if !s.hqOrFail(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.hq.Running())
}

// hqWorkspaces is GET /api/hq/workspaces.
func (s *Server) hqWorkspaces(w http.ResponseWriter, _ *http.Request) {
	if !s.hqOrFail(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.hq.Workspaces())
}

// hqLaunchOptions is GET /api/hq/launch/options.
func (s *Server) hqLaunchOptions(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) {
		return
	}
	q := r.URL.Query()
	options, err := s.hq.LaunchOptions(q.Get("project"), q.Get("definition"), q.Get("place"))
	answerHQ(w, options, err)
}

// hqLaunch is POST /api/hq/launch: the start goes on after the answer, and its
// progress is read from GET /api/hq/launches/{id}.
func (s *Server) hqLaunch(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) || !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[HQLaunchIn](w, r)
	if !ok {
		return
	}
	launch, err := s.hq.StartLaunch(in)
	if err != nil {
		answerHQ(w, nil, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"launch": launch})
}

// hqLaunchState is GET /api/hq/launches/{id}.
func (s *Server) hqLaunchState(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) {
		return
	}
	launch, ok := s.hq.LaunchState(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiErr{status: http.StatusNotFound, Text: t("web.error.not_found")})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"launch": launch})
}

// hqLaunchCancel is DELETE /api/hq/launches/{id}: stop waiting, leave what hergy
// made for the stop to put away.
func (s *Server) hqLaunchCancel(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) || !s.apiGuard(w, r) {
		return
	}
	launch, ok := s.hq.CancelLaunch(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, apiErr{status: http.StatusNotFound, Text: t("web.error.not_found")})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"launch": launch})
}

// hqStop is POST /api/hq/stop: hergy down, from the screens.
func (s *Server) hqStop(w http.ResponseWriter, r *http.Request) {
	if !s.hqOrFail(w) || !s.apiGuard(w, r) {
		return
	}
	in, ok := readHQIn[struct {
		Team string `json:"team"`
	}](w, r)
	if !ok {
		return
	}
	stopped, err := s.hq.Stop(in.Team)
	answerHQ(w, stopped, err)
}

// hqRefusal turns a headquarters error into the boundary's refusal.
func hqRefusal(err error) *apiErr {
	if hq, ok := errors.AsType[*HQError](err); ok {
		return refuse(hq.Status, hq.Text)
	}
	return refuse(http.StatusInternalServerError, err.Error())
}
