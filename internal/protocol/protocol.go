// Package protocol interprets the Herdr CLI: its help text, its JSON replies,
// and what a live agent is. It guesses compatibility from no Herdr version;
// it inspects the installed commands and rejects JSON it does not understand.
package protocol

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Agent is a live agent as Herdr reports it.
type Agent struct {
	Name   string
	Kind   string
	Status string
	PaneID string
	TabID  string
	// Title is what the agent's terminal says it is doing, with Herdr's own
	// marks stripped. It is empty when Herdr reports none.
	Title string
}

// Check verifies the live agent still is the member the config describes, and
// is not in the middle of work.
func (a Agent) Check(name, kind string) error {
	if a.Name != name || a.Kind != kind {
		return i18n.Errorf("error.live_mismatch", name)
	}
	return a.CheckLive(name)
}

// CheckLive verifies the live agent is the member and is not in the middle of
// work, without comparing the kind. refresh reads the settings again first, so
// the live agent is still running the kind those settings had before an edit;
// requiring the new kind here would block the very refresh that applies it.
func (a Agent) CheckLive(name string) error {
	if a.Name != name {
		return i18n.Errorf("error.live_name_mismatch", name)
	}
	if a.Status != "idle" && a.Status != "done" {
		return i18n.Errorf("error.reset_before_refresh", name, a.Status)
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
	agent.TabID, _ = object["tab_id"].(string)
	// Herdr reports the terminal's title twice: as it stands, and with its own
	// marks (the working mark, the pane's app) stripped. The stripped one is what
	// a person reads as what the agent is doing.
	agent.Title, _ = object["terminal_title_stripped"].(string)
	if agent.Title == "" {
		agent.Title, _ = object["terminal_title"].(string)
	}
	return agent
}

// Tab is one live tab as Herdr reports it.
type Tab struct {
	ID    string
	Label string
}

// TabFrom builds a Tab from a Herdr tab object.
func TabFrom(object map[string]any) Tab {
	tab := Tab{}
	tab.ID, _ = object["tab_id"].(string)
	tab.Label, _ = object["label"].(string)
	return tab
}

// Pane is one live pane as Herdr reports it. Label is the name pane rename set.
type Pane struct {
	ID    string
	Label string
}

// PaneFrom builds a Pane from a Herdr pane object. The name hergy gives a pane
// with pane rename comes back as "label".
func PaneFrom(object map[string]any) Pane {
	pane := Pane{}
	pane.ID, _ = object["pane_id"].(string)
	pane.Label, _ = object["label"].(string)
	return pane
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

var repromptGroups = []Group{
	{Name: "agent", Subs: []string{"get", "prompt"}},
}

var refreshGroups = []Group{
	{Name: "agent", Subs: []string{"get", "prompt", "start"}},
	{Name: "pane", Subs: []string{"process-info", "list"}},
}

var downGroups = []Group{
	{Name: "agent", Subs: []string{"list"}},
	{Name: "pane", Subs: []string{"close"}},
	{Name: "tab", Subs: []string{"list"}},
}

var statusGroups = []Group{
	{Name: "agent", Subs: []string{"list"}},
	{Name: "tab", Subs: []string{"list"}},
}

// Mode is one hergy operation, which needs its own commands from Herdr.
const (
	// ModeBoot is a boot or a hire: it makes panes and starts agents.
	ModeBoot = "boot"
	// ModeRefresh restarts live agents: it stops each one and starts it again.
	ModeRefresh = "refresh"
	// ModeReprompt re-prompts live agents in place; it starts nothing.
	ModeReprompt = "reprompt"
	// ModeDown closes live agents' panes; it starts nothing.
	ModeDown = "down"
	// ModeStatus reads the live agents and tabs; it starts and stops nothing.
	ModeStatus = "status"
)

// RequiredCommands is the groups and subcommands needed for one operation.
// Check them in this order.
func RequiredCommands(mode string) []Group {
	switch mode {
	case ModeRefresh:
		return refreshGroups
	case ModeReprompt:
		return repromptGroups
	case ModeDown:
		return downGroups
	case ModeStatus:
		return statusGroups
	default:
		return bootGroups
	}
}

// CheckHelp fails when `herdr <group>` does not advertise the subcommands the
// launcher needs. `herdr <group>` prints usage and may exit 2 for a missing one.
func CheckHelp(group string, subs []string, help string, exitStatus int) error {
	if exitStatus != 0 && exitStatus != 2 {
		return i18n.Errorf("error.herdr_inspect_exit", group, exitStatus, strings.TrimSpace(help))
	}
	if !strings.Contains(help, "herdr "+group) {
		return i18n.Errorf("error.herdr_inspect", group, strings.TrimSpace(help))
	}
	for _, sub := range subs {
		if !strings.Contains(help, "herdr "+group+" "+sub) {
			return i18n.Errorf("error.herdr_not_advertise", group, sub, group)
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
		return nil, &Error{
			Code: replyCode(detail),
			err:  i18n.Errorf("error.herdr_failed", strings.Join(firstTwo(args), " "), exitStatus, detail),
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(stdout), &data); err != nil {
		return nil, i18n.Errorf("error.herdr_json", err)
	}
	if value, ok := data["error"]; ok && value != nil {
		return nil, &Error{Code: replyCode(stdout), err: i18n.Errorf("error.herdr_error", value)}
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_no_result")
	}
	return result, nil
}

// Error is a Herdr command that answered with an error. Code is Herdr's error
// code, empty when the reply carried none.
type Error struct {
	Code string
	err  error
}

func (e *Error) Error() string { return e.err.Error() }

// ErrorCode is the Herdr error code behind err, or empty.
func ErrorCode(err error) string {
	var herdrErr *Error
	if errors.As(err, &herdrErr) {
		return herdrErr.Code
	}
	return ""
}

// replyCode reads the code of a JSON error reply, or empty.
func replyCode(reply string) string {
	var data struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(reply), &data) != nil {
		return ""
	}
	return data.Error.Code
}

// Dig walks a nested object, failing when a key is missing.
func Dig(object map[string]any, keys ...string) (any, error) {
	var current any = object
	for _, key := range keys {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, i18n.Errorf("error.herdr_missing", key)
		}
		value, ok := mapping[key]
		if !ok || value == nil {
			return nil, i18n.Errorf("error.herdr_missing", key)
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
		return "", i18n.Errorf("error.herdr_not_string", keys[len(keys)-1])
	}
	return text, nil
}

// DigInt is Dig for a whole-number value. A JSON reply decodes its numbers as
// float64; a test that builds the reply as a Go map may use any int width.
func DigInt(object map[string]any, keys ...string) (int, error) {
	value, err := Dig(object, keys...)
	if err != nil {
		return 0, err
	}
	switch number := value.(type) {
	case float64:
		return int(number), nil
	case int:
		return number, nil
	case int64:
		return int(number), nil
	}
	return 0, i18n.Errorf("error.herdr_not_number", keys[len(keys)-1])
}

func firstTwo(args []string) []string {
	if len(args) <= 2 {
		return args
	}
	return args[:2]
}
