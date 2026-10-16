package prompt

import (
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// Run is what a prompt needs beyond the definition: the facts of this run, which
// are the team's at run time, not the definition's.
type Run struct {
	// Team is the team name at run time: --as, else the definition name. It
	// prefixes every member's herdr name.
	Team string
	// Executable is the hergy binary, embedded in the hire command a member
	// reads; empty means "hergy".
	Executable string
	// Base is the project root, for cwd values and the embedded hire command.
	Base string
	// Reader reads the rule files the definition names.
	Reader source.Reader
}

// Built is one member's prompt and what it was built from.
type Built struct {
	Text string
	// Role is the role document the prompt read, with HasRole saying whether
	// there was one.
	Role    source.Doc
	HasRole bool
	// Read is the documents the prompt read, in the order they appear: the role
	// document first, then the definition's documents.
	Read []source.Doc
}

// Member builds the bootstrap prompt of one member of a definition (this is the
// projection of refactor-design.md の 1a 節): the role document, the documents
// every member reads, the roster, the contact guide and what it may hire,
// against the template the member, the team or the locales give. It reads the
// graph and adds nothing to it.
func Member(def *definition.Definition, node *definition.MemberNode, run Run) (*Built, error) {
	template := MemberTemplate()
	switch {
	case node.HasPrompt:
		template = node.Prompt
	case def.Prompt != "":
		template = def.Prompt
	}

	var buildErr error
	var roleDoc *source.Doc
	var read []source.Doc
	var parsed []*rules.Document

	readDoc := func(doc source.Doc) string {
		document, err := readRuleFile(run.Reader, doc)
		if err != nil {
			if buildErr == nil {
				buildErr = err
			}
			return ""
		}
		parsed = append(parsed, document)
		return strings.TrimRight(document.Markdown(), "\n")
	}

	memberName := run.Team + "-" + node.Name
	parts := map[string]func() string{
		"team": func() string { return run.Team },
		"role": func() string { return node.Role },
		"name": func() string { return memberName },
		"role_doc": func() string {
			if buildErr != nil {
				return ""
			}
			doc, err := roleFile(node.Role, def.RoleDirs, run.Reader)
			if err != nil {
				buildErr = err
				return ""
			}
			found := doc
			roleDoc = &found
			return readDoc(doc)
		},
		"docs": func() string {
			if buildErr != nil {
				return ""
			}
			texts := make([]string, 0, len(def.Docs))
			for _, doc := range def.Docs {
				texts = append(texts, readDoc(doc))
			}
			read = append(read, def.Docs...)
			if len(texts) == 0 {
				return ""
			}
			return "\n" + strings.Join(texts, "\n\n") + "\n"
		},
		"roster": func() string { return Roster(rosterOf(def, run.Team), memberName) },
		"herdr": func() string {
			return HerdrMember(i18n.Tf("prompt.self_intro_member", run.Team, node.Role, memberName))
		},
		"spawn": func() string {
			workers := workersOf(def, node.Name)
			if len(workers) == 0 {
				return ""
			}
			return SpawnNote(node.Name, workers, hireCommand(def, run, node.Name))
		},
	}
	text, err := Expand(template, parts)
	if err != nil {
		return nil, err
	}
	if buildErr != nil {
		return nil, buildErr
	}
	if err := rules.Check(parsed); err != nil {
		return nil, err
	}
	built := &Built{Text: text, Read: read}
	if roleDoc != nil {
		built.Role = *roleDoc
		built.HasRole = true
	}
	return built, nil
}

// rosterOf is the team's members as the roster shows them: the herdr name each
// one answers to, and the role it reads.
func rosterOf(def *definition.Definition, team string) []RosterEntry {
	out := make([]RosterEntry, len(def.Graph.Members))
	for i, member := range def.Graph.Members {
		out[i] = RosterEntry{Name: team + "-" + member.Name, Role: member.Role}
	}
	return out
}

// workersOf is what a member may hire, from the graph's edges out of it (I12c).
func workersOf(def *definition.Definition, member string) []SpawnPair {
	var out []SpawnPair
	for _, edge := range def.Graph.Out(definition.NodeID{Kind: definition.KindMember, Name: member}) {
		if edge.To.Kind != definition.KindWorker {
			continue
		}
		node, ok := def.Graph.Node(edge.To)
		if !ok {
			continue
		}
		worker, ok := node.(definition.WorkerNode)
		if !ok {
			continue
		}
		out = append(out, SpawnPair{Purpose: worker.Purpose, Profile: worker.Profile})
	}
	return out
}

// readRuleFile reads one rule file and renders it to the Markdown a member's
// prompt carries.
func readRuleFile(reader source.Reader, doc source.Doc) (*rules.Document, error) {
	text, err := reader.Read(doc)
	if err != nil {
		return nil, err
	}
	return rules.Parse(text, doc.Path)
}

// hireCommand is embedded in a member's prompt. It names the exact team, layer
// and project, so a hire works even when the member runs outside the project or
// the same name exists in both layers. --team carries the team name the member
// runs under (hire takes a team name, not a team definition name), so the hire
// finds the running team even when --as named it. The --from value is the
// hiring member's short name: the command line takes the short name, and hergy
// adds the team name to the herdr name itself.
func hireCommand(def *definition.Definition, run Run, shortName string) string {
	layerFlag := "--local"
	if def.Layer == source.LayerGlobal {
		layerFlag = "--global"
	}
	executable := run.Executable
	if executable == "" {
		executable = "hergy"
	}
	return i18n.Tf("hire.command",
		shellQuote(executable), run.Team, layerFlag, i18n.Language(), shellQuote(run.Base), shortName)
}

func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'\\$&|;<>()*?[]#~") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// roleFile is the first role directory that has the file; files are never merged.
func roleFile(role string, dirs []source.Doc, reader source.Reader) (source.Doc, error) {
	for _, dir := range dirs {
		doc := source.Doc{Layer: dir.Layer, Path: joinPath(dir.Path, role+".yaml")}
		if reader.IsFile(doc) {
			return doc, nil
		}
	}
	return source.Doc{}, missingRole(role, dirs, reader)
}

// missingRole says which it is: a role file that is still Markdown, or no role
// file at all.
func missingRole(role string, dirs []source.Doc, reader source.Reader) error {
	for _, dir := range dirs {
		doc := source.Doc{Layer: dir.Layer, Path: joinPath(dir.Path, role+".md")}
		if reader.IsFile(doc) {
			return i18n.Errorf("error.role_markdown", role, doc.Path)
		}
	}
	if len(dirs) == 0 {
		return i18n.Errorf("error.no_role_file_no_dirs", role)
	}
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = dir.Path
	}
	return i18n.Errorf("error.no_role_file_paths", role, strings.Join(paths, ", "))
}

func joinPath(a, b string) string { return filepath.Join(a, b) }
