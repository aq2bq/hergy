package ops

import (
	"net/http"
	"path/filepath"
	"slices"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
)

// CheckDefinitionRename reads the entire connected session, before the screen's
// project filter. It is called under the same file locks that protect up.
func (a *Reader) CheckDefinitionRename(target domainplace.Project) error {

	read := a.readHerdr(time.Now())
	if read.err != nil {
		return hqRefuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	records, err := readLaunchRecords()
	if err != nil {
		return hqRefuse(http.StatusServiceUnavailable, i18n.T("web.rename.unavailable"))
	}
	for _, running := range team.RunningTeams(read.tabs, launchTeams(), read.agentsRead) {
		record, known := renameLaunch(records, running)
		if !known || record.TeamYAML.String() == "" {
			if running.Definition == filepath.Base(filepath.Dir(target.String())) {
				return hqRefuse(http.StatusConflict, i18n.Tf("web.rename.unknown", running.Team.String()))
			}
			continue
		}
		if record.TeamYAML.Project().Compare(target) == domainplace.Same {
			return hqRefuse(http.StatusConflict, i18n.Tf("web.rename.running", running.Team.String()))
		}
	}
	return nil
}

func renameLaunch(records []launchlog.Record, running team.Running) (launchlog.Record, bool) {
	for _, one := range slices.Backward(records) {
		if !one.IsHire() && one.Team.String() == running.Team.String() && (running.Project.IsZero() || one.Project.Project().Compare(running.Project) == domainplace.Same) {
			return one, true
		}
	}
	return launchlog.Record{}, false
}
