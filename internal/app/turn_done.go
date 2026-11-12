package app

import (
	"io"

	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/options"
)

func runTurnDone(opts *options.Options, stdin io.Reader) error {
	var raw []byte
	if opts.Body == "" {
		var err error
		raw, err = io.ReadAll(stdin)
		if err != nil {
			return err
		}
	}
	return ops.TurnDone(ops.TurnDoneInput{Team: opts.Team.String(), Member: opts.Member, Payload: opts.Body, Raw: raw})
}
