package herdr

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aq2bq/hergy/internal/protocol"
	"go.uber.org/goleak"
)

type socketRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type socketTestCommandPort struct {
	verify []string
	json   [][]string
	text   [][]string
}

func (p *socketTestCommandPort) Verify(mode string) error {
	p.verify = append(p.verify, mode)
	return nil
}

func (p *socketTestCommandPort) JSON(args ...string) (map[string]any, error) {
	p.json = append(p.json, append([]string(nil), args...))
	return map[string]any{"write": true}, nil
}

func (p *socketTestCommandPort) Text(args ...string) (string, error) {
	p.text = append(p.text, append([]string(nil), args...))
	return "command text", nil
}

func fakeSocketServer(t *testing.T, handle func(socketRequest) map[string]any) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "hs-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var closeOnce sync.Once
	closeServer := func() {
		closeOnce.Do(func() { _ = listener.Close() })
		<-done
	}
	t.Cleanup(closeServer)
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			serveSocketRequest(conn, handle)
		}
	}()
	return path
}

func serveSocketRequest(conn net.Conn, handle func(socketRequest) map[string]any) {
	defer func() { _ = conn.Close() }()
	var request socketRequest
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&request); err != nil {
		return
	}
	response := handle(request)
	if _, ok := response["id"]; !ok {
		response["id"] = request.ID
	}
	_ = json.NewEncoder(conn).Encode(response)
}

func TestSocketPortReadsSnapshotAndIndividualMethods(t *testing.T) {
	var mu sync.Mutex
	var requests []socketRequest
	path := fakeSocketServer(t, func(request socketRequest) map[string]any {
		mu.Lock()
		requests = append(requests, request)
		mu.Unlock()
		switch request.Method {
		case "session.snapshot":
			return map[string]any{"result": map[string]any{"type": "session_snapshot", "snapshot": map[string]any{
				"version": "test", "protocol": 22, "layouts": []any{},
				"workspaces": []any{map[string]any{"workspace_id": "w1", "number": 1, "label": "main", "focused": true, "pane_count": 1, "tab_count": 1, "active_tab_id": "t1", "agent_status": "idle"}},
				"tabs":       []any{map[string]any{"tab_id": "t1", "label": "core", "workspace_id": "w1", "number": 1, "focused": true, "pane_count": 1, "agent_status": "idle"}},
				"panes":      []any{map[string]any{"pane_id": "p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "t1", "label": "core-lead", "cwd": "/project", "focused": true, "agent_status": "idle", "revision": 17}},
				"agents":     []any{map[string]any{"name": "core-lead", "agent": "claude", "agent_status": "idle", "pane_id": "p1", "tab_id": "t1", "workspace_id": "w1", "terminal_id": "term1", "focused": true, "revision": 17, "state_change_seq": 17, "terminal_title_stripped": "ready", "interactive_ready": true, "launch_pending": false, "screen_detection_skipped": false}},
			}}}
		case "agent.get":
			return map[string]any{"result": map[string]any{"type": "agent_info", "agent": map[string]any{
				"name": request.Params["target"], "agent": "claude", "agent_status": "idle", "pane_id": "p1", "tab_id": "t1", "workspace_id": "w1", "terminal_id": "term1", "focused": false, "revision": 17,
			}}}
		case "agent.read":
			return map[string]any{"result": map[string]any{"type": "pane_read", "read": map[string]any{
				"pane_id": "p1", "workspace_id": "w1", "tab_id": "t1", "source": "detection", "format": "text", "text": "waiting", "revision": 17, "truncated": false,
			}}}
		case "pane.process_info":
			return map[string]any{"result": map[string]any{"type": "pane_process_info", "process_info": map[string]any{"pane_id": request.Params["pane_id"], "shell_pid": 9}}}
		default:
			return map[string]any{"error": map[string]any{"code": "unknown_method", "message": request.Method}}
		}
	})
	port := NewSocketPort(&socketTestCommandPort{}, path, nil)

	snapshot, err := port.SessionSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0] != (Workspace{ID: "w1", Label: "main", Focused: true}) {
		t.Errorf("workspaces = %+v", snapshot.Workspaces)
	}
	if len(snapshot.Tabs) != 1 || snapshot.Tabs[0] != (protocol.Tab{ID: "t1", Label: "core", WorkspaceID: "w1"}) {
		t.Errorf("tabs = %+v", snapshot.Tabs)
	}
	if len(snapshot.Panes) != 1 || snapshot.Panes[0] != (protocol.Pane{ID: "p1", Label: "core-lead", TabID: "t1", Cwd: "/project"}) {
		t.Errorf("panes = %+v", snapshot.Panes)
	}
	if len(snapshot.Agents) != 1 || snapshot.Agents[0].Name != "core-lead" || snapshot.Agents[0].Seq != 17 || snapshot.Agents[0].Title != "ready" {
		t.Errorf("agents = %+v", snapshot.Agents)
	}
	if _, err := port.JSON("agent", "get", "core-lead"); err != nil {
		t.Fatal(err)
	}
	if got, err := port.Text("agent", "read", "core-lead", "--source", "detection"); err != nil || got != "waiting" {
		t.Errorf("agent.read = %q, %v", got, err)
	}
	processInfo, err := port.JSON("pane", "process-info", "--pane", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := processInfo["process_info"].(map[string]any); got["shell_pid"] != float64(9) {
		t.Errorf("pane.process_info result = %+v", processInfo)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 4 {
		t.Fatalf("socket requests = %d, want snapshot and three individual reads: %+v", len(requests), requests)
	}
	if requests[0].Method != "session.snapshot" || len(requests[0].Params) != 0 {
		t.Errorf("session.snapshot request = %+v", requests[0])
	}
	if requests[1].Method != "agent.get" || !reflect.DeepEqual(requests[1].Params, map[string]any{"target": "core-lead"}) {
		t.Errorf("agent.get request = %+v", requests[1])
	}
	if requests[2].Method != "agent.read" || requests[2].Params["source"] != "detection" || requests[2].Params["format"] != "text" || requests[2].Params["strip_ansi"] != true {
		t.Errorf("agent.read request = %+v", requests[2])
	}
	if requests[3].Method != "pane.process_info" || !reflect.DeepEqual(requests[3].Params, map[string]any{"pane_id": "p1"}) {
		t.Errorf("pane.process_info request = %+v", requests[3])
	}
}

func TestSocketPortKeepsHerdrErrorsAndDelegatesWrites(t *testing.T) {
	path := fakeSocketServer(t, func(socketRequest) map[string]any {
		return map[string]any{"error": map[string]any{"code": "agent_not_found", "message": "missing"}}
	})
	command := &socketTestCommandPort{}
	port := NewSocketPort(command, path, nil)
	if _, err := port.JSON("agent", "get", "missing"); err == nil || protocol.ErrorCode(err) != "agent_not_found" {
		t.Fatalf("agent.get error = %v, code %q", err, protocol.ErrorCode(err))
	}
	if _, err := port.JSON("agent", "prompt", "core-lead", "text"); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Text("agent", "start", "core-lead"); err != nil {
		t.Fatal(err)
	}
	if err := port.Verify("boot"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(command.json, [][]string{{"agent", "prompt", "core-lead", "text"}}) {
		t.Errorf("CLI JSON calls = %v", command.json)
	}
	if !reflect.DeepEqual(command.text, [][]string{{"agent", "start", "core-lead"}}) {
		t.Errorf("CLI text calls = %v", command.text)
	}
	if !reflect.DeepEqual(command.verify, []string{"boot"}) {
		t.Errorf("CLI verification calls = %v", command.verify)
	}
}

// TestSocketPortReadsAgentReadForEveryFlagShape: agent read goes over the
// socket as long as the target is there, and --source replaces the detection
// default wherever it stands. Other flags, or --source without a value, leave
// the default alone.
func TestSocketPortReadsAgentReadForEveryFlagShape(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		source string
	}{
		{"target only", []string{"agent", "read", "core-lead"}, "detection"},
		{"other flag", []string{"agent", "read", "core-lead", "--format", "text"}, "detection"},
		{"source flag", []string{"agent", "read", "core-lead", "--source", "buffer"}, "buffer"},
		{"source flag after another flag", []string{"agent", "read", "core-lead", "--format", "text", "--source", "buffer"}, "buffer"},
		{"source flag without a value", []string{"agent", "read", "core-lead", "--source"}, "detection"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var mu sync.Mutex
			var sources []any
			path := fakeSocketServer(t, func(request socketRequest) map[string]any {
				mu.Lock()
				sources = append(sources, request.Params["source"])
				mu.Unlock()
				return map[string]any{"result": map[string]any{"type": "pane_read", "read": map[string]any{"text": "waiting"}}}
			})
			command := &socketTestCommandPort{}
			port := NewSocketPort(command, path, nil)

			got, err := port.Text(c.args...)
			if err != nil || got != "waiting" {
				t.Fatalf("Text(%q) = %q, %v", c.args, got, err)
			}
			if len(command.text) != 0 {
				t.Fatalf("Text(%q) went to the command port: %v", c.args, command.text)
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(sources, []any{c.source}) {
				t.Fatalf("sources = %v, want [%s]", sources, c.source)
			}
		})
	}
}

// TestSocketPortFixedPathWithoutDiscoveryFails: a fixed-path port whose lookup
// failed reports that failure, even when the path it was given would answer.
func TestSocketPortFixedPathWithoutDiscoveryFails(t *testing.T) {
	path := fakeSocketServer(t, func(socketRequest) map[string]any {
		return map[string]any{"result": map[string]any{"type": "agent_info", "agent": map[string]any{"name": "core-lead"}}}
	})
	port := NewSocketPort(&socketTestCommandPort{}, path, errors.New("discovery failed"))

	if _, err := port.JSON("agent", "get", "core-lead"); err == nil || !strings.Contains(err.Error(), "discovery failed") {
		t.Fatalf("agent.get error = %v, want the discovery failure", err)
	}
}

func TestSocketRejectsMismatchedResponseID(t *testing.T) {
	path := fakeSocketServer(t, func(socketRequest) map[string]any {
		return map[string]any{"id": "wrong", "result": map[string]any{}}
	})
	if _, err := NewSocket(path).Call("session.snapshot", map[string]any{}); err == nil || !strings.Contains(err.Error(), "does not match request") {
		t.Fatalf("mismatched response error = %v", err)
	}
}
