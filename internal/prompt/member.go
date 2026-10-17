package prompt

import (
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
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
	// Display is how a path is written in the sentences the prompt builds: a
	// member that misses a document reads a path the way a person writes it.
	// Empty means a path stands as it is.
	Display func(string) string
}

// shown writes one path the way the environment writes it.
func (r Run) shown(path string) string {
	if r.Display == nil {
		return path
	}
	return r.Display(path)
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
	// RolePlaces is every path a role document is looked for, in the one order a
	// name is looked up. It is what the sentence a missing role document gets
	// names, so a person sees the whole list.
	RolePlaces(name string) []string
	// SharedAgreement is the agreement every team of a layer shares, and
	// whether a layer wrote one.
	SharedAgreement() (source.Doc, bool)
}

// MemberName is the name herdr knows a member by: "<team>-<member>". It is the
// same name %{name} writes, so the projection and a check refuse a name that is
// longer than herdr's pattern in one place.
func MemberName(team string, node *definition.MemberNode) (string, error) {
	name := team + "-" + node.Name
	if !profile.NamePattern.MatchString(name) {
		return "", i18n.Errorf("error.herdr_name_pattern", name)
	}
	return name, nil
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
	if err := checkTemplate(run, def, node, template); err != nil {
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
		"turns": func() string {
			return TurnsNote(turnsOf(def, node.Name, run.Team))
		},
		"workers": func() string {
			workers := workersOf(def, node.Name)
			if len(workers) == 0 {
				return ""
			}
			// The worker reports back to this member: the note names the herdr
			// name, the same name the worker's own prompt names, so it follows
			// --as (the hire command takes the short name).
			return WorkersNote(memberName, workers, hireCommand(run, node.Name))
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

// turnsOf is who a member hands the turn to, from the graph's edges out of it
// (I13): the human, or another member with the role it reads. The order is the
// order turns: writes them.
func turnsOf(def *definition.Definition, member, team string) []TurnPart {
	var out []TurnPart
	for _, edge := range def.Graph.Out(definition.NodeID{Kind: definition.KindMember, Name: member}) {
		// A worker's edge is made by workers: and is not a turn the member
		// writes or is told about (I12c); the worker note covers it.
		if edge.To.Kind != definition.KindHuman && edge.To.Kind != definition.KindMember {
			continue
		}
		node, ok := def.Graph.Node(edge.To)
		if !ok {
			continue
		}
		switch edge.To.Kind {
		case definition.KindHuman:
			out = append(out, TurnPart{})
		case definition.KindMember:
			peer, ok := node.(definition.MemberNode)
			if !ok {
				continue
			}
			out = append(out, TurnPart{Name: team + "-" + peer.Name, Role: peer.Role})
		case definition.KindWorker:
			// A worker's edge back is made by workers: (I12c); the worker note
			// covers it, so it is not a turn the member is told about.
		}
	}
	return out
}

// workersOf is what a member may hire, from the graph's edges out of it (I12c).
func workersOf(def *definition.Definition, member string) []WorkerPair {
	var out []WorkerPair
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
		out = append(out, WorkerPair{Purpose: worker.Purpose, Profile: worker.Profile})
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
// own prompt still has to leave the two doors in (I4). The sentence names the
// team.yaml it read and, when the template is a member's own, the member, so a
// person knows which file to open (spec/32 D2).
func checkTemplate(run Run, def *definition.Definition, node *definition.MemberNode, template string) error {
	where := i18n.T("prompt.template_default")
	switch {
	case node.HasPrompt:
		where = i18n.Tf("prompt.template_member", node.Name)
	case def.Prompt != "":
		where = i18n.T("prompt.template_team")
	}
	for _, part := range []string{"role_doc", "docs"} {
		if !strings.Contains(template, "%{"+part+"}") {
			return i18n.Errorf("error.prompt_missing_part", run.shown(def.TeamYAML), where, part)
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
// at all. A document that is nowhere names every place it was looked for, in the
// one order a name is looked up (K1), so a person moving the file over sees the
// whole list (spec/32 D2).
func missingRole(node *definition.MemberNode, run Run) error {
	if run.Sources != nil {
		if doc, ok := run.Sources.RoleMarkdown(node.Role); ok {
			return i18n.Errorf("error.role_markdown", node.Role, run.shown(doc.Path))
		}
	}
	places := []string{run.shown("roles/" + node.Role + ".yaml")}
	if run.Sources != nil {
		if found := run.Sources.RolePlaces(node.Role); len(found) > 0 {
			places = make([]string, len(found))
			for i, path := range found {
				places[i] = run.shown(path)
			}
		}
	}
	return i18n.Errorf("error.no_role_doc", node.Name, node.Role, strings.Join(places, i18n.T("list.separator")))
}
