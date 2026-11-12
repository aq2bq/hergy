package app

import (
	"io"

	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops"
)

func launcherFor(r *ops.Resolved, timeout int, out io.Writer) *launcher.Launcher {
	_, _ = io.WriteString(out, ops.PluginNote())
	return ops.NewLauncher(r, timeout, out)
}
