package check

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

func fixtureNaming(s string) domainname.Naming {
	d, _ := domainname.ParseDefinitionName(s)
	return domainname.DefinitionNames(d)
}

func fixtureNamingList(ss []string) []domainname.Naming {
	out := make([]domainname.Naming, len(ss))
	for i, s := range ss {
		out[i] = fixtureNaming(s)
	}
	return out
}
