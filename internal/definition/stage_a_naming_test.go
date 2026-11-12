package definition_test

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureNaming(s string) domainname.Naming {
	d, _ := domainname.ParseDefinitionName(s)
	return domainname.DefinitionNames(d)
}
