package team

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureRunningTeam(s string) domainname.TeamRef {
	return domainname.TeamFromObservation(domainname.Observe(s))
}
