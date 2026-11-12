package launchlog

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
)

func fixtureSeen(s string) domainname.Seen { return observedname.Observe(s) }
