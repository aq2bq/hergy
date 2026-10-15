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
	"github.com/aq2bq/hergy/internal/i18n"
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

// latestRecord is the newest record of one team definition, from a set read
// oldest first. A definition is named by the team.yaml its launches read: the
// record holds every project's launches, and two projects can hold a definition
// of the same name, so the layer and the name alone would pick up another
// project's launch (M1).
func latestRecord(records []launchlog.Record, teamYAML string) (launchlog.Record, bool) {
	var out launchlog.Record
	found := false
	for _, rec := range records {
		if rec.TeamYAML == teamYAML {
			out, found = rec, true
		}
	}
	return out, found
}

// definitionOf is the definition of one layer a record belongs to, matched by the
// team.yaml its launch read. ok is false for a record of another layer, and for
// one of another project whose team.yaml no definition of this layer stands at.
func definitionOf(rec launchlog.Record, defs []layerDef) (layerDef, bool) {
	for _, def := range defs {
		if def.cfg.TeamYAML.Path == rec.TeamYAML {
			return def, true
		}
	}
	return layerDef{}, false
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
	for _, rec := range records {
		if _, ok := definitionOf(rec, defs); ok {
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
	// Launched is when that launch ran, as a person reads it, and Hash is the
	// head of the hash it wrote: the home names both beside the document, so a
	// reader sees which launch the changes are against (試作 homeView).
	Launched string
	Hash     string
	// Doc is what that launch wrote down about the document.
	Doc launchlog.Doc
}

// launchedDocs is what the record holds for each document, by path. Only the
// records of the layer's own team definitions are read, told apart by the
// team.yaml their launches read (M1).
func launchedDocs(records []launchlog.Record, defs []layerDef, docs []source.Doc) map[string]launchedDoc {
	wanted := map[string]bool{}
	for _, doc := range docs {
		wanted[doc.Path] = true
	}
	out := map[string]launchedDoc{}
	for _, rec := range records {
		def, ok := definitionOf(rec, defs)
		if !ok {
			continue
		}
		for _, doc := range rec.Docs {
			if wanted[doc.Path] {
				// a later record overwrites an earlier one
				out[doc.Path] = launchedDoc{
					Definition: def.name,
					Launched:   timeOf(rec.Time),
					Hash:       shortHash(rec.Hash),
					Doc:        doc,
				}
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
		entry.Launched = was.Launched
		entry.Hash = was.Hash
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
// Only a document the launch record holds, whose content has moved since the
// launch that read it, and whose version has not been raised since that launch,
// is written: a page left open, a resend, or a double press writes nothing and
// the answer says why (M3).
//
// The write goes through the writer the rules screen uses, so a path no member of
// the layer reads is refused and a file that moved under the screen is not
// overwritten. What comes back is what the screen shows after it: the version it
// moved from and to, and the team definition whose next launch hands the change
// over.
func (p servePort) Bump(layer, path string) (homeview.Answer, error) {
	// A path no member of this layer reads is refused the way the rules screen
	// refuses it, before the record is asked about the document.
	if !p.reads(p.layerOf(path), path) {
		return homeview.Answer{}, i18n.Errorf("error.rules_not_writable", p.env.Display(path))
	}
	read, err := p.ReadRule(path)
	if err != nil {
		return homeview.Answer{}, err
	}
	document, err := rules.Parse(read.Text, path)
	if err != nil {
		return homeview.Answer{}, err
	}
	// The record says whether this document can be raised at all: only the text
	// the launch that read it wrote down can tell that it moved.
	log, err := launchlog.Open()
	if err != nil {
		return homeview.Answer{}, err
	}
	records, err := log.Read()
	if err != nil {
		return homeview.Answer{}, err
	}
	launched, recorded := launchedDocs(records, layerDefinitions(p.env, layer), []source.Doc{{Path: path}})[path]
	display := p.env.Display(path)
	if !recorded {
		return homeview.Answer{Why: i18n.Tf("error.bump_not_recorded", display)}, nil
	}
	before, err := log.Text(launched.Doc.SHA256)
	if err != nil {
		// The launch read the document and its text is not in the record, so the
		// two cannot be compared: the screen cannot say it moved either.
		return homeview.Answer{Why: i18n.Tf("error.bump_not_recorded", display)}, nil
	}
	if string(before) == read.Text {
		return homeview.Answer{Why: i18n.Tf("error.bump_unchanged", display)}, nil
	}
	from := versionOf(document)
	if from > launched.Doc.Revision {
		return homeview.Answer{Why: i18n.Tf("error.bump_already_raised", display)}, nil
	}
	to := from + 1
	document.Revision = &to
	text, err := rules.Emit(document)
	if err != nil {
		return homeview.Answer{}, err
	}
	if err := p.WriteRule(path, text, read.Hash); err != nil {
		return homeview.Answer{}, err
	}
	return homeview.Answer{Written: true, Raised: homeview.Bump{
		Path:       display,
		File:       path,
		Layer:      p.layerOf(path),
		From:       from,
		To:         to,
		Definition: p.definitionToRun(layer, path),
	}}, nil
}

// definitionToRun is the team definition whose next launch hands a document's
// change over: the definition of the newest launch that read the document, when
// that definition still reads it, and otherwise the first definition of the
// layer that reads it now. Empty when no definition of the layer reads it.
func (p servePort) definitionToRun(layer, path string) string {
	defs := layerDefinitions(p.env, layer)
	var reading []layerDef
	for _, def := range defs {
		if readsPath(def.cfg, path) {
			reading = append(reading, def)
		}
	}
	if len(reading) == 0 {
		return ""
	}
	// Only the layer's own records are read, so a launch of another project's
	// definition of the same name is not what names the command (M1).
	records := readRecords()
	for i := len(records) - 1; i >= 0; i-- {
		def, mine := definitionOf(records[i], defs)
		if !mine || !recordReads(records[i], path) {
			continue
		}
		if readsPath(def.cfg, path) {
			return def.name
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

// ruleLine is one rule of one text: its id, the heading of the section it is
// written under, and the rule itself, so two readings of one document can be
// lined up by id and what each rule carries can be compared (試作 ruleMap).
type ruleLine struct {
	id      string
	section string
	rule    rules.Rule
}

// ruleLines reads one text as its rules, in the written order, each with the
// heading of the section it is written under. ok is false when the text does not
// read as a rule file, which is when two texts cannot be lined up by rule.
func ruleLines(text, path string) (lines []ruleLine, ok bool) {
	doc, err := rules.Parse(text, path)
	if err != nil {
		return nil, false
	}
	for _, section := range doc.Sections {
		for _, rule := range section.Rules {
			lines = append(lines, ruleLine{id: rule.ID, section: section.Heading, rule: rule})
		}
	}
	return lines, true
}

// ruleFields is the parts of a rule the file writes, in the order the change
// row names them (試作 RF). The section a rule stands in is named after these.
var ruleFields = []string{"text", "why", "send_why", "note", "origin", "report", "tests"}

// ruleField is one part of a rule as a comparable value: the two readings are
// lined up by id, and a rule counts as changed when anything it carries differs.
func ruleField(rule rules.Rule, key string) string {
	switch key {
	case "text":
		return rule.Text
	case "why":
		return rule.Why
	case "send_why":
		return sendWhy(rule)
	case "note":
		return rule.Note
	case "origin":
		return fmt.Sprint(rule.Origin)
	case "report":
		return rule.Report
	case "tests":
		return fmt.Sprint(rule.Tests)
	}
	return ""
}

// sendWhy is a rule's send_why as written, with a missing one listed as true.
func sendWhy(rule rules.Rule) string {
	if rule.SendWhy == nil {
		return "true"
	}
	return strconv.FormatBool(*rule.SendWhy)
}

// ruleChanges lines up two readings of one document by rule id: the rules the
// newer text added, the ones it dropped, and the ones whose parts moved. The
// added and changed rules keep the order the document writes them in (試作
// pending), and each carries what the change row draws: the parts that moved,
// and their text before and after (試作 changeHtml).
func ruleChanges(before, after []ruleLine) (added, removed, changed []homeview.Rule) {
	was := make(map[string]ruleLine, len(before))
	for _, line := range before {
		was[line.id] = line
	}
	now := make(map[string]bool, len(after))
	for _, line := range after {
		now[line.id] = true
		old, ok := was[line.id]
		if !ok {
			added = append(added, homeview.Rule{
				ID:      line.id,
				Section: line.section,
				Diffs:   []homeview.Part{{After: line.rule.Text}},
			})
			continue
		}
		if fields := changedFields(old, line); len(fields) > 0 {
			changed = append(changed, homeview.Rule{
				ID:      line.id,
				Section: line.section,
				Fields:  fields,
				Diffs:   changeDiffs(old, line, fields),
			})
		}
	}
	for _, line := range before {
		if !now[line.id] {
			removed = append(removed, homeview.Rule{
				ID:      line.id,
				Section: line.section,
				Diffs:   []homeview.Part{{Before: line.rule.Text}},
			})
		}
	}
	return added, removed, changed
}

// changedFields is the parts of a rule that differ between two readings, in the
// order the file writes them, with the section named last when the rule moved
// to another one (試作 pending).
func changedFields(before, after ruleLine) []string {
	var fields []string
	for _, key := range ruleFields {
		if ruleField(before.rule, key) != ruleField(after.rule, key) {
			fields = append(fields, key)
		}
	}
	if before.section != after.section {
		fields = append(fields, "section")
	}
	return fields
}

// changeDiffs is what moved between two readings of one rule, each part with its
// text before and after: the text, the reason and the origin, which are the
// parts the change row shows as a diff (試作 changeHtml). A part the row names
// but does not draw (send_why, note, report, tests) is left out.
func changeDiffs(before, after ruleLine, fields []string) []homeview.Part {
	var out []homeview.Part
	for _, key := range fields {
		switch key {
		case "text":
			out = append(out, homeview.Part{Before: before.rule.Text, After: after.rule.Text})
		case "why":
			out = append(out, homeview.Part{Before: before.rule.Why, After: after.rule.Why})
		case "origin":
			out = append(out, homeview.Part{
				Before: originText(before.rule.Origin),
				After:  originText(after.rule.Origin),
			})
		}
	}
	return out
}

// originText is a rule's origins as the change row reads them, one per origin,
// joined by " / ", with no origin said in words (試作 originLabel).
func originText(origins []rules.Origin) string {
	if len(origins) == 0 {
		return i18n.T("web.home.changes.origin_none")
	}
	parts := make([]string, 0, len(origins))
	for _, origin := range origins {
		parts = append(parts, originLabel(origin))
	}
	return strings.Join(parts, " / ")
}

// originLabel is one origin as the change row reads it: its kind, and then when
// it happened and what it points at (試作 originLabel).
func originLabel(origin rules.Origin) string {
	var label string
	switch origin.Kind {
	case rules.KindIncident:
		label = i18n.T("web.rules.origin.kind.incident")
	case rules.KindInstruction:
		label = i18n.T("web.rules.origin.kind.instruction")
	case rules.KindDerived:
		label = i18n.T("web.rules.origin.kind.derived")
	default:
		label = origin.Kind
	}
	if origin.Kind == rules.KindDerived {
		return label + " ← " + strings.Join(origin.Refs, " ")
	}
	if origin.Date != "" {
		label += " " + origin.Date
	}
	if len(origin.Refs) > 0 {
		label += " " + strings.Join(origin.Refs, " ")
	}
	return label
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
		_, found := latestRecord(records, def.cfg.TeamYAML.Path)
		item := homeview.Definition{Name: def.name, Launched: found}
		for i := range def.cfg.Members {
			item.Members = append(item.Members, p.memberDiff(log, records, def, &def.cfg.Members[i]))
		}
		out.Definitions = append(out.Definitions, item)
	}
	return out
}

// memberDiff is one member's prompt against the one the newest launch of its team
// definition that handed this member a prompt wrote down. A member's short name is
// what lines the two up: a launch under another team name is the same member, and
// its prompt differs where the name does.
//
// A launch that handed prompts to one member only (refresh, reprompt) says what
// that member got and nothing about the others, so each member is compared with
// the launch that last handed it one (M2).
func (p servePort) memberDiff(log launchlog.Log, records []launchlog.Record, def layerDef, member *config.Member) homeview.Member {
	diff := homeview.Member{Name: member.Name}
	text, ok := recordedPrompt(log, records, def.cfg.TeamYAML.Path, member.ShortName)
	if !ok {
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

// recordedPrompt is the prompt one member was last handed by a launch of one team
// definition: the definition's records (matched by the team.yaml the launch read,
// M1) are walked newest first, and the first that holds this member answers. A
// launch of another project, or one that handed this member nothing, is skipped.
func recordedPrompt(log launchlog.Log, records []launchlog.Record, teamYAML, short string) ([]byte, bool) {
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].TeamYAML != teamYAML {
			continue
		}
		member, ok := recordedMember(records[i], short)
		if !ok {
			continue
		}
		text, err := log.Text(member.Prompt)
		if err != nil {
			continue
		}
		return text, true
	}
	return nil, false
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
// when it ran, the hash it wrote, and whether the files build that hash now. The
// definition's team.yaml is what picks the record out of the machine's log, so
// another project's launch of a definition of the same name is not it (M1).
func (p servePort) launchNote(records []launchlog.Record, cfg *config.Config, name, layer string) web.Team {
	rec, ok := latestRecord(records, cfg.TeamYAML.Path)
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
