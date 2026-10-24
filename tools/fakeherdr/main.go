// Command fakeherdr is the made-up herdr the showcase runs (tools/showcase). It
// answers from the scene's herdr directory and remembers what hergy changes, so
// a person can start and stop a team from the screens: a tab create, a pane
// split, an agent start and prompt, and a pane close write the scene's own state.
//
// The state lives in $SHOWCASE_HERDR_DIR/state.json. The first run reads the
// scene's tabs.json, panes.json, agents.json and workspaces.json; a scene that
// has none of the latter three starts from its tabs and agents. A name listed in
// startup-blocked.json answers "blocked during startup" for the seconds written
// there, so a start from the screens can be walked through a startup question
// (d-d589, audit A5, R1).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type tab struct {
	ID          string `json:"tab_id"`
	Label       string `json:"label"`
	WorkspaceID string `json:"workspace_id"`
}

type pane struct {
	ID          string `json:"pane_id"`
	Label       string `json:"label"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Cwd         string `json:"cwd,omitempty"`
}

type agent struct {
	Name   string `json:"name"`
	Kind   string `json:"agent"`
	Status string `json:"agent_status"`
	PaneID string `json:"pane_id"`
	TabID  string `json:"tab_id"`
	Seq    int    `json:"state_change_seq"`
	Title  string `json:"terminal_title_stripped,omitempty"`
}

type workspace struct {
	ID      string `json:"workspace_id"`
	Label   string `json:"label"`
	Focused bool   `json:"focused"`
}

type state struct {
	Tabs       []tab            `json:"tabs"`
	Panes      []pane           `json:"panes"`
	Agents     []agent          `json:"agents"`
	Workspaces []workspace      `json:"workspaces"`
	NextTab    int              `json:"next_tab"`
	NextPane   int              `json:"next_pane"`
	NextSpace  int              `json:"next_space"`
	Blocked    map[string]int64 `json:"blocked,omitempty"`
}

type result struct {
	Result any `json:"result,omitempty"`
	Error  any `json:"error,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	dir := os.Getenv("SHOWCASE_HERDR_DIR")
	if dir == "" {
		fmt.Fprintln(os.Stderr, "fakeherdr: SHOWCASE_HERDR_DIR is not set")
		return 2
	}
	if len(args) < 2 {
		printHelp()
		return 0
	}
	path := filepath.Join(dir, "state.json")
	st := load(dir, path)
	code := dispatch(st, path, dir, args[0], args[1], args[2:])
	if code == 0 {
		save(path, st)
	}
	return code
}

// printHelp advertises every command the protocol checks for, whichever operation
// asks: herdr prints its own usage for a missing subcommand.
func printHelp() {
	fmt.Println("herdr - the showcase's made-up herdr")
	for _, group := range []struct {
		name string
		subs []string
	}{
		{"agent", []string{"get", "prompt", "start", "list", "read", "focus", "send-keys"}},
		{"pane", []string{"current", "split", "rename", "close", "list", "process-info"}},
		{"tab", []string{"create", "list", "close"}},
		{"workspace", []string{"list", "get", "create", "close"}},
	} {
		for _, sub := range group.subs {
			fmt.Printf("  herdr %s %s\n", group.name, sub)
		}
	}
}

func dispatch(st *state, path, dir, group, sub string, args []string) int {
	switch group + " " + sub {
	case "tab list":
		return answer(map[string]any{"tabs": st.Tabs})
	case "tab create":
		return createTab(st, path, args)
	case "tab close":
		closeTab(st, arg(args, 0))
		return answer(map[string]any{})
	case "pane list":
		return answer(map[string]any{"panes": st.Panes})
	case "pane split":
		return splitPane(st, args)
	case "pane rename":
		renamePane(st, args)
		return answer(map[string]any{})
	case "pane close":
		closePane(st, arg(args, 0))
		return answer(map[string]any{})
	case "pane current":
		space := "w1"
		if len(st.Panes) > 0 && st.Panes[0].WorkspaceID != "" {
			space = st.Panes[0].WorkspaceID
		}
		return answer(map[string]any{"pane": map[string]any{"workspace_id": space}})
	case "pane process-info":
		return answer(map[string]any{"process_info": map[string]any{"foreground_process_group_id": 100, "shell_pid": 100}})
	case "agent list":
		return answer(map[string]any{"agents": st.Agents})
	case "agent get":
		return getAgent(st, path, arg(args, 0))
	case "agent start":
		return startAgent(st, path, dir, args)
	case "agent prompt":
		prompt(st, args)
		return answer(map[string]any{})
	case "agent read":
		if text, err := os.ReadFile(filepath.Join(dir, "screens", arg(args, 0)+".txt")); err == nil {
			fmt.Print(string(text))
			return 0
		}
		return 0
	case "workspace list":
		return answer(map[string]any{"workspaces": st.Workspaces})
	case "workspace get":
		for _, space := range st.Workspaces {
			if space.ID == arg(args, 0) {
				return answer(map[string]any{"workspace": space})
			}
		}
		return fail("workspace_not_found")
	case "workspace create":
		return createWorkspace(st, args)
	case "workspace close":
		closeWorkspace(st, arg(args, 0))
		return answer(map[string]any{})
	}
	return answer(map[string]any{})
}

func answer(value any) int {
	raw, _ := json.Marshal(result{Result: value})
	fmt.Println(string(raw))
	return 0
}

func fail(code string) int {
	raw, _ := json.Marshal(result{Error: map[string]any{"code": code, "message": code}})
	fmt.Println(string(raw))
	return 1
}

func arg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// flagValue is the value after --name in args.
func flagValue(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--"+name {
			return args[i+1]
		}
	}
	return ""
}

func createTab(st *state, _ string, args []string) int {
	st.NextTab++
	id := "t" + strconv.Itoa(st.NextTab)
	paneID := "p" + strconv.Itoa(st.NextPaneOf())
	space := flagValue(args, "workspace")
	label := flagValue(args, "label")
	cwd := flagValue(args, "cwd")
	st.Tabs = append(st.Tabs, tab{ID: id, Label: label, WorkspaceID: space})
	st.Panes = append(st.Panes, pane{ID: paneID, TabID: id, WorkspaceID: space, Cwd: cwd})
	return answer(map[string]any{
		"tab":       map[string]any{"tab_id": id, "label": label, "workspace_id": space},
		"root_pane": map[string]any{"pane_id": paneID, "tab_id": id, "workspace_id": space},
	})
}

func splitPane(st *state, args []string) int {
	paneID := "p" + strconv.Itoa(st.NextPaneOf())
	source := arg(args, 0)
	tabID, space := "", ""
	for _, one := range st.Panes {
		if one.ID == source {
			tabID, space = one.TabID, one.WorkspaceID
		}
	}
	st.Panes = append(st.Panes, pane{ID: paneID, TabID: tabID, WorkspaceID: space, Cwd: flagValue(args, "cwd")})
	return answer(map[string]any{"pane": map[string]any{"pane_id": paneID, "tab_id": tabID, "workspace_id": space}})
}

func renamePane(st *state, args []string) {
	id, label := arg(args, 0), arg(args, 1)
	for i := range st.Panes {
		if st.Panes[i].ID == id {
			st.Panes[i].Label = label
		}
	}
}

func closePane(st *state, id string) {
	st.Panes = removePane(st.Panes, func(one pane) bool { return one.ID == id })
	st.Agents = removeAgent(st.Agents, func(one agent) bool { return one.PaneID == id })
	for _, one := range append([]tab(nil), st.Tabs...) {
		if !tabHasPane(st.Panes, one.ID) {
			st.Tabs = removeTab(st.Tabs, one.ID)
		}
	}
}

func closeTab(st *state, id string) {
	st.Tabs = removeTab(st.Tabs, id)
	st.Panes = removePane(st.Panes, func(one pane) bool { return one.TabID == id })
	st.Agents = removeAgent(st.Agents, func(one agent) bool { return one.TabID == id })
}

func tabHasPane(panes []pane, tabID string) bool {
	for _, one := range panes {
		if one.TabID == tabID {
			return true
		}
	}
	return false
}

func getAgent(st *state, path, target string) int {
	for i := range st.Agents {
		one := &st.Agents[i]
		if one.Name != target && one.PaneID != target {
			continue
		}
		if until := st.Blocked[one.Name]; until > 0 {
			if time.Now().UnixNano() < until {
				one.Status = "blocked"
				save(path, st)
				return answer(map[string]any{"agent": *one})
			}
			delete(st.Blocked, one.Name)
			one.Status = "idle"
			save(path, st)
			return answer(map[string]any{"agent": *one})
		}
		return answer(map[string]any{"agent": *one})
	}
	return fail("agent_not_found")
}

func startAgent(st *state, path, dir string, args []string) int {
	name := arg(args, 0)
	paneID := flagValue(args, "pane")
	tabID, space := "", ""
	for _, one := range st.Panes {
		if one.ID == paneID {
			tabID, space = one.TabID, one.WorkspaceID
		}
	}
	st.Agents = removeAgent(st.Agents, func(one agent) bool { return one.Name == name })
	entry := agent{Name: name, Kind: flagValue(args, "kind"), Status: "idle", PaneID: paneID, TabID: tabID, Seq: nextSeq(st)}
	st.Agents = append(st.Agents, entry)
	_ = space
	if seconds := blockedSeconds(dir, name); seconds > 0 {
		st.Blocked[name] = time.Now().Add(time.Duration(seconds) * time.Second).UnixNano()
		entry.Status = "blocked"
		st.Agents[len(st.Agents)-1] = entry
		save(path, st)
		raw, _ := json.Marshal(result{Error: map[string]any{
			"code": "agent_not_ready", "message": "agent is blocked during startup",
		}})
		fmt.Println(string(raw))
		return 1
	}
	return answer(map[string]any{})
}

func prompt(st *state, args []string) {
	target := arg(args, 0)
	for i := range st.Agents {
		if st.Agents[i].Name == target || st.Agents[i].PaneID == target {
			if st.Agents[i].Status != "blocked" {
				st.Agents[i].Status = "working"
			}
		}
	}
	dir := os.Getenv("SHOWCASE_HERDR_DIR")
	file, err := os.OpenFile(filepath.Join(dir, "prompts.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = fmt.Fprintln(file, target)
		_ = file.Close()
	}
}

func createWorkspace(st *state, args []string) int {
	st.NextSpace++
	if st.NextSpace < 1 {
		st.NextSpace = 1
	}
	for _, one := range st.Workspaces {
		if one.ID == "w"+strconv.Itoa(st.NextSpace) {
			st.NextSpace++
		}
	}
	id := "w" + strconv.Itoa(st.NextSpace)
	label := flagValue(args, "label")
	cwd := flagValue(args, "cwd")
	st.Workspaces = append(st.Workspaces, workspace{ID: id, Label: label})
	st.NextTab++
	tabID := "t" + strconv.Itoa(st.NextTab)
	paneID := "p" + strconv.Itoa(st.NextPaneOf())
	st.Tabs = append(st.Tabs, tab{ID: tabID, Label: "1", WorkspaceID: id})
	st.Panes = append(st.Panes, pane{ID: paneID, TabID: tabID, WorkspaceID: id, Cwd: cwd})
	return answer(map[string]any{
		"workspace": workspace{ID: id, Label: label},
		"tab":       map[string]any{"tab_id": tabID, "label": "1", "workspace_id": id},
		"root_pane": map[string]any{"pane_id": paneID, "tab_id": tabID, "workspace_id": id},
	})
}

func closeWorkspace(st *state, id string) {
	st.Workspaces = removeSpace(st.Workspaces, id)
	for _, one := range append([]tab(nil), st.Tabs...) {
		if one.WorkspaceID == id {
			closeTab(st, one.ID)
		}
	}
}

// blockedSeconds is how long one member waits at startup, from the scene's
// startup-blocked.json: {"<herdr name>": {"seconds": 30}} (audit R1).
func blockedSeconds(dir, name string) int {
	raw, err := os.ReadFile(filepath.Join(dir, "startup-blocked.json"))
	if err != nil {
		return 0
	}
	var waits map[string]struct {
		Seconds int `json:"seconds"`
	}
	if json.Unmarshal(raw, &waits) != nil {
		return 0
	}
	return waits[name].Seconds
}

// NextPaneOf is the next pane number, made unique against the scene's own ids.
func (st *state) NextPaneOf() int {
	st.NextPane++
	for hasPane(st.Panes, "p"+strconv.Itoa(st.NextPane)) {
		st.NextPane++
	}
	return st.NextPane
}

func hasPane(panes []pane, id string) bool {
	for _, one := range panes {
		if one.ID == id {
			return true
		}
	}
	return false
}

func nextSeq(st *state) int {
	highest := 0
	for _, one := range st.Agents {
		if one.Seq > highest {
			highest = one.Seq
		}
	}
	return highest + 1
}

func removePane(panes []pane, drop func(pane) bool) []pane {
	out := panes[:0]
	for _, one := range panes {
		if !drop(one) {
			out = append(out, one)
		}
	}
	return out
}

func removeAgent(agents []agent, drop func(agent) bool) []agent {
	out := agents[:0]
	for _, one := range agents {
		if !drop(one) {
			out = append(out, one)
		}
	}
	return out
}

func removeTab(tabs []tab, id string) []tab {
	out := tabs[:0]
	for _, one := range tabs {
		if one.ID != id {
			out = append(out, one)
		}
	}
	return out
}

func removeSpace(spaces []workspace, id string) []workspace {
	out := spaces[:0]
	for _, one := range spaces {
		if one.ID != id {
			out = append(out, one)
		}
	}
	return out
}

// load reads the scene's files on the first run, and the saved state after that.
func load(dir, path string) *state {
	if raw, err := os.ReadFile(path); err == nil {
		var st state
		if json.Unmarshal(raw, &st) == nil {
			if st.Blocked == nil {
				st.Blocked = map[string]int64{}
			}
			return &st
		}
	}
	st := &state{Blocked: map[string]int64{}}
	readResult(filepath.Join(dir, "tabs.json"), "tabs", &st.Tabs)
	readResult(filepath.Join(dir, "workspaces.json"), "workspaces", &st.Workspaces)
	readResult(filepath.Join(dir, "panes.json"), "panes", &st.Panes)
	readResult(filepath.Join(dir, "agents.json"), "agents", &st.Agents)
	if len(st.Workspaces) == 0 {
		st.Workspaces = []workspace{{ID: "w1", Label: "1"}}
	}
	// A scene that names no panes keeps its agents' panes: the named pane is what
	// makes a team with no live agent a team (d-da92).
	for _, one := range st.Agents {
		if one.PaneID == "" || hasPane(st.Panes, one.PaneID) {
			continue
		}
		st.Panes = append(st.Panes, pane{ID: one.PaneID, Label: one.Name, TabID: one.TabID, WorkspaceID: "w1"})
	}
	if st.NextTab < len(st.Tabs) {
		st.NextTab = len(st.Tabs) + 1
	}
	if st.NextPane < len(st.Panes) {
		st.NextPane = len(st.Panes) + 1
	}
	if st.NextSpace < len(st.Workspaces) {
		st.NextSpace = len(st.Workspaces) + 1
	}
	return st
}

func readResult(path, key string, into any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(doc["result"], &inner) != nil {
		return
	}
	_ = json.Unmarshal(inner[key], into)
}

// save writes the state beside the scene's files, with a rename so a reader never
// sees half of it.
func save(path string, st *state) {
	raw, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
