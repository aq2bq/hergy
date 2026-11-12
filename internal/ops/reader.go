package ops

import (
	"context"
	"time"

	"github.com/aq2bq/hergy/internal/source"
)

// ReaderInput supplies one serve lifetime, its sources and clock.
type ReaderInput struct {
	UseSocket bool
	Env       Environment
	Source    source.Reader
	Now       func() time.Time
	Context   context.Context
	Port      Port
}

// NewReader builds the shared project, activity and operation reading.
func NewReader(in ReaderInput) *Reader {
	reader := in.Source
	if reader == nil {
		reader = source.Disk{}
	}
	r := newActivityReadWith(in.Env, reader)
	r.now = in.Now
	r.ctx = in.Context
	r.port = in.Port
	if r.port == nil && in.UseSocket {
		r.port = serveHerdrPort()
	}
	return r
}
