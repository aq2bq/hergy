package ops

import (
	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// RecordedTeamNames reads every team name this definition has been run under.
func RecordedTeamNames(def *definition.Definition) []domainname.Naming {
	names := []domainname.Naming{domainname.DefinitionNames(def.Name)}
	seen := map[string]bool{def.Name.String(): true}
	records, err := readLaunchRecords()
	if err != nil {
		return names
	}
	for _, record := range records {
		if (record.TeamYAML.Project().Compare(def.TeamYAML.Project()) != domainplace.Same && (record.TeamYAML.String() != "" || def.TeamYAML.String() != "")) || record.Team.String() == "" || seen[record.Team.String()] {
			continue
		}
		seen[record.Team.String()] = true
		names = append(names, domainname.RunningNames(domainname.TeamFromObservation(record.Team)))
	}
	return names
}
