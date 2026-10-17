package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/homeview"
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
	cat := p.catalog()
	for _, row := range cat.Definitions() {
		if !inLayer(row, layer) {
			continue
		}
		r, err := validateTeam(cat, p.env.ProjectRoot, row.Name, layer)
		if err != nil {
			continue
		}
		out = append(out, p.ruleTeam(layer, row.Name, r))
	}
	return out
}

// Changes counts how many rules moved in one layer's documents since the launch
// each of them was last read by: the number the guide's last step names, counted
// the way the left list counts a document (a document that moved but whose rules
// could not be lined up counts once).
func (p servePort) Changes(layer string) int {
	total := 0
	for _, change := range p.docChanges(layer) {
		if change.whole {
			total++
			continue
		}
		total += change.count
	}
	return total
}

// rulesPort is the rules screen's port: the app behind every screen (servePort),
// with the two reads the rules screen makes in its own words. Everything else is
// the same read the other screens make, so the rules screen and the home screen
// cannot disagree about a document.
type rulesPort struct{ servePort }

// DocChanges is what the launch record says moved in one document since the
// launch that last read it, in the words the rules screen draws. It is the home
// screen's own comparison (p.Home), mapped: the two cannot disagree (d-0a25).
func (p rulesPort) DocChanges(layer, path string) rulesview.DocChanges {
	for _, doc := range p.Home(layer).Docs {
		if doc.File != path {
			continue
		}
		return rulesview.DocChanges{
			// A launch read the document when it carries the time and the
			// definition that launch wrote, the same way the home screen tells.
			Recorded: doc.Launched != "",
			Version:  doc.Version,
			Bumped:   doc.Bumped,
			Added:    changeRules(p, layer, doc.File, doc.Added),
			Removed:  changeRules(p, layer, doc.File, doc.Removed),
			Changed:  changeRules(p, layer, doc.File, doc.Changed),
			Whole:    doc.Whole,
			Missing:  doc.Missing,
		}
	}
	return rulesview.DocChanges{}
}

// changeRules maps one document's moved rules into the words the rules screen
// draws, each with where its own panel opens the rule.
func changeRules(p rulesPort, layer, path string, rules []homeview.Rule) []rulesview.ChangeRule {
	out := make([]rulesview.ChangeRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rulesview.ChangeRule{
			ID:      rule.ID,
			Section: rule.Section,
			Fields:  rule.Fields,
			Diffs:   changeParts(rule.Diffs),
			Href:    p.RuleHref(layer, path, rule.ID),
		})
	}
	return out
}

// changeParts is one moved rule's parts in the words the rules screen draws.
func changeParts(parts []homeview.Part) []rulesview.ChangePart {
	out := make([]rulesview.ChangePart, 0, len(parts))
	for _, part := range parts {
		out = append(out, rulesview.ChangePart{Before: part.Before, After: part.After})
	}
	return out
}

// Bump raises one document's version from the changes tab. It is the write the
// home screen's /bump makes, through the same checks (spec/10 の 7 節), answered
// in the words the rules screen draws: the version it moved from and to, or why
// nothing was written.
func (p rulesPort) Bump(layer, path string) (rulesview.BumpAnswer, error) {
	answer, err := p.servePort.Bump(layer, path)
	if err != nil {
		return rulesview.BumpAnswer{}, err
	}
	if !answer.Written {
		return rulesview.BumpAnswer{Why: answer.Why}, nil
	}
	return rulesview.BumpAnswer{Written: true, From: answer.Raised.From, To: answer.Raised.To}, nil
}

// ruleTeam is one team definition as the rules screen reads it. Each file
// carries the group its row in the left list falls into, so the screen's kicker
// and the left list say the same words for one group.
func (p servePort) ruleTeam(layer, name string, r *resolved) rulesview.Team {
	root := p.layerDir(layer)
	file := func(path string, role bool) rulesview.File {
		rel := relIn(root, path)
		group := groupOf(rel)
		name, _ := docName(rel, group)
		if group == groupTeam {
			// The row in the left list goes by the team's name alone; the
			// screen's heading needs the document to be named, so it says whose
			// document it is (試作 docLabel: "<チーム> チームの文書").
			name = i18n.Tf("web.rules.doc.team_name", name)
		}
		return rulesview.File{
			Path:       path,
			Display:    p.env.Display(path),
			Role:       role,
			Name:       name,
			GroupLabel: groupLabel(group),
		}
	}
	team := rulesview.Team{Name: name}
	for i := range r.starts {
		member := &r.starts[i]
		read := rulesview.Member{Name: member.Name}
		if member.HasRole {
			read.Files = append(read.Files, file(member.RoleDoc.Path, true))
		}
		for _, doc := range member.Read {
			read.Files = append(read.Files, file(doc.Path, false))
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
	if err := source.WriteIfUnchanged(path, hash, text); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return i18n.Errorf("error.rules_file_changed", p.env.Display(path))
		}
		return err
	}
	return nil
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
	cat := p.catalog()
	for _, row := range cat.Definitions() {
		if !inLayer(row, layer) {
			continue
		}
		def, err := cat.Definition(row.Name, layer)
		if err != nil {
			continue
		}
		// Only a team definition that reads this file can be affected by the
		// edit, and one that is broken on disk already says so elsewhere: the
		// preview speaks about this edit and nothing else.
		onDisk, err := buildResolved(cat, source.Disk{}, p.env.ProjectRoot, def, def.Name)
		if err != nil || !readsPath(onDisk, path) {
			continue
		}
		if _, err := buildResolved(cat, reader, p.env.ProjectRoot, def, def.Name); err != nil {
			out.Break = breakLine(p.env, path, err)
			return out, nil
		}
	}
	return out, nil
}

// readsPath reports whether a member of a built team reads one rule file.
func readsPath(r *resolved, path string) bool {
	for i := range r.starts {
		member := &r.starts[i]
		if member.HasRole && member.RoleDoc.Path == path {
			return true
		}
		for _, doc := range member.Read {
			if doc.Path == path {
				return true
			}
		}
	}
	return false
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
	cat := p.catalog()
	for _, row := range cat.Definitions() {
		if !inLayer(row, layer) {
			continue
		}
		r, err := validateTeam(cat, p.env.ProjectRoot, row.Name, layer)
		if err != nil {
			continue
		}
		if readsPath(r, path) {
			return true
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
func inLayer(row catalog.Listing, layer string) bool {
	if layer == web.LayerGlobal {
		return row.Global
	}
	return row.Local
}

// under reports whether path stands inside base.
func under(base, path string) bool {
	return strings.HasPrefix(path, strings.TrimSuffix(base, string(filepath.Separator))+string(filepath.Separator))
}

// hashText is the hash a screen carries from a read to a write. The one
// function is source.Hash (段 1a, C11); this name stays until the screens that
// call it are gone, so nothing has to change on either side yet.
func hashText(text string) string { return source.Hash(text) }
