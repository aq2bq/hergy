package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// This file draws the three rule documents a screen writes (段 4, 設計 3.8): a
// team's own agreement, a role's document, and the agreement every team of the
// place shares. All three draw the same card: a heading per section, a card per
// rule (its id and the first clause of its text, so a card never stands for its
// id alone), and, in the panel a card opens, the five things a rule holds. The
// save path is the one of 設計 7.3, and the sentences a refusal shows are the
// ones hergy validate prints, from the same check.

// rulesPlace is one rule document as a screen holds it: where it stands, the
// definition whose own agreement it is or the role whose document it is, and the
// page that draws it.
type rulesPlace struct {
	Doc  source.Doc
	Def  string
	Role string
	Page rulesPage
}

// rulesPage is the screen a rule document is drawn on.
type rulesPage int

const (
	pageDefinition rulesPage = iota // the definition's detail, which holds the team agreement
	pageRole                        // one role's document
	pageShared                      // the agreement every team shares
)

// document is the document's own place: the field the document stands at, and
// what a card the document is adding stands beside (設計 3.8).
func (p rulesPlace) document() string {
	switch p.Page {
	case pageRole:
		return rolePath(p.Role)
	case pageShared:
		return "/agreement"
	default:
		return agreementPath(p.Def)
	}
}

// parent is what closing one of the document's cards goes back to: the page the
// document is drawn on (設計 2 節の「閉じる」).
func (p rulesPlace) parent() string {
	if p.Page == pageDefinition {
		return definitionPath(p.Def)
	}
	return p.document()
}

// ruleURL is where one rule's card stands.
func (p rulesPlace) ruleURL(id string) string {
	switch p.Page {
	case pageRole:
		return roleRulePath(p.Role, id)
	case pageShared:
		return agreementRulePath(id)
	default:
		return rulePath(p.Def, id)
	}
}

// addURL is where the card a rule added at the end of one section stands. The
// section is a place in the document, not a name, so it travels as a value.
func (p rulesPlace) addURL(section int) string {
	return p.document() + "/new?section=" + strconv.Itoa(section)
}

// missingRule is the one line a field says when the document holds no such rule.
func (p rulesPlace) missingRule(id string) string {
	switch p.Page {
	case pageRole:
		return tf("web.missing.role_rule", p.Role, id)
	case pageShared:
		return tf("web.missing.shared_rule", id)
	default:
		return tf("web.missing.rule", p.Def, id)
	}
}

// teamPlace is the team agreement of one definition; a definition that holds
// none has no document to write yet (W8).
func (s *Server) teamPlace(name string) rulesPlace {
	place := rulesPlace{Def: name, Page: pageDefinition}
	if def, err := s.readDefinition(name); err == nil {
		place.Doc = def.Agreement
	}
	return place
}

// rolePlace is one role's document.
func (s *Server) rolePlace(role string) rulesPlace {
	place := rulesPlace{Role: role, Page: pageRole}
	if doc, ok := s.cat.Role(role); ok {
		place.Doc = doc
	}
	return place
}

// sharedPlace is the agreement every team of the place shares.
func (s *Server) sharedPlace() rulesPlace {
	place := rulesPlace{Page: pageShared}
	if doc, ok := s.cat.SharedAgreement(); ok {
		place.Doc = doc
	}
	return place
}

// madePlace is where a document that is not there yet goes (K2, W8): the place
// itself, with nothing behind it, so the same save path can write it.
func (s *Server) madeRolePlace(role string) rulesPlace {
	return rulesPlace{Doc: source.Doc{Path: s.cat.NewPlace(catalog.NewRole, role)}, Role: role, Page: pageRole}
}

// madeSharedPlace is where a shared agreement that is not there yet goes.
func (s *Server) madeSharedPlace() rulesPlace {
	return rulesPlace{Doc: source.Doc{Path: s.cat.NewPlace(catalog.NewAgreement, "")}, Page: pageShared}
}

// ruleOpen is which card a page draws open: one rule by its id, or the card a
// document is adding at the end of a section (設計 3.8).
type ruleOpen struct {
	ID      string
	Add     bool
	Section int
	Ask     bool // the delete is asked for, and a second press writes it (W15)
}

// ruleForm is one rule as its card writes it: the whole of the text, its reason,
// its origins, its check and its note, writable from the start (C1, 3.8). The
// report shape a rule points at is shown and not written (W14).
type ruleForm struct {
	Add         bool
	ID          string
	Section     int
	SectionName string
	Text        string
	Why         string
	Note        string
	Report      string
	Tests       string
	Origins     []originRow
	Draft       string
	Token       string
	Hash        string
	Action      string
	DelAction   string
	Parent      string
	AskDelete   bool
	Saved       bool
}

// originRow is one origin as the card holds it: its kind, its date, the records
// it points at, and a note.
type originRow struct {
	Kind string
	Date string
	Refs string
	Note string
}

// saved is the line a write shows and what a running team has to take up (設計
// 3.0).
type saved struct {
	Line     string
	Handover handover
}

// ruleCard is the card one place draws open: the rule itself, or the card the
// document is adding. A document that does not read, and a rule it does not
// hold, come back as the one line that says so (N2, 3.8).
func (s *Server) ruleCard(place rulesPlace, open ruleOpen, notice string, reload, saved bool, warnings map[string][]string) panel {
	doc, err := s.ruleDoc(place.Doc)
	if err != nil {
		return missingPanelAt(place.parent(), "rule:"+open.ID, tf("web.panel.rule", open.ID), s.breakLine(place.Doc.Path, err))
	}
	if open.Add {
		return s.newRuleCard(place, doc, open, notice, reload, saved)
	}
	card, ok := s.ruleCardOf(place, doc, open, notice, reload, saved, warnings)
	if !ok {
		return missingPanelAt(place.parent(), "rule:"+open.ID, tf("web.panel.rule", open.ID), place.missingRule(open.ID))
	}
	return card
}

// ruleCardOf is one rule of a document as its card writes it: the form, the
// file's hash (so a save can tell whether it moved), and the sentences check
// writes about its text (設計 3.8、spec/33 H2).
func (s *Server) ruleCardOf(place rulesPlace, doc *rules.Document, open ruleOpen, notice string, reload, saved bool, warnings map[string][]string) (panel, bool) {
	section, index, ok := findRule(doc, open.ID)
	if !ok {
		return panel{}, false
	}
	rule := doc.Sections[section].Rules[index]
	form := &ruleForm{
		ID:        rule.ID,
		Section:   section,
		Text:      rule.Text,
		Why:       rule.Why,
		Note:      rule.Note,
		Report:    rule.Report,
		Tests:     strings.Join(rule.Tests, ", "),
		Origins:   originRows(rule.Origin),
		Draft:     "rule:" + rule.ID,
		Token:     s.token,
		Hash:      s.hashOf(place.Doc.Path),
		Action:    place.ruleURL(rule.ID),
		DelAction: place.document(),
		Parent:    place.parent(),
		AskDelete: open.Ask,
		Saved:     saved,
	}
	return panel{
		Title:    tf("web.panel.rule", rule.ID),
		Key:      "rule:" + rule.ID,
		Parent:   place.parent(),
		Crumb:    rule.ID,
		Form:     form,
		Notice:   notice,
		Reload:   reload,
		Findings: warnings[rule.ID],
	}, true
}

// newRuleCard is the card a document is adding: the same five fields, an id the
// core proposes, and the section the rule goes at the end of (3.8).
func (s *Server) newRuleCard(place rulesPlace, doc *rules.Document, open ruleOpen, notice string, reload, saved bool) panel {
	section := open.Section
	if section < 0 || section >= len(doc.Sections) {
		section = 0
	}
	form := &ruleForm{
		Add:         true,
		Saved:       saved,
		ID:          proposedRuleID(doc),
		Section:     section,
		SectionName: doc.Sections[section].Heading,
		Origins:     []originRow{{}},
		Draft:       "new:" + place.parent() + ":" + strconv.Itoa(section),
		Token:       s.token,
		Hash:        s.hashOf(place.Doc.Path),
		Action:      place.addURL(section),
		Parent:      place.parent(),
	}
	return panel{
		Title:  t("web.rule.new"),
		Key:    "new",
		Parent: place.parent(),
		Crumb:  t("web.rule.new"),
		Form:   form,
		Notice: notice,
		Reload: reload,
	}
}

// findRule is where one rule stands in a document, by its id.
func findRule(doc *rules.Document, id string) (section, index int, ok bool) {
	for i := range doc.Sections {
		for j := range doc.Sections[i].Rules {
			if doc.Sections[i].Rules[j].ID == id {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// proposedRuleID is the id the core offers for a rule a screen is adding (3.8):
// SuggestRuleID over the document's last id, moved on to the next number, so the
// id reads like the ones in the document. The id is a proposal: the card writes
// it and the person may change it.
func proposedRuleID(doc *rules.Document) string {
	last := ""
	if all := doc.Rules(); len(all) > 0 {
		last = all[len(all)-1].ID
	}
	return doc.SuggestRuleID(nextRuleID(last))
}

// nextRuleID moves an id's trailing number on by one (LEAD-3 becomes LEAD-4). An
// id with no number gets -1 (P-TURN becomes P-TURN-1), and a document with no
// rule at all starts at R-1.
func nextRuleID(id string) string {
	if id == "" {
		return "R-1"
	}
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	if i == len(id) {
		return id + "-1"
	}
	n, err := strconv.Atoi(id[i:])
	if err != nil {
		return id + "-1"
	}
	return id[:i] + strconv.Itoa(n+1)
}

// originRows is a rule's origins as the card holds them, with one empty row at
// the end: a person fills that one to write a new origin, and a row with no kind
// is not an origin (3.8).
func originRows(origins []rules.Origin) []originRow {
	rows := make([]originRow, 0, len(origins)+1)
	for _, origin := range origins {
		rows = append(rows, originRow{
			Kind: origin.Kind,
			Date: origin.Date,
			Refs: strings.Join(origin.Refs, ", "),
			Note: origin.Note,
		})
	}
	return append(rows, originRow{})
}

// originList is the origins a card wrote: a row with no kind stands for no
// origin, and the records of a row are separated by commas.
func originList(rows []originRow) []rules.Origin {
	var out []rules.Origin
	for _, row := range rows {
		kind := strings.TrimSpace(row.Kind)
		if kind == "" {
			continue
		}
		out = append(out, rules.Origin{
			Kind: kind,
			Date: strings.TrimSpace(row.Date),
			Refs: listValues(row.Refs),
			Note: row.Note,
		})
	}
	return out
}

// listValues is a comma-separated list as its values: empty ones are left out.
func listValues(text string) []string {
	var out []string
	for part := range strings.SplitSeq(text, ",") {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// writtenRule is the rule a card wrote: the five things it holds, with the
// report shape and the reason's sending the file wrote kept as they are (W14).
func writtenRule(old rules.Rule, form *ruleForm) rules.Rule {
	next := old
	next.ID = strings.TrimSpace(form.ID)
	next.Text = form.Text
	next.Why = form.Why
	next.Note = form.Note
	next.Origin = originList(form.Origins)
	next.Tests = listValues(form.Tests)
	return next
}

// formOf is the card's fields as a write sent them.
func formOf(r *http.Request) *ruleForm {
	kinds := r.PostForm["origin_kind"]
	dates := r.PostForm["origin_date"]
	refs := r.PostForm["origin_refs"]
	notes := r.PostForm["origin_note"]
	rows := make([]originRow, 0, len(kinds))
	for i, kind := range kinds {
		rows = append(rows, originRow{
			Kind: kind,
			Date: at(dates, i),
			Refs: at(refs, i),
			Note: at(notes, i),
		})
	}
	return &ruleForm{
		ID:      r.PostFormValue("id"),
		Text:    r.PostFormValue("text"),
		Why:     r.PostFormValue("why"),
		Note:    r.PostFormValue("note"),
		Tests:   r.PostFormValue("tests"),
		Origins: rows,
	}
}

// at is one value of a list of form values, empty when the list is shorter.
func at(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

// openRule is the read of one rule's card (/…/<rule>).
func (s *Server) openRule(r *http.Request, place rulesPlace) (view, error) {
	return s.rulesPage(place, &ruleOpen{ID: r.PathValue("rule")}, saved{}, "", false)
}

// addRule is the read of the card a document is adding (/…/new?section=n).
func (s *Server) addRule(r *http.Request, place rulesPlace) (view, error) {
	return s.rulesPage(place, &ruleOpen{Add: true, Section: sectionOf(r)}, saved{}, "", false)
}

// saveRule is the one save path of a rule document (設計 7.3): read the file the
// page read, change one rule, read the change through the catalog, check it with
// the definitions that read the document, and write only when the file is still
// the one the page read.
func (s *Server) saveRule(r *http.Request, place rulesPlace, add bool) (view, error) {
	text, err := s.cat.Reader().Read(place.Doc)
	if err != nil {
		return view{}, err
	}
	doc, err := rules.Parse(text, place.Doc.Path)
	if err != nil {
		return s.rulesPage(place, nil, saved{}, s.breakLine(place.Doc.Path, err), false)
	}
	if r.PostFormValue("action") == "delete" {
		// The first press asks once more in place, and writes nothing (W15).
		return s.rulesPage(place, &ruleOpen{ID: r.PathValue("rule"), Ask: true}, saved{}, "", false)
	}

	form := formOf(r)
	open := &ruleOpen{ID: r.PathValue("rule")}
	edit := rules.Edit{}
	if add {
		section := sectionOf(r)
		if section < 0 || section >= len(doc.Sections) {
			section = len(doc.Sections) - 1
		}
		open = &ruleOpen{Add: true, Section: section}
		edit = rules.Edit{Section: section, Rule: len(doc.Sections[section].Rules), Added: true}
	}
	// What the person's own fields say, before the file's own reading is asked
	// (spec/42 N3): the screen's sentence names the field, and the card the
	// person was writing in stays open.
	if msg := fieldNotice(form); msg != "" {
		return s.rulesPage(place, open, saved{}, msg, false)
	}
	if add {
		if err := doc.AddRule(edit.Section, writtenRule(rules.Rule{}, form)); err != nil {
			return s.rulesPage(place, open, saved{}, err.Error(), false)
		}
	} else {
		id := r.PathValue("rule")
		section, index, ok := findRule(doc, id)
		if !ok {
			return s.rulesPage(place, nil, saved{}, place.missingRule(id), false)
		}
		// The card shows the id as its name, so the id it writes is the one it
		// was opened with.
		form.ID = id
		edit = rules.Edit{Section: section, Rule: index}
		if err := doc.SetRule(section, index, writtenRule(doc.Sections[section].Rules[index], form)); err != nil {
			return s.rulesPage(place, open, saved{}, err.Error(), false)
		}
	}
	return s.writeRules(r, place, text, doc, edit, form, open, false)
}

// fieldNotice is the sentence the screen itself says about the fields a person
// wrote (spec/42 N3): an empty text, and a date the format cannot hold. Both are
// said by the field's name rather than by the file's own numbering
// (sections[0].rules[2]).
func fieldNotice(form *ruleForm) string {
	if strings.TrimSpace(form.Text) == "" {
		return t("web.rule.need_text")
	}
	for _, row := range form.Origins {
		if strings.TrimSpace(row.Kind) == "" || strings.TrimSpace(row.Date) == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(row.Date)); err != nil {
			return t("web.rule.need_date")
		}
	}
	return ""
}

// takeRule is the write whose subject is the document itself: the second press
// of 消す, which takes one rule out (3.8, W15). The rule is named by the form,
// because the request goes to the document's own page — the page the person is
// left on once the card is gone.
func (s *Server) takeRule(r *http.Request, place rulesPlace) (view, error) {
	id := r.PostFormValue("rule")
	text, err := s.cat.Reader().Read(place.Doc)
	if err != nil {
		return view{}, err
	}
	doc, err := rules.Parse(text, place.Doc.Path)
	if err != nil {
		return s.rulesPage(place, nil, saved{}, s.breakLine(place.Doc.Path, err), false)
	}
	section, index, ok := findRule(doc, id)
	if !ok {
		return s.rulesPage(place, nil, saved{}, place.missingRule(id), false)
	}
	edit := rules.Edit{Section: section, Rule: index, Removed: true}
	if err := doc.DeleteRule(section, index); err != nil {
		return s.rulesPage(place, &ruleOpen{ID: id}, saved{}, err.Error(), false)
	}
	return s.writeRules(r, place, text, doc, edit, &ruleForm{ID: id}, nil, true)
}

// writeRules checks the written document and writes it: only the changed rule's
// own lines are written (rules.Write, d-7b58), the written text is read back
// with the format's own reader (so an id or a date the format refuses is refused
// before the file is written), the definitions that read the document are
// checked before and after, and the file is written only when it is still the
// one the page read (設計 7.3). open is the card the page draws afterwards: the
// rule that was written, the card that was adding, or none when a rule was taken
// out. removed says whether the write took the rule out, which the saved line
// says in its own words (設計 3.0 の表、spec/42 N2).
func (s *Server) writeRules(r *http.Request, place rulesPlace, text string, doc *rules.Document, edit rules.Edit, form *ruleForm, open *ruleOpen, removed bool) (view, error) {
	written, err := rules.Write([]byte(text), doc, edit)
	if err != nil {
		return s.rulesPage(place, open, saved{}, err.Error(), false)
	}
	if _, err := rules.Parse(string(written), place.Doc.Path); err != nil {
		return s.rulesPage(place, open, saved{}, s.breakLine(place.Doc.Path, err), false)
	}
	with := s.cat.With(place.Doc.Path, string(written))
	if msg, ok := s.ruleStop(place, with); ok {
		return s.rulesPage(place, open, saved{}, msg, false)
	}
	// The comparison runs before the write: the file still holds the text the
	// page read, which is the "before" side of the change (設計 7.3 step 5).
	line := s.rulesSaved(place, with, s.ruleSavedLine(place, form.ID, removed))
	if err := source.WriteIfUnchanged(place.Doc.Path, r.PostFormValue("hash"), string(written)); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return s.rulesPage(place, open, saved{}, tf("web.saved.clash", s.cat.Display(place.Doc.Path)), true)
		}
		return view{}, err
	}
	return s.rulesPage(place, open, line, "", false)
}

// ruleSavedLine is the line a write to a rule document shows (設計 3.0 版 5 の
// 鍵の語の表): the path, and the rule by the word a person reads ("規則 T2"),
// written or taken out. The rule's own keys carry the quotes the design writes
// around the key word; the older web.saved.line has no quotes yet (the mission
// still passes the YAML key, spec/40 N3).
func (s *Server) ruleSavedLine(place rulesPlace, id string, removed bool) string {
	key := tf("web.rule.key", id)
	if removed {
		return tf("web.rule.removed", s.cat.Display(place.Doc.Path), key)
	}
	return tf("web.rule.saved", s.cat.Display(place.Doc.Path), key)
}

// writeDocument is the write whose subject is the document itself: taking a rule
// out (the second press of 消す), and making a document that is not there (W8).
// A rule's own save posts to the rule's card, so a save never comes here.
func (s *Server) writeDocument(r *http.Request, place rulesPlace) (view, error) {
	if r.PostFormValue("make") != "" {
		return s.makeDocument(r, place)
	}
	return s.takeRule(r, place)
}

// makeDocument writes one empty rule document (W8): a document with one section
// and no rule, which is a document a person can add the first rule to.
func (s *Server) makeDocument(r *http.Request, place rulesPlace) (view, error) {
	// The section an empty document gets carries a heading, so the first rule a
	// person adds stands under one (spec/42 N4).
	text, err := rules.Emit(&rules.Document{Sections: []rules.Section{{Heading: t("web.rule.heading")}}})
	if err != nil {
		return view{}, err
	}
	if err := source.WriteIfUnchanged(place.Doc.Path, r.PostFormValue("hash"), text); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return s.rulesPage(place, nil, saved{}, tf("web.saved.clash", s.cat.Display(place.Doc.Path)), true)
		}
		return view{}, err
	}
	with := s.cat.With(place.Doc.Path, text)
	return s.rulesPage(place, nil, s.rulesSaved(place, with, tf("web.made", s.cat.Display(place.Doc.Path))), "", false)
}

// rulesSaved is the saved line and what a running team takes up after a write to
// one rule document (設計 3.0 の表): the comparison runs once per definition that
// reads the document, so a member that reads it is named once. It is asked
// before the file is written, while the disk still holds the text the page read.
func (s *Server) rulesSaved(place rulesPlace, with *catalog.Catalog, line string) saved {
	var changes []effect.Change
	for _, name := range s.ruleReaders(place.Doc.Path, s.cat, with) {
		before, err := s.cat.Read(name)
		if err != nil {
			continue
		}
		after, err := with.Read(name)
		if err != nil {
			continue
		}
		changes = append(changes, effect.Of(s.view(before, s.cat.Reader()), s.view(after, with.Reader()))...)
	}
	running, _ := s.runningTeams()
	return saved{Line: line, Handover: handoverOf(effect.Commands(changes, running))}
}

// ruleReaders is the definitions that read one rule document, each once, in the
// catalog's order (C8). Both the catalog as it stands and the catalog with the
// write laid over it are asked: a document that is being made is read by the
// members that name it only after the write (W8).
func (s *Server) ruleReaders(path string, catalogs ...*catalog.Catalog) []string {
	var out []string
	seen := map[string]bool{}
	for _, cat := range catalogs {
		for _, ref := range cat.Readers(path) {
			if seen[ref.Definition] {
				continue
			}
			seen[ref.Definition] = true
			out = append(out, ref.Definition)
		}
	}
	return out
}

// ruleFindings is what check writes about one rule document, read through every
// definition that reads it (設計 7.3 step 4).
func (s *Server) ruleFindings(path string, cat *catalog.Catalog) []check.Finding {
	var out []check.Finding
	for _, name := range s.ruleReaders(path, cat) {
		out = append(out, check.Definition(check.Input{Catalog: cat, Name: name})...)
	}
	return out
}

// ruleStop is the sentence a write to a rule document brings in: a stop the
// definitions that read it did not have before. A stop that was already there
// does not stop this write (T1).
func (s *Server) ruleStop(place rulesPlace, with *catalog.Catalog) (string, bool) {
	had := map[string]bool{}
	for _, finding := range s.ruleFindings(place.Doc.Path, s.cat) {
		if finding.Stops {
			had[finding.Field+"\x00"+finding.Text] = true
		}
	}
	for _, finding := range s.ruleFindings(place.Doc.Path, with) {
		if finding.Stops && !had[finding.Field+"\x00"+finding.Text] {
			return finding.Text, true
		}
	}
	return "", false
}

// warningTexts is what check says about one document's rules, by rule id: the
// sentences whose Field points at a card (設計 3.8).
func warningTexts(findings []check.Finding, path string) map[string][]string {
	if path == "" {
		return nil
	}
	out := map[string][]string{}
	prefix := "rule:" + path + "#"
	for _, finding := range findings {
		if id, ok := strings.CutPrefix(finding.Field, prefix); ok {
			out[id] = append(out[id], finding.Text)
		}
	}
	return out
}

// ruleTexts is the same, read through the definitions that read the document.
func (s *Server) ruleTexts(place rulesPlace) map[string][]string {
	return warningTexts(s.ruleFindings(place.Doc.Path, s.cat), place.Doc.Path)
}

// sectionOf is the section a card adds a rule to, from the request.
func sectionOf(r *http.Request) int {
	section, err := strconv.Atoi(r.URL.Query().Get("section"))
	if err != nil {
		return 0
	}
	return section
}

// rulesPage draws the page one rule document stands on, with one card open, and,
// after a write, the saved line and what a running team takes up (設計 3.8).
func (s *Server) rulesPage(place rulesPlace, open *ruleOpen, line saved, notice string, reload bool) (view, error) {
	switch place.Page {
	case pageRole:
		return s.rolePage(place, open, line, notice, reload)
	case pageShared:
		return s.agreementPage(place, open, line, notice, reload)
	default:
		return s.teamDocumentPage(place, open, line, notice, reload)
	}
}

// teamDocumentPage draws the definition's detail, with one rule of its team
// agreement open (設計 3.4 (5)): the same page the mission is written on.
func (s *Server) teamDocumentPage(place rulesPlace, open *ruleOpen, line saved, notice string, reload bool) (view, error) {
	def, err := s.readDefinition(place.Def)
	if err != nil {
		return s.brokenDefinition(place.Def, err)
	}
	var openFn panelFn
	if open != nil {
		openFn = func(*definition.Definition) panel {
			return s.ruleCard(place, *open, notice, reload, line.Line != "", s.ruleTexts(place))
		}
	}
	data := s.definitionData(def, openFn)
	if line.Line != "" {
		data.Saved, data.Handover = line.Line, line.Handover
	}
	if open == nil && notice != "" {
		// A write tried against a document that does not read: the one line
		// stands under the heading, where a file's own sentences stand.
		data.Heading = append(data.Heading, notice)
	}
	v, err := s.renderDefinition(data)
	if err != nil {
		return v, err
	}
	if data.PanelOpen && data.Panel.Missing {
		v.status = http.StatusNotFound
	}
	return v, nil
}
