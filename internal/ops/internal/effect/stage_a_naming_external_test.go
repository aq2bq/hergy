package effect_test

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureDemoNaming() domainname.Naming {
	d, _ := domainname.ParseDefinitionName("demo")
	return domainname.DefinitionNames(d)
}
