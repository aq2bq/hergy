package app

import (
	"context"
	"fmt"
	"io"
	"os"
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

// hergy turns <team>: the watcher that hands a stopped member's turn on
// (r-bc26, d-90e0, d-5b60). It reads herdr once a second, hands the step in
// internal/turns what it read, and does what the step says: prompts the party a
// turn goes to, keeps the box, writes the turn's state, and prints one line per
// handing in its own pane for a person to read. The judgement is the turns
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
	w := newTurnWatcher(r.team.Name, r.team.Definition.Graph, h, turns.Path(dir, r.team.Name), stdout)
	w.read = herdrRead
	w.hirers = hirersOf(r.team.Name)
	if err := w.load(); err != nil {
		return err
	}
	fmt.Fprint(stdout, i18n.Tf("turns.watching", w.team, w.state.Holder))
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
	// snaps is each agent's pane as it stood when its turn started, so what it
	// wrote during the turn is the part after it.
	snaps map[string]string
	// names is each agent's herdr name by its short name, as of the last read.
	names map[string]string
}

func newTurnWatcher(teamName string, graph *definition.Graph, h *herdr.Herdr, path string, out io.Writer) *turnWatcher {
	return &turnWatcher{
		team: teamName, graph: graph, herdr: h, path: path, out: out,
		hirers: map[string]string{}, clock: time.Now,
		snaps: map[string]string{}, names: map[string]string{},
	}
}

func (w *turnWatcher) load() error {
	st, err := turns.Load(w.path, w.team)
	if err != nil {
		return err
	}
	w.state = st
	return nil
}

// tick reads herdr once and does what the step says.
func (w *turnWatcher) tick() {
	tabs, agents, agentsRead, err := w.read()
	if err != nil {
		fmt.Fprint(w.out, i18n.Tf("turns.read_error", err.Error()))
		return
	}
	if !agentsRead {
		return
	}
	now := w.observe(teamAgents(tabs, agents, w.team))
	actions := turns.Step(w.state, turns.Input{
		Graph: w.graph, Before: w.prev, Now: now, Wrote: w.wrote, At: w.clock(),
	})
	w.prev = now
	for _, action := range actions {
		w.act(action)
	}
	if len(actions) > 0 {
		if err := turns.Save(w.path, w.state); err != nil {
			fmt.Fprint(w.out, i18n.Tf("turns.save_error", err.Error()))
		}
	}
}

// observe is the team's live agents as the step reads them, and takes a
// snapshot of each pane whose turn just started.
func (w *turnWatcher) observe(live []protocol.Agent) []turns.Agent {
	was := map[string]string{}
	for _, a := range w.prev {
		was[a.Name] = a.Status
	}
	out := make([]turns.Agent, 0, len(live))
	for _, agent := range live {
		short := shortMember(agent.Name, w.team)
		w.names[short] = agent.Name
		out = append(out, turns.Agent{Name: short, Hirer: w.hirers[short], Status: agent.Status, Seq: agent.Seq})
		if agent.Status == "working" && was[short] != "working" {
			if text, err := w.herdr.ReadRecent(agent.Name, paneLines); err == nil {
				w.snaps[short] = text
			}
		}
	}
	return out
}

// wrote is what one agent wrote in its pane during the turn that just ended:
// the pane now, past where it stood when the turn started.
func (w *turnWatcher) wrote(short string) string {
	text, err := w.herdr.ReadRecent(w.names[short], paneLines)
	if err != nil {
		return ""
	}
	return paneDelta(w.snaps[short], text)
}

func (w *turnWatcher) act(action turns.Action) {
	switch action.Kind {
	case turns.Deliver:
		w.deliver(action)
	case turns.Box:
		fmt.Fprint(w.out, i18n.Tf("turns.boxed", action.From, action.To))
	case turns.ToHuman:
		switch action.Why {
		case turns.WhySilent:
			fmt.Fprint(w.out, i18n.Tf("turns.to_human_silent", action.From))
		case turns.WhyNoEdge:
			fmt.Fprint(w.out, i18n.Tf("turns.to_human_no_edge", action.From))
		default:
			fmt.Fprint(w.out, i18n.Tf("turns.to_human_edge", action.From))
		}
	case turns.Started:
		fmt.Fprint(w.out, i18n.Tf("turns.started", action.To))
	}
}

// deliver prompts the recipient and waits to see it working; a delivery that
// does not land is tried once more, then the human holds the turn (d-b4b5).
func (w *turnWatcher) deliver(action turns.Action) {
	text := w.handoffText(action.Messages)
	target := w.names[action.To]
	if target == "" {
		target = w.team + "-" + action.To
	}
	var err error
	for range 2 {
		if err = w.herdr.PromptWait(target, text, deliverWaitMS); err == nil {
			fmt.Fprint(w.out, i18n.Tf("turns.delivered", action.From, action.To, strings.Count(text, "\n")+1))
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
	lines := strings.Split(strings.TrimRight(after, "\n"), "\n")
	// The anchor is the agent's own last words, not the chrome under them: the
	// prompt box is drawn again differently once a turn runs.
	anchor := lastNonEmpty(trimChrome(strings.Split(strings.TrimRight(before, "\n"), "\n")), 3)
	if len(anchor) > 0 {
		for i := len(lines) - len(anchor); i >= 0; i-- {
			if matchesAt(lines, i, anchor) {
				lines = lines[i+len(anchor):]
				break
			}
		}
	}
	return strings.Join(trimChrome(lines), "\n")
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

func matchesAt(lines []string, at int, anchor []string) bool {
	for j, want := range anchor {
		if strings.TrimRight(lines[at+j], " ") != strings.TrimRight(want, " ") {
			return false
		}
	}
	return true
}

// chromeMarks start the lines a harness draws for itself under what the agent
// wrote: Claude Code's prompt box and status line, opencode's frame and status
// bar, Codex's prompt. A line that starts with one is not the agent's text.
var chromeMarks = []string{"❯", "┃", "╹", "╭", "╰", "│", "─", "━", "➜", "⏵⏵", "✻", "✔", "›", "Build ·", "Plan ·", "~/"}

// chromeWords mark a status line wherever they stand in it.
var chromeWords = []string{"ctrl+", "esc to", "? for shortcuts", "shift+tab", "tok/s"}

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
