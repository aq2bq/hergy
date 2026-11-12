package turns

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

func fixtureNaming(s string) domainname.Naming {
	d, _ := domainname.ParseDefinitionName(s)
	return domainname.DefinitionNames(d)
}
func fixtureSeen(s string) domainname.Seen { return observedname.Observe(s) }
