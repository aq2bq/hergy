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
	Kind   string
	Chosen bool
}

// control is one control of an edit form: a select of names, or a line of free
// text.
type control struct {
	Name    string
	Type    string // "select" or "text"
	Draft   string // the sessionStorage key of this control's draft (C5)
	Origin  string // the value the file writes, for 取り消す and the change mark
	Value   string
	Choices []choice
	KindOf  bool   // show the picked profile's kind beside it (I7)
	Kind    string // that kind
}

// formValue is one hidden value of an edit form.
type formValue struct{ Name, Value string }

// editForm is one writable field of a panel: the form that writes one place in
// the definition's team.yaml. A save clears the draft of the form it wrote and
// leaves the other drafts alone (C5), which is what Key is for.
type editForm struct {
	Key      string
	Action   string
	Token    string
	Hash     string
	Op       string
	Hidden   []formValue
	Controls []control
	Submit   string
	Findings []string
	Saved    bool
}

// confirmForm is a write a press has chosen but nothing has written yet: the
// "保存すると" line that says what the write would do, and the two ways out (I6).
type confirmForm struct {
	Action  string
	Token   string
	Hash    string
	Op      string
	Hidden  []formValue
	Preview []string
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
	}}
	return form
}

// addWorkerForm is the form that hires one more worker for a member: a purpose
// and a profile (設計 3.4 の表). It is there only when a profile is, so the
// purpose is the only thing a person has to write.
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
