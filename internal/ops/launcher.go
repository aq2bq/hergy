package ops

import (
	"io"
	"os"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/launcher"
)

// NewLauncher assembles the launcher used by operations still migrating from app.
func NewLauncher(r *Resolved, timeout int, out io.Writer) *launcher.Launcher {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return NewLauncherWithPort(r, timeout, out, port)
}

// NewLauncherWithPort uses a caller-supplied herdr transport.
func NewLauncherWithPort(r *Resolved, timeout int, out io.Writer, port herdr.Port) *launcher.Launcher {
	return launcher.New(r.Starts, r.Team.Name.Observation(), r.Project, herdr.New(port), timeout, out)
}
