package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/launchlog"
)

// ReadLaunchRecords reads the machine's history, oldest first.
func ReadLaunchRecords() ([]launchlog.Record, error) {
	log, err := launchlog.Open()
	if err != nil {
		return nil, err
	}
	return log.Read()
}

// LatestLaunch reads the newest launch of an exact team.yaml path.
func LatestLaunch(path domainplace.Project) (launchlog.Record, bool, error) {
	log, err := launchlog.Open()
	if err != nil {
		return launchlog.Record{}, false, err
	}
	return log.Latest(path)
}

// RecordedPromptText reads a previously recorded prompt body.
func RecordedPromptText(hash string) ([]byte, error) {
	log, err := launchlog.Open()
	if err != nil {
		return nil, err
	}
	return log.Text(hash)
}
