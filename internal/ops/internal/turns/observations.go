package turns

import (
	"encoding/json"

	"github.com/aq2bq/hergy/internal/ops/internal/name"
)

// UnmarshalJSON restores the team spelling at the private state-file boundary.
func (s *State) UnmarshalJSON(raw []byte) error {
	type plain State
	w := struct {
		*plain
		Team string `json:"team"`
	}{plain: (*plain)(s)}
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	s.Team = name.Observe(w.Team)
	return nil
}
