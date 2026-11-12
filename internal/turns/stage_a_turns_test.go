package turns

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureMemberName(s string) domainname.MemberName {
	n, e := domainname.ParseMemberName(s)
	if e != nil {
		panic(e)
	}
	return n
}

func fixtureNaming(s string) domainname.Naming {
	d, _ := domainname.ParseDefinitionName(s)
	return domainname.DefinitionNames(d)
}
func fixtureSeen(s string) domainname.Seen {
	var n domainname.Seen
	_ = n.UnmarshalText([]byte(s))
	return n
}
