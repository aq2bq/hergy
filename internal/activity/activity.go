// Package activity judges how a team is doing: whether it waits for the human,
// is stopped, or is working, and the one line a person reads (C5, 設計 5.2). It
// reads nothing itself — no herdr, no gy, no file — the caller hands it what it
// read and it returns the judgement, so the judgement can be tested with
// made-up input.
package activity

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// tailLimit is how much of a pane's tail the screen keeps (6 A6): the last lines,
// with the empty ones at the end dropped.
const tailLimit = 40

// State is how a team is doing (設計 5.2).
type State int

const (
	// WaitingAnswer is a node blocked on a permission or an input: the human
	// answers and the node goes on.
	WaitingAnswer State = iota
	// WaitingTurn is every agent stopped, with the node that stopped last
	// handing the turn to the human.
	WaitingTurn
	// Stalled is every agent stopped, with the node that stopped last handing
	// the turn to no one (or there being no agent at all).
	Stalled
	// Working is at least one agent working.
	Working
)

// NodeState is one node's live state, for the figure's overlay.
type NodeState int

const (
	// Absent is a node no agent is live for.
	Absent NodeState = iota
	// Idle is an agent stopped (idle, done, or a state herdr does not know).
	Idle
	// Active is an agent working.
	Active
	// Blocked is an agent waiting for the human (a permission or an input).
	Blocked
	// HandedOver is the node that handed the turn to the human, with the team
	// waiting for it.
	HandedOver
)

// Agent is one live agent of the team, as the caller read it from herdr.
type Agent struct {
	// Herdr is the name herdr knows the agent by: "<team>-<member>".
	Herdr string
	// Node is the graph node the agent is, by its name: a member's short name.
	// Empty means the agent is no node of this graph (a hired worker).
	Node string
	// Status is herdr's agent_status: working, idle, blocked, done, or unknown.
	Status string
	// Seq is herdr's state_change_seq: the greater it is, the more recently the
	// agent changed state.
	Seq int
}

// Watch is when the caller first saw an agent's current state.
type Watch struct {
	At time.Time
	// Lower says that time is when the caller began watching: the agent was
	// already in that state, so the real length may be longer.
	Lower bool
}

// Question is one open question from gy. 段 6 reads it; the judgement only
// carries it, and an empty list is the common case (hergy does not need gy).
type Question struct {
	ID      string
	Title   string
	Options []string
	Decider string
	At      time.Time
	// Writer is who asked it, as gy's write unit names them. The activity
	// shows the question on a team's own section when the writer is one of that
	// team's herdr names (設計 5.3, W4).
	Writer string
}

// Input is everything the judgement reads (設計 5.2).
type Input struct {
	// Team is the team's name.
	Team string
	// Graph is the definition's graph: which nodes hand the turn to the human.
	Graph *definition.Graph
	// Agents are the live agents of the team. An agent whose Node is empty (a
	// hired worker) counts for whether the team works or waits, and stands in
	// the line by its herdr name.
	Agents []Agent
	// Tails is each node's pane tail, by node name, for the node the line is
	// about. A node with no tail has none.
	Tails map[string]string
	// Questions are gy's open questions (段 6); empty is fine.
	Questions []Question
	// Seen is when each agent's current state was first seen, by herdr name.
	Seen map[string]Watch
	// Now is when the judgement is made.
	Now time.Time
}

// Node is one node of the team, with its live state.
type Node struct {
	Name string
	// Human says the node hands the turn to the human.
	Human bool
	State NodeState
	// Herdr is the live agent's name, empty when none.
	Herdr string
	Seq   int
	// At is when this state began, as the caller first saw it (6 A2), for the
	// states the screen shows a moment for: a node that is stopped or waiting
	// for the human. It is zero for a working node and for one with no agent.
	At time.Time
}

// Team is one team's activity (設計 5.2).
type Team struct {
	Name  string
	State State
	// Line is the one line the section shows.
	Line string
	// Last is the node the line is about: the blocked node for a wait on an
	// answer, and the node that stopped last (the last turn) for the two
	// stopped states. It is empty when no agent is live.
	Last string
	// Stalled is how long the node the line is about has been where it is.
	Stalled time.Duration
	// Lower says Stalled is a lower bound: the state was already there when the
	// caller began watching.
	Lower bool
	// At is when that state began, as the caller first saw it (6 A2), for the
	// row's "13:34 から"/"13:34 より前". It is zero when no agent is live.
	At time.Time
	// Nodes is the graph's nodes: the ones waiting for the human first, then the
	// working ones, then the stopped ones, then the ones with no agent.
	Nodes []Node
	// Tail is the pane tail of the node the line is about, as the lines the
	// screen shows: the last 40 lines, with the empty ones at the end dropped
	// (6 A6). It is empty when there is no tail.
	Tail []string
	// Hired is the live agents that are no node of the graph, by the herdr name
	// herdr knows them by: the workers a member hired (3.2). The screen lists
	// each under the workers: node of the member that hired it.
	Hired []Node
	// Questions are gy's open questions (段 6).
	Questions []Question
}

// Of judges one team. Nothing is read: everything comes from in.
func Of(in Input) Team {
	lives := livesOf(in)
	team := Team{Name: in.Team, Questions: in.Questions}

	working := filter(lives, Active)
	blocked := filter(lives, Blocked)
	stopped := filter(lives, Idle)

	switch {
	case len(blocked) > 0:
		// A node that waits for the human is what the human is asked for, even
		// when another node is working.
		last := newest(blocked)
		length := since(in, last)
		team.State = WaitingAnswer
		team.Last = last.name
		team.Stalled, team.Lower = length.d, length.lower
		team.At = last.at
		team.Tail = tailLines(in.Tails[last.name])
		team.Line = i18n.Tf("activity.line_stuck", last.name, length.words())
	case len(working) > 0:
		team.State = Working
		team.Line = workingLine(lives)
	case len(stopped) == 0:
		// No agent is live at all: nothing has been running, so there is no
		// length to say and no last turn to name (N4).
		team.State = Stalled
		team.Line = i18n.Tf("activity.line_no_agent", in.Team)
	default:
		// Every live agent is stopped. The node that stopped last decides which
		// wait it is.
		last := newest(stopped)
		length := since(in, last)
		team.Last = last.name
		team.Stalled, team.Lower = length.d, length.lower
		team.At = last.at
		team.Tail = tailLines(in.Tails[last.name])
		if last.human {
			team.State = WaitingTurn
			team.Line = i18n.Tf("activity.line_waiting_human", last.name, length.words())
			break
		}
		team.State = Stalled
		team.Line = i18n.Tf("activity.line_team_stuck", length.words()) +
			i18n.Tf("activity.line_last_turn", last.name)
	}
	if team.State == WaitingTurn {
		// The node that handed the turn over is the one the human answers, so
		// the figure shows it as waiting too.
		for i := range lives {
			if lives[i].name == team.Last {
				lives[i].state = HandedOver
			}
		}
	}
	team.Nodes = nodesOf(in, lives)
	team.Hired = hiredOf(lives)
	return team
}

// Watcher remembers when each agent's current state was first seen: the part of
// 5.2 that has to be remembered between two reads, so a screen only reads herdr
// and hands the agents over (N5). It holds nothing across a program's life
// except what it was told — and the one file NewWatcherAt was given — and it
// uses no goroutine: the caller calls Observe whenever it has read the agents
// (every 5 seconds on the team screen).
type Watcher struct {
	seen map[string]watcherEntry
	// began says Observe has been called before, or the file this watcher was
	// given held agents, so the agents of the first read are the ones that were
	// already there.
	began bool
	// path is the file the memory is kept in, empty when the memory is this
	// process's only (6 A2).
	path string
}

// watcherEntry is one agent as the watcher last saw it.
type watcherEntry struct {
	seq   int
	watch Watch
}

// watcherRecord is one agent as the file writes it: the seq and the watch, so a
// serve that is started again can pick the state up (6 A2).
type watcherRecord struct {
	Seq   int    `json:"seq"`
	At    string `json:"at"`
	Lower bool   `json:"lower"`
}

// NewWatcher makes a watcher that has seen nothing yet.
func NewWatcher() *Watcher {
	return NewWatcherAt("")
}

// NewWatcherAt makes a watcher whose memory is kept in one file, so a serve that
// is started again remembers when an agent stopped: an agent whose seq did not
// move keeps the moment the earlier serve saw (6 A2). An empty path keeps the
// memory in this process only.
//
// A file that cannot be read is forgotten — the watcher begins with what it
// sees, the way a first read does — and a file that cannot be written leaves the
// memory as it is: the screen keeps working, it only forgets across a restart.
func NewWatcherAt(path string) *Watcher {
	w := &Watcher{seen: map[string]watcherEntry{}, path: path}
	w.load()
	return w
}

// load reads the file the watcher keeps its memory in, if there is one.
func (w *Watcher) load() {
	if w.path == "" {
		return
	}
	data, err := os.ReadFile(w.path)
	if err != nil {
		return
	}
	var file map[string]watcherRecord
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}
	for name, record := range file {
		at, err := time.Parse(time.RFC3339, record.At)
		if err != nil {
			continue
		}
		w.seen[name] = watcherEntry{seq: record.Seq, watch: Watch{At: at, Lower: record.Lower}}
	}
	// Agents from before say this is not the first watching: an agent that is
	// not in the file appeared while nobody was watching it, so its state is
	// taken from now on rather than called a lower bound.
	w.began = len(w.seen) > 0
}

// save writes the memory, so the next serve starts with it. Failing is not an
// error: the memory still holds for this process (6 A2).
func (w *Watcher) save() {
	if w.path == "" {
		return
	}
	file := make(map[string]watcherRecord, len(w.seen))
	for name, entry := range w.seen {
		file[name] = watcherRecord{Seq: entry.seq, At: entry.watch.At.Format(time.RFC3339), Lower: entry.watch.Lower}
	}
	data, err := json.Marshal(file)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(w.path, data, 0o644)
}

// Observe takes the agents as they were just read and returns, for each of them,
// when its current state was first seen. An agent the first read saw was already
// in its state when the watching began, so its length is a lower bound; an agent
// that appeared later, or whose state_change_seq moved, is taken from now on. An
// agent that is gone is forgotten. When the memory changes it is written to the
// watcher's file, so the next serve remembers it.
func (w *Watcher) Observe(agents []Agent, now time.Time) map[string]Watch {
	next := make(map[string]watcherEntry, len(agents))
	out := make(map[string]Watch, len(agents))
	changed := len(agents) != len(w.seen)
	for _, agent := range agents {
		seen, ok := w.seen[agent.Herdr]
		switch {
		case ok && seen.seq == agent.Seq:
			next[agent.Herdr] = seen
			out[agent.Herdr] = seen.watch
		default:
			watch := Watch{At: now, Lower: !w.began}
			next[agent.Herdr] = watcherEntry{seq: agent.Seq, watch: watch}
			out[agent.Herdr] = watch
			changed = true
		}
	}
	w.seen, w.began = next, true
	if changed {
		w.save()
	}
	return out
}

// Sort puts teams in the order of 5.2: the teams waiting for a human first, the
// one waiting longest first; then the stopped ones, the one stopped longest
// first; then the working ones. Teams of the same state and length are ordered
// by name, so the list holds still between one draw and the next.
func Sort(teams []Team) {
	slices.SortFunc(teams, func(a, b Team) int {
		if d := cmp.Compare(group(a.State), group(b.State)); d != 0 {
			return d
		}
		if d := cmp.Compare(b.Stalled, a.Stalled); d != 0 {
			return d
		}
		if d := cmp.Compare(a.State, b.State); d != 0 {
			return d
		}
		return cmp.Compare(a.Name, b.Name)
	})
}

// group is where a state stands in the list: the two waits for the human are
// one group (5.2).
func group(state State) int {
	switch state {
	case WaitingAnswer, WaitingTurn:
		return 0
	case Stalled:
		return 1
	default:
		return 2
	}
}

// live is one agent of the team, with the node it is and its state.
type live struct {
	name  string // the node's name, or the herdr name when it is no node
	herdr string
	seq   int
	state NodeState
	human bool      // the node hands the turn to the human
	hired bool      // the agent is no node of the graph: a worker a member hired
	at    time.Time // when this state began, for the states the screen dates
}

// livesOf is the live agents, the graph's members first (in the graph's order,
// the way the figure reads), then the agents with no node.
func livesOf(in Input) []live {
	byNode := map[string]Agent{}
	var others []Agent
	for _, agent := range in.Agents {
		if _, ok := in.Graph.Member(agent.Node); ok {
			byNode[agent.Node] = agent
			continue
		}
		others = append(others, agent)
	}
	lives := make([]live, 0, len(in.Graph.Members)+len(others))
	for i := range in.Graph.Members {
		member := &in.Graph.Members[i]
		l := live{name: member.Name, state: Absent, human: handsToHuman(in.Graph, member.Name)}
		if agent, ok := byNode[member.Name]; ok {
			l.herdr, l.seq, l.state = agent.Herdr, agent.Seq, stateOf(agent.Status)
			l.at = atOf(l.state, in.Seen, agent.Herdr)
		}
		lives = append(lives, l)
	}
	for _, agent := range others {
		state := stateOf(agent.Status)
		lives = append(lives, live{
			name: agent.Herdr, herdr: agent.Herdr, seq: agent.Seq, state: state,
			hired: true, at: atOf(state, in.Seen, agent.Herdr),
		})
	}
	return lives
}

// atOf is when a node's state began, for the states the screen shows a moment
// for: a node that is stopped or waiting for the human. A working node and one
// with no agent have no such moment (2.5), and a caller that handed no watch for
// the agent knows none (6 A2).
func atOf(state NodeState, seen map[string]Watch, herdr string) time.Time {
	if state != Idle && state != Blocked {
		return time.Time{}
	}
	return seen[herdr].At
}

// hiredOf is the live agents that are no node of the graph, by their herdr
// names: the workers a member hired, which the team screen lists under the
// member that hired them (3.2). The order is the nodes' own (6 A3): waiting for
// the human first, then working, then stopped, and the last one to stop first.
func hiredOf(lives []live) []Node {
	var out []Node
	for _, l := range lives {
		if !l.hired {
			continue
		}
		out = append(out, Node{Name: l.name, State: l.state, Herdr: l.herdr, Seq: l.seq, At: l.at})
	}
	slices.SortStableFunc(out, stopOrder)
	return out
}

// nodesOf is the graph's members as nodes, with their states, in the order 5.2
// lists them: waiting for the human first, then working, then stopped, then the
// ones with no agent. Within one state the one that stopped last comes first (6
// A3), so a person reads the freshest stop at the top.
func nodesOf(in Input, lives []live) []Node {
	state := map[string]live{}
	for _, l := range lives {
		state[l.name] = l
	}
	nodes := make([]Node, 0, len(in.Graph.Members))
	for i := range in.Graph.Members {
		member := &in.Graph.Members[i]
		l := state[member.Name]
		nodes = append(nodes, Node{
			Name: member.Name, Human: l.human, State: l.state, Herdr: l.herdr, Seq: l.seq, At: l.at,
		})
	}
	slices.SortStableFunc(nodes, stopOrder)
	return nodes
}

// stopOrder is the order of one state's nodes and workers (6 A3): waiting for
// the human first, then working, then stopped, then the ones with no agent, and
// within one state the one whose state_change_seq is greatest first — it stopped
// last. The name settles two that share a seq, so the list holds still between
// one read and the next.
func stopOrder(a, b Node) int {
	if d := cmp.Compare(nodeRank(a.State), nodeRank(b.State)); d != 0 {
		return d
	}
	if d := cmp.Compare(b.Seq, a.Seq); d != 0 {
		return d
	}
	return cmp.Compare(a.Name, b.Name)
}

// nodeRank is where a node stands in a team's node list (5.2: the nodes waiting
// for the human first).
func nodeRank(state NodeState) int {
	switch state {
	case Blocked, HandedOver:
		return 0
	case Active:
		return 1
	case Idle:
		return 2
	default:
		return 3
	}
}

// workingLine is the one line of a working team: each live node with what it is
// doing, the working ones first ("lead が仕事中・spec が止まっています").
func workingLine(lives []live) string {
	parts := make([]string, 0, len(lives))
	for _, rank := range []NodeState{Active, Idle} {
		for _, l := range lives {
			if l.state == rank {
				parts = append(parts, i18n.Tf("activity.node_state", l.name, stateWord(rank)))
			}
		}
	}
	return strings.Join(parts, i18n.T("activity.node_separator"))
}

// stateWord is what a node is doing, as the line reads it.
func stateWord(state NodeState) string {
	if state == Active {
		return i18n.T("activity.working")
	}
	return i18n.T("activity.stopped")
}

// stateOf is what herdr's agent_status means for a node: the states hergy does
// not know (done, unknown) are stopped.
func stateOf(status string) NodeState {
	switch status {
	case "working":
		return Active
	case "blocked":
		return Blocked
	default:
		return Idle
	}
}

// handsToHuman is whether the graph holds a turn from this member to the human.
func handsToHuman(g *definition.Graph, name string) bool {
	for _, edge := range g.Out(definition.NodeID{Kind: definition.KindMember, Name: name}) {
		if edge.To == (definition.Human{}).ID() {
			return true
		}
	}
	return false
}

// filter is the lives in one state, in the order the list holds them.
func filter(lives []live, state NodeState) []live {
	var out []live
	for _, l := range lives {
		if l.state == state {
			out = append(out, l)
		}
	}
	return out
}

// newest is the agent that changed state last: the greatest state_change_seq,
// and the first of the list when two share one.
func newest(lives []live) live {
	best := lives[0]
	for _, l := range lives[1:] {
		if l.seq > best.seq {
			best = l
		}
	}
	return best
}

// lasted is how long a state has lasted: the length, whether it is a lower bound
// (the state was already there when the watching began), and whether the caller
// knows at all. Nothing is made up when it does not: a line never says a time it
// does not have (N4).
type lasted struct {
	d     time.Duration
	lower bool
	known bool
}

// words is how the length reads in a line.
func (l lasted) words() string {
	if !l.known {
		return i18n.T("activity.unknown_length")
	}
	return durationText(l.d, l.lower)
}

// since is how long the agent has been where it is: from when the caller first
// saw this state, and a lower bound when the caller began watching in that state
// (the agent was already there). A caller that handed no watch for the agent
// knows nothing about the length, and says nothing.
func since(in Input, l live) lasted {
	watch, ok := in.Seen[l.herdr]
	if !ok {
		return lasted{}
	}
	if !in.Now.After(watch.At) {
		return lasted{lower: watch.Lower, known: true}
	}
	return lasted{d: in.Now.Sub(watch.At), lower: watch.Lower, known: true}
}

// durationText is how long a state has lasted, as a person reads it ("25 分",
// "1 時間 5 分", "30 秒"), with "以上" saying it is a lower bound: the state was
// already there when the watching began, so the real length may be longer (5.2).
func durationText(d time.Duration, lower bool) string {
	switch {
	case d >= time.Hour:
		hours, minutes := int(d/time.Hour), int(d%time.Hour/time.Minute)
		if minutes == 0 {
			return markLower(count(hours, "activity.hour", "activity.hours"), lower)
		}
		return markLower(i18n.Tf("activity.hours_minutes",
			count(hours, "activity.hour", "activity.hours"),
			count(minutes, "activity.minute", "activity.minutes")), lower)
	case d >= time.Minute:
		return markLower(count(int(d/time.Minute), "activity.minute", "activity.minutes"), lower)
	default:
		// The seconds of a length under a minute are shown rather than rounded,
		// so a node that just stopped does not read as "0 分".
		return markLower(count(max(int(d/time.Second), 1), "activity.second", "activity.seconds"), lower)
	}
}

// count is one number with the word for it: the singular word when it is one.
// The singular words carry the number themselves ("1 minute"), so nothing is
// formatted into them.
func count(n int, singular, plural string) string {
	if n == 1 {
		return i18n.T(singular)
	}
	return i18n.Tf(plural, n)
}

// markLower says a length is a lower bound.
func markLower(text string, lower bool) string {
	if !lower {
		return text
	}
	return i18n.Tf("activity.or_more", text)
}

// tailLines is a pane's tail as the screen shows it (6 A6): the lines, with the
// empty lines at the end dropped, and the last 40 of what is left. A tail with
// nothing in it is no lines at all.
func tailLines(tail string) []string {
	if tail == "" {
		return nil
	}
	lines := strings.Split(tail, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > tailLimit {
		lines = lines[len(lines)-tailLimit:]
	}
	return lines
}
