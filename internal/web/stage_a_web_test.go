package web

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureNaming(s string) domainname.Naming {
	d, _ := domainname.ParseDefinitionName(s)
	return domainname.DefinitionNames(d)
}

func fixtureSeen(s string) domainname.Seen {
	var n domainname.Seen
	_ = n.UnmarshalText([]byte(s))
	return n
}
func fixtureTeam(s string) domainname.TeamRef { return domainname.TeamFromObservation(fixtureSeen(s)) }
