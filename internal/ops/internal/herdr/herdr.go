// Package herdr is the set of Herdr operations the launcher needs, over a port
// that answers Verify and JSON the way herdrcli.CLI does.
package herdr

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
)

// Port is what Herdr needs from the CLI: the command check, JSON replies, and
// a command's plain-text output (agent read).
type Port interface {
	Verify(mode string) error
	JSON(args ...string) (map[string]any, error)
	Text(args ...string) (string, error)
}

// Herdr drives Herdr through a port.
type Herdr struct{ port Port }

// New wraps a port.
func New(port Port) *Herdr { return &Herdr{port: port} }

// Verify checks the installed commands.
func (h *Herdr) Verify(mode string) error { return h.port.Verify(mode) }

// CurrentWorkspace returns the calling pane's workspace.
func (h *Herdr) CurrentWorkspace() (string, error) {
	result, err := h.port.JSON("pane", "current", "--current")
	if err != nil {
		return "", err
	}
	return protocol.DigString(result, "pane", "workspace_id")
}

// Workspace is one herdr workspace: its id, the label on it, and whether it is
// focused. The headquarters offers these, and names the one a team runs in
// (d-73fc).
type Workspace struct {
	ID      string
	Label   string
	Focused bool
}

// Workspaces lists the server's workspaces, in herdr's order.
func (h *Herdr) Workspaces() ([]Workspace, error) {
	result, err := h.port.JSON("workspace", "list")
	if err != nil {
		return nil, err
	}
	raw, err := protocol.Dig(result, "workspaces")
	if err != nil {
		return nil, err
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_workspaces")
	}
	out := make([]Workspace, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, workspaceFrom(object))
	}
	return out, nil
}

// Workspace reads one workspace by id.
func (h *Herdr) Workspace(id string) (Workspace, error) {
	result, err := h.port.JSON("workspace", "get", id)
	if err != nil {
		return Workspace{}, err
	}
	raw, err := protocol.Dig(result, "workspace")
	if err != nil {
		return Workspace{}, err
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return Workspace{}, i18n.Errorf("error.herdr_workspaces")
	}
	return workspaceFrom(object), nil
}

// CreateWorkspace makes a workspace with this label and returns it. It does not
// move the person's focus: a start from the screens leaves them where they are.
func (h *Herdr) CreateWorkspace(label, cwd string) (Workspace, error) {
	args := []string{"workspace", "create", "--label", label, "--no-focus"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	result, err := h.port.JSON(args...)
	if err != nil {
		return Workspace{}, err
	}
	raw, err := protocol.Dig(result, "workspace")
	if err != nil {
		return Workspace{}, err
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return Workspace{}, i18n.Errorf("error.herdr_workspaces")
	}
	return workspaceFrom(object), nil
}

// workspaceFrom reads one workspace object.
func workspaceFrom(object map[string]any) Workspace {
	space := Workspace{}
	space.ID, _ = object["workspace_id"].(string)
	space.Label, _ = object["label"].(string)
	space.Focused, _ = object["focused"].(bool)
	return space
}

// LiveAgents lists the agents running on the server, in Herdr's order.
func (h *Herdr) LiveAgents() ([]protocol.Agent, error) {
	result, err := h.port.JSON("agent", "list")
	if err != nil {
		return nil, err
	}
	raw, err := protocol.Dig(result, "agents")
	if err != nil {
		return nil, err
	}
	agents, ok := raw.([]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_agents")
	}
	live := make([]protocol.Agent, 0, len(agents))
	for _, item := range agents {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		live = append(live, protocol.AgentFrom(object))
	}
	return live, nil
}

// Focus moves the focus to an agent: the name herdr knows it by, or the pane it
// runs in. It is what the WebUI's "go to this member's pane" asks for, and the
// only thing hergy does that moves a person's focus; herdr also marks the agent
// seen, so its done mark clears.
func (h *Herdr) Focus(target domainname.Target) error {
	_, err := h.port.JSON("agent", "focus", target.String())
	return err
}

// LiveAgentNames lists the names already in use on the server.
func (h *Herdr) LiveAgentNames() ([]string, error) {
	agents, err := h.LiveAgents()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent.Name.String() != "" {
			names = append(names, agent.Name.String())
		}
	}
	return names, nil
}

// Tabs lists the server's tabs, in Herdr's order. Without a workspace, herdr tab
// list reports every tab, whichever workspace it belongs to.
func (h *Herdr) Tabs() ([]protocol.Tab, error) {
	result, err := h.port.JSON("tab", "list")
	if err != nil {
		return nil, err
	}
	raw, err := protocol.Dig(result, "tabs")
	if err != nil {
		return nil, err
	}
	tabs, ok := raw.([]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_tabs")
	}
	out := make([]protocol.Tab, 0, len(tabs))
	for _, item := range tabs {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, protocol.TabFrom(object))
	}
	return out, nil
}

// FindPane returns the id of the pane hergy named with this label, or "" when no
// pane carries it. hergy names a pane after its member with pane rename.
func (h *Herdr) FindPane(label domainname.Target) (domainname.PaneID, error) {
	panes, err := h.Panes()
	if err != nil {
		return "", err
	}
	for _, pane := range panes {
		if pane.Label.String() == label.String() {
			return pane.ID, nil
		}
	}
	return "", nil
}

// Panes lists the server's panes, with the label hergy gave each and the tab it
// is in. A pane hergy did not name has no label.
func (h *Herdr) Panes() ([]protocol.Pane, error) {
	result, err := h.port.JSON("pane", "list")
	if err != nil {
		return nil, err
	}
	raw, err := protocol.Dig(result, "panes")
	if err != nil {
		return nil, err
	}
	panes, ok := raw.([]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_panes")
	}
	out := make([]protocol.Pane, 0, len(panes))
	for _, item := range panes {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, protocol.PaneFrom(object))
	}
	return out, nil
}

// CloseTab closes one tab, and the panes in it. hergy closes a worker's own tab
// only when the worker's pane was the last one in it (d-fa05).
func (h *Herdr) CloseTab(tab string) error {
	_, err := h.port.JSON("tab", "close", tab)
	return err
}

// ClosePane closes one pane and nothing else. A tab is never closed as a unit:
// the human may have added their own shell to the team's tab.
func (h *Herdr) ClosePane(pane domainname.PaneID) error {
	_, err := h.port.JSON("pane", "close", pane.String())
	return err
}

// CreateTab makes the team's tab. onTab runs as soon as the tab id is read, so a
// malformed reply still reports a tab that now exists. It returns the root pane.
func (h *Herdr) CreateTab(workspace, label, cwd string, env map[string]string, onTab func(string)) (domainname.PaneID, error) {
	args := []string{"tab", "create", "--workspace", workspace, "--label", label, "--cwd", cwd}
	args = append(args, envArgs(env)...)
	args = append(args, "--no-focus")
	result, err := h.port.JSON(args...)
	if err != nil {
		return "", err
	}
	tabID, err := protocol.DigString(result, "tab", "tab_id")
	if err != nil {
		return "", err
	}
	if onTab != nil {
		onTab(tabID)
	}
	id, err := protocol.DigString(result, "root_pane", "pane_id")
	return domainname.PaneID(id), err
}

// Split splits a pane and returns the new pane.
func (h *Herdr) Split(pane domainname.PaneID, direction, cwd string, env map[string]string) (domainname.PaneID, error) {
	args := []string{"pane", "split", pane.String(), "--direction", direction, "--ratio", "0.5", "--cwd", cwd}
	args = append(args, envArgs(env)...)
	args = append(args, "--no-focus")
	result, err := h.port.JSON(args...)
	if err != nil {
		return "", err
	}
	id, err := protocol.DigString(result, "pane", "pane_id")
	return domainname.PaneID(id), err
}

// SplitRatio splits a pane, the pane it splits keeping ratio of the room, and
// returns the new pane: the strip under the members a team's watcher runs in
// (d-5b60).
func (h *Herdr) SplitRatio(pane domainname.PaneID, direction string, ratio float64, cwd string, env map[string]string) (domainname.PaneID, error) {
	args := []string{"pane", "split", pane.String(), "--direction", direction, "--ratio", strconv.FormatFloat(ratio, 'f', 2, 64), "--cwd", cwd}
	args = append(args, envArgs(env)...)
	args = append(args, "--no-focus")
	result, err := h.port.JSON(args...)
	if err != nil {
		return "", err
	}
	id, err := protocol.DigString(result, "pane", "pane_id")
	return domainname.PaneID(id), err
}

// Resize moves the split a pane stands in by amount of the tab's room, in a
// direction (up, down, left, right), and says whether anything moved: herdr
// answers changed=false at a pane's smallest height (measured 2026-11-09: a
// strip keeps 6 rows). An answer without the flag counts as not moved.
func (h *Herdr) Resize(pane domainname.PaneID, direction string, amount float64) (bool, error) {
	result, err := h.port.JSON("pane", "resize", "--pane", pane.String(), "--direction", direction, "--amount", strconv.FormatFloat(amount, 'f', 2, 64))
	if err != nil {
		return false, err
	}
	value, err := protocol.Dig(result, "resize", "changed")
	if err != nil {
		return false, nil
	}
	changed, _ := value.(bool)
	return changed, nil
}

// PaneRows is how many rows a pane shows (herdr pane get's viewport_rows).
func (h *Herdr) PaneRows(pane domainname.PaneID) (int, error) {
	result, err := h.port.JSON("pane", "get", pane.String())
	if err != nil {
		return 0, err
	}
	return protocol.DigInt(result, "pane", "scroll", "viewport_rows")
}

// Run runs one command in a pane's shell. herdr prints nothing on success, so
// the answer is read as text, not JSON.
func (h *Herdr) Run(pane domainname.PaneID, command []string) error {
	_, err := h.port.Text(append([]string{"pane", "run", pane.String()}, command...)...)
	return err
}

// Rename names a pane after its member.
func (h *Herdr) Rename(pane domainname.PaneID, name domainname.PaneLabel) error {
	_, err := h.port.JSON("pane", "rename", pane.String(), name.String())
	return err
}

// paneBusy is Herdr's answer when the pane is not at its shell prompt, as a new
// pane is while its shell startup files still run.
const paneBusy = "agent_pane_busy"

// Start starts the member's agent in its pane. Herdr has no command that waits
// for a new pane's shell, so a busy pane is tried again until the timeout.
func (h *Herdr) Start(member *definition.Start, pane domainname.PaneID, timeoutMS int) error {
	args := []string{"agent", "start", member.Name.String(), "--kind", member.Kind, "--pane", pane.String(),
		"--timeout", fmt.Sprintf("%d", timeoutMS), "--"}
	args = append(args, member.StartArgs()...)
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	for {
		_, err := h.port.JSON(args...)
		if err == nil || protocol.ErrorCode(err) != paneBusy || time.Now().After(deadline) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// StartupBlockedCode is Herdr's error code when a start is refused because the
// agent the start made is asking a human a question and is not ready for
// prompts, and StartupBlockedMark is what its message says: "blocked during
// startup". The agent is live and herdr knows it by name; the start only failed.
const (
	StartupBlockedCode = "agent_not_ready"
	StartupBlockedMark = "blocked during startup"
)

// StartupBlocked is whether a start failed only because the agent is waiting for
// a human to answer a question while it starts (whether to trust a folder, for
// example). That is not a failure to report: the agent handed the turn to the
// person. Every other start failure is the caller's to report (d-d589).
func StartupBlocked(err error) bool {
	return protocol.ErrorCode(err) == StartupBlockedCode && strings.Contains(err.Error(), StartupBlockedMark)
}

// Prompt sends text to a live agent name or a pane id.
func (h *Herdr) Prompt(target domainname.Target, text string) error {
	_, err := h.port.JSON("agent", "prompt", target.String(), text)
	return err
}

// PromptWait sends text to a live agent and waits until herdr sees it working,
// up to timeoutMS. ConfirmPrompt uses this for each receipt attempt; the watcher
// also uses a single attempt when asking a member who its reply is for. An agent that does not start is herdr's
// agent_prompt_stalled error; one that is blocked is agent_blocked.
func (h *Herdr) PromptWait(target domainname.Target, text string, timeoutMS int) error {
	_, err := h.port.JSON("agent", "prompt", target.String(), text, "--wait", "--until", "working", "--timeout", fmt.Sprintf("%d", timeoutMS))
	return err
}

// ReadRecent reads the last lines of a live agent's terminal, unwrapped, from
// the scrollback: what the agent wrote, for hergy turns to hand on.
func (h *Herdr) ReadRecent(target domainname.Target, lines int) (string, error) {
	return h.port.Text("agent", "read", target.String(), "--source", "recent-unwrapped", "--lines", fmt.Sprintf("%d", lines))
}

// SendKeys presses keys in a live agent's pane. hergy stop sends esc, which ends
// the agent's current turn and keeps its conversation.
func (h *Herdr) SendKeys(target domainname.Target, key string) error {
	_, err := h.port.JSON("agent", "send-keys", target.String(), key)
	return err
}

// Read reads what a live agent's terminal shows, from the detection source: the
// snapshot hergy status hashes to tell whether a working member's screen has
// changed. It is a plain-text command, not JSON.
func (h *Herdr) Read(target domainname.Target) (string, error) {
	return h.port.Text("agent", "read", target.String(), "--source", "detection")
}

// Agent looks up a live agent by name or pane id.
func (h *Herdr) Agent(target domainname.Target) (protocol.Agent, error) {
	result, err := h.port.JSON("agent", "get", target.String())
	if err != nil {
		return protocol.Agent{}, err
	}
	object, err := protocol.Dig(result, "agent")
	if err != nil {
		return protocol.Agent{}, err
	}
	mapping, ok := object.(map[string]any)
	if !ok {
		return protocol.Agent{}, i18n.Errorf("error.herdr_agent_object")
	}
	return protocol.AgentFrom(mapping), nil
}

// AgentNotFound is Herdr's error code when an agent name is not live.
const AgentNotFound = "agent_not_found"

// ProcessInfo is what a pane's process tree shows: the foreground process
// group, and the pid of the shell the pane runs.
type ProcessInfo struct {
	Pgid     int
	ShellPID int
}

// PaneProcessInfo reads the pane's foreground process group and shell pid. When
// no agent is running, the foreground group is the shell itself.
func (h *Herdr) PaneProcessInfo(pane domainname.PaneID) (ProcessInfo, error) {
	result, err := h.port.JSON("pane", "process-info", "--pane", pane.String())
	if err != nil {
		return ProcessInfo{}, err
	}
	pgid, err := protocol.DigInt(result, "process_info", "foreground_process_group_id")
	if err != nil {
		return ProcessInfo{}, err
	}
	shellPID, err := protocol.DigInt(result, "process_info", "shell_pid")
	if err != nil {
		return ProcessInfo{}, err
	}
	return ProcessInfo{Pgid: pgid, ShellPID: shellPID}, nil
}

// envArgs passes a pane's environment; Herdr agent start has no env of its own,
// so the pane's shell carries it to the agent.
func envArgs(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := sortedKeys(env)
	args := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		args = append(args, "--env", key+"="+env[key])
	}
	return args
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
