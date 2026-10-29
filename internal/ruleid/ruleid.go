// Package ruleid gives hergy's rules their ids (d-4aab). A person and a screen
// write no id: every rule document hergy reads may leave it out, and the next
// write gives it one that no rule of this project and no launch record holds.
// The id is for telling one rule from another, and for nothing else.
//
// The package reads every agreement.yaml and role document of both layers, so an
// id is chosen against all of them at once; a file it cannot read is left alone,
// because a write must not stop on the file whose broken shape another check
// names.
package ruleid

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// File is one rule document on disk, read as it stands.
type File struct {
	Path    string
	Display string
	Screen  rules.SaveScreen
	Text    string
	Saved   *rules.Document
}

// Files is every agreement.yaml and role document of both layers, in the order
// the project's layer comes before the shared one. A file that does not read is
// left out: another check says why, and assigning ids is not the place to stop.
func Files(cat *catalog.Catalog) []File {
	var out []File
	for _, layer := range []string{source.LayerLocal, source.LayerGlobal} {
		dir := cat.LayerDir(layer)
		for _, pattern := range []string{
			filepath.Join(dir, "teams", "*", "agreement.yaml"),
			filepath.Join(dir, "roles", "*.yaml"),
		} {
			paths, _ := filepath.Glob(pattern)
			for _, path := range paths {
				file := read(cat, path)
				if file.Saved != nil {
					out = append(out, file)
				}
			}
		}
	}
	return out
}

// read is one file parsed, with the screen a write of it comes from.
func read(cat *catalog.Catalog, path string) File {
	text, err := source.Disk{}.Read(source.Doc{Path: path})
	if err != nil {
		return File{}
	}
	screen := rules.SaveFromTeamDefinition
	if filepath.Base(filepath.Dir(path)) == "roles" {
		screen = rules.SaveFromRoles
	}
	display := cat.Display(path)
	doc, _ := rules.ReadAll(text, display)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	return File{Path: path, Display: display, Screen: screen, Text: text, Saved: doc}
}

// Missing is how many rules hergy has not given an id yet: a rule with no id at
// all. A rule with an id in an older shape has one, and is written again with
// hergy's on the next write without being counted here. A read-only operation
// says the number; the next write gives them one.
func Missing(cat *catalog.Catalog) int {
	n := 0
	for _, file := range Files(cat) {
		for _, rule := range file.Saved.Rules() {
			if rule.ID == "" {
				n++
			}
		}
	}
	return n
}

// Taken is every id a rule hergy reads holds, and every id the launch record
// holds: an id of a rule that was taken out is not given again, so the apply's
// changed list and a rule's off day keep their meaning.
func Taken(cat *catalog.Catalog) map[string]bool {
	taken := map[string]bool{}
	for _, file := range Files(cat) {
		for _, rule := range file.Saved.Rules() {
			if rule.ID != "" {
				taken[rule.ID] = true
			}
		}
	}
	for id := range recordedIDs() {
		taken[id] = true
	}
	return taken
}

// recordedIDs is the ids the launch record's texts hold: a prompt writes a rule
// as "**ID** text", and a rule document writes "id: ID".
func recordedIDs() map[string]bool {
	out := map[string]bool{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	entries, err := os.ReadDir(filepath.Join(log.Dir(), "blobs"))
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		text, err := os.ReadFile(filepath.Join(log.Dir(), "blobs", entry.Name()))
		if err != nil {
			continue
		}
		for _, re := range idInText {
			for _, match := range re.FindAllSubmatch(text, -1) {
				id := string(match[1])
				if rules.RuleIDPattern.MatchString(id) {
					out[id] = true
				}
			}
		}
	}
	return out
}

// idInText is where an id stands in a recorded text: the prompt's bold mark and
// the rule document's own key.
var idInText = []*regexp.Regexp{
	regexp.MustCompile(`\*\*([A-HJKMNP-TV-Z][A-HJKMNP-TV-Z0-9]{3})\*\*`),
	regexp.MustCompile(`(?m)^\s*id:\s*([A-Za-z0-9-]+)\s*$`),
}

// Assign gives an id to every rule that has none in hergy's shape, writes the
// new ids into every rule text that names an old one, and writes the files it
// changed. It answers how many rules it gave an id.
func Assign(cat *catalog.Catalog) (int, error) {
	files := Files(cat)
	taken := Taken(cat)
	ids := make([]map[rules.Place]string, len(files))
	renames := map[string]string{}
	given := 0
	for i := range files {
		ids[i] = map[rules.Place]string{}
		for si := range files[i].Saved.Sections {
			for ri := range files[i].Saved.Sections[si].Rules {
				rule := &files[i].Saved.Sections[si].Rules[ri]
				if !rules.NeedsRuleID(rule.ID) {
					taken[rule.ID] = true
					continue
				}
				next := rules.NewRuleID(taken)
				if next == "" {
					return given, fmt.Errorf("no rule id is free")
				}
				taken[next] = true
				if rule.ID != "" {
					renames[rule.ID] = next
				}
				ids[i][rules.Place{Section: si, Rule: ri}] = next
				given++
			}
		}
	}
	if given == 0 {
		return 0, nil
	}
	for i := range files {
		edited, err := rules.ApplyIDs([]byte(files[i].Text), files[i].Path, renames, ids[i])
		if err != nil {
			return given, err
		}
		if string(edited) == files[i].Text {
			continue
		}
		if err := source.WriteIfUnchanged(files[i].Path, source.Hash(files[i].Text), string(edited)); err != nil {
			return given, err
		}
	}
	return given, nil
}

// Print writes the one line a write operation says when it gave ids.
func Print(out io.Writer, given int) {
	if given == 0 {
		return
	}
	fmt.Fprintln(out, i18n.Tf("rules.ids_given", given))
}
