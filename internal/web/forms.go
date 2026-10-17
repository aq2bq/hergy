package web

import (
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
)

// This file holds the writable fields a panel draws (設計 3.4 の表): one form per
// place in team.yaml, drawn from the start (C1) and written through the save
// road (7.3). What the model says about a place stays in the core: a form only
// lays out the choices the catalog resolves and the sentences the check wrote.

// choice is one option of a select: the name a person picks, and the attribute
// that goes with the name. A profile is picked by its name alone, and its kind
// stands beside the pick as an attribute, never inside a choice (I7、po-r6 の 4).
type choice struct {
	Value  string
	Text   string // what a person reads, when it is not the value (the human)
	Kind   string
	Chosen bool
}

// control is one control of an edit form: a select of names, or a line of free
// text. Label is what a text control has to be named by, so a person knows what
// to write in it; Preview is where a pick shows what it would do before it is
// written (T1).
type control struct {
	Name    string
	Type    string // "select" or "text"
	Label   string // a text control's own name (the purpose)
	Draft   string // the sessionStorage key of this control's draft (C5)
	Origin  string // the value the file writes, for 取り消す and the change mark
	Value   string
	Choices []choice
	KindOf  bool   // show the picked profile's kind beside it (I7)
	Kind    string // that kind
	Preview string // where a pick is previewed (T1), "" when it is not
}

// formValue is one hidden value of an edit form.
type formValue struct{ Name, Value string }

// editForm is one writable field of a panel: the form that writes one place in
// the definition's team.yaml. A save clears the draft of the form it wrote and
// leaves the other drafts alone (C5), which is what Key is for.
//
// Pending and PendingStop are what a pick that has not been written yet would
// do: the lines effect.Preview writes, or the stop a save would be refused with
// (T1, 設計 7.3 の 5).
type editForm struct {
	Key         string
	Action      string
	Token       string
	Hash        string
	Op          string
	Hidden      []formValue
	Controls    []control
	Submit      string
	Findings    []string
	Pending     []string
	PendingStop string
	Saved       bool
}

// Changed reports whether a control reads a value other than the one the file
// writes: a preview URL picks a value before anything is typed, and the page has
// to show the save and revert buttons for it even with no script (設計 7.3).
func (f *editForm) Changed() bool {
	for _, control := range f.Controls {
		if control.Value != control.Origin {
			return true
		}
	}
	return false
}

// confirmForm is a write a press has chosen but nothing has written yet: the// "保存すると" line that says what the write would do, and the two ways out (I6).
// Stop is the stop the write would bring in, which takes the write away.
type confirmForm struct {
	Action  string
	Token   string
	Hash    string
	Op      string
	Hidden  []formValue
	Preview []string
	Stop    string
	Submit  string
	Cancel  string
}

// draftKey is the sessionStorage key one field's draft is kept under (C5): the
// definition, the field, and the thing the field is about, so two fields of two
// screens never share one.
func draftKey(parts ...string) string { return "field:" + strings.Join(parts, ":") }

// form is one writable field of this panel: the action, the run's token and the
// hash of the file the page read are the same for every form of it.
func (p panel) form(key, op string) *editForm {
	return &editForm{Key: key, Action: p.URL, Token: p.Token, Hash: p.Hash, Op: op, Submit: t("web.save")}
}

// confirm is one pending write of this panel (I6).
func (p panel) confirm(op string) *confirmForm {
	return &confirmForm{Action: p.URL, Token: p.Token, Hash: p.Hash, Op: op, Submit: t("web.save"), Cancel: p.URL}
}

// workerKey is one worker's key inside a panel: the member that hires it and the
// purpose. It is the form's own key (data-key), never the identity a settings
// file writes, so a page never shows "worker:" (S1).
func workerKey(from, purpose string) string { return "worker/" + from + "/" + purpose }

// removeHref is where a press chooses one worker's removal, before anything is
// written (I6).
func removeHref(panelURL, purpose string) string {
	return panelURL + "?" + url.Values{"remove": {purpose}}.Encode()
}

// turnRemoveHref is where a press chooses one turn's removal, before anything is
// written (4 節). The far end travels in the URL, so the row itself shows the
// line about what the removal would do.
func turnRemoveHref(panelURL, to string) string {
	return panelURL + "?" + url.Values{"turn": {to}}.Encode()
}

// previewHref is where a pick shows what it would do before it is written (T1):
// the field's own URL with the op being previewed, and, for a worker, which
// worker. The picked value travels as the control's own name, so nothing about
// the pick is decided here.
func previewHref(panelURL, op string, extra ...formValue) string {
	values := url.Values{"preview": {op}}
	for _, one := range extra {
		values.Set(one.Name, one.Value)
	}
	return panelURL + "?" + values.Encode()
}

// roleForm is a member's role: picked by name from the roles hergy resolves (the
// roles list, C1), and the name the member writes now.
func (s *Server) roleForm(p panel, def *definition.Definition, member definition.MemberNode, findings []string) *editForm {
	form := p.form("role", opMemberRole)
	form.Controls = []control{{
		Name:    "role",
		Type:    "select",
		Draft:   draftKey(def.Name, "members/"+member.Name, "role"),
		Origin:  member.Role,
		Value:   member.Role,
		Choices: s.roleChoices(member.Role),
		Preview: previewHref(p.URL, opMemberRole),
	}}
	form.Findings = findings
	return form
}

// profileForm is a member's profile: picked by name alone, with the kind it
// picks standing beside the pick (I7).
func (s *Server) profileForm(p panel, def *definition.Definition, member definition.MemberNode, findings []string) *editForm {
	form := p.form("profile", opMemberProfile)
	form.Controls = []control{{
		Name:    "profile",
		Type:    "select",
		Draft:   draftKey(def.Name, "members/"+member.Name, "profile"),
		Origin:  member.Profile,
		Value:   member.Profile,
		Choices: s.profileChoices(member.Profile),
		KindOf:  true,
		Kind:    s.kindOf(member.Profile),
		Preview: previewHref(p.URL, opMemberProfile),
	}}
	form.Findings = findings
	return form
}

// workerProfileForm is one worker's profile (設計 3.4 の表).
func (s *Server) workerProfileForm(p panel, def *definition.Definition, from, purpose, profile string) *editForm {
	form := p.form(workerKey(from, purpose), opWorkerProfile)
	form.Hidden = []formValue{{Name: "worker", Value: purpose}}
	form.Controls = []control{{
		Name:    "profile",
		Type:    "select",
		Draft:   draftKey(def.Name, "workers/"+from+"/"+purpose, "profile"),
		Origin:  profile,
		Value:   profile,
		Choices: s.profileChoices(profile),
		KindOf:  true,
		Kind:    s.kindOf(profile),
		Preview: previewHref(p.URL, opWorkerProfile, formValue{Name: "worker", Value: purpose}),
	}}
	return form
}

// addWorkerForm is the form that hires one more worker for a member: a purpose
// and a profile (設計 3.4 の表). It is there only when a profile is, so the
// purpose is the only thing a person has to write; the purpose carries its own
// label, so the field says what to write there (S1). A hire cannot be previewed:
// it is not written until the purpose is.
func (s *Server) addWorkerForm(p panel, def *definition.Definition, member definition.MemberNode) *editForm {
	choices := s.profileChoices(member.Profile)
	if len(choices) == 0 {
		return nil
	}
	form := p.form("worker-add", opWorkerAdd)
	form.Submit = t("web.add")
	form.Controls = []control{
		{
			Name:   "purpose",
			Type:   "text",
			Label:  t("label.purpose"),
			Draft:  draftKey(def.Name, "members/"+member.Name, "worker-add", "purpose"),
			Origin: "",
			Value:  "",
		},
		{
			Name:    "profile",
			Type:    "select",
			Draft:   draftKey(def.Name, "members/"+member.Name, "worker-add", "profile"),
			Origin:  member.Profile,
			Value:   member.Profile,
			Choices: choices,
			KindOf:  true,
			Kind:    s.kindOf(member.Profile),
		},
	}
	return form
}

// addTurnForm is the form that hands the turn to one more party (設計 3.4 の表、
// 4 節): the members and the human this node does not hand the turn to yet, and
// not itself. A turn to a worker is made by workers:, so a worker is never a
// choice. The form is there only when there is a party to pick.
func (s *Server) addTurnForm(p panel, def *definition.Definition, from definition.NodeID, pending pendingEdit) *editForm {
	choices := turnChoices(def, from)
	if len(choices) < 2 {
		return nil
	}
	form := p.form("turn-add", opTurnAdd)
	form.Submit = t("web.add")
	form.Controls = []control{{
		Name:    "to",
		Type:    "select",
		Draft:   draftKey(def.Name, "turns/"+nodeLabel(from), "add"),
		Origin:  "",
		Value:   pending.Value,
		Choices: choices,
		Preview: previewHref(p.URL, opTurnAdd),
	}}
	return form
}

// turnChoices is who one node may hand the turn to next: the members in the
// order the definition writes them, and the human last, without the node itself
// and without the parties its turns: already name (4 節). The choice is the
// node's identity, which is what a turn is written with.
func turnChoices(def *definition.Definition, from definition.NodeID) []choice {
	taken := map[definition.NodeID]bool{}
	for _, edge := range def.Graph.Out(from) {
		if isTurn(edge) {
			taken[edge.To] = true
		}
	}
	// The first choice is empty, so a field that is just drawn writes nothing
	// until a party is picked (the value the file writes for it is "not
	// chosen").
	out := []choice{{Text: t("web.panel.turn_pick"), Chosen: true}}
	for i := range def.Graph.Members {
		id := def.Graph.Members[i].ID()
		if id == from || taken[id] {
			continue
		}
		out = append(out, choice{Value: id.String()})
	}
	human := (definition.Human{}).ID()
	if from != human && !taken[human] {
		out = append(out, choice{Value: human.String(), Text: nodeLabel(human)})
	}
	return out
}

// roleChoices is the roles a person may pick for a member: the names hergy
// resolves, and the name the member writes now, so a definition that names a
// role no layer holds still writes the same name back.
func (s *Server) roleChoices(current string) []choice {
	names := make([]string, 0, 4)
	for _, entry := range s.cat.Roles() {
		names = append(names, entry.Name)
	}
	if current != "" && !slices.Contains(names, current) {
		names = append(names, current)
		sort.Strings(names)
	}
	out := make([]choice, 0, len(names))
	for _, name := range names {
		out = append(out, choice{Value: name, Chosen: name == current})
	}
	return out
}

// profileChoices is the profiles a person may pick, by name (C1): the profiles a
// definition sees, each with the kind it picks as its attribute (I7).
func (s *Server) profileChoices(current string) []choice {
	profiles, _, err := s.cat.Profiles()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]choice, 0, len(names))
	for _, name := range names {
		out = append(out, choice{Value: name, Kind: profiles[name].Kind, Chosen: name == current})
	}
	return out
}

// workerIndex is where one worker stands in the definition's worker list, for
// the check's workers[i] fields.
func workerIndex(def *definition.Definition, from, purpose string) int {
	for i, worker := range def.Graph.Workers {
		if worker.From == from && worker.Purpose == purpose {
			return i
		}
	}
	return -1
}
