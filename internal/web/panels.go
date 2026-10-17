package web

import (
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// panel is one open field: what it is called, the values it shows, and the field
// it belongs to. A rule's card carries the form that writes it (段 4); the other
// fields show what the model says. A field the URL names but the definition does
// not hold comes back with Missing set and the one line Notice, so the page
// answers 404 rather than opening nothing (N2).
type panel struct {
	Title    string
	Key      string
	Parent   string
	Crumb    string
	Fields   []panelField
	Form     *ruleForm
	Missing  bool
	Notice   string
	Reload   bool
	Findings []string
}

// panelField is one labelled value in a panel.
type panelField struct {
	Label    string
	Value    string
	Href     string
	Mono     bool
	Findings []string
}

// profileLabel is one profile as a panel shows it: its name, with the kind it
// starts beside it (I7: the choice is the name, the kind is an attribute).
func (s *Server) profileLabel(name string) string {
	if name == "" {
		return ""
	}
	kind := s.kindOf(name)
	if kind == "" {
		return name
	}
	return tf("web.panel.profile_kind", name, kind)
}

// memberPanel is the member's field (設計 3.4 の表).
func (s *Server) memberPanel(r *http.Request) (view, error) {
	member := r.PathValue("member")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		return s.memberPanelOf(def, member, s.findings(def.Name))
	})
}

// memberPanelOf builds one member's field, with the sentences the check writes
// about the member and its role and profile fields (R1).
func (s *Server) memberPanelOf(def *definition.Definition, name string, findings []check.Finding) panel {
	member, ok := def.Graph.Member(name)
	if !ok {
		return missingPanel(def, "member:"+name, tf("web.panel.member", name), tf("web.missing.member", def.Name, name))
	}
	field := panel{
		Title:  tf("web.panel.member", name),
		Key:    "member:" + name,
		Parent: definitionPath(def.Name),
		Crumb:  name,
	}
	all, roleFindings, profileFindings := memberFindings(findings, memberIndex(def, name))
	if problem := nodeProblems(findings)[name]; problem != "" {
		all = append(all, problem)
	}
	field.Findings = all
	field.Fields = append(field.Fields, panelField{Label: t("label.role"), Value: member.Role, Href: rolePath(member.Role), Findings: roleFindings})
	field.Fields = append(field.Fields, panelField{Label: t("label.profile"), Value: s.profileLabel(member.Profile), Mono: true, Findings: profileFindings})
	for _, edge := range def.Graph.Out(member.ID()) {
		if !isTurn(edge) {
			continue
		}
		field.Fields = append(field.Fields, panelField{
			Label: t("label.turns"),
			Value: nodeLabel(edge.To),
			Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
		})
	}
	for _, worker := range def.Graph.Workers {
		if worker.From != name {
			continue
		}
		field.Fields = append(field.Fields, panelField{
			Label: t("label.workers"),
			Value: tf("web.panel.worker_line", worker.Purpose, worker.Profile),
			Href:  workerPath(def.Name, worker.From, worker.Purpose),
		})
	}
	field.Fields = append(field.Fields, s.memberDocs(def, member)...)
	return field
}

// memberDocs is the three documents a member reads, in the order the prompt
// puts them in (role, shared agreement, team agreement).
func (s *Server) memberDocs(def *definition.Definition, member definition.MemberNode) []panelField {
	var out []panelField
	if doc, ok := s.cat.Role(member.Role); ok {
		out = append(out, panelField{
			Label: t("web.panel.reads_role"),
			Value: tf("web.panel.rules_count", member.Role, s.ruleCount(doc.Path)),
			Href:  rolePath(member.Role),
		})
	} else {
		out = append(out, panelField{Label: t("web.panel.reads_role"), Value: member.Role})
	}
	out = append(out, s.docField(t("web.panel.reads_shared"), s.cat.SharedAgreement, "/agreement"))
	out = append(out, s.docField(t("web.panel.reads_team"), func() (source.Doc, bool) {
		return def.Agreement, def.HasAgreement
	}, agreementPath(def.Name)))
	return out
}

// docField is one document and its rule count, as a field.
func (s *Server) docField(label string, find func() (source.Doc, bool), href string) panelField {
	doc, ok := find()
	if !ok {
		return panelField{Label: label, Value: t("web.panel.no_document")}
	}
	return panelField{Label: label, Value: tf("web.panel.rules_count", "", s.ruleCount(doc.Path)), Href: href}
}

// workerPanel is the worker's field (設計 3.4 の表).
func (s *Server) workerPanel(r *http.Request) (view, error) {
	from, purpose := r.PathValue("from"), r.PathValue("purpose")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		for _, worker := range def.Graph.Workers {
			if worker.From != from || worker.Purpose != purpose {
				continue
			}
			title := tf("web.panel.worker", purpose, from)
			return panel{
				Title:  title,
				Key:    "worker:" + from + "/" + purpose,
				Parent: definitionPath(def.Name),
				Crumb:  title,
				Fields: []panelField{
					{Label: t("web.panel.worker_calls"), Value: from, Href: memberPath(def.Name, from)},
					{Label: t("label.purpose"), Value: purpose},
					{Label: t("label.profile"), Value: worker.Profile, Mono: true},
					{Label: t("web.panel.worker_returns"), Value: from},
				},
			}
		}
		key := "worker:" + from + "/" + purpose
		return missingPanel(def, key, tf("web.panel.worker", purpose, from), tf("web.missing.worker", def.Name, purpose, from))
	})
}

// humanPanel is the human's field (設計 3.4 の表). The human is always a node of
// the graph, so this field is never missing.
func (s *Server) humanPanel(r *http.Request) (view, error) {
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		human := (definition.Human{}).ID()
		field := panel{Title: t("label.human"), Key: "human", Parent: definitionPath(def.Name), Crumb: t("label.human")}
		for _, edge := range def.Graph.Out(human) {
			field.Fields = append(field.Fields, panelField{
				Label: t("web.panel.human_hands"),
				Value: nodeLabel(edge.To),
				Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
			})
		}
		for _, edge := range def.Graph.Edges {
			if edge.To != human {
				continue
			}
			field.Fields = append(field.Fields, panelField{
				Label: t("web.panel.human_receives"),
				Value: nodeLabel(edge.From),
				Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
			})
		}
		return field
	})
}

// turnPanel is one edge's field (設計 3.4 の表). Editing an edge is stage 3; here
// the field says which turn it is.
func (s *Server) turnPanel(r *http.Request) (view, error) {
	from, to := r.PathValue("from"), r.PathValue("to")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		for _, edge := range def.Graph.Turns {
			if edge.From.String() != from || edge.To.String() != to {
				continue
			}
			title := turnLabel(edge)
			return panel{
				Title:  title,
				Key:    turnKey(from, to),
				Parent: definitionPath(def.Name),
				Crumb:  title,
				Fields: []panelField{
					{Label: t("web.panel.turn_from"), Value: nodeLabel(edge.From)},
					{Label: t("web.panel.turn_to"), Value: nodeLabel(edge.To)},
				},
			}
		}
		label := turnEndLabel(from) + " → " + turnEndLabel(to)
		return missingPanel(def, turnKey(from, to), label, tf("web.missing.turn", def.Name, label))
	})
}

// turnEndLabel is one end of a turn as it stands in a URL, read the way a person
// says it.
func turnEndLabel(raw string) string {
	if node, ok := parseNode(raw); ok {
		return nodeLabel(node)
	}
	return raw
}

// parseNode reads a node identity as a settings file writes it ("human",
// "worker:<from>/<purpose>", a member name).
func parseNode(raw string) (definition.NodeID, bool) {
	switch {
	case raw == (definition.Human{}).ID().String():
		return (definition.Human{}).ID(), true
	case strings.HasPrefix(raw, string(definition.KindWorker)+":"):
		return definition.NodeID{Kind: definition.KindWorker, Name: strings.TrimPrefix(raw, string(definition.KindWorker)+":")}, true
	case raw == "":
		return definition.NodeID{}, false
	default:
		return definition.NodeID{Kind: definition.KindMember, Name: raw}, true
	}
}

// agreementPanel is the team agreement's own field: its path and who reads it
// (設計 1 節).
func (s *Server) agreementPanel(r *http.Request) (view, error) {
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		if !def.HasAgreement {
			return missingPanel(def, "agreement", t("web.panel.team_agreement"), tf("web.missing.agreement", def.Name))
		}
		return panel{
			Title:  t("web.panel.team_agreement"),
			Key:    "agreement",
			Parent: definitionPath(def.Name),
			Crumb:  t("web.panel.team_agreement"),
			Fields: []panelField{
				{Label: t("label.path"), Value: s.cat.Display(def.Agreement.Path), Mono: true},
				{Label: t("web.common.readers"), Value: readerLine(s.cat.Readers(def.Agreement.Path))},
			},
		}
	})
}

// teamRule is one rule of a team's own agreement, open (設計 3.8).
func (s *Server) teamRule(r *http.Request) (view, error) {
	return s.openRule(r, s.teamPlace(r.PathValue("name")))
}

// teamNew is the card a team's own agreement is adding a rule with.
func (s *Server) teamNew(r *http.Request) (view, error) {
	return s.addRule(r, s.teamPlace(r.PathValue("name")))
}

// saveTeamRule writes one rule of a team's own agreement.
func (s *Server) saveTeamRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.teamPlace(r.PathValue("name")), false)
}

// addTeamRule adds a rule to the end of one section of a team's own agreement.
func (s *Server) addTeamRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.teamPlace(r.PathValue("name")), true)
}

// writeTeamAgreement is the write of the team agreement itself: the second press
// of 消す takes a rule out. A team agreement is made by hergy new, so this page
// makes none.
func (s *Server) writeTeamAgreement(r *http.Request) (view, error) {
	return s.writeDocument(r, s.teamPlace(r.PathValue("name")))
}

// readerLine is who reads a document as one line: "hergy-dev の 2 人", the same
// line the shared agreement shows.
func readerLine(refs []catalog.MemberRef) string {
	lines := readerLines(refs)
	if len(lines) == 0 {
		return t("web.panel.no_document")
	}
	return strings.Join(lines, t("list.separator"))
}

// missingPanel is the field a URL named and the definition does not hold: the
// place's own name as the title, and the one line that says it is not there
// (N2). The sentence is shown once, in the body (R2).
func missingPanel(def *definition.Definition, key, title, notice string) panel {
	return missingPanelAt(definitionPath(def.Name), key, title, notice)
}

// missingPanelAt is the same, for a field whose parent is not a definition.
func missingPanelAt(parent, key, title, notice string) panel {
	return panel{
		Key:     key,
		Parent:  parent,
		Missing: true,
		Notice:  notice,
		Title:   title,
	}
}

// memberIndex is where one member stands in the definition's member list, for the
// check's members[i] fields.
func memberIndex(def *definition.Definition, name string) int {
	for i := range def.Graph.Members {
		if def.Graph.Members[i].Name == name {
			return i
		}
	}
	return -1
}
