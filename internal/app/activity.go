package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/gycli"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file is the reading behind the activity screen (spec-activity, r-8a67):
// which teams of this project are running, how each one is doing, and, for one
// team that is open, its questions and its pane. It is here rather than in
// internal/web because it starts herdr and gy, which only this layer does (6 A1);
// the screen is handed the answer.
//
// The reading of herdr is shared with hergy status: both call readRunningTeams,
// which is runningTeams plus each team's built definition (6 A1).

// tailShown is how many lines of a pane the screen shows at first, before 上に N
// 行 (2.4).
const tailShown = 6

// runningTeam is one running team with its team definition built: what hergy
// status lists and what the activity screen shows. Broken says why a definition
// that cannot be built could not be, in the words hergy status writes; the team
// is still listed then, so one such team does not hide the rest.
type runningTeam struct {
	// Name is the team's name: the label on its tab in herdr.
	Name string
	// Definition is the team definition the team was booted from, as the launch
	// record names it. It differs from Name for a team booted with --as.
	Definition string
	// Built is the built definition: its graph, its members and its workers.
	// nil when the definition cannot be built.
	Built *resolved
	// Broken is why the definition cannot be built, as hergy status's 詳細 line
	// reads. Empty when it builds.
	Broken string
	// Elsewhere says the definition is not in its project's catalog at all.
	Elsewhere bool
	// Project is the listed project the team belongs to (d-73fc): the launch
	// record's project, or the project whose layer holds the definition. Name is
	// what a person reads, and Catalog is that project's reading, which the
	// delivery check and the opened part use.
	Project string
	// Place is the definition's layer: "local" for a project's definition,
	// "global" for one started in a project from the global layer.
	Place       string
	ProjectName string
	Catalog     *catalog.Catalog
	// Workspace is the herdr workspace the team's tab stands in.
	WorkspaceID    string
	WorkspaceLabel string
	// TabID is the tab's id, for the answer location.
	TabID string
}

// readRunningTeams reads this machine's running teams and builds each one's team
// definition. It is the one reading hergy status and the activity screen call
// (6 A1), so the two cannot disagree about which teams are running, which
// definition a team was booted from, or why one's definition does not read.
//
// The reading of herdr is handed in rather than done here, so a screen that has
// just read herdr — the activity's band and its list share one read for five
// seconds (spec-activity 3) — does not read it again to find the teams.
func readRunningTeams(env team.Env, read herdrNow) []runningTeam {
	cat := env.Catalog(source.Disk{})
	view := projectView{Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot), Env: env, Cat: cat}
	return readRunningTeamsIn([]projectView{view}, read, true)
}

// projectView is one project the activity reads: its root, the name a person
// reads, the environment its layer lives in, and the catalog over that layer.
type projectView struct {
	Path string
	Name string
	Env  team.Env
	Cat  *catalog.Catalog
}

// readRunningTeamsIn reads the running teams that belong to the given projects:
// a team belongs to the project its launch record names, and a team with no
// record belongs to the project whose layer holds its definition. Teams outside
// these projects are left out (d-73fc: a serve shows the projects it lists).
//
// The herdr read is handed in rather than done here, so a screen that has just
// read herdr — the activity's band and its list share one read for five seconds
// (spec-activity 3) — does not read it again to find the teams.
func readRunningTeamsIn(views []projectView, read herdrNow, lenient bool) []runningTeam {
	running := team.RunningTeams(read.tabs, launchTeams(), read.agentsRead)
	records := latestLaunches()
	out := make([]runningTeam, 0, len(running))
	for _, entry := range running {
		view, recorded, ok := projectOf(entry, records, views, read.tabs, lenient)
		if !ok {
			continue
		}
		one := runningTeam{
			Name: entry.Team, Definition: entry.Definition,
			Project: view.Path, ProjectName: view.Name, Catalog: view.Cat,
			Place: recorded.Layer,
		}
		if tab, found := teamTab(read.tabs, entry.Team); found {
			one.TabID = tab.ID
			one.WorkspaceID = tab.WorkspaceID
			one.WorkspaceLabel = workspaceLabel(read.spaces, tab.WorkspaceID)
		}
		r, err := buildRunningIn(view.Cat, view.Env.ProjectRoot, entry, recorded.Layer)
		if err != nil {
			// A definition that does not read is not this reading's failure: the
			// row says why in the definition's own words (2.2), as hergy status
			// says it, so the reason is kept on the team rather than returned.
			one.Broken = brokenTeamLine(view.Env, err)
			one.Elsewhere = errors.Is(err, catalog.ErrNotFound)
			out = append(out, one)
			continue
		}
		one.Built = r
		one.Place = r.team.Definition.Layer
		out = append(out, one)
	}
	return out
}

// projectOf is the project one running team belongs to, and its newest launch
// record. The record the reading chose says the project when it names one: it is
// the record of the project the team's panes stand in, so another project's team
// of the same name is not taken for this one's (r-87c8). A team whose record
// names a project outside the views is left out: it is another project's, and a
// serve shows only the projects it lists. A team with no record belongs to the one
// view whose layer holds its definition, or, when the record says nothing, whose
// panes its own stand among (d-73fc).
func projectOf(entry team.Running, records launchRecords, views []projectView, tabs []team.Tab, lenient bool) (projectView, launchlog.Record, bool) {
	if entry.Project != "" {
		for _, view := range views {
			if samePath(entry.Project, view.Path) {
				record, _ := records.at(view.Path, entry.Team)
				return view, record, true
			}
		}
		if lenient && len(views) == 1 {
			record, _ := records.at(views[0].Path, entry.Team)
			return views[0], record, true
		}
		return projectView{}, launchlog.Record{}, false
	}
	// A team with no record belongs to the project its named panes stand in: a
	// boot that failed once the tab was made leaves the panes in the project the
	// work runs in, and another project's team is not taken for this one's
	// (d-73fc).
	for _, view := range views {
		if panesIn(view.Path, tabs, entry.Team) {
			record, _ := records.at(view.Path, entry.Team)
			return view, record, true
		}
	}
	for _, view := range views {
		if view.Cat.HoldsDefinition(entry.Definition, source.LayerLocal) {
			return view, launchlog.Record{}, true
		}
	}
	// The CLI reads one project and keeps every team it cannot place, saying why
	// its definition does not read; the screens show only teams they can place.
	if lenient && len(views) == 1 {
		return views[0], launchlog.Record{}, true
	}
	return projectView{}, launchlog.Record{}, false
}

// panesIn reports whether one team's named panes stand in a project. The panes
// are paired with their tab's PaneDirs by position.
func panesIn(project string, tabs []team.Tab, name string) bool {
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label, name) {
			continue
		}
		for i, pane := range tab.Panes {
			if !protocol.NamedForTeam(name, pane) || i >= len(tab.PaneDirs) {
				continue
			}
			if rel, err := filepath.Rel(project, tab.PaneDirs[i]); err == nil &&
				rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// launchRecords is this machine's launch record as reading a running team needs
// it: the newest record of each project and team name, and of each team name
// alone, for a team whose tab does not say where it stands (r-87c8).
type launchRecords struct {
	byPlace map[string]launchlog.Record
	byTeam  map[string]launchlog.Record
}

// at is the newest record of one team in one project.
func (r launchRecords) at(project, name string) (launchlog.Record, bool) {
	record, ok := r.byPlace[project+"\x00"+name]
	return record, ok
}

// latestLaunches is the newest launch record of each project and team name, and
// of each team name alone.
func latestLaunches() launchRecords {
	log, err := launchlog.Open()
	if err != nil {
		return launchRecords{}
	}
	records, err := log.Read()
	if err != nil {
		return launchRecords{}
	}
	out := launchRecords{
		byPlace: make(map[string]launchlog.Record, len(records)),
		byTeam:  make(map[string]launchlog.Record, len(records)),
	}
	for _, record := range records {
		if record.Team == "" || record.IsHire() {
			continue
		}
		out.byTeam[record.Team] = record
		if record.Project != "" {
			out.byPlace[record.Project+"\x00"+record.Team] = record
		}
	}
	return out
}

// teamTab is the tab labelled for one team, with the workspace it stands in.
func teamTab(tabs []team.Tab, name string) (team.Tab, bool) {
	for _, tab := range tabs {
		if protocol.NamesTeam(tab.Label, name) && team.HoldsMember(tab, name) {
			return tab, true
		}
	}
	return team.Tab{}, false
}

// workspaceLabel is one workspace's label, or the id when herdr did not list it.
func workspaceLabel(spaces []herdr.Workspace, id string) string {
	for _, space := range spaces {
		if space.ID == id {
			return space.Label
		}
	}
	return id
}

// samePath reports whether two paths name the same directory. A launch records
// the project root as it was then, and this serve has its own: a path through a
// symbolic link (macOS's /tmp beside /private/tmp) is the same project, so both
// spellings are tried before the two are called different projects.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// herdrNow is one read of herdr as the activity needs it: the tabs, each with the
// live agents in it, the live agents themselves, whether the agent list could be
// read at all, when each agent's state was first seen, and, when herdr could not
// be reached at all, why not (2.7).
type herdrNow struct {
	tabs       []team.Tab
	agents     []protocol.Agent
	agentsRead bool
	// spaces are herdr's workspaces: a team's tab names the one it stands in,
	// and the headquarters offers them to start a team in (d-73fc).
	spaces []herdr.Workspace
	seen   map[string]activity.Watch
	at     time.Time
	err    error
}

// herdrMemory is how long one read of herdr stands (spec-activity 3): the screen
// reads the list every ten seconds and the band reads the same numbers with every
// page, which would otherwise run herdr's two commands for every one of them.
const herdrMemory = 5 * time.Second

// activityRead is the reading behind the activity screen. hergy serve builds one
// and hands it to web.New, so one serve has one memory of when each agent
// stopped (6 A2) and one 5-second memory of herdr (3 章).
//
// The mutex is what the HTTP handlers need: they run at the same time, and both
// memories are read and written per request. No goroutine is added — the reading
// happens in the request that asks for it — so nothing has to be stopped when the
// server stops.
type activityRead struct {
	env team.Env
	// now is the clock: a test places the moments with it. A serve built without
	// one reads the serve's own clock (serveClock).
	now func() time.Time
	// ctx is the serve's own lifetime: a start from the screens stops waiting
	// when the serve stops. hq is the headquarters reading built on this one, so
	// the screens' running teams and the activity are one reading (r-b7e1).
	ctx context.Context
	hq  *hqRead

	mu      sync.Mutex
	herdr   herdrNow
	watcher *activity.Watcher

	// deliveryMu holds the last delivery answer for five seconds, the same
	// memory the band count reads on every page. Deliver clears it.
	deliveryMu   sync.Mutex
	deliveryPlan deliveryPlan
	deliveryAt   time.Time
}

// newActivityRead is the reading behind one hergy serve. The watcher's memory is
// kept in this machine's state directory, so a serve that is started again reads
// a stop from when it was first seen rather than from its own start (6 A2).
func newActivityRead(env team.Env) *activityRead {
	read := &activityRead{env: env, watcher: activity.NewWatcherAt(activityStatePath())}
	read.hq = newHQRead(read)
	return read
}

// activityStatePath is where the watcher keeps its memory (6 A2):
// $XDG_STATE_HOME/hergy/activity.json, or the state directory under the home
// directory when XDG_STATE_HOME is unset, beside the launch record.
func activityStatePath() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "hergy", "activity.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "hergy", "activity.json")
}

// clock is now, or the test's own clock.
func (a *activityRead) clock() time.Time {
	if a.now == nil {
		return serveClock()
	}
	return a.now()
}

// readHerdr reads herdr, or answers with the read it made less than five seconds
// ago (3 章). The moment each agent's state was first seen is taken here too, so
// the watcher is asked once per read and not once per team.
func (a *activityRead) readHerdr(now time.Time) herdrNow {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.herdr.at.IsZero() && now.Sub(a.herdr.at) < herdrMemory {
		return a.herdr
	}
	tabs, agents, agentsRead, err := herdrRead()
	read := herdrNow{tabs: tabs, agents: agents, agentsRead: agentsRead, at: now, err: err}
	if err == nil {
		read.spaces, _ = herdrSpaceList()
		read.seen = a.watcher.Observe(watchedAgents(agents), now)
	}
	a.herdr = read
	return read
}

// forgetHerdr drops the read held for five seconds, so the next reader sees what
// herdr reports now. A stop and a start change herdr under hergy's feet, and the
// screens must show the change on their next read rather than up to five seconds
// later (r-b7e1).
func (a *activityRead) forgetHerdr() {
	a.mu.Lock()
	a.herdr = herdrNow{}
	a.mu.Unlock()
}

// readHerdrFresh reads herdr now, whatever was read less than five seconds ago.
// The decisions a start and a stop make read through this: acting on a memory of
// a team that has just gone, or of a workspace that has just emptied, would start
// a team beside another or refuse to stop one that is there (r-b7e1).
func (a *activityRead) readHerdrFresh(now time.Time) herdrNow {
	a.forgetHerdr()
	return a.readHerdr(now)
}

// watchedAgents is the live agents as the watcher reads them: the name herdr
// knows each by, its state, and its state_change_seq. Every agent herdr reports
// is taken, not only this project's teams' members, because the watcher remembers
// one machine's agents by name (6 A2).
func watchedAgents(agents []protocol.Agent) []activity.Agent {
	out := make([]activity.Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Name == "" {
			continue
		}
		out = append(out, activity.Agent{Herdr: agent.Name, Status: agent.Status, Seq: agent.Seq})
	}
	return out
}

// teamsOf is the running teams of the projects this serve shows, in herdr's
// order, with their definitions built, from one read of herdr. The projects are
// the ones config.yaml lists; a serve whose config names none shows the project
// it was started in, as it always has (d-73fc).
func (a *activityRead) teamsOf(read herdrNow) []runningTeam {
	return readRunningTeamsIn(a.projectViews(), read, false)
}

// projectViews is the projects this serve reads, with their names, layers and
// catalogs: the listed ones in the config's order, or the project serve was
// started in when the config names none.
func (a *activityRead) projectViews() []projectView {
	cfg, err := a.env.ReadConfigFile()
	if err != nil {
		return []projectView{a.viewOf(a.env)}
	}
	paths, present, err := a.env.ProjectPaths(cfg)
	if err != nil || !present {
		return []projectView{a.viewOf(a.env)}
	}
	views := make([]projectView, 0, len(paths))
	for _, path := range paths {
		env, err := a.env.AtProject(path)
		if err != nil {
			continue
		}
		views = append(views, a.viewOf(env))
	}
	return nameProjects(views)
}

// viewOf is one project's view of this environment.
func (a *activityRead) viewOf(env team.Env) projectView {
	return projectView{
		Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot),
		Env: env, Cat: env.Catalog(source.Disk{}),
	}
}

// nameProjects adds the parent directory to a name two projects share, so a
// person tells them apart (d-73fc: the name is the directory's, and the parent is
// added only when names clash).
func nameProjects(views []projectView) []projectView {
	count := map[string]int{}
	for _, view := range views {
		count[view.Name]++
	}
	for i := range views {
		if count[views[i].Name] > 1 {
			views[i].Name = filepath.Join(filepath.Base(filepath.Dir(views[i].Path)), views[i].Name)
		}
	}
	return views
}

// List is GET /api/activity: the running teams of this project, in the order of
// activity.Sort, and the band's two numbers. It reads herdr and the launch record
// and the team definitions, and neither gy nor a pane (spec-activity 3).
func (a *activityRead) List() web.ActivityList {
	now := a.clock()
	list := web.ActivityList{Items: []web.ActivityItem{}, Herdr: web.ActivityNote{OK: true}}
	read := a.readHerdr(now)
	if read.err != nil {
		// herdr is not there: the screen says so in herdr's own words and the
		// band writes no number (2.7).
		list.Herdr = web.ActivityNote{OK: false, Text: read.err.Error()}
		return list
	}
	teams := a.teamsOf(read)
	rows := make([]activityRow, 0, len(teams))
	for _, one := range teams {
		rows = append(rows, activityRow{
			team:        judgeOf(one, read, now),
			definition:  one.Definition,
			broken:      one.Broken,
			project:     one.Project,
			projectName: one.ProjectName,
			workspace:   web.ActivitySpace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		})
	}
	list.Items, list.Running, list.Waiting = orderedRows(rows, now)
	return list
}

// activityRow is one row of the list: the judgement of one team and what the wire
// needs beside it.
type activityRow struct {
	team        activity.Team
	definition  string
	broken      string
	project     string
	projectName string
	workspace   web.ActivitySpace
}

// orderedRows sorts the rows the way the screen lists them (2.2: the teams
// waiting for a human first, then the stopped ones, then the working ones) and
// turns them into the list's items, counting the band's two numbers on the way:
// the running teams, and the members that are waiting for the human.
func orderedRows(rows []activityRow, now time.Time) ([]web.ActivityItem, int, int) {
	judged := make([]activity.Team, len(rows))
	byName := make(map[string]activityRow, len(rows))
	for i, row := range rows {
		judged[i] = row.team
		byName[row.team.Name] = row
	}
	activity.Sort(judged)
	items := make([]web.ActivityItem, 0, len(judged))
	waiting := 0
	for _, judged := range judged {
		row := byName[judged.Name]
		items = append(items, itemOf(row, now))
		waiting += waitingMembers(judged)
	}
	return items, len(items), waiting
}

// waitingMembers is how many of a team's members are waiting for the human: the
// ones herdr reports blocked, and the member that handed the turn over (2.1).
// Workers are not members: a worker that is blocked is not counted.
func waitingMembers(t activity.Team) int {
	count := 0
	for _, node := range t.Nodes {
		if node.State == activity.Blocked || node.State == activity.HandedOver {
			count++
		}
	}
	return count
}

// judgeOf judges one team: how it is doing, who the line is about, and the members
// and workers in it (6 A2, A3). A definition that does not read leaves the live
// agents as the only members known, so what the row says comes from herdr alone,
// as 2.2 says.
func judgeOf(one runningTeam, read herdrNow, now time.Time) activity.Team {
	var graph *definition.Graph
	agents := agentsOf(read, one.Name)
	if one.Built != nil {
		graph = one.Built.team.Definition.Graph
	} else {
		// The definition is the thing that could not be read, so the graph here
		// is made of the team's live agents alone: the row and the right-hand
		// column then name the members that are really there, by the short name
		// hergy gave them, rather than showing nothing at all (2.2).
		graph = definition.NewGraph(membersOf(agents), nil, nil)
	}
	return activity.Of(activity.Input{
		Team:   one.Name,
		Graph:  graph,
		Agents: agents,
		Seen:   read.seen,
		Now:    now,
	})
}

// membersOf is the live agents as members of a graph, for a team definition that
// does not read: their short names are all the definition the screen can name
// (2.2). A graph made this way holds no turn, so the team can only read as
// working or stopped — which is what 2.2 says a definition that does not read
// can be told apart.
func membersOf(agents []activity.Agent) []definition.MemberNode {
	out := make([]definition.MemberNode, 0, len(agents))
	for _, agent := range agents {
		if agent.Node != "" {
			out = append(out, definition.MemberNode{Name: agent.Node})
		}
	}
	return out
}

// agentsOf is one team's live agents as the judgement reads them: the name herdr
// knows each by, the node it stands for when the graph names it, its state and
// its state_change_seq.
func agentsOf(read herdrNow, name string) []activity.Agent {
	live := teamAgents(read.tabs, read.agents, name)
	out := make([]activity.Agent, 0, len(live))
	for _, agent := range live {
		out = append(out, activity.Agent{
			Herdr:  agent.Name,
			Node:   shortMember(agent.Name, name),
			Status: agent.Status,
			Seq:    agent.Seq,
		})
	}
	return out
}

// teamAgents is one team's live agents: the ones herdr names after the team and
// stands in the team's own tab, and the ones in the tabs hergy made for the
// workers the team hired (labelled "worker:<team>/<name>", spec-activity 1). The
// tab is what says whose agent it is: a team named "core" beside a team named
// "core-hotfix" would otherwise take the other's members for its own.
func teamAgents(tabs []team.Tab, agents []protocol.Agent, name string) []protocol.Agent {
	wanted := map[string]bool{}
	for _, agent := range team.MemberAgents(tabs, name) {
		wanted[agent] = true
	}
	prefix := "worker:" + name + "/"
	for _, tab := range tabs {
		if !strings.HasPrefix(tab.Label, prefix) {
			continue
		}
		for _, agent := range tab.Agents {
			wanted[agent] = true
		}
	}
	out := make([]protocol.Agent, 0, len(wanted))
	for _, agent := range agents {
		if agent.Name != "" && wanted[agent.Name] {
			out = append(out, agent)
		}
	}
	return out
}

// shortMember is a member's short name: the herdr name with the team's own prefix
// taken off ("lead" for "core-lead"). A name that does not carry the prefix is
// left as it is.
func shortMember(herdrName, teamName string) string {
	return strings.TrimPrefix(herdrName, teamName+"-")
}

// itemOf is one row of the list as the boundary writes it (spec-activity 3).
func itemOf(row activityRow, now time.Time) web.ActivityItem {
	t := row.team
	item := web.ActivityItem{
		Name: t.Name,
		Summary: web.ActivitySummary{
			Definition: row.definition,
			State:      t.State.Slug(),
			Last:       t.Last,
		},
		Error:       row.broken,
		Project:     row.project,
		ProjectName: row.projectName,
		Workspace:   row.workspace,
	}
	// いつから is left out for a working team and for one with no live agent
	// (2.2): a working node has no moment to read, and there is no last turn.
	if since := sinceOf(t, now); since != nil {
		item.Summary.Since = since
	}
	if t.State == activity.Working {
		item.Summary.Working = workingOf(t)
	}
	return item
}

// sinceOf is the row's いつから: how long the state has lasted and when it began,
// in the server's own words (6 A4). It is nil for a working team and for one with
// no live agent.
func sinceOf(t activity.Team, now time.Time) *web.ActivitySince {
	if t.Last == "" || t.State == activity.Working {
		return nil
	}
	at := activity.SinceAt(now, t.At, t.Lower)
	if at == "" {
		return nil
	}
	return &web.ActivitySince{Len: activity.Length(t.Stalled, t.Lower), At: at, Lower: t.Lower}
}

// workingOf is a working team's 誰: the members that are working, and how many of
// its workers are (2.2).
func workingOf(t activity.Team) *web.ActivityWork {
	work := &web.ActivityWork{Members: []string{}}
	for _, node := range t.Nodes {
		if node.State == activity.Active {
			work.Members = append(work.Members, node.Name)
		}
	}
	for _, hired := range t.Hired {
		if hired.State == activity.Active {
			work.Workers++
		}
	}
	return work
}

// One is GET /api/activity/{team}: one team's opened part. ok is false for a team
// that is not running in this project, which the handler answers as a 404.
func (a *activityRead) One(name string) (web.ActivityDoc, bool) {
	now := a.clock()
	read := a.readHerdr(now)
	if read.err != nil {
		// A herdr that cannot be read cannot say which teams are running, so
		// there is no such team to open. The list is the answer that carries the
		// sentence (2.7).
		return web.ActivityDoc{}, false
	}
	teams := a.teamsOf(read)
	var one runningTeam
	found := false
	for _, cand := range teams {
		if cand.Name == name {
			one, found = cand, true
			break
		}
	}
	if !found {
		return web.ActivityDoc{}, false
	}

	var graph *definition.Graph
	agents := agentsOf(read, one.Name)
	if one.Built != nil {
		graph = one.Built.team.Definition.Graph
	} else {
		// A definition that does not read leaves the live agents as the only
		// members known (2.2), so they stand in the right-hand column.
		graph = definition.NewGraph(membersOf(agents), nil, nil)
	}
	in := activity.Input{Team: one.Name, Graph: graph, Agents: agents, Seen: read.seen, Now: now}
	t := activity.Of(in)
	// The pane of the row's own member is what the left column shows (2.4), and
	// which member that is only the judgement says, so the team is judged once
	// to find them and, with their pane in hand, judged again.
	main := mainAgent(t, in.Agents)
	if main != "" {
		if text, err := herdrPane(main); err == nil {
			in.Tails = map[string]string{t.Last: text}
			t = activity.Of(in)
		}
	}

	// What each agent's terminal says it is doing, and which member added a
	// worker: the title herdr strips for us (d-b4b9), and the hire lines of the
	// launch record. The title stands as herdr gives it, whatever the state.
	titles := map[string]string{}
	for _, agent := range read.agents {
		if agent.Title != "" {
			titles[agent.Name] = agent.Title
		}
	}
	doc := web.ActivityDoc{
		Name:        one.Name,
		Definition:  one.Definition,
		Project:     one.Project,
		ProjectName: one.ProjectName,
		Workspace:   web.ActivitySpace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		State:       t.State.Slug(),
		Last:        t.Last,
		Nodes:       docNodes(t, now, titles),
		Hired:       docHired(t, one.Name, now, titles, hireSources(one.Project, one.Name)),
		Questions:   []web.ActivityQuestion{},
		Tail:        web.ActivityTail{Lines: []string{}, Shown: tailShown},
		Gy:          web.ActivityNote{OK: true},
		Error:       one.Broken,
	}
	if since := sinceOf(t, now); since != nil {
		doc.Since = since
	}
	if main != "" {
		doc.Herdr = main
		doc.Role = roleOf(graph, t.Last)
		if len(t.Tail) > 0 {
			doc.Tail.Lines = t.Tail
		}
		projectEnv := a.env
		if scoped, err := a.env.AtProject(one.Project); err == nil {
			projectEnv = scoped
		}
		questions, err := gycli.OpenQuestionsBy(projectEnv.ProjectRoot, []string{main})
		if err != nil {
			// gy is there and cannot be read: the question's place says so in
			// one line rather than showing no question (2.4).
			doc.Gy = web.ActivityNote{OK: false, Text: err.Error()}
		} else {
			doc.Questions = questionsOf(questions, now)
		}
	}
	// With no member the line is about there is no question and no pane to show:
	// a working team's opened part is the right-hand column alone (2.4, 2.5).
	return doc, true
}

// mainAgent is the herdr name of the member the line is about: the one the
// judgement named last, as herdr knows it. Empty when the team has no such
// member (a working team, or no live agent at all).
func mainAgent(t activity.Team, agents []activity.Agent) string {
	if t.Last == "" {
		return ""
	}
	for _, agent := range agents {
		if agent.Node == t.Last && agent.Herdr != "" {
			return agent.Herdr
		}
	}
	return ""
}

// roleOf is the role team.yaml gives a member (2.4: the heading's ロール).
func roleOf(graph *definition.Graph, name string) string {
	member, ok := graph.Member(name)
	if !ok {
		return ""
	}
	return member.Role
}

// docNodes is a team's members as the opened part shows them (2.5): the state,
// when it began, and what the agent's terminal says it is doing (d-b4b9).
func docNodes(t activity.Team, now time.Time, titles map[string]string) []web.ActivityNode {
	out := make([]web.ActivityNode, 0, len(t.Nodes))
	for _, node := range t.Nodes {
		at := activity.Clock(now, node.At)
		out = append(out, web.ActivityNode{
			Name: node.Name, Herdr: node.Herdr, State: node.State.Slug(), At: at,
			SinceAt: activity.SinceAt(now, node.At, node.Lower),
			Doing:   titles[node.Herdr],
		})
	}
	return out
}

// docHired is a team's workers as the opened part shows them: the name without
// the team's own prefix (X1, 2.5), the member that added it, and its title.
func docHired(t activity.Team, teamName string, now time.Time, titles, from map[string]string) []web.ActivityNode {
	out := make([]web.ActivityNode, 0, len(t.Hired))
	for _, hired := range t.Hired {
		out = append(out, web.ActivityNode{
			Name:  shortMember(hired.Herdr, teamName),
			Herdr: hired.Herdr,
			State: hired.State.Slug(),
			At:    activity.Clock(now, hired.At),
			Doing: titles[hired.Herdr],
			From:  from[hired.Herdr],
		})
	}
	return out
}

// hireSources is the member that added each of one team's workers, from the hire
// lines of this machine's launch record (d-b4b9). The newest line for one worker
// wins; a worker hired before those lines were written has none, and the screen
// says no "が追加" for it.
func hireSources(project, teamName string) map[string]string {
	out := map[string]string{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	records, err := log.Read()
	if err != nil {
		return out
	}
	for _, record := range records {
		if !record.IsHire() || record.Hire == nil || record.Team != teamName {
			continue
		}
		if record.Project != "" && !samePath(record.Project, project) {
			continue
		}
		if record.Hire.Worker != "" && record.Hire.From != "" {
			out[record.Hire.Worker] = record.Hire.From
		}
	}
	return out
}

// questionsOf is the questions the row's own member asked, newest first, with how
// long ago each was asked in the server's words (2.4, 6 A4). The screen shows one
// of them and the rest in a line each, so all of them are sent.
func questionsOf(questions []activity.Question, now time.Time) []web.ActivityQuestion {
	// 新しい順 (2.4): gy lists them newest first already, and a question gy does
	// not date keeps the place it was listed in, at the end.
	sorted := slices.Clone(questions)
	slices.SortStableFunc(sorted, func(a, b activity.Question) int { return b.At.Compare(a.At) })
	out := make([]web.ActivityQuestion, 0, len(sorted))
	for _, question := range sorted {
		out = append(out, web.ActivityQuestion{
			ID:      question.ID,
			Title:   question.Title,
			Ago:     activity.Ago(now, question.At),
			Options: question.Options,
		})
	}
	return out
}

// herdrPane reads what one member's terminal shows, the way hergy status reads a
// screen (herdr agent read --source detection). An error is a pane that is not
// there to read, which leaves the screen's last lines out (2.4).
func herdrPane(name string) (string, error) {
	return herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))).Read(name)
}
