// Package turns owns the turn: who holds it, who opened whose turn, what waits
// in the box for a member that is still working, and what hergy does when a
// turn ends without being handed on (r-103d; d-8b4e, d-80a5, d-b024, with the
// box and the guards of r-bc26: d-90e0, d-b4b5, d-3baa, d-5b60). A turn ends in
// one of two ways: the harness says so (a Signal, r-378e), or herdr sees the
// agent go from working to stopped. The signal comes first and carries the
// turn's text; the pane is read only for a member without one (d-ef1d,
// d-5f1a). The package reads nothing itself: hergy turns (internal/ops) reads
// the tells, the signals, herdr and the panes and hands the step what it read,
// then does what the step says. So the judgement is tested with made-up input.
//
// The rule, in a few sentences. Members hand the turn on themselves, with
// hergy tell; hergy records who opened whose turn and hands nothing on of its
// own (d-8b4e). A turn the outside opened (the person, the start prompt, a
// question of hergy's) ends and nothing happens: the reply is the outside's
// (d-b024). A turn another member opened that ends without a tell is the one
// case hergy acts on: it asks the member who the result is for, delivers it as
// the member answers, and tells the outside when the member names nobody or
// cannot be understood (d-80a5). hergy never reads the text to decide.
package turns

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/definition"
)

// Human is the outside of the loop: the person today, a parent team one day
// (d-b024). It is who holds the turn when no member does, who opens a turn that
// no member opened, and where a turn goes when hergy cannot tell where else.
const Human = "human"

// Nobody is a member's answer that its result is for no one: the loop rests,
// and the outside is told.
const Nobody = "nobody"

// Herdr's agent_status values the step reads.
const (
	working = "working"
	blocked = "blocked"
)

// State is one running team's turn: the holder, who handed it to them, who has
// opened whose turn and not been answered, what waits in the box, what hergy
// has asked, and the stops hergy already acted on. hergy turns keeps it in one
// file (Path) so hergy status and the screens read the same answer.
type State struct {
	Team domainname.Seen `json:"team"`
	// Holder is who has the turn: a member's or worker's short name, or Human.
	Holder string    `json:"holder"`
	Since  time.Time `json:"since"`
	// From is who handed the turn to Holder: a short name, or Human.
	From string `json:"from,omitempty"`
	// Why says why the human holds it when hergy handed it over (a Why constant);
	// empty when a member answered the outside or named the human.
	Why string `json:"why,omitempty"`
	// Opened is, per member, who opened a turn of theirs that has not ended:
	// a member that told them (hergy tell), hergy delivering an answer, or Human
	// (the person's hergy tell). Oldest first. A member whose turn ends with
	// openers among the members and no tell of its own is asked (d-80a5).
	Opened map[string][]Opening `json:"opened,omitempty"`
	// Started is, per member, when herdr last saw it begin a turn: openers that
	// came before it opened this turn, later ones the next.
	Started map[string]time.Time `json:"started,omitempty"`
	// Told is, per member, its last hergy tell: when, and to whom. A turn with
	// a tell in it was handed on by the member itself; one told to Human is the
	// human's when it ends, and the member is not asked (d-e601).
	Told map[string]Handoff `json:"told,omitempty"`
	// Asked is, per member, the question hergy sent it: the stop it is about,
	// the text waiting to be handed on, and who the answer may name. The
	// member's next stop is the answer.
	Asked map[string]Question `json:"asked,omitempty"`
	// Rested is, per member, a turn that ended as the outside's while a tell
	// that came during it still waits: if no next turn begins within
	// OpenGrace, the tell was answered in that turn (a harness that folds a
	// queued message into the running turn), and the member is asked.
	Rested map[string]Rest `json:"rested,omitempty"`
	// Pending is the box: what hergy delivers to a member once it stops, by the
	// member's short name, in the order it came (d-5b60 の箱).
	Pending map[string][]Message `json:"pending,omitempty"`
	// Handed is, per member, the seq of the stop hergy last acted on: one act
	// per stop (d-b4b5).
	Handed map[string]int `json:"handed,omitempty"`
	// Waiting is, per wired member, the stop herdr saw whose signal has not
	// come yet: it is read from the pane once SignalGrace has passed (d-5f1a).
	Waiting map[string]Stop `json:"waiting,omitempty"`
	// Signaled is, per member, when a signal ended a turn before herdr saw the
	// agent stop: the stop herdr then sees within SignalGrace is that turn's,
	// already acted on (d-5f1a).
	Signaled map[string]time.Time `json:"signaled,omitempty"`
}

// Opening is one party opening a member's turn: who, and when.
type Opening struct {
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// Question is what hergy sent a member whose turn ended unhanded.
type Question struct {
	Seq int       `json:"seq"`
	At  time.Time `json:"at"`
	// Text is what the member wrote in that turn: handed on as it answers.
	Text string `json:"text"`
	// Candidates are the members the answer may name: those waiting for it
	// first, then those the definition's turns name.
	Candidates []string `json:"candidates"`
	// Waiting are the members that opened the turn and were not told back.
	Waiting []string `json:"waiting,omitempty"`
}

// Rest is one turn that ended as the outside's while later openings waited.
type Rest struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// OpenGrace is how long a member that rested stays stopped before a tell that
// came during that turn is taken as answered in it: a harness that starts a
// new turn for a queued message does so within a second or two; one that
// folded it into the running turn starts nothing.
const OpenGrace = 10 * time.Second

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

// Handoff is one member's hergy tell as the state keeps it: when, and to whom
// (a short name, or Human).
type Handoff struct {
	At time.Time `json:"at"`
	To string    `json:"to"`
}

// Tell is one hergy tell as hergy turns reads it: who handed what turn to
// whom. From is a member's short name, or "" for the person. To is a short
// name, or Human when the member marked its text as the person's.
type Tell struct {
	From string    `json:"from"`
	To   string    `json:"to"`
	At   time.Time `json:"at"`
}

// Agent is one live agent of the team, as hergy turns read it from herdr.
type Agent struct {
	// Name is the short name: a member's, or a hired worker's.
	Name string
	// Hirer is the member that hired a worker, by short name; empty for a member.
	// A worker's turn no one opened is the hirer's opening (d-3baa).
	Hirer string
	// Status is herdr's agent_status: working, idle, done, blocked, unknown.
	Status string
	// Seq is herdr's state_change_seq.
	Seq int
}

// Input is one observation: the agents as they were at the last step and as
// they are now, what was told and signaled since, and how to read what an
// agent wrote during the turn that just ended.
type Input struct {
	// Team is the team's name: an answer that names a member by its herdr name
	// ("demo-spec") is read as the short name.
	Team   domainname.Naming
	Graph  *definition.Graph
	Before []Agent
	Now    []Agent
	// Wrote is what the named agent wrote in its pane during the turn that just
	// ended; hergy turns reads the pane. Nil reads as nothing written.
	Wrote func(name string) string
	// Tells is what hergy tell recorded since the last step, oldest first.
	Tells []Tell
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
	// Started notes that the person started To's turn (typed in the pane):
	// nothing is sent.
	Started Kind = "started"
	// Told notes that From handed the turn to To with hergy tell: nothing is
	// sent, the tell did that.
	Told Kind = "told"
	// Rested notes that From's turn, which the outside opened, ended: its reply
	// is the outside's and nothing is sent (d-b024).
	Rested Kind = "rested"
	// Ask sends To the question: who is the result for? Candidates and Waiting
	// say who it may name and who waits (d-80a5).
	Ask Kind = "ask"
	// Passed notes that From handed its turn on itself (a tell in the turn), so
	// hergy has nothing to do; the state moved on.
	Passed Kind = "passed"
	// Dropped notes that From's signal came for a stop already acted on from
	// the pane, so it is not acted on again (d-5f1a).
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
	// Candidates and Waiting are the Ask's: who the answer may name, and who
	// waits for it.
	Candidates, Waiting []string
	// Route is how From's turn was seen to end (a Route constant), and Source
	// the harness that spoke for RouteSignal. The box emptying into a member and
	// a tell have neither.
	Route, Source string
}

// Why the human came to hold the turn.
const (
	// WhySilent: the member stopped without writing a line while members waited
	// for it, so there is nothing to hand on and nothing to ask about.
	WhySilent = "silent"
	// WhyNobody: the member answered that its result is for nobody.
	WhyNobody = "nobody"
	// WhyUnclear: the member's answer named no one candidate.
	WhyUnclear = "unclear"
	// WhyUnanswered: hergy's question could not be put to the member (blocked,
	// or it never started).
	WhyUnanswered = "unanswered"
	// WhyUndelivered: hergy handed the turn on as the member answered and the
	// recipient never started (herdr saw no working state, after one more try),
	// so the person is told (d-b4b5).
	WhyUndelivered = "undelivered"
)

// Undelivered records that a delivery to the holder did not land: the human
// holds the turn, and the state says why.
func (st *State) Undelivered(at time.Time) {
	st.Holder, st.Since, st.Why = Human, at, WhyUndelivered
}

// Unanswered records that hergy's question could not be put to name: the ask
// is forgotten, the human holds the turn, and the state says why.
func (st *State) Unanswered(name string, at time.Time) {
	delete(st.Asked, name)
	st.Holder, st.From, st.Since, st.Why = Human, name, at, WhyUnanswered
}

// Waits is who waits for whose answer, as "a → b" lines: every member opening
// of a turn that has not ended. It is what status and the screens show.
func (st *State) Waits() []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(st.Opened)) {
		for _, o := range st.Opened[name] {
			if o.By != Human {
				out = append(out, o.By+" → "+name)
			}
		}
	}
	return out
}

func (st *State) init() {
	if st.Opened == nil {
		st.Opened = map[string][]Opening{}
	}
	if st.Started == nil {
		st.Started = map[string]time.Time{}
	}
	if st.Told == nil {
		st.Told = map[string]Handoff{}
	}
	if st.Asked == nil {
		st.Asked = map[string]Question{}
	}
	if st.Rested == nil {
		st.Rested = map[string]Rest{}
	}
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
}

// Step reads one observation and says what to do. It changes st.
func Step(st *State, in Input) []Action {
	st.init()
	before := map[string]Agent{}
	for _, a := range in.Before {
		before[a.Name] = a
	}
	now := map[string]Agent{}
	for _, a := range in.Now {
		now[a.Name] = a
	}
	var out []Action
	out = append(out, st.told(in)...)
	out = append(out, st.signaled(in, now)...)
	out = append(out, st.seenStopped(in, before, now)...)
	out = append(out, st.waitedEnough(in, now)...)
	out = append(out, st.started(in, before)...)
	out = append(out, st.lateOpeners(in, now)...)
	out = append(out, st.emptyBox(in)...)
	return out
}

// lateOpeners asks a member whose turn rested while a tell waited for its next
// turn, once OpenGrace has passed with no next turn: the tell was answered in
// the turn that rested. A member that began a new turn is left to it.
func (st *State) lateOpeners(in Input, now map[string]Agent) []Action {
	var out []Action
	for _, name := range slices.Sorted(maps.Keys(st.Rested)) {
		rest := st.Rested[name]
		a, live := now[name]
		if !live || a.Seq != rest.Seq || !stopped(a) || len(st.Opened[name]) == 0 {
			delete(st.Rested, name)
			continue
		}
		if in.At.Sub(rest.At) < OpenGrace {
			continue
		}
		delete(st.Rested, name)
		st.Started[name] = in.At
		out = append(out, st.turnOf(in, a, rest.Text)...)
	}
	return out
}

// told records each hergy tell: the sender handed its turn on (Told), and
// opened the recipient's next turn (Opened). The tell itself sent the text. A
// tell to Human marks the sender's text as the person's: it opens no turn, and
// the sender's turn is the human's when it ends (d-e601).
func (st *State) told(in Input) []Action {
	var out []Action
	for _, tell := range in.Tells {
		from := tell.From
		if from == "" {
			from = Human
		} else {
			st.Told[from] = Handoff{At: tell.At, To: tell.To}
		}
		if tell.To != Human {
			st.Opened[tell.To] = append(st.Opened[tell.To], Opening{By: from, At: tell.At})
			st.hold(tell.To, from, tell.At)
		}
		out = append(out, Action{Kind: Told, From: from, To: tell.To})
	}
	return out
}

// signaled acts on the turns the harnesses said ended, from the signal's text,
// whatever herdr says (d-ef1d). A signal for a stop already acted on from the
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
		if live && !stopped(a) {
			// herdr still says working: a queued message may have started the
			// next turn without a stop herdr could see, so that turn begins
			// here, and a tell that came during the turn just ended opens it.
			st.Started[a.Name] = in.At
		}
	}
	return out
}

// seenStopped acts on the turns herdr saw end (working, then stopped) from the
// pane; a wired member's stop is left to wait for its signal, and a stop a
// signal ended within SignalGrace is that turn's, already acted on (d-5f1a).
func (st *State) seenStopped(in Input, before, now map[string]Agent) []Action {
	var out []Action
	for _, a := range in.Now {
		was, seen := before[a.Name]
		if !seen || was.Status != working || !stopped(a) {
			continue
		}
		if st.Handed[a.Name] == a.Seq {
			continue // one act per stop
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

// started notes each turn herdr saw begin. One a tell, a delivery or a
// question of hergy's opened is their landing; any other is the person's,
// typed in the pane.
func (st *State) started(in Input, before map[string]Agent) []Action {
	var out []Action
	for _, a := range in.Now {
		was, seen := before[a.Name]
		if a.Status != working || (seen && was.Status == working) {
			continue
		}
		st.Started[a.Name] = in.At
		if _, asked := st.Asked[a.Name]; asked || len(st.Opened[a.Name]) > 0 {
			continue
		}
		st.hold(a.Name, Human, in.At)
		out = append(out, Action{Kind: Started, To: a.Name, From: Human})
	}
	return out
}

// emptyBox delivers the box to each member that is stopped and is not being
// asked: the answer comes first, so the question's reply is read on its own.
func (st *State) emptyBox(in Input) []Action {
	var out []Action
	for _, a := range in.Now {
		if !stopped(a) || len(st.Pending[a.Name]) == 0 {
			continue
		}
		if _, asked := st.Asked[a.Name]; asked {
			continue
		}
		msgs := st.Pending[a.Name]
		delete(st.Pending, a.Name)
		from := msgs[len(msgs)-1].From
		st.open(a.Name, from, in.At)
		out = append(out, Action{Kind: Deliver, To: a.Name, From: from, Messages: msgs})
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
	var out []Action
	if ask, asked := st.Asked[a.Name]; asked {
		out = st.answered(in, a, now, ask, text)
	} else {
		out = st.turnOf(in, a, text)
	}
	for i := range out {
		out[i].Route, out[i].Source = route, source
	}
	return out
}

// turnOf decides one ended turn (d-80a5, d-b024): the outside's reply rests;
// one the member handed on itself (a tell in the turn) needs nothing, and one
// it marked as the person's (hergy tell 人間) is the human's; one that members
// opened and the member did not hand on is asked about, unless it wrote
// nothing, which goes to the human.
func (st *State) turnOf(in Input, a Agent, text string) []Action {
	openers := st.takeOpeners(a)
	toldTo, told := st.toldIn(a.Name)
	if len(openers) == 0 {
		st.toHuman(a.Name, "", in.At)
		if !told && len(st.Opened[a.Name]) > 0 {
			st.Rested[a.Name] = Rest{Seq: a.Seq, At: in.At, Text: text}
		}
		return []Action{{Kind: Rested, From: a.Name}}
	}
	if told {
		return st.handedOn(a.Name, toldTo, openers, in.At)
	}
	if strings.TrimSpace(text) == "" {
		st.toHuman(a.Name, WhySilent, in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Why: WhySilent, Waiting: openers}}
	}
	cands := Candidates(in.Graph, a, openers)
	st.Asked[a.Name] = Question{Seq: a.Seq, At: in.At, Text: text, Candidates: cands, Waiting: openers}
	st.hold(a.Name, st.From, in.At)
	return []Action{{Kind: Ask, To: a.Name, Candidates: cands, Waiting: openers}}
}

// answered reads the member's answer to hergy's question and does as it says:
// a member it names gets the text that waited; the human, nobody or an answer
// that names no one candidate leaves the turn with the human, saying why.
func (st *State) answered(in Input, a Agent, now map[string]Agent, ask Question, text string) []Action {
	delete(st.Asked, a.Name)
	if toldTo, told := st.toldIn(a.Name); told {
		return st.handedOn(a.Name, toldTo, ask.Waiting, in.At) // it handed on itself, asked
	}
	to := Answer(text, in.Team, ask.Candidates)
	switch to {
	case "":
		st.toHuman(a.Name, WhyUnclear, in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Why: WhyUnclear, Messages: []Message{{From: a.Name, At: in.At, Text: text}}}}
	case Nobody:
		st.toHuman(a.Name, WhyNobody, in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Why: WhyNobody}}
	case Human:
		st.toHuman(a.Name, "", in.At)
		return []Action{{Kind: ToHuman, From: a.Name, Messages: []Message{{From: a.Name, At: ask.At, Text: ask.Text}}}}
	}
	msg := Message{From: a.Name, At: ask.At, Text: ask.Text}
	if recipient, live := now[to]; live && (recipient.Status == working || recipient.Status == blocked) {
		st.Pending[to] = append(st.Pending[to], msg)
		return []Action{{Kind: Box, To: to, From: a.Name, Messages: []Message{msg}}}
	}
	msgs := append(append([]Message(nil), st.Pending[to]...), msg)
	delete(st.Pending, to)
	st.open(to, a.Name, in.At)
	return []Action{{Kind: Deliver, To: to, From: a.Name, Messages: msgs}}
}

// takeOpeners is who opened the turn that just ended, by member: the openings
// that came before herdr saw it begin (all of them when it was not seen), taken
// out of Opened; Human among them says the outside opened it too. A worker's
// turn no one opened is its hirer's (d-3baa).
func (st *State) takeOpeners(a Agent) []string {
	started := st.Started[a.Name]
	var keep []Opening
	var members []string
	outside := false
	for _, o := range st.Opened[a.Name] {
		if !started.IsZero() && o.At.After(started) {
			keep = append(keep, o)
			continue
		}
		if o.By == Human {
			outside = true
			continue
		}
		if !slices.Contains(members, o.By) {
			members = append(members, o.By)
		}
	}
	if len(keep) == 0 {
		delete(st.Opened, a.Name)
	} else {
		st.Opened[a.Name] = keep
	}
	if len(members) == 0 && !outside && a.Hirer != "" {
		return []string{a.Hirer}
	}
	return members
}

// toldIn is whether the member handed on with hergy tell during the turn that
// just ended, and to whom, and forgets the tell.
func (st *State) toldIn(name string) (string, bool) {
	told, ok := st.Told[name]
	if !ok {
		return "", false
	}
	delete(st.Told, name)
	started := st.Started[name]
	if !started.IsZero() && told.At.Before(started) {
		return "", false
	}
	return told.To, true
}

// handedOn ends a turn the member handed on itself. Told to a member, the
// tell's own line said it all. Told to Human, the text is the person's: the
// turn is the human's, and those that waited wait no more (d-e601).
func (st *State) handedOn(name, to string, waiting []string, at time.Time) []Action {
	if to == Human {
		st.toHuman(name, "", at)
		return []Action{{Kind: ToHuman, From: name, Waiting: waiting}}
	}
	return []Action{{Kind: Passed, From: name}}
}

func (st *State) hold(holder, from string, at time.Time) {
	st.Holder, st.From, st.Since, st.Why = holder, from, at, ""
}

// open gives to the turn from from, by hergy's hand: the delivery opens to's
// turn, so its reply without a tell is asked about.
func (st *State) open(to, from string, at time.Time) {
	st.Opened[to] = append(st.Opened[to], Opening{By: from, At: at})
	st.hold(to, from, at)
}

func (st *State) toHuman(from, why string, at time.Time) {
	st.Holder, st.From, st.Since, st.Why = Human, from, at, why
}

// Candidates are the members an asked member may name: those waiting for its
// answer first, then the members the definition's turns name out of it, then
// the member that hired a worker. The human and nobody are always understood.
func Candidates(g *definition.Graph, a Agent, waiting []string) []string {
	out := append([]string(nil), waiting...)
	add := func(name string) {
		if name != "" && name != a.Name && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	if g != nil {
		for _, edge := range g.Out(definition.NodeID{Kind: definition.KindMember, Name: a.Name}) {
			if edge.To.Kind == definition.KindMember {
				add(edge.To.Name)
			}
		}
	}
	add(a.Hirer)
	return out
}

// humanWords and nobodyWords are how an answer names the outside and no one.
var (
	humanWords  = []string{"人間", "human", "外側", "outside"}
	nobodyWords = []string{"誰にも", "nobody", "no one", "none"}
)

// IsHuman is whether a name, as hergy tell's target or an answer's one word,
// is the outside: the same words an answer may use.
func IsHuman(name string) bool {
	return slices.Contains(humanWords, strings.ToLower(strings.TrimSpace(name)))
}

// Answer reads a member's answer to hergy's question: the one candidate it
// names (by short name or herdr name), Human, Nobody, or "" when it names none
// or more than one. When the whole answer names several (a pane read back with
// the question's own list in it), its last line alone is read: the one word
// the member was asked for. A name is a whole word among ASCII letters, so
// "dev" is not found in "developer".
func Answer(text string, team domainname.Naming, cands []string) string {
	if to := answerIn(text, team, cands); to != "" {
		return to
	}
	lines := strings.Split(strings.TrimRight(text, "\n \t"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == strings.TrimSpace(text) {
		return ""
	}
	return answerIn(last, team, cands)
}

func answerIn(text string, team domainname.Naming, cands []string) string {
	var hits []string
	lower := strings.ToLower(text)
	for _, cand := range cands {
		if namesWord(lower, strings.ToLower(cand)) || (team.String() != "" && namesWord(lower, strings.ToLower(team.Worker(cand)))) {
			hits = append(hits, cand)
		}
	}
	for _, w := range humanWords {
		if namesWord(lower, w) {
			hits = append(hits, Human)
			break
		}
	}
	for _, w := range nobodyWords {
		if namesWord(lower, w) {
			hits = append(hits, Nobody)
			break
		}
	}
	if len(hits) != 1 {
		return ""
	}
	return hits[0]
}

// namesWord is whether text holds word as a whole word: not inside a longer run
// of ASCII letters, digits, hyphens or underscores.
func namesWord(text, word string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_-]|$)`)
	return re.MatchString(text)
}
