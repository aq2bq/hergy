// Package turns owns the turn: who holds it, how a stopped member's turn is
// handed on along the definition's edges, what waits in the box for a member
// that is still working, and the guards that keep a handing from looping
// (r-bc26; d-90e0, d-ef43, d-b4b5, d-3baa, d-5b60). A turn ends in one of two
// ways: the harness says so (a Signal, r-378e), or herdr sees the agent go from
// working to stopped. The signal comes first and carries the turn's text; the
// pane is read only for a member without one (d-ef1d, d-5f1a). It reads nothing
// itself: hergy turns (internal/app) reads the signals, herdr and the panes and
// hands the step what it read, then does what the step says. So the judgement
// is tested with made-up input, like internal/activity.
//
// The one rule, in a sentence: a member writes its result in its own pane and
// stops; hergy hands what it wrote to the party the definition's turns name.
// Members do not hand the turn themselves.
package turns

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
)

// Human is the holder when the turn is the person's.
const Human = "human"

// Herdr's agent_status values the step reads.
const (
	working = "working"
	blocked = "blocked"
)

// State is one running team's turn: the holder, who handed it to them, what
// waits in the box, and the stops hergy already handed on. hergy turns keeps it
// in one file (Path) so hergy status and the screens read the same answer.
type State struct {
	Team string `json:"team"`
	// Holder is who has the turn: a member's or worker's short name, or Human.
	Holder string    `json:"holder"`
	Since  time.Time `json:"since"`
	// From is who handed the turn to Holder: a short name, or Human.
	From string `json:"from,omitempty"`
	// Why says why the human holds it when hergy handed it over (a Why constant);
	// empty when a turn reached the human along an edge.
	Why string `json:"why,omitempty"`
	// Pending is the box: what came for a member while it was working, by the
	// member's short name, in the order it came. It goes out as one message when
	// the member stops (d-5b60 の箱).
	Pending map[string][]Message `json:"pending,omitempty"`
	// Handed is, per member, the seq of the stop hergy last handed on: one
	// handing per stop (d-b4b5).
	Handed map[string]int `json:"handed,omitempty"`
	// Waiting is, per wired member, the stop herdr saw whose signal has not
	// come yet: it is handed from the pane once SignalGrace has passed (d-5f1a).
	Waiting map[string]Stop `json:"waiting,omitempty"`
	// Signaled is, per member, when a signal ended a turn before herdr saw the
	// agent stop: the stop herdr then sees within SignalGrace is that turn's,
	// already handed on (d-5f1a).
	Signaled map[string]time.Time `json:"signaled,omitempty"`
}

// Stop is one stop as herdr saw it: its seq and when the watcher saw it.
type Stop struct {
	Seq int       `json:"seq"`
	At  time.Time `json:"at"`
}

// Message is one thing a party wrote for another: who wrote it, when it
// stopped, and the text it wrote in its pane during that turn.
type Message struct {
	From string    `json:"from"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// Agent is one live agent of the team, as hergy turns read it from herdr.
type Agent struct {
	// Name is the short name: a member's, or a hired worker's.
	Name string
	// Hirer is the member that hired a worker, by short name; empty for a member.
	Hirer string
	// Status is herdr's agent_status: working, idle, done, blocked, unknown.
	Status string
	// Seq is herdr's state_change_seq.
	Seq int
}

// Input is one observation: the agents as they were at the last step and as
// they are now, and how to read what an agent wrote during the turn that just
// ended.
type Input struct {
	// Team is the team's name: a hint written with the herdr name ("→ demo-spec")
	// is read as the short name.
	Team   string
	Graph  *definition.Graph
	Before []Agent
	Now    []Agent
	// Wrote is what the named agent wrote in its pane during the turn that just
	// ended; hergy turns reads the pane. Nil reads as nothing written.
	Wrote func(name string) string
	// Signals is what the harnesses said since the last step, oldest first.
	Signals []Signal
	// Wired says whether a member's harness was given a signal to send, so its
	// stop waits for the signal (SignalGrace) instead of reading the pane. Nil
	// wires nobody.
	Wired func(name string) bool
	At    time.Time
}

// Kind is what hergy turns does for one Action.
type Kind string

const (
	// Deliver hands Messages to To, now: `herdr agent prompt To`.
	Deliver Kind = "deliver"
	// Box keeps the message for To, who is working; nothing is sent.
	Box Kind = "box"
	// ToHuman makes the human the holder: the pane is left as it is, the state
	// says why, and the screens show it.
	ToHuman Kind = "to-human"
	// Started notes that From started To's turn outside hergy's handing (the
	// person typed in the pane): nothing is sent.
	Started Kind = "started"
	// Dropped notes that From's signal came for a stop already handed on from
	// the pane, so it is not handed again (d-5f1a).
	Dropped Kind = "dropped"
)

// How a turn was seen to end.
const (
	// RouteSignal: the harness said so.
	RouteSignal = "signal"
	// RouteHerdr: herdr saw the agent stop, and the member has no signal.
	RouteHerdr = "herdr"
	// RouteLate: herdr saw a wired member stop and no signal came within
	// SignalGrace, so the pane was read.
	RouteLate = "late"
)

// Action is one thing the step decided.
type Action struct {
	Kind     Kind
	To, From string
	Messages []Message
	// Why is a Why constant for ToHuman.
	Why string
	// Route is how From's turn was seen to end (a Route constant) on the action
	// that hands it on, and Source the harness that spoke for RouteSignal. The
	// box emptying into a member has neither.
	Route, Source string
}

// Why the human came to hold the turn.
const (
	// WhyEdge: the stopped member's turn goes to the human along an edge.
	WhyEdge = "edge"
	// WhySilent: the member stopped without writing a line, so there is nothing
	// to hand on and handing an empty turn on would loop (d-b4b5).
	WhySilent = "silent"
	// WhyNoEdge: the member hands the turn to nobody (the definition should not
	// hold this: I12a).
	WhyNoEdge = "no-edge"
	// WhyUndelivered: hergy handed the turn on and the recipient never started
	// (herdr saw no working state, after one more try), so the person is told
	// (d-b4b5).
	WhyUndelivered = "undelivered"
)

// Undelivered records that a delivery to the holder did not land: the human
// holds the turn, and the state says why.
func (st *State) Undelivered(at time.Time) {
	st.Holder, st.Since, st.Why = Human, at, WhyUndelivered
}

// Step reads one observation and says what to do. It changes st.
func Step(st *State, in Input) []Action {
	if st.Pending == nil {
		st.Pending = map[string][]Message{}
	}
	if st.Handed == nil {
		st.Handed = map[string]int{}
	}
	if st.Waiting == nil {
		st.Waiting = map[string]Stop{}
	}
	if st.Signaled == nil {
		st.Signaled = map[string]time.Time{}
	}
	before := map[string]Agent{}
	for _, a := range in.Before {
		before[a.Name] = a
	}
	now := map[string]Agent{}
	for _, a := range in.Now {
		now[a.Name] = a
	}
	var out []Action
	out = append(out, st.signaled(in, now)...)
	out = append(out, st.seenStopped(in, before, now)...)
	out = append(out, st.waitedEnough(in, now)...)
	out = append(out, st.startedByHand(in, before)...)
	out = append(out, st.emptyBox(in)...)
	return out
}

// signaled hands the turns the harnesses said ended, from the signal's text,
// whatever herdr says (d-ef1d). A signal for a stop already handed on from the
// pane is dropped (d-5f1a).
func (st *State) signaled(in Input, now map[string]Agent) []Action {
	var out []Action
	for _, sig := range in.Signals {
		a, live := now[sig.Member]
		if !live {
			a = Agent{Name: sig.Member}
		}
		if live && stopped(a) && st.Handed[a.Name] == a.Seq {
			out = append(out, Action{Kind: Dropped, From: a.Name, Source: sig.Source})
			continue
		}
		delete(st.Waiting, a.Name)
		if live && stopped(a) {
			st.Handed[a.Name] = a.Seq
			delete(st.Signaled, a.Name)
		} else {
			st.Signaled[a.Name] = in.At
		}
		out = append(out, st.ended(in, a, now, sig.Text, RouteSignal, sig.Source)...)
	}
	return out
}

// seenStopped hands the turns herdr saw end (working, then stopped) from the
// pane; a wired member's stop is left to wait for its signal, and a stop a
// signal ended within SignalGrace is that turn's, already handed (d-5f1a).
func (st *State) seenStopped(in Input, before, now map[string]Agent) []Action {
	var out []Action
	for _, a := range in.Now {
		was, seen := before[a.Name]
		if !seen || was.Status != working || !stopped(a) {
			continue
		}
		if st.Handed[a.Name] == a.Seq {
			continue // one handing per stop
		}
		if since, ok := st.Signaled[a.Name]; ok {
			delete(st.Signaled, a.Name)
			if in.At.Sub(since) <= SignalGrace {
				st.Handed[a.Name] = a.Seq
				continue
			}
		}
		if in.Wired != nil && in.Wired(a.Name) {
			st.Waiting[a.Name] = Stop{Seq: a.Seq, At: in.At}
			continue
		}
		st.Handed[a.Name] = a.Seq
		out = append(out, st.ended(in, a, now, in.wrote(a.Name), RouteHerdr, "")...)
	}
	return out
}

// waitedEnough reads the pane of a wired member whose stop waited SignalGrace
// for a signal that did not come; a stop the agent moved past is forgotten.
func (st *State) waitedEnough(in Input, now map[string]Agent) []Action {
	var out []Action
	for _, name := range slices.Sorted(maps.Keys(st.Waiting)) {
		w := st.Waiting[name]
		a, live := now[name]
		if !live || a.Seq != w.Seq || !stopped(a) {
			delete(st.Waiting, name)
			continue
		}
		if in.At.Sub(w.At) < SignalGrace {
			continue
		}
		delete(st.Waiting, name)
		st.Handed[name] = a.Seq
		out = append(out, st.ended(in, a, now, in.wrote(name), RouteLate, "")...)
	}
	return out
}

// startedByHand notes the turns that started outside hergy's handing: the
// person typed in the pane.
func (st *State) startedByHand(in Input, before map[string]Agent) []Action {
	var out []Action
	for _, a := range in.Now {
		was, seen := before[a.Name]
		if a.Status != working || (seen && was.Status == working) {
			continue
		}
		if st.Holder == a.Name {
			continue // hergy handed it; the start is the delivery landing
		}
		st.hold(a.Name, Human, in.At)
		out = append(out, Action{Kind: Started, To: a.Name, From: Human})
	}
	return out
}

// emptyBox delivers the box to each member that is stopped.
func (st *State) emptyBox(in Input) []Action {
	var out []Action
	for _, a := range in.Now {
		if !stopped(a) || len(st.Pending[a.Name]) == 0 {
			continue
		}
		msgs := st.Pending[a.Name]
		delete(st.Pending, a.Name)
		st.hold(a.Name, msgs[len(msgs)-1].From, in.At)
		out = append(out, Action{Kind: Deliver, To: a.Name, From: msgs[len(msgs)-1].From, Messages: msgs})
	}
	return out
}

// stopped is whether an agent's turn is over as herdr sees it: not working and
// not waiting on a person (blocked is not a stop).
func stopped(a Agent) bool {
	return a.Status != working && a.Status != blocked
}

func (in Input) wrote(name string) string {
	if in.Wrote == nil {
		return ""
	}
	return in.Wrote(name)
}

// ended is what one stopped agent's turn becomes, given the text of the turn
// and how the end was seen.
func (st *State) ended(in Input, a Agent, now map[string]Agent, text, route, source string) []Action {
	out := st.turnOf(in, a, now, text)
	for i := range out {
		out[i].Route, out[i].Source = route, source
	}
	return out
}

func (st *State) turnOf(in Input, a Agent, now map[string]Agent, text string) []Action {
	if strings.TrimSpace(text) == "" {
		st.toHuman(a.Name, WhySilent, in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Why: WhySilent}}
	}
	from := ""
	if st.Holder == a.Name {
		from = st.From
	}
	hint := Hint(text)
	if in.Team != "" {
		hint = strings.TrimPrefix(hint, in.Team+"-")
	}
	to, human, why := Destination(in.Graph, a, from, hint)
	if human {
		st.toHuman(a.Name, why, in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Why: why, Messages: []Message{{From: a.Name, At: in.At, Text: text}}}}
	}
	msg := Message{From: a.Name, At: in.At, Text: text}
	if recipient, live := now[to]; live && (recipient.Status == working || recipient.Status == blocked) {
		st.Pending[to] = append(st.Pending[to], msg)
		return []Action{{Kind: Box, To: to, From: a.Name, Messages: []Message{msg}}}
	}
	msgs := append(append([]Message(nil), st.Pending[to]...), msg)
	delete(st.Pending, to)
	st.hold(to, a.Name, in.At)
	return []Action{{Kind: Deliver, To: to, From: a.Name, Messages: msgs}}
}

func (st *State) hold(holder, from string, at time.Time) {
	st.Holder, st.From, st.Since, st.Why = holder, from, at, ""
}

func (st *State) toHuman(from, why string, at time.Time) {
	st.Holder, st.From, st.Since = Human, from, at
	if why != WhyEdge {
		st.Why = why
	} else {
		st.Why = ""
	}
}

// Destination is who a stopped agent's turn goes to (d-ef43, d-3baa): a worker
// returns to the member that hired it; a member whose turns name one party
// hands it there; one with several hands it back to the party that handed it
// the turn, or else to the first its turns name; a hint written on the last
// line (Hint) wins when it names one of those parties. human is whether the
// turn goes to the person, and why is a Why constant then.
func Destination(g *definition.Graph, a Agent, from, hint string) (to string, human bool, why string) {
	if a.Hirer != "" {
		return a.Hirer, false, ""
	}
	var cands []string
	if g != nil {
		for _, edge := range g.Out(definition.NodeID{Kind: definition.KindMember, Name: a.Name}) {
			switch edge.To.Kind {
			case definition.KindHuman:
				cands = append(cands, Human)
			case definition.KindMember:
				cands = append(cands, edge.To.Name)
			case definition.KindWorker:
				// A worker is hired, not handed the turn.
			}
		}
	}
	if len(cands) == 0 {
		return Human, true, WhyNoEdge
	}
	pick := func(name string) (string, bool, string) {
		if name == Human {
			return Human, true, WhyEdge
		}
		return name, false, ""
	}
	if hint != "" && slices.Contains(cands, hint) {
		return pick(hint)
	}
	if len(cands) == 1 {
		return pick(cands[0])
	}
	if from != "" && slices.Contains(cands, from) {
		return pick(from)
	}
	return pick(cands[0])
}

// hintLine is the one way a member names the party it writes for: a last line
// of "→ name" (or "-> name"); "→ 人間" and "→ human" name the person.
var hintLine = regexp.MustCompile(`^(?:→|->)\s*(\S+)\s*$`)

// Hint is the party the last non-empty line of a turn's text names, or "".
func Hint(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n \t"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	m := hintLine.FindStringSubmatch(last)
	if m == nil {
		return ""
	}
	if m[1] == "人間" || m[1] == Human {
		return Human
	}
	return m[1]
}
