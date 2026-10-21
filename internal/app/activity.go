package app

import (
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
	// Elsewhere says the definition is not in this project's catalog at all:
	// it belongs to another project, which hergy status says in its own words.
	Elsewhere bool
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
	running := team.RunningTeams(read.tabs, launchTeams(), read.agentsRead)
	cat := env.Catalog(source.Disk{})
	out := make([]runningTeam, 0, len(running))
	for _, entry := range running {
		one := runningTeam{Name: entry.Team, Definition: entry.Definition}
		r, err := buildRunning(cat, env.ProjectRoot, entry)
		if err != nil {
			// A definition that does not read is not this reading's failure: the
			// row says why in the definition's own words (2.2), as hergy status
			// says it, so the reason is kept on the team rather than returned.
			one.Broken = brokenTeamLine(env, err)
			one.Elsewhere = errors.Is(err, catalog.ErrNotFound)
			out = append(out, one)
			continue
		}
		one.Built = r
		out = append(out, one)
	}
	return out
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

// ofThisProject reports whether one running team is this project's, which is the
// only filter the activity screen keeps (spec-activity 1): the launch record says
// which project a team was launched from, and a team with no record belongs to
// this project when this project's own catalog holds its definition.
func ofThisProject(one runningTeam, projects map[string]string, cat *catalog.Catalog, root string) bool {
	if project, ok := projects[one.Name]; ok {
		return samePath(project, root)
	}
	_, err := cat.Definition(one.Definition, source.LayerLocal)
	return err == nil
}

// launchedProjects is the project each team was launched from, by team name, from
// the newest record of that team (spec-activity 1). A team this machine has no
// record of is not in the map.
func launchedProjects() map[string]string {
	log, err := launchlog.Open()
	if err != nil {
		return nil
	}
	records, err := log.Read()
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(records))
	for _, record := range records {
		// The record is history, oldest first: the last record of a team is the
		// newest one.
		if record.Team != "" && record.Project != "" {
			out[record.Team] = record.Project
		}
	}
	return out
}

// herdrNow is one read of herdr as the activity needs it: the tabs, each with the
// live agents in it, the live agents themselves, whether the agent list could be
// read at all, when each agent's state was first seen, and, when herdr could not
// be reached at all, why not (2.7).
type herdrNow struct {
	tabs       []team.Tab
	agents     []protocol.Agent
	agentsRead bool
	seen       map[string]activity.Watch
	at         time.Time
	err        error
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

	mu      sync.Mutex
	herdr   herdrNow
	watcher *activity.Watcher
}

// newActivityRead is the reading behind one hergy serve. The watcher's memory is
// kept in this machine's state directory, so a serve that is started again reads
// a stop from when it was first seen rather than from its own start (6 A2).
func newActivityRead(env team.Env) *activityRead {
	return &activityRead{env: env, watcher: activity.NewWatcherAt(activityStatePath())}
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
		read.seen = a.watcher.Observe(watchedAgents(agents), now)
	}
	a.herdr = read
	return read
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

// teamsOf is this project's running teams, in herdr's order, with their
// definitions built, from one read of herdr. A team of another project running
// on this machine is not the activity screen's (spec-activity 1).
func (a *activityRead) teamsOf(read herdrNow) []runningTeam {
	all := readRunningTeams(a.env, read)
	projects := launchedProjects()
	cat := a.env.Catalog(source.Disk{})
	out := make([]runningTeam, 0, len(all))
	for _, one := range all {
		if ofThisProject(one, projects, cat, a.env.ProjectRoot) {
			out = append(out, one)
		}
	}
	return out
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
			team:       judgeOf(one, read, now),
			definition: one.Definition,
			broken:     one.Broken,
		})
	}
	list.Items, list.Running, list.Waiting = orderedRows(rows, now)
	return list
}

// activityRow is one row of the list: the judgement of one team and what the wire
// needs beside it.
type activityRow struct {
	team       activity.Team
	definition string
	broken     string
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
		Error: row.broken,
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

	doc := web.ActivityDoc{
		Name:       one.Name,
		Definition: one.Definition,
		State:      t.State.Slug(),
		Last:       t.Last,
		Nodes:      docNodes(t, now),
		Hired:      docHired(t, one.Name, now),
		Questions:  []web.ActivityQuestion{},
		Tail:       web.ActivityTail{Lines: []string{}, Shown: tailShown},
		Gy:         web.ActivityNote{OK: true},
		Error:      one.Broken,
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
		questions, err := gycli.OpenQuestionsBy(a.env.ProjectRoot, []string{main})
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

// docNodes is a team's members as the opened part shows them (2.5).
func docNodes(t activity.Team, now time.Time) []web.ActivityNode {
	out := make([]web.ActivityNode, 0, len(t.Nodes))
	for _, node := range t.Nodes {
		out = append(out, web.ActivityNode{
			Name: node.Name, Herdr: node.Herdr, State: node.State.Slug(), At: activity.Clock(now, node.At),
		})
	}
	return out
}

// docHired is a team's workers as the opened part shows them: the name without
// the team's own prefix (X1, 2.5).
func docHired(t activity.Team, teamName string, now time.Time) []web.ActivityNode {
	out := make([]web.ActivityNode, 0, len(t.Hired))
	for _, hired := range t.Hired {
		out = append(out, web.ActivityNode{
			Name:  shortMember(hired.Herdr, teamName),
			Herdr: hired.Herdr,
			State: hired.State.Slug(),
			At:    activity.Clock(now, hired.At),
		})
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
