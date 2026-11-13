package ops

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
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
	act   *Reader
	locks *OperationLocks
	// mu guards the starts: an HTTP request reads one while its goroutine
	// writes it.
	mu sync.Mutex
	// removed holds the lines a project's removal took, by path, so 取り消す puts
	// config.yaml back as it was (d-2cce). A serve's memory only.
	removed map[string]removedProject
	ops     map[string]*launchOp
	order   []string
	seq     int
	started time.Time
	// The name-release wait uses real time unless a test supplies a clock.
	stopNow   func() time.Time
	stopPause func(time.Duration)
}

// launchOp is one start from the screens, with the state the polling reads.
type launchOp struct {
	mu       sync.Mutex
	teams    []string
	launch   HQLaunch
	cancel   func()
	finished time.Time
}

// newHQRead builds the headquarters reading on one activity reading.
func newHQRead(act *Reader) *hqRead {
	return &hqRead{act: act, locks: &OperationLocks{}, ops: map[string]*launchOp{}, started: time.Now()}
}

// Projects reads the listed projects and their current state.
func (a *Reader) Projects() (HQProjects, error) { return a.hq.Projects() }

// AddProject lists a project and returns the updated reading.
func (a *Reader) AddProject(path string, at int) (HQProjects, error) {
	return a.hq.AddProject(path, at)
}

// RemoveProject unlists a project and remembers the removed lines for undo.
func (a *Reader) RemoveProject(path string) (HQProjects, int, error) {
	return a.hq.RemoveProject(path)
}

// Running reads all visible running teams.
func (a *Reader) Running() HQRunning { return a.hq.Running() }

// Workspaces reads current workspace occupancy.
func (a *Reader) Workspaces() HQWorkspaces { return a.hq.Workspaces() }

// LaunchOptions reads the choices for starting a definition.
func (a *Reader) LaunchOptions(project domainplace.Input, definition, place string) (HQLaunchOptions, error) {
	return a.hq.LaunchOptions(project, definition, place)
}

// StartLaunch accepts a start and retains its progress.
func (a *Reader) StartLaunch(in HQLaunchIn) (HQLaunch, error) {
	return a.hq.StartLaunch(in)
}

// LaunchState reads one operation by its identifier.
func (a *Reader) LaunchState(id string) (HQLaunch, bool) { return a.hq.LaunchState(id) }

// CancelLaunch cancels an operation through its context.
func (a *Reader) CancelLaunch(id string) (HQLaunch, bool) {
	return a.hq.CancelLaunch(id)
}

// Stop closes one team with the shared stop operation.
func (a *Reader) Stop(teamName string) (HQStopped, error) { return a.hq.Stop(teamName) }

// ProjectCatalog is one listed project's layer, for a screen that names a
// project beside a team, role or profile. Only the listed projects (or, when the
// config names none, the project serve was started in) are reachable; a write
// that names any other directory is refused (api.md, r-b7e1).
func (a *Reader) ProjectCatalog(path domainplace.Project) (*catalog.Catalog, error) {
	allowed, err := a.hq.allowedProjects()
	if err != nil {
		return nil, err
	}
	want := path
	if !a.hq.isListedProject(allowed, want) {
		return nil, hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.error.project_unknown"))
	}
	return a.hq.catalogAt(want)
}

// GlobalCatalog is the global layer read once for the headquarters, with the
// paths a person reads wherever serve was started.
func (a *Reader) GlobalCatalog() *catalog.Catalog {
	return a.hq.globalCatalog()
}

// catalogAt builds one project's catalog with the headquarters' display: the
// same project reads the same paths in every serve.
func (h *hqRead) catalogAt(path domainplace.Project) (*catalog.Catalog, error) {
	env, err := h.act.envAt(path)
	if err != nil {
		return nil, err
	}
	return catalog.New(env.GlobalDir, env.LocalDir, env.ProjectRoot, h.act.fileReader(), env.DisplayPath).InRepo(env.InRepo()), nil
}

// globalCatalog is the global layer alone, with the headquarters' display.
func (h *hqRead) globalCatalog() *catalog.Catalog {
	env := h.act.env
	base := env.Catalog(h.act.fileReader()).GlobalBase()
	return catalog.New(env.GlobalDir, filepath.Join(env.GlobalDir, ".hq", "no-layer"), base, h.act.fileReader(), env.DisplayPath).InRepo(env.InRepo())
}

// hqRefuse is a refusal the boundary answers with this status and sentence.
func hqRefuse(status int, text string) *HQError {
	return &HQError{Status: status, Text: text}
}

// listedProjects is the projects config.yaml lists, in its order, and whether it
// lists any at all. A broken entry is the serve's failure, which runServe
// refuses before it starts.
func (h *hqRead) listedProjects() ([]domainplace.Project, bool, error) {
	cfg, err := h.act.env.ReadConfigFile()
	if err != nil {
		return nil, false, err
	}
	return h.act.env.ProjectPaths(cfg)
}

// isListedProject reports whether path is one of the projects config.yaml lists.
// A listed directory inside a git repository is that repository's project: the
// headquarters shows the repository's root, and the screen asks for what it
// shows, so both spellings name the listed project (r-1db7 L1).
func (h *hqRead) isListedProject(paths []domainplace.Project, want domainplace.Project) bool {
	for _, one := range paths {
		if one.Compare(want) == domainplace.Same {
			return true
		}
		if env, err := h.act.envAt(one); err == nil && env.ProjectRoot.Compare(want) == domainplace.Same {
			return true
		}
	}
	return false
}

// allowedProjects is every project a screen may read or write: the listed ones,
// and the project serve was started in (which a config naming no projects shows).
func (h *hqRead) allowedProjects() ([]domainplace.Project, error) {
	paths, present, err := h.listedProjects()
	if err != nil {
		return nil, err
	}
	if !present {
		return []domainplace.Project{h.act.env.ProjectRoot}, nil
	}
	return paths, nil
}

// Projects is GET /api/hq/projects.
func (h *hqRead) Projects() (HQProjects, error) {
	paths, present, err := h.listedProjects()
	if err != nil {
		return HQProjects{}, hqRefuse(http.StatusInternalServerError, err.Error())
	}
	current, err := h.projectOf(h.act.env.ProjectRoot, true, "")
	if err != nil {
		return HQProjects{}, err
	}
	current.Current = true
	current.Listed = false
	for _, path := range paths {
		if path.Compare(h.act.env.ProjectRoot) == domainplace.Same {
			current.Listed = true
		}
	}
	out := HQProjects{
		Config:   h.act.env.DisplayPath(h.act.env.ConfigPath()),
		Mode:     "current",
		Current:  current,
		Projects: []HQProject{},
		Global:   h.globalCounts(),
	}
	if present {
		out.Mode = "listed"
	}
	records := h.launchRecords()
	seen := map[domainplace.Project]bool{}
	for _, path := range paths {
		one, err := h.projectOf(path, true, "projects")
		if err != nil {
			return HQProjects{}, err
		}
		one.Current = path.Compare(h.act.env.ProjectRoot) == domainplace.Same
		seen[one.Path] = true
		out.Projects = append(out.Projects, one)
	}
	// The launch record's projects that are not listed are the candidates a
	// person can list again (api.md 1).
	for _, one := range launchProjects(records) {
		if seen[one] || one.Compare(h.act.env.ProjectRoot) == domainplace.Same {
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
func (h *hqRead) AddProject(path string, at int) (HQProjects, error) {
	if err := screenPath(path); err != nil {
		return HQProjects{}, err
	}
	var err error
	if took, ok := h.takeRemoved(path, at); ok {
		err = h.act.env.AddProjectBlock(at, took)
	} else {
		_, err = h.act.env.AddProjectAt(path, at)
	}
	if err != nil {
		return HQProjects{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	return h.Projects()
}

// takeRemoved gives back the lines a removal took at this position, once.
func (h *hqRead) takeRemoved(path string, at int) ([]string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	took, ok := h.removed[path]
	if !ok || took.at != at {
		return nil, false
	}
	delete(h.removed, path)
	return took.lines, true
}

// RemoveProject is DELETE /api/hq/projects: unlist one directory, leaving its
// files as they are.
func (h *hqRead) RemoveProject(path string) (HQProjects, int, error) {
	if err := screenPath(path); err != nil {
		return HQProjects{}, -1, err
	}
	at, block, err := h.act.env.RemoveProjectBlock(path)
	if err != nil {
		return HQProjects{}, -1, hqRefuse(http.StatusNotFound, err.Error())
	}
	h.mu.Lock()
	if h.removed == nil {
		h.removed = map[string]removedProject{}
	}
	h.removed[path] = removedProject{at: at, lines: block}
	h.mu.Unlock()
	projects, err := h.Projects()
	return projects, at, err
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
func (h *hqRead) projectOf(path domainplace.Project, listed bool, sourceName string) (HQProject, error) {
	env, err := h.act.envAt(path)
	if err != nil {
		return HQProject{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	cat := env.Catalog(h.act.fileReader())
	defs, roles, profiles := ResourceCounts(cat, source.LayerLocal)
	_, statErr := os.Stat(env.ProjectRoot.String())
	_, hergyErr := os.Stat(env.LocalDir)
	one := HQProject{
		Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot.String()),
		Display: h.act.env.DisplayPath(env.ProjectRoot.String()),
		Listed:  listed, Exists: statErr == nil, HasHergy: hergyErr == nil,
		Defs: defs, Roles: roles, Profiles: profiles,
		Source: sourceName,
	}
	one.Running = h.runningIn(env.ProjectRoot)
	if record, ok := h.latestProjectLaunch(env.ProjectRoot); ok {
		one.LastLaunch = record.Time
		one.LastWorkspace = &HQWorkspace{ID: record.WorkspaceID, Label: record.Workspace}
	}
	return one, nil
}

// currentName is the current project's name among the projects the sheet lists:
// the directory's name, or the parent's and the directory's when another project
// has the same name (d-73fc).
func currentName(current HQProject, projects []HQProject) string {
	base := filepath.Base(current.Path.String())
	for _, one := range projects {
		if one.Path != current.Path && filepath.Base(one.Path.String()) == base {
			return filepath.Join(filepath.Base(filepath.Dir(current.Path.String())), base)
		}
	}
	return base
}

// nameProjectsOf adds the parent directory to the names two projects share, so a
// person tells them apart (d-73fc).
func nameProjectsOf(projects []HQProject) {
	count := map[string]int{}
	for _, one := range projects {
		count[one.Name]++
	}
	for i := range projects {
		if count[projects[i].Name] > 1 {
			projects[i].Name = filepath.Join(filepath.Base(filepath.Dir(projects[i].Path.String())), projects[i].Name)
		}
	}
}

// globalCounts is the global layer's own counts.
func (h *hqRead) globalCounts() HQGlobal {
	cat := h.act.env.Catalog(h.act.fileReader())
	defs, roles, profiles := ResourceCounts(cat, source.LayerGlobal)
	return HQGlobal{
		Path: h.act.env.GlobalDir, Display: h.act.env.DisplayPath(h.act.env.GlobalDir),
		Defs: defs, Roles: roles, Profiles: profiles,
	}
}

// runningIn is how many running teams belong to one project.
func (h *hqRead) runningIn(path domainplace.Project) int {
	now := h.act.clock()
	read := h.act.readHerdr(now)
	if read.err != nil {
		return 0
	}
	count := 0
	for _, one := range h.act.teamsOf(read) {
		if one.Project.Compare(path) == domainplace.Same {
			count++
		}
	}
	return count
}

// launchRecords is this machine's launch record, oldest first.
func (h *hqRead) launchRecords() []launchlog.Record {
	records, err := readLaunchRecords()
	if err != nil {
		return nil
	}
	return records
}

// launchProjects is every project the launch record names, newest first.
func launchProjects(records []launchlog.Record) []domainplace.Project {
	seen := map[domainplace.Project]bool{}
	var out []domainplace.Project
	for _, record := range slices.Backward(records) {
		project := record.Project.Project()
		if project.IsZero() || seen[project] {
			continue
		}
		seen[project] = true
		out = append(out, project)
	}
	return out
}

// latestProjectLaunch is the newest workspace-bearing up of one project.
func (h *hqRead) latestProjectLaunch(path domainplace.Project) (launchlog.Record, bool) {
	records := h.launchRecords()
	for _, record := range slices.Backward(records) {
		if record.StartedInWorkspace() && record.Project.Project().Compare(path) == domainplace.Same {
			return record, true
		}
	}
	return launchlog.Record{}, false
}

// hqError is an error that is already a refusal, with its status.
func hqError(err error) error {
	if refusal, ok := errors.AsType[*HQError](err); ok {
		return refusal
	}
	return hqRefuse(http.StatusInternalServerError, err.Error())
}

// clockText is one moment in the wire's shape.
func clockText(at time.Time) string { return at.Format(time.RFC3339) }

// removedProject is the lines a project's removal took, and where it stood.
type removedProject struct {
	at    int
	lines []string
}

func hqOperationError(err error) error {
	if err == nil {
		return nil
	}
	if refusal, ok := errors.AsType[*Refusal](err); ok {
		status := http.StatusUnprocessableEntity
		switch refusal.Kind {
		case "conflict":
			status = http.StatusConflict
		case "unavailable":
			status = http.StatusServiceUnavailable
		case "notfound":
			status = http.StatusNotFound
		case "internal":
			status = http.StatusInternalServerError
		}
		return &HQError{Status: status, At: refusal.At, Text: refusal.Error(), SuggestedName: refusal.SuggestedName, cause: err}
	}
	return hqError(err)
}
