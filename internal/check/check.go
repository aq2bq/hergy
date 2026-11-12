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

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// Finding is one thing a check found about a definition.
type Finding struct {
	// Field is the key of the team.yaml field the sentence belongs beside, so a
	// screen can put it next to what the person writes. The form is the same in
	// both languages (N2):
	//
	//	description, team, prompt, docs, roles, members  a top-level key
	//	members[i]                                   the member at i (0-based)
	//	members[i].name  .role  .profile  .prompt    one of the member's keys
	//	members[i].kind  .args  .env                 the engine the member writes
	//	turns                                        turns: as a whole
	//	turns[i]                                     the i-th line of turns:
	//	ledger[i]                                    the i-th line of ledger:
	//	workers[i]                                   the worker at i (0-based)
	//	workers[i].from                              the worker's from:
	//	node:human, node:<member>                    a sentence about a node
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
	// Doc is the displayed path of the document the finding stands in.
	Doc string
	// At is the document field the finding marks.
	At string
	// Role names the role whose document the finding concerns.
	Role string
	// RoleDoc is the displayed path the command line puts before the finding.
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
	// KindDescription is the warning about a description still written as the
	// starter's example (d-3cae).
	KindDescription
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
		cat.Display(filepath.Join(layer, "teams", i18n.T("place.team_definition"), "agreement.md")),
		cat.Display(filepath.Join(layer, "roles", i18n.T("place.role"))+".md"))
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
	warnings = append(warnings, descriptionPlaceholderFindings(def)...)
	warnings = append(warnings, teamSizeFinding(in.Catalog, def)...)
	warnings = append(warnings, hiddenGlobalFindings(in)...)
	return append(breaks, warnings...)
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
		Team: def.Name.String(), Reader: in.Catalog.Reader(), Sources: in.Catalog,
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
// the description, the numbers and the figure, so a person can fix the name — but
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
	if _, err := domainname.MemberForInput(run.Team, node.Name); err != nil {
		return fmt.Sprintf("members[%d].name", i), i18n.Errorf("error.herdr_name_pattern", domainname.PreviewName(run.Team, node.Name.String()))
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
			in.Catalog.Display(old), in.Catalog.Display(filepath.Join(dir, "teams", in.Name, "agreement.md")))})
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "teams", in.Name, "roles", "*.yaml"))
	slices.Sort(matches)
	for _, path := range matches {
		role := strings.TrimSuffix(filepath.Base(path), ".yaml")
		out = append(out, Finding{Kind: KindOldFile, Text: i18n.Tf("validate.old_team_role",
			in.Catalog.Display(path), in.Catalog.Display(filepath.Join(dir, "roles", role+".md")))})
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

// descriptionPlaceholderFindings is the one warning a starter team gets: hergy
// new writes an example description, and a definition that still writes it has
// not said which case its team is meant for (d-3cae). It moves no exit code.
func descriptionPlaceholderFindings(def *definition.Definition) []Finding {
	if !scaffold.IsPlaceholderDescription(def.Description) {
		return nil
	}
	return []Finding{{Field: "description", Text: i18n.T("validate.description_placeholder"), Kind: KindDescription}}
}

// hiddenGlobalFindings says that a global definition of this name is hidden by
// the project's own (I11), where a person reading the global file finds it.
func hiddenGlobalFindings(in Input) []Finding {
	if in.Layer != source.LayerGlobal || !isFile(in.Catalog.TeamYAML(in.Name, source.LayerLocal)) {
		return nil
	}
	return []Finding{{Text: i18n.Tf("validate.global_hidden", in.Name), Kind: KindHidden}}
}

// isFile reports whether a path is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
