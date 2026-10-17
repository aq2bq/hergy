package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the save road of the writable fields (設計 7.3): one form writes
// one place in a definition's team.yaml, the change is laid over the catalog and
// checked before anything is written, and the file is written only when it is
// still the one the page read.

// The ops a form writes: one form writes one place in team.yaml (設計 3.4 の表).
const (
	opMemberRole    = "role"
	opMemberProfile = "profile"
	opWorkerAdd     = "worker-add"
	opWorkerRemove  = "worker-remove"
	opWorkerProfile = "worker-profile"
)

// teamEdit is one edit a form writes: the op, the form's own key (so a save
// clears just that field's draft, C5), the word the saved line names the place
// by, and the value the person wrote, which a refusal of a page-level field puts
// back.
type teamEdit struct {
	Op    definition.Op
	Key   string
	Label string
	Value string
	Hash  string
}

// saveTeamYAML runs one edit of a definition's team.yaml through the save road
// (設計 7.3) and draws the definition's page with the edited field open again.
// open is the field the page opens for the edit; a page-level field (the
// mission) has none, and its sentence stands at the page's own place.
//
// The edit is made on the text in memory (definition.EditYAML), laid over the
// catalog so the check reads it, and refused when it brings in a stop the
// definition did not have (a stop it already had does not keep a person from
// writing). The file is then written only when it is still the one the page read
// (source.WriteIfUnchanged), and the page shows what was written and what a
// running team has to take up.
func (s *Server) saveTeamYAML(name string, open panelFn, edit teamEdit) (view, error) {
	def, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: def.TeamYAML})
	if err != nil {
		return view{}, err
	}
	edited, err := definition.EditYAML([]byte(text), edit.Op)
	if err != nil {
		return s.editRefused(def, open, edit, err.Error(), false)
	}
	with := s.cat.With(def.TeamYAML, string(edited))
	if msg, ok := newStop(s.findings(name), check.Definition(check.Input{Catalog: with, Name: name})); ok {
		return s.editRefused(def, open, edit, msg, false)
	}
	if err := source.WriteIfUnchanged(def.TeamYAML, edit.Hash, string(edited)); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return s.editRefused(def, open, edit, tf("web.saved.clash", s.cat.Display(def.TeamYAML)), true)
		}
		return view{}, err
	}
	after, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	changes := effect.Of(s.view(def, s.cat.Reader()), s.view(after, s.cat.Reader()))
	return s.editSaved(after, open, edit, changes)
}

// editRefused draws the page with the field open again and one sentence beside
// it, the file as it was. The field keeps what the person wrote: a field's draft
// is still in the browser and the field reads it back, and a page-level field is
// drawn with the value that was written (設計 3.0 の「書きかけ」). reload adds the
// link that reads the file again, for a file that moved under the page.
func (s *Server) editRefused(def *definition.Definition, open panelFn, edit teamEdit, notice string, reload bool) (view, error) {
	data := s.definitionData(def, open)
	if open == nil {
		data.Mission = edit.Value
		data.Notice = append([]string{notice}, data.Notice...)
		data.NoticeReload = reload
		return s.renderDefinition(data)
	}
	data.Panel.Notice = notice
	data.Panel.Reload = reload
	return s.renderDefinition(data)
}

// editSaved draws the page after a save: the saved line, what the change does to
// the members' prompts (effect.Preview), and what a running team has to take up
// (effect.Commands, with a copy button on each command). For a field of a panel,
// only the form that was written is told, so the script clears its draft and
// leaves the others (C5).
func (s *Server) editSaved(after *definition.Definition, open panelFn, edit teamEdit, changes []effect.Change) (view, error) {
	running, _ := s.runningTeams()
	data := s.definitionData(after, open)
	saved := tf("web.saved.line", s.cat.Display(after.TeamYAML), edit.Label)
	handover := handoverOf(effect.Commands(changes, running))
	preview := effect.Preview(changes)
	if open == nil {
		data.Saved = saved
		data.Preview = preview
		data.Handover = handover
		return s.renderDefinition(data)
	}
	data.Panel.Saved = saved
	data.Panel.Preview = preview
	data.Panel.Handover = handover
	data.Panel.markSaved(edit.Key)
	return s.renderDefinition(data)
}

// previewOf is what an edit would say, and the stop it would bring in: the edit
// is made on the text in memory, read back through the catalog's overlay,
// checked, and compared with the definition as it stands. Nothing is written
// (設計 7.3 の 5). A stop that is already there is not this edit's, so it says
// nothing; a new one is what a save would be refused with.
func (s *Server) previewOf(def *definition.Definition, op definition.Op) (lines []string, stop string) {
	text, err := s.cat.Reader().Read(source.Doc{Path: def.TeamYAML})
	if err != nil {
		return nil, ""
	}
	edited, err := definition.EditYAML([]byte(text), op)
	if err != nil {
		return nil, ""
	}
	with := s.cat.With(def.TeamYAML, string(edited))
	if msg, ok := newStop(s.findings(def.Name), check.Definition(check.Input{Catalog: with, Name: def.Name})); ok {
		return nil, msg
	}
	after, err := with.Read(def.Name)
	if err != nil {
		return nil, ""
	}
	return effect.Preview(effect.Of(s.view(def, s.cat.Reader()), s.view(after, with.Reader()))), ""
}

// pendingEdit is what a press has chosen and nothing has written yet, read from
// the field's own URL: a worker whose removal was chosen (I6), or a value whose
// pick is being looked at before it is written (T1).
type pendingEdit struct {
	Remove  string // a worker's purpose, for the removal that was chosen
	Preview string // the op whose pick is being looked at
	Value   string // the value that was picked
	Worker  string // the worker the picked value belongs to
}

// pendingOf reads the pending state out of a field's URL.
func pendingOf(r *http.Request) pendingEdit {
	q := r.URL.Query()
	out := pendingEdit{Remove: q.Get("remove"), Preview: q.Get("preview"), Worker: q.Get("worker")}
	switch out.Preview {
	case opMemberRole:
		out.Value = q.Get("role")
	case opMemberProfile, opWorkerProfile:
		out.Value = q.Get("profile")
	}
	return out
}

// pendingOp is the edit a pending preview would write, and the field it belongs
// to. A pending state that names nothing this member writes has no op.
func (p pendingEdit) pendingOp(member string) (definition.Op, string, bool) {
	switch p.Preview {
	case opMemberRole:
		if p.Value == "" {
			return definition.Op{}, "", false
		}
		return definition.MemberRole(member, p.Value), "role", true
	case opMemberProfile:
		if p.Value == "" {
			return definition.Op{}, "", false
		}
		return definition.MemberProfile(member, p.Value), "profile", true
	case opWorkerProfile:
		if p.Value == "" || p.Worker == "" {
			return definition.Op{}, "", false
		}
		return definition.WorkerProfile(member, p.Worker, p.Value), workerKey(member, p.Worker), true
	default:
		return definition.Op{}, "", false
	}
}

// applyPreview is what a picked value would do, shown beside the field itself
// before anything is written (T1): the lines effect.Preview writes, or the stop a
// save would be refused with. The field reads the picked value, so the pick is
// kept and the way out is at hand. Nothing is written.
func (s *Server) applyPreview(def *definition.Definition, member string, field panel, pending pendingEdit) panel {
	op, key, ok := pending.pendingOp(member)
	if !ok {
		return field
	}
	lines, stop := s.previewOf(def, op)
	if len(lines) == 0 && stop == "" {
		return field
	}
	for i := range field.Fields {
		form := field.Fields[i].Edit
		if form == nil || form.Key != key {
			continue
		}
		form.Pending, form.PendingStop = lines, stop
		for j := range form.Controls {
			form.Controls[j].Value = pending.Value
			for k := range form.Controls[j].Choices {
				form.Controls[j].Choices[k].Chosen = form.Controls[j].Choices[k].Value == pending.Value
			}
			if form.Controls[j].KindOf {
				form.Controls[j].Kind = s.kindOf(pending.Value)
			}
		}
	}
	return field
}

// missionEdit writes a definition's mission, the one field that stands on the
// page itself rather than in a panel (設計 3.4). It goes through the same save
// road as every other field (N4).
func (s *Server) missionEdit(r *http.Request) (view, error) {
	name := r.PathValue("name")
	mission := r.PostFormValue("mission")
	edit := teamEdit{Op: definition.Mission(mission), Key: "mission", Label: t("label.mission"), Value: mission, Hash: r.PostFormValue("hash")}
	return s.saveTeamYAML(name, nil, edit)
}

// memberEdit writes one place of a member's definition (設計 3.4 の表): its role,
// its profile, or one of the workers it may hire.
func (s *Server) memberEdit(r *http.Request) (view, error) {
	name, member := r.PathValue("name"), r.PathValue("member")
	def, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	open := s.memberOpen(member)
	node, ok := def.Graph.Member(member)
	if !ok {
		// A field the definition does not hold is a 404, not a write (N2).
		return s.definitionPage(name, open)
	}
	edit, err := memberOp(node, r)
	if err != nil {
		return s.editRefused(def, open, edit, err.Error(), false)
	}
	edit.Hash = r.PostFormValue("hash")
	return s.saveTeamYAML(name, open, edit)
}

// workerEdit writes one worker's profile, or takes the worker away (設計 3.4 の
// 表). A worker taken away leaves the field it stood in: the screen lands on the
// member that hired it, so the write it just made is not the one the page says is
// not there (N1).
func (s *Server) workerEdit(r *http.Request) (view, error) {
	name, from, purpose := r.PathValue("name"), r.PathValue("from"), r.PathValue("purpose")
	def, err := s.readDefinition(name)
	if err != nil {
		return view{}, err
	}
	open := s.workerOpen(from, purpose)
	if workerIndex(def, from, purpose) < 0 {
		return s.definitionPage(name, open)
	}
	edit, err := workerOp(from, purpose, r)
	if err != nil {
		return s.editRefused(def, open, edit, err.Error(), false)
	}
	edit.Hash = r.PostFormValue("hash")
	if edit.Op.Kind == definition.OpRemoveWorker {
		open = s.memberOpen(from)
	}
	return s.saveTeamYAML(name, open, edit)
}

// memberOp is the edit one member's form names, and the sentence the field is
// refused with when the form names nothing to write. An input a person left
// empty is refused in the field's own words, not with the place a broken file
// would be told about (N2); a value the file cannot read is still validate's
// sentence, from the check.
func memberOp(member definition.MemberNode, r *http.Request) (teamEdit, error) {
	switch r.PostFormValue("op") {
	case opMemberRole:
		role := r.PostFormValue("role")
		if role == "" {
			return teamEdit{}, i18n.Errorf("web.need.role")
		}
		return teamEdit{Op: definition.MemberRole(member.Name, role), Key: "role", Label: t("label.role"), Value: role}, nil
	case opMemberProfile:
		profile := r.PostFormValue("profile")
		if profile == "" {
			return teamEdit{}, i18n.Errorf("web.need.profile")
		}
		return teamEdit{Op: definition.MemberProfile(member.Name, profile), Key: "profile", Label: t("label.profile"), Value: profile}, nil
	case opWorkerAdd:
		purpose := strings.TrimSpace(r.PostFormValue("purpose"))
		profile := r.PostFormValue("profile")
		if purpose == "" {
			return teamEdit{}, i18n.Errorf("web.need.purpose")
		}
		if profile == "" {
			return teamEdit{}, i18n.Errorf("web.need.profile")
		}
		return teamEdit{Op: definition.AddWorker(member.Name, purpose, profile), Key: "worker-add", Label: t("label.workers"), Value: purpose}, nil
	case opWorkerRemove:
		purpose := r.PostFormValue("worker")
		return teamEdit{Op: definition.RemoveWorker(member.Name, purpose), Key: workerKey(member.Name, purpose), Label: t("label.workers"), Value: purpose}, nil
	case opWorkerProfile:
		return workerProfileOp(member.Name, r.PostFormValue("worker"), r.PostFormValue("profile"))
	default:
		return teamEdit{}, i18n.Errorf("error.edit_unknown_op")
	}
}

// workerOp is the edit one worker's own field names (設計 3.4 の表).
func workerOp(from, purpose string, r *http.Request) (teamEdit, error) {
	switch r.PostFormValue("op") {
	case opWorkerProfile:
		return workerProfileOp(from, purpose, r.PostFormValue("profile"))
	case opWorkerRemove:
		return teamEdit{Op: definition.RemoveWorker(from, purpose), Key: workerKey(from, purpose), Label: t("label.workers"), Value: purpose}, nil
	default:
		return teamEdit{}, i18n.Errorf("error.edit_unknown_op")
	}
}

// workerProfileOp is one worker's profile, named by a profile name.
func workerProfileOp(from, purpose, profile string) (teamEdit, error) {
	if profile == "" {
		return teamEdit{}, i18n.Errorf("web.need.profile")
	}
	return teamEdit{Op: definition.WorkerProfile(from, purpose, profile), Key: workerKey(from, purpose), Label: t("label.profile"), Value: profile}, nil
}

// memberOpen and workerOpen are the panel one definition's page opens, for the
// handlers that draw a page after a write.
func (s *Server) memberOpen(member string) panelFn {
	return func(def *definition.Definition) panel {
		return s.memberPanelOf(def, member, s.findings(def.Name), pendingEdit{})
	}
}

func (s *Server) workerOpen(from, purpose string) panelFn {
	return func(def *definition.Definition) panel {
		return s.workerPanelOf(def, from, purpose, pendingEdit{})
	}
}
