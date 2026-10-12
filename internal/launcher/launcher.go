// Package launcher drives a boot, a refresh, a hire or a down through Herdr and
// records how far it got. It is the layer that touches Herdr and the
// filesystem; the values it works on are immutable.
package launcher

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

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

	mu       sync.Mutex
	progress *progress.Progress
}

// New builds a launcher.
func New(cfg *config.Config, h *herdr.Herdr, timeoutMS int, out io.Writer) *Launcher {
	return &Launcher{cfg: cfg, timeout: timeoutMS, herdr: h, out: out, progress: progress.Start(memberNames(cfg.Members))}
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

// Refresh re-submits the bootstrap prompt to idle agents. It starts nothing.
func (l *Launcher) Refresh(members []*config.Member) error {
	if err := l.herdr.Verify(protocol.ModeRefresh); err != nil {
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
	fmt.Fprintf(l.out, i18n.T("launcher.refresh_submitted"), strings.Join(namesOf(members), ", "))
	return nil
}

// Hire starts one worker in a tab of its own, so the team layout stays as booted.
func (l *Launcher) Hire(worker *config.Member) error {
	l.progress = progress.Start([]string{worker.Name})
	workspace, err := l.preflight([]*config.Member{worker})
	if err != nil {
		return err
	}
	pane, err := l.createTab(workspace, worker, "worker:"+worker.Name)
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

// downTarget is one member hergy down acts on: its live pane, and whether that
// pane is the one hergy itself runs in.
type downTarget struct {
	member *config.Member
	pane   string
	self   bool
}

// Down closes the panes of the team's live members. It finds a member by name
// among the live agents; a member that is not live is reported and skipped, and
// a team with nobody live closes nothing. The pane hergy itself runs in is
// closed last, so the rest of the team still get closed. With dry, it prints
// what would close and closes nothing.
func (l *Launcher) Down(members []*config.Member, selfPane string, dry bool) error {
	if !dry {
		if err := l.herdr.Verify(protocol.ModeDown); err != nil {
			return err
		}
	}
	panes, err := l.livePanes()
	if err != nil {
		return err
	}
	l.stage(i18n.T("progress.stage.closing"))
	closed, absent := 0, 0
	var self *downTarget
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
		target := downTarget{member: member, pane: pane, self: pane == selfPane}
		if target.self {
			// Hold it back: closing this pane first would take hergy with it,
			// and the rest of the team would stay open.
			held := target
			self = &held
			continue
		}
		if err := l.closePane(target, dry); err != nil {
			return err
		}
		closed++
	}
	if self != nil {
		if err := l.closePane(*self, dry); err != nil {
			return err
		}
		closed++
	}
	if closed == 0 {
		fmt.Fprint(l.out, i18n.Tf("down.absent_only", l.cfg.Team))
		return nil
	}
	if dry {
		fmt.Fprint(l.out, i18n.Tf("down.dry_done", l.cfg.Team, closed, absent))
		return nil
	}
	fmt.Fprint(l.out, i18n.Tf("down.done", l.cfg.Team, closed, absent))
	return nil
}

// closePane reports one pane and, unless dry, closes it. The line comes first:
// when the pane is hergy's own, the close ends the process before anything else
// could be printed.
func (l *Launcher) closePane(target downTarget, dry bool) error {
	name, pane := target.member.Name, target.pane
	switch {
	case dry && target.self:
		fmt.Fprint(l.out, i18n.Tf("down.would_close_self", name, pane))
		return nil
	case dry:
		fmt.Fprint(l.out, i18n.Tf("down.would_close", name, pane))
		return nil
	case target.self:
		fmt.Fprint(l.out, i18n.Tf("down.closing_self", name, pane))
	default:
		fmt.Fprint(l.out, i18n.Tf("down.closing", name, pane))
	}
	return l.herdr.ClosePane(pane)
}

// livePanes maps each live agent name to its pane. The first pane wins, so a
// duplicate name never leads to the same pane being closed twice.
func (l *Launcher) livePanes() (map[string]string, error) {
	agents, err := l.herdr.LiveAgents()
	if err != nil {
		return nil, err
	}
	panes := make(map[string]string, len(agents))
	for _, agent := range agents {
		if agent.Name == "" || agent.PaneID == "" {
			continue
		}
		if _, ok := panes[agent.Name]; !ok {
			panes[agent.Name] = agent.PaneID
		}
	}
	return panes, nil
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
