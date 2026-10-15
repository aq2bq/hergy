package app

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/rulesview"
)

// The frame's own half of the port (d-207f): the left list of a layer, and the
// three counts of the header. Both read the same documents the rules screen
// reads, so the list, the meters and the rules screen never disagree about how
// many rules the layer holds.
//
// The list shows the documents in the groups the prototype groups them in
// (common/, the documents of each team, roles/), and every document counts once
// however many team definitions read it.

// The groups a document falls into, as the left list and the prototype name
// them: a document every member reads, a document of one team, or a role file.
const (
	groupCommon = "common"
	groupTeam   = "team"
	groupRole   = "role"
)

// roleAtEnd is the path of a role file inside a team's own directory
// ("teams/demo/roles/lead.yaml"), which the prototype counts as a role file
// too.
var roleAtEnd = regexp.MustCompile(`/roles/[^/]+\.ya?ml$`)

// groupOf is which group one document falls into, told from where it stands in
// its layer ("common/00_TEAM_PROTOCOL.yaml", "teams/demo/protocol.yaml").
func groupOf(rel string) string {
	switch {
	case strings.HasPrefix(rel, "common/"):
		return groupCommon
	case strings.HasPrefix(rel, "roles/"), roleAtEnd.MatchString(rel):
		return groupRole
	}
	return groupTeam
}

// Index is the left list of one layer: its team definitions, the documents they
// read, grouped, and how many profiles the layer holds.
func (p servePort) Index(layer string) web.Index {
	defs := layerDefinitions(p.env, layer)
	out := web.Index{
		Teams:    p.indexTeams(layer, defs),
		Profiles: p.profileCount(layer),
	}
	byGroup := map[string][]web.IndexRow{}
	for _, doc := range p.layerDocs(layer, defs) {
		group := doc.group
		byGroup[group] = append(byGroup[group], doc.row(layer))
	}
	for _, group := range []string{groupCommon, groupTeam, groupRole} {
		rows := byGroup[group]
		if len(rows) == 0 {
			continue
		}
		out.Groups = append(out.Groups, web.IndexGroup{Label: groupLabel(group), Rows: rows})
	}
	return out
}

// groupLabel is one group's heading, in the language in use.
func groupLabel(group string) string {
	return i18n.T("web.frame.index.group." + group)
}

// indexTeams is the team definitions of one layer, read the way the team list
// reads them: a definition that cannot be read keeps its row with the reason.
func (p servePort) indexTeams(layer string, defs []layerDef) []web.IndexTeam {
	read := map[string]*config.Config{}
	for _, def := range defs {
		read[def.name] = def.cfg
	}
	var out []web.IndexTeam
	for _, row := range p.env.List() {
		if !inLayer(row, layer) {
			continue
		}
		entry := web.IndexTeam{
			Name: row.Name,
			Href: teamHref(layer, row.Name),
		}
		if layer == web.LayerGlobal {
			entry.Shadowed = localDefinition(p.env, row.Name)
		}
		if cfg, ok := read[row.Name]; ok {
			entry.Members = len(cfg.Members)
			out = append(out, entry)
			continue
		}
		// The definition is listed either way: the left list is the way into
		// the screen that says why it cannot be read.
		if _, err := validateTeam(p.env, row.Name, layer); err != nil {
			entry.Broken = breakLine(p.env, listingPath(row, layer), err)
		}
		out = append(out, entry)
	}
	return out
}

// localDefinition reports whether the local layer holds a definition of one
// name: a global definition of that name is the one a local one takes the place
// of (the local one is what hergy up boots).
func localDefinition(env team.Env, name string) bool {
	for _, row := range env.List() {
		if row.Name == name && row.Local {
			return true
		}
	}
	return false
}

// Meters counts every rule of one layer's documents, once each, the way the
// prototype's header counts them: a rule with no origin, no reason or no check
// is not counted in that meter.
func (p servePort) Meters(layer string) web.Meters {
	var out web.Meters
	for _, doc := range p.layerDocs(layer, layerDefinitions(p.env, layer)) {
		out.Rules += doc.tally.Total
		out.Origin += doc.tally.WithOrigin
		out.Reason += doc.tally.WithWhy
		out.Check += doc.tally.WithCheck
	}
	return out
}

// layerDoc is one document of a layer, read the way the left list and the
// meters need it: where it stands, which group it belongs to, what it holds, and
// what the launch record says moved in it.
type layerDoc struct {
	doc   source.Doc
	group string
	// name is what the row shows: the team's name, the role file's name, or
	// the document's own title.
	name   string
	mono   bool
	tally  rules.Tally
	broken bool
	// changed is how many rules moved since the newest launch that read the
	// document, and whole says the document moved but the rules could not be
	// lined up, so the row says "changed" instead of a number.
	changed int
	whole   bool
}

// row turns one document into a row of the left list.
func (d layerDoc) row(layer string) web.IndexRow {
	return web.IndexRow{
		Name:    d.name,
		Mono:    d.mono,
		Path:    d.doc.Path,
		Rules:   d.tally.Total,
		Origin:  d.tally.WithOrigin,
		Changed: d.changed,
		Whole:   d.whole,
		Href:    rulesHref(layer, d.doc.Path),
		Broken:  d.broken,
	}
}

// layerDocs reads one layer's documents once each, parses them, and matches each
// with what the launch record says moved in it. A document that cannot be read
// keeps its row and its group, so one broken file hides nothing.
func (p servePort) layerDocs(layer string, defs []layerDef) []layerDoc {
	changes := p.docChanges(layer)
	root := p.layerDir(layer)
	out := make([]layerDoc, 0, len(defs))
	for _, doc := range documentsOf(defs) {
		rel := relIn(root, doc.Path)
		entry := layerDoc{
			doc:   doc,
			group: groupOf(rel),
		}
		entry.name, entry.mono = docName(rel, entry.group)
		text, err := os.ReadFile(doc.Path)
		if err != nil {
			entry.broken = true
		} else if parsed, err := rules.Parse(string(text), doc.Path); err != nil {
			entry.broken = true
		} else {
			entry.tally.Add(parsed)
			if entry.group == groupCommon && parsed.Title != "" {
				entry.name = parsed.Title
			}
		}
		if changed, ok := changes[doc.Path]; ok {
			entry.changed, entry.whole = changed.count, changed.whole
		}
		out = append(out, entry)
	}
	return out
}

// docChange is what the record says moved in one document: how many rules, or
// that it moved without the rules being lineable up.
type docChange struct {
	count int
	whole bool
}

// docChanges is what moved in each of one layer's documents since the newest
// launch that read it, by path. It is the home screen's comparison, read the
// same way, so the left list and the home screen say the same thing.
func (p servePort) docChanges(layer string) map[string]docChange {
	out := map[string]docChange{}
	for _, doc := range p.Home(layer).Docs {
		if !doc.Changes() {
			continue
		}
		out[doc.File] = docChange{
			count: len(doc.Added) + len(doc.Removed) + len(doc.Changed),
			whole: doc.Whole || doc.Missing,
		}
	}
	return out
}

// docName is what one document's row shows: the name of the team whose document
// it is, the name of the role file, or the document's own title (which the
// caller fills in when it reads one, and which falls back to the file's name).
func docName(rel, group string) (name string, mono bool) {
	switch group {
	case groupCommon:
		return filepath.Base(rel), false
	case groupRole:
		return strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)), false
	}
	if rest, ok := strings.CutPrefix(rel, "teams/"); ok {
		if team, _, found := strings.Cut(rest, "/"); found {
			return team, true
		}
	}
	return filepath.Base(rel), false
}

// layerDir is where one layer's files live.
func (p servePort) layerDir(layer string) string {
	if layer == web.LayerGlobal {
		return p.env.GlobalDir
	}
	return p.env.LocalDir
}

// relIn is where a file stands inside its layer, the way the prototype writes
// it ("common/00_TEAM_PROTOCOL.yaml").
func relIn(root, file string) string {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return filepath.Base(file)
	}
	return filepath.ToSlash(rel)
}

// teamPath is the team definition screen's own path. The rules screen's path is
// rulesview's, so only one place names it.
const teamPath = "/team"

// rulesHref is the rules screen at one document.
func rulesHref(layer, path string) string {
	return rulesview.PathRules + "?layer=" + url.QueryEscape(layer) + "&doc=" + url.QueryEscape(path)
}

// teamHref is the team definition screen at one definition.
func teamHref(layer, name string) string {
	return teamPath + "?layer=" + url.QueryEscape(layer) + "&name=" + url.QueryEscape(name)
}

// profileCount is how many profiles one layer's own profiles.yaml holds. A
// layer with no file has none, and a file that does not read shows none rather
// than stopping the screen.
func (p servePort) profileCount(layer string) int {
	text, err := os.ReadFile(filepath.Join(p.layerDir(layer), "profiles.yaml"))
	if err != nil {
		return 0
	}
	profiles, err := config.ParseProfiles(string(text), filepath.Join(p.layerDir(layer), "profiles.yaml"))
	if err != nil {
		return 0
	}
	return len(profiles)
}
