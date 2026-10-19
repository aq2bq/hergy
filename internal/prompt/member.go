package prompt

import (
	"errors"
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
	// Lang is the language the prompt is written in: the launch record's when a
	// prompt is rebuilt to compare it with what was handed out (C9), and empty
	// for the language in use.
	Lang string
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

// language is the language this run's prompt is written in: the run's own when
// it carries one, and the one in use otherwise.
func (r Run) language() string {
	if r.Lang != "" {
		return r.Lang
	}
	return i18n.Language()
}

// text is one of hergy's own words in the language this run's prompt is written
// in.
func (r Run) text(key string) string { return i18n.Text(r.Lang, key) }

// textf is text with fmt.Sprintf.
func (r Run) textf(key string, args ...any) string { return i18n.Textf(r.Lang, key, args...) }

// errorf is one of hergy's sentences, in the language this run's prompt is
// written in, as an error.
func (r Run) errorf(key string, args ...any) error { return errors.New(r.textf(key, args...)) }

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

// Where is what a prompt error is about: the team.yaml field a screen puts the
// sentence beside (C2, N2). The sentence a person reads is the error's own, and
// the key's form is check.Finding.Field's.
type Where int

const (
	// WhereFile is a rule file that is not the member's own: the agreement every
	// team of the layer shares, or the team's own. The sentence names the file.
	WhereFile Where = iota
	// WhereRole is the member's role document: not there, still Markdown, or
	// unreadable.
	WhereRole
	// WhereTemplate is the template that drops a door (I4).
	WhereTemplate
	// WhereRules is the member's documents taken together: two of them hold one
	// rule id.
	WhereRules
)

// Error is a prompt error that knows what it is about, so check can put it
// beside the right field. Error and Unwrap pass the sentence and the cause
// through unchanged, so every reader of the error reads what it always said.
type Error struct {
	Where Where
	Err   error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// withWhere wraps err with what it is about.
func withWhere(what Where, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Where: what, Err: err}
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
	template := MemberTemplate(run.Lang)
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

	readDoc := func(doc source.Doc, what Where) string {
		document, err := readRuleFile(run.Reader, doc)
		if err != nil {
			if buildErr == nil {
				buildErr = withWhere(what, err)
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
		"actor": func() string {
			// The name hergy writes into the pane's env as GY_ACTOR; the member's
			// herdr name, which follows --as.
			return run.textf("prompt.actor", memberName)
		},
		"role_doc": func() string {
			if buildErr != nil {
				return ""
			}
			doc, err := memberRole(node, run)
			if err != nil {
				buildErr = withWhere(WhereRole, err)
				return ""
			}
			found := doc
			roleDoc = &found
			return readDoc(doc, WhereRole)
		},
		"docs": func() string {
			if buildErr != nil {
				return ""
			}
			docs := documentsOf(def, run)
			texts := make([]string, 0, len(docs))
			for _, doc := range docs {
				texts = append(texts, readDoc(doc, WhereFile))
			}
			read = append(read, docs...)
			if len(texts) == 0 {
				return ""
			}
			return "\n" + strings.Join(texts, "\n\n") + "\n"
		},
		"roster": func() string { return Roster(run.Lang, RosterOf(def, run.Team), memberName) },
		"herdr": func() string {
			return HerdrMember(run.Lang, run.textf("prompt.self_intro_member", run.Team, node.Role, memberName))
		},
		"turns": func() string {
			return TurnsNote(run.Lang, TurnsOf(def, node.Name, run.Team))
		},
		"workers": func() string {
			workers := WorkersOf(def, node.Name)
			if len(workers) == 0 {
				return ""
			}
			// The worker reports back to this member: the note names the herdr
			// name, the same name the worker's own prompt names, so it follows
			// --as (the hire command takes the short name).
			return WorkersNote(run.Lang, memberName, workers, hireCommand(run, node.Name))
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
		return nil, withWhere(WhereRules, err)
	}
	built := &Built{Text: text, Read: read}
	if roleDoc != nil {
		built.Role = *roleDoc
		built.HasRole = true
	}
	return built, nil
}

// RosterOf is the team's members as the roster shows them: the herdr name each
// one answers to, and the role it reads. It is the roster the prompt and the
// figure are both made of.
func RosterOf(def *definition.Definition, team string) []RosterEntry {
	out := make([]RosterEntry, len(def.Graph.Members))
	for i, member := range def.Graph.Members {
		out[i] = RosterEntry{Name: team + "-" + member.Name, Role: member.Role}
	}
	return out
}

// TurnsOf is who a member hands the turn to, from the graph's edges out of it
// (I13): the human, or another member with the role it reads. The order is the
// order turns: writes them. It is the projection a member's prompt and the
// "保存すると" line are both made from.
func TurnsOf(def *definition.Definition, member, team string) []TurnPart {
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

// WorkersOf is what a member may hire, from the graph's edges out of it (I12c).
func WorkersOf(def *definition.Definition, member string) []WorkerPair {
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
	return run.textf("hire.command",
		shellQuote(executable), run.Team, run.language(), shellQuote(run.Base), shortName)
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
	where := run.text("prompt.template_default")
	switch {
	case node.HasPrompt:
		where = run.textf("prompt.template_member", node.Name)
	case def.Prompt != "":
		where = run.text("prompt.template_team")
	}
	for _, part := range []string{"role_doc", "docs"} {
		if !strings.Contains(template, "%{"+part+"}") {
			return withWhere(WhereTemplate, run.errorf("error.prompt_missing_part", run.shown(def.TeamYAML), where, part))
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
			return run.errorf("error.role_markdown", node.Role, run.shown(doc.Path))
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
	return run.errorf("error.no_role_doc", node.Name, node.Role, strings.Join(places, run.text("list.separator")))
}
