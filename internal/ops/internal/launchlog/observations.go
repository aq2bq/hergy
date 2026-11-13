package launchlog

import (
	"encoding/json"

	"github.com/aq2bq/hergy/internal/ops/internal/name"
	"github.com/aq2bq/hergy/internal/ops/internal/place"
)

// UnmarshalJSON replays the observation at its private record boundary.
// Observation replay belongs to the record boundary, never to the public
// domain types. Legacy spellings and missing paths remain exactly as recorded.
func (r *Record) UnmarshalJSON(raw []byte) error {
	type plain Record
	w := struct {
		*plain
		Team     string `json:"team"`
		Project  string `json:"project"`
		TeamYAML string `json:"team_yaml"`
	}{plain: (*plain)(r)}
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	r.Team = name.Observe(w.Team)
	r.Project = place.Observe(w.Project)
	r.TeamYAML = place.Observe(w.TeamYAML)
	return nil
}

// UnmarshalJSON restores the member spelling exactly as recorded.
func (m *Member) UnmarshalJSON(raw []byte) error {
	type plain Member
	w := struct {
		*plain
		Name string `json:"name"`
	}{plain: (*plain)(m)}
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	m.Name = name.Observe(w.Name)
	return nil
}
