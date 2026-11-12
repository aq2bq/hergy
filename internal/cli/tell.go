package cli

import (
	"context"
	"io"

	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/options"
)

func runTell(env ops.Environment, opts *options.Options, stdout io.Writer) error {
	_, err := ops.Tell(context.Background(), ops.TellInput{MemberInput: ops.MemberInput{Env: env, Team: opts.Team, Member: opts.Member, Dry: opts.Dry, Timeout: opts.Timeout, Observe: func(e ops.Event) { _, _ = io.WriteString(stdout, e.Text) }}, From: opts.From, Body: opts.Body})
	return err
}
