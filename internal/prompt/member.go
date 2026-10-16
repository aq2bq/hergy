package prompt

import (
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
	// Reader reads the rule files the prompt names.
	Reader source.Reader
	// Sources is where the prompt gets the two documents that are not the
	// definition's own folder: one member's role document, and the agreement
	// every team of the layer shares. internal/catalog answers it, so the layer
	// stays in one place (K1, I11).
	Sources Sources
}

// Sources is the one place a role document and the shared agreement are looked
// up (K1). internal/catalog implements it; a prompt reads through it and never
// sees a layer.
type Sources interface {
	// Role is the role document for a role name, and whether a layer has one.
	Role(name string) (source.Doc, bool)
	// RoleMarkdown is a role document still left as Markdown, for the sentence
	// that says how to move it over. It is asked only when Role found none.
	RoleMarkdown(name string) (source.Doc, bool)
	// SharedAgreement is the agreement every team of a layer shares, and
	// whether a layer wrote one.
	SharedAgreement() (source.Doc, bool)
}

// Built is one member's prompt and what it was built from.
type Built struct {
	Text string
	// Role is the role document the prompt read, with HasRole saying whether
	// there was one.
	Role    source.Doc
	HasRole bool
	// Read is the documents the prompt read besides the role document, in the
	// order they appear: the agreement every team of the layer shares, then the
	// team's own.
	Read []source.Doc
}

// Member builds the bootstrap prompt of one member of a definition (this is the
// projection of refactor-design.md の 1a 節): the role document, the agreements,
// the roster, the contact guide and what it may hire, against the template the
// member, the team or the locales give. It reads the graph and the catalog and
// adds nothing to either.
func Member(def *definition.Definition, node *definition.MemberNode, run Run) (*Built, error) {
	template := MemberTemplate()
	switch {
	case node.HasPrompt:
		template = node.Prompt
	case def.Prompt != "":
		template = def.Prompt
	}
	if err := checkTemplate(template); err != nil {
		return nil, err
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
			doc, err := memberRole(node, run)
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
			docs := documentsOf(def, run)
			texts := make([]string, 0, len(docs))
			for _, doc := range docs {
				texts = append(texts, readDoc(doc))
			}
			read = append(read, docs...)
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
			// The worker reports back to this member: the note names the herdr
			// name, the same name the worker's own prompt names, so it follows
			// --as (the hire command takes the short name).
			return SpawnNote(memberName, workers, hireCommand(run, node.Name))
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

// hireCommand is embedded in a member's prompt. It names the exact team and
// project, so a hire works even when the member runs outside the project.
// --team carries the team name the member runs under (hire takes a team name,
// not a team definition name), so the hire finds the running team even when
// --as named it, and the project's own definition wins by the one way a name is
// looked up (段 3). The --from value is the hiring member's short name: the
// command line takes the short name, and hergy adds the team name to the herdr
// name itself.
func hireCommand(run Run, shortName string) string {
	executable := run.Executable
	if executable == "" {
		executable = "hergy"
	}
	return i18n.Tf("hire.command",
		shellQuote(executable), run.Team, i18n.Language(), shellQuote(run.Base), shortName)
}

func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'\\$&|;<>()*?[]#~") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// checkTemplate stops a template that would drop the role document or the
// agreements from a member's prompt (d-0ccd): a member or a team that writes its
// own prompt still has to leave the two doors in (I4).
func checkTemplate(template string) error {
	for _, part := range []string{"role_doc", "docs"} {
		if !strings.Contains(template, "%{"+part+"}") {
			return i18n.Errorf("error.prompt_missing_part", part)
		}
	}
	return nil
}

// documentsOf is what every member of a definition reads besides its own role
// document, in the order a prompt joins them: the agreement every team of the
// layer shares, then the team's own agreement (段 4). Either may be missing, and
// a definition that wrote neither has none.
func documentsOf(def *definition.Definition, run Run) []source.Doc {
	var docs []source.Doc
	if run.Sources != nil {
		if shared, ok := run.Sources.SharedAgreement(); ok {
			docs = append(docs, shared)
		}
	}
	if def.HasAgreement {
		docs = append(docs, def.Agreement)
	}
	return docs
}

// memberRole is one member's role document, from the catalog: the one way a name
// is looked up (K1). It is an error when there is none, and missingRole says
// which kind of missing it is.
func memberRole(node *definition.MemberNode, run Run) (source.Doc, error) {
	if run.Sources != nil {
		if doc, ok := run.Sources.Role(node.Role); ok {
			return doc, nil
		}
	}
	return source.Doc{}, missingRole(node, run)
}

// missingRole says which it is: a role document still left as Markdown, or none
// at all. The place is always roles/<role>.yaml, in the layer the catalog looks
// in (段 4).
func missingRole(node *definition.MemberNode, run Run) error {
	if run.Sources != nil {
		if doc, ok := run.Sources.RoleMarkdown(node.Role); ok {
			return i18n.Errorf("error.role_markdown", node.Role, doc.Path)
		}
	}
	return i18n.Errorf("error.no_role_doc", node.Name, node.Role, node.Role)
}
