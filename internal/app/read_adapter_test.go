package app

import (
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/team"
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
	now    func() time.Time
	rename func(string) error
}

func newActivityRead(env team.Env) *activityRead       { return fixtureReader(env, false) }
func newSocketActivityRead(env team.Env) *activityRead { return fixtureReader(env, true) }
func fixtureReader(env team.Env, useSocket bool) *activityRead {
	a := &activityRead{}
	r := ops.NewReader(ops.ReaderInput{UseSocket: useSocket, Env: env, Now: func() time.Time {
		if a.now != nil {
			return a.now()
		}
		return serveClock()
	}})
	a.fixtureRead = web.FromOps(r)
	a.rename = func(path string) error {
		p, e := domainplace.NewProject(path)
		if e != nil {
			return e
		}
		return r.CheckDefinitionRename(p)
	}
	return a
}

func (a *activityRead) CheckDefinitionRename(path domainplace.Project) error {
	return a.rename(path.String())
}
