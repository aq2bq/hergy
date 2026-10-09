// Package team resolves a team name to a team.yaml through the layers, and
// builds the config from it. It is the layer-aware part of the CLI.
//
// Layers, in order: global -> local. A team lives in one team.yaml; when the
// same name exists in both global and local, local wins.
package team

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// Env is where teams and profiles live for this invocation.
type Env struct {
	ProjectRoot string // the git root, or the current directory
	GlobalDir   string // $XDG_CONFIG_HOME/hergy, or ~/.config/hergy
	LocalDir    string // <project>/.hergy
}

// NewEnv finds the project root and the global config directory.
func NewEnv() (Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Env{}, err
	}
	root := gitRoot(cwd)
	return Env{
		ProjectRoot: root,
		GlobalDir:   globalDir(),
		LocalDir:    filepath.Join(root, ".hergy"),
	}, nil
}

func globalDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "hergy")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "hergy")
}

// Lang is the language named in the global config.yaml under GlobalDir, or ""
// when the file is absent or names none. i18n.Resolve applies the precedence
// with --lang; this only reads what the file says. The file is held to the same
// strictness as team.yaml: the version must be 1 and unknown keys stop.
func Lang() (string, error) {
	path := filepath.Join(globalDir(), "config.yaml")
	if !isFile(path) {
		return "", nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return config.ParseConfig(string(text), path)
}

// WithProject pins the project root, so a hire run from outside the project
// (or with a different cwd) still resolves the local layer.
func (e Env) WithProject(dir string) (Env, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Env{}, err
	}
	e.ProjectRoot = abs
	e.LocalDir = filepath.Join(abs, ".hergy")
	return e, nil
}

func gitRoot(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return dir
	}
	if root := strings.TrimSpace(string(out)); root != "" {
		return root
	}
	return dir
}

// Paths to a team's team.yaml in each layer.
func (e Env) localTeamYAML(name string) string {
	return filepath.Join(e.LocalDir, "teams", name, "team.yaml")
}

func (e Env) globalTeamYAML(name string) string {
	return filepath.Join(e.GlobalDir, "teams", name, "team.yaml")
}

// Resolved is where a team name was found.
type Resolved struct {
	Name           string
	Layer          string
	TeamYAMLPath   string
	TeamDir        string
	GlobalTeamYAML string // set when the global layer also defines this name
	LocalTeamYAML  string // set when the local layer also defines this name
}

// Resolve finds a team by name. force is "", "global" or "local".
func (e Env) Resolve(name, force string) (*Resolved, error) {
	if name == "" {
		return e.resolveDefault(force)
	}
	local := e.localTeamYAML(name)
	global := e.globalTeamYAML(name)
	hasLocal := isFile(local)
	hasGlobal := isFile(global)

	resolved := &Resolved{Name: name}
	if hasGlobal {
		resolved.GlobalTeamYAML = global
	}
	if hasLocal {
		resolved.LocalTeamYAML = local
	}

	switch force {
	case "local":
		if !hasLocal {
			return nil, i18n.Errorf("error.team_not_local", name, local, e.LocalDir)
		}
		resolved.Layer = source.LayerLocal
		resolved.TeamYAMLPath = local
	case "global":
		if !hasGlobal {
			return nil, i18n.Errorf("error.team_not_global", name, global, e.GlobalDir)
		}
		resolved.Layer = source.LayerGlobal
		resolved.TeamYAMLPath = global
	default:
		switch {
		case hasLocal:
			resolved.Layer = source.LayerLocal
			resolved.TeamYAMLPath = local
		case hasGlobal:
			resolved.Layer = source.LayerGlobal
			resolved.TeamYAMLPath = global
		default:
			return nil, e.notFound(name, local, global)
		}
	}
	resolved.TeamDir = filepath.Dir(resolved.TeamYAMLPath)
	return resolved, nil
}

// resolveDefault is what a missing team name does: the single local team, or a
// list of candidates to choose from.
func (e Env) resolveDefault(force string) (*Resolved, error) {
	switch force {
	case "global":
		names := e.teamNames(e.GlobalDir)
		if len(names) == 1 {
			return e.Resolve(names[0], "global")
		}
		return nil, e.candidates(i18n.Tf("error.global_count", len(names)))
	case "local":
		names := e.teamNames(e.LocalDir)
		if len(names) == 1 {
			return e.Resolve(names[0], "local")
		}
		return nil, e.candidates(i18n.Tf("error.local_count", len(names)))
	default:
		names := e.teamNames(e.LocalDir)
		if len(names) == 1 {
			return e.Resolve(names[0], "local")
		}
		return nil, e.candidates(i18n.Tf("error.need_team_name", len(names)))
	}
}

func (e Env) notFound(name, local, global string) error {
	var b strings.Builder
	fmt.Fprintf(&b, i18n.T("error.team_not_found"), name, local, global)
	if candidates := e.candidatesList(); len(candidates) > 0 {
		fmt.Fprintf(&b, i18n.T("error.candidates"), strings.Join(candidates, ", "))
	}
	return fmt.Errorf("%s", b.String())
}

func (e Env) candidates(head string) error {
	list := e.candidatesList()
	if len(list) == 0 {
		return i18n.Errorf("error.no_candidates", head, e.GlobalDir, e.LocalDir)
	}
	return i18n.Errorf("error.candidates_head", head, strings.Join(list, ", "))
}

// candidatesList is every team name visible in either layer.
func (e Env) candidatesList() []string {
	seen := map[string]bool{}
	var names []string
	for _, name := range append(e.teamNames(e.LocalDir), e.teamNames(e.GlobalDir)...) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// teamNames lists the team directories under a layer that have a team.yaml.
func (e Env) teamNames(dir string) []string {
	matches, err := filepath.Glob(filepath.Join(dir, "teams", "*", "team.yaml"))
	if err != nil {
		return nil
	}
	var names []string
	for _, match := range matches {
		names = append(names, filepath.Base(filepath.Dir(match)))
	}
	sort.Strings(names)
	return names
}

// Listing is one row of `hergy ls`.
type Listing struct {
	Name       string
	Local      bool
	Global     bool
	LocalPath  string
	GlobalPath string
}

// List returns the visible teams, local and global, by name.
func (e Env) List() []Listing {
	byName := map[string]*Listing{}
	add := func(name string, local bool, path string) {
		row, ok := byName[name]
		if !ok {
			row = &Listing{Name: name}
			byName[name] = row
		}
		if local {
			row.Local = true
			row.LocalPath = path
		} else {
			row.Global = true
			row.GlobalPath = path
		}
	}
	for _, name := range e.teamNames(e.LocalDir) {
		add(name, true, e.localTeamYAML(name))
	}
	for _, name := range e.teamNames(e.GlobalDir) {
		add(name, false, e.globalTeamYAML(name))
	}
	rows := make([]Listing, 0, len(byName))
	for _, row := range byName {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// BuildConfig reads the resolved team.yaml and the global profiles.yaml and
// parses them into a Config.
func (e Env) BuildConfig(resolved *Resolved, reader source.Reader) (*config.Config, error) {
	text, err := os.ReadFile(resolved.TeamYAMLPath)
	if err != nil {
		return nil, err
	}
	profiles, profilesPath, err := e.LoadProfiles()
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = "hergy"
	}
	return config.Parse(config.Input{
		YAML:           string(text),
		TeamName:       resolved.Name,
		Layer:          resolved.Layer,
		TeamYAMLPath:   resolved.TeamYAMLPath,
		TeamDir:        resolved.TeamDir,
		Profiles:       profiles,
		ProfilesYAML:   profilesPath,
		Base:           e.ProjectRoot,
		Reader:         reader,
		Executable:     executable,
		GlobalTeamYAML: resolved.GlobalTeamYAML,
		LocalTeamYAML:  resolved.LocalTeamYAML,
	})
}

// LoadProfiles reads the machine-global profiles.yaml. It is absent until one is
// written, which is fine when every member spells its engine out inline.
func (e Env) LoadProfiles() (map[string]config.Profile, string, error) {
	path := filepath.Join(e.GlobalDir, "profiles.yaml")
	if !isFile(path) {
		return map[string]config.Profile{}, "", nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	profiles, err := config.ParseProfiles(string(text), path)
	if err != nil {
		return nil, "", err
	}
	return profiles, path, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
