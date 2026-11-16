package ops

import (
	"os"
	"path/filepath"
	"strings"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/projectinput"
)

// This file is the projects config.yaml lists, as a command reads and writes
// them. The list says which projects hergy serve shows (d-73fc): a team started
// in a project the list leaves out runs, and no screen shows it.

// AddedProject is what listing one project did: the entry as config.yaml holds
// it, the file as a person reads its path, and whether the entry was already
// there (nothing was written then).
type AddedProject struct {
	Entry, Config string
	Already       bool
}

// AddProject lists the project one directory stands in (its git repository's
// root, or the directory itself) at the end of the global config.yaml's
// projects:. A relative directory is read from where the command runs. A project
// already listed is not written twice.
func AddProject(env Environment, dir string) (AddedProject, error) {
	path := dir
	if !filepath.IsAbs(path) && path != "~" && !strings.HasPrefix(path, "~/") {
		abs, err := filepath.Abs(path)
		if err != nil {
			return AddedProject{}, err
		}
		path = abs
	}
	if info, err := os.Stat(expandHome(path)); err != nil || !info.IsDir() {
		return AddedProject{}, i18n.Errorf("error.project_not_dir", dir)
	}
	seen, err := projectinput.Read(domainplace.Input(path))
	if err != nil {
		return AddedProject{}, err
	}
	at, err := env.AtProject(seen)
	if err != nil {
		return AddedProject{}, err
	}
	already := false
	if cfg, err := env.ReadConfigFile(); err == nil {
		if paths, _, err := env.ProjectPaths(cfg); err == nil {
			already = listedProject(paths, at.ProjectRoot, env.AtProject)
		}
	}
	entry, err := env.AddProject(at.ProjectRoot.String())
	if err != nil {
		return AddedProject{}, err
	}
	return AddedProject{Entry: entry, Config: env.DisplayPath(env.ConfigPath()), Already: already}, nil
}

// expandHome reads ~ and ~/… the way a config.yaml entry is read.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// listedProject reports whether want is one of the projects config.yaml lists.
// A listed directory inside a git repository is that repository's project, so
// both spellings name the listed project (r-1db7 L1).
func listedProject(paths []domainplace.Project, want domainplace.Project, at func(domainplace.Project) (Environment, error)) bool {
	for _, one := range paths {
		if one.Compare(want) == domainplace.Same {
			return true
		}
		if env, err := at(one); err == nil && env.ProjectRoot.Compare(want) == domainplace.Same {
			return true
		}
	}
	return false
}

// unlistedProjectNote is the line a start prints when config.yaml lists projects
// and leaves this one out: hergy serve does not show the team, and one command
// lists the project (n-ab6c). It is empty when the project is listed, when the
// file lists none (serve then shows the project it is started in), and when the
// file does not read.
func unlistedProjectNote(env Environment) string {
	cfg, err := env.ReadConfigFile()
	if err != nil {
		return ""
	}
	paths, present, err := env.ProjectPaths(cfg)
	if err != nil || !present || listedProject(paths, env.ProjectRoot, env.AtProject) {
		return ""
	}
	return i18n.Tf("start.project_unlisted", env.DisplayPath(env.ConfigPath()), projectArgument(env))
}

// projectArgument is the directory to type after hergy projects add: . from
// inside the project, the project's own path from anywhere else.
func projectArgument(env Environment) string {
	if wd, err := os.Getwd(); err == nil {
		if resolved, err := filepath.EvalSymlinks(wd); err == nil {
			wd = resolved
		}
		if rel, err := filepath.Rel(env.ProjectRoot.String(), wd); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "."
		}
	}
	return env.ConfigProjectPath(env.ProjectRoot)
}
