package team

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
)

func projectPath(raw string) domainplace.Project {
	p, err := domainplace.NewProject(raw)
	if err != nil {
		panic(err)
	}
	return p
}
func seenPaths(raw []string) []domainplace.Seen {
	out := make([]domainplace.Seen, len(raw))
	for i, path := range raw {
		out[i] = observedplace.Observe(path)
	}
	return out
}
