package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
)

// serveHerdrPort resolves the socket once for this serve. Herdr-managed panes
// pass HERDR_SOCKET_PATH; a serve started elsewhere asks Herdr for its socket.
func serveHerdrPort() *herdr.SocketPort {
	command := herdrcli.New(os.Getenv("HERGY_HERDR"))
	path := strings.TrimSpace(os.Getenv("HERDR_SOCKET_PATH"))
	if path != "" {
		return herdr.NewSocketPort(command, path, nil)
	}
	status, err := command.Text("status", "server")
	if err != nil {
		return herdr.NewSocketPort(command, "", err)
	}
	path, err = socketPathFrom(status)
	return herdr.NewSocketPort(command, path, err)
}

func socketPathFrom(status string) (string, error) {
	for line := range strings.SplitSeq(status, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "socket" {
			path := strings.TrimSpace(value)
			if path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("herdr status server did not report a socket path")
}
