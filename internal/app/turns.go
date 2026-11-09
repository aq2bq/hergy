package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// hergy turns <team>: the watcher that keeps the team's turn and acts when a
// turn ends unhanded (r-103d; d-90e0, d-5b60). Once a second it takes the
// tells hergy tell recorded and the signals the harnesses wrote (hergy
// turn-done, r-378e), reads herdr, hands the step in internal/turns all of it,
// and does what the step says: asks a member who its result is for, delivers
// it as the member answers, keeps the box, writes the turn's state, and writes
// one line per event, with how the end was seen, to the team's log
// (turns.LogPath, read with hergy turns --log). Its own pane shows the band
// (turns_band.go): three lines redrawn in place. The judgement is the turns
// package's; this file is the reading and the doing.

const (
	// turnsTick is how often herdr is read.
	turnsTick = time.Second
	// paneLines is how much of a pane's scrollback is read for what a member
	// wrote: enough for a long report, little enough to read in a second.
	paneLines = 400
	// handoffLines is the most of a member's text that is handed on; the whole
	// pane is named in the message for the rest.
	handoffLines = 80
	// deliverWaitMS is how long a delivery waits to see the recipient working
	// before it counts as not landed.
	deliverWaitMS = 5000
)

func runTurns(ctx context.Context, env team.Env, opts *options.Options, stdout io.Writer) error {
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	h := herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR")))
	if err := h.Verify(protocol.ModeTurns); err != nil {
		return err
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	if opts.Log {
		return printTurnsLog(r.team.Name, turns.LogPath(dir, r.team.Name), stdout)
	}
	logFile, err := openTurnsLog(turns.LogPath(dir, r.team.Name))
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	w := newTurnWatcher(r.team.Name, r.team.Definition.Graph, h, turns.Path(dir, r.team.Name), &stampWriter{out: logFile, clock: time.Now})
	w.screen = &screen{out: stdout}
	defer w.screen.end()
	w.read = herdrRead
	w.hirers = hirersOf(r.team.Name)
	w.signals = turns.SignalDir(dir, r.team.Name)
	w.tells = turns.TellDir(dir, r.team.Name)
	// Tells that waited while no watcher ran are about turns this watcher did
	// not see: they are dropped, so a member is not asked about a turn long gone.
	if _, err := turns.TakeTells(w.tells); err != nil {
		return err
	}
	for _, s := range r.starts {
		if s.Signal.Wired {
			w.wired[s.Short] = true
		}
	}
	if err := w.load(); err != nil {
		return err
	}
	fmt.Fprint(w.out, i18n.Tf("turns.watching", w.team, w.state.Holder))
	if names := slices.Sorted(maps.Keys(w.wired)); len(names) > 0 {
		fmt.Fprint(w.out, i18n.Tf("turns.wired", strings.Join(names, ", ")))
	}
	w.draw()
	ticker := time.NewTicker(turnsTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.tick()
		}
	}
}

// openTurnsLog opens the team's log for appending, making the directory.
func openTurnsLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// printTurnsLog prints the team's log as it stands (hergy turns --log).
func printTurnsLog(teamName, path string, stdout io.Writer) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprint(stdout, i18n.Tf("turns.log_missing", teamName, path))
		return nil
	}
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

// hirersOf is, by a worker's short name, the member that hired it, from this
// machine's launch record (hergy hire writes one line per worker).
func hirersOf(teamName string) map[string]string {
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
		if record.Team != teamName || !record.IsHire() || record.Hire == nil {
			continue
		}
		out[shortMember(record.Hire.Worker, teamName)] = record.Hire.From
	}
	return out
}

// turnWatcher is one team's watcher: what it reads herdr through, where the
// turn's state is kept, and what it saw last time.
type turnWatcher struct {
	team   string
	graph  *definition.Graph
	herdr  *herdr.Herdr
	path   string
	out    io.Writer
	read   func() (tabs []team.Tab, agents []protocol.Agent, agentsRead bool, err error)
	hirers map[string]string
	clock  func() time.Time
	state  *turns.State
	prev   []turns.Agent
	// snaps is each agent's pane as it last stood while the agent was stopped,
	// so what it writes in its next turn is the part after it. The pane is read
	// when the agent stops (and once at the start, for agents already stopped):
	// herdr cannot read a working agent's scrollback (agent_not_idle, measured
	// 2026-11-06), so the turn's start is too late to read it.
	snaps map[string]snapshot
	// names is each agent's herdr name by its short name, as of the last read.
	names map[string]string
	// signals is the directory the harnesses' signals wait in, and tells the
	// one hergy tell writes to; empty reads none. wired is who has a harness
	// that signals: the members hergy wired, and anyone a signal has come from.
	signals string
	tells   string
	wired   map[string]bool
	// band is the picture the pane shows, and screen where it is drawn; no
	// screen draws nothing (the tests read the log alone).
	band   *band
	screen *screen
}

func newTurnWatcher(teamName string, graph *definition.Graph, h *herdr.Herdr, path string, out io.Writer) *turnWatcher {
	return &turnWatcher{
		team: teamName, graph: graph, herdr: h, path: path, out: out,
		hirers: map[string]string{}, clock: time.Now,
		snaps: map[string]snapshot{}, names: map[string]string{}, wired: map[string]bool{},
		band: newBand(teamName),
	}
}

// draw redraws the band from the state and what the band noted.
func (w *turnWatcher) draw() {
	if w.screen == nil {
		return
	}
	w.screen.draw(w.band.render(w.state, w.band.width()))
}

func (w *turnWatcher) load() error {
	st, err := turns.Load(w.path, w.team)
	if err != nil {
		return err
	}
	w.state = st
	return nil
}

// tick takes the tells and the signals, reads herdr once and does what the
// step says.
func (w *turnWatcher) tick() {
	var tells []turns.Tell
	if w.tells != "" {
		var err error
		if tells, err = turns.TakeTells(w.tells); err != nil {
			fmt.Fprint(w.out, i18n.Tf("turns.read_error", err.Error()))
		}
	}
	var signals []turns.Signal
	if w.signals != "" {
		var err error
		if signals, err = turns.TakeSignals(w.signals); err != nil {
			fmt.Fprint(w.out, i18n.Tf("turns.read_error", err.Error()))
		}
		for _, sig := range signals {
			w.wired[sig.Member] = true
		}
	}
	tabs, agents, agentsRead, err := w.read()
	if err != nil {
		fmt.Fprint(w.out, i18n.Tf("turns.read_error", err.Error()))
		w.band.failed(err.Error())
		w.draw()
		return
	}
	if !agentsRead {
		return
	}
	now := w.observe(teamAgents(tabs, agents, w.team))
	w.band.seen(now)
	actions := turns.Step(w.state, turns.Input{
		Team: w.team, Graph: w.graph, Before: w.prev, Now: now, Wrote: w.wrote, At: w.clock(),
		Tells: tells, Signals: signals, Wired: func(name string) bool { return w.wired[name] },
	})
	w.prev = now
	for _, action := range actions {
		w.act(action)
		w.band.note(action, w.clock())
	}
	w.remember(now)
	if len(actions) > 0 {
		if err := turns.Save(w.path, w.state); err != nil {
			fmt.Fprint(w.out, i18n.Tf("turns.save_error", err.Error()))
		}
	}
	w.draw()
}

// snapshot is a pane as it stood at one stop of its agent.
type snapshot struct {
	seq  int
	text string
}

// observe is the team's live agents as the step reads them. It reads nothing
// of the panes: the snapshots are taken after the step (remember), so a stop's
// text is read once, after it has been handed on.
func (w *turnWatcher) observe(live []protocol.Agent) []turns.Agent {
	out := make([]turns.Agent, 0, len(live))
	for _, agent := range live {
		short := shortMember(agent.Name, w.team)
		w.names[short] = agent.Name
		out = append(out, turns.Agent{Name: short, Hirer: w.hirers[short], Status: agent.Status, Seq: agent.Seq})
	}
	return out
}

// remember takes a snapshot of each stopped agent's pane at this stop (by
// herdr's seq), once: what the agent writes in its next turn is read past it.
func (w *turnWatcher) remember(now []turns.Agent) {
	for _, a := range now {
		if a.Status == "working" || a.Status == "blocked" {
			continue
		}
		if have, ok := w.snaps[a.Name]; ok && have.seq == a.Seq {
			continue
		}
		if waiting, ok := w.state.Waiting[a.Name]; ok && waiting.Seq == a.Seq {
			continue // this stop's text is still to be read, once its signal comes or not
		}
		if text, err := w.herdr.ReadRecent(w.names[a.Name], paneLines); err == nil {
			w.snaps[a.Name] = snapshot{seq: a.Seq, text: text}
		}
	}
}

// wrote is what one agent wrote in its pane during the turn that just ended:
// the pane now, past where it stood at the agent's last stop.
func (w *turnWatcher) wrote(short string) string {
	text, err := w.herdr.ReadRecent(w.names[short], paneLines)
	if err != nil {
		return ""
	}
	return paneDelta(w.snaps[short].text, text)
}

func (w *turnWatcher) act(action turns.Action) {
	route := routeText(action)
	switch action.Kind {
	case turns.Deliver:
		w.deliver(action, route)
	case turns.Box:
		fmt.Fprint(w.out, i18n.Tf("turns.boxed", action.From, action.To, route))
	case turns.ToHuman:
		w.toHuman(action, route)
	case turns.Ask:
		w.ask(action, route)
	case turns.Told:
		fmt.Fprint(w.out, i18n.Tf("turns.told", w.party(action.From), w.party(action.To)))
	case turns.Rested:
		fmt.Fprint(w.out, i18n.Tf("turns.rested", action.From, route))
	case turns.Passed:
		// The tell's own line said it; nothing more to say.
	case turns.Started:
		fmt.Fprint(w.out, i18n.Tf("turns.started", action.To))
	case turns.Dropped:
		fmt.Fprint(w.out, i18n.Tf("turns.dropped", action.From, action.Source))
	}
}

// party is a name as the band writes it: a member's short name, or the human.
func (w *turnWatcher) party(name string) string {
	if name == turns.Human {
		return i18n.T("send.who_human")
	}
	return name
}

// toHuman prints why the human came to hold the turn.
func (w *turnWatcher) toHuman(action turns.Action, route string) {
	switch action.Why {
	case turns.WhySilent:
		fmt.Fprint(w.out, i18n.Tf("turns.to_human_silent", action.From, waitingText(action.Waiting), route))
	case turns.WhyNobody:
		fmt.Fprint(w.out, i18n.Tf("turns.to_human_nobody", action.From, route))
	case turns.WhyUnclear:
		fmt.Fprint(w.out, i18n.Tf("turns.to_human_unclear", action.From, route))
	default:
		fmt.Fprint(w.out, i18n.Tf("turns.to_human_edge", action.From, route))
	}
}

// waitingText names who waits for a member's answer, for the band.
func waitingText(waiting []string) string {
	if len(waiting) == 0 {
		return ""
	}
	return i18n.Tf("turns.waiting_for", strings.Join(waiting, ", "))
}

// ask puts hergy's question to the member whose turn ended unhanded: who is
// the result for? The answer is read at its next stop. A question that cannot
// be put (the member is blocked, or never starts) leaves the turn with the
// human, saying why.
func (w *turnWatcher) ask(action turns.Action, route string) {
	target := w.names[action.To]
	if target == "" {
		target = w.team + "-" + action.To
	}
	cands := append(append([]string(nil), action.Candidates...), i18n.T("send.who_human"), i18n.T("turns.nobody"))
	text := i18n.Tf("turns.question", waitingText(action.Waiting), strings.Join(cands, i18n.T("turns.or")))
	fmt.Fprint(w.out, i18n.Tf("turns.asked", action.To, waitingText(action.Waiting), route))
	if err := w.herdr.PromptWait(target, text, deliverWaitMS); err != nil {
		w.state.Unanswered(action.To, w.clock())
		fmt.Fprint(w.out, i18n.Tf("turns.to_human_unanswered", action.To, err.Error()))
	}
}

// routeText says how a turn's end was seen, after the line about it: the
// harness's signal, herdr's reading, or herdr's reading after the signal did
// not come (r-378e). A delivery the member's answer decided says so. The box
// emptying and a tell have no route and say nothing.
func routeText(action turns.Action) string {
	switch action.Route {
	case turns.RouteSignal:
		return i18n.Tf("turns.route_signal", action.Source)
	case turns.RouteHerdr:
		return i18n.T("turns.route_herdr")
	case turns.RouteLate:
		return i18n.Tf("turns.route_late", int(turns.SignalGrace/time.Second))
	}
	return ""
}

// deliver prompts the recipient and waits to see it working; a delivery that
// does not land is tried once more, then the human holds the turn (d-b4b5). A
// delivery the member's answer decided says so after the line.
func (w *turnWatcher) deliver(action turns.Action, route string) {
	if route != "" {
		route = i18n.Tf("turns.route_answer", route)
	}
	text := w.handoffText(action.Messages)
	target := w.names[action.To]
	if target == "" {
		target = w.team + "-" + action.To
	}
	var err error
	for range 2 {
		if err = w.herdr.PromptWait(target, text, deliverWaitMS); err == nil {
			fmt.Fprint(w.out, i18n.Tf("turns.delivered", action.From, action.To, strings.Count(text, "\n")+1, route))
			return
		}
	}
	w.state.Undelivered(w.clock())
	fmt.Fprint(w.out, i18n.Tf("turns.to_human_undelivered", action.From, action.To, err.Error()))
}

// handoffText is the message a recipient gets: one member's text, or everything
// the box held, each part naming who wrote it and where the whole pane is.
func (w *turnWatcher) handoffText(msgs []turns.Message) string {
	if len(msgs) == 1 {
		m := msgs[0]
		return i18n.Tf("turns.handoff", m.From, m.From, lastLines(m.Text, handoffLines), w.names[m.From])
	}
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		parts = append(parts, i18n.Tf("turns.handoff_one_of_many", m.From, w.names[m.From], lastLines(m.Text, handoffLines)))
	}
	return i18n.Tf("turns.handoff_many", len(msgs), strings.Join(parts, "\n\n"))
}

// paneDelta is what a pane holds past where it stood before: the lines after
// the last place the end of before is found in after. Scrollback moves, so the
// end of before (its last three non-empty lines) is searched for, not compared
// by position. When it is not found, the whole of after is taken. The
// harness's own chrome at the bottom (the prompt box, the status line) is cut.
func paneDelta(before, after string) string {
	lines := columnOf(strings.Split(strings.TrimRight(after, "\n"), "\n"))
	// The anchor is the agent's own last words, not the chrome under them (the
	// prompt box is drawn again differently once a turn runs) and not the blank
	// lines between them: so both sides are read as the sequence of the agent's
	// non-empty lines, and the match is mapped back to the pane's lines.
	anchor := lastNonEmpty(trimChrome(columnOf(strings.Split(strings.TrimRight(before, "\n"), "\n"))), 3)
	if len(anchor) > 0 {
		words := wordsOf(lines)
		for i := len(words) - len(anchor); i >= 0; i-- {
			if matchesAt(words, i, anchor) {
				lines = lines[words[i+len(anchor)-1].at+1:]
				break
			}
		}
	}
	return strings.Join(trimChrome(lines), "\n")
}

// word is one of the agent's own lines, and where it stands in the pane.
type word struct {
	text string
	at   int
}

// wordsOf is the agent's non-empty, non-chrome lines of a pane, in order, each
// with its index in the pane.
func wordsOf(lines []string) []word {
	var out []word
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || isChrome(line) {
			continue
		}
		out = append(out, word{text: line, at: i})
	}
	return out
}

// panelIndent is how far right an agent's own line never starts: beyond it, a
// line is the side panel's alone.
const panelIndent = 24

// columnOf keeps the agent's own column of each line: a harness that draws a
// side panel (opencode's context and title, to the right of a wide pane) puts
// it on the same lines after a run of blank space, and that panel changes from
// one reading to the next, so it is neither the agent's text nor a stable anchor.
func columnOf(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		// The indentation is the agent's; the run of blank space after its words
		// is the panel's.
		body := strings.TrimLeft(line, " ")
		if len(line)-len(body) > panelIndent {
			// Nothing of the agent's stands this far right: the line is the
			// panel's alone.
			out[i] = ""
			continue
		}
		if at := strings.Index(body, "    "); at > 0 {
			line = line[:len(line)-len(body)+at]
		}
		out[i] = strings.TrimRight(line, " ")
	}
	return out
}

func lastNonEmpty(lines []string, n int) []string {
	var out []string
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append([]string{lines[i]}, out...)
		}
	}
	return out
}

func matchesAt(words []word, at int, anchor []string) bool {
	for j, want := range anchor {
		if strings.TrimRight(words[at+j].text, " ") != strings.TrimRight(want, " ") {
			return false
		}
	}
	return true
}

// chromeMarks start the lines a harness draws for itself under what the agent
// wrote: Claude Code's prompt box and status line, opencode's frame and status
// bar, Codex's prompt. A line that starts with one is not the agent's text.
var chromeMarks = []string{"❯", "┃", "╹", "╭", "╰", "│", "─", "━", "➜", "⏵⏵", "✻", "✔", "›", "Build ·", "Plan ·", "~/", "Context"}

// chromeWords mark a status line wherever they stand in it.
var chromeWords = []string{"ctrl+", "esc to", "? for shortcuts", "shift+tab", "tok/s", "% used"}

func isChrome(line string) bool {
	s := strings.TrimSpace(line)
	for _, mark := range chromeMarks {
		if strings.HasPrefix(s, mark) {
			return true
		}
	}
	for _, word := range chromeWords {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

// trimChrome cuts the blank and chrome lines at both ends, and the chrome lines
// among the last few, so the text ends where the agent's words end.
func trimChrome(lines []string) []string {
	for len(lines) > 0 && (strings.TrimSpace(lines[0]) == "" || isChrome(lines[0])) {
		lines = lines[1:]
	}
	for len(lines) > 0 && (strings.TrimSpace(lines[len(lines)-1]) == "" || isChrome(lines[len(lines)-1])) {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if !isChrome(line) {
			out = append(out, strings.TrimRight(line, " "))
		}
	}
	return out
}

func lastLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
