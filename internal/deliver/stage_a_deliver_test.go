package deliver

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureMemberTarget(team, short string) domainname.HerdrName {
	t, e := domainname.ParseTeamName(team)
	if e != nil {
		panic(e)
	}
	m, e := domainname.ParseMemberName(short)
	if e != nil {
		panic(e)
	}
	n, e := t.Member(m)
	if e != nil {
		panic(e)
	}
	return n
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

func fixtureDemoTeam() domainname.TeamName {
	n, e := domainname.ParseTeamName("demo")
	if e != nil {
		panic(e)
	}
	return n
}
