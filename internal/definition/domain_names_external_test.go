package definition_test

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func testProfileName(s string) domainname.ProfileName {
	n, err := domainname.ParseProfileName(s)
	if err != nil {
		panic(err)
	}
	return n
}
