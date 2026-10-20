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
	// inRepo says the project root is a git repository, which is what tells
	// NewPlace where something new goes (K2). It is the environment's fact
	// (team.Env), not something a path can be read back from, so the catalog is
	// told once, when it is built. inRepoKnown says whether it was told at all:
	// a catalog nobody told answers no place, rather than writing outside the
	// project by default (spec/38 T3).
	inRepo      bool
	inRepoKnown bool
	// overlays are the files the catalog reads as something else (C3b): the
	// path and the text that stands in for what is on disk. A screen reads its
	// own edit through With, so a text it has not written yet reads back. The
	// zero value reads every file from disk.
	overlays map[string]string
	// hidden are the paths the catalog reads as though they were not there
	// (C5): a screen checks a draft that removes a document, so the reading has
	// to see the removal before it is written. Without adds one.
	hidden map[string]bool
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

// InRepo returns a catalog that knows whether the project root is a git
// repository, which is the fact K2 reads: something new goes into the project's
// own layer inside one, and into the global layer outside one (team.Env.Catalog
// hands it the environment's answer). The receiver is left as it was; NewPlace
// answers nothing until a catalog has been told.
func (c *Catalog) InRepo(inRepo bool) *Catalog {
	out := *c
	out.inRepo = inRepo
	out.inRepoKnown = true
	return &out
}

// With returns a catalog that reads files as texts, whatever is on disk: it is
// how a screen reads a change it has not written yet, whether the file is a
// team.yaml, an agreement, a role document or a profiles.yaml (C3b). A file at
// any other path reads from disk, and a file that is not there yet reads as the
// text, exactly as an edit of a file that is not there yet.
//
// Calls add up: a screen that is checking two files at once (a definition's
// role changed, and the new role's document) overlays both, and both read back.
// Overlaying the same path twice takes the later text. The receiver is left as
// it was; the returned catalog carries the texts.
func (c *Catalog) With(path, text string) *Catalog {
	out := *c
	out.overlays = make(map[string]string, len(c.overlays)+1)
	maps.Copy(out.overlays, c.overlays)
	out.overlays[filepath.Clean(path)] = text
	out.reader = overlayReader{base: c.reader, overlays: out.overlays, hidden: c.hidden}
	return &out
}

// Without returns a catalog that reads one path as though the file were not
// there (C5): a screen checks a draft that removes a document, and the reading
// has to see the removal before it is written. A text laid over the same path
// by With is dropped, because a file cannot be both there and not there.
// Everything else reads as before, and the receiver is left as it was.
func (c *Catalog) Without(path string) *Catalog {
	out := *c
	clean := filepath.Clean(path)
	out.hidden = make(map[string]bool, len(c.hidden)+1)
	maps.Copy(out.hidden, c.hidden)
	out.hidden[clean] = true
	if _, overlaid := c.overlays[clean]; overlaid {
		out.overlays = make(map[string]string, len(c.overlays))
		maps.Copy(out.overlays, c.overlays)
		delete(out.overlays, clean)
	}
	out.reader = overlayReader{base: c.reader, overlays: out.overlays, hidden: out.hidden}
	return &out
}

// Reader is the reader the catalog reads through: the disk, or, after With, the
// disk with the texts the catalog carries laid over it (C3b). A caller that
// reads a rule document's text to check an edit hands this to whatever reads it
// (the prompt.Run.Reader of a preview, the check), so the text that is read is
// the text the screen is editing.
func (c *Catalog) Reader() source.Reader { return c.reader }

// readThrough is the text an overlay carries for a path, if one does. A hidden
// path carries none: it reads as absent (C5).
func (c *Catalog) readThrough(path string) (string, bool) {
	if c.hidden[filepath.Clean(path)] {
		return "", false
	}
	text, ok := c.overlays[filepath.Clean(path)]
	return text, ok
}

// overlayReader is a reader with the files a catalog carries answered by their
// texts rather than by disk (C3b), and the files it hides answered as absent
// (C5): the files the screen is editing, and the one it is removing.
type overlayReader struct {
	base     source.Reader
	overlays map[string]string
	hidden   map[string]bool
}

func (r overlayReader) text(path string) (string, bool) {
	if r.hidden[filepath.Clean(path)] {
		return "", false
	}
	text, ok := r.overlays[filepath.Clean(path)]
	return text, ok
}

// gone reports whether a path reads as absent (C5).
func (r overlayReader) gone(path string) bool { return r.hidden[filepath.Clean(path)] }

func (r overlayReader) Read(d source.Doc) (string, error) {
	if r.gone(d.Path) {
		return "", os.ErrNotExist
	}
	if text, ok := r.text(d.Path); ok {
		return text, nil
	}
	return r.base.Read(d)
}

func (r overlayReader) IsFile(d source.Doc) bool {
	if r.gone(d.Path) {
		return false
	}
	if _, ok := r.text(d.Path); ok {
		return true
	}
	return r.base.IsFile(d)
}

func (r overlayReader) IsDir(d source.Doc) bool {
	if r.gone(d.Path) {
		return false
	}
	if _, ok := r.text(d.Path); ok {
		return false
	}
	return r.base.IsDir(d)
}

func (r overlayReader) YAMLIn(d source.Doc) ([]source.Doc, error) {
	if r.gone(d.Path) {
		return nil, nil
	}
	if _, ok := r.text(d.Path); ok {
		return nil, nil
	}
	return r.base.YAMLIn(d)
}

func (r overlayReader) MarkdownIn(d source.Doc) ([]source.Doc, error) {
	if r.gone(d.Path) {
		return nil, nil
	}
	if _, ok := r.text(d.Path); ok {
		return nil, nil
	}
	return r.base.MarkdownIn(d)
}

// readText reads one settings file, answering a carried text when there is one
// for this path (C3b). A file that is not there is an error, as it is for a read
// from disk.
func (c *Catalog) readText(path string) (string, error) {
	if c.hidden[filepath.Clean(path)] {
		return "", os.ErrNotExist
	}
	if text, ok := c.readThrough(path); ok {
		return text, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// hasFile reports whether a settings file is there to read: on disk, or as a
// carried text standing in for it (C3b). A hidden path is not there (C5).
func (c *Catalog) hasFile(path string) bool {
	if c.hidden[filepath.Clean(path)] {
		return false
	}
	if _, ok := c.readThrough(path); ok {
		return true
	}
	return isFile(path)
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

// Base is the project root the catalog resolves against: the folder cwd values
// and a member's embedded hire command are written from.
func (c *Catalog) Base() string { return c.base }

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
// It is the read validate and the screens that show one layer take; a screen
// reads by name alone (Read). A definition whose turns cannot hold does not read
// (I12a, I12b): it stops here, the way a file that does not read does.
// DefinitionForEffect is the one reading that takes such a definition.
func (c *Catalog) Definition(name, layer string) (*definition.Definition, error) {
	return c.definition(name, layer, readStops)
}

// DefinitionForEffect reads a team definition the way a screen's card needs it:
// one whose turns cannot hold is read with its graph rather than refused, because
// the parts of a prompt a change would move are readable from a graph that stops
// the definition (spec-teams 1.4.4, 監査 R11). check.TeamEffect reads the two
// views of a change through this, so a card shows the line a change takes away
// even when the change itself is what stops the definition. Every command, and
// every other reading of a definition, goes through Definition, which refuses.
func (c *Catalog) DefinitionForEffect(name, layer string) (*definition.Definition, error) {
	return c.definition(name, layer, readForEffect)
}

// readMode is which reading a caller needs of a definition whose turns cannot
// hold: readStops refuses it, the way every command is stopped (I12a, I12b), and
// readForEffect hands it back with its graph, for the parts of a prompt a change
// moves (監査 R11).
type readMode int

const (
	readStops readMode = iota
	readForEffect
)

func (c *Catalog) definition(name, layer string, mode readMode) (*definition.Definition, error) {
	if name == "" {
		return c.defaultDefinition(layer, mode)
	}
	return c.definitionIn(name, layer, mode)
}

func (c *Catalog) definitionIn(name, layer string, mode readMode) (*definition.Definition, error) {
	local := c.TeamYAML(name, source.LayerLocal)
	global := c.TeamYAML(name, source.LayerGlobal)
	hasLocal := c.hasFile(local)
	hasGlobal := c.hasFile(global)

	var chosen, chosenLayer string
	switch layer {
	case source.LayerLocal:
		if !hasLocal {
			return nil, c.missing(name)
		}
		chosen, chosenLayer = local, source.LayerLocal
	case source.LayerGlobal:
		if !hasGlobal {
			return nil, c.missing(name)
		}
		chosen, chosenLayer = global, source.LayerGlobal
	default:
		switch {
		case hasLocal:
			chosen, chosenLayer = local, source.LayerLocal
		case hasGlobal:
			chosen, chosenLayer = global, source.LayerGlobal
		default:
			return nil, c.missing(name)
		}
	}
	return c.readDefinition(name, chosen, chosenLayer, mode)
}

// defaultDefinition is what a missing name does: the single local definition, or
// a list of candidates to choose from.
func (c *Catalog) defaultDefinition(layer string, mode readMode) (*definition.Definition, error) {
	switch layer {
	case source.LayerGlobal:
		names := teamNames(c.globalDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerGlobal, mode)
		}
	case source.LayerLocal:
		names := teamNames(c.localDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerLocal, mode)
		}
	default:
		names := teamNames(c.localDir)
		if len(names) == 1 {
			return c.definitionIn(names[0], source.LayerLocal, mode)
		}
	}
	return nil, c.candidates(layer)
}

// readDefinition reads one team.yaml with the profiles its layer reads.
func (c *Catalog) readDefinition(name, path, layer string, mode readMode) (*definition.Definition, error) {
	text, err := c.readText(path)
	if err != nil {
		return nil, &ReadError{Path: path, Err: err}
	}
	// profiles.yaml must read before a definition does, so a team file is not
	// read against names a broken profiles file hides. The names a definition
	// writes are the check's business, not the reading's (spec-teams 1.9).
	if _, _, err := c.Profiles(); err != nil {
		return nil, err
	}
	def, err := definition.Read(definition.Input{
		YAML:         string(text),
		Name:         name,
		Layer:        layer,
		TeamYAMLPath: path,
		TeamDir:      filepath.Dir(path),
		Base:         c.base,
		Reader:       c.reader,
	})
	if err != nil {
		var gerr *definition.GraphError
		if mode == readForEffect && def != nil && errors.As(err, &gerr) {
			// The turns cannot hold, so every command stops on the sentence
			// (I12a, I12b). The definition is handed back all the same, because
			// the parts of a prompt a change moves are readable from it (監査
			// R11).
			return def, nil
		}
		return nil, &ReadError{Path: path, Err: err}
	}
	return def, nil
}

// missing is the one not-found message: the name, every place it was looked for
// (the project first, then outside it), and the names that are there to choose
// from. The layer is not a word a person reads (I11).
func (c *Catalog) missing(name string) error {
	places := strings.Join([]string{
		c.display(c.TeamYAML(name, source.LayerLocal)),
		c.display(c.TeamYAML(name, source.LayerGlobal)),
	}, i18n.T("list.separator"))
	candidates := c.candidateNames()
	var msg string
	if len(candidates) == 0 {
		msg = i18n.Tf("error.team_not_found", name, places)
	} else {
		msg = i18n.Tf("error.team_not_found_candidates", name, places, strings.Join(candidates, i18n.T("list.separator")))
		msg += "\n" + i18n.Tf("next", nextCommand(candidates))
	}
	return notFoundError{msg}
}

// nextCommand is the command that moves a not-found error on: boot the one
// candidate, or list the teams when the choice is not obvious.
func nextCommand(candidates []string) string {
	if len(candidates) == 1 {
		return "up " + candidates[0]
	}
	return "ls"
}

// candidates is what a missing name does when several definitions are there:
// name one, with the ones there to choose from. The count is of the definitions
// a name would read (the project's), so it can be 0 while the list is not: a name
// outside the project is a candidate, but a name that is omitted does not use it
// (D1, spec/35).
func (c *Catalog) candidates(layer string) error {
	list := c.candidateNames()
	if len(list) == 0 {
		return notFoundError{i18n.T("error.team_count_none") + "\n" + i18n.Tf("next", "new <name>")}
	}
	dir := c.localDir
	if layer == source.LayerGlobal {
		dir = c.globalDir
	}
	var b strings.Builder
	fmt.Fprint(&b, i18n.Tf("error.team_count_candidates", len(teamNames(dir)), strings.Join(list, i18n.T("list.separator"))))
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

// ProjectProfileFile is the project's own profiles.yaml, the one a team
// definition in the project reads first (K1). A screen shows and writes it as
// "プロジェクト".
func (c *Catalog) ProjectProfileFile() string {
	return filepath.Join(c.localDir, "profiles.yaml")
}

// SharedProfileFile is the profiles.yaml outside the project, read by every
// definition that the project's own does not hide (K1). A screen shows and
// writes it as "共通".
func (c *Catalog) SharedProfileFile() string {
	return filepath.Join(c.globalDir, "profiles.yaml")
}

// Profiles reads the profiles a team definition names: the global layer's
// profiles.yaml, with the local layer's laid over them (K1). The files read are
// returned local first. Either may be absent.
func (c *Catalog) Profiles() (map[string]profile.Profile, string, error) {
	profiles := map[string]profile.Profile{}
	var paths []string
	for _, dir := range []string{c.globalDir, c.localDir} {
		path := filepath.Join(dir, "profiles.yaml")
		text, ok, err := c.fileText(path)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			continue
		}
		// The reading is told how a person reads the file, so a sentence about
		// one that does not read names the place a person fixes, not the
		// machine's path (X1). Each profile keeps the file's own path: hergy
		// reads and writes the file it found.
		more, err := profile.ParseProfiles(text, c.Display(path))
		if err != nil {
			return nil, "", err
		}
		for name, one := range more {
			one.Path = path
			more[name] = one
		}
		maps.Copy(profiles, more)
		paths = append([]string{path}, paths...)
	}
	return profiles, strings.Join(paths, ", "), nil
}

// fileText is one settings file's text: the carried text's when there is one for
// the path (C3b), the file's when it is there, and ok=false when there is
// nothing to read.
func (c *Catalog) fileText(path string) (text string, ok bool, err error) {
	if c.hidden[filepath.Clean(path)] {
		return "", false, nil
	}
	if text, carried := c.readThrough(path); carried {
		return text, true, nil
	}
	if !isFile(path) {
		return "", false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
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

// RolePlaces is every path a role document is looked for, in the one order a
// name is looked up (K1): the project's layer first, then the global one. It is
// what the sentence a missing role document gets names, so a person moving a
// definition over sees the whole list rather than one place that may already
// hold the file.
func (c *Catalog) RolePlaces(name string) []string {
	out := make([]string, 0, len(c.layers()))
	for _, one := range c.layers() {
		out = append(out, filepath.Join(one.dir, "roles", name+".yaml"))
	}
	return out
}

// Display is how a path is shown to a person; the app hands the catalog the
// environment's own. A caller that builds a sentence of its own asks for it
// here, so a path reads the same wherever hergy prints it.
func (c *Catalog) Display(path string) string { return c.display(path) }

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
