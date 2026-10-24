// Package team is where a team's files live for this invocation: the project
// root, the two layer directories, the language, and the team itself (I1, I2).
// The lookup of a name by layer is the catalog's.
package team

import (
	"errors"
	"fmt"
	"os"
	"os/exec" //nolint:depguard // gitRoot asks git for the project root; it is not a herdr call (d-ddd2 D2)
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// Env is where teams and profiles live for this invocation.
type Env struct {
	ProjectRoot string // the git root, or the current directory
	GlobalDir   string // $XDG_CONFIG_HOME/hergy, or ~/.config/hergy
	LocalDir    string // <project>/.hergy
	// inRepo says the project root is a real git repository, which is what tells
	// hergy new where to write (K2).
	inRepo bool
}

// NewEnv finds the project root and the global config directory.
func NewEnv() (Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Env{}, err
	}
	root, inRepo := gitRoot(cwd)
	return Env{
		ProjectRoot: root,
		GlobalDir:   globalDir(),
		LocalDir:    filepath.Join(root, ".hergy"),
		inRepo:      inRepo,
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

// InRepo reports whether the project root is inside a git repository. hergy new
// writes to the local layer then, and to the global layer otherwise (K2).
func (e Env) InRepo() bool { return e.inRepo }

// Catalog builds the lookup over this environment's two layers. It carries
// whether the project root is a git repository, which is where NewPlace reads
// K2 from (C13).
func (e Env) Catalog(reader source.Reader) *catalog.Catalog {
	return catalog.New(e.GlobalDir, e.LocalDir, e.ProjectRoot, reader, e.Display).InRepo(e.inRepo)
}

// Display is how a path is shown to a person: relative to the project root when
// it is inside, after ~/ when it is inside the home directory, and absolute
// otherwise. The project wins over the home directory, so a project under the
// home directory still shows its project-relative form.
func (e Env) Display(path string) string {
	if rel, ok := relativeUnder(e.ProjectRoot, path); ok {
		return rel
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rel, ok := relativeUnder(home, path); ok {
			if rel == "." {
				return "~"
			}
			return "~/" + rel
		}
	}
	return path
}

// DisplayPath is how a path is shown to a person when it is not read against a
// project: after ~/ when it is under the home directory, and absolute otherwise.
// The headquarters shows every project with this, so the same project reads the
// same wherever hergy serve was started (unlike Display, which makes a path under
// the project serve runs in relative to it).
func (e Env) DisplayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if resolved, err := filepath.EvalSymlinks(home); err == nil {
			home = resolved
		}
		if rel, ok := relativeUnder(home, path); ok {
			if rel == "." {
				return "~"
			}
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return path
}

// relativeUnder is path as seen from base, or false when it is not inside base.
func relativeUnder(base, path string) (string, bool) {
	if base == "" {
		return "", false
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
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
	return ParseConfig(string(text), path)
}

// EnsureGlobalConfig writes the global config.yaml when it is missing, so a
// first hergy new leaves the machine with a language it can read. It returns the
// path and whether it created the file. An existing file is left untouched,
// broken or not.
func (e Env) EnsureGlobalConfig(lang string) (string, bool, error) {
	path := filepath.Join(e.GlobalDir, "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return path, false, err
	}
	if err := os.MkdirAll(e.GlobalDir, 0o755); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(GlobalYAML(lang)), 0o644); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// AtProject is this environment with another project's root and layer: the
// global directory, the language and the machine stay the same, and the local
// layer becomes that project's .hergy/ (d-73fc: the headquarters reads every
// listed project through its own layer). Whether that project is a git
// repository is read here, the way NewEnv reads this one.
func (e Env) AtProject(dir string) (Env, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Env{}, err
	}
	root, inRepo := gitRoot(abs)
	e.ProjectRoot = root
	e.LocalDir = filepath.Join(root, ".hergy")
	e.inRepo = inRepo
	return e, nil
}

// WithProject pins the project root, so a hire run from outside the project
// (or with a different cwd) still resolves the local layer.
func (e Env) WithProject(dir string) (Env, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Env{}, err
	}
	root, inRepo := gitRoot(abs)
	e.ProjectRoot = root
	e.LocalDir = filepath.Join(root, ".hergy")
	e.inRepo = inRepo
	return e, nil
}

// gitRoot is the git root of dir, and whether dir is inside a repository at all.
func gitRoot(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel") //nolint:gosec // G204: dir is the cwd, the command is fixed
	out, err := cmd.Output()
	if err != nil {
		return dir, false
	}
	if root := strings.TrimSpace(string(out)); root != "" {
		return root, true
	}
	return dir, false
}

// TeamDir is where a team definition's files live in one layer.
func (e Env) TeamDir(name, layer string) string {
	if layer == source.LayerLocal {
		return filepath.Join(e.LocalDir, "teams", name)
	}
	return filepath.Join(e.GlobalDir, "teams", name)
}

// Team is one team: the definition it was booted from and the name it runs under
// (I1, I2). The definition is the type; a team is one run of it, and the name it
// runs under is its own (--as, else the definition's).
type Team struct {
	Name       string
	Definition *definition.Definition
}

type rawConfig struct {
	Version  *int     `yaml:"version"`
	Lang     string   `yaml:"lang"`
	Projects []string `yaml:"projects"`
}

// Config is the global config.yaml as hergy reads it: the language the screens
// and the prompts speak, and the projects the headquarters lists (d-73fc).
type Config struct {
	Lang string
	// Projects are the paths as the file writes them: absolute, or starting at
	// ~. Use ProjectPaths to read the directories they name.
	Projects []string
	// HasProjects says the file names projects: at all. A file without it is
	// the single-project serve hergy has always had; an empty list is a
	// headquarters with nothing listed.
	HasProjects bool
}

// ReadConfig reads the global config.yaml. It is held to the same strictness as
// team.yaml: the version must be present and 1, and any unknown key is an error
// with a reason.
func ReadConfig(text, path string) (Config, error) {
	if _, err := profile.ReadVersion(text, "config.yaml", path); err != nil {
		return Config{}, err
	}
	var raw rawConfig
	if err := profile.StrictDecode(text, &raw); err != nil {
		return Config{}, fmt.Errorf("%s: %s", path, profile.CleanError(err))
	}
	cfg := Config{Lang: raw.Lang, Projects: raw.Projects}
	cfg.HasProjects = namesProjects(text)
	return cfg, nil
}

// namesProjects reports whether the file writes a projects: key, whether or not
// the list under it has an item. The struct alone cannot tell "no key" from
// "the empty list", and the two mean different things (d-73fc).
func namesProjects(text string) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc.Content) == 0 {
		return false
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "projects" {
			return true
		}
	}
	return false
}

// ParseConfig reads the global config.yaml and returns the language it names, or
// "" when it names none.
func ParseConfig(text, path string) (string, error) {
	cfg, err := ReadConfig(text, path)
	if err != nil {
		return "", err
	}
	return cfg.Lang, nil
}

// ConfigPath is the global config.yaml of this environment.
func (e Env) ConfigPath() string { return filepath.Join(e.GlobalDir, "config.yaml") }

// ReadConfigFile reads this environment's global config.yaml. A file that is not
// there is the config of a machine that has never run hergy: no language, no
// projects, and that is not an error.
func (e Env) ReadConfigFile() (Config, error) {
	path := e.ConfigPath()
	text, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	return ReadConfig(string(text), e.Display(path))
}

// ProjectPaths reads the projects: list as the directories it names: each path
// expanded (~ is the home directory), made absolute, and resolved through any
// symbolic links, in the order the file writes them, with duplicates kept once.
// present says the file names projects: at all (Config.HasProjects).
//
// A relative path is a broken setting rather than a path guessed at: it names no
// directory hergy can know, and the sentence says which entry and where the file
// is. An entry that cannot be read as a path stops the whole list, so a broken
// setting is never half served.
func (e Env) ProjectPaths(cfg Config) (paths []string, present bool, err error) {
	seen := map[string]bool{}
	for _, raw := range cfg.Projects {
		path, err := ExpandProjectPath(raw)
		if err != nil {
			return nil, cfg.HasProjects, err
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths, cfg.HasProjects, nil
}

// ExpandProjectPath reads one projects: entry as the directory it names: ~ and
// ~/ are the home directory, a relative path is refused, and the path is made
// absolute and resolved through symbolic links when it is there.
func ExpandProjectPath(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", i18n.Errorf("error.project_path_empty")
	}
	if text == "~" || strings.HasPrefix(text, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		text = filepath.Join(home, strings.TrimPrefix(text, "~"))
	}
	if !filepath.IsAbs(text) {
		return "", i18n.Errorf("error.project_path_relative", raw)
	}
	text = filepath.Clean(text)
	if resolved, err := filepath.EvalSymlinks(text); err == nil {
		return resolved, nil
	}
	return text, nil
}

// GlobalYAML is the global config.yaml hergy new writes when the file is
// missing: the version every settings file carries, and the language that run
// displayed in. It holds exactly the two keys ParseConfig reads.
func GlobalYAML(lang string) string {
	return fmt.Sprintf("version: 1\nlang: %s\n", lang)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
