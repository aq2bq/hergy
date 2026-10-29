package app

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file is the headquarters reading (d-73fc, r-b7e1): the projects a global
// config.yaml lists, their names, the running teams, the workspaces, and the
// starts this serve is carrying out. It stands on activityRead, so the running
// teams and their states are one reading with the activity screen.
//
// The starts are kept here, in memory: a serve that stops forgets them, and the
// teams it started stay in herdr to be read as running teams by the next serve
// (lead's answer to api.md 10 節).

// hqRead is one serve's headquarters reading.
type hqRead struct {
	act *activityRead
	// mu guards the starts: an HTTP request reads one while its goroutine
	// writes it.
	mu      sync.Mutex
	ops     map[string]*launchOp
	order   []string
	seq     int
	started time.Time
}

// launchOp is one start from the screens, with the state the polling reads.
type launchOp struct {
	mu       sync.Mutex
	launch   web.HQLaunch
	cancel   func()
	finished time.Time
}

// newHQRead builds the headquarters reading on one activity reading.
func newHQRead(act *activityRead) *hqRead {
	return &hqRead{act: act, ops: map[string]*launchOp{}, started: time.Now()}
}

// The activityRead answers the headquarters interface itself, so the server that
// was handed the activity reading finds the headquarters too (web.New).
func (a *activityRead) Projects() (web.HQProjects, error) { return a.hq.Projects() }
func (a *activityRead) AddProject(path string) (web.HQProjects, error) {
	return a.hq.AddProject(path)
}
func (a *activityRead) RemoveProject(path string) (web.HQProjects, error) {
	return a.hq.RemoveProject(path)
}
func (a *activityRead) Running() web.HQRunning       { return a.hq.Running() }
func (a *activityRead) Workspaces() web.HQWorkspaces { return a.hq.Workspaces() }
func (a *activityRead) LaunchOptions(project, definition, place string) (web.HQLaunchOptions, error) {
	return a.hq.LaunchOptions(project, definition, place)
}
func (a *activityRead) StartLaunch(in web.HQLaunchIn) (web.HQLaunch, error) {
	return a.hq.StartLaunch(in)
}
func (a *activityRead) LaunchState(id string) (web.HQLaunch, bool) { return a.hq.LaunchState(id) }
func (a *activityRead) CancelLaunch(id string) (web.HQLaunch, bool) {
	return a.hq.CancelLaunch(id)
}
func (a *activityRead) Stop(teamName string) (web.HQStopped, error) { return a.hq.Stop(teamName) }

// ProjectCatalog is one listed project's layer, for a screen that names a
// project beside a team, role or profile. Only the listed projects (or, when the
// config names none, the project serve was started in) are reachable; a write
// that names any other directory is refused (api.md, r-b7e1).
func (a *activityRead) ProjectCatalog(path string) (*catalog.Catalog, error) {
	allowed, err := a.hq.allowedProjects()
	if err != nil {
		return nil, err
	}
	want, err := team.ExpandProjectPath(path)
	if err != nil {
		return nil, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	for _, one := range allowed {
		if samePath(one, want) {
			return a.hq.catalogAt(want)
		}
	}
	return nil, hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.error.project_unknown"))
}

// GlobalCatalog is the global layer read once for the headquarters, with the
// paths a person reads wherever serve was started.
func (a *activityRead) GlobalCatalog() *catalog.Catalog {
	return a.hq.globalCatalog()
}

// catalogAt builds one project's catalog with the headquarters' display: the
// same project reads the same paths in every serve.
func (h *hqRead) catalogAt(path string) (*catalog.Catalog, error) {
	env, err := h.act.envAt(path)
	if err != nil {
		return nil, err
	}
	return catalog.New(env.GlobalDir, env.LocalDir, env.ProjectRoot, h.act.fileReader(), env.DisplayPath).InRepo(env.InRepo()), nil
}

// globalCatalog is the global layer alone, with the headquarters' display.
func (h *hqRead) globalCatalog() *catalog.Catalog {
	env := h.act.env
	return catalog.New(env.GlobalDir, filepath.Join(env.GlobalDir, ".hq", "no-layer"), env.GlobalDir, h.act.fileReader(), env.DisplayPath).InRepo(env.InRepo())
}

// hqRefuse is a refusal the boundary answers with this status and sentence.
func hqRefuse(status int, text string) *web.HQError {
	return &web.HQError{Status: status, Text: text}
}

// listedProjects is the projects config.yaml lists, in its order, and whether it
// lists any at all. A broken entry is the serve's failure, which runServe
// refuses before it starts.
func (h *hqRead) listedProjects() ([]string, bool, error) {
	cfg, err := h.act.env.ReadConfigFile()
	if err != nil {
		return nil, false, err
	}
	return h.act.env.ProjectPaths(cfg)
}

// allowedProjects is every project a screen may read or write: the listed ones,
// and the project serve was started in (which a config naming no projects shows).
func (h *hqRead) allowedProjects() ([]string, error) {
	paths, present, err := h.listedProjects()
	if err != nil {
		return nil, err
	}
	if !present {
		return []string{h.act.env.ProjectRoot}, nil
	}
	return paths, nil
}

// Projects is GET /api/hq/projects.
func (h *hqRead) Projects() (web.HQProjects, error) {
	paths, present, err := h.listedProjects()
	if err != nil {
		return web.HQProjects{}, hqRefuse(http.StatusInternalServerError, err.Error())
	}
	current, err := h.projectOf(h.act.env.ProjectRoot, true, "")
	if err != nil {
		return web.HQProjects{}, err
	}
	current.Current = true
	current.Listed = false
	for _, path := range paths {
		if samePath(path, h.act.env.ProjectRoot) {
			current.Listed = true
		}
	}
	out := web.HQProjects{
		Config:   h.act.env.DisplayPath(h.act.env.ConfigPath()),
		Mode:     "current",
		Current:  current,
		Projects: []web.HQProject{},
		Global:   h.globalCounts(),
	}
	if present {
		out.Mode = "listed"
	}
	records := h.launchRecords()
	seen := map[string]bool{}
	for _, path := range paths {
		one, err := h.projectOf(path, true, "projects")
		if err != nil {
			return web.HQProjects{}, err
		}
		one.Current = samePath(path, h.act.env.ProjectRoot)
		seen[one.Path] = true
		out.Projects = append(out.Projects, one)
	}
	// The launch record's projects that are not listed are the candidates a
	// person can list again (api.md 1).
	for _, one := range launchProjects(records) {
		if seen[one] || samePath(one, h.act.env.ProjectRoot) {
			continue
		}
		candidate, err := h.projectOf(one, false, "launch")
		if err != nil {
			continue
		}
		out.Projects = append(out.Projects, candidate)
	}
	nameProjectsOf(out.Projects)
	current.Name = currentName(current, out.Projects)
	out.Current = current
	return out, nil
}

// AddProject is POST /api/hq/projects: list one directory. The written entry is
// ~/… when it is under the home directory (lead's addition 3).
func (h *hqRead) AddProject(path string) (web.HQProjects, error) {
	if err := screenPath(path); err != nil {
		return web.HQProjects{}, err
	}
	if _, err := h.act.env.AddProject(path); err != nil {
		return web.HQProjects{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	return h.Projects()
}

// RemoveProject is DELETE /api/hq/projects: unlist one directory, leaving its
// files as they are.
func (h *hqRead) RemoveProject(path string) (web.HQProjects, error) {
	if err := screenPath(path); err != nil {
		return web.HQProjects{}, err
	}
	if err := h.act.env.RemoveProject(path); err != nil {
		return web.HQProjects{}, hqRefuse(http.StatusNotFound, err.Error())
	}
	return h.Projects()
}

// screenPath is a path a person typed in a screen, held to the same two promises
// a config.yaml entry is (absolute or ~), with the words a screen writes rather
// than the ones a person reading config.yaml gets (audit B3).
func screenPath(path string) error {
	text := strings.TrimSpace(path)
	if text == "" {
		return hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.hq.path_empty"))
	}
	if text != "~" && !strings.HasPrefix(text, "~/") && !filepath.IsAbs(text) {
		return hqRefuse(http.StatusUnprocessableEntity, i18n.Tf("web.hq.path_relative", text))
	}
	return nil
}

// projectOf is one project as the headquarters reads it.
func (h *hqRead) projectOf(path string, listed bool, sourceName string) (web.HQProject, error) {
	env, err := h.act.envAt(path)
	if err != nil {
		return web.HQProject{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	cat := env.Catalog(h.act.fileReader())
	defs, roles, profiles := cat.Counts(source.LayerLocal)
	_, statErr := os.Stat(env.ProjectRoot)
	_, hergyErr := os.Stat(env.LocalDir)
	one := web.HQProject{
		Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot),
		Display: h.act.env.DisplayPath(env.ProjectRoot),
		Listed:  listed, Exists: statErr == nil, HasHergy: hergyErr == nil,
		Defs: defs, Roles: roles, Profiles: profiles,
		Source: sourceName,
	}
	one.Running = h.runningIn(env.ProjectRoot)
	if record, ok := h.latestProjectLaunch(env.ProjectRoot); ok {
		one.LastLaunch = record.Time
		if record.Workspace != "" {
			one.LastWorkspace = &web.HQWorkspace{ID: record.WorkspaceID, Label: record.Workspace}
		}
	}
	return one, nil
}

// currentName is the current project's name among the projects the sheet lists:
// the directory's name, or the parent's and the directory's when another project
// has the same name (d-73fc).
func currentName(current web.HQProject, projects []web.HQProject) string {
	base := filepath.Base(current.Path)
	for _, one := range projects {
		if one.Path != current.Path && filepath.Base(one.Path) == base {
			return filepath.Join(filepath.Base(filepath.Dir(current.Path)), base)
		}
	}
	return base
}

// nameProjectsOf adds the parent directory to the names two projects share, so a
// person tells them apart (d-73fc).
func nameProjectsOf(projects []web.HQProject) {
	count := map[string]int{}
	for _, one := range projects {
		count[one.Name]++
	}
	for i := range projects {
		if count[projects[i].Name] > 1 {
			projects[i].Name = filepath.Join(filepath.Base(filepath.Dir(projects[i].Path)), projects[i].Name)
		}
	}
}

// globalCounts is the global layer's own counts.
func (h *hqRead) globalCounts() web.HQGlobal {
	cat := h.act.env.Catalog(h.act.fileReader())
	defs, roles, profiles := cat.Counts(source.LayerGlobal)
	return web.HQGlobal{
		Path: h.act.env.GlobalDir, Display: h.act.env.DisplayPath(h.act.env.GlobalDir),
		Defs: defs, Roles: roles, Profiles: profiles,
	}
}

// runningIn is how many running teams belong to one project.
func (h *hqRead) runningIn(path string) int {
	now := h.act.clock()
	read := h.act.readHerdr(now)
	if read.err != nil {
		return 0
	}
	count := 0
	for _, one := range h.act.teamsOf(read) {
		if samePath(one.Project, path) {
			count++
		}
	}
	return count
}

// launchRecords is this machine's launch record, oldest first.
func (h *hqRead) launchRecords() []launchlog.Record {
	log, err := launchlog.Open()
	if err != nil {
		return nil
	}
	records, err := log.Read()
	if err != nil {
		return nil
	}
	return records
}

// launchProjects is every project the launch record names, newest first.
func launchProjects(records []launchlog.Record) []string {
	seen := map[string]bool{}
	var out []string
	for _, record := range slices.Backward(records) {
		if record.IsHire() {
			continue
		}
		project := record.Project
		if project == "" || seen[project] {
			continue
		}
		seen[project] = true
		out = append(out, project)
	}
	return out
}

// latestProjectLaunch is the newest launch record of one project.
func (h *hqRead) latestProjectLaunch(path string) (launchlog.Record, bool) {
	records := h.launchRecords()
	for _, record := range slices.Backward(records) {
		if record.IsHire() {
			continue
		}
		if samePath(record.Project, path) {
			return record, true
		}
	}
	return launchlog.Record{}, false
}

// hqError is an error that is already a refusal, with its status.
func hqError(err error) error {
	if refusal, ok := errors.AsType[*web.HQError](err); ok {
		return refusal
	}
	return hqRefuse(http.StatusInternalServerError, err.Error())
}

// clockText is one moment in the wire's shape.
func clockText(at time.Time) string { return at.Format(time.RFC3339) }
