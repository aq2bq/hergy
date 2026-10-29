package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
)

// serveHerdrPort resolves the socket on the first read. A failed lookup is
// retried on the next read, as is a path that stops accepting connections.
func serveHerdrPort() *herdr.SocketPort {
	command := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.NewResolvingSocketPort(command, func(forceStatus bool) (string, error) {
		if !forceStatus {
			path := strings.TrimSpace(os.Getenv("HERDR_SOCKET_PATH"))
			if path != "" {
				return path, nil
			}
		}
		status, err := command.Text("status", "server")
		if err != nil {
			return "", err
		}
		return socketPathFrom(status)
	})
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
