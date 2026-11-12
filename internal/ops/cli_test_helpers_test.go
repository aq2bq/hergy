package ops

import (
	"fmt"
	"io"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/options"
)

// These are presentation/input fixtures for component tests, not production paths.
type reportedError struct {
	err  error
	code int
}

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }
func fail(out io.Writer, err error)   { fmt.Fprintln(out, i18n.Tf("error.prefix", err)) }
func label(key string) string         { return i18n.T("label." + key) }
func memberInput(env Environment, opts *options.Options, _, _ io.Writer) MemberInput {
	return MemberInput{Env: env, Team: opts.Team, Member: opts.Member, All: opts.All, Dry: opts.Dry, Timeout: opts.Timeout}
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

func internalChanges(in []Change) []deliver.Change {
	out := make([]deliver.Change, len(in))
	for i, c := range in {
		out[i] = deliver.Change(c)
	}
	return out
}
