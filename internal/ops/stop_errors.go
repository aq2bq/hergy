package ops

import (
	"errors"

	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
)

// NameReleaseTimeout means pane closure reached the deadline for freeing names.
type NameReleaseTimeout struct{ err error }

func (e *NameReleaseTimeout) Error() string { return e.err.Error() }
func (e *NameReleaseTimeout) Unwrap() error { return e.err }

func stopFailure(err error) error {
	if _, ok := errors.AsType[*launcher.TeamDownTimeout](err); ok {
		return &NameReleaseTimeout{err: err}
	}
	return err
}
