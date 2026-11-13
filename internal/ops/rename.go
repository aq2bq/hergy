package ops

import (
	"net/http"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
)

// CheckDefinitionRename reads the entire connected session, before the screen's
// project filter. It is called under the same file locks that protect up.
func (a *Reader) CheckDefinitionRename(target domainplace.Project) error {

	read := a.readHerdr(time.Now())
	if read.err != nil {
		return hqRefuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	store, err := runningstate.Open()
	if err != nil {
		return hqRefuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	states, err := store.List()
	if err != nil {
		return hqRefuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	for _, running := range team.RunningTeams(read.tabs, states) {
		if running.State.Launch().TeamYAML.Compare(target) == domainplace.Same {
			return hqRefuse(http.StatusConflict, i18n.Tf("web.rename.running", running.Team.String()))
		}
	}
	return nil
}
