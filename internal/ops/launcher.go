package ops

import (
	"io"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
)

// newLauncher assembles the launcher used by operations still migrating from app.

// newLauncherWithPort uses a caller-supplied herdr transport.
func newLauncherWithPort(r *Resolved, timeout int, out io.Writer, port herdr.Port) *launcher.Launcher {
	return launcher.New(r.Starts, r.Team.Name.Observation(), r.Project, herdr.New(port), timeout, out)
}
