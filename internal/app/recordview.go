package app

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/homeview"
)

// This file reads what the screens of the launch record show (spec/10 の 7 節):
// the documents of one layer against the text the last launch read, and each
// member's prompt against the one the files build now. Everything is compared
// here, with the same parsers the commands use, so the page holds no comparison
// of its own.

// layerDef is one team definition of a layer, read the way the commands read it.
type layerDef struct {
	name string
	cfg  *config.Config
}

// layerDefinitions reads one layer's team definitions, the broken ones left out:
// a definition that cannot be read has no documents and no prompts to show, and
// the team list already carries why it is broken.
func layerDefinitions(env team.Env, layer string) []layerDef {
	var out []layerDef
	for _, row := range env.List() {
		if !inLayer(row, layer) {
			continue
		}
		cfg, err := validateTeam(env, row.Name, layer)
		if err != nil {
			continue
		}
		out = append(out, layerDef{name: row.Name, cfg: cfg})
	}
	return out
}

// readRecords is this machine's launch record, oldest first, when it can be read.
// The team list shows what it has without one: a record that cannot be read is
// not a broken definition.
func readRecords() []launchlog.Record {
	log, err := launchlog.Open()
	if err != nil {
		return nil
	}
	records, err := log.Read()
	if err != nil {
		return nil
	}
	return records
}

// latestRecord is the newest record of one definition of one layer, from a set
// read oldest first.
func latestRecord(records []launchlog.Record, layer, definition string) (launchlog.Record, bool) {
	var out launchlog.Record
	found := false
	for _, rec := range records {
		if rec.Layer == layer && rec.Definition == definition {
			out, found = rec, true
		}
	}
	return out, found
}

// recordLine writes why the launch record could not be read as one line, the way
// a broken settings file is shown.
func recordLine(env team.Env, log launchlog.Log, err error) string {
	return breakLine(env, log.Dir(), err)
}

// Home reads one layer for the home screen: every document a team definition of
// the layer reads now, against the text the newest launch that read it wrote
// down. The rules with no check are read from the files themselves, so they show
// on a machine that has not launched anything yet.
func (p servePort) Home(layer string) homeview.Home {
	defs := layerDefinitions(p.env, layer)
	log, openErr := launchlog.Open()
	records, readErr := log.Read()
	out := homeview.Home{}
	if err := errors.Join(openErr, readErr); err != nil {
		out.Problem = recordLine(p.env, log, err)
	}
	docs := documentsOf(defs)
	out.Launched = anyLaunched(records, defs)
	launched := launchedDocs(records, defs, docs)
	out.Docs = make([]homeview.Doc, 0, len(docs))
	for _, doc := range docs {
		out.Docs = append(out.Docs, p.homeDoc(log, doc, launched))
	}
	return out
}

// anyLaunched reports whether any of the definitions has a record: without one
// there is nothing to differ from.
func anyLaunched(records []launchlog.Record, defs []layerDef) bool {
	reading := map[string]bool{}
	for _, def := range defs {
		reading[def.cfg.Layer+"\x00"+def.name] = true
	}
	for _, rec := range records {
		if reading[rec.Layer+"\x00"+rec.Definition] {
			return true
		}
	}
	return false
}

// documentsOf is every rule file the definitions read, once each, by path. A
// document two definitions share is one entry: both read the same text.
func documentsOf(defs []layerDef) []source.Doc {
	seen := map[string]bool{}
	var out []source.Doc
	for _, def := range defs {
		for i := range def.cfg.Members {
			for _, doc := range docsReadBy(&def.cfg.Members[i]) {
				if seen[doc.Path] {
					continue
				}
				seen[doc.Path] = true
				out = append(out, doc)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// launchedDoc is what the record holds for one document: the newest launch,
// among the definitions of the layer that read it now, that read it.
type launchedDoc struct {
	// Definition is the team definition that launch started.
	Definition string
	// Doc is what that launch wrote down about the document.
	Doc launchlog.Doc
}

// launchedDocs is what the record holds for each document, by path.
func launchedDocs(records []launchlog.Record, defs []layerDef, docs []source.Doc) map[string]launchedDoc {
	reading := map[string]bool{}
	for _, def := range defs {
		reading[def.cfg.Layer+"\x00"+def.name] = true
	}
	wanted := map[string]bool{}
	for _, doc := range docs {
		wanted[doc.Path] = true
	}
	out := map[string]launchedDoc{}
	for _, rec := range records {
		if !reading[rec.Layer+"\x00"+rec.Definition] {
			continue
		}
		for _, doc := range rec.Docs {
			if wanted[doc.Path] {
				// a later record overwrites an earlier one
				out[doc.Path] = launchedDoc{Definition: rec.Definition, Doc: doc}
			}
		}
	}
	return out
}

// homeDoc is one document of the layer: what it holds now, how that differs from
// the last launch's text, and whether it has been given a new version since.
func (p servePort) homeDoc(log launchlog.Log, doc source.Doc, launched map[string]launchedDoc) homeview.Doc {
	text, err := os.ReadFile(doc.Path)
	if err != nil {
		return homeview.Doc{Path: p.env.Display(doc.Path), File: doc.Path, Layer: doc.Layer}
	}
	parsed, err := rules.Parse(string(text), doc.Path)
	if err != nil {
		// The definition that read it parsed it a moment ago; a file that broke
		// since has nothing to show here.
		return homeview.Doc{Path: p.env.Display(doc.Path), File: doc.Path, Layer: doc.Layer}
	}
	entry := homeview.Doc{
		Path:    p.env.Display(doc.Path),
		File:    doc.Path,
		Layer:   doc.Layer,
		Rules:   len(parsed.Rules()),
		Version: versionOf(parsed),
	}
	for _, rule := range parsed.Rules() {
		if rule.Report == "" && len(rule.Tests) == 0 {
			entry.NoCheck = append(entry.NoCheck, homeview.Rule{
				ID:   rule.ID,
				Href: p.RuleHref(doc.Layer, doc.Path, rule.ID),
			})
		}
	}
	if was, ok := launched[doc.Path]; ok {
		entry.Definition = was.Definition
		p.changesSince(log, doc, text, was.Doc, &entry)
	}
	return entry
}

// changesSince fills in what moved in one document since the last launch that
// read it: the version it has moved to since, and the rules it added, dropped
// and changed.
func (p servePort) changesSince(log launchlog.Log, doc source.Doc, text []byte, was launchlog.Doc, entry *homeview.Doc) {
	entry.Bumped = entry.Version > was.Revision
	before, err := log.Text(was.SHA256)
	if err != nil {
		entry.Missing = true
		return
	}
	if string(before) == string(text) {
		return
	}
	beforeRules, okBefore := ruleLines(string(before), doc.Path)
	nowRules, okNow := ruleLines(string(text), doc.Path)
	if !okBefore || !okNow {
		entry.Whole = true
		return
	}
	entry.Added, entry.Removed, entry.Changed = ruleChanges(beforeRules, nowRules)
}

// versionOf is a rule file's own version: the revision it writes, or 0 when it
// writes none.
func versionOf(doc *rules.Document) int {
	if doc.Revision == nil {
		return 0
	}
	return *doc.Revision
}

// Bump raises one document's version by one and writes the document back
// (spec/10 の 7 節). The version is the only thing about it that changes: the
// document is read, its version is raised (a file that writes none is at 0), and
// the document is written with rules.Emit, never by hand (H5).
//
// The write goes through the writer the rules screen uses, so a path no member of
// the layer reads is refused and a file that moved under the screen is not
// overwritten. What comes back is what the screen shows after it: the version it
// moved from and to, and the team definition whose next launch hands the change
// over.
func (p servePort) Bump(layer, path string) (homeview.Bump, error) {
	read, err := p.ReadRule(path)
	if err != nil {
		return homeview.Bump{}, err
	}
	document, err := rules.Parse(read.Text, path)
	if err != nil {
		return homeview.Bump{}, err
	}
	from := versionOf(document)
	to := from + 1
	document.Revision = &to
	text, err := rules.Emit(document)
	if err != nil {
		return homeview.Bump{}, err
	}
	if err := p.WriteRule(path, text, read.Hash); err != nil {
		return homeview.Bump{}, err
	}
	return homeview.Bump{
		Path:       p.env.Display(path),
		File:       path,
		Layer:      p.layerOf(path),
		From:       from,
		To:         to,
		Definition: p.definitionToRun(layer, path),
	}, nil
}

// definitionToRun is the team definition whose next launch hands a document's
// change over: the definition of the newest launch that read the document, when
// that definition still reads it, and otherwise the first definition of the
// layer that reads it now. Empty when no definition of the layer reads it.
func (p servePort) definitionToRun(layer, path string) string {
	var reading []layerDef
	for _, def := range layerDefinitions(p.env, layer) {
		if readsPath(def.cfg, path) {
			reading = append(reading, def)
		}
	}
	if len(reading) == 0 {
		return ""
	}
	records := readRecords()
	for i := len(records) - 1; i >= 0; i-- {
		if !recordReads(records[i], path) {
			continue
		}
		for _, def := range reading {
			if def.name == records[i].Definition {
				return def.name
			}
		}
		// The newest launch that read it belongs to a definition that no longer
		// reads it; another definition that reads it now is named instead.
		break
	}
	return reading[0].name
}

// recordReads reports whether one record holds a document, which is how a record
// says that launch read it.
func recordReads(rec launchlog.Record, path string) bool {
	for _, doc := range rec.Docs {
		if doc.Path == path {
			return true
		}
	}
	return false
}

// ruleLine is one rule of one text: its id, and what it carries.
type ruleLine struct {
	id      string
	content string
}

// ruleLines reads one text as its rules, in the written order. ok is false when
// the text does not read as a rule file, which is when two texts cannot be lined
// up by rule.
func ruleLines(text, path string) (lines []ruleLine, ok bool) {
	doc, err := rules.Parse(text, path)
	if err != nil {
		return nil, false
	}
	for _, rule := range doc.Rules() {
		lines = append(lines, ruleLine{id: rule.ID, content: ruleContent(rule)})
	}
	return lines, true
}

// ruleContent is one rule as the file writes it, without its id: the two texts
// are lined up by id, and a rule counts as changed when anything it carries
// differs.
func ruleContent(rule rules.Rule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "text=%s\nwhy=%s\nsend_why=%s\nnote=%s\nreport=%s\ntests=%s\norigin=%v",
		rule.Text, rule.Why, sendWhy(rule), rule.Note, rule.Report, rule.Tests, rule.Origin)
	return b.String()
}

// sendWhy is a rule's send_why as written, with a missing one listed as true.
func sendWhy(rule rules.Rule) string {
	if rule.SendWhy == nil {
		return "true"
	}
	return strconv.FormatBool(*rule.SendWhy)
}

// ruleChanges lines up two readings of one document by rule id: the ids the
// newer text added, the ids it dropped, and the ids whose rule changed. The
// added and changed ids keep the order the document writes them in.
func ruleChanges(before, after []ruleLine) (added, removed, changed []string) {
	was := make(map[string]string, len(before))
	for _, line := range before {
		was[line.id] = line.content
	}
	now := make(map[string]bool, len(after))
	for _, line := range after {
		now[line.id] = true
		content, ok := was[line.id]
		switch {
		case !ok:
			added = append(added, line.id)
		case content != line.content:
			changed = append(changed, line.id)
		}
	}
	for _, line := range before {
		if !now[line.id] {
			removed = append(removed, line.id)
		}
	}
	return added, removed, changed
}

// Diff reads one layer for the prompt diff screen: for each member of each team
// definition, the prompt the last launch handed over against the prompt the
// files build now.
func (p servePort) Diff(layer string) homeview.Diff {
	log, openErr := launchlog.Open()
	records, readErr := log.Read()
	if err := errors.Join(openErr, readErr); err != nil {
		return homeview.Diff{Problem: recordLine(p.env, log, err)}
	}
	var out homeview.Diff
	for _, def := range layerDefinitions(p.env, layer) {
		rec, found := latestRecord(records, def.cfg.Layer, def.name)
		item := homeview.Definition{Name: def.name, Launched: found}
		for i := range def.cfg.Members {
			item.Members = append(item.Members, p.memberDiff(log, &def.cfg.Members[i], rec, found))
		}
		out.Definitions = append(out.Definitions, item)
	}
	return out
}

// memberDiff is one member's prompt against the one the record holds for it. A
// member's short name is what lines the two up: a launch under another team name
// is the same member, and its prompt differs where the name does.
func (p servePort) memberDiff(log launchlog.Log, member *config.Member, rec launchlog.Record, found bool) homeview.Member {
	diff := homeview.Member{Name: member.Name}
	if !found {
		return diff
	}
	was, ok := recordedMember(rec, member.ShortName)
	if !ok {
		return diff
	}
	text, err := log.Text(was.Prompt)
	if err != nil {
		return diff
	}
	diff.Launched = true
	if string(text) == member.Prompt {
		diff.Lines = sameLines(member.Prompt)
		return diff
	}
	diff.Lines = homeview.DiffLines(splitLines(string(text)), splitLines(member.Prompt))
	return diff
}

// recordedMember is what the record holds for one member of a definition, matched
// by the short name team.yaml writes.
func recordedMember(rec launchlog.Record, short string) (launchlog.Member, bool) {
	for _, member := range rec.Members {
		if member.Short == short {
			return member, true
		}
	}
	return launchlog.Member{}, false
}

// sameLines is a prompt as a diff where nothing moved.
func sameLines(prompt string) []homeview.Line {
	lines := splitLines(prompt)
	out := make([]homeview.Line, 0, len(lines))
	for _, text := range lines {
		out = append(out, homeview.Line{Kind: homeview.Same, Text: text})
	}
	return out
}

// splitLines splits a text into its lines, without a trailing empty one.
func splitLines(text string) []string {
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// RuleHref is where the rules screen opens one rule of one document in its edit
// panel. That screen owns its paths; this is the one place that knows them,
// because the home links into it (hergy-impl-rules, /rules?layer=&doc=&rule=).
func (p servePort) RuleHref(layer, path, id string) string {
	query := url.Values{}
	query.Set("layer", layer)
	query.Set("doc", path)
	query.Set("rule", id)
	return "/rules?" + query.Encode()
}

// launchNote is what the team list shows about the last launch of a definition:
// when it ran, the hash it wrote, and whether the files build that hash now.
func (p servePort) launchNote(records []launchlog.Record, name, layer string) web.Team {
	rec, ok := latestRecord(records, layer, name)
	if !ok {
		return web.Team{}
	}
	return web.Team{
		Launched: timeOf(rec.Time),
		Hash:     shortHash(rec.Hash),
		Same:     rec.Hash == p.teamHash(name, layer, rec.Team),
	}
}

// teamHash is the hash of one definition as the files build it now, under the
// team name given: the name reaches the prompts, so it is part of what a launch
// hands over, and the comparison is made under the name that was used.
func (p servePort) teamHash(name, layer, teamName string) string {
	resolved, err := p.env.Resolve(name, layer)
	if err != nil {
		return ""
	}
	cfg, err := p.env.BuildConfig(resolved, source.Disk{}, teamName)
	if err != nil {
		return ""
	}
	material, err := teamMaterial(cfg)
	if err != nil {
		return ""
	}
	return launchlog.Hash(material)
}

// timeOf writes a recorded RFC 3339 time for reading. A time that does not read
// is shown as it was written.
func timeOf(when string) string {
	at, err := time.Parse(time.RFC3339, when)
	if err != nil {
		return when
	}
	return at.Format("2006-01-02 15:04")
}

// shortHash is the head of a hash, enough to tell one team from another.
func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}
