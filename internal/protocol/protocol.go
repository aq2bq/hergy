// Package protocol interprets the Herdr CLI: its help text, its JSON replies,
// and what a live agent is. It guesses compatibility from no Herdr version;
// it inspects the installed commands and rejects JSON it does not understand.
package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Agent is a live agent as Herdr reports it.
type Agent struct {
	Name   string
	Kind   string
	Status string
	PaneID string
}

// Check verifies the live agent still is the member the config describes, and
// is not in the middle of work.
func (a Agent) Check(name, kind string) error {
	if a.Name != name || a.Kind != kind {
		return fmt.Errorf("%s: live name/kind does not match the config", name)
	}
	if a.Status != "idle" && a.Status != "done" {
		return fmt.Errorf("%s: status=%q; reset/settle the agent before refreshing", name, a.Status)
	}
	return nil
}

// AgentFrom builds an Agent from a Herdr agent object.
func AgentFrom(object map[string]any) Agent {
	agent := Agent{}
	agent.Name, _ = object["name"].(string)
	agent.Kind, _ = object["agent"].(string)
	agent.Status, _ = object["agent_status"].(string)
	agent.PaneID, _ = object["pane_id"].(string)
	return agent
}

// Group is a Herdr command group and the subcommands the launcher needs.
type Group struct {
	Name string
	Subs []string
}

var bootGroups = []Group{
	{Name: "agent", Subs: []string{"get", "prompt", "start", "list"}},
	{Name: "pane", Subs: []string{"current", "split", "rename"}},
	{Name: "tab", Subs: []string{"create"}},
}

var refreshGroups = []Group{
	{Name: "agent", Subs: []string{"get", "prompt"}},
}

// RequiredCommands is the groups and subcommands needed for a boot (or a hire)
// or a refresh. Check them in this order.
func RequiredCommands(boot bool) []Group {
	if boot {
		return bootGroups
	}
	return refreshGroups
}

// CheckHelp fails when `herdr <group>` does not advertise the subcommands the
// launcher needs. `herdr <group>` prints usage and may exit 2 for a missing one.
func CheckHelp(group string, subs []string, help string, exitStatus int) error {
	if exitStatus != 0 && exitStatus != 2 {
		return fmt.Errorf("cannot inspect installed Herdr %s CLI (exit %d): %s", group, exitStatus, strings.TrimSpace(help))
	}
	if !strings.Contains(help, "herdr "+group) {
		return fmt.Errorf("cannot inspect installed Herdr %s CLI: %s", group, strings.TrimSpace(help))
	}
	for _, sub := range subs {
		if !strings.Contains(help, "herdr "+group+" "+sub) {
			return fmt.Errorf("installed Herdr does not advertise %s %s; inspect `herdr %s`", group, sub, group)
		}
	}
	return nil
}

// Result turns a Herdr JSON reply into its result object, or an error.
func Result(args []string, stdout, stderr string, exitStatus int) (map[string]any, error) {
	if exitStatus != 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = strings.TrimSpace(stdout)
		}
		return nil, fmt.Errorf("herdr %s failed (%d): %s", strings.Join(firstTwo(args), " "), exitStatus, detail)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(stdout), &data); err != nil {
		return nil, fmt.Errorf("unexpected Herdr JSON: %v", err)
	}
	if value, ok := data["error"]; ok && value != nil {
		return nil, fmt.Errorf("Herdr error: %v", value)
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Herdr response has no result object")
	}
	return result, nil
}

// Dig walks a nested object, failing when a key is missing.
func Dig(object map[string]any, keys ...string) (any, error) {
	var current any = object
	for _, key := range keys {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected Herdr response: missing %s", key)
		}
		value, ok := mapping[key]
		if !ok || value == nil {
			return nil, fmt.Errorf("unexpected Herdr response: missing %s", key)
		}
		current = value
	}
	return current, nil
}

// DigString is Dig for a string value.
func DigString(object map[string]any, keys ...string) (string, error) {
	value, err := Dig(object, keys...)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected Herdr response: %s is not a string", keys[len(keys)-1])
	}
	return text, nil
}

func firstTwo(args []string) []string {
	if len(args) <= 2 {
		return args
	}
	return args[:2]
}
