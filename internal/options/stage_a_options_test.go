package options

import domainname "github.com/aq2bq/hergy/internal/domain/name"

func fixtureSeen(s string) domainname.Seen {
	var n domainname.Seen
	_ = n.UnmarshalText([]byte(s))
	return n
}
