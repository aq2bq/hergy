package app

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops"
)

func lastPromptRecord(records []launchlog.Record, project, target domainplace.Project, definition, team, member string) (launchlog.Record, launchlog.Member, bool) {
	return ops.LastPromptRecord(records, project, target, definition, team, member)
}
