package ops

import (
	"io"
	"os"

	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/launcher"
)

func NewLauncher(r *Resolved, timeout int, out io.Writer) *launcher.Launcher {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return NewLauncherWithPort(r, timeout, out, port)
}

func NewLauncherWithPort(r *Resolved, timeout int, out io.Writer, port herdr.Port) *launcher.Launcher {
	return launcher.New(r.Starts, r.Team.Name.Observation(), r.Project, herdr.New(port), timeout, out)
}
