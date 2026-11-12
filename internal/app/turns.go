package app

import (
	"context"
	"io"
	"os"

	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
	"golang.org/x/sys/unix"
)

func runTurns(ctx context.Context, env team.Env, opts *options.Options, stdout io.Writer) error {
	return ops.WatchTurns(ctx, ops.WatchInput{Env: env, Team: opts.Team, Log: opts.Log, Width: terminalWidth, Observe: func(e ops.Event) { _, _ = io.WriteString(stdout, e.Text) }})
}
func terminalWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 100
	}
	return int(ws.Col)
}
