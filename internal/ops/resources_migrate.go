package ops

import (
	"fmt"

	"github.com/aq2bq/hergy/internal/ops/internal/migrate"
)

// MigratedResource is the shared resource model.
type MigratedResource struct{ Path, Destination, Backup string }

// MigrateResources preflights the entire layer before applying any conversion.
func MigrateResources(layer string, dry bool) ([]MigratedResource, error) {
	plan, err := migrate.Plan(layer)
	if err != nil {
		return nil, err
	}
	out := make([]MigratedResource, 0, len(plan))
	for _, f := range plan {
		out = append(out, MigratedResource{Path: f.Path, Destination: f.Destination, Backup: f.Backup})
	}
	if !dry {
		for _, f := range plan {
			if err := migrate.Apply(f); err != nil {
				return out, fmt.Errorf("%s: %w", f.Path, err)
			}
		}
	}
	return out, nil
}
