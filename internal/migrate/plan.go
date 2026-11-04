package migrate

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Plan prepares every legacy definition in one explicitly selected layer.
// It deliberately excludes team.yaml, profiles.yaml and the retired common
// agreement.yaml. A partial conversion is found through its adjacent backup.
func Plan(layer string) ([]File, error) {
	paths := map[string]bool{}
	for _, pattern := range []string{"roles/*.yaml", "roles/*.yaml.bak", "teams/*/agreement.yaml", "teams/*/agreement.yaml.bak"} {
		found, err := filepath.Glob(filepath.Join(layer, pattern))
		if err != nil {
			return nil, err
		}
		for _, path := range found {
			paths[strings.TrimSuffix(path, ".bak")] = true
		}
	}
	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	out := make([]File, 0, len(names))
	for _, path := range names {
		// A completed migration's backup is historical data: a later edit to the
		// Markdown must not be compared with the original conversion again.
		if _, err := readRegular(path); errors.Is(err, fs.ErrNotExist) {
			if _, err := readRegular(strings.TrimSuffix(path, ".yaml") + ".md"); err == nil {
				continue
			}
		}
		file, err := Prepare(path)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, nil
}
