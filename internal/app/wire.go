package app

import (
	"io"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops"
)

func printSignalNotes(out io.Writer, starts []definition.Start) {
	for _, note := range ops.SignalNotes(starts) {
		_, _ = io.WriteString(out, note)
	}
}

func launcherFor(r *ops.Resolved, timeout int, out io.Writer) *launcher.Launcher {
	_, _ = io.WriteString(out, ops.PluginNote())
	return ops.NewLauncher(r, timeout, out)
}
func launcherWithPort(r *ops.Resolved, timeout int, out io.Writer, port herdr.Port) *launcher.Launcher {
	_, _ = io.WriteString(out, ops.PluginNote())
	return ops.NewLauncherWithPort(r, timeout, out, port)
}
