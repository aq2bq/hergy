package web

import (
	"errors"
	"fmt"
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
// clears just that field's draft, C5), and the word the saved line names the
// place by.
type teamEdit struct {
	Op    definition.Op
	Key   string
	Label string
	Hash  string
}

// saveTeamYAML runs one edit of a definition's team.yaml through the save road
// (設計 7.3) and draws the definition's page with the edited field open again.
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
		return s.editRefused(def, open, err.Error(), false)
	}
	with := s.cat.With(def.TeamYAML, string(edited))
	if msg, ok := newStop(s.findings(name), check.Definition(check.Input{Catalog: with, Name: name})); ok {
		return s.editRefused(def, open, msg, false)
	}
	if err := source.WriteIfUnchanged(def.TeamYAML, edit.Hash, string(edited)); err != nil {
		if errors.Is(err, source.ErrChanged) {
			return s.editRefused(def, open, tf("web.saved.clash", s.cat.Display(def.TeamYAML)), true)
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
// it, the file as it was. The field keeps what the person wrote: the form's
// draft is still in the browser, and the field reads it back (設計 3.0 の
// 「書きかけ」). reload adds the link that reads the file again, for a file that
// moved under the page.
func (s *Server) editRefused(def *definition.Definition, open panelFn, notice string, reload bool) (view, error) {
	data := s.definitionData(def, open)
	data.Panel.Notice = notice
	data.Panel.Reload = reload
	return s.renderDefinition(data)
}

// editSaved draws the page after a save: the saved line, what the change does to
// the members' prompts (effect.Preview), and what a running team has to take up
// (effect.Commands, with a copy button on each command). Only the form that was
// written is told, so the script clears its draft and leaves the others (C5).
func (s *Server) editSaved(after *definition.Definition, open panelFn, edit teamEdit, changes []effect.Change) (view, error) {
	running, _ := s.runningTeams()
	data := s.definitionData(after, open)
	data.Panel.Saved = tf("web.saved.line", s.cat.Display(after.TeamYAML), edit.Label)
	data.Panel.Preview = effect.Preview(changes)
	data.Panel.Handover = handoverOf(effect.Commands(changes, running))
	data.Panel.markSaved(edit.Key)
	return s.renderDefinition(data)
}

// previewOf is what an edit would say, as effect.Preview writes it: the edit is
// made on the text in memory, read back through the catalog's overlay, and
// compared with the definition as it stands. Nothing is written (設計 7.3 の 5).
func (s *Server) previewOf(def *definition.Definition, op definition.Op) []string {
	text, err := s.cat.Reader().Read(source.Doc{Path: def.TeamYAML})
	if err != nil {
		return nil
	}
	edited, err := definition.EditYAML([]byte(text), op)
	if err != nil {
		return nil
	}
	with := s.cat.With(def.TeamYAML, string(edited))
	after, err := with.Read(def.Name)
	if err != nil {
		return nil
	}
	return effect.Preview(effect.Of(s.view(def, s.cat.Reader()), s.view(after, with.Reader())))
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
	edit, err := memberOp(def, node, r)
	if err != nil {
		return s.editRefused(def, open, err.Error(), false)
	}
	edit.Hash = r.PostFormValue("hash")
	return s.saveTeamYAML(name, open, edit)
}

// workerEdit writes one worker's profile, or takes the worker away (設計 3.4 の
// 表).
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
	edit, err := workerOp(def, from, purpose, r)
	if err != nil {
		return s.editRefused(def, open, err.Error(), false)
	}
	edit.Hash = r.PostFormValue("hash")
	return s.saveTeamYAML(name, open, edit)
}

// memberOp is the edit one member's form names, and the sentence validate writes
// when the form names nothing to write.
func memberOp(def *definition.Definition, member definition.MemberNode, r *http.Request) (teamEdit, error) {
	index := memberIndex(def, member.Name)
	switch r.PostFormValue("op") {
	case opMemberRole:
		role := r.PostFormValue("role")
		if role == "" {
			return teamEdit{}, i18n.Errorf("error.role_empty", fmt.Sprintf("members[%d]", index))
		}
		return teamEdit{Op: definition.MemberRole(member.Name, role), Key: "role", Label: t("label.role")}, nil
	case opMemberProfile:
		profile := r.PostFormValue("profile")
		if profile == "" {
			return teamEdit{}, i18n.Errorf("error.member_needs_profile", member.Name)
		}
		return teamEdit{Op: definition.MemberProfile(member.Name, profile), Key: "profile", Label: t("label.profile")}, nil
	case opWorkerAdd:
		purpose := strings.TrimSpace(r.PostFormValue("purpose"))
		profile := r.PostFormValue("profile")
		label := fmt.Sprintf("workers[%d]", len(def.Graph.Workers))
		if purpose == "" {
			return teamEdit{}, i18n.Errorf("error.workers_purpose_empty", label)
		}
		if profile == "" {
			return teamEdit{}, i18n.Errorf("error.workers_profile_empty", label)
		}
		return teamEdit{Op: definition.AddWorker(member.Name, purpose, profile), Key: "worker-add", Label: t("label.workers")}, nil
	case opWorkerRemove:
		purpose := r.PostFormValue("worker")
		return teamEdit{Op: definition.RemoveWorker(member.Name, purpose), Key: workerKey(member.Name, purpose), Label: t("label.workers")}, nil
	case opWorkerProfile:
		return workerProfileOp(def, member.Name, r.PostFormValue("worker"), r.PostFormValue("profile"))
	default:
		return teamEdit{}, i18n.Errorf("error.edit_unknown_op")
	}
}

// workerOp is the edit one worker's own field names (設計 3.4 の表).
func workerOp(def *definition.Definition, from, purpose string, r *http.Request) (teamEdit, error) {
	switch r.PostFormValue("op") {
	case opWorkerProfile:
		return workerProfileOp(def, from, purpose, r.PostFormValue("profile"))
	case opWorkerRemove:
		return teamEdit{Op: definition.RemoveWorker(from, purpose), Key: workerKey(from, purpose), Label: t("label.workers")}, nil
	default:
		return teamEdit{}, i18n.Errorf("error.edit_unknown_op")
	}
}

// workerProfileOp is one worker's profile, named by a profile name.
func workerProfileOp(def *definition.Definition, from, purpose, profile string) (teamEdit, error) {
	if profile == "" {
		label := fmt.Sprintf("workers[%d]", workerIndex(def, from, purpose))
		return teamEdit{}, i18n.Errorf("error.workers_profile_empty", label)
	}
	return teamEdit{Op: definition.WorkerProfile(from, purpose, profile), Key: workerKey(from, purpose), Label: t("label.profile")}, nil
}

// memberOpen and workerOpen are the panel one definition's page opens, for the
// handlers that draw a page after a write.
func (s *Server) memberOpen(member string) panelFn {
	return func(def *definition.Definition) panel {
		return s.memberPanelOf(def, member, s.findings(def.Name), "")
	}
}

func (s *Server) workerOpen(from, purpose string) panelFn {
	return func(def *definition.Definition) panel {
		return s.workerPanelOf(def, from, purpose, "")
	}
}
