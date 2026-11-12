package ops

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
)

func fixtureMemberName(s string) domainname.MemberName {
	n, e := domainname.ParseMemberName(s)
	if e != nil {
		panic(e)
	}
	return n
}

func fixtureRunningTeam(s string) domainname.TeamRef {
	return domainname.TeamFromObservation(fixtureSeen(s))
}

func fixtureSeen(s string) domainname.Seen { return observedname.Observe(s) }

func fixtureDemoTeam() domainname.TeamName {
	n, e := domainname.ParseTeamName("demo")
	if e != nil {
		panic(e)
	}
	return n
}

func fixtureSeenList(ss []string) []domainname.Seen {
	out := make([]domainname.Seen, len(ss))
	for i, s := range ss {
		out[i] = fixtureSeen(s)
	}
	return out
}

func fixturePane(s string) domainname.PaneID { return domainname.PaneID(s) }
