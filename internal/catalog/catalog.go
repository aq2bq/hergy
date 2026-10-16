// Package catalog is the one place a settings name is looked up: a team
// definition and a profile. The layers are hers (I11): a name hergy resolves
// goes local first, then global, and the layer never leaves this package.
//
// A team definition of the global layer reads the project's profiles too (K1):
// there is one way to a name, so a definition carried from one project to
// another reads what the project it stands in carries.
package catalog

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// Catalog reads the two layers. Display is how a path is shown to a person; the
// app hands the environment's own.
type Catalog struct {
	globalDir string
	localDir  string
	base      string
	reader    source.Reader
	display   func(string) string
}

// New builds a catalog over the two layer directories and the project root.
func New(globalDir, localDir, base string, reader source.Reader, display func(string) string) *Catalog {
	return &Catalog{
		globalDir: globalDir,
		localDir:  localDir,
		base:      base,
		reader:    reader,
		display:   display,
	}
}

// LayerDir is where one layer's files live.
func (c *Catalog) LayerDir(layer string) string {
	if layer == source.LayerLocal {
		return c.localDir
	}
	return c.globalDir
}

// TeamYAML is one layer's team.yaml for a definition name.
func (c *Catalog) TeamYAML(name, layer string) string {
	return filepath.Join(c.LayerDir(layer), "teams", name, "team.yaml")
}

// Listing is one row of `hergy ls`: a visible name and where each layer holds it.
type Listing struct {
	Name       string
	Local      bool
	Global     bool
	LocalPath  string
	GlobalPath string
}

// Definitions returns the visible team definitions, local and global, by name.
func (c *Catalog) Definitions() []Listing {
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
	for _, name := range teamNames(c.localDir) {
		add(name, true, c.TeamYAML(name, source.LayerLocal))
	}
	for _, name := range teamNames(c.globalDir) {
		add(name, false, c.TeamYAML(name, source.LayerGlobal))
	}
	rows := make([]Listing, 0, len(byName))
	for _, row := range byName {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// Definition reads a team definition by name. layer forces one layer ("local" or
// "global", for a screen that shows one); empty means the one way, local first.
func (c *Catalog) Definition(name, layer string) (*definition.Definition, error) {
	if name == "" {
		return c.defaultDefinition(layer)
	}
	return c.definitionIn(name, layer, ProfilesEdit{})
}

// DefinitionEdit reads a team definition with edit's profiles.yaml in place of
// what is on disk, with layer forced ("local" or "global"). It is how a screen
// checks an edit before it writes it (r-daf8).
func (c *Catalog) DefinitionEdit(name, layer string, edit ProfilesEdit) (*definition.Definition, error) {
	return c.definitionIn(name, layer, edit)
}

func (c *Catalog) definitionIn(name, layer string, edit ProfilesEdit) (*definition.Definition, error) {
	local := c.TeamYAML(name, source.LayerLocal)
	global := c.TeamYAML(name, source.LayerGlobal)
	hasLocal := isFile(local)
	hasGlobal := isFile(global)

	var chosen, other, chosenLayer string
	switch layer {
	case source.LayerLocal:
		if !hasLocal {
			return nil, i18n.Errorf("error.team_not_local", name, c.display(local), c.display(c.localDir))
		}
		chosen, chosenLayer = local, source.LayerLocal
	case source.LayerGlobal:
		if !hasGlobal {
			return nil, i18n.Errorf("error.team_not_global", name, c.display(global), c.display(c.globalDir))
		}
		chosen, chosenLayer = global, source.LayerGlobal
	default:
		switch {
		case hasLocal:
			chosen, chosenLayer = local, source.LayerLocal
		case hasGlobal:
			chosen, chosenLayer = global, source.LayerGlobal
		default:
			return nil, c.notFound(name, local, global)
		}
	}
	if chosen == local && hasGlobal {
		other = global
	}
	if chosen == global && hasLocal {
		other = local
	}
	return c.readDefinition(name, chosen, chosenLayer, other, edit)
}

// defaultDefinition is what a missing name does: the single local definition, or
// a list of candidates to choose from.
func (c *Catalog) defaultDefinition(layer string) (*definition.Definition, error) {
	switch layer {
	case source.LayerGlobal:
		names := teamNames(c.globalDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerGlobal, ProfilesEdit{})
		}
		return nil, c.candidates(i18n.Tf("error.global_count", len(names)))
	case source.LayerLocal:
		names := teamNames(c.localDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerLocal, ProfilesEdit{})
		}
		return nil, c.candidates(i18n.Tf("error.local_count", len(names)))
	default:
		names := teamNames(c.localDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerLocal, ProfilesEdit{})
		}
		return nil, c.candidates(i18n.Tf("error.need_team_name", len(names)))
	}
}

// readDefinition reads one team.yaml with the profiles its layer reads, and
// records the same-name team.yaml of the other layer for the note that says so.
func (c *Catalog) readDefinition(name, path, layer, other string, edit ProfilesEdit) (*definition.Definition, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	profiles, _, err := c.ProfilesEdit(edit)
	if err != nil {
		return nil, err
	}
	def, err := definition.Read(definition.Input{
		YAML:         string(text),
		Name:         name,
		Layer:        layer,
		TeamYAMLPath: path,
		TeamDir:      filepath.Dir(path),
		Profiles:     profiles,
		Base:         c.base,
		Reader:       c.reader,
	})
	if err != nil {
		return nil, err
	}
	def.OtherYAML = other
	return def, nil
}

func (c *Catalog) notFound(name, local, global string) error {
	var b strings.Builder
	fmt.Fprintf(&b, i18n.T("error.team_not_found"), name)
	fmt.Fprintf(&b, "\n  %s: %s", i18n.T("label.local"), c.display(local))
	fmt.Fprintf(&b, "\n  %s: %s", i18n.T("label.global"), c.display(global))
	if candidates := c.candidateNames(); len(candidates) > 0 {
		fmt.Fprintf(&b, "\n  %s: %s", i18n.T("label.candidates"), strings.Join(candidates, i18n.T("list.separator")))
		fmt.Fprintf(&b, "\n%s", i18n.Tf("next", nextCommand(candidates)))
	}
	return fmt.Errorf("%s", b.String())
}

// nextCommand is the command that moves a not-found error on: boot the one
// candidate, or list the teams when the choice is not obvious.
func nextCommand(candidates []string) string {
	if len(candidates) == 1 {
		return "up " + candidates[0]
	}
	return "ls"
}

func (c *Catalog) candidates(head string) error {
	list := c.candidateNames()
	if len(list) == 0 {
		return i18n.Errorf("error.no_candidates", head, c.display(c.globalDir), c.display(c.localDir))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s", head)
	fmt.Fprintf(&b, "\n  %s: %s", i18n.T("label.candidates"), strings.Join(list, i18n.T("list.separator")))
	fmt.Fprintf(&b, "\n%s", i18n.Tf("next", nextCommand(list)))
	return fmt.Errorf("%s", b.String())
}

// candidateNames is every team definition name visible in either layer.
func (c *Catalog) candidateNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, name := range append(teamNames(c.localDir), teamNames(c.globalDir)...) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// teamNames lists the team directories under a layer that have a team.yaml.
func teamNames(dir string) []string {
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

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ProfilesEdit is one layer's profiles.yaml as an edit that is not on disk yet:
// the file's path and the text a screen would write there. The zero value is
// "no edit", which is what every reader but a screen's check passes.
type ProfilesEdit struct {
	Path string
	Text string
}

// Profiles reads the profiles a team definition names: the global layer's
// profiles.yaml, with the local layer's laid over them (K1). The files read are
// returned local first. Either may be absent.
func (c *Catalog) Profiles() (map[string]profile.Profile, string, error) {
	return c.ProfilesEdit(ProfilesEdit{})
}

// ProfilesEdit is Profiles with one profiles.yaml replaced by an edit: the file
// at the edit's path answers with the edit's text whether or not it is on disk,
// so a layer whose profiles.yaml does not exist yet is built as if the screen
// had already written it.
func (c *Catalog) ProfilesEdit(edit ProfilesEdit) (map[string]profile.Profile, string, error) {
	profiles := map[string]profile.Profile{}
	var paths []string
	for _, dir := range []string{c.globalDir, c.localDir} {
		path := filepath.Join(dir, "profiles.yaml")
		var text string
		switch {
		case edit.Path != "" && path == edit.Path:
			text = edit.Text
		case isFile(path):
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, "", err
			}
			text = string(b)
		default:
			continue
		}
		more, err := profile.ParseProfiles(text, path)
		if err != nil {
			return nil, "", err
		}
		maps.Copy(profiles, more)
		paths = append([]string{path}, paths...)
	}
	return profiles, strings.Join(paths, ", "), nil
}

// Profile returns the named profile, and whether a team definition sees it.
func (c *Catalog) Profile(name string) (profile.Profile, bool) {
	profiles, _, err := c.Profiles()
	if err != nil {
		return profile.Profile{}, false
	}
	p, ok := profiles[name]
	return p, ok
}
