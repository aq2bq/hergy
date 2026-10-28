// Package check turns one team definition into the sentences a person reads
// about it: what stops hergy from using it, and what it warns about. hergy
// validate and the WebUI print the same findings from here, so a sentence a
// person reads on a screen is the sentence the command line prints (C2).
package check

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/topology"
)

// Finding is one thing a check found about a definition.
type Finding struct {
	// Field is the key of the team.yaml field the sentence belongs beside, so a
	// screen can put it next to what the person writes. The form is the same in
	// both languages (N2):
	//
	//	mission, team, prompt, docs, roles, members  a top-level key
	//	members[i]                                   the member at i (0-based)
	//	members[i].name  .role  .profile  .prompt    one of the member's keys
	//	members[i].kind  .args  .env                 the engine the member writes
	//	turns                                        turns: as a whole
	//	turns[i]                                     the i-th line of turns:
	//	ledger[i]                                    the i-th line of ledger:
	//	workers[i]                                   the worker at i (0-based)
	//	workers[i].from                              the worker's from:
	//	node:human, node:<member>                    a sentence about a node
	//	rule:<document path>#<rule id>               a sentence about a rule's text
	//	""                                           the file as a whole
	Field string
	// Text is the sentence, as a person reads it, with the paths written the way
	// the catalog writes them.
	Text string
	// Kind is what the finding is about. A reader that has to print one sort
	// differently from another reads this (the command line does, N1).
	Kind Kind
	// Stops says the definition does not read (a break) rather than a warning. A
	// definition with a finding that stops is one hergy will not run.
	Stops bool
	// Doc is the displayed path of the document the finding stands in, when it
	// stands in one. A sentence about a rule a role document writes names that
	// document, so a screen can open it (d-8bbf).
	Doc string
	// At is the document's own field the sentence marks, when the finding
	// comes from a rule file: "id", "text", "origin.0.refs.0", a report's name.
	// Field says where the sentence stands (the rule, or the file); At says which
	// part of that rule a screen marks. A screen showing one file uses it; the
	// team's figure does not.
	At string
	// Role is the role whose document a finding about a combination names, and
	// RuleID the rule in that document: enough for a screen to open the role at
	// the rule. Both are empty when the finding is not about a role's rule.
	Role   string
	RuleID string
	// RoleDoc is the displayed path of that role's document. It is what the
	// command line puts before the sentence ("writer.yaml の規則 WRITE-7 が…"),
	// so a person reading hergy validate knows which document to fix (T13 R2).
	RoleDoc string
}

// Kind is what a finding is about.
type Kind int

const (
	// KindRead is a file hergy could not read: broken YAML, a value that does
	// not hold, a key that is not there.
	KindRead Kind = iota
	// KindGraph is a graph the definition cannot hold (I12a, I12b).
	KindGraph
	// KindPrompt is a prompt that cannot be built: a role document that is not
	// there, a template that drops a door (I4).
	KindPrompt
	// KindOldFile is the warning about a file hergy no longer reads a team's
	// rules from (段 4).
	KindOldFile
	// KindMission is the warning about a mission still written as the starter's
	// example (d-3cae).
	KindMission
	// KindTurn is the warning about a turn a rule's text writes and turns: does
	// not hold (段 5, spec/33 H2).
	KindTurn
	// KindHidden is the warning that a global definition of this name is hidden
	// by the project's own (I11).
	KindHidden
	// KindModel is the warning that a role document's rule text names a model
	// (C8): a role document is meant to read the same whatever model runs it.
	KindModel
	// KindStyle is the warning that a rule file still writes a section's style:
	// (d-f4f6): the layout of a section is one way now, and the line does
	// nothing.
	KindStyle
	// KindRule is a problem with one rule file itself, read as part of its own
	// unit: a duplicate id, a derived reference that is not there, a style: line
	// (d-a575). It is not a break of the definition unless it stops the file.
	KindRule
	// KindBetween is a problem that only the combination of the documents one
	// member reads can cause: an id two of them hold, a derived rule across them
	// (d-a575). It belongs to the team definition's check.
	KindBetween
	// KindSize is the warning that one definition is over the provisional size
	// budget of its own unit (d-81c1, d-00a9).
	KindSize
	// KindLedger is the warning that a write a member of this team made broke a
	// rule the definition's ledger: gives (spec-ledger). It does not stop a run
	// (d-db1b).
	KindLedger
)

// Input is what one check needs: the catalog to read through, the definition's
// name, and the layer to read. An empty layer is the one way a name is looked up
// (the project's own first), which is what a screen shows; hergy validate asks
// one layer at a time, so every file is reported.
type Input struct {
	Catalog *catalog.Catalog
	Name    string
	Layer   string
}

// SharedAgreementText is the sentence a definition's check carries, and the
// commands stop with, while a layer still holds agreement.yaml at its top — the
// agreement every team used to share (d-31ad). It names the files, and the two
// places a rule goes now, in the layer the first file was found in.
func SharedAgreementText(cat *catalog.Catalog, docs []source.Doc) string {
	paths := make([]string, 0, len(docs))
	for _, doc := range docs {
		paths = append(paths, cat.Display(doc.Path))
	}
	layer := cat.LayerDir(docs[0].Layer)
	return i18n.Tf("error.shared_agreement_gone",
		strings.Join(paths, i18n.T("list.separator")),
		cat.Display(filepath.Join(layer, "teams", i18n.T("place.team_definition"), "agreement.yaml")),
		cat.Display(filepath.Join(layer, "roles", i18n.T("place.role"))+".yaml"))
}

// sharedAgreementFindings is the one break every definition's check carries
// while a layer still holds the agreement every team used to share: the screens
// show it on every row and under the figure, so a leftover is not silent there
// while the commands stop on it (d-31ad, T13 R1).
func sharedAgreementFindings(cat *catalog.Catalog) []Finding {
	docs := cat.SharedAgreements()
	if len(docs) == 0 {
		return nil
	}
	return []Finding{{Kind: KindRead, Stops: true, Text: SharedAgreementText(cat, docs)}}
}

// Definition checks one team definition and returns what it found, in the order
// a person reads it: what stops it (one finding per thing to fix), then the
// warnings.
//
// A definition that reads is reported whole: every member whose prompt cannot be
// built, and every warning, so a screen shows the list at once rather than one
// more sentence per fixing (N1). A definition that does not read reports what
// stopped it and the warnings about old files, as it always has. The command
// line prints a part of what a reading definition gets, so its own output does
// not change (see internal/app).
func Definition(in Input) []Finding {
	def, err := in.Catalog.Definition(in.Name, in.Layer)
	// The places hergy no longer reads a team's files from are the filesystem's,
	// not the reading's: they are named whether or not the file reads.
	warnings := oldFileFindings(in)
	if err != nil {
		return append(breakFindings(in, err), warnings...)
	}
	breaks := roleNameFindings(in, def)
	breaks = append(breaks, promptFindings(in, def)...)
	breaks = append(breaks, profileFindings(in, def)...)
	agreementBreaks, agreementWarnings := agreementFinding(in, def)
	betweenBreaks, betweenWarnings := betweenFindings(in, def)
	breaks = append(breaks, agreementBreaks...)
	breaks = append(breaks, betweenBreaks...)
	breaks = dropPromptDuplicates(in, breaks)
	warnings = append(warnings, missionPlaceholderFindings(def)...)
	warnings = append(warnings, TurnFindings(in, def)...)
	warnings = append(warnings, agreementWarnings...)
	warnings = append(warnings, betweenWarnings...)
	warnings = append(warnings, teamSizeFinding(in.Catalog, def)...)
	warnings = append(warnings, hiddenGlobalFindings(in)...)
	return append(breaks, warnings...)
}

// dropPromptDuplicates removes a prompt break that a rule-file finding of the
// same check already says better. A duplicate id between a role document and the
// team's agreement makes the prompt unbuildable and is also the combination's
// finding (betweenFindings); the person is shown the one standing on the rule,
// not the same sentence twice. A duplicate inside one document is not in the
// combination's findings, so its prompt break stays: the team's prompt cannot be
// built even though the problem is the document's own.
func dropPromptDuplicates(in Input, findings []Finding) []Finding {
	said := map[string]bool{}
	for _, finding := range findings {
		if finding.Kind == KindBetween || finding.Kind == KindRule {
			said[finding.Text] = true
		}
	}
	layer := in.Layer
	if layer == "" {
		layer = resolvedLayer(in)
	}
	teamYAML := in.Catalog.Display(in.Catalog.TeamYAML(in.Name, layer))
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Kind == KindPrompt && said[strings.TrimPrefix(finding.Text, teamYAML+": ")] {
			continue
		}
		out = append(out, finding)
	}
	return out
}

// agreementFinding is the team's own rules read as its unit (d-a575): the
// document's own problems (a repeated id, a report or a derived rule that is not
// there, a style: line) and the model names its rule texts write. A rule's turn
// warnings are TurnFindings's, which reads the agreement with the team's turns.
//
// The document is the team's own, so none of this belongs beside a role
// document: a role document's own problems are check.Role's.
func agreementFinding(in Input, def *definition.Definition) (breaks, warnings []Finding) {
	if !def.HasAgreement {
		return nil, nil
	}
	text, err := in.Catalog.Reader().Read(def.Agreement)
	if err != nil {
		return nil, nil
	}
	display := in.Catalog.Display(def.Agreement.Path)
	doc, problems := rules.ReadAll(text, display)
	if doc != nil {
		problems = append(problems, localRuleProblems(doc)...)
	}
	for _, p := range problems {
		finding := Finding{
			Field:  problemField(p, display),
			At:     p.Field,
			Text:   p.Text,
			Kind:   KindRule,
			Stops:  p.Sev == rules.SevError,
			Doc:    display,
			RuleID: p.RuleID(),
		}
		if finding.Stops {
			breaks = append(breaks, finding)
		} else {
			warnings = append(warnings, finding)
		}
	}
	if doc == nil {
		return breaks, warnings
	}
	for _, finding := range modelDocFindings(display, doc) {
		finding.Doc = display
		finding.At = "text"
		warnings = append(warnings, finding)
	}
	return breaks, warnings
}

// betweenFindings is what only the documents one member reads can say together
// (d-a575): an id that stands in the member's role document and in the team's
// agreement, a derived rule that points across them, a report one names and the
// other holds. Each finding names the document it stands in, and the role whose
// document a rule belongs to, so a screen can open that role at the rule
// (d-8bbf).
func betweenFindings(in Input, def *definition.Definition) (breaks, warnings []Finding) {
	if !def.HasAgreement || def.Graph == nil {
		return nil, nil
	}
	agreement := parseDisplayed(in.Catalog, def.Agreement)
	if agreement == nil {
		return nil, nil
	}
	var outBreaks, outWarnings []Finding
	seen := map[string]bool{}
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		roleDoc, ok := in.Catalog.Role(member.Role)
		if !ok {
			continue
		}
		role := parseDisplayed(in.Catalog, roleDoc)
		if role == nil {
			continue
		}
		rolePath := in.Catalog.Display(roleDoc.Path)
		for _, p := range rules.CheckAll([]*rules.Document{role, agreement}) {
			if !betweenDocProblem(p, role, agreement) {
				continue
			}
			field := problemField(p, p.Path)
			key := field + "\x00" + p.Text
			if seen[key] {
				continue
			}
			seen[key] = true
			finding := Finding{
				Field:  field,
				At:     p.Field,
				Text:   p.Text,
				Kind:   KindBetween,
				Stops:  p.Sev == rules.SevError,
				Doc:    p.Path,
				RuleID: p.RuleID(),
			}
			if p.Path == rolePath || (p.Field == "id" && hasRuleID(role, p.RuleID())) {
				finding.Role = member.Role
				finding.RoleDoc = rolePath
			}
			if finding.Stops {
				outBreaks = append(outBreaks, finding)
			} else {
				outWarnings = append(outWarnings, finding)
			}
		}
	}
	return outBreaks, outWarnings
}

// betweenDocProblem says whether one problem of the two documents a member reads
// is the combination's rather than one document's own. A repeated id or report
// is the combination's only when both documents hold it; a repeated name inside
// one document belongs to that document's own check (check.Role for a role,
// agreementFinding for a team's agreement). A report or a derived reference that
// is not there is the combination's: either document may hold what it names.
func betweenDocProblem(p rules.Problem, role, agreement *rules.Document) bool {
	switch p.Field {
	case "id":
		if p.RuleID() == "" {
			return false
		}
		return hasRuleID(role, p.RuleID()) && hasRuleID(agreement, p.RuleID())
	default:
		// The new rule model has no report or derived references; an id that
		// stands in one document alone is that document's own problem.
		return false
	}
}

// localRuleProblems is the problems one document has by itself that ReadAll does
// not gather: an id or a report name that stands twice in it. The references
// that point outside it are not named here: whether one is missing is the
// combination's question (betweenFindings).
func localRuleProblems(doc *rules.Document) []rules.Problem {
	var out []rules.Problem
	for _, p := range rules.CheckAll([]*rules.Document{doc}) {
		if p.Field == "id" {
			out = append(out, p)
		}
	}
	return out
}

// problemField is the field a rule-file problem stands beside: the rule it is
// about, in the document the problem stands in, so a screen puts the sentence
// on that rule's slip. A problem about the file as a whole keeps no field.
func problemField(p rules.Problem, path string) string {
	if p.RuleID() == "" {
		return p.Field
	}
	return "rule:" + path + "#" + p.RuleID()
}

// parseDisplayed reads one rule document through the catalog's path, the way a
// person reads it, so the sentences about it name the file the same way a screen
// does.
func parseDisplayed(cat *catalog.Catalog, doc source.Doc) *rules.Document {
	text, err := cat.Reader().Read(doc)
	if err != nil {
		return nil
	}
	parsed, err := rules.Parse(text, cat.Display(doc.Path))
	if err != nil {
		return nil
	}
	return parsed
}

// hasRuleID reports whether a document holds a rule id.
func hasRuleID(doc *rules.Document, id string) bool {
	if id == "" {
		return false
	}
	for _, rule := range doc.Rules() {
		if rule.ID == id {
			return true
		}
	}
	return false
}

// modelDocFindings is one role document's warnings: one finding per rule whose
// text names a model, in the written order, with the field that puts the
// sentence on that rule's slip (N3).
func modelDocFindings(path string, doc *rules.Document) []Finding {
	var out []Finding
	for _, rule := range doc.Rules() {
		words := rules.ModelWords(rule.Text)
		if len(words) == 0 {
			continue
		}
		out = append(out, Finding{
			Field: "rule:" + path + "#" + rule.ID,
			Kind:  KindModel,
			Text: i18n.Tf("validate.rule_model_name", rule.ID,
				strings.Join(words, i18n.T("list.separator"))),
		})
	}
	return out
}

// breakFindings is what stops a definition from reading. A graph the definition
// cannot hold is one finding per problem (I12a, I12b): each is a thing to fix,
// and validate has always printed one detail line each (d-edd4).
func breakFindings(in Input, err error) []Finding {
	if gerr, ok := errors.AsType[*definition.GraphError](err); ok {
		out := make([]Finding, 0, len(gerr.Problems))
		for _, problem := range gerr.Problems {
			out = append(out, Finding{
				Field: problem.Field,
				Text:  BreakLine(in.Catalog.Display, gerr.Path, errors.New(problem.Message())),
				Kind:  KindGraph,
				Stops: true,
			})
		}
		return out
	}
	return []Finding{{
		Field: definition.Field(err),
		Text:  BreakLine(in.Catalog.Display, breakPath(in, err), err),
		Kind:  KindRead,
		Stops: true,
	}}
}

// breakPath is the file a break is about: the one the reading stopped on, or the
// layer's team.yaml.
func breakPath(in Input, err error) string {
	if readErr, ok := errors.AsType[*catalog.ReadError](err); ok {
		return readErr.Path
	}
	return in.Catalog.TeamYAML(in.Name, in.Layer)
}

// BreakLine is one broken settings file as one line: where it is, as a person
// writes it, and why, with the whole reason on one line. hergy validate and the
// WebUI both show a broken file this way.
func BreakLine(display func(string) string, path string, err error) string {
	msg := strings.ReplaceAll(err.Error(), path, display(path))
	// Some team-file errors do not name the file; the file-level parser errors
	// already do. Show the location once either way.
	if !strings.Contains(msg, display(path)) {
		msg = fmt.Sprintf("%s: %s", display(path), msg)
	}
	return strings.Join(strings.Fields(msg), " ")
}

// withoutOwnName is one problem's sentence without the name of the file the
// problem stands in, which the sentence carries at its head: the path a person
// reads the file by, or the file's own name when the problem is about the file
// as a whole. The place a person reads a problem of one document — a screen
// panel's head, the YAML tab's file — names that file already, so the head would
// be read twice. What the sentence keeps is the location it gives at its end and
// the names of the other files it is about, which no line above gives (lead
// 2026-10-21).
func withoutOwnName(text, file string) string {
	for _, head := range []string{file + ": ", filepath.Base(file) + ": "} {
		if after, ok := strings.CutPrefix(text, head); ok {
			return after
		}
	}
	return text
}

// promptFindings is the break a member's prompt cannot be built from: a role
// document that is not there (spec/32 D2), a template that drops a door (I4), a
// rule id two documents of one member both hold, and a herdr name that is too
// long. The definition reads, but no prompt can be handed out, so it stops, as
// it always has. Every member that cannot be built is reported, so a screen
// shows the whole list at once (N1). Each prompt is built through the catalog, so
// a text the screen is editing (C3b, With) is the text that is judged, rather
// than the file on disk.
func promptFindings(in Input, def *definition.Definition) []Finding {
	run := prompt.Run{
		Team: def.Name, Reader: in.Catalog.Reader(), Sources: in.Catalog,
		Display: in.Catalog.Display, Base: in.Catalog.Base(),
	}
	var out []Finding
	seen := map[string]bool{}
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		key, err := memberPromptError(def, node, run, i)
		if err == nil {
			continue
		}
		finding := Finding{
			Field: key,
			Text:  BreakLine(in.Catalog.Display, def.TeamYAML, err),
			Kind:  KindPrompt,
			Stops: true,
		}
		// A template the whole team writes fails for every member with one
		// sentence beside one field: it is reported once.
		one := finding.Field + "\x00" + finding.Text
		if seen[one] {
			continue
		}
		seen[one] = true
		out = append(out, finding)
	}
	return out
}

// profileFindings names a profile a member or a worker writes and profiles.yaml
// does not hold (spec-teams 1.9): the definition reads — a screen draws it, with
// the mission, the numbers and the figure, so a person can fix the name — but
// hergy does not start the member from it. The reading no longer refuses this
// name, so the sentence is written here, and it is the same sentence the reading
// used to stop with.
// roleNameFindings is the break a member makes whose role is not a role name:
// the reading of team.yaml holds such a member, but hergy cannot build the
// definition from it (app.startsOf stops on catalog.RoleNameProblem), so the
// check says it as the definition's own break rather than leaving the team
// unbuildable and silent (r-fc16, d-841e). The sentence is the one the team's
// save and the roles screen use, and it says how to fix the name.
func roleNameFindings(in Input, def *definition.Definition) []Finding {
	if def.Graph == nil {
		return nil
	}
	var out []Finding
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		err := catalog.RoleNameProblem(node.Role)
		if err == nil {
			continue
		}
		label := fmt.Sprintf("members[%d]", i)
		out = append(out, Finding{
			Field: label + ".role",
			Text:  BreakLine(in.Catalog.Display, def.TeamYAML, err),
			Kind:  KindRead,
			Stops: true,
		})
	}
	return out
}

func profileFindings(in Input, def *definition.Definition) []Finding {
	if def.Graph == nil {
		return nil
	}
	profiles, _, err := in.Catalog.Profiles()
	if err != nil {
		return nil
	}
	var out []Finding
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		if _, ok := profiles[node.Profile]; ok {
			continue
		}
		label := fmt.Sprintf("members[%d]", i)
		out = append(out, Finding{
			Field: label + ".profile",
			Text:  BreakLine(in.Catalog.Display, def.TeamYAML, i18n.Errorf("error.no_profile", label, node.Profile)),
			Kind:  KindRead,
			Stops: true,
		})
	}
	for i := range def.Graph.Workers {
		worker := &def.Graph.Workers[i]
		if _, ok := profiles[worker.Profile]; ok {
			continue
		}
		label := fmt.Sprintf("workers[%d]", i)
		out = append(out, Finding{
			Field: label,
			Text:  BreakLine(in.Catalog.Display, def.TeamYAML, i18n.Errorf("error.workers_no_profile", label, worker.Profile)),
			Kind:  KindRead,
			Stops: true,
		})
	}
	return out
}

// memberPromptError is what stops one member's prompt from being built, and the
// field it is about.
func memberPromptError(def *definition.Definition, node *definition.MemberNode, run prompt.Run, i int) (string, error) {
	if _, err := prompt.MemberName(run.Team, node); err != nil {
		return fmt.Sprintf("members[%d].name", i), err
	}
	if _, err := prompt.Member(def, node, run); err != nil {
		return promptField(err, node, i), err
	}
	return "", nil
}

// promptField is the field a prompt error is about (N2): the member's role
// document, the template that dropped a door (the team's own or the member's
// own), or the member's documents taken together.
func promptField(err error, node *definition.MemberNode, i int) string {
	var perr *prompt.Error
	if !errors.As(err, &perr) {
		return ""
	}
	switch perr.Where {
	case prompt.WhereRole:
		return fmt.Sprintf("members[%d].role", i)
	case prompt.WhereTemplate:
		if node.HasPrompt {
			return fmt.Sprintf("members[%d].prompt", i)
		}
		return "prompt"
	case prompt.WhereRules:
		return fmt.Sprintf("members[%d]", i)
	default:
		return ""
	}
}

// oldFileFindings warns about files a team definition used to keep where hergy
// no longer reads them (段 4, spec/32 D1): a protocol.yaml that was not renamed
// to agreement.yaml, and role documents that were not moved out of the
// definition's folder. The warning names each file and the place it goes now,
// and moves no exit code: the definition still reads, but the rules those files
// hold are read no more.
func oldFileFindings(in Input) []Finding {
	layer := in.Layer
	if layer == "" {
		layer = resolvedLayer(in)
	}
	dir := in.Catalog.LayerDir(layer)
	var out []Finding
	old := filepath.Join(dir, "teams", in.Name, "protocol.yaml")
	if isFile(old) {
		out = append(out, Finding{Kind: KindOldFile, Text: i18n.Tf("validate.old_team_agreement",
			in.Catalog.Display(old), in.Catalog.Display(filepath.Join(dir, "teams", in.Name, "agreement.yaml")))})
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "teams", in.Name, "roles", "*.yaml"))
	slices.Sort(matches)
	for _, path := range matches {
		role := strings.TrimSuffix(filepath.Base(path), ".yaml")
		out = append(out, Finding{Kind: KindOldFile, Text: i18n.Tf("validate.old_team_role",
			in.Catalog.Display(path), in.Catalog.Display(filepath.Join(dir, "roles", role+".yaml")))})
	}
	return out
}

// resolvedLayer is the layer a definition with no layer asked for is in: the way
// a name is looked up, the project's own first.
func resolvedLayer(in Input) string {
	if isFile(in.Catalog.TeamYAML(in.Name, source.LayerLocal)) {
		return source.LayerLocal
	}
	return source.LayerGlobal
}

// missionPlaceholderFindings is the one warning a starter team gets: hergy new
// writes an example mission, and a definition that still writes it has not said
// what its team is for (d-3cae). It moves no exit code.
func missionPlaceholderFindings(def *definition.Definition) []Finding {
	if !scaffold.IsPlaceholderMission(def.Mission) {
		return nil
	}
	return []Finding{{Field: "mission", Text: i18n.T("validate.mission_placeholder"), Kind: KindMission}}
}

// hiddenGlobalFindings says that a global definition of this name is hidden by
// the project's own (I11), where a person reading the global file finds it.
func hiddenGlobalFindings(in Input) []Finding {
	if in.Layer != source.LayerGlobal || !isFile(in.Catalog.TeamYAML(in.Name, source.LayerLocal)) {
		return nil
	}
	return []Finding{{Text: i18n.Tf("validate.global_hidden", in.Name), Kind: KindHidden}}
}

// TurnFindings is the turn directions a team's rule texts claim: the ones turns:
// does not hold (段 5), and the names it writes that are no node of the graph
// (spec/33 H2). A rule text writes a turn as "a → b"; the turns live in turns:
// now, and this says when the two disagree. A name that is a worker's purpose is
// not a node — workers: ties that worker to its member — so nothing is said
// about it.
func TurnFindings(in Input, def *definition.Definition) []Finding {
	members, docs := turnReading(in.Catalog, def)
	roles := roleOfDoc(in.Catalog, def)
	var out []Finding
	seen := map[string]bool{}
	for _, edge := range topology.Read(members, docs).Edges {
		role := roles[edge.DocKey]
		roleDoc := ""
		if role != "" {
			roleDoc = edge.DocKey
		}
		if word, ok := strangerSide(edge); ok {
			if isPurpose(def, word) {
				continue
			}
			key := edge.RuleID + "\x00" + word
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Finding{Field: ruleField(edge), At: "text", Kind: KindTurn, Doc: edge.DocKey, Role: role, RoleDoc: roleDoc, RuleID: edge.RuleID, Text: i18n.Tf("validate.turn_unknown_name",
				edge.RuleID, edgeText(edge, members), word)})
			continue
		}
		for _, pair := range edgePairs(edge, members) {
			if graphHasTurn(def.Graph, pair[0], pair[1]) {
				continue
			}
			key := edge.RuleID + "\x00" + pair[0].String() + "\x00" + pair[1].String()
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Finding{Field: ruleField(edge), At: "text", Kind: KindTurn, Doc: edge.DocKey, Role: role, RoleDoc: roleDoc, RuleID: edge.RuleID, Text: i18n.Tf("validate.turn_not_in_turns", edge.RuleID,
				pair[0].String()+" → "+pair[1].String())})
		}
	}
	return out
}

// roleOfDoc is the role each of a team's rule documents belongs to, by the path
// a person reads it by. The team's own agreement has no role, and neither does
// a document a role no longer names.
func roleOfDoc(cat *catalog.Catalog, def *definition.Definition) map[string]string {
	out := map[string]string{}
	for i := range def.Graph.Members {
		role := def.Graph.Members[i].Role
		doc, ok := cat.Role(role)
		if !ok {
			continue
		}
		out[cat.Display(doc.Path)] = role
	}
	return out
}

// ruleField is the field a sentence about a rule's text belongs beside: the
// document the rule is written in, and the rule's id, so a screen puts the
// sentence on that rule's slip (N3).
func ruleField(edge topology.Edge) string {
	return "rule:" + edge.DocKey + "#" + edge.RuleID
}

// edgeText is the arrow of a rule text as a person reads it: the names it
// matched, and the words it wrote for the ones it did not.
func edgeText(edge topology.Edge, members []topology.Member) string {
	return sideText(edge.From, members) + " " + string(edgeArrow(edge)) + " " + sideText(edge.To, members)
}

// edgeArrow is the arrow the rule text wrote.
func edgeArrow(edge topology.Edge) rune {
	if edge.Both {
		return '⇄'
	}
	return '→'
}

// sideText is one end of an arrow as a person reads it: the node's name when it
// matched, and the written word when it did not.
func sideText(side topology.Side, members []topology.Member) string {
	if side.Kind == topology.SideStranger {
		return side.Word
	}
	if id, ok := turnNode(side, members); ok {
		return id.String()
	}
	return ""
}

// strangerSide is the word an arrow wrote that matched no member of the team, and
// whether there is one. An arrow with both ends matching has none.
func strangerSide(edge topology.Edge) (string, bool) {
	if edge.From.Kind == topology.SideStranger {
		return edge.From.Word, true
	}
	if edge.To.Kind == topology.SideStranger {
		return edge.To.Word, true
	}
	return "", false
}

// isPurpose reports whether a word is a purpose some member's workers: names: a
// worker is not a node of the graph, and its turn to the member that hires it is
// workers:'s, not the rule text's.
func isPurpose(def *definition.Definition, word string) bool {
	for _, worker := range def.Graph.Workers {
		if worker.Purpose == word {
			return true
		}
	}
	return false
}

// edgePairs is the turns one rule-text arrow claims: one, or both ways for "⇄".
// An arrow with an end the graph has no node for claims none of them.
func edgePairs(edge topology.Edge, members []topology.Member) [][2]definition.NodeID {
	from, okFrom := turnNode(edge.From, members)
	to, okTo := turnNode(edge.To, members)
	if !okFrom || !okTo {
		return nil
	}
	pairs := [][2]definition.NodeID{{from, to}}
	if edge.Both {
		pairs = append(pairs, [2]definition.NodeID{to, from})
	}
	return pairs
}

// turnNode is one end of a rule-text arrow as a node of the graph: a member the
// team has, or the human. A name that matched no member is not a node.
func turnNode(side topology.Side, members []topology.Member) (definition.NodeID, bool) {
	switch side.Kind {
	case topology.SideHuman:
		return definition.NodeID{Kind: definition.KindHuman, Name: "human"}, true
	case topology.SideMember:
		if side.Member >= 0 && side.Member < len(members) {
			return definition.NodeID{Kind: definition.KindMember, Name: members[side.Member].Name}, true
		}
	case topology.SideStranger:
		// A worker's purpose, or a name the team has no node for: not a node.
	}
	return definition.NodeID{}, false
}

// graphHasTurn is whether the graph already holds this turn.
func graphHasTurn(g *definition.Graph, from, to definition.NodeID) bool {
	for _, edge := range g.Turns {
		if edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}

// topologyMembers is the members of one definition as the arrow reader wants
// them (topology.Member): the short name, the role, and the workers that member
// may hire, so a word a rule's text writes can be told from a purpose.
func topologyMembers(def *definition.Definition) []topology.Member {
	members := make([]topology.Member, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		var workers []topology.Worker
		for _, worker := range def.Graph.Workers {
			if worker.From == member.Name {
				workers = append(workers, topology.Worker{Purpose: worker.Purpose, Profile: worker.Profile})
			}
		}
		members = append(members, topology.Member{Name: member.Name, Role: member.Role, Workers: workers})
	}
	return members
}

// turnReading is the members and the rule files the reading of a rule text's
// turn arrows needs: the members of the team, and the documents a member reads
// (段 4: the agreement every team of the layer shares, the team's own agreement,
// and each member's role document), with the rules parsed. It reads what the
// definition names, not what one prompt happened to ask for, so a team whose
// prompt drops %{docs} still has its rule texts compared with turns:. It reads
// through the catalog, so a text the screen is editing (C3b, With) is the text
// that is compared, rather than the file on disk. A file that cannot be read is
// left out: the definition was built a moment ago with the same reading, so the
// break was already reported.
func turnReading(cat *catalog.Catalog, def *definition.Definition) ([]topology.Member, []topology.Doc) {
	members := topologyMembers(def)
	var docs []topology.Doc
	seen := map[string]bool{}
	reader := cat.Reader()
	add := func(doc source.Doc, role string) {
		if seen[doc.Path] {
			return
		}
		seen[doc.Path] = true
		text, err := reader.Read(doc)
		if err != nil {
			return
		}
		parsed, err := rules.Parse(string(text), doc.Path)
		if err != nil {
			return
		}
		entry := topology.Doc{Key: cat.Display(doc.Path), Label: parsed.Title, Role: role}
		for _, rule := range parsed.Rules() {
			entry.Rules = append(entry.Rules, topology.Rule{ID: rule.ID, Text: rule.Text})
		}
		docs = append(docs, entry)
	}
	if def.HasAgreement {
		add(def.Agreement, "")
	}
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		if doc, ok := cat.Role(member.Role); ok {
			add(doc, member.Role)
		}
	}
	return members, docs
}

// isFile reports whether a path is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
