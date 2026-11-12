package herdr

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
)

func fixtureHerdrName(s string) domainname.HerdrName {
	var n domainname.HerdrName
	if e := n.UnmarshalText([]byte(s)); e != nil {
		panic(e)
	}
	return n
}
func fixtureSeen(s string) domainname.Seen { return observedname.Observe(s) }
