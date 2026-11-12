package herdr

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureHerdrName(s string) domainname.HerdrName {
	var n domainname.HerdrName
	if e := n.UnmarshalText([]byte(s)); e != nil {
		panic(e)
	}
	return n
}
func fixtureSeen(s string) domainname.Seen { return domainname.Observe(s) }
