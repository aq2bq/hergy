package herdr

import (
	"encoding/json"
	"sync"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// SessionSnapshot is Herdr's workspaces, tabs, panes and agents from one
// session.snapshot request.
type SessionSnapshot struct {
	Workspaces []Workspace
	Tabs       []protocol.Tab
	Panes      []protocol.Pane
	Agents     []protocol.Agent
}

// SnapshotReader is implemented by ports that can read the whole session in
// one operation.
type SnapshotReader interface {
	SessionSnapshot() (SessionSnapshot, error)
}

// SocketPort sends Herdr reads over its socket and leaves writes on the CLI.
// lookupErr is used by the fixed-path constructor when discovery failed.
type SocketPort struct {
	command     Port
	mu          sync.Mutex
	socket      *Socket
	lookupErr   error
	resolve     func(forceStatus bool) (string, error)
	forceStatus bool
}

// NewSocketPort builds a fixed-path Herdr port. command remains the path for
// writes and command verification; read calls use path.
func NewSocketPort(command Port, path string, lookupErr error) *SocketPort {
	return &SocketPort{command: command, socket: NewSocket(path), lookupErr: lookupErr}
}

// NewResolvingSocketPort looks up its socket path on the first read and again
// after a failed socket call. A path that keeps working is reused. forceStatus
// tells resolve to skip any inherited path and ask Herdr for its current socket.
func NewResolvingSocketPort(command Port, resolve func(forceStatus bool) (string, error)) *SocketPort {
	return &SocketPort{command: command, resolve: resolve}
}

// Verify checks the command groups used for writes.
func (p *SocketPort) Verify(mode string) error { return p.command.Verify(mode) }

// JSON sends the Herdr reads that have CLI-shaped counterparts through the
// socket. Every other operation stays on the command port.
func (p *SocketPort) JSON(args ...string) (map[string]any, error) {
	switch {
	case len(args) == 3 && args[0] == "agent" && args[1] == "get":
		result, err := p.result("agent.get", map[string]any{"target": args[2]}, args)
		if err != nil {
			return nil, err
		}
		return nestedResult(result, "agent")
	case len(args) == 4 && args[0] == "pane" && args[1] == "process-info" && args[2] == "--pane":
		result, err := p.result("pane.process_info", map[string]any{"pane_id": args[3]}, args)
		if err != nil {
			return nil, err
		}
		return nestedResult(result, "process_info")
	case len(args) == 2 && args[1] == "list":
		if field, ok := snapshotListField(args[0]); ok {
			snapshot, err := p.SessionSnapshot()
			if err != nil {
				return nil, err
			}
			return map[string]any{field: snapshotList(snapshot, args[0])}, nil
		}
	}
	return p.command.JSON(args...)
}

// Text sends agent.read through the socket. The remaining text operations stay
// on the command port.
func (p *SocketPort) Text(args ...string) (string, error) {
	if len(args) >= 3 && args[0] == "agent" && args[1] == "read" {
		source := "detection"
		for i := 3; i+1 < len(args); i++ {
			if args[i] == "--source" {
				source = args[i+1]
				break
			}
		}
		result, err := p.result("agent.read", map[string]any{
			"target": args[2], "source": source, "format": "text", "strip_ansi": true,
		}, args)
		if err != nil {
			return "", err
		}
		read, ok := result["read"].(map[string]any)
		if !ok {
			return "", i18n.Errorf("error.herdr_missing", "read")
		}
		text, ok := read["text"].(string)
		if !ok {
			return "", i18n.Errorf("error.herdr_not_string", "text")
		}
		return text, nil
	}
	return p.command.Text(args...)
}

// SessionSnapshot reads the complete session in one socket request.
func (p *SocketPort) SessionSnapshot() (SessionSnapshot, error) {
	result, err := p.result("session.snapshot", map[string]any{}, []string{"session", "snapshot"})
	if err != nil {
		return SessionSnapshot{}, err
	}
	typeName, ok := result["type"].(string)
	if !ok || typeName != "session_snapshot" {
		return SessionSnapshot{}, i18n.Errorf("error.herdr_no_result")
	}
	snapshot, ok := result["snapshot"].(map[string]any)
	if !ok {
		return SessionSnapshot{}, i18n.Errorf("error.herdr_missing", "snapshot")
	}
	workspaces, err := snapshotObjects(snapshot, "workspaces")
	if err != nil {
		return SessionSnapshot{}, err
	}
	tabs, err := snapshotObjects(snapshot, "tabs")
	if err != nil {
		return SessionSnapshot{}, err
	}
	panes, err := snapshotObjects(snapshot, "panes")
	if err != nil {
		return SessionSnapshot{}, err
	}
	agents, err := snapshotObjects(snapshot, "agents")
	if err != nil {
		return SessionSnapshot{}, err
	}
	out := SessionSnapshot{
		Workspaces: make([]Workspace, 0, len(workspaces)),
		Tabs:       make([]protocol.Tab, 0, len(tabs)),
		Panes:      make([]protocol.Pane, 0, len(panes)),
		Agents:     make([]protocol.Agent, 0, len(agents)),
	}
	for _, object := range workspaces {
		out.Workspaces = append(out.Workspaces, workspaceFrom(object))
	}
	for _, object := range tabs {
		out.Tabs = append(out.Tabs, protocol.TabFrom(object))
	}
	for _, object := range panes {
		out.Panes = append(out.Panes, protocol.PaneFrom(object))
	}
	for _, object := range agents {
		out.Agents = append(out.Agents, protocol.AgentFrom(object))
	}
	return out, nil
}

func (p *SocketPort) result(method string, params map[string]any, args []string) (map[string]any, error) {
	socket, err := p.socketForRead()
	if err != nil {
		return nil, i18n.Errorf("error.herdr_socket", err)
	}
	response, err := socket.Call(method, params)
	if err != nil {
		p.forgetSocket(socket)
		return nil, i18n.Errorf("error.herdr_socket", err)
	}
	if failure, ok := response["error"]; ok && failure != nil {
		data, marshalErr := json.Marshal(map[string]any{"error": failure})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return protocol.Result(args, string(data), "", 0)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_no_result")
	}
	return result, nil
}

func (p *SocketPort) socketForRead() (*Socket, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolve == nil {
		if p.lookupErr != nil {
			return nil, p.lookupErr
		}
		return p.socket, nil
	}
	if p.socket != nil {
		return p.socket, nil
	}
	path, err := p.resolve(p.forceStatus)
	if err != nil {
		p.forceStatus = true
		return nil, err
	}
	p.socket = NewSocket(path)
	p.forceStatus = false
	return p.socket, nil
}

func (p *SocketPort) forgetSocket(socket *Socket) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolve != nil && p.socket == socket {
		p.socket = nil
		p.forceStatus = true
	}
}

func nestedResult(result map[string]any, field string) (map[string]any, error) {
	value, ok := result[field].(map[string]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_missing", field)
	}
	return map[string]any{field: value}, nil
}

func snapshotListField(group string) (string, bool) {
	switch group {
	case "workspace":
		return "workspaces", true
	case "tab":
		return "tabs", true
	case "pane":
		return "panes", true
	case "agent":
		return "agents", true
	default:
		return "", false
	}
}

func snapshotList(snapshot SessionSnapshot, group string) []any {
	switch group {
	case "workspace":
		out := make([]any, 0, len(snapshot.Workspaces))
		for _, one := range snapshot.Workspaces {
			out = append(out, map[string]any{"workspace_id": one.ID, "label": one.Label, "focused": one.Focused})
		}
		return out
	case "tab":
		out := make([]any, 0, len(snapshot.Tabs))
		for _, one := range snapshot.Tabs {
			out = append(out, map[string]any{"tab_id": one.ID, "label": one.Label, "workspace_id": one.WorkspaceID})
		}
		return out
	case "pane":
		out := make([]any, 0, len(snapshot.Panes))
		for _, one := range snapshot.Panes {
			out = append(out, map[string]any{"pane_id": one.ID, "label": one.Label, "tab_id": one.TabID, "cwd": one.Cwd})
		}
		return out
	case "agent":
		out := make([]any, 0, len(snapshot.Agents))
		for _, one := range snapshot.Agents {
			out = append(out, map[string]any{
				"name": one.Name, "agent": one.Kind, "agent_status": one.Status,
				"pane_id": one.PaneID, "tab_id": one.TabID,
				"state_change_seq": one.Seq, "terminal_title_stripped": one.Title,
			})
		}
		return out
	default:
		return []any{}
	}
}

func snapshotObjects(snapshot map[string]any, field string) ([]map[string]any, error) {
	values, ok := snapshot[field].([]any)
	if !ok {
		return nil, i18n.Errorf("error.herdr_missing", field)
	}
	objects := make([]map[string]any, 0, len(values))
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		objects = append(objects, object)
	}
	return objects, nil
}
