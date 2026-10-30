// Package scaffold holds the starter team hergy new writes out: a two-member
// team and the agreements between them, in each language, embedded in the
// binary. The files are written unchanged; after that the user owns them.
package scaffold

import (
	"embed"
	"io/fs"
	"regexp"
)

//go:embed files
var embedded embed.FS

// Fallback is the scaffold used when a language has none. It mirrors the
// language fallback of the CLI.
const Fallback = "en"

// NamePattern is the team definition names hergy new accepts: short enough that
// "<team>-builder" is still a valid member name.
var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,24}$`)

// Names are the files a starter team has, in the order hergy new writes them.
var Names = []string{"team.yaml", "agreement.yaml"}

// RoleNames are the role documents the starter team's members read. They are not
// the team's own files: a role document lives once per layer, at
// <layer>/roles/<role>.yaml (K5), so hergy new writes them there only when
// neither place holds a role document, and the team screen only when the person
// presses the starter-roles button (r-54b8).
var RoleNames = []string{"lead.yaml", "builder.yaml"}

// The profile the starter team's members name, and the engine it picks. hergy
// new writes it into the layer's profiles.yaml when that layer cannot already
// resolve the name, so a starter team boots as it is (I7). The name says what
// the profile picks, not the kind it happens to run.
const (
	ProfileName = "claude-default"
	ProfileKind = "claude"
)

// DescriptionPlaceholder is the description line the starter team's team.yaml
// writes, in each language. It is an example, not a description: a definition
// that still writes it has not said which case its team is meant for, so hergy
// validate warns about it (d-3cae). It is kept in step with
// files/<lang>/team.yaml by a test.
var DescriptionPlaceholder = map[string]string{
	Fallback: "which case this team is meant for, in one line",
	"ja":     "このチームが想定するケースを 1 行で",
}

// IsPlaceholderDescription reports whether a description is still a starter
// example, in any language: a definition carried from another language still
// has not said which case its team is meant for.
func IsPlaceholderDescription(description string) bool {
	for _, example := range DescriptionPlaceholder {
		if description == example {
			return true
		}
	}
	return false
}

// File is one file of a starter team: its path inside the team directory, and
// its contents.
type File struct {
	Path string
	Data []byte
}

// Team returns the starter files for a language, in the order they are written.
// A language with no scaffold falls back to English. The files are written
// unchanged; the member names in team.yaml are the short member names (lead, builder),
// and hergy adds the team name when it boots them.
func Team(lang string) ([]File, error) {
	root := root(lang)
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

// Roles returns the starter team's role documents for a language, in the order
// they are written. Path is the file's name under the layer's roles/ directory:
// a role document is the layer's, not the team's (K5). hergy new leaves one that
// is already there alone, so this only says what the starter team reads.
func Roles(lang string) ([]File, error) {
	root := root(lang)
	files := make([]File, 0, len(RoleNames))
	for _, name := range RoleNames {
		data, err := embedded.ReadFile(root + "/roles/" + name)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: name, Data: data})
	}
	return files, nil
}

// root is where a language's files are, with English as the fallback.
func root(lang string) string {
	dir := "files/" + lang
	if _, err := fs.Stat(embedded, dir); err != nil {
		dir = "files/" + Fallback
	}
	return dir
}
