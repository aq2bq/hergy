package ops

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// This file is the reading behind the activity screen (spec-activity, r-8a67):
// which teams of this project are running, how each one is doing, and, for one
// team tile, its members and workers. It is here rather than in
// internal/web because this layer reads herdr (6 A1);
// the screen is handed the answer.
//
// The reading of herdr is shared with hergy status: both call readRunningTeams,
// which is runningTeams plus each team's built definition (6 A1).

type herdrNow struct {
	tabs       []team.Tab
	agents     []protocol.Agent
	agentsRead bool
	// spaces are herdr's workspaces: a team's tab names the one it stands in,
	// and the headquarters offers them to start a team in (d-73fc).
	spaces []herdr.Workspace
	seen   map[string]activity.Watch
	err    error
}

// Reader is the shared reading behind a headquarters and activity screen. Herdr is read for each
// request; the watcher remembers when each agent's state was first seen (6 A2).
type Reader struct {
	env team.Env
	// port is the serve's socket-backed Herdr port. Tests and CLI readers without
	// one keep the command port.
	port herdr.Port
	// now is the clock: a test places the moments with it. A serve built without
	// one reads the serve's own clock (serveClock).
	now func() time.Time
	// ctx is the serve's own lifetime: a start from the screens stops waiting
	// when the serve st hq is the headquarters reading built on this one, so
	// the screens' running teams and the activity are one reading (r-b7e1).
	ctx context.Context
	hq  *hqRead
	// reader is how this reading opens the settings files: a serve hands it one
	// reading that holds each file under its version (d-e00b); a command uses
	// the disk as it always did.
	reader source.Reader
	// envMu holds the projects this serve has placed, so the git command that
	// finds a project's root runs once per project and not once per request
	// (d-e00b).
	envMu sync.Mutex
	envs  map[domainplace.Project]team.Env

	watcherMu sync.Mutex
	watcher   *activity.Watcher

	// runs is the live reading the prompt owner shares (⑤ r-ba9b): one herdr
	// reading for two seconds, so a prompt and a check in the same moment do
	// not read it twice.
	runs runningHold
}

// newActivityRead is the reading behind one hergy serve. The watcher's memory is
// kept in this machine's state directory, so a serve that is started again reads
// a stop from when it was first seen rather than from its own start (6 A2).
func newActivityRead(env team.Env) *Reader {
	return newActivityReadWith(env, source.Disk{})
}

// newActivityReadWith is this reading over one reader: a serve hands every
// project the same version-keyed reader, so a file is opened once and read
// again only when it changed (d-e00b).
func newActivityReadWith(env team.Env, reader source.Reader) *Reader {
	read := &Reader{env: env, reader: reader, watcher: activity.NewWatcherAt(activityStatePath()), envs: map[domainplace.Project]team.Env{}}
	read.hq = newHQRead(read)
	return read
}

// envAt is one project's environment, placed once: the git command that finds
// the root is not run again for every answer (d-e00b). A project whose path is
// new is placed on its first use.
func (a *Reader) envAt(want domainplace.Project) (team.Env, error) {
	a.envMu.Lock()
	if env, ok := a.envs[want]; ok {
		a.envMu.Unlock()
		return env, nil
	}
	a.envMu.Unlock()
	env, err := a.env.AtProject(want)
	if err != nil {
		return team.Env{}, err
	}
	a.envMu.Lock()
	if a.envs == nil {
		a.envs = map[domainplace.Project]team.Env{}
	}
	a.envs[want] = env
	a.envMu.Unlock()
	return env, nil
}

// fileReader is how this reading opens files: the reader it was built with, or
// the disk when a test built one without it.
func (a *Reader) fileReader() source.Reader {
	if a.reader == nil {
		return source.Disk{}
	}
	return a.reader
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
func (a *Reader) clock() time.Time {
	if a.now == nil {
		return time.Now()
	}
	return a.now()
}

func (a *Reader) herdrPort() herdr.Port {
	if a.port != nil {
		return a.port
	}
	return herdrcli.New(os.Getenv("HERGY_HERDR"))
}

// readHerdr reads the current Herdr state. The serve's socket port reads all four
// lists with one session.snapshot; the command-backed path remains for CLI reads.
func (a *Reader) readHerdr(now time.Time) herdrNow {
	read := herdrNow{}
	if snapshotReader, ok := a.herdrPort().(herdr.SnapshotReader); ok {
		snapshot, err := snapshotReader.SessionSnapshot()
		if err != nil {
			read.err = err
			return read
		}
		read.tabs = TeamTabs(snapshot.Tabs, snapshot.Agents, snapshot.Panes)
		read.agents = snapshot.Agents
		read.agentsRead = true
		read.spaces = snapshot.Workspaces
	} else {
		tabs, agents, agentsRead, err := HerdrRead()
		read = herdrNow{tabs: tabs, agents: agents, agentsRead: agentsRead, err: err}
		if err != nil {
			return read
		}
		read.spaces, _ = HerdrSpaceList()
	}
	a.watcherMu.Lock()
	read.seen = a.watcher.Observe(watchedAgents(read.agents), now)
	a.watcherMu.Unlock()
	return read
}

// watchedAgents is the live agents as the watcher reads them: the name herdr
// knows each by, its state, and its state_change_seq. Every agent herdr reports
// is taken, not only this project's teams' members, because the watcher remembers
// one machine's agents by name (6 A2).
func watchedAgents(agents []protocol.Agent) []activity.Agent {
	out := make([]activity.Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Name.String() == "" {
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
func (a *Reader) teamsOf(read herdrNow) []runningTeam {
	return readRunningTeamsIn(a.projectViews(), read, false)
}

// projectViews is the projects this serve reads, with their names, layers and
// catalogs: the listed ones in the config's order, or the project serve was
// started in when the config names none.
func (a *Reader) projectViews() []projectView {
	return ProjectViews(a.env, a.fileReader(), a.envAt)
}

// List is GET /api/activity: the running teams of this project, in the order of
// activity.Sort, and the band's two numbers. It reads herdr and the launch record
// and the team definitions, and neither gy nor a pane (spec-activity 3).
func (a *Reader) List() ActivityList {
	now := a.clock()
	list := ActivityList{Items: []ActivityItem{}, Herdr: ActivityNote{OK: true}}
	read := a.readHerdr(now)
	if read.err != nil {
		// herdr is not there: the screen says so in herdr's own words and the
		// band writes no number (2.7).
		list.Herdr = ActivityNote{OK: false, Text: i18n.T("web.activity.herdr_unavailable")}
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
			place:       one.Place,
			workspace:   ActivitySpace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
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
	project     domainplace.Project
	projectName string
	place       domainplace.Place
	workspace   ActivitySpace
}

// orderedRows sorts the rows the way the screen lists them (2.2: the teams
// waiting for a human first, then the stopped ones, then the working ones) and
// turns them into the list's items, counting the band's two numbers on the way:
// the running teams, and the members that are waiting for the human.
func orderedRows(rows []activityRow, now time.Time) ([]ActivityItem, int, int) {
	judged := make([]activity.Team, len(rows))
	byName := make(map[string]activityRow, len(rows))
	for i, row := range rows {
		judged[i] = row.team
		byName[row.team.Name.String()] = row
	}
	activity.Sort(judged)
	items := make([]ActivityItem, 0, len(judged))
	waiting := 0
	for _, judged := range judged {
		row := byName[judged.Name.String()]
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
// itemOf is one row of the list as the boundary writes it (spec-activity 3).
func itemOf(row activityRow, now time.Time) ActivityItem {
	t := row.team
	item := ActivityItem{
		Name: t.Name.String(),
		Summary: ActivitySummary{
			Definition: row.definition,
			State:      t.State.Slug(),
		},
		Error:       row.broken,
		Project:     row.project,
		ProjectName: row.projectName,
		Place:       row.place.API(),
		Workspace:   row.workspace,
	}
	// いつから is left out for a working team and for one with no live agent
	// (2.2): a working node has no moment to read, and there is no last turn.
	if since := sinceOf(t, now); since != nil {
		item.Summary.Since = since
	}
	item.Summary.Turn = turnOf(t.Name)
	return item
}

// turnOf is the tile's turn: the holder hergy's watcher keeps for this team
// (r-bc26), or nil for a team the watcher has not seen.
func turnOf(teamName domainname.Seen) *ActivityTurn {
	path := activityStatePath()
	if path == "" {
		return nil
	}
	st, err := turns.Load(turns.Path(filepath.Dir(path), teamName.String()), teamName)
	if err != nil || st.Since.IsZero() {
		return nil
	}
	return &ActivityTurn{Holder: st.Holder, From: st.From, Why: st.Why, Text: turnText(st)}
}

// turnText is the turn line in the server's words: the holder and who handed it
// over, whether the holder is being asked who its result is for, why the human
// holds it when hergy handed it to the person, and who waits for whose answer.
func turnText(st *turns.State) string {
	from := st.From
	if from == turns.Human {
		from = i18n.T("activity.turn_the_human")
	}
	var text string
	switch {
	case st.Holder != turns.Human && st.Asked[st.Holder].Seq != 0:
		text = i18n.Tf("activity.turn_holder_asked", st.Holder, from)
	case st.Holder != turns.Human:
		text = i18n.Tf("activity.turn_holder", st.Holder, from)
	default:
		why := ""
		if st.Why != "" {
			why = i18n.T("status.turn_why_" + st.Why)
		}
		text = i18n.Tf("activity.turn_human", from, why)
	}
	if waits := st.Waits(); len(waits) > 0 {
		text += i18n.T("list.separator") + i18n.Tf("activity.turn_waits", strings.Join(waits, i18n.T("list.separator")))
	}
	return text
}

// sinceOf is the row's いつから: how long the state has lasted and when it began,
// in the server's own words (6 A4). It is nil for a working team and for one with
// no live agent.
func sinceOf(t activity.Team, now time.Time) *ActivitySince {
	if t.Last == "" || t.State == activity.Working {
		return nil
	}
	at := activity.SinceAt(now, t.At, t.Lower)
	if at == "" {
		return nil
	}
	return &ActivitySince{Len: activity.Length(t.Stalled, t.Lower), At: at, Lower: t.Lower}
}

// StopTargets describes the panes the stop operation would close.
// One is GET /api/activity/{team}: one team's opened part. ok is false for a team
// that is not running in this project, which the handler answers as a 404.
// StopTargets is the stop's own answer, read without closing anything: the
// members and worker panes Stop would close (r-ccf8).
func (a *Reader) StopTargets(name string) ([]string, int, bool) {
	if a.hq == nil {
		return nil, 0, false
	}
	read := a.hq.act.readHerdr(a.hq.act.clock())
	one, found := a.hq.findRunning(read, name)
	if !found {
		return nil, 0, false
	}
	env, err := a.hq.act.envAt(one.Project)
	if err != nil {
		return nil, 0, false
	}
	target := a.hq.stopTargets(env, one, read)
	return target.members, len(target.workers), true
}

// One reads one team's members, workers and current question.
func (a *Reader) One(name string) (ActivityDoc, bool) {
	now := a.clock()
	read := a.readHerdr(now)
	if read.err != nil {
		// A herdr that cannot be read cannot say which teams are running, so
		// there is no such team to open. The list is the answer that carries the
		// sentence (2.7).
		return ActivityDoc{}, false
	}
	teams := a.teamsOf(read)
	var one runningTeam
	found := false
	for _, cand := range teams {
		if cand.Name.String() == name {
			one, found = cand, true
			break
		}
	}
	if !found {
		return ActivityDoc{}, false
	}

	var graph *definition.Graph
	agents := agentsOf(read, one.Name)
	if one.Built != nil {
		graph = one.Built.Team.Definition.Graph
	} else {
		// A definition that does not read leaves the live agents as the only
		// members known (2.2), so they stand in the right-hand column.
		graph = nil
	}
	in := activity.Input{Team: one.Name, Graph: graph, Agents: agents, Seen: read.seen, Now: now}
	t := activity.Of(in)
	// Keep the one line the waiting row already shows, without returning the
	// pane's tail to the screen. The opened tile itself needs no pane content.
	main := mainAgent(t, in.Agents)
	question := ""
	if main.String() != "" && (t.State == activity.WaitingAnswer || t.State == activity.WaitingTurn) {
		if text, err := a.herdrPane(main); err == nil {
			question = askingLine(text)
			if t.State == activity.WaitingAnswer {
				question = permissionQuestion(text)
			}
		}
	}
	// A member waiting for an answer whose screen herdr cannot read, or whose
	// screen holds no line, still has a question: the row says the answer is not
	// on this screen rather than leaving the field empty (r-1db7 L5). The screen
	// writes no sentence of its own (spec-activity 3).
	if question == "" && t.State == activity.WaitingAnswer {
		question = i18n.T("web.activity.question_unread")
	}

	// What each agent's terminal says it is doing, and which member added a
	// worker: the title herdr strips for us (d-b4b9), and the hire lines of the
	// launch record. The title stands as herdr gives it, whatever the state.
	titles := map[string]string{}
	for _, agent := range read.agents {
		if agent.Title != "" {
			titles[agent.Name.String()] = agent.Title
		}
	}
	doc := ActivityDoc{
		Nodes:    docNodes(t, now, titles),
		Hired:    docHired(t, one.Name, now, titles, hireSources(one.Project, one.Name.String())),
		Question: question,
	}
	for i := range doc.Nodes {
		node := &doc.Nodes[i]
		if node.State != "blocked" && node.State != "handed" {
			continue
		}
		text, err := a.herdrPane(node.Herdr)
		if err != nil {
			node.Question = i18n.T("web.activity.question_unread")
			continue
		}
		node.Question = askingLine(text)
		if node.State == "blocked" {
			node.Question = permissionQuestion(text)
		}
		if node.Question == "" {
			node.Question = i18n.T("web.activity.question_unread")
		}
	}
	// With no member the line is about, there is no question: a working team's
	// opened part is the right-hand column alone (2.5).
	return doc, true
}

// askingLine keeps the legacy team-level summary. Each waiting member also
// carries a focused question or permission block, without terminal history.
func askingLine(text string) string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "●"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	for _, line := range lines {
		for _, mark := range []string{"よいか", "ますか", "ですか", "?", "？"} {
			if strings.Contains(line, mark) {
				return line
			}
		}
	}
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

// permissionQuestion exposes only a recognized approval block, never the preceding
// terminal history. Unknown or oversized prompts stay in the terminal.
var permissionYes = regexp.MustCompile(`(?i)^(?:[›❯>]\s*)?1\.\s+Yes(?:$|[, .].*)`)
var permissionAgain = regexp.MustCompile(`(?i)^(?:[›❯>]\s*)?2\.\s+Yes(?:$|[, .].*)`)
var permissionNo = regexp.MustCompile(`(?i)^(?:[›❯>]\s*)?3\.\s+No(?:$|[, .].*)`)

func permissionQuestion(text string) string {
	var lines []string
	start, yes, no := -1, -1, -1
	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		// Here-documents can contain literal headings and menu choices. Text alone
		// cannot distinguish those from the terminal UI, so leave them in the terminal.
		if strings.Contains(line, "<<") {
			return ""
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		i := len(lines)
		lines = append(lines, strings.TrimRight(line, " \t"))
		lower := strings.ToLower(trimmed)
		if lower == "would you like to run the following command?" || lower == "bash command" || lower == "permission required" {
			if start >= 0 {
				return ""
			}
			start = i
		}
		if permissionYes.MatchString(trimmed) {
			if yes >= 0 {
				return ""
			}
			yes = i
		}
		if permissionNo.MatchString(trimmed) {
			if no >= 0 {
				return ""
			}
			no = i
		}
	}
	if start < 0 || yes <= start+1 || no != len(lines)-1 || no <= yes {
		return ""
	}
	// The choices must be consecutive at the tail, never separated by command text
	// or output from an earlier answered prompt.
	if no != yes+1 && (no != yes+2 || !permissionAgain.MatchString(strings.TrimSpace(lines[yes+1]))) {
		return ""
	}
	result := strings.Join(lines[start:], "\n")
	if len(lines)-start > 24 || len(result) > 4000 {
		return ""
	}
	return result
}

// mainAgent is the herdr name of the member the line is about: the one the
// judgement named last, as herdr knows it. Empty when the team has no such
// member (a working team, or no live agent at all).
func mainAgent(t activity.Team, agents []activity.Agent) domainname.Seen {
	if t.Last == "" {
		return domainname.Seen{}
	}
	for _, agent := range agents {
		if agent.Node.String() == t.Last && agent.Herdr.String() != "" {
			return agent.Herdr
		}
	}
	return domainname.Seen{}
}

// docNodes is a team's members as the opened part shows them (2.5): the state,
// when it began, and what the agent's terminal says it is doing (d-b4b9).
func docNodes(t activity.Team, now time.Time, titles map[string]string) []ActivityNode {
	out := make([]ActivityNode, 0, len(t.Nodes))
	for _, node := range t.Nodes {
		at := activity.Clock(now, node.At)
		out = append(out, ActivityNode{
			Name: node.Name, Herdr: node.Herdr, State: node.State.Slug(), At: at,
			SinceAt: activity.SinceAt(now, node.At, node.Lower),
			Doing:   titles[node.Herdr.String()],
		})
	}
	return out
}

// docHired is a team's workers as the opened part shows them: the name without
// the team's own prefix (X1, 2.5), the member that added it, and its title.
func docHired(t activity.Team, teamName domainname.Seen, now time.Time, titles, from map[string]string) []ActivityNode {
	out := make([]ActivityNode, 0, len(t.Hired))
	for _, hired := range t.Hired {
		out = append(out, ActivityNode{
			Name:  domainname.ShortIn(hired.Herdr, teamName).String(),
			Herdr: hired.Herdr,
			State: hired.State.Slug(),
			At:    activity.Clock(now, hired.At),
			Doing: titles[hired.Herdr.String()],
			From:  from[hired.Herdr.String()],
		})
	}
	return out
}

// hireSources is the member that added each of one team's workers, from the hire
// lines of this machine's launch record (d-b4b9). The newest line for one worker
// wins; a worker hired before those lines were written has none, and the screen
// says no "が追加" for it.
func hireSources(project domainplace.Project, teamName string) map[string]string {
	out := map[string]string{}
	records, err := ReadLaunchRecords()
	if err != nil {
		return out
	}
	for _, record := range records {
		if !record.IsHire() || record.Hire == nil || record.Team.String() != teamName {
			continue
		}
		if !record.Project.Project().IsZero() && record.Project.Project().Compare(project) != domainplace.Same {
			continue
		}
		if record.Hire.Worker.String() != "" && record.Hire.From != "" {
			out[record.Hire.Worker.String()] = record.Hire.From
		}
	}
	return out
}

// herdrPane reads what one member's terminal shows so the waiting row can keep
// its one-line question (herdr agent read --source detection). An error leaves
// that line empty.
func (a *Reader) herdrPane(name domainname.Seen) (string, error) {
	return herdr.New(a.herdrPort()).Read(name)
}

// FocusMember rechecks membership before moving focus; it never answers a prompt.
func (a *Reader) FocusMember(teamName, member string) error {
	return hqOperationError(Focus(context.Background(), FocusInput{MemberInput: MemberInput{Env: a.env, Member: member, Port: a.herdrPort()}, Policy: FocusLive, TeamName: teamName, Views: a.projectViews()}))
}
