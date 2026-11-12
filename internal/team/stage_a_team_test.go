package team

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureRunningTeam(s string) domainname.TeamRef {
	return domainname.TeamFromObservation(fixtureSeen(s))
}

func fixtureSeen(s string) domainname.Seen {
	var n domainname.Seen
	_ = n.UnmarshalText([]byte(s))
	return n
}
func fixtureSeenList(ss []string) []domainname.Seen {
	out := make([]domainname.Seen, len(ss))
	for i, s := range ss {
		out[i] = fixtureSeen(s)
	}
	return out
}

func fixtureSpellings(ns []domainname.Seen) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.String()
	}
	return out
}
