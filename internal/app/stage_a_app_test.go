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
	return domainname.TeamFromObservation(fixtureSeen(s))
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
func fixtureSeen(s string) domainname.Seen {
	var n domainname.Seen
	_ = n.UnmarshalText([]byte(s))
	return n
}

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

func namingStrings(ns []domainname.Naming) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.String()
	}
	return out
}

func fixturePane(s string) domainname.PaneID { return domainname.PaneID(s) }
