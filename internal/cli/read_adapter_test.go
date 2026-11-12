package cli

import (
	"time"

	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/web"
)

// This test adapter lets existing CLI/HTTP integration fixtures inject their
// clock and transport. Production constructs ops.Reader directly in runServe.
type fixtureRead interface {
	web.Activity
	web.HQ
	web.Applier
	web.PromptReader
	web.CatalogProvider
}
type activityRead struct {
	fixtureRead
	now func() time.Time
}

func newActivityRead(env ops.Environment) *activityRead       { return fixtureReader(env, false) }
func newSocketActivityRead(env ops.Environment) *activityRead { return fixtureReader(env, true) }
func fixtureReader(env ops.Environment, useSocket bool) *activityRead {
	a := &activityRead{}
	r := ops.NewReader(ops.ReaderInput{UseSocket: useSocket, Env: env, Now: func() time.Time {
		if a.now != nil {
			return a.now()
		}
		return serveClock()
	}})
	a.fixtureRead = web.FromOps(r)
	return a
}
