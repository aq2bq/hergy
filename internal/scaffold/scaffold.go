// Package scaffold holds the starter team hergy new writes out: a two-member
// team and the agreements between them, in each language, embedded in the
// binary. The files are written unchanged; after that the user owns them.
package scaffold

import (
	"bytes"
	"embed"
	"io/fs"
	"regexp"
)

//go:embed files
var embedded embed.FS

// Fallback is the scaffold used when a language has none. It mirrors the
// language fallback of the CLI.
const Fallback = "en"

// NamePattern is the team names hergy new accepts: short enough that
// "<team>-worker" is still a valid member name.
var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,24}$`)

// Names are the files a starter team has, in the order hergy new writes them.
var Names = []string{"team.yaml", "protocol.md", "roles/lead.md", "roles/worker.md"}

// File is one file of a starter team: its path inside the team directory, and
// its contents.
type File struct {
	Path string
	Data []byte
}

// Team returns the starter files for a language, in the order they are written.
// A language with no scaffold falls back to English. %{team} in team.yaml becomes
// the team name, so members of different teams get different herdr names.
func Team(lang, team string) ([]File, error) {
	root := "files/" + lang
	if _, err := fs.Stat(embedded, root); err != nil {
		root = "files/" + Fallback
	}
	files := make([]File, 0, len(Names))
	for _, name := range Names {
		data, err := embedded.ReadFile(root + "/" + name)
		if err != nil {
			return nil, err
		}
		if name == "team.yaml" {
			data = bytes.ReplaceAll(data, []byte("%{team}"), []byte(team))
		}
		files = append(files, File{Path: name, Data: data})
	}
	return files, nil
}
