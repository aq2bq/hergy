package web

import (
	"errors"
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"
)

// FromOps is the sole adapter from operation read models to HTTP DTOs.
func FromOps(read *ops.Reader) ReadAdapter { return &opsReader{read: read} }

type opsReader struct{ read *ops.Reader }

func (r *opsReader) List() ActivityList { return dtoActivityList(r.read.List()) }
func (r *opsReader) One(name string) (ActivityDoc, bool) {
	v, ok := r.read.One(name)
	return dtoActivityDoc(v), ok
}
func (r *opsReader) StopTargets(name string) ([]string, int, bool) { return r.read.StopTargets(name) }
func (r *opsReader) FocusMember(team, member string) error {
	return dtoReadError(r.read.FocusMember(team, member))
}
func (r *opsReader) Apply() ApplyList { return ApplyDTO(r.read.Apply()) }
func (r *opsReader) ApplyCount() int  { return r.read.ApplyCount() }
func (r *opsReader) ApplyMembers(names []string) (HQLaunch, error) {
	v, e := r.read.ApplyMembers(names)
	return dtoHQLaunch(v), dtoReadError(e)
}
func (r *opsReader) Projects() (HQProjects, error) {
	v, e := r.read.Projects()
	return dtoHQProjects(v), dtoReadError(e)
}
func (r *opsReader) AddProject(path string, at int) (HQProjects, error) {
	v, e := r.read.AddProject(path, at)
	return dtoHQProjects(v), dtoReadError(e)
}
func (r *opsReader) RemoveProject(path string) (HQProjects, int, error) {
	v, i, e := r.read.RemoveProject(path)
	return dtoHQProjects(v), i, dtoReadError(e)
}
func (r *opsReader) Running() HQRunning       { return dtoHQRunning(r.read.Running()) }
func (r *opsReader) Workspaces() HQWorkspaces { return dtoHQWorkspaces(r.read.Workspaces()) }
func (r *opsReader) LaunchOptions(project domainplace.Input, definition, place string) (HQLaunchOptions, error) {
	v, e := r.read.LaunchOptions(project, definition, place)
	return dtoHQLaunchOptions(v), dtoReadError(e)
}
func (r *opsReader) StartLaunch(in HQLaunchIn) (HQLaunch, error) {
	v, e := r.read.StartLaunch(ops.HQLaunchIn{Project: in.Project, Definition: in.Definition, Place: in.Place, Name: in.Name, Workspace: ops.HQWorkspaceIn{ID: in.Workspace.ID, New: in.Workspace.New}})
	return dtoHQLaunch(v), dtoReadError(e)
}
func (r *opsReader) LaunchState(id string) (HQLaunch, bool) {
	v, ok := r.read.LaunchState(id)
	return dtoHQLaunch(v), ok
}
func (r *opsReader) CancelLaunch(id string) (HQLaunch, bool) {
	v, ok := r.read.CancelLaunch(id)
	return dtoHQLaunch(v), ok
}
func (r *opsReader) Stop(name string) (HQStopped, error) {
	v, e := r.read.Stop(name)
	return dtoHQStopped(v), dtoReadError(e)
}
func (r *opsReader) ProjectCatalog(path domainplace.Project) (*catalog.Catalog, error) {
	v, e := r.read.ProjectCatalog(path)
	return v, dtoReadError(e)
}
func (r *opsReader) GlobalCatalog() *catalog.Catalog { return r.read.GlobalCatalog() }
func (r *opsReader) CheckDefinitionRename(path domainplace.Project) error {
	return dtoReadError(r.read.CheckDefinitionRename(path))
}
func (r *opsReader) PromptRuns(project domainplace.Project, definition, place string) ([]PromptRun, error) {
	v, e := r.read.PromptRuns(project, definition, place)
	var out []PromptRun
	if v != nil {
		out = make([]PromptRun, len(v))
		for i, x := range v {
			out[i] = dtoPromptRun(x)
		}
	}
	return out, dtoReadError(e)
}
func (r *opsReader) PromptHistory(project domainplace.Project, definition, place, member, running string) (PromptHistory, error) {
	v, e := r.read.PromptHistory(project, definition, place, member, running)
	return dtoPromptHistory(v), dtoReadError(e)
}
func dtoReadError(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*ops.HQError](err); ok {
		return &HQError{Status: e.Status, At: e.At, Text: e.Text, SuggestedName: e.SuggestedName}
	}
	return &HQError{Status: http.StatusInternalServerError, Text: err.Error()}
}

// ReadAdapter is the HTTP-facing interface implemented by the ops conversion boundary.
type ReadAdapter interface {
	Activity
	HQ
	Applier
	PromptReader
	CatalogProvider
	FocusMember(team, member string) error
	CheckDefinitionRename(domainplace.Project) error
}
