package launcher

import (
	"fmt"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// An agent that asks a human a question while it starts — Claude Code asking
// whether to trust this folder, and then whether to import a parent folder's
// CLAUDE.md — is not a failure: it handed the turn to the person (d-d589). Herdr
// refuses the start with agent_not_ready while the agent is blocked, and the
// agent is live under its name. hergy then names the tab and the pane, waits
// with no limit until the member takes input, and goes on with the bootstrap
// prompt. It answers nothing itself, writes no agent setting, and moves no
// focus.

// StartupNext is how one operation goes on after a member asked a question while
// it starts: the line an interrupt prints, naming the tab and the member, and the
// command the ended message names. A boot, a hire and a refresh each go on
// differently, so each builds its own.
type StartupNext struct {
	// Line is the line printed when the person stops the wait with Ctrl-C. tab is
	// the tab as the lines name it, with its label and its id.
	Line func(member *definition.Start, tab string) string
	// Command is the way on the ended message names. It names no tab: the member
	// may be gone, and its tab with it.
	Command func(member *definition.Start) string
}

// NextUp is how a boot goes on: the team is put away, and the same hergy up is
// run again. up is that command as the person typed it.
func NextUp(team, up string) StartupNext {
	return StartupNext{
		Line: func(member *definition.Start, tab string) string {
			return i18n.Tf("launcher.startup_next_up", tab, member.Name, team, up)
		},
		Command: func(*definition.Start) string {
			return i18n.Tf("launcher.startup_command_up", team, up)
		},
	}
}

// NextHire is how a hire goes on: its tab is closed in herdr, and the same hergy
// hire is run again. The hire names the worker, its purpose and its task, so the
// line leaves the command to the person rather than writing it out again.
func NextHire() StartupNext {
	return StartupNext{
		Line: func(member *definition.Start, tab string) string {
			return i18n.Tf("launcher.startup_next_hire", tab, member.Name)
		},
		Command: func(*definition.Start) string {
			return i18n.T("launcher.startup_command_hire")
		},
	}
}

// NextRefresh is how a refresh goes on: the same hergy refresh is run again.
func NextRefresh() StartupNext {
	return StartupNext{
		Line: func(member *definition.Start, tab string) string {
			return i18n.Tf("launcher.startup_next_refresh", tab, member.Name)
		},
		Command: func(member *definition.Start) string {
			return i18n.Tf("launcher.startup_command_refresh", member.Name)
		},
	}
}

// startupWatchStep is how long the wait between two readings of a blocked
// member's state is. A person answering a question takes a human length of time,
// and one reading every 200 ms finds it answered soon after they do.
const startupWatchStep = 200 * time.Millisecond

// startupSettleFor is how long a member that has stopped asking is read again
// after. An agent that was told not to trust the folder answers by ending: herdr
// shows it idle for a moment while it exits, and forgets it right after, so a
// single idle reading is not yet a member that takes input. One that is ready
// stays idle through the settle.
const startupSettleFor = time.Second

// awaitStartup says once what hergy is waiting on, then waits, with no limit,
// until the member takes input: the person answers in that pane, and hergy goes
// on. A second question in a row (the trust of a folder, then the import of a
// parent's CLAUDE.md) prints nothing more. A member herdr no longer knows ended
// after its question — it was told not to trust the folder, for example — and the
// start failed: the error names the way on.
//
// tabLabel is the label hergy gave the member's tab; a refresh passes none,
// because hergy makes no tab for it, and herdr's own label is read instead.
func (l *Launcher) awaitStartup(member *definition.Start, next StartupNext, tabLabel string) error {
	name := member.Name
	command := next.Command(member)
	agent, err := l.herdr.Agent(name)
	if err != nil {
		if protocol.ErrorCode(err) == herdr.AgentNotFound {
			return l.endedStartup(member, command)
		}
		return err
	}
	id := agent.TabID
	if id == "" {
		id = l.tabID()
	}
	if tabLabel == "" && id != "" {
		// A refresh starts a member in the tab it already runs in, so the label
		// is herdr's.
		if tabLabel, err = l.labelOfTab(id); err != nil {
			return err
		}
	}
	tab := tabNamed(tabLabel, id)
	l.stage(i18n.Tf("progress.stage.awaiting", name))
	l.mark(name, i18n.T("progress.mark.blocked"))
	// The line is kept for an interrupt: it is printed under the progress
	// report, which a Ctrl-C reads while this wait is still running.
	l.waitOn(next.Line(member, tab))
	defer l.waitOver()
	fmt.Fprint(l.out, i18n.Tf("launcher.startup_blocked", name, tab, l.paneOf(name), member.Kind))
	for {
		switch {
		case err != nil && protocol.ErrorCode(err) == herdr.AgentNotFound:
			return l.endedStartup(member, command)
		case err != nil:
			return err
		case agent.Status == "idle" || agent.Status == "done":
			// It stopped asking. Read it again once it has had time to settle: a
			// member that is really ready is still there, and one that ended
			// with the question is gone.
			ready, settleErr := l.settleStartup(name)
			switch {
			case settleErr != nil && protocol.ErrorCode(settleErr) == herdr.AgentNotFound:
				return l.endedStartup(member, command)
			case settleErr != nil:
				return settleErr
			case ready:
				fmt.Fprint(l.out, i18n.Tf("launcher.startup_unblocked", name))
				return nil
			}
			// A second question in a row (the trust of a folder, then the import
			// of a parent's CLAUDE.md): wait for its answer.
			agent, err = l.herdr.Agent(name)
		default:
			time.Sleep(startupWatchStep)
			agent, err = l.herdr.Agent(name)
		}
	}
}

// settleStartup reads a member once more after it stopped asking, and answers
// whether it takes input. A member that asks again answers false, and a member
// herdr no longer knows answers agent_not_found: the caller reads that as an end.
func (l *Launcher) settleStartup(name string) (bool, error) {
	time.Sleep(startupSettleFor)
	agent, err := l.herdr.Agent(name)
	if err != nil {
		return false, err
	}
	return agent.Status == "idle" || agent.Status == "done", nil
}

// labelOfTab is herdr's label for one tab, or "" when herdr no longer lists it.
func (l *Launcher) labelOfTab(id string) (string, error) {
	tabs, err := l.herdr.Tabs()
	if err != nil {
		return "", err
	}
	for _, tab := range tabs {
		if tab.ID == id {
			return tab.Label, nil
		}
	}
	return "", nil
}

// tabNamed is a tab as the lines name it: its label and its id, so a person can
// find it in herdr either way. A tab whose label is not known is named by its id
// alone.
func tabNamed(label, id string) string {
	if label == "" || id == "" {
		return id
	}
	return i18n.Tf("launcher.tab_named", label, id)
}

// endedStartup is the failure of a start whose agent ended with the question. The
// wait is over, so the report says where the start stopped rather than that the
// member still waits, and the error names the way on.
func (l *Launcher) endedStartup(member *definition.Start, command string) error {
	l.stage(i18n.Tf("progress.stage.starting", member.Name))
	l.mark(member.Name, i18n.T("progress.mark.start_requested"))
	return i18n.Errorf("error.startup_ended", member.Name, member.Kind, command)
}

// waitOn records the line an interrupt prints while hergy waits on a member's
// question, or clears it when the wait is over.
func (l *Launcher) waitOn(line string) {
	l.mu.Lock()
	l.waiting = line
	l.mu.Unlock()
}

func (l *Launcher) waitOver() { l.waitOn("") }
