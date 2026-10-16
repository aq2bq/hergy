// Package catalog is the one place a settings name is looked up: a team
// definition and a profile. The layers are hers (I11): a name hergy resolves
// goes local first, then global, and the layer never leaves this package.
//
// A team definition of the global layer reads the project's profiles too (K1):
// there is one way to a name, so a definition carried from one project to
// another reads what the project it stands in carries.
package catalog

import (
	"errors"
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

// ErrNotFound says a team definition name has none: in the layer asked for, or
// in either one. A caller that has to tell "no such definition" from "a
// definition that does not read" asks errors.Is.
var ErrNotFound = errors.New("no team definition")

// notFoundError is a not-found message: the text a person reads, and the
// sentinel ErrNotFound for such a caller. The sentinel is not in the text.
type notFoundError struct{ message string }

func (e notFoundError) Error() string        { return e.message }
func (e notFoundError) Is(target error) bool { return target == ErrNotFound }

// ReadError is a team.yaml that is there but does not read: which file, and why.
// A caller that shows the reason where a person reads it (status, for a running
// team whose definition does not read) takes the path from here.
type ReadError struct {
	Path string
	Err  error
}

func (e *ReadError) Error() string { return e.Err.Error() }
func (e *ReadError) Unwrap() error { return e.Err }

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
			return nil, notFoundError{i18n.Errorf("error.team_not_local", name, c.display(local), c.display(c.localDir)).Error()}
		}
		chosen, chosenLayer = local, source.LayerLocal
	case source.LayerGlobal:
		if !hasGlobal {
			return nil, notFoundError{i18n.Errorf("error.team_not_global", name, c.display(global), c.display(c.globalDir)).Error()}
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
		return nil, &ReadError{Path: path, Err: err}
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
		return nil, &ReadError{Path: path, Err: err}
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
	return notFoundError{b.String()}
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
	return notFoundError{b.String()}
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

// Role is one member's role document by role name: roles/<name>.yaml in the
// project's layer, else in the global layer's. It is the one way to a name (K1),
// the same one a profile takes. The second return is whether a layer has one; a
// run stops when it has none, and prompt.Member says so with the member's name.
func (c *Catalog) Role(name string) (source.Doc, bool) {
	return c.firstFile(filepath.Join("roles", name+".yaml"))
}

// RoleMarkdown is a role document still left as Markdown (roles/<name>.md): it
// is what the sentence that says how to move the file over names. It is asked
// only when Role found nothing.
func (c *Catalog) RoleMarkdown(name string) (source.Doc, bool) {
	return c.firstFile(filepath.Join("roles", name+".md"))
}

// SharedAgreement is the agreement every team of a layer shares: agreement.yaml
// in the project's layer, else in the global layer's. There is one, not a list
// (d-0ccd), and writing none is allowed (段 4); the second return says whether a
// layer wrote one.
func (c *Catalog) SharedAgreement() (source.Doc, bool) {
	return c.firstFile("agreement.yaml")
}

// firstFile is the first layer that holds one file, in the one order a name is
// looked up: the project's layer first, then the global one (K1).
func (c *Catalog) firstFile(rel string) (source.Doc, bool) {
	for _, one := range c.layers() {
		doc := source.Doc{Layer: one.name, Path: filepath.Join(one.dir, rel)}
		if c.reader.IsFile(doc) {
			return doc, true
		}
	}
	return source.Doc{}, false
}

// layerDir is one layer with its name, as the one order of a lookup walks them.
type layerDir struct{ dir, name string }

// layers is the two layers in the one order a name is looked up: local, then
// global (K1).
func (c *Catalog) layers() []layerDir {
	return []layerDir{
		{dir: c.localDir, name: source.LayerLocal},
		{dir: c.globalDir, name: source.LayerGlobal},
	}
}
