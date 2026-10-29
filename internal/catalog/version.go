package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Version is the version of every settings file this catalog reads: both
// layers' profiles.yaml, each team's team.yaml and agreement.yaml, and each role
// document and role Markdown. A screen that keeps what it built from them keys
// it by this string, so a file written again gives a new key and the next answer
// is built again; a request with the same files finds what was built (d-e00b).
//
// Only names and the files' own versions (size and modification time) are read,
// never their texts: the version is cheap, and a text that changed always moves
// its version.
func (c *Catalog) Version() string {
	var b strings.Builder
	for _, dir := range []string{c.globalDir, c.localDir} {
		paths := []string{
			filepath.Join(dir, "profiles.yaml"),
			// The shared agreement.yaml a layer still holds is a problem every
			// document of that layer carries (d-31ad).
			filepath.Join(dir, "agreement.yaml"),
		}
		for _, name := range c.teamNames(dir) {
			paths = append(paths,
				filepath.Join(dir, "teams", name, "team.yaml"),
				filepath.Join(dir, "teams", name, "agreement.yaml"),
			)
		}
		for _, name := range c.roleNames(dir) {
			paths = append(paths,
				filepath.Join(dir, "roles", name+".yaml"),
				filepath.Join(dir, "roles", name+".md"),
			)
		}
		sort.Strings(paths)
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			fmt.Fprintf(&b, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}
