package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// panel is one open field: what it is called, the values it shows, and the field
// it belongs to. A field of a team definition is written from the start (C1) —
// a member's role and profile, a worker, a turn and the human (段 2・3), and the
// cards of a rule document (段 4) — and the other screens' fields are read only.
// A field the URL names but the definition does not hold comes back with Missing
// set and the one line Notice, so the page answers 404 rather than opening
// nothing (N2).
type panel struct {
	Title    string
	Key      string
	Parent   string
	Crumb    string
	URL      string
	Token    string
	Hash     string
	Fields   []panelField
	Form     *ruleForm
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
	// Pending is the turn a press chose to add or take away, for the figure
	// (4 節). Nil when no turn is being looked at.
	Pending *turnPending
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
// link that chose it (Remove) (設計 3.4 の表). Next says the value stands under
// the same name as the one before it: the parties one node hands the turn to,
// and the workers one member hires, are one name with a value each (walk-2 P4).
type panelField struct {
	Label    string
	Value    string
	Href     string
	Mono     bool
	Findings []string
	Edit     *editForm
	Confirm  *confirmForm
	Remove   string
	Next     bool
}

// fieldGroup is one name of a panel and the values under it.
type fieldGroup struct {
	Label  string
	Row    bool
	Values []panelField
}

// Groups is the panel's fields as a person reads them: one name, and every value
// that stands under it. A field marked Next joins the group before it.
func (p panel) Groups() []fieldGroup {
	var out []fieldGroup
	for _, field := range p.Fields {
		if field.Next && len(out) > 0 {
			group := &out[len(out)-1]
			group.Values = append(group.Values, field)
			group.Row = group.Row || field.Edit != nil
			continue
		}
		out = append(out, fieldGroup{Label: field.Label, Row: field.Edit != nil, Values: []panelField{field}})
	}
	return out
}

// memberPanel is the member's field (設計 3.4 の表), with what a press has chosen
// and nothing has written yet: a worker's removal (I6), or a picked value whose
// preview is being shown (T1).
func (s *Server) memberPanel(r *http.Request) (view, error) {
	name, member := r.PathValue("name"), r.PathValue("member")
	pending := pendingOf(r)
	return s.definitionPage(name, func(def *definition.Definition) panel {
		return s.memberPanelOf(def, member, s.findings(def.Name), pending)
	})
}

// memberPanelOf builds one member's field, with the sentences the check writes
// about the member and its role and profile fields (R1).
func (s *Server) memberPanelOf(def *definition.Definition, name string, findings []check.Finding, pending pendingEdit) panel {
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
	rows, figurePending := s.turnFields(def, field, member.ID(), pending)
	field.Fields = append(field.Fields, rows...)
	field.Pending = figurePending
	if add := s.addTurnForm(field, def, member.ID(), pending); add != nil {
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.turn_add"), Edit: add})
	}
	field.Fields = append(field.Fields, s.workerFields(def, field, member, findings, pending)...)
	if add := s.addWorkerForm(field, def, member); add != nil {
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.worker_add"), Edit: add})
	}
	field.Fields = append(field.Fields, s.memberDocs(def, member)...)
	return s.applyPreview(def, member.ID(), field, pending)
}

// turnFields is the parties one node hands the turn to (設計 3.4 の表、4 節):
// each row opens the turn's own field and offers the press that takes it away.
// A press that chose the removal turns the row into the line about what the
// removal would do, and the write (I12a, I12b).
func (s *Server) turnFields(def *definition.Definition, p panel, from definition.NodeID, pending pendingEdit) (rows []panelField, chosen *turnPending) {
	for _, edge := range def.Graph.Out(from) {
		if !isTurn(edge) {
			continue
		}
		row := panelField{
			Label: t("label.turns"),
			Value: nodeLabel(edge.To),
			Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
			Next:  len(rows) > 0,
		}
		if pending.Turn == edge.To.String() {
			confirm := p.confirm(opTurnRemove)
			confirm.Hidden = []formValue{{Name: "to", Value: edge.To.String()}}
			confirm.Preview, confirm.Stop = s.previewOf(def, definition.RemoveTurn(from, edge.To))
			row.Confirm = confirm
			// The figure draws the edge being taken away faint (4 節).
			chosen = &turnPending{From: from, To: edge.To}
		} else {
			row.Remove = turnRemoveHref(p.URL, edge.To.String())
		}
		rows = append(rows, row)
	}
	return rows, chosen
}

// workerFields is the workers one member may hire, each written from the field
// (設計 3.4 の表): the purpose it is hired for, as a link to its own field, its
// profile, and the press that takes it away, which shows what the hire would
// lose before anything is written (S1, I6).
func (s *Server) workerFields(def *definition.Definition, p panel, member definition.MemberNode, findings []check.Finding, pending pendingEdit) []panelField {
	var out []panelField
	for _, worker := range def.Graph.Workers {
		if worker.From != member.Name {
			continue
		}
		if worker.Purpose == pending.Remove {
			confirm := p.confirm(opWorkerRemove)
			confirm.Hidden = []formValue{{Name: "worker", Value: worker.Purpose}}
			confirm.Preview, confirm.Stop = s.previewOf(def, definition.RemoveWorker(member.Name, worker.Purpose))
			out = append(out, panelField{Label: t("label.workers"), Confirm: confirm, Next: len(out) > 0})
			continue
		}
		form := s.workerProfileForm(p, def, member.Name, worker.Purpose, worker.Profile)
		form.Findings = fieldTexts(findings, fmt.Sprintf("workers[%d]", workerIndex(def, member.Name, worker.Purpose)))
		out = append(out, panelField{
			Label:  t("label.workers"),
			Value:  worker.Purpose,
			Href:   workerPath(def.Name, worker.From, worker.Purpose),
			Edit:   form,
			Remove: removeHref(p.URL, worker.Purpose),
			Next:   len(out) > 0,
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
	pending := pendingOf(r)
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		return s.workerPanelOf(def, from, purpose, pending)
	})
}

// workerPanelOf builds one worker's field, with the removal pending when the
// press that chose it is still unwritten (I6).
func (s *Server) workerPanelOf(def *definition.Definition, from, purpose string, pending pendingEdit) panel {
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
		if pending.Remove == purpose {
			confirm := field.confirm(opWorkerRemove)
			// The worker is gone once this is written, so the write goes to the
			// member's field: the field that still holds the workers (N1).
			confirm.Action = memberPath(def.Name, from)
			confirm.Preview, confirm.Stop = s.previewOf(def, definition.RemoveWorker(from, purpose))
			field.Fields = append(field.Fields, panelField{Label: t("label.profile"), Confirm: confirm})
		} else {
			field.Fields = append(field.Fields, panelField{
				Label:  t("label.profile"),
				Edit:   s.workerProfileForm(field, def, from, purpose, worker.Profile),
				Remove: removeHref(field.URL, purpose),
			})
		}
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.worker_returns"), Value: from})
		return s.applyPreview(def, definition.NodeID{Kind: definition.KindMember, Name: from}, field, pending)
	}
	key := "worker:" + from + "/" + purpose
	return missingPanel(def, key, tf("web.panel.worker", purpose, from), tf("web.missing.worker", def.Name, purpose, from))
}

// humanPanel is the human's field (設計 3.4 の表). The human is always a node of
// the graph, so this field is never missing.
func (s *Server) humanPanel(r *http.Request) (view, error) {
	pending := pendingOf(r)
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		return s.humanPanelOf(def, s.findings(def.Name), pending)
	})
}

// humanPanelOf builds the human's field: the parties it hands the turn to (and
// may take one away from), the members that hand it the turn, and the form that
// adds one more party (設計 3.4 の表、4 節).
func (s *Server) humanPanelOf(def *definition.Definition, findings []check.Finding, pending pendingEdit) panel {
	human := (definition.Human{}).ID()
	field := panel{
		Title:  t("label.human"),
		Key:    "human",
		Parent: definitionPath(def.Name),
		Crumb:  t("label.human"),
		URL:    humanPath(def.Name),
		Token:  s.token,
		Hash:   s.hashOf(def.TeamYAML),
	}
	field.Findings = append(field.Findings, fieldTexts(findings, "node:human")...)
	rows, chosen := s.turnFields(def, field, human, pending)
	field.Fields = append(field.Fields, rows...)
	field.Pending = chosen
	if add := s.addTurnForm(field, def, human, pending); add != nil {
		field.Fields = append(field.Fields, panelField{Label: t("web.panel.turn_add"), Edit: add})
	}
	for _, edge := range def.Graph.Edges {
		if edge.To != human || edge.From.Kind == definition.KindWorker {
			continue
		}
		field.Fields = append(field.Fields, panelField{
			Label: t("web.panel.human_receives"),
			Value: nodeLabel(edge.From),
			Href:  turnHref(def.Name, edge.From.String(), edge.To.String()),
		})
	}
	return s.applyPreview(def, human, field, pending)
}

// turnPanel is one edge's field (設計 3.4 の表): which turn it is, what taking it
// away would do, and the press that chooses the removal (4 節). The write goes
// to the field of the party that hands the turn on, because the edge's own
// field is gone once the turn is (N1).
func (s *Server) turnPanel(r *http.Request) (view, error) {
	from, to := r.PathValue("from"), r.PathValue("to")
	pending := pendingOf(r)
	return s.definitionPage(r.PathValue("name"), func(def *definition.Definition) panel {
		for _, edge := range def.Graph.Turns {
			if edge.From.String() != from || edge.To.String() != to {
				continue
			}
			title := turnLabel(edge)
			field := panel{
				Title:  title,
				Key:    turnKey(from, to),
				Parent: definitionPath(def.Name),
				Crumb:  title,
				URL:    turnPath(def.Name, from, to),
				Token:  s.token,
				Hash:   s.hashOf(def.TeamYAML),
			}
			row := panelField{Label: t("web.panel.turn_edge"), Value: title}
			if pending.Turn == to {
				confirm := field.confirm(opTurnRemove)
				confirm.Action = s.nodeHref(def, edge.From)
				confirm.Hidden = []formValue{{Name: "to", Value: to}}
				confirm.Preview, confirm.Stop = s.previewOf(def, definition.RemoveTurn(edge.From, edge.To))
				row.Confirm = confirm
				field.Pending = &turnPending{From: edge.From, To: edge.To}
			} else {
				row.Remove = turnRemoveHref(field.URL, to)
			}
			field.Fields = []panelField{
				{Label: t("web.panel.turn_from"), Value: nodeLabel(edge.From)},
				{Label: t("web.panel.turn_to"), Value: nodeLabel(edge.To)},
				row,
			}
			return field
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
