package herdr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const socketTimeout = time.Second

// Socket talks to Herdr's local JSON API. Each call has one newline-delimited
// request and response on a fresh Unix-domain connection.
type Socket struct {
	path string
	seq  atomic.Uint64
}

// NewSocket connects to the Herdr socket at path.
func NewSocket(path string) *Socket { return &Socket{path: path} }

// Call sends one read request and returns the response object, including its
// result or error. The API's schema is printed by `herdr api schema --json`.
func (s *Socket) Call(method string, params map[string]any) (map[string]any, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, fmt.Errorf("herdr socket path is empty")
	}
	conn, err := net.DialTimeout("unix", s.path, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("herdr socket: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(socketTimeout)); err != nil {
		return nil, fmt.Errorf("herdr socket: %w", err)
	}
	id := strconv.FormatUint(s.seq.Add(1), 10)
	request := struct {
		ID     string         `json:"id"`
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}{ID: id, Method: method, Params: params}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return nil, fmt.Errorf("herdr socket: %w", err)
	}
	line, err := bufio.NewReader(io.LimitReader(conn, 8<<20)).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("herdr socket: %w", err)
	}
	var response map[string]any
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, fmt.Errorf("herdr socket returned invalid JSON: %w", err)
	}
	if got, ok := response["id"].(string); !ok || got != id {
		return nil, fmt.Errorf("herdr socket response id %v does not match request %s", response["id"], id)
	}
	return response, nil
}
