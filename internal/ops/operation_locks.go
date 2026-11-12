package ops

import (
	"context"
	"sync"

	"github.com/aq2bq/hergy/internal/i18n"
)

// OperationLocks reserves all teams of a screen operation atomically.
// Its zero value is ready to use; release and cancellation are separate actions.
type OperationLocks struct {
	mu    sync.Mutex
	teams map[string]*teamOperation
}
type teamOperation struct{ cancel context.CancelFunc }

func (l *OperationLocks) acquire(ctx context.Context, names []string) (context.Context, func(), error) {
	if l == nil {
		return ctx, func() {}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.teams == nil {
		l.teams = map[string]*teamOperation{}
	}
	for _, name := range names {
		if l.teams[name] != nil {
			return nil, nil, refuse("conflict", i18n.Errorf("apply.refused_busy", name))
		}
	}
	child, cancel := context.WithCancel(ctx)
	owner := &teamOperation{cancel: cancel}
	for _, name := range names {
		l.teams[name] = owner
	}
	return child, func() {
		cancel()
		l.mu.Lock()
		defer l.mu.Unlock()
		for _, name := range names {
			if l.teams[name] == owner {
				delete(l.teams, name)
			}
		}
	}, nil
}

// CancelTeam cancels the entire operation that includes this team.
func (l *OperationLocks) CancelTeam(name string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	owner := l.teams[name]
	l.mu.Unlock()
	if owner != nil {
		owner.cancel()
	}
}
