package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
)

// readLaunchRecords reads the machine's history, oldest first.
func readLaunchRecords() ([]launchlog.Record, error) {
	log, err := launchlog.Open()
	if err != nil {
		return nil, err
	}
	return log.Read()
}

// latestLaunch reads the newest workspace-bearing up of an exact team.yaml path.
func latestLaunch(path domainplace.Project) (launchlog.Record, bool, error) {
	log, err := launchlog.Open()
	if err != nil {
		return launchlog.Record{}, false, err
	}
	return log.Latest(path)
}
