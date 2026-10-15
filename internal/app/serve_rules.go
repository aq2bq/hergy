package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/rulesview"
)

// The rules screen's half of the port (r-1df6). The screen reads and writes
// nothing but through these, so the promises of spec/10 の 4 節 are kept in one
// place: WriteRule is the only way a page changes a file, and it refuses a path
// no member of the layer reads and a file that moved under the screen.

// RuleTeams lists one layer's team definitions with the rule files each member
// reads, the role file first and then the documents, the way a prompt is built.
// A definition that cannot be read is left out: the rules screen shows what it
// can read.
func (p servePort) RuleTeams(layer string) []rulesview.Team {
	var out []rulesview.Team
	for _, row := range p.env.List() {
		if !inLayer(row, layer) {
			continue
		}
		cfg, err := validateTeam(p.env, row.Name, layer)
		if err != nil {
			continue
		}
		out = append(out, ruleTeam(row.Name, cfg))
	}
	return out
}

// ruleTeam is one team definition as the rules screen reads it.
func ruleTeam(name string, cfg *config.Config) rulesview.Team {
	team := rulesview.Team{Name: name}
	for i := range cfg.Members {
		member := &cfg.Members[i]
		read := rulesview.Member{Name: member.Name}
		if member.HasRoleSource {
			read.Files = append(read.Files, rulesview.File{Path: member.RoleSource.Path, Role: true})
		}
		for _, doc := range member.DocsRead {
			read.Files = append(read.Files, rulesview.File{Path: doc.Path})
		}
		team.Members = append(team.Members, read)
	}
	return team
}

// ReadRule reads one rule file and hashes what it read. The same bytes give the
// same hash, so a save can tell whether the file moved under the screen.
func (p servePort) ReadRule(path string) (rulesview.Source, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return rulesview.Source{}, err
	}
	return rulesview.Source{Path: path, Text: string(text), Hash: hashText(string(text))}, nil
}

// WriteRule writes one rule file back. It refuses a path that is not a rule file
// a member of this layer's team definitions reads, and a file whose content is
// not the one that was read. The write goes to a file beside the target and is
// renamed over it, so a reader never sees half a file.
func (p servePort) WriteRule(path, text, hash string) error {
	layer := p.layerOf(path)
	if !p.reads(layer, path) {
		return i18n.Errorf("error.rules_not_writable", p.env.Display(path))
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if hashText(string(before)) != hash {
		return i18n.Errorf("error.rules_file_changed", p.env.Display(path))
	}
	return writeFileBeside(path, text)
}

// Preview is the edited rule as a member would read it: the section it stands
// in, and the break the reader names. The edited text stands in for the file on
// disk and everything else is read as it is, so the rest of the prompt and the
// checks are what hergy up would build (spec/10 の 3 節).
func (p servePort) Preview(layer, path string, section int, text string) (rulesview.Preview, error) {
	document, err := rules.Parse(text, path)
	if err != nil {
		return rulesview.Preview{Break: breakLine(p.env, path, err)}, nil
	}
	out := rulesview.Preview{}
	if section >= 0 && section < len(document.Sections) {
		out.Section, _ = document.SectionMarkdown(section)
	}
	reader := editing{base: source.Disk{}, path: path, text: text}
	fallback := ""
	for _, row := range p.env.List() {
		if !inLayer(row, layer) {
			continue
		}
		resolved, err := p.env.Resolve(row.Name, layer)
		if err != nil {
			continue
		}
		if _, err := p.env.BuildConfig(resolved, reader, ""); err != nil {
			// A break that names the file being edited is this edit's; one that
			// does not is kept only in case nothing names it.
			if strings.Contains(err.Error(), path) {
				out.Break = breakLine(p.env, path, err)
				return out, nil
			}
			if fallback == "" {
				fallback = breakLine(p.env, path, err)
			}
		}
	}
	out.Break = fallback
	return out, nil
}

// editing is a reader with one file replaced: the file being edited answers
// with the text in the form, and every other file is read as it is. This is the
// Reader spec/10 の 3 節 asks for, so an edit is checked the way hergy up checks
// it, without writing anything.
type editing struct {
	base source.Reader
	path string
	text string
}

func (e editing) Read(d source.Doc) (string, error) {
	if d.Path == e.path {
		return e.text, nil
	}
	return e.base.Read(d)
}

func (e editing) IsFile(d source.Doc) bool {
	if d.Path == e.path {
		return true
	}
	return e.base.IsFile(d)
}

func (e editing) IsDir(d source.Doc) bool { return e.base.IsDir(d) }

func (e editing) YAMLIn(d source.Doc) ([]source.Doc, error) { return e.base.YAMLIn(d) }

func (e editing) MarkdownIn(d source.Doc) ([]source.Doc, error) { return e.base.MarkdownIn(d) }

// reads reports whether a member of one layer's team definitions reads a rule
// file: the range the rules screen may write in.
func (p servePort) reads(layer, path string) bool {
	for _, row := range p.env.List() {
		if !inLayer(row, layer) {
			continue
		}
		cfg, err := validateTeam(p.env, row.Name, layer)
		if err != nil {
			continue
		}
		for i := range cfg.Members {
			member := &cfg.Members[i]
			if member.HasRoleSource && member.RoleSource.Path == path {
				return true
			}
			for _, doc := range member.DocsRead {
				if doc.Path == path {
					return true
				}
			}
		}
	}
	return false
}

// layerOf is the layer a file belongs to, read from where it stands.
func (p servePort) layerOf(path string) string {
	if under(p.env.GlobalDir, path) {
		return web.LayerGlobal
	}
	return web.LayerLocal
}

// inLayer reports whether a listing row belongs to one layer.
func inLayer(row team.Listing, layer string) bool {
	if layer == web.LayerGlobal {
		return row.Global
	}
	return row.Local
}

// under reports whether path stands inside base.
func under(base, path string) bool {
	return strings.HasPrefix(path, strings.TrimSuffix(base, string(filepath.Separator))+string(filepath.Separator))
}

// hashText is the hash the screen carries from a read to a write.
func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// writeFileBeside writes text to a file in the same directory and renames it
// over path, so the file is either the old one or the new one, never half of
// either. The temp file is 0644 like the file it replaces: hergy's files are
// read by the person who runs it.
func writeFileBeside(path, text string) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	// A temp file that does not reach the rename is removed. A removal that
	// fails has nothing to say: the target file is untouched either way.
	defer func() {
		if name != "" {
			_ = os.Remove(name)
		}
	}()
	if _, err := temp.WriteString(text); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	name = "" // renamed: there is nothing left to remove
	return nil
}
