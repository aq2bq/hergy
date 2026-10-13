// Package launcher drives a boot, a reprompt, a refresh, a hire or a down
// through Herdr and records how far it got. It is the layer that touches Herdr
// and the filesystem; the values it works on are immutable.
package launcher

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/layout"
	"github.com/aq2bq/hergy/internal/progress"
	"github.com/aq2bq/hergy/internal/protocol"
)

// Launcher runs one operation. The mutex protects the progress record, which an
// interrupt may report while the operation is still running.
type Launcher struct {
	cfg     *config.Config
	timeout int
	herdr   *herdr.Herdr
	out     io.Writer

	// kill stops a process group. It is a field so a test can record the call
	// instead of signalling a real process group.
	kill func(pgid int) error

	mu       sync.Mutex
	progress *progress.Progress
}

// New builds a launcher.
func New(cfg *config.Config, h *herdr.Herdr, timeoutMS int, out io.Writer) *Launcher {
	return &Launcher{
		cfg:      cfg,
		timeout:  timeoutMS,
		herdr:    h,
		out:      out,
		kill:     defaultKill,
		progress: progress.Start(memberNames(cfg.Members)),
	}
}

// defaultKill sends SIGTERM to a process group.
func defaultKill(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// Boot creates the team tab, lays out the panes, starts the agents and submits
// the bootstrap prompts.
func (l *Launcher) Boot() error {
	members, err := l.cfg.Select("")
	if err != nil {
		return err
	}
	workspace, err := l.preflight(members)
	if err != nil {
		return err
	}
	if _, err := l.createTab(workspace, members[0], "team:"+l.cfg.Team); err != nil {
		return err
	}
	l.stage(i18n.T("progress.stage.creating_panes"))
	for _, step := range layout.Plan(namesOf(members), 0) {
		if err := l.layOut(step); err != nil {
			return err
		}
	}
	for _, member := range members {
		if err := l.start(member); err != nil {
			return err
		}
	}
	fmt.Fprintf(l.out, i18n.T("launcher.boot_complete"), l.cfg.Team, l.tabID())
	fmt.Fprint(l.out, i18n.T("launcher.boot_submitted"))
	return nil
}

// Reprompt re-submits the bootstrap prompt to idle agents, keeping each one's
// conversation. It starts nothing.
func (l *Launcher) Reprompt(members []*config.Member) error {
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
	for _, member := range members {
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

// Refresh restarts each selected member: it stops the member's process group,
// waits for Herdr to forget the agent name, and starts the member again in the
// same pane with the settings as they are now. The conversation starts empty.
// It reads the settings again first, so an edited profile, kind or args takes
// effect. It checks every selected member before it stops any, so one member
// that is not idle or done stops the whole command with nothing stopped.
func (l *Launcher) Refresh(members []*config.Member) error {
	if err := l.herdr.Verify(protocol.ModeRefresh); err != nil {
		return err
	}
	for _, member := range members {
		l.stage(i18n.Tf("progress.stage.checking", member.Name))
		agent, err := l.herdr.Agent(member.Name)
		if err != nil {
			return err
		}
		// No kind check here: the settings were just read again, and the live
		// agent still runs the kind those settings had before the edit.
		if err := agent.CheckLive(member.Name); err != nil {
			return err
		}
		if agent.PaneID == "" {
			return i18n.Errorf("error.missing_pane_id", member.Name)
		}
		l.place(member.Name, agent.PaneID)
	}
	for _, member := range members {
		if err := l.restart(member); err != nil {
			return err
		}
	}
	fmt.Fprintf(l.out, i18n.T("launcher.refresh_restarted"), strings.Join(namesOf(members), ", "))
	return nil
}

// restart stops one member's process group, waits for Herdr to drop the name,
// and starts the member again in the same pane.
func (l *Launcher) restart(member *config.Member) error {
	pane := l.paneOf(member.Name)
	l.stage(i18n.Tf("progress.stage.stopping", member.Name))
	info, err := l.herdr.PaneProcessInfo(pane)
	if err != nil {
		return err
	}
	// When no agent runs in the pane, the foreground group is the shell itself;
	// there is nothing to stop.
	if info.Pgid != info.ShellPID {
		if err := l.kill(info.Pgid); err != nil {
			return i18n.Errorf("error.kill_failed", member.Name, err)
		}
	}
	l.stage(i18n.Tf("progress.stage.waiting", member.Name))
	if err := l.waitGone(member.Name); err != nil {
		return err
	}
	return l.start(member)
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
			return i18n.Errorf("error.agent_not_gone", name, l.timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Hire starts one worker in a tab of its own, so the team layout stays as booted.
func (l *Launcher) Hire(worker *config.Member) error {
	l.progress = progress.Start([]string{worker.Name})
	workspace, err := l.preflight([]*config.Member{worker})
	if err != nil {
		return err
	}
	pane, err := l.createTab(workspace, worker, "worker:"+l.cfg.Team+"/"+worker.Seat)
	if err != nil {
		return err
	}
	l.mark(worker.Name, i18n.T("progress.mark.pane_created"))
	if err := l.herdr.Rename(pane, worker.Name); err != nil {
		return err
	}
	if err := l.start(worker); err != nil {
		return err
	}
	fmt.Fprintf(l.out, i18n.T("launcher.hired"), worker.Name, l.tabID())
	return nil
}

// downTarget is one pane hergy down acts on: the name to report, its pane,
// whether the pane belongs to a hired worker, and whether it is the pane hergy
// itself runs in.
type downTarget struct {
	name   string
	pane   string
	worker bool
	self   bool
}

// Down closes the panes of the team's live members and of the workers it hired.
// It finds a member by name among the live agents; a member that is not live is
// reported and skipped. A worker is found by its tab: the tab's label starts
// with "worker:<team>/", and the live agent in that tab is closed. A team with
// neither a live member nor a live worker closes nothing. The pane hergy itself
// runs in is closed last, so the rest still get closed. With dry, it prints what
// would close and closes nothing. It returns how many panes it closed (or would
// close), so a caller can tell a no-op from real work.
func (l *Launcher) Down(members []*config.Member, selfPane string, dry bool) (int, error) {
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
	panes := panesOf(agents)
	l.stage(i18n.T("progress.stage.closing"))
	closed, absent := 0, 0
	// A pane is closed once, however many targets point at it.
	seen := make(map[string]bool)
	var self *downTarget
	// Members first, then the workers; hergy's own pane is held back to last.
	targets := make([]downTarget, 0, len(members)+len(workers))
	for _, member := range members {
		pane := panes[member.Name]
		if pane == "" {
			absent++
			if dry {
				fmt.Fprint(l.out, i18n.Tf("down.would_skip", member.Name))
			} else {
				fmt.Fprint(l.out, i18n.Tf("down.skipped", member.Name))
			}
			continue
		}
		targets = append(targets, downTarget{name: member.Name, pane: pane})
	}
	targets = append(targets, workers...)
	for _, target := range targets {
		if seen[target.pane] {
			continue
		}
		seen[target.pane] = true
		if target.pane == selfPane {
			// Hold it back: closing this pane first would take hergy with it,
			// and the rest would stay open.
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
		fmt.Fprint(l.out, i18n.Tf("down.absent_only", l.cfg.Team))
		return 0, nil
	}
	if dry {
		fmt.Fprint(l.out, i18n.Tf("down.dry_done", closed, absent))
		return closed, nil
	}
	fmt.Fprint(l.out, i18n.Tf("down.done", closed, absent))
	return closed, nil
}

// workerTargets finds the panes of the workers this team hired: a live agent
// whose tab's label starts with "worker:<team>/". Tabs are never closed as a
// unit; only the worker's own pane is.
func (l *Launcher) workerTargets(agents []protocol.Agent) ([]downTarget, error) {
	tabs, err := l.herdr.Tabs()
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(tabs))
	for _, tab := range tabs {
		labels[tab.ID] = tab.Label
	}
	prefix := "worker:" + l.cfg.Team + "/"
	var targets []downTarget
	for _, agent := range agents {
		if agent.Name == "" || agent.PaneID == "" || agent.TabID == "" {
			continue
		}
		if !strings.HasPrefix(labels[agent.TabID], prefix) {
			continue
		}
		targets = append(targets, downTarget{name: agent.Name, pane: agent.PaneID, worker: true})
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
	case dry && target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_worker", name, pane))
		return nil
	case dry && target.self:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_self", name, pane))
		return nil
	case dry:
		fmt.Fprint(l.out, i18n.Tf("down.would_close", name, pane))
		return nil
	case target.self && target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.closing_worker_self", name, pane))
	case target.worker:
		fmt.Fprint(l.out, i18n.Tf("down.closing_worker", name, pane))
	case target.self:
		fmt.Fprint(l.out, i18n.Tf("down.closing_self", name, pane))
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
	l.mu.Lock()
	lines := l.progress.Report()
	l.mu.Unlock()
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

// preflight is everything that can be checked without changing Herdr; it also
// returns the caller's workspace.
func (l *Launcher) preflight(members []*config.Member) (string, error) {
	for i := range members {
		if !isDir(members[i].Cwd) {
			return "", i18n.Errorf("error.cwd_not_dir", members[i].Cwd)
		}
	}
	if err := l.herdr.Verify(protocol.ModeBoot); err != nil {
		return "", err
	}
	workspace, err := l.herdr.CurrentWorkspace()
	if err != nil {
		return "", err
	}
	live, err := l.herdr.LiveAgentNames()
	if err != nil {
		return "", err
	}
	if conflicts := intersect(namesOf(members), live); len(conflicts) > 0 {
		return "", i18n.Errorf("error.live_names_in_use", strings.Join(conflicts, ", "))
	}
	return workspace, nil
}

// createTab makes the new tab whose root pane the first member holds.
func (l *Launcher) createTab(workspace string, first *config.Member, label string) (string, error) {
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
	return root, nil
}

func (l *Launcher) layOut(step layout.Step) error {
	switch s := step.(type) {
	case layout.Split:
		target, err := l.cfg.Member(s.Target)
		if err != nil {
			return err
		}
		pane, err := l.herdr.Split(l.paneOf(s.Source), s.Direction, target.Cwd, target.Env)
		if err != nil {
			return err
		}
		l.place(s.Target, pane)
	case layout.Rename:
		l.mark(s.Member, i18n.T("progress.mark.pane_created"))
		if err := l.herdr.Rename(l.paneOf(s.Member), s.Member); err != nil {
			return err
		}
	}
	return nil
}

func (l *Launcher) start(member *config.Member) error {
	l.stage(i18n.Tf("progress.stage.starting", member.Name))
	l.mark(member.Name, i18n.T("progress.mark.start_requested"))
	if err := l.herdr.Start(member, l.paneOf(member.Name), l.timeout); err != nil {
		return err
	}
	l.mark(member.Name, i18n.T("progress.mark.ready"))
	return l.submit(member, member.Name)
}

func (l *Launcher) submit(member *config.Member, target string) error {
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
}

func (l *Launcher) mark(name, state string) {
	l.mu.Lock()
	l.progress.Mark(name, state)
	l.mu.Unlock()
}

func (l *Launcher) place(name, pane string) {
	l.mu.Lock()
	l.progress.Place(name, pane)
	l.mu.Unlock()
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

func memberNames(members []config.Member) []string {
	names := make([]string, len(members))
	for i, member := range members {
		names[i] = member.Name
	}
	return names
}

func namesOf(members []*config.Member) []string {
	names := make([]string, len(members))
	for i, member := range members {
		names[i] = member.Name
	}
	return names
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
