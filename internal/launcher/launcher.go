// Package launcher drives a boot, a reprompt, a refresh, a hire or a down
// through Herdr and records how far it got. It is the layer that touches Herdr
// and the filesystem; the values it works on are immutable.
package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/layout"
	"github.com/aq2bq/hergy/internal/progress"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/status"
)

// Launcher runs one operation. The mutex protects the progress record, which an
// interrupt may report while the operation is still running.
type Launcher struct {
	starts  []definition.Start
	team    string
	timeout int
	herdr   *herdr.Herdr
	out     io.Writer
	// ctx stops a wait when the caller that asked for the operation goes away,
	// as the headquarters does when the start is given up. A launcher without
	// one waits as the command line always has.
	ctx       context.Context
	observe   func(Event)
	workspace string
	wsLabel   string
	// noNext drops the "next: hergy reprompt --all" line a refresh prints after
	// adding a member: apply is the caller that hands the lineup itself, so the
	// command line it suggests would be wrong (d-36ef).
	noNext bool

	// kill stops a process group. It is a field so a test can record the call
	// instead of signalling a real process group.
	kill func(pgid int) error

	mu       sync.Mutex
	progress *progress.Progress
	// waiting is the line an interrupt prints while hergy waits on a member's
	// question at startup, and empty when no wait is on.
	waiting string
	// failedMember is the member whose start was last asked for, and started the
	// members whose start and prompt got through: a start that stops partway
	// says both (d-5f02).
	failedMember string
	started      []string
}

// New builds a launcher over the members of one team: the team name, and the
// members as starts.
func New(starts []definition.Start, team string, h *herdr.Herdr, timeoutMS int, out io.Writer) *Launcher {
	return &Launcher{
		starts:   starts,
		team:     team,
		timeout:  timeoutMS,
		herdr:    h,
		out:      out,
		kill:     defaultKill,
		progress: progress.Start(namesOf(starts)),
	}
}

// ErrCancelled is a start stopped by its caller while the launcher was waiting:
// the panes hergy made are left as they are, for down to put away.
var ErrCancelled = errors.New("launcher: cancelled")

// Event is one step of an operation as the headquarters reads it: the stage the
// launcher moved to, a member's state, a pane, or a wait on a member's startup
// question (d-d589).
type Event struct {
	Stage     string
	Member    string
	State     string
	Pane      string
	TabID     string
	Workspace string
	// Waiting is set while the launcher waits on a member's question, and nil
	// when the wait is over.
	Waiting *Waiting
	// CreatedTab is the label of the tab the launcher just made, and
	// CreatedPanes is how many panes a boot made in it: the lines a screen can
	// keep as finished rather than as a step in progress (r-5b96 audit 9).
	CreatedTab   string
	CreatedPanes int
}

// Waiting is where a person answers a member's startup question.
type Waiting struct {
	Member    string
	Herdr     string
	Workspace string
	Tab       string
	Pane      string
}

// WithContext stops the wait when ctx is done. It returns the launcher.
func (l *Launcher) WithContext(ctx context.Context) *Launcher {
	l.ctx = ctx
	return l
}

// Observe hands each step to fn, for a caller that shows the progress while the
// operation runs (the headquarters). A launcher without one writes the progress
// to its out as before.
func (l *Launcher) Observe(fn func(Event)) *Launcher {
	l.observe = fn
	return l
}

// WorkspaceID is the workspace the tab was made in, set once Boot or BootIn
// creates it.
func (l *Launcher) WorkspaceID() string { return l.workspace }

// WorkspaceLabel is that workspace's label, when herdr could be read.
func (l *Launcher) WorkspaceLabel() string { return l.wsLabel }

// defaultKill sends SIGTERM to a process group.
func defaultKill(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// Boot creates the team tab, lays out the panes, starts the agents and submits
// the bootstrap prompts. up is the hergy up the person typed, printed as the way
// on when a member's question stops the boot.
func (l *Launcher) Boot(up string) error {
	members := l.starts
	workspace, err := l.preflight(members)
	if err != nil {
		return err
	}
	return l.BootIn(workspace, up)
}

// BootIn boots the team in one workspace, chosen by the caller: the headquarters
// starts a team in a workspace a person picked, which is not the one hergy runs
// in (d-73fc). The preflight's own workspace is not read here.
func (l *Launcher) BootIn(workspace, up string) error {
	members := l.starts
	if err := l.preflightIn(workspace, members); err != nil {
		return err
	}
	l.workspace = workspace
	if space, err := l.herdr.Workspace(workspace); err == nil {
		l.wsLabel = space.Label
	}
	if _, err := l.createTab(workspace, &members[0], l.teamTabLabel()); err != nil {
		return err
	}
	l.stage(i18n.T("progress.stage.creating_panes"))
	for _, step := range layout.Plan(namesOf(members), 0) {
		if err := l.layOut(step); err != nil {
			return err
		}
	}
	l.notify(Event{CreatedPanes: len(members), Workspace: l.workspace})
	next := NextUp(l.team, up)
	for i := range members {
		if err := l.start(&members[i], next, l.teamTabLabel()); err != nil {
			return err
		}
	}
	fmt.Fprintf(l.out, i18n.T("launcher.boot_complete"), l.team, l.tabID())
	fmt.Fprint(l.out, i18n.T("launcher.boot_submitted"))
	return nil
}

// teamTabLabel is the label of the team's tab: the team's own name, and nothing
// else. A team booted with --as is called by that name there, and the team
// definition it was booted from is not written into the label but read from this
// machine's launch record when a command needs it (d-a72d): the tab is what a
// person reads, and the definition is hergy's business.
func (l *Launcher) teamTabLabel() string { return l.team }

// Reprompt re-submits the bootstrap prompt to idle agents, keeping each one's
// conversation. It starts nothing.
func (l *Launcher) Reprompt(members []definition.Start) error {
	if err := l.herdr.Verify(protocol.ModeReprompt); err != nil {
		return err
	}
	// Resolve every selected name before sending anything; never prompt a working agent.
	for _, member := range members {
		l.stage(i18n.Tf("progress.stage.checking", member.Name))
		agent, err := l.herdr.Agent(member.Name)
		if err != nil {
			return err
		}
		if err := agent.Check(member.Name, member.Kind); err != nil {
			return err
		}
		if agent.PaneID == "" {
			return i18n.Errorf("error.missing_pane_id", member.Name)
		}
		l.place(member.Name, agent.PaneID)
	}
	for i := range members {
		member := &members[i]
		pane := l.paneOf(member.Name)
		l.stage(i18n.Tf("progress.stage.rechecking", member.Name))
		agent, err := l.herdr.Agent(pane)
		if err != nil {
			return err
		}
		if err := agent.Check(member.Name, member.Kind); err != nil {
			return err
		}
		if err := l.submit(member, pane); err != nil {
			return err
		}
	}
	fmt.Fprintf(l.out, i18n.T("launcher.reprompt_submitted"), strings.Join(namesOf(members), ", "))
	return nil
}

// Refresh restarts each selected member, and gives a pane to a member the
// definition has and hergy has none for. A member that is live has its process
// group stopped, Herdr's name waited away, and the member started again in the
// same pane; a member that is not live, but whose pane hergy named after it is
// still there at its shell, is started again in that pane. A member with no pane
// at all is added to the running team: hergy makes a pane for it in the team's tab
// and starts it there, the way a boot makes one member's pane, with the same
// bootstrap prompt and the same wait on a startup question (d-8fcf); the other
// members are left alone. Either way the conversation starts empty, and the
// settings are read again first, so an edited profile, kind, args or prompt takes
// effect. selfPane is the pane hergy itself runs in: a member there cannot be
// restarted, because stopping it would stop hergy before it could start the agent
// again. It checks every selected member before it stops, starts or makes any
// pane, so one member that is not idle or done, is hergy's own, or whose pane is
// busy stops the whole command with nothing changed.
//
// A team that is not running has no tab to add a member to, and refresh starts no
// team: it refuses and names hergy up, as it always has.
func (l *Launcher) Refresh(members []definition.Start, selfPane string) error {
	if err := l.herdr.Verify(protocol.ModeRefresh); err != nil {
		return err
	}
	targets, tab, err := l.checkTargets(members, selfPane)
	if err != nil {
		return err
	}
	var restarted, added []string
	from, depth := tab.from, 0
	for _, target := range targets {
		if !target.makesPane() {
			if err := l.restart(target, NextRefresh()); err != nil {
				return err
			}
			restarted = append(restarted, target.member.Name)
			continue
		}
		if err := l.addMember(target.member, from, layout.Direction(depth), tab.label); err != nil {
			return err
		}
		// The member added before is the pane the next one is split from, so a
		// second new member does not halve the pane the team was using.
		from, depth = l.paneOf(target.member.Name), depth+1
		added = append(added, target.member.Name)
	}
	if len(restarted) > 0 {
		fmt.Fprintf(l.out, i18n.T("launcher.refresh_restarted"), strings.Join(restarted, ", "))
	}
	if len(added) > 0 {
		fmt.Fprint(l.out, i18n.Tf("launcher.refresh_added", strings.Join(added, ", "), tabNamed(tab.label, tab.id)))
		// The members that were already running do not know the new ones: the
		// bootstrap prompt is what tells them, and it keeps their conversation.
		// A caller that hands that prompt itself (apply) asks for no next line.
		if !l.noNext {
			fmt.Fprintln(l.out, i18n.Tf("next", i18n.Tf("launcher.refresh_added_next", l.team)))
		}
	}
	return nil
}

// NewPanes is the part of a refresh that makes panes: the members the definition
// has and hergy has no pane for, and the team's tab, named as the lines name it.
type NewPanes struct {
	Members []string
	Tab     string
}

// WithoutNextHint says the caller goes on with the members a refresh adds, so a
// refresh prints no "next: hergy reprompt --all" line (apply): the next command
// it would name is the one the caller is already doing (d-36ef).
func (l *Launcher) WithoutNextHint() *Launcher {
	l.noNext = true
	return l
}

// RefreshDry checks the same targets a real refresh checks and changes nothing,
// so --dry shows the same refusal, and says which members a refresh would give a
// pane to. Like down --dry, it reads the live agents, tabs and panes.
func (l *Launcher) RefreshDry(members []definition.Start, selfPane string) (NewPanes, error) {
	targets, tab, err := l.checkTargets(members, selfPane)
	if err != nil {
		return NewPanes{}, err
	}
	plan := NewPanes{Tab: tabNamed(tab.label, tab.id)}
	for _, target := range targets {
		if target.makesPane() {
			plan.Members = append(plan.Members, target.member.Name)
		}
	}
	return plan, nil
}

// Status reads the live agents and tabs and reports one row per selected member
// and per worker this team hired. It starts and stops nothing, so it runs
// outside a herdr-managed pane too.
func (l *Launcher) Status(members []definition.Start) ([]status.Row, error) {
	if err := l.herdr.Verify(protocol.ModeStatus); err != nil {
		return nil, err
	}
	agents, err := l.herdr.LiveAgents()
	if err != nil {
		return nil, err
	}
	states := make(map[string]protocol.Agent, len(agents))
	for _, agent := range agents {
		if agent.Name == "" {
			continue
		}
		if _, ok := states[agent.Name]; !ok {
			states[agent.Name] = agent
		}
	}
	rows := make([]status.Row, 0, len(members))
	for _, member := range members {
		agent, live := states[member.Name]
		// A member with no live agent still carries the role it reads and the
		// engine its settings name: the WebUI shows them while nothing runs.
		kind := agent.Kind
		if kind == "" {
			kind = member.Kind
		}
		row := status.Row{
			Name: member.Name, State: agent.Status, Live: live,
			Title: agent.Title, PaneID: agent.PaneID,
			Role: member.Role, Kind: kind,
		}
		// A live member's screen is read so status can tell a long turn from a
		// stall. A screen that cannot be read leaves the hash empty, and the row
		// is reported without the mark rather than failing the whole command.
		if live {
			if screen, err := l.herdr.Read(member.Name); err == nil {
				row.Screen = screenlog.Hash(screen)
			}
		}
		rows = append(rows, row)
	}
	workers, err := l.workerAgents(agents)
	if err != nil {
		return nil, err
	}
	for _, worker := range workers {
		agent := states[worker.Name]
		rows = append(rows, status.Row{
			Name: worker.Name, State: agent.Status, Live: true, Worker: true,
			Title: agent.Title, PaneID: agent.PaneID, Kind: agent.Kind,
		})
	}
	return rows, nil
}

// Focus moves the focus to one member's pane, so a person can answer it there.
// It changes nothing else.
func (l *Launcher) Focus(member *definition.Start) error {
	if err := l.herdr.Verify(protocol.ModeFocus); err != nil {
		return err
	}
	if err := l.herdr.Focus(member.Name); err != nil {
		return err
	}
	fmt.Fprintf(l.out, i18n.T("focus.focused"), member.Name)
	return nil
}

// Nudge sends one member a short push to carry on, only when it is idle or done:
// a member that is working or blocked is left alone, and the whole command
// errors. text is the message to send, already carrying who it is from.
func (l *Launcher) Nudge(member *definition.Start, text string) error {
	if err := l.herdr.Verify(protocol.ModePrompt); err != nil {
		return err
	}
	agent, err := l.herdr.Agent(member.Name)
	if err != nil {
		return err
	}
	if agent.Status != "idle" && agent.Status != "done" {
		return i18n.Errorf("error.nudge_busy", member.Name, agent.Status)
	}
	if err := l.herdr.Prompt(member.Name, text); err != nil {
		return err
	}
	fmt.Fprintf(l.out, i18n.T("send.sent"), member.Name, text)
	return nil
}

// Tell sends one member a message, whoever it is from. Unlike a nudge it does
// not read the member's state: a person's message reaches an agent in any state.
// text already carries who it is from.
func (l *Launcher) Tell(member *definition.Start, text string) error {
	if err := l.herdr.Verify(protocol.ModePrompt); err != nil {
		return err
	}
	if err := l.herdr.Prompt(member.Name, text); err != nil {
		return err
	}
	fmt.Fprintf(l.out, i18n.T("send.sent"), member.Name, text)
	return nil
}

// Stop interrupts one member's current turn with esc, keeping its conversation.
// An agent that is not working is left alone, and the command still succeeds:
// there is nothing to stop. A member running in hergy's own pane (selfPane) is
// refused, because hergy itself runs there. stopped is whether esc was sent, so
// the caller names refresh as the next step only when something actually stopped.
//
// Some agents need esc twice: the first only asks them to interrupt and they
// carry on (opencode answers "esc interrupt"), the second ends the turn. hergy
// keeps no rule per harness (d-bd76): it sends esc, watches the member's state,
// and sends a second esc only while the member is still working. An agent that
// stopped on the first esc is never sent a second, because on some harnesses the
// second esc does something else. A member that is still working after two escs
// is StopNotStopped: the turn did not end.
func (l *Launcher) Stop(member *definition.Start, selfPane string) (stopped bool, err error) {
	if err := l.herdr.Verify(protocol.ModeStop); err != nil {
		return false, err
	}
	agent, err := l.herdr.Agent(member.Name)
	if err != nil {
		return false, err
	}
	if selfPane != "" && agent.PaneID == selfPane {
		return false, i18n.Errorf("error.stop_self", member.Name)
	}
	if agent.Status != "working" {
		fmt.Fprintf(l.out, i18n.T("stop.not_working"), member.Name, agent.Status)
		return false, nil
	}
	for range stopTries {
		if err := l.herdr.SendKeys(member.Name, "esc"); err != nil {
			return false, err
		}
		stopped, err := l.stoppedByNow(member.Name)
		if err != nil {
			return false, err
		}
		if stopped {
			fmt.Fprintf(l.out, i18n.T("stop.stopped"), member.Name)
			return true, nil
		}
	}
	return false, StopNotStopped{Member: member.Name}
}

// stopTries is how many escs Stop sends at most before it says the turn did not
// end: the first, and one more for a harness that asks twice.
const stopTries = 2

const (
	// stopWatchStep is how long Stop waits between two readings of a member's
	// state, and stopWatchFor how long it watches after one esc before it sends
	// the next. An agent that ends its turn at once is not made to wait for a
	// second esc, and one that needs it is not made to wait long either.
	stopWatchStep = 300 * time.Millisecond
	stopWatchFor  = 3 * time.Second
)

// StopNotStopped is what Stop returns when the member is still working after
// esc was sent twice: the turn did not end, and starting the member again
// (refresh) is the next step. It is told apart from the other failures so the
// caller can name that step.
type StopNotStopped struct{ Member string }

func (e StopNotStopped) Error() string {
	return i18n.Tf("error.stop_not_stopped", e.Member)
}

// stoppedByNow watches the member's state after one esc, up to stopWatchFor, and
// answers whether it is no longer working. An agent Herdr no longer knows is
// stopped: its turn ended with the agent.
func (l *Launcher) stoppedByNow(name string) (bool, error) {
	deadline := time.Now().Add(stopWatchFor)
	for {
		agent, err := l.herdr.Agent(name)
		switch {
		case err == nil && agent.Status != "working":
			return true, nil
		case err != nil && protocol.ErrorCode(err) == herdr.AgentNotFound:
			return true, nil
		case err != nil:
			return false, err
		}
		if !time.Now().Before(deadline) {
			return false, nil
		}
		time.Sleep(stopWatchStep)
	}
}

// refreshTarget is one member a refresh acts on: its pane, and whether an agent is
// live there. A member with no live agent is started again in the pane hergy named
// for it, when that pane is at its shell. A member with no pane at all is one
// hergy has not run yet: refresh makes it a pane in the team's tab (d-8fcf), and
// pane is empty.
type refreshTarget struct {
	member *definition.Start
	pane   string
	live   bool
}

// makesPane is whether refresh gives the member a new pane.
func (t refreshTarget) makesPane() bool { return t.pane == "" }

// teamTab is the tab hergy made for the team, read when a refresh gives a member a
// pane: the tab the pane is made in, its label, and the pane the first new pane is
// split from.
type teamTab struct {
	id    string
	label string
	from  string
}

// checkTargets reads every selected member and refuses the whole command unless
// each one can be restarted or given a pane: live and idle or done, not live with
// a named pane still at the shell, or with no pane at all, which the team's tab
// takes. A member in hergy's own pane is refused. When a member has no pane the
// team's tab is read here too, so a team that is not running refuses the command
// before anything is stopped or made. Nothing is stopped, started or made until
// every member passes.
func (l *Launcher) checkTargets(members []definition.Start, selfPane string) ([]refreshTarget, teamTab, error) {
	targets := make([]refreshTarget, 0, len(members))
	for i := range members {
		member := &members[i]
		l.stage(i18n.Tf("progress.stage.checking", member.Name))
		agent, err := l.herdr.Agent(member.Name)
		if err != nil {
			if protocol.ErrorCode(err) != herdr.AgentNotFound {
				return nil, teamTab{}, err
			}
			target, err := l.deadTarget(member, selfPane)
			if err != nil {
				return nil, teamTab{}, err
			}
			targets = append(targets, target)
			continue
		}
		// No kind check here: the settings were just read again, and the live
		// agent still runs the kind those settings had before the edit.
		if err := agent.CheckLive(member.Name); err != nil {
			return nil, teamTab{}, err
		}
		if agent.PaneID == "" {
			return nil, teamTab{}, i18n.Errorf("error.missing_pane_id", member.Name)
		}
		if selfPane != "" && agent.PaneID == selfPane {
			return nil, teamTab{}, i18n.Errorf("error.refresh_self", member.Name)
		}
		l.place(member.Name, agent.PaneID)
		targets = append(targets, refreshTarget{member: member, pane: agent.PaneID, live: true})
	}
	for _, target := range targets {
		if !target.makesPane() {
			continue
		}
		tab, err := l.teamTab(target.member)
		if err != nil {
			return nil, teamTab{}, err
		}
		return targets, tab, nil
	}
	return targets, teamTab{}, nil
}

// deadTarget finds the pane hergy named after a member that has no live agent.
// The member can be started again only when that pane is still there and its
// foreground process is the shell; a pane that is gone is a member hergy has not
// run, which refresh gives a pane to, and one that runs something hergy did not
// start is refused, because refresh must not take it over.
func (l *Launcher) deadTarget(member *definition.Start, selfPane string) (refreshTarget, error) {
	pane, err := l.herdr.FindPane(member.Name)
	if err != nil {
		return refreshTarget{}, err
	}
	if pane == "" {
		return refreshTarget{member: member}, nil
	}
	if selfPane != "" && pane == selfPane {
		return refreshTarget{}, i18n.Errorf("error.refresh_self", member.Name)
	}
	info, err := l.herdr.PaneProcessInfo(pane)
	if err != nil {
		return refreshTarget{}, err
	}
	if info.Pgid != info.ShellPID {
		return refreshTarget{}, i18n.Errorf("error.refresh_pane_busy", member.Name, pane)
	}
	l.place(member.Name, pane)
	return refreshTarget{member: member, pane: pane, live: false}, nil
}

// teamTab finds the tab hergy made for this team, and the pane a first new pane
// is split from: the first pane in the tab that herdr lists. A team whose tab is
// not there, or whose tab has no pane to split, is refused, and hergy up is named:
// refresh starts no team, as before.
func (l *Launcher) teamTab(member *definition.Start) (teamTab, error) {
	tabs, err := l.herdr.Tabs()
	if err != nil {
		return teamTab{}, err
	}
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label, l.team) {
			continue
		}
		panes, err := l.herdr.Panes()
		if err != nil {
			return teamTab{}, err
		}
		for _, pane := range panes {
			if pane.TabID == tab.ID {
				return teamTab{id: tab.ID, label: tab.Label, from: pane.ID}, nil
			}
		}
	}
	return teamTab{}, i18n.Errorf("error.refresh_no_pane", member.Name)
}

// addMember makes one member's pane in the team's tab and starts the member in it,
// the way a boot makes each member of its plan: the pane is split from another
// pane in the tab and named after the member, then the agent is started and the
// bootstrap prompt submitted. A startup question is waited on as a boot waits.
func (l *Launcher) addMember(member *definition.Start, from, direction, tabLabel string) error {
	l.stage(i18n.T("progress.stage.creating_panes"))
	if err := l.splitPane(from, member, direction); err != nil {
		return err
	}
	if err := l.namePane(member.Name); err != nil {
		return err
	}
	return l.start(member, NextRefresh(), tabLabel)
}

// restart stops a live member's process group and waits for Herdr to drop the
// name, then starts the member again in its pane. A member that was already not
// live is only started.
func (l *Launcher) restart(target refreshTarget, next StartupNext) error {
	member, pane := target.member, target.pane
	if target.live {
		// Check once more, right before stopping: time passed since the first
		// check (with --all, the earlier members' restarts), and the member may
		// have started working again. This is the same second look reprompt takes.
		l.stage(i18n.Tf("progress.stage.rechecking", member.Name))
		agent, err := l.herdr.Agent(pane)
		if err != nil {
			return err
		}
		if err := agent.CheckLive(member.Name); err != nil {
			return err
		}
		l.stage(i18n.Tf("progress.stage.stopping", member.Name))
		info, err := l.herdr.PaneProcessInfo(pane)
		if err != nil {
			return err
		}
		// When no agent runs in the pane, the foreground group is the shell
		// itself; there is nothing to stop.
		if info.Pgid != info.ShellPID {
			if err := l.kill(info.Pgid); err != nil {
				return i18n.Errorf("error.kill_failed", member.Name, err)
			}
		}
		l.stage(i18n.Tf("progress.stage.waiting", member.Name))
		if err := l.waitGone(member.Name); err != nil {
			return err
		}
	}
	// A refresh makes no tab of its own: the member's tab is the one herdr reports.
	return l.start(member, next, "")
}

// waitGone waits until Herdr no longer knows the agent name. Herdr refuses a
// start with agent_name_taken until then. It gives up after the timeout.
func (l *Launcher) waitGone(name string) error {
	deadline := time.Now().Add(time.Duration(l.timeout) * time.Millisecond)
	for {
		_, err := l.herdr.Agent(name)
		if err != nil {
			if protocol.ErrorCode(err) == herdr.AgentNotFound {
				return nil
			}
			return err
		}
		if !time.Now().Before(deadline) {
			return i18n.Errorf("error.agent_not_gone", name, l.timeout, name)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Hire starts one worker in a tab of its own, so the team layout stays as booted.
func (l *Launcher) Hire(worker *definition.Start) error {
	l.progress = progress.Start([]string{worker.Name})
	workspace, err := l.preflight([]definition.Start{*worker})
	if err != nil {
		return err
	}
	label := workerTabLabel(l.team, worker.Short)
	pane, err := l.createTab(workspace, worker, label)
	if err != nil {
		return err
	}
	l.mark(worker.Name, i18n.T("progress.mark.pane_created"))
	if err := l.herdr.Rename(pane, worker.Name); err != nil {
		return err
	}
	if err := l.start(worker, NextHire(), label); err != nil {
		return err
	}
	fmt.Fprintf(l.out, i18n.T("launcher.hired"), worker.Name, l.tabID())
	return nil
}

// downTarget is one pane hergy down acts on: the name to report, its pane,
// whether the pane belongs to a hired worker, whether no agent runs in it, and
// whether it is the pane hergy itself runs in.
type downTarget struct {
	name   string
	pane   string
	worker bool
	// stopped is whether no agent runs in the pane: the pane hergy named after a
	// member or a worker, whose agent never started or has ended. A pane that has
	// no name at all is not a target.
	stopped bool
	self    bool
}

// Down closes every pane hergy made for the team: the panes its live members and
// workers stand in, and the panes it named after them that no agent stands in. A
// member is found by its live agent's name first, then by the name on its pane, so
// a member that never started (a boot stopped at a question) or whose agent ended
// is closed with the rest (d-da92). A member with neither is reported and skipped.
// The pane hergy itself runs in is closed last, so the rest still get closed. With
// dry, it prints what would close and closes nothing. It returns how many panes it
// closed (or would close), so a caller can tell a no-op from real work.
func (l *Launcher) Down(members []definition.Start, selfPane string, dry bool) (int, error) {
	if !dry {
		if err := l.herdr.Verify(protocol.ModeDown); err != nil {
			return 0, err
		}
	}
	agents, err := l.herdr.LiveAgents()
	if err != nil {
		return 0, err
	}
	workers, err := l.workerTargets(agents)
	if err != nil {
		return 0, err
	}
	named, err := l.namedTargets(l.team, agents)
	if err != nil {
		return 0, err
	}
	// The panes hergy named after a member, by name, so a member that is not live
	// is still closed.
	byName := make(map[string]string, len(named))
	for _, target := range named {
		if _, ok := byName[target.name]; !ok {
			byName[target.name] = target.pane
		}
	}
	panes := panesOf(agents)
	// Members first, then the workers and the rest of hergy's named panes;
	// hergy's own pane is held back to last.
	targets := make([]downTarget, 0, len(members)+len(workers)+len(named))
	absent := 0
	for _, member := range members {
		pane, live := panes[member.Name]
		if pane == "" {
			pane = byName[member.Name]
		}
		if pane == "" {
			absent++
			if dry {
				fmt.Fprint(l.out, i18n.Tf("down.would_skip", member.Name))
			} else {
				fmt.Fprint(l.out, i18n.Tf("down.skipped", member.Name))
			}
			continue
		}
		targets = append(targets, downTarget{name: member.Name, pane: pane, stopped: !live})
	}
	targets = append(targets, workers...)
	targets = append(targets, named...)
	return l.closeTargets(targets, selfPane, absent, dry)
}

// DownByName closes what this team started when the team definition behind it
// cannot be read. names are the members' live agent names, which the caller found
// in this team's own tab: a name is not enough to go by, because another team's
// name can start the same way ("qw2-x" beside "qw2"). The panes hergy named after
// the team are found here too, as down finds them, so a member whose agent ended
// is closed with the rest. A team definition is what a prompt is built from, and
// putting a team away needs no prompt, so a team whose boot failed once its tab
// was made (and left no launch record to read the definition from) can still be
// closed.
func (l *Launcher) DownByName(names []string, selfPane string, dry bool) (int, error) {
	if !dry {
		if err := l.herdr.Verify(protocol.ModeDown); err != nil {
			return 0, err
		}
	}
	agents, err := l.herdr.LiveAgents()
	if err != nil {
		return 0, err
	}
	workers, err := l.workerTargets(agents)
	if err != nil {
		return 0, err
	}
	named, err := l.namedTargets(l.team, agents)
	if err != nil {
		return 0, err
	}
	byName := make(map[string]string, len(named))
	for _, target := range named {
		if _, ok := byName[target.name]; !ok {
			byName[target.name] = target.pane
		}
	}
	panes := panesOf(agents)
	targets := make([]downTarget, 0, len(names)+len(workers)+len(named))
	absent := 0
	for _, name := range names {
		pane, live := panes[name]
		if pane == "" {
			pane = byName[name]
		}
		if pane == "" {
			absent++
			if dry {
				fmt.Fprint(l.out, i18n.Tf("down.would_skip", name))
			} else {
				fmt.Fprint(l.out, i18n.Tf("down.skipped", name))
			}
			continue
		}
		targets = append(targets, downTarget{name: name, pane: pane, stopped: !live})
	}
	targets = append(targets, workers...)
	targets = append(targets, named...)
	return l.closeTargets(targets, selfPane, absent, dry)
}

// DownStopped closes what hergy made for a team that no live agent stands in: the
// tab it labelled with the team's name, and the panes in it (and in its worker
// tabs) named after the team. A boot stopped at a startup question, or at a
// question answered with an end, leaves exactly that, and no running team is left
// either: down puts it away by the names hergy gave, reading no definition
// (d-da92).
func (l *Launcher) DownStopped(teamName, selfPane string, dry bool) (int, error) {
	if !dry {
		if err := l.herdr.Verify(protocol.ModeDown); err != nil {
			return 0, err
		}
	}
	named, err := l.namedTargets(teamName, nil)
	if err != nil {
		return 0, err
	}
	return l.closeTargets(named, selfPane, 0, dry)
}

// closeTargets closes what a down found: a pane once, however many targets point
// at it, and hergy's own pane last, because closing it first would take hergy
// with it and leave the rest open. absent is how many members were not there.
func (l *Launcher) closeTargets(targets []downTarget, selfPane string, absent int, dry bool) (int, error) {
	l.stage(i18n.T("progress.stage.closing"))
	closed := 0
	seen := make(map[string]bool)
	var self *downTarget
	for _, target := range targets {
		if seen[target.pane] {
			continue
		}
		seen[target.pane] = true
		if target.pane == selfPane {
			target.self = true
			held := target
			self = &held
			continue
		}
		if err := l.closePane(target, dry); err != nil {
			return 0, err
		}
		closed++
	}
	if self != nil {
		if err := l.closePane(*self, dry); err != nil {
			return 0, err
		}
		closed++
	}
	if closed == 0 {
		fmt.Fprint(l.out, i18n.Tf("down.absent_only", l.team))
		return 0, nil
	}
	if dry {
		fmt.Fprint(l.out, i18n.Tf("down.dry_done", closed, absent))
		return closed, nil
	}
	fmt.Fprint(l.out, i18n.Tf("down.done", closed, absent))
	return closed, nil
}

// workerTabLabel is the label of one worker's tab: "worker:<team>/<worker>".
func workerTabLabel(name, short string) string { return protocol.WorkerTabPrefix(name) + short }

// workerAgents lists the live agents in this team's own worker tabs: a tab whose
// label starts with "worker:<team>/". It leaves another team's workers alone.
func (l *Launcher) workerAgents(agents []protocol.Agent) ([]protocol.Agent, error) {
	tabs, err := l.herdr.Tabs()
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(tabs))
	for _, tab := range tabs {
		labels[tab.ID] = tab.Label
	}
	prefix := protocol.WorkerTabPrefix(l.team)
	var out []protocol.Agent
	for _, agent := range agents {
		if agent.Name == "" || agent.PaneID == "" || agent.TabID == "" {
			continue
		}
		if !strings.HasPrefix(labels[agent.TabID], prefix) {
			continue
		}
		out = append(out, agent)
	}
	return out, nil
}

// workerTargets finds the panes of the workers this team hired that have a live
// agent: a live agent in a tab labelled "worker:<team>/". Tabs are never closed as
// a unit; only the worker's own pane is. A worker whose agent ended is found by
// the name on its pane, as the members are (d-da92).
func (l *Launcher) workerTargets(agents []protocol.Agent) ([]downTarget, error) {
	workers, err := l.workerAgents(agents)
	if err != nil {
		return nil, err
	}
	targets := make([]downTarget, 0, len(workers))
	for _, agent := range workers {
		targets = append(targets, downTarget{name: agent.Name, pane: agent.PaneID, worker: true})
	}
	return targets, nil
}

// namedTargets is every pane hergy made for one team, found by the names on it: a
// pane in the tab labelled with the team's name, or in one of its worker tabs,
// whose own label or whose live agent is named "<team>-<something>". The agent
// need not be running: a member that never started, and one whose agent ended, are
// still hergy's panes to close. A pane with no such name is a person's own and is
// not here (d-da92). agents is what the caller already read; nil reads them.
func (l *Launcher) namedTargets(teamName string, agents []protocol.Agent) ([]downTarget, error) {
	tabs, err := l.herdr.Tabs()
	if err != nil {
		return nil, err
	}
	own := make(map[string]string, len(tabs))
	prefix := protocol.WorkerTabPrefix(teamName)
	for _, tab := range tabs {
		if protocol.NamesTeam(tab.Label, teamName) || strings.HasPrefix(tab.Label, prefix) {
			own[tab.ID] = tab.Label
		}
	}
	if len(own) == 0 {
		return nil, nil
	}
	panes, err := l.herdr.Panes()
	if err != nil {
		return nil, err
	}
	if agents == nil {
		if agents, err = l.herdr.LiveAgents(); err != nil {
			return nil, err
		}
	}
	inPane := make(map[string]string, len(agents))
	for _, agent := range agents {
		if agent.Name == "" || agent.PaneID == "" {
			continue
		}
		if _, ok := inPane[agent.PaneID]; !ok {
			inPane[agent.PaneID] = agent.Name
		}
	}
	targets := make([]downTarget, 0, len(panes))
	for _, pane := range panes {
		label, mine := own[pane.TabID]
		if !mine {
			continue
		}
		name := pane.Label
		if !protocol.NamedForTeam(teamName, name) {
			if name = inPane[pane.ID]; !protocol.NamedForTeam(teamName, name) {
				continue
			}
		}
		targets = append(targets, downTarget{
			name: name, pane: pane.ID,
			worker:  strings.HasPrefix(label, prefix),
			stopped: inPane[pane.ID] == "",
		})
	}
	return targets, nil
}

// closePane reports one pane and, unless dry, closes it. The line comes first:
// when the pane is hergy's own, the close ends the process before anything else
// could be printed.
func (l *Launcher) closePane(target downTarget, dry bool) error {
	name, pane := target.name, target.pane
	switch {
	case dry && target.self && target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_worker_self", name, pane))
		return nil
	case dry && target.self:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_self", name, pane))
		return nil
	case dry && target.stopped:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_stopped", name, pane))
		return nil
	case dry && target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_worker", name, pane))
		return nil
	case dry:
		fmt.Fprint(l.out, i18n.Tf("down.would_close", name, pane))
		return nil
	case target.self && target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.closing_worker_self", name, pane))
	case target.self:
		fmt.Fprint(l.out, i18n.Tf("down.closing_self", name, pane))
	case target.stopped:
		fmt.Fprint(l.out, i18n.Tf("down.closing_stopped", name, pane))
	case target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.closing_worker", name, pane))
	default:
		fmt.Fprint(l.out, i18n.Tf("down.closing", name, pane))
	}
	return l.herdr.ClosePane(pane)
}

// panesOf maps each live agent name to its pane. The first pane wins, so a
// duplicate name never leads to the same pane being closed twice.
func panesOf(agents []protocol.Agent) map[string]string {
	panes := make(map[string]string, len(agents))
	for _, agent := range agents {
		if agent.Name == "" || agent.PaneID == "" {
			continue
		}
		if _, ok := panes[agent.Name]; !ok {
			panes[agent.Name] = agent.PaneID
		}
	}
	return panes
}

// ReportFailure writes where the operation stopped.
func (l *Launcher) ReportFailure(w io.Writer) {
	for _, line := range l.report() {
		fmt.Fprintln(w, line)
	}
}

// StartFailure is a start that stopped partway: the member herdr refused, the
// members that had started before it, what hergy left in herdr, and herdr's own
// reason in words (d-5f02). A failure before any member is asked for has an
// empty Member.
type StartFailure struct {
	Member  string
	Started []string
	// Tab is the team tab's label, and Panes how many panes were made.
	Tab    string
	Panes  int
	Reason string
}

// Failure is the account of a failed operation from what this launcher recorded:
// which member it stopped on, who had started, and what remains for a person to
// put away.
func (l *Launcher) Failure(err error) StartFailure {
	l.mu.Lock()
	defer l.mu.Unlock()
	failure := StartFailure{
		Member:  l.failedMember,
		Started: append([]string(nil), l.started...),
		Tab:     l.team,
		Panes:   len(l.progress.Panes),
		Reason:  protocol.Message(err),
	}
	return failure
}

// ReportInterrupted writes where an interrupted operation stopped, and, while
// hergy was waiting on a member's question at startup, the one line that says
// how to go on. Nothing was answered, so the member still waits and its pane
// stays: the person can answer there and run the command again.
func (l *Launcher) ReportInterrupted(w io.Writer) {
	for _, line := range l.report() {
		fmt.Fprintln(w, line)
	}
	l.mu.Lock()
	line := l.waiting
	l.mu.Unlock()
	if line != "" {
		fmt.Fprint(w, line)
	}
}

// report is the progress record as the lines to print.
func (l *Launcher) report() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.progress.Report()
}

// preflight is everything that can be checked without changing Herdr; it also
// returns the caller's workspace.
func (l *Launcher) preflight(members []definition.Start) (string, error) {
	workspace, err := l.herdr.CurrentWorkspace()
	if err != nil {
		return "", err
	}
	if err := l.preflightIn(workspace, members); err != nil {
		return "", err
	}
	return workspace, nil
}

// preflightIn is the checks every boot makes, in a workspace the caller chose:
// the members' directories, the herdr commands, that no member's name is live,
// and that the workspace is there.
func (l *Launcher) preflightIn(workspace string, members []definition.Start) error {
	for i := range members {
		if !isDir(members[i].Cwd) {
			return i18n.Errorf("error.cwd_not_dir", members[i].Cwd)
		}
	}
	if err := l.herdr.Verify(protocol.ModeBoot); err != nil {
		return err
	}
	if workspace == "" {
		return i18n.Errorf("error.workspace_missing")
	}
	live, err := l.herdr.LiveAgentNames()
	if err != nil {
		return err
	}
	if conflicts := intersect(namesOf(members), live); len(conflicts) > 0 {
		return i18n.Errorf("error.live_names_in_use", strings.Join(conflicts, ", "))
	}
	return nil
}

// createTab makes the new tab whose root pane the first member holds.
func (l *Launcher) createTab(workspace string, first *definition.Start, label string) (string, error) {
	l.stage(i18n.T("progress.stage.creating_tab"))
	root, err := l.herdr.CreateTab(workspace, label, first.Cwd, first.Env, func(id string) {
		l.mu.Lock()
		l.progress.TabID = id
		l.mu.Unlock()
	})
	if err != nil {
		return "", err
	}
	l.place(first.Name, root)
	l.notify(Event{CreatedTab: label, Workspace: l.workspace})
	return root, nil
}

func (l *Launcher) layOut(step layout.Step) error {
	switch s := step.(type) {
	case layout.Split:
		target, err := l.lookup(s.Target)
		if err != nil {
			return err
		}
		return l.splitPane(l.paneOf(s.Source), target, s.Direction)
	case layout.Rename:
		return l.namePane(s.Member)
	}
	return nil
}

// splitPane splits one pane and gives the new pane to one member.
func (l *Launcher) splitPane(source string, member *definition.Start, direction string) error {
	pane, err := l.herdr.Split(source, direction, member.Cwd, member.Env)
	if err != nil {
		return err
	}
	l.place(member.Name, pane)
	return nil
}

// namePane names a member's pane after it, so hergy finds the member by the name
// on its pane when its agent is not live.
func (l *Launcher) namePane(member string) error {
	l.mark(member, i18n.T("progress.mark.pane_created"))
	return l.herdr.Rename(l.paneOf(member), member)
}

func (l *Launcher) start(member *definition.Start, next StartupNext, tabLabel string) error {
	l.stage(i18n.Tf("progress.stage.starting", member.Name))
	l.mu.Lock()
	l.failedMember = member.Name
	l.mu.Unlock()
	l.mark(member.Name, i18n.T("progress.mark.start_requested"))
	if err := l.herdr.Start(member, l.paneOf(member.Name), l.timeout); err != nil {
		// A member that asks a human a question while it starts is waited on,
		// not reported: herdr calls it not ready, and the person answers in the
		// pane (d-d589).
		if !herdr.StartupBlocked(err) {
			return err
		}
		if err := l.awaitStartup(member, next, tabLabel); err != nil {
			return err
		}
	}
	l.mark(member.Name, i18n.T("progress.mark.ready"))
	if err := l.submit(member, member.Name); err != nil {
		return err
	}
	l.mu.Lock()
	l.started = append(l.started, member.Name)
	l.mu.Unlock()
	return nil
}

func (l *Launcher) submit(member *definition.Start, target string) error {
	l.stage(i18n.Tf("progress.stage.submitting", member.Name))
	l.mark(member.Name, i18n.T("progress.mark.prompt_requested"))
	if err := l.herdr.Prompt(target, member.Prompt); err != nil {
		return err
	}
	l.mark(member.Name, i18n.T("progress.mark.bootstrap_submitted"))
	fmt.Fprintf(l.out, i18n.T("launcher.submitted"), member.Name, l.paneOf(member.Name))
	return nil
}

func (l *Launcher) stage(stage string) {
	l.mu.Lock()
	l.progress.At(stage)
	l.mu.Unlock()
	l.notify(Event{Stage: stage, Workspace: l.workspace})
}

func (l *Launcher) mark(name, state string) {
	l.mu.Lock()
	l.progress.Mark(name, state)
	l.mu.Unlock()
	l.notify(Event{Member: name, State: state, Workspace: l.workspace})
}

func (l *Launcher) place(name, pane string) {
	l.mu.Lock()
	l.progress.Place(name, pane)
	l.mu.Unlock()
	l.notify(Event{Member: name, Pane: pane, Workspace: l.workspace})
}

// waitingAt tells the observer where a person answers a startup question, and
// that the wait is over when member is nil (d-d589).
func (l *Launcher) waitingAt(member *definition.Start, tabLabel, tabID string) {
	if l.observe == nil {
		return
	}
	if member == nil {
		l.observe(Event{Workspace: l.workspace})
		return
	}
	l.observe(Event{
		Stage:     i18n.Tf("progress.stage.awaiting", member.Name),
		Member:    member.Name,
		Pane:      l.paneOf(member.Name),
		TabID:     tabID,
		Workspace: l.workspace,
		Waiting: &Waiting{
			Member: member.Name, Herdr: member.Name,
			// The pane is named as a person reads it on the screen, not herdr's
			// pane id: the row the question stands on names the same place the
			// activity screen does (r-5b96 audit 9).
			Workspace: l.wsLabel, Tab: tabLabel, Pane: member.Name,
		},
	})
}

// notify hands one step to the observer, when there is one.
func (l *Launcher) notify(event Event) {
	if l.observe == nil {
		return
	}
	event.TabID = l.tabID()
	l.observe(event)
}

// sleeping waits one step of a watch. A cancelled launcher stops at once with
// ErrCancelled instead of waiting for the member.
func (l *Launcher) sleeping(step time.Duration) error {
	if l.ctx == nil {
		time.Sleep(step)
		return nil
	}
	select {
	case <-l.ctx.Done():
		return ErrCancelled
	case <-time.After(step):
		return nil
	}
}

func (l *Launcher) paneOf(name string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.progress.Panes[name]
}

func (l *Launcher) tabID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.progress.TabID
}

func namesOf(members []definition.Start) []string {
	names := make([]string, len(members))
	for i, member := range members {
		names[i] = member.Name
	}
	return names
}

// lookup finds one member as a start: by its herdr name first, then by its short
// name, so both the internal lookups (which use the herdr name) and the
// command-line member names (the short member name) resolve.
func (l *Launcher) lookup(name string) (*definition.Start, error) {
	for i := range l.starts {
		if l.starts[i].Name == name || l.starts[i].Short == name {
			return &l.starts[i], nil
		}
	}
	return nil, i18n.Errorf("error.no_member", name)
}

func intersect(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, item := range b {
		set[item] = true
	}
	var out []string
	for _, item := range a {
		if set[item] {
			out = append(out, item)
		}
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
