package ops

import (
	"context"
	"time"

	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
)

// stopPort stops between herdr calls when the CLI is interrupted. This keeps
// cancellation in the operation without a detached closing goroutine.
type stopPort struct {
	herdr.Port
	ctx context.Context
}

func (p stopPort) Verify(mode string) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	return p.Port.Verify(mode)
}

func (p stopPort) JSON(args ...string) (map[string]any, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	return p.Port.JSON(args...)
}

func (p stopPort) Text(args ...string) (string, error) {
	if err := p.ctx.Err(); err != nil {
		return "", err
	}
	return p.Port.Text(args...)
}

func stopPause(ctx context.Context, pause func(time.Duration)) func(time.Duration) {
	if pause != nil {
		return pause
	}
	return func(step time.Duration) {
		timer := time.NewTimer(step)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	}
}
