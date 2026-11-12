package definition

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func testPurpose(s string) domainname.Purpose {
	n, err := domainname.ParsePurpose(s)
	if err != nil {
		panic(err)
	}
	return n
}
