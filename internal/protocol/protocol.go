// Package protocol interprets the Herdr CLI: its help text, its JSON replies,
// and what a live agent is. It guesses compatibility from no Herdr version;
// it inspects the installed commands and rejects JSON it does not understand.
package protocol

import (
	"encoding/json"
	"errors"
	"strconv"
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
	// Seq is Herdr's state_change_seq: the greater it is, the more recently the
	// agent changed state. The activity screen reads it to tell the last turn
	// (設計 5.2).
	Seq int
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
	agent.Seq = seqOf(object["state_change_seq"])
	// Herdr reports the terminal's title twice: as it stands, and with its own
	// marks (the working mark, the pane's app) stripped. The stripped one is what
	// a person reads as what the agent is doing.
	agent.Title, _ = object["terminal_title_stripped"].(string)
	if agent.Title == "" {
		agent.Title, _ = object["terminal_title"].(string)
	}
	return agent
}

// seqOf reads Herdr's state_change_seq, which a JSON reply decodes as float64.
func seqOf(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case int64:
		return int(number)
	case string:
		n, err := strconv.Atoi(number)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// Tab is one live tab as Herdr reports it.
type Tab struct {
	ID    string
	Label string
	// WorkspaceID is the workspace the tab stands in. The headquarters names
	// the workspace a running team is in (d-73fc).
	WorkspaceID string
}

// TabFrom builds a Tab from a Herdr tab object.
func TabFrom(object map[string]any) Tab {
	tab := Tab{}
	tab.ID, _ = object["tab_id"].(string)
	tab.Label, _ = object["label"].(string)
	tab.WorkspaceID, _ = object["workspace_id"].(string)
	return tab
}

// Pane is one live pane as Herdr reports it. Label is the name pane rename set,
// and TabID is the tab the pane is in.
type Pane struct {
	ID    string
	Label string
	TabID string
	// Cwd is the directory the pane stands in. The headquarters reads it to tell
	// an unrecorded team's project: a tab and its named panes are in the project
	// the work runs in (d-73fc).
	Cwd string
}

// PaneFrom builds a Pane from a Herdr pane object. The name hergy gives a pane
// with pane rename comes back as "label", which is empty for a pane hergy did not
// name.
func PaneFrom(object map[string]any) Pane {
	pane := Pane{}
	pane.ID, _ = object["pane_id"].(string)
	pane.Label, _ = object["label"].(string)
	pane.TabID, _ = object["tab_id"].(string)
	pane.Cwd, _ = object["cwd"].(string)
	return pane
}

// WorkerTabPrefix is how the label of one team's worker tabs starts:
// "worker:<team>/". hergy puts each hired worker in a tab of its own.
func WorkerTabPrefix(teamName string) string { return "worker:" + teamName + "/" }

// NamedForTeam reports whether a herdr name is one hergy gave a member, a worker
// or an agent of this team: it starts with the team's name and a dash,
// "<team>-<name>". A pane hergy did not name has no name at all, which is how down
// tells hergy's panes from a person's (d-da92).
func NamedForTeam(teamName, herdrName string) bool {
	return strings.HasPrefix(herdrName, teamName+"-")
}

// NamesTeam reports whether a tab label stands for one team: the team's name
// alone, or the older "team:<team>@<definition>" / "team:<team>" shapes.
func NamesTeam(label, name string) bool {
	if label == name {
		return true
	}
	rest, ok := strings.CutPrefix(label, "team:")
	if !ok {
		return false
	}
	before, _, _ := strings.Cut(rest, "@")
	return before == name
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
	// A refresh starts a member again where it stands, and gives a member the
	// definition has and hergy has no pane for a pane in the team's tab.
	{Name: "agent", Subs: []string{"get", "prompt", "start"}},
	{Name: "pane", Subs: []string{"list", "process-info", "rename", "split"}},
	{Name: "tab", Subs: []string{"list"}},
}

var downGroups = []Group{
	{Name: "agent", Subs: []string{"list"}},
	{Name: "pane", Subs: []string{"close", "list"}},
	{Name: "tab", Subs: []string{"list"}},
}

var statusGroups = []Group{
	{Name: "agent", Subs: []string{"list", "read"}},
	{Name: "tab", Subs: []string{"list"}},
}

var focusGroups = []Group{
	{Name: "agent", Subs: []string{"focus"}},
}

var promptGroups = []Group{
	{Name: "agent", Subs: []string{"get", "prompt"}},
}

var stopGroups = []Group{
	{Name: "agent", Subs: []string{"get", "send-keys"}},
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
	// ModeStatus reads the live agents, their screens and the tabs; it starts
	// and stops nothing.
	ModeStatus = "status"
	// ModeFocus moves a person's focus to a live agent's pane; it changes
	// nothing else.
	ModeFocus = "focus"
	// ModePrompt sends a live agent a prompt (nudge, tell); it starts nothing.
	ModePrompt = "prompt"
	// ModeStop interrupts a live agent's turn (stop); it starts nothing.
	ModeStop = "stop"
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
	case ModeFocus:
		return focusGroups
	case ModePrompt:
		return promptGroups
	case ModeStop:
		return stopGroups
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
		return nil, Failure(args, stdout, stderr, exitStatus)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(stdout), &data); err != nil {
		return nil, i18n.Errorf("error.herdr_json", err)
	}
	if value, ok := data["error"]; ok && value != nil {
		return nil, &Error{Code: replyCode(stdout), Message: replyMessage(stdout), err: i18n.Errorf("error.herdr_error", value)}
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_no_result")
	}
	return result, nil
}

// Failure is the error a Herdr command that ran and failed gives, from its exit
// status and output. detail is the stderr, or the stdout when stderr is empty;
// Code is Herdr's error code and Message the reason it gives in words, where the
// reply carries them, so a caller can tell one kind of failure from another and
// can say why without showing JSON.
func Failure(args []string, stdout, stderr string, exitStatus int) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	message := replyMessage(detail)
	if message == "" && detail != "" && !json.Valid([]byte(detail)) {
		// Not a reply at all: the command's own words are the reason.
		message = detail
	}
	return &Error{
		Code:    replyCode(detail),
		Message: message,
		err:     i18n.Errorf("error.herdr_failed", strings.Join(firstTwo(args), " "), exitStatus, detail),
	}
}

// Error is a Herdr command that answered with an error. Code is Herdr's error
// code, and Message the reason herdr gave in words; both are empty when the
// reply carried neither.
type Error struct {
	Code    string
	Message string
	err     error
}

func (e *Error) Error() string { return e.err.Error() }

// ErrorCode is the Herdr error code behind err, or empty.
func ErrorCode(err error) string {
	if herdrErr, ok := errors.AsType[*Error](err); ok {
		return herdrErr.Code
	}
	return ""
}

// Message is herdr's own reason for the failure, as a person reads it: the
// message of its JSON error reply, and her own words when the failure is not a
// reply at all (a command that is not there, say). A reply that carries no
// message gets the one sentence hergy has for it; the JSON itself is never
// handed to a person (d-5f02).
func Message(err error) string {
	if herdrErr, ok := errors.AsType[*Error](err); ok {
		if herdrErr.Message != "" {
			return herdrErr.Message
		}
		return i18n.T("error.herdr_no_message")
	}
	return err.Error()
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

// replyMessage reads the reason a JSON error reply gives, or empty.
func replyMessage(reply string) string {
	var data struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(reply), &data) != nil {
		return ""
	}
	return strings.TrimSpace(data.Error.Message)
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
