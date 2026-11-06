package team

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// This file writes the projects: list of the global config.yaml without touching
// the lines around it (d-73fc: the list is a person's own file, which they read
// by hand). Adding an entry appends one item to the block sequence, and removing
// one takes its own lines out. A file with no projects: key gets one at its end.

// AddProject writes one directory into the global config.yaml's projects: list,
// and returns the path as the file now writes it: ~/… when the directory is under
// the home directory, the absolute path otherwise (lead's addition 3: the person
// reads the file by hand). A directory already listed is not written twice.
func (e Env) AddProject(dir string) (string, error) {
	return e.AddProjectAt(dir, -1)
}

// AddProjectAt adds one directory at a position in the projects: list (a
// position outside it means the end), so a project taken out can go back where
// it stood (d-2cce).
func (e Env) AddProjectAt(dir string, at int) (string, error) {
	path, err := ExpandProjectPath(dir)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", i18n.Errorf("error.project_not_dir", dir)
	}
	written := e.ConfigProjectPath(path)
	pathFile := e.ConfigPath()
	text, err := readOrCreateConfig(pathFile)
	if err != nil {
		return "", err
	}
	out, err := EditProjects(text, e.Display(pathFile), addProject{path: path, written: written, at: at})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(pathFile), 0o755); err != nil {
		return "", err
	}
	if err := source.WriteBeside(pathFile, string(out)); err != nil {
		return "", err
	}
	return written, nil
}

// RemoveProject takes one directory out of the global config.yaml's projects:
// list. A path that is not listed is an error naming the file.
func (e Env) RemoveProject(dir string) (int, error) {
	path, err := ExpandProjectPath(dir)
	if err != nil {
		return -1, err
	}
	pathFile := e.ConfigPath()
	text, err := os.ReadFile(pathFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return -1, i18n.Errorf("error.project_not_listed", dir)
		}
		return -1, err
	}
	at := projectIndex(text, path)
	out, err := EditProjects(text, e.Display(pathFile), removeProject{path: path})
	if err != nil {
		return -1, err
	}
	return at, source.WriteBeside(pathFile, string(out))
}

// projectIndex is where one expanded path stands in the projects: list, or -1.
func projectIndex(text []byte, path string) int {
	var doc yaml.Node
	if yaml.Unmarshal(text, &doc) != nil || len(doc.Content) == 0 {
		return -1
	}
	projects := yamledit.MappingValue(doc.Content[0], "projects")
	if projects == nil {
		return -1
	}
	for i, item := range projects.Content {
		expanded, err := ExpandProjectPath(item.Value)
		if err == nil && expanded == path {
			return i
		}
	}
	return -1
}

// ConfigProjectPath is how one project path is written into config.yaml: ~/…
// when it is under the home directory, the absolute path otherwise.
func (e Env) ConfigProjectPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	// The path came through ExpandProjectPath, which resolves symbolic links
	// (macOS's /tmp beside /private/tmp); the home directory is read the same way
	// so the two are compared as the directories they are.
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}

// readOrCreateConfig is the config file's text, or the text a first write starts
// from. The language is not invented: a file hergy makes holds the version and
// the projects: key only, and the default language is the one no line names.
func readOrCreateConfig(path string) ([]byte, error) {
	text, err := os.ReadFile(path)
	if err == nil {
		return text, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return []byte("version: 1\n"), nil
}

// projectEdit is one change to the projects: list.
type projectEdit interface {
	// apply writes the change on the file's lines and answers the list the file
	// must read back as.
	apply(src *yamledit.Source, root, projects *yaml.Node) ([]string, error)
}

type addProject struct {
	path    string // the expanded directory
	written string // how the entry is written (~/… or absolute)
	at      int    // where the entry goes; outside the list means the end
}

func (a addProject) apply(src *yamledit.Source, root, projects *yaml.Node) ([]string, error) {
	items, err := projectItems(src, root, projects)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		expanded, err := ExpandProjectPath(item)
		if err != nil {
			return nil, err
		}
		if expanded == a.path {
			return items, nil
		}
	}
	if projects == nil {
		src.Append("projects:", yamledit.Spaces(2)+"- "+yamledit.ScalarLiteral(a.written, 0))
		return append(items, a.written), nil
	}
	if projects.Style&yaml.FlowStyle != 0 || len(projects.Content) == 0 {
		key := yamledit.MappingKeyNode(root, "projects")
		if key == nil {
			return nil, src.Shape("projects")
		}
		indent := yamledit.LeadingSpaces(src.Line(key.Line))
		all := append(append([]string{}, items...), a.written)
		lines := append([]string{yamledit.Spaces(indent) + "projects:"}, yamledit.ItemLines(quotedItems(all), indent+2)...)
		src.ReplaceLine(key.Line, lines)
		return all, nil
	}
	place, err := yamledit.BlockPlace(src, root, "projects", projects)
	if err != nil {
		return nil, err
	}
	line := place.AfterLast
	if a.at >= 0 && a.at < len(projects.Content) && projects.Content[a.at].Line > 0 {
		line = projects.Content[a.at].Line - 1
	}
	src.Insert(line, yamledit.Spaces(place.DashCol)+"- "+yamledit.ScalarLiteral(a.written, 0))
	all := append([]string{}, items...)
	at := a.at
	if at < 0 || at > len(all) {
		at = len(all)
	}
	all = append(all[:at], append([]string{a.written}, all[at:]...)...)
	return all, nil
}

type removeProject struct{ path string }

func (r removeProject) apply(src *yamledit.Source, root, projects *yaml.Node) ([]string, error) {
	if projects == nil {
		return nil, i18n.Errorf("error.project_not_listed", r.path)
	}
	items, err := projectItems(src, root, projects)
	if err != nil {
		return nil, err
	}
	for i, item := range projects.Content {
		expanded, err := ExpandProjectPath(item.Value)
		if err != nil {
			return nil, err
		}
		if expanded != r.path {
			continue
		}
		if err := yamledit.RemoveItemLines(src, root, "projects", projects, item); err != nil {
			return nil, err
		}
		out := append([]string{}, items[:i]...)
		return append(out, items[i+1:]...), nil
	}
	return nil, i18n.Errorf("error.project_not_listed", r.path)
}

// quotedItems writes each entry as a YAML scalar on its own, the way a block
// sequence holds it.
func quotedItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, yamledit.ScalarLiteral(item, 0))
	}
	return out
}

// projectItems is the list as it reads now, and the checks every edit needs: a
// squashed shape the line editor cannot place is refused before anything is
// written.
func projectItems(src *yamledit.Source, root, projects *yaml.Node) ([]string, error) {
	if projects == nil {
		return nil, nil
	}
	if _, bad := yamledit.ShapeKeyOf(root); bad {
		return nil, src.Shape("projects")
	}
	items := make([]string, 0, len(projects.Content))
	for _, item := range projects.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, src.Shape("projects")
		}
		items = append(items, item.Value)
	}
	return items, nil
}

// EditProjects applies one change to a global config.yaml's projects: list and
// returns the whole file, every other line as it was.
func EditProjects(text []byte, display string, edit projectEdit) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", display, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.edit_not_mapping", display)
	}
	root := doc.Content[0]
	projects := yamledit.MappingValue(root, "projects")
	src := yamledit.New(text, display)
	want, err := edit.apply(src, root, projects)
	if err != nil {
		return nil, err
	}
	out := src.Bytes()
	if err := sameProjectList(out, want, display); err != nil {
		return nil, err
	}
	return out, nil
}

// sameProjectList reads the written file back and compares its projects: list
// with the one the edit meant. A line edit that put a value on the wrong line is
// hergy's own fault, and the sentence says so before the file is written.
func sameProjectList(after []byte, want []string, display string) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(after, &doc); err != nil {
		return i18n.Errorf("error.edit_yaml", display, err)
	}
	var raw rawConfig
	if err := doc.Decode(&raw); err != nil {
		return i18n.Errorf("error.edit_yaml", display, err)
	}
	if len(raw.Projects) != len(want) {
		return i18n.Errorf("error.edit_shape", display, "projects")
	}
	for i := range want {
		if raw.Projects[i] != want[i] {
			return i18n.Errorf("error.edit_shape", display, "projects")
		}
	}
	return nil
}
