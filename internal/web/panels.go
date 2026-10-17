package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// panel is one open field: what it is called, the values it shows, and the field
// it belongs to. A field of a team definition is written from the start (C1);
// the other screens' fields are read only. A field the URL names but the
// definition does not hold comes back with Missing set and the one line Notice,
// so the page answers 404 rather than opening nothing (N2).
type panel struct {
	Title    string
	Key      string
	Parent   string
	Crumb    string
	URL      string
	Token    string
	Hash     string
	Fields   []panelField
	Rule     *ruleCard
	Missing  bool
	Notice   string
	Reload   bool
	Findings []string
	// Saved is the line a save wrote, Preview is what the change does to the
	// members' prompts, and Handover is what a running team has to take up
	// (C2). SavedKey is the form that was written, so its draft is cleared and
	// the other fields' drafts are left alone (C5).
	Saved    string
	Preview  []string
	Handover handover
	SavedKey string
}

// markSaved tells the one form a save wrote, so the script clears just its
// draft (C5).
func (p *panel) markSaved(key string) {
	for i := range p.Fields {
		form := p.Fields[i].Edit
		if form != nil && form.Key == key {
			form.Saved = true
		}
	}
}

// panelField is one labelled value in a panel. A writable field carries the form
// that writes it (Edit), or the pending write a press chose (Confirm) with the
// link that chose it (Remove) (設計 3.4 の表).
type panelField struct {
	Label    string
	Value    string
	Href     string
	Mono     bool
	Findings []string
	Edit     *editForm
	Confirm  *confirmForm
	Remove   string
}

// memberPanel is the member's field (設計 3.4 の表), with one worker's removal
// pending when the press that chose it is still unwritten (I6).
func (s *Server) memberPanel(r *http.Request) (view, error) {
	name, member := r.PathValue("name"), r.PathValue("member")
	pending := r.URL.Query().Get("remove")
	return s.definitionPage(name, func(def *definition.Definition) panel {
		return s.memberPanelOf(def, member, s.findings(def.Name), pending)
	})
}

// memberPanelOf builds one member's field, with the sentences the check writes
// about the member and its role and profile fields (R1).
func (s *Server) memberPanelOf(def *definition.Definition, name string, findings []check.Finding, pending string) panel {
	member, ok := def.Graph.Member(name)
	if !ok {
		return missingPanel(def, "member:"+name, tf("web.panel.member", name), tf("web.missing.member", def.Name, name))
	}
	field := panel{
		Title:  tf("web.panel.member", name),
		Key:    "member:" + name,
		Parent: definitionPath(def.Name),
		Crumb:  name,
		URL:    memberPath(def.Name, name),
		Token:  s.token,
		Hash:   s.hashOf(def.TeamYAML),
	}
	all, roleFindings, profileFindings := memberFindings(findings, memberIndex(def, name))
	if problem := nodeProblems(findings)[name]; problem != "" {
		all = append(all, problem)
	}
	field.Findings = all
	field.Fields = append(field.Fields,
		panelField{Label: t("label.role"), Edit: s.roleForm(field, def, member, roleFindings)},
		panelField{Label: t("label.profile"), Edit: s.profileForm(field, def, member, profileFindings)},
	)
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
	field.Fields = append(field.Fields, s.workerFields(def, field, member, findings, pending)...)
	if add := s.addWorkerForm(field, def, member); add != nil {
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.worker_add"), Edit: add})
	}
	field.Fields = append(field.Fields, s.memberDocs(def, member)...)
	return field
}

// workerFields is the workers one member may hire, each written from the field
// (設計 3.4 の表): its profile, and the press that takes it away, which shows
// what the hire would lose before anything is written (I6).
func (s *Server) workerFields(def *definition.Definition, p panel, member definition.MemberNode, findings []check.Finding, pending string) []panelField {
	var out []panelField
	for _, worker := range def.Graph.Workers {
		if worker.From != member.Name {
			continue
		}
		if worker.Purpose == pending {
			confirm := p.confirm(opWorkerRemove)
			confirm.Hidden = []formValue{{Name: "worker", Value: worker.Purpose}}
			confirm.Preview = s.previewOf(def, definition.RemoveWorker(member.Name, worker.Purpose))
			out = append(out, panelField{Label: t("label.workers"), Confirm: confirm})
			continue
		}
		form := s.workerProfileForm(p, def, member.Name, worker.Purpose, worker.Profile)
		form.Findings = fieldTexts(findings, fmt.Sprintf("workers[%d]", workerIndex(def, member.Name, worker.Purpose)))
		out = append(out, panelField{
			Label:  t("label.workers"),
			Edit:   form,
			Remove: removeHref(p.URL, worker.Purpose),
		})
	}
	return out
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

// workerPanel is the worker's field (設計 3.4 の表). The profile is written here,
// and the worker can be taken away; adding one is the member's field's to do.
func (s *Server) workerPanel(r *http.Request) (view, error) {
	from, purpose := r.PathValue("from"), r.PathValue("purpose")
	pending := r.URL.Query().Get("remove")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		return s.workerPanelOf(def, from, purpose, pending)
	})
}

// workerPanelOf builds one worker's field, with the removal pending when the
// press that chose it is still unwritten (I6).
func (s *Server) workerPanelOf(def *definition.Definition, from, purpose, pending string) panel {
	for _, worker := range def.Graph.Workers {
		if worker.From != from || worker.Purpose != purpose {
			continue
		}
		title := tf("web.panel.worker", purpose, from)
		field := panel{
			Title:  title,
			Key:    "worker:" + from + "/" + purpose,
			Parent: definitionPath(def.Name),
			Crumb:  title,
			URL:    workerPath(def.Name, from, purpose),
			Token:  s.token,
			Hash:   s.hashOf(def.TeamYAML),
		}
		field.Fields = append(field.Fields,
			panelField{Label: t("web.panel.worker_calls"), Value: from, Href: memberPath(def.Name, from)},
			panelField{Label: t("label.purpose"), Value: purpose},
		)
		if pending == purpose {
			confirm := field.confirm(opWorkerRemove)
			confirm.Preview = s.previewOf(def, definition.RemoveWorker(from, purpose))
			field.Fields = append(field.Fields, panelField{Label: t("label.profile"), Confirm: confirm})
		} else {
			field.Fields = append(field.Fields, panelField{
				Label:  t("label.profile"),
				Edit:   s.workerProfileForm(field, def, from, purpose, worker.Profile),
				Remove: removeHref(field.URL, purpose),
			})
		}
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.worker_returns"), Value: from})
		return field
	}
	key := "worker:" + from + "/" + purpose
	return missingPanel(def, key, tf("web.panel.worker", purpose, from), tf("web.missing.worker", def.Name, purpose, from))
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

// agreementPanel is the team agreement's own field: its path.
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
			Fields: []panelField{{Label: t("label.path"), Value: s.cat.Display(def.Agreement.Path), Mono: true}},
		}
	})
}

// rulePanel is one rule of the team agreement (設計 3.8): the whole rule, read
// only in this stage.
func (s *Server) rulePanel(r *http.Request) (view, error) {
	id := r.PathValue("rule")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		missing := func() panel {
			return missingPanel(def, "rule:"+id, tf("web.panel.rule", id), tf("web.missing.rule", def.Name, id))
		}
		if !def.HasAgreement {
			return missing()
		}
		doc, err := s.ruleDoc(def.Agreement)
		if err != nil {
			return missing()
		}
		card, ok := cardOf(doc, id)
		if !ok {
			return missing()
		}
		return panel{
			Title:  tf("web.panel.rule", id),
			Key:    "rule:" + id,
			Parent: agreementPath(def.Name),
			Crumb:  id,
			Rule:   &card,
		}
	})
}

// missingPanel is the field a URL named and the definition does not hold: the
// place's own name as the title, and the one line that says it is not there
// (N2). The sentence is shown once, in the body (R2).
func missingPanel(def *definition.Definition, key, title, notice string) panel {
	return panel{
		Key:     key,
		Parent:  definitionPath(def.Name),
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

// cardOf finds one rule in a document, as the card the panel shows.
func cardOf(doc *rules.Document, id string) (ruleCard, bool) {
	for _, rule := range doc.Rules() {
		if rule.ID != id {
			continue
		}
		return ruleCard{
			ID:     rule.ID,
			Clause: firstClause(rule.Text),
			Text:   rule.Text,
			Why:    rule.Why,
			Origin: originText(rule.Origin),
			Check:  checkText(rule),
			Note:   rule.Note,
		}, true
	}
	return ruleCard{}, false
}
