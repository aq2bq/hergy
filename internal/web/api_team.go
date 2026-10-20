package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// apiTeam is the team kind (spec-boundary 4, 8). In this request it carries what
// the two 割り当て tabs read and write: one team definition's members and
// workers. The team-definition screen and the agreement arrive in the request
// after this one; the shape below is the one they will grow into.
type apiTeam struct{ s *Server }

// apiTeamDoc is one team definition as the screen drafts it: the people, and who
// each of them plays and reads.
type apiTeamDoc struct {
	Name    string          `json:"name"`
	Members []apiTeamMember `json:"members"`
	Workers []apiTeamWorker `json:"workers"`
}

type apiTeamMember struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Profile string `json:"profile"`
}

type apiTeamWorker struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// apiTeamDraft is the doc one 割り当て tab sends on a save: the picks it changed,
// and nothing else. A member's role or profile is left out when that pick did
// not move, so a card that only changed roles does not blank the profiles
// (spec-boundary 6).
type apiTeamDraft struct {
	Name    string            `json:"name"`
	Members []apiTeamDraftRow `json:"members"`
	Workers []apiTeamDraftRow `json:"workers"`
}

type apiTeamDraftRow struct {
	Name    string  `json:"name"`
	From    string  `json:"from"`
	Purpose string  `json:"purpose"`
	Role    *string `json:"role"`
	Profile *string `json:"profile"`
}

// apiTeamItem is one team definition in the list.
type apiTeamItem struct {
	Name    string          `json:"name"`
	Path    string          `json:"path"`
	Hash    string          `json:"hash"`
	Members []apiTeamMember `json:"members"`
	Workers []apiTeamWorker `json:"workers"`
}

// List is GET /api/team: every team definition and the people in it.
func (k apiTeam) List() (any, *apiErr) {
	defs := k.s.p2Defs()
	items := make([]apiTeamItem, 0, len(defs))
	for _, def := range defs {
		items = append(items, apiTeamItemOf(def))
	}
	return map[string]any{"items": items}, nil
}

// apiTeamItemOf is one definition as the list carries it.
func apiTeamItemOf(def p2Def) apiTeamItem {
	item := apiTeamItem{Name: def.name, Path: def.display, Hash: def.hash}
	for _, member := range def.members {
		item.Members = append(item.Members, apiTeamMember(member))
	}
	for _, worker := range def.workers {
		item.Workers = append(item.Workers, apiTeamWorker(worker))
	}
	return item
}

// Get is GET /api/team/{name}: one definition's document and text.
func (k apiTeam) Get(name, _ string) (any, *apiErr) {
	def, ok := k.s.p2FindDef(name)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	text, err := k.s.cat.Reader().Read(source.Doc{Path: def.path})
	if err != nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	return map[string]any{
		"doc":  apiTeamDocOf(def),
		"text": text,
		"hash": source.Hash(text),
		"path": def.display,
	}, nil
}

// apiTeamDocOf is one definition as the screen's draft.
func apiTeamDocOf(def p2Def) apiTeamDoc {
	doc := apiTeamDoc{Name: def.name}
	for _, member := range def.members {
		doc.Members = append(doc.Members, apiTeamMember(member))
	}
	for _, worker := range def.workers {
		doc.Workers = append(doc.Workers, apiTeamWorker(worker))
	}
	return doc
}

// Check is POST /api/team/check: what is wrong with one team definition. It is
// the same check.Definition the command line runs.
func (k apiTeam) Check(in apiIn) (any, *apiErr) {
	findings := check.Definition(check.Input{Catalog: k.s.cat, Name: in.Name})
	return map[string]any{"problems": findings}, nil
}

// Save is PUT /api/team/{name}: the doc with members[].profile / role 替えた,
// written through the same ops the old assign saves used, so every other line of
// the file stays as it was (spec-boundary 6, 20).
func (k apiTeam) Save(name string, in apiIn) (any, *apiErr) {
	s := k.s
	def, ok := s.p2FindDef(name)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	if len(in.Doc) == 0 {
		return nil, refuse(http.StatusUnprocessableEntity, "team doc missing")
	}
	var doc apiTeamDraft
	if err := json.Unmarshal(in.Doc, &doc); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: def.path})
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if in.Hash != source.Hash(text) {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", def.display))
	}
	ops, changes := apiTeamOps(def, doc)
	edited := text
	for _, op := range ops {
		next, err := definition.EditYAML([]byte(edited), def.display, op)
		if err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, err.Error())
		}
		edited = string(next)
	}
	if problem, bad := s.p2MissingProfile(def, changes); bad {
		return nil, refuse(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(def.path, source.Hash(text), edited); err != nil {
		return p2WriteError(err, def.display)
	}
	def2, _ := s.p2FindDef(name)
	return map[string]any{"hash": source.Hash(edited), "team": apiTeamItemOf(def2)}, nil
}

// Delete is DELETE /api/team/{name}: take one team definition's file away.
func (k apiTeam) Delete(name, _ string, hash string) (any, *apiErr) {
	def, ok := k.s.p2FindDef(name)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	text, err := k.s.cat.Reader().Read(source.Doc{Path: def.path})
	if err != nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	if hash != source.Hash(text) {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", def.display))
	}
	if err := os.Remove(def.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	return k.List()
}

// apiTeamOps is the ops one draft means: every member's and worker's pick that
// changed, and the same changes in the words p2MissingProfile reads.
func apiTeamOps(def p2Def, doc apiTeamDraft) ([]definition.Op, []p2AssignChange) {
	var ops []definition.Op
	var changes []p2AssignChange
	savedMembers := map[string]p2Member{}
	for _, member := range def.members {
		savedMembers[member.Name] = member
	}
	for _, row := range doc.Members {
		saved, ok := savedMembers[row.Name]
		if !ok {
			continue
		}
		if row.Profile != nil && *row.Profile != saved.Profile {
			ops = append(ops, definition.MemberProfile(row.Name, *row.Profile))
			changes = append(changes, p2AssignChange{Member: row.Name, Profile: *row.Profile})
		}
		if row.Role != nil && *row.Role != saved.Role {
			ops = append(ops, definition.MemberRole(row.Name, *row.Role))
		}
	}
	type workerKey struct{ from, purpose string }
	savedWorkers := map[workerKey]p2Worker{}
	for _, worker := range def.workers {
		savedWorkers[workerKey{worker.From, worker.Purpose}] = worker
	}
	for _, row := range doc.Workers {
		saved, ok := savedWorkers[workerKey{row.From, row.Purpose}]
		if !ok {
			continue
		}
		if row.Profile != nil && *row.Profile != saved.Profile {
			ops = append(ops, definition.WorkerProfile(row.From, row.Purpose, *row.Profile))
			changes = append(changes, p2AssignChange{From: row.From, Purpose: row.Purpose, Profile: *row.Profile})
		}
	}
	return ops, changes
}
