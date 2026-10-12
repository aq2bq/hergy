// Package herdrcli runs the herdr binary. It is the only place that starts a
// process, so a fake herdr can be substituted with HERGY_HERDR.
package herdrcli

import (
	"bytes"
	"errors"
	"os/exec"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// CLI runs one executable. Arguments go to it as an argv array, never through a shell.
type CLI struct{ Bin string }

// New defaults to `herdr`, or to the HERGY_HERDR override.
func New(bin string) CLI {
	if bin == "" {
		bin = "herdr"
	}
	return CLI{Bin: bin}
}

// Verify checks the installed commands advertise what the operation needs.
func (c CLI) Verify(mode string) error {
	for _, group := range protocol.RequiredCommands(mode) {
		stdout, stderr, status, err := c.capture(group.Name)
		if err != nil {
			return err
		}
		if err := protocol.CheckHelp(group.Name, group.Subs, stdout+stderr, status); err != nil {
			return err
		}
	}
	return nil
}

// JSON runs one herdr command and returns its result object.
func (c CLI) JSON(args ...string) (map[string]any, error) {
	stdout, stderr, status, err := c.capture(args...)
	if err != nil {
		return nil, err
	}
	return protocol.Result(args, stdout, stderr, status)
}

func (c CLI) capture(args ...string) (stdout, stderr string, status int, err error) {
	cmd := exec.Command(c.Bin, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if runErr != nil {
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			return out.String(), errBuf.String(), exit.ExitCode(), nil
		}
		return out.String(), errBuf.String(), -1, i18n.Errorf("error.cannot_execute", c.Bin, runErr)
	}
	return out.String(), errBuf.String(), 0, nil
}
