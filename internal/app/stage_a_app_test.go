package app

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureMemberName(s string) domainname.MemberName {
	n, e := domainname.ParseMemberName(s)
	if e != nil {
		panic(e)
	}
	return n
}
func fixtureDefinitionName(s string) domainname.DefinitionName {
	n, _ := domainname.ParseDefinitionName(s)
	return n
}
func fixtureHerdrName(s string) domainname.HerdrName {
	var n domainname.HerdrName
	if e := n.UnmarshalText([]byte(s)); e != nil {
		panic(e)
	}
	return n
}
func fixtureRunningTeam(s string) domainname.TeamRef {
	return domainname.TeamFromObservation(domainname.Observe(s))
}
func fixtureShortName(s string) domainname.ShortName {
	if s == "" {
		return domainname.ShortName{}
	}
	n, e := domainname.ParseMemberName(s)
	if e != nil {
		panic(e)
	}
	return n.Short()
}
func fixtureSeen(s string) domainname.Seen { return domainname.Observe(s) }

func fixtureTeamName(s string) domainname.TeamName {
	n, e := domainname.ParseTeamName(s)
	if e != nil {
		panic(e)
	}
	return n
}
