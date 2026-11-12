package definition_test

import domainplace "github.com/aq2bq/hergy/internal/domain/place"

func projectPath(raw string) domainplace.Project {
	p, err := domainplace.NewProject(raw)
	if err != nil {
		panic(err)
	}
	return p
}
