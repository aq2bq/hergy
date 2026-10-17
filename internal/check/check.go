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
	// Field is the key of the team.yaml field the sentence is about: "mission",
	// "members[1].profile", a turns: line, and "" when it is about the file as a
	// whole. A screen puts the sentence beside that field.
	Field string
	// Text is the sentence, as a person reads it, with the paths written the way
	// the catalog writes them.
	Text string
	// Stops says the definition does not read (a break) rather than a warning. A
	// definition with a finding that stops is one hergy will not run.
	Stops bool
}

// Input is what one check needs: the catalog to read through, the definition's
// name, and the layer to read. An empty layer is the one way a name is looked up
// (the project's own first), which is what a screen shows; hergy validate asks
// one layer at a time, so every file is reported.
type Input struct {
	Catalog *catalog.Catalog
	Name    string
	Layer   string
}

// Definition checks one team definition and returns what it found, in the order
// a person reads it: what stops it (one finding per thing to fix), then the
// warnings.
func Definition(in Input) []Finding {
	var breaks, warnings []Finding
	def, err := in.Catalog.Definition(in.Name, in.Layer)
	// The places hergy no longer reads a team's files from are the filesystem's,
	// not the reading's: they are named whether or not the file reads.
	warnings = append(warnings, oldFileFindings(in)...)
	if err != nil {
		breaks = breakFindings(in, err)
		return append(breaks, warnings...)
	}
	breaks = promptFindings(in, def)
	if len(breaks) > 0 {
		return append(breaks, warnings...)
	}
	warnings = append(warnings, missionPlaceholderFindings(def)...)
	warnings = append(warnings, TurnFindings(in, def)...)
	warnings = append(warnings, hiddenGlobalFindings(in)...)
	return append(breaks, warnings...)
}

// breakFindings is what stops a definition from reading. A graph the definition
// cannot hold is one finding per problem (I12a, I12b): each is a thing to fix,
// and validate has always printed one detail line each (d-edd4).
func breakFindings(in Input, err error) []Finding {
	var gerr *definition.GraphError
	if errors.As(err, &gerr) {
		out := make([]Finding, 0, len(gerr.Problems))
		for _, problem := range gerr.Problems {
			out = append(out, Finding{
				Field: problem.At,
				Text:  BreakLine(in.Catalog.Display, gerr.Path, errors.New(problem.Message())),
				Stops: true,
			})
		}
		return out
	}
	return []Finding{{
		Field: definition.Field(err),
		Text:  BreakLine(in.Catalog.Display, breakPath(in, err), err),
		Stops: true,
	}}
}

// breakPath is the file a break is about: the one the reading stopped on, or the
// layer's team.yaml.
func breakPath(in Input, err error) string {
	var readErr *catalog.ReadError
	if errors.As(err, &readErr) {
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
// report or a rule id two documents of one member both hold, and a herdr name
// that is too long. The definition reads, but no prompt can be handed out, so it
// stops, as it always has.
func promptFindings(in Input, def *definition.Definition) []Finding {
	run := prompt.Run{
		Team: def.Name, Reader: source.Disk{}, Sources: in.Catalog,
		Display: in.Catalog.Display, Base: in.Catalog.Base(),
	}
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		if _, err := prompt.MemberName(run.Team, node); err != nil {
			return []Finding{{Text: BreakLine(in.Catalog.Display, def.TeamYAML, err), Stops: true}}
		}
		if _, err := prompt.Member(def, node, run); err != nil {
			return []Finding{{Text: BreakLine(in.Catalog.Display, def.TeamYAML, err), Stops: true}}
		}
	}
	return nil
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
		out = append(out, Finding{Text: i18n.Tf("validate.old_team_agreement",
			in.Catalog.Display(old), in.Catalog.Display(filepath.Join(dir, "teams", in.Name, "agreement.yaml")))})
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "teams", in.Name, "roles", "*.yaml"))
	slices.Sort(matches)
	for _, path := range matches {
		role := strings.TrimSuffix(filepath.Base(path), ".yaml")
		out = append(out, Finding{Text: i18n.Tf("validate.old_team_role",
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
	return []Finding{{Field: "mission", Text: i18n.T("validate.mission_placeholder")}}
}

// hiddenGlobalFindings says that a global definition of this name is hidden by
// the project's own (I11), where a person reading the global file finds it.
func hiddenGlobalFindings(in Input) []Finding {
	if in.Layer != source.LayerGlobal || !isFile(in.Catalog.TeamYAML(in.Name, source.LayerLocal)) {
		return nil
	}
	return []Finding{{Text: i18n.Tf("validate.global_hidden", in.Name)}}
}

// TurnFindings is the turn directions a team's rule texts claim: the ones turns:
// does not hold (段 5), and the names it writes that are no node of the graph
// (spec/33 H2). A rule text writes a turn as "a → b"; the turns live in turns:
// now, and this says when the two disagree. A name that is a worker's purpose is
// not a node — workers: ties that worker to its member — so nothing is said
// about it.
func TurnFindings(in Input, def *definition.Definition) []Finding {
	members, docs := turnReading(in.Catalog, def)
	var out []Finding
	seen := map[string]bool{}
	for _, edge := range topology.Read(members, docs).Edges {
		if word, ok := strangerSide(edge); ok {
			if isPurpose(def, word) {
				continue
			}
			key := edge.RuleID + "\x00" + word
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Finding{Field: "turns:", Text: i18n.Tf("validate.turn_unknown_name",
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
			out = append(out, Finding{Field: "turns:", Text: i18n.Tf("validate.turn_not_in_turns", edge.RuleID,
				pair[0].String()+" → "+pair[1].String())})
		}
	}
	return out
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

// turnReading is the members and the rule files the reading of a rule text's
// turn arrows needs: the members of the team, and the documents a member reads
// (段 4: the agreement every team of the layer shares, the team's own agreement,
// and each member's role document), with the rules parsed. It reads what the
// definition names, not what one prompt happened to ask for, so a team whose
// prompt drops %{docs} still has its rule texts compared with turns:. A file that
// cannot be read is left out: the definition was built a moment ago with the same
// reading, so the break was already reported.
func turnReading(cat *catalog.Catalog, def *definition.Definition) ([]topology.Member, []topology.Doc) {
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
	var docs []topology.Doc
	seen := map[string]bool{}
	add := func(doc source.Doc, role string) {
		if seen[doc.Path] {
			return
		}
		seen[doc.Path] = true
		text, err := os.ReadFile(doc.Path)
		if err != nil {
			return
		}
		parsed, err := rules.Parse(string(text), doc.Path)
		if err != nil {
			return
		}
		entry := topology.Doc{Key: doc.Path, Label: parsed.Title, Role: role}
		for _, rule := range parsed.Rules() {
			entry.Rules = append(entry.Rules, topology.Rule{ID: rule.ID, Text: rule.Text})
		}
		docs = append(docs, entry)
	}
	if shared, ok := cat.SharedAgreement(); ok {
		add(shared, "")
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
