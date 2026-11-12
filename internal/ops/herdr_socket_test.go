package ops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
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

func appSocketResult(id string, result map[string]any) map[string]any {
	return map[string]any{"id": id, "result": result}
}

func TestSocketPathFromStatus(t *testing.T) {
	path, err := socketPathFrom("status: running\nsocket: /tmp/herdr.sock\n")
	if err != nil || path != "/tmp/herdr.sock" {
		t.Fatalf("socketPathFrom = %q, %v", path, err)
	}
	if _, err := socketPathFrom("status: stopped\n"); err == nil {
		t.Fatal("socketPathFrom accepted status without socket path")
	}
}

func TestServeHerdrPortDiscoversSocketFromStatus(t *testing.T) {
	path := startAppHerdrSocket(t, func(request appSocketRequest) map[string]any {
		if request.Method != "session.snapshot" {
			t.Errorf("method = %q, want session.snapshot", request.Method)
		}
		return appSocketResult(request.ID, map[string]any{"type": "session_snapshot", "snapshot": map[string]any{
			"version": "test", "protocol": 22, "workspaces": []any{}, "tabs": []any{}, "panes": []any{}, "layouts": []any{}, "agents": []any{},
		}})
	})
	status := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(status, []byte("#!/bin/sh\nprintf 'status: running\\nsocket: %s\\n' "+fmt.Sprintf("%q", path)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERGY_HERDR", status)
	snapshot, err := serveHerdrPort().SessionSnapshot()
	if err != nil || len(snapshot.Tabs) != 0 {
		t.Fatalf("status-discovered socket read = %+v, %v", snapshot, err)
	}
}

func TestServeHerdrPortRetriesSocketDiscoveryAfterLookupFailure(t *testing.T) {
	path := startAppHerdrSocket(t, func(request appSocketRequest) map[string]any {
		if request.Method != "session.snapshot" {
			t.Errorf("method = %q, want session.snapshot", request.Method)
		}
		return appSocketResult(request.ID, map[string]any{"type": "session_snapshot", "snapshot": map[string]any{
			"version": "test", "protocol": 22, "workspaces": []any{}, "tabs": []any{}, "panes": []any{}, "layouts": []any{}, "agents": []any{},
		}})
	})
	status := filepath.Join(t.TempDir(), "herdr")
	count := filepath.Join(t.TempDir(), "status-count")
	script := "#!/bin/sh\n" +
		"n=0\n" +
		"if [ -f \"$HERDR_STATUS_COUNT\" ]; then n=$(cat \"$HERDR_STATUS_COUNT\"); fi\n" +
		"n=$((n + 1))\n" +
		"printf '%s' \"$n\" > \"$HERDR_STATUS_COUNT\"\n" +
		"if [ \"$n\" -eq 1 ]; then exit 1; fi\n" +
		"printf 'status: running\\nsocket: %s\\n' \"$HERDR_STATUS_SOCKET\"\n"
	if err := os.WriteFile(status, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_STATUS_COUNT", count)
	t.Setenv("HERDR_STATUS_SOCKET", path)
	t.Setenv("HERGY_HERDR", status)

	port := serveHerdrPort()
	if _, err := port.SessionSnapshot(); err == nil {
		t.Fatal("first read succeeded even though herdr status server failed")
	}
	snapshot, err := port.SessionSnapshot()
	if err != nil || len(snapshot.Tabs) != 0 {
		t.Fatalf("second read after failed lookup = %+v, %v", snapshot, err)
	}
	if _, err := port.SessionSnapshot(); err != nil {
		t.Fatalf("read using the resolved socket = %v", err)
	}
	if got, err := os.ReadFile(count); err != nil || string(got) != "2" {
		t.Fatalf("herdr status calls = %q, %v; want 2", got, err)
	}
}

func TestServeHerdrPortRediscoversAfterSocketStopsAcceptingConnections(t *testing.T) {
	path := startAppHerdrSocket(t, func(request appSocketRequest) map[string]any {
		return appSocketResult(request.ID, map[string]any{"type": "session_snapshot", "snapshot": map[string]any{
			"version": "test", "protocol": 22, "workspaces": []any{}, "tabs": []any{}, "panes": []any{}, "layouts": []any{}, "agents": []any{},
		}})
	})
	status := filepath.Join(t.TempDir(), "herdr")
	count := filepath.Join(t.TempDir(), "status-count")
	deadDir, err := os.MkdirTemp("/tmp", "hd-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(deadDir) })
	deadSocket := filepath.Join(deadDir, "gone.sock")
	script := "#!/bin/sh\n" +
		"n=0\n" +
		"if [ -f \"$HERDR_STATUS_COUNT\" ]; then n=$(cat \"$HERDR_STATUS_COUNT\"); fi\n" +
		"n=$((n + 1))\n" +
		"printf '%s' \"$n\" > \"$HERDR_STATUS_COUNT\"\n" +
		"printf 'status: running\\nsocket: %s\\n' \"$HERDR_STATUS_SOCKET\"\n"
	if err := os.WriteFile(status, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", deadSocket)
	t.Setenv("HERDR_STATUS_COUNT", count)
	t.Setenv("HERDR_STATUS_SOCKET", path)
	t.Setenv("HERGY_HERDR", status)

	port := serveHerdrPort()
	if _, err := port.SessionSnapshot(); err == nil {
		t.Fatal("first read succeeded with a socket that does not accept connections")
	}
	snapshot, err := port.SessionSnapshot()
	if err != nil || len(snapshot.Tabs) != 0 {
		t.Fatalf("second read after socket failure = %+v, %v", snapshot, err)
	}
	if got, err := os.ReadFile(count); err != nil || string(got) != "1" {
		t.Fatalf("herdr status calls = %q, %v; want 1 after the inherited path stopped accepting connections", got, err)
	}
}
