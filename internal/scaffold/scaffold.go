// Package scaffold holds the starter team hergy new writes out: a two-member
// team and the agreements between them, in each language, embedded in the
// binary. The files are written unchanged; after that the user owns them.
package scaffold

import (
	"embed"
	"io/fs"
)

//go:embed files
var embedded embed.FS

// Fallback is the scaffold used when a language has none. It mirrors the
// language fallback of the CLI.
const Fallback = "en"

// Names are the files a starter team has, in the order hergy new writes them.
var Names = []string{"team.yaml", "protocol.md", "roles/lead.md", "roles/worker.md"}

// File is one file of a starter team: its path inside the team directory, and
// its contents.
type File struct {
	Path string
	Data []byte
}

// Team returns the starter files for a language, in the order they are written.
// A language with no scaffold falls back to English.
func Team(lang string) ([]File, error) {
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
		files = append(files, File{Path: name, Data: data})
	}
	return files, nil
}
