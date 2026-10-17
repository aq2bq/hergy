package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

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

// panel is one open field: what it is called, the values it shows, and the field
// it belongs to. In this stage every field is read only (設計 9 節 1b): a panel
// shows what the model says, and writing comes with stages 2 to 5.
type panel struct {
	Title  string
	Key    string
	Parent string
	Fields []panelField
	Rule   *ruleCard
}

// panelField is one labelled value in a panel.
type panelField struct {
	Label string
	Value string
	Href  string
	Mono  bool
}

// memberPanel is the member's field (設計 3.4 の表).
func (s *Server) memberPanel(r *http.Request) (view, error) {
	member := r.PathValue("member")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		return s.memberPanelOf(def, member)
	})
}

// memberPanelOf builds one member's field.
func (s *Server) memberPanelOf(def *definition.Definition, name string) (panel, bool) {
	member, ok := def.Graph.Member(name)
	if !ok {
		return panel{}, false
	}
	field := panel{
		Title:  tf("web.panel.member", name),
		Key:    "member:" + name,
		Parent: definitionPath(def.Name),
	}
	field.Fields = append(field.Fields, panelField{Label: t("label.role"), Value: member.Role, Href: rolePath(member.Role)})
	field.Fields = append(field.Fields, panelField{Label: t("label.profile"), Value: s.profileLabel(member.Profile), Mono: true})
	for _, edge := range def.Graph.Out(member.ID()) {
		field.Fields = append(field.Fields, panelField{
			Label: t("label.turns"),
			Value: edge.To.String(),
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
	return field, true
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
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		for _, worker := range def.Graph.Workers {
			if worker.From != from || worker.Purpose != purpose {
				continue
			}
			return panel{
				Title:  tf("web.panel.worker", purpose, from),
				Key:    "worker:" + from + "/" + purpose,
				Parent: definitionPath(def.Name),
				Fields: []panelField{
					{Label: t("web.panel.worker_calls"), Value: from, Href: memberPath(def.Name, from)},
					{Label: t("label.purpose"), Value: purpose},
					{Label: t("label.profile"), Value: worker.Profile, Mono: true},
					{Label: t("web.panel.worker_returns"), Value: from},
				},
			}, true
		}
		return panel{}, false
	})
}

// humanPanel is the human's field (設計 3.4 の表).
func (s *Server) humanPanel(r *http.Request) (view, error) {
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		human := (definition.Human{}).ID()
		field := panel{Title: t("label.human"), Key: "human", Parent: definitionPath(def.Name)}
		for _, edge := range def.Graph.Out(human) {
			field.Fields = append(field.Fields, panelField{
				Label: t("web.panel.human_hands"),
				Value: edge.To.String(),
				Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
			})
		}
		for _, edge := range def.Graph.Edges {
			if edge.To != human {
				continue
			}
			field.Fields = append(field.Fields, panelField{
				Label: t("web.panel.human_receives"),
				Value: edge.From.String(),
				Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
			})
		}
		return field, true
	})
}

// turnPanel is one edge's field (設計 3.4 の表). Editing an edge is stage 3; here
// the field says which turn it is.
func (s *Server) turnPanel(r *http.Request) (view, error) {
	from, to := r.PathValue("from"), r.PathValue("to")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		for _, edge := range def.Graph.Turns {
			if edge.From.String() != from || edge.To.String() != to {
				continue
			}
			return panel{
				Title:  edge.String(),
				Key:    turnKey(from, to),
				Parent: definitionPath(def.Name),
				Fields: []panelField{
					{Label: t("web.panel.turn_from"), Value: from},
					{Label: t("web.panel.turn_to"), Value: to},
				},
			}, true
		}
		return panel{}, false
	})
}

// agreementPanel is the team agreement's own field: its path and its sections.
func (s *Server) agreementPanel(r *http.Request) (view, error) {
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		field := panel{
			Title:  t("web.panel.team_agreement"),
			Key:    "agreement",
			Parent: definitionPath(def.Name),
			Fields: []panelField{{Label: t("label.path"), Value: s.cat.Display(def.Agreement.Path), Mono: true}},
		}
		return field, true
	})
}

// rulePanel is one rule of the team agreement (設計 3.8): the whole rule, read
// only in this stage.
func (s *Server) rulePanel(r *http.Request) (view, error) {
	id := r.PathValue("rule")
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) (panel, bool) {
		if !def.HasAgreement {
			return panel{}, false
		}
		doc, err := s.ruleDoc(def.Agreement)
		if err != nil {
			return panel{}, false
		}
		card, ok := cardOf(doc, id)
		if !ok {
			return panel{}, false
		}
		return panel{
			Title:  tf("web.panel.rule", id),
			Key:    "rule:" + id,
			Parent: agreementPath(def.Name),
			Rule:   &card,
		}, true
	})
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
