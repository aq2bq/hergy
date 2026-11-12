package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"errors"
	"os/exec"
)

type appSocketRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func startAppHerdrSocket(t *testing.T, handle func(appSocketRequest) map[string]any) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ha-")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "s")
	listener, err := net.Listen("unix", path)
	if err != nil {
		_ = os.RemoveAll(dir)
		t.Fatal(err)
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() { _ = listener.Close() })
		<-done
		_ = os.RemoveAll(dir)
	}
	t.Cleanup(stop)
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			var request appSocketRequest
			if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&request); err == nil {
				response := handle(request)
				if _, ok := response["id"]; !ok {
					response["id"] = request.ID
				}
				_ = json.NewEncoder(conn).Encode(response)
			}
			_ = conn.Close()
		}
	}()
	return path
}
func startHerdrCLIProxy(t *testing.T) string {
	t.Helper()
	command := fixtureCommandPort{path: os.Getenv("HERGY_HERDR")}
	path := startAppHerdrSocket(t, func(request appSocketRequest) map[string]any {
		switch request.Method {
		case "session.snapshot":
			snapshot := map[string]any{}
			for _, group := range []struct {
				name, field string
				required    bool
			}{
				{"workspace", "workspaces", false},
				{"tab", "tabs", true},
				{"pane", "panes", false},
				{"agent", "agents", true},
			} {
				result, err := command.JSON(group.name, "list")
				if err != nil {
					if group.required {
						return appSocketError(request.ID, err)
					}
					snapshot[group.field] = []any{}
					continue
				}
				items, ok := result[group.field].([]any)
				if !ok {
					items = []any{}
				}
				snapshot[group.field] = items
			}
			snapshot["version"] = "test"
			snapshot["protocol"] = 22
			snapshot["layouts"] = []any{}
			return appSocketResult(request.ID, map[string]any{"type": "session_snapshot", "snapshot": snapshot})
		case "agent.get":
			result, err := command.JSON("agent", "get", fmt.Sprint(request.Params["target"]))
			if err != nil {
				return appSocketError(request.ID, err)
			}
			result["type"] = "agent_info"
			return appSocketResult(request.ID, result)
		case "agent.read":
			text, err := command.Text("agent", "read", fmt.Sprint(request.Params["target"]), "--source", fmt.Sprint(request.Params["source"]))
			if err != nil {
				return appSocketError(request.ID, err)
			}
			return appSocketResult(request.ID, map[string]any{"type": "pane_read", "read": map[string]any{"text": text}})
		case "pane.process_info":
			result, err := command.JSON("pane", "process-info", "--pane", fmt.Sprint(request.Params["pane_id"]))
			if err != nil {
				return appSocketError(request.ID, err)
			}
			result["type"] = "pane_process_info"
			return appSocketResult(request.ID, result)
		default:
			return map[string]any{"id": request.ID, "error": map[string]any{"code": "unknown_method", "message": request.Method}}
		}
	})
	t.Setenv("HERDR_SOCKET_PATH", path)
	return path
}
func appSocketResult(id string, result map[string]any) map[string]any {
	return map[string]any{"id": id, "result": result}
}
func appSocketError(id string, err error) map[string]any {
	return map[string]any{"id": id, "error": map[string]any{"code": fixtureErrorCode(err), "message": err.Error()}}
}
func startActivityHerdrSocket(t *testing.T, work string) string {
	t.Helper()
	path := startAppHerdrSocket(t, func(request appSocketRequest) map[string]any {
		switch request.Method {
		case "session.snapshot":
			tabs, agents, err := activityFixtureSnapshot(work)
			if err != nil {
				return map[string]any{"id": request.ID, "error": map[string]any{"code": "fixture_error", "message": err.Error()}}
			}
			return appSocketResult(request.ID, map[string]any{"type": "session_snapshot", "snapshot": map[string]any{
				"version": "test", "protocol": 22, "workspaces": []any{}, "tabs": tabs, "panes": []any{}, "layouts": []any{}, "agents": agents,
			}})
		case "agent.read":
			target := fmt.Sprint(request.Params["target"])
			_, agents, err := activityFixtureSnapshot(work)
			if err != nil {
				return map[string]any{"id": request.ID, "error": map[string]any{"code": "fixture_error", "message": err.Error()}}
			}
			agent := activityAgent(agents, target)
			if agent == nil {
				return map[string]any{"id": request.ID, "error": map[string]any{"code": "agent_not_found", "message": "agent not found"}}
			}
			name := fmt.Sprint(agent["name"])
			text, err := os.ReadFile(filepath.Join(work, "herdr", "screens", name+".txt"))
			if err != nil {
				return map[string]any{"id": request.ID, "error": map[string]any{"code": "agent_not_found", "message": err.Error()}}
			}
			return appSocketResult(request.ID, map[string]any{"type": "pane_read", "read": map[string]any{
				"pane_id": agent["pane_id"], "workspace_id": agent["workspace_id"], "tab_id": agent["tab_id"],
				"source": request.Params["source"], "format": request.Params["format"],
				"text": string(text), "revision": agent["revision"], "truncated": false,
			}})
		case "agent.get":
			target := fmt.Sprint(request.Params["target"])
			_, agents, err := activityFixtureSnapshot(work)
			if err != nil {
				return map[string]any{"id": request.ID, "error": map[string]any{"code": "fixture_error", "message": err.Error()}}
			}
			if agent := activityAgent(agents, target); agent != nil {
				return appSocketResult(request.ID, map[string]any{"type": "agent_info", "agent": agent})
			}
			return map[string]any{"id": request.ID, "error": map[string]any{"code": "agent_not_found", "message": "agent not found"}}
		case "pane.process_info":
			return appSocketResult(request.ID, map[string]any{"type": "pane_process_info", "process_info": map[string]any{
				"pane_id": request.Params["pane_id"], "foreground_process_group_id": 900, "shell_pid": 900,
			}})
		default:
			return map[string]any{"id": request.ID, "error": map[string]any{"code": "unknown_method", "message": request.Method}}
		}
	})
	t.Setenv("HERDR_SOCKET_PATH", path)
	return path
}
func activityFixtureList(path, field string) ([]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, err
	}
	var values []any
	if err := json.Unmarshal(reply.Result[field], &values); err != nil {
		return nil, err
	}
	return values, nil
}
func activityFixtureSnapshot(work string) ([]any, []map[string]any, error) {
	tabs, err := activityFixtureList(filepath.Join(work, "herdr", "tabs.json"), "tabs")
	if err != nil {
		return nil, nil, err
	}
	agentItems, err := activityFixtureList(filepath.Join(work, "herdr", "agents.json"), "agents")
	if err != nil {
		return nil, nil, err
	}
	agents := make([]map[string]any, 0, len(agentItems))
	for _, value := range agentItems {
		agent, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if agent["workspace_id"] == nil {
			agent["workspace_id"] = "w1"
		}
		if agent["focused"] == nil {
			agent["focused"] = false
		}
		if agent["revision"] == nil {
			if agent["state_change_seq"] == nil {
				agent["revision"] = 0
			} else {
				agent["revision"] = agent["state_change_seq"]
			}
		}
		if agent["agent_status"] == nil {
			agent["agent_status"] = "unknown"
		}
		if agent["tab_id"] == nil {
			agent["tab_id"] = ""
		}
		if agent["pane_id"] == nil {
			agent["pane_id"] = ""
		}
		if agent["terminal_id"] == nil {
			agent["terminal_id"] = agent["pane_id"]
		}
		agent["interactive_ready"] = true
		agent["launch_pending"] = false
		agent["screen_detection_skipped"] = false
		agents = append(agents, agent)
	}
	for i, value := range tabs {
		tab, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if tab["workspace_id"] == nil {
			tab["workspace_id"] = "w1"
		}
		if tab["tab_id"] == nil {
			tab["tab_id"] = ""
		}
		if tab["label"] == nil {
			tab["label"] = ""
		}
		if tab["number"] == nil {
			tab["number"] = i + 1
		}
		if tab["focused"] == nil {
			tab["focused"] = false
		}
		if tab["pane_count"] == nil {
			count := 0
			for _, agent := range agents {
				if agent["tab_id"] == tab["tab_id"] {
					count++
				}
			}
			tab["pane_count"] = count
		}
		if tab["agent_status"] == nil {
			tab["agent_status"] = "idle"
		}
	}
	return tabs, agents, nil
}
func activityAgent(agents []map[string]any, target string) map[string]any {
	for _, agent := range agents {
		if agent["name"] == target || agent["pane_id"] == target {
			return agent
		}
	}
	return nil
}

type fixtureCommandPort struct{ path string }
type fixtureCommandError struct{ code, text string }

func (e *fixtureCommandError) Error() string { return e.text }
func fixtureErrorCode(err error) string {
	if e, ok := errors.AsType[*fixtureCommandError](err); ok {
		return e.code
	}
	return "fixture_error"
}
func (p fixtureCommandPort) JSON(args ...string) (map[string]any, error) {
	raw, err := exec.Command(p.path, args...).Output()
	var wire struct {
		Result map[string]any                  `json:"result"`
		Error  *struct{ Code, Message string } `json:"error"`
	}
	if decode := json.Unmarshal(raw, &wire); decode != nil {
		return nil, decode
	}
	if wire.Error != nil {
		return nil, &fixtureCommandError{wire.Error.Code, wire.Error.Message}
	}
	if err != nil {
		return nil, err
	}
	return wire.Result, nil
}
func (p fixtureCommandPort) Text(args ...string) (string, error) {
	raw, err := exec.Command(p.path, args...).Output()
	return string(raw), err
}
