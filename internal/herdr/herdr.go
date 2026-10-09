// Package herdr is the set of Herdr operations the launcher needs, over a port
// that answers Verify and JSON the way herdrcli.CLI does.
package herdr

import (
	"fmt"
	"sort"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/protocol"
)

// Port is what Herdr needs from the CLI: the command check and JSON replies.
type Port interface {
	Verify(boot bool) error
	JSON(args ...string) (map[string]any, error)
}

// Herdr drives Herdr through a port.
type Herdr struct{ port Port }

// New wraps a port.
func New(port Port) *Herdr { return &Herdr{port: port} }

// Verify checks the installed commands.
func (h *Herdr) Verify(boot bool) error { return h.port.Verify(boot) }

// CurrentWorkspace returns the calling pane's workspace.
func (h *Herdr) CurrentWorkspace() (string, error) {
	result, err := h.port.JSON("pane", "current", "--current")
	if err != nil {
		return "", err
	}
	return protocol.DigString(result, "pane", "workspace_id")
}

// LiveAgentNames lists the names already in use on the server.
func (h *Herdr) LiveAgentNames() ([]string, error) {
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
		return nil, fmt.Errorf("unexpected agents response")
	}
	names := make([]string, 0, len(agents))
	for _, item := range agents {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := object["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

// CreateTab makes the team's tab. onTab runs as soon as the tab id is read, so a
// malformed reply still reports a tab that now exists. It returns the root pane.
func (h *Herdr) CreateTab(workspace, label, cwd string, env map[string]string, onTab func(string)) (string, error) {
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
	return protocol.DigString(result, "root_pane", "pane_id")
}

// Split splits a pane and returns the new pane.
func (h *Herdr) Split(pane, direction, cwd string, env map[string]string) (string, error) {
	args := []string{"pane", "split", pane, "--direction", direction, "--ratio", "0.5", "--cwd", cwd}
	args = append(args, envArgs(env)...)
	args = append(args, "--no-focus")
	result, err := h.port.JSON(args...)
	if err != nil {
		return "", err
	}
	return protocol.DigString(result, "pane", "pane_id")
}

// Rename names a pane after its member.
func (h *Herdr) Rename(pane, name string) error {
	_, err := h.port.JSON("pane", "rename", pane, name)
	return err
}

// Start starts the member's agent in its pane.
func (h *Herdr) Start(member *config.Member, pane string, timeoutMS int) error {
	args := []string{"agent", "start", member.Name, "--kind", member.Kind, "--pane", pane,
		"--timeout", fmt.Sprintf("%d", timeoutMS), "--"}
	args = append(args, member.Args...)
	_, err := h.port.JSON(args...)
	return err
}

// Prompt sends text to a live agent name or a pane id.
func (h *Herdr) Prompt(target, text string) error {
	_, err := h.port.JSON("agent", "prompt", target, text)
	return err
}

// Agent looks up a live agent by name or pane id.
func (h *Herdr) Agent(target string) (protocol.Agent, error) {
	result, err := h.port.JSON("agent", "get", target)
	if err != nil {
		return protocol.Agent{}, err
	}
	object, err := protocol.Dig(result, "agent")
	if err != nil {
		return protocol.Agent{}, err
	}
	mapping, ok := object.(map[string]any)
	if !ok {
		return protocol.Agent{}, fmt.Errorf("unexpected Herdr response: agent is not an object")
	}
	return protocol.AgentFrom(mapping), nil
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
