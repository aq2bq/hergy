package ops

import (
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// ResourcePromptNames is the shared resource model.
type ResourcePromptNames struct {
	Team         string
	Naming       domainname.Naming
	Kind, Status string
	Runs         []PromptRun
	Selected     *PromptRun
	Reason       string
}

// ResourcePromptText is the shared resource model.
type ResourcePromptText struct {
	Name, Text string
	Segments   []definition.PromptSegment
	Role       source.Doc
	HasRole    bool
	Read       []source.Doc
}

// ResourceWorkerPrompt is the shared resource model.
type ResourceWorkerPrompt struct {
	Name, Purpose, Task, From string
	Preview                   bool
}

// ResourcePrompts is the shared resource model.
type ResourcePrompts struct {
	Names     ResourcePromptNames
	Built     []ResourcePromptText
	Worker    *ResourcePromptText
	WorkerErr error
	Project   domainplace.Project
	Cat       *catalog.Catalog
	Def       *definition.Definition
	DefErr    error
	Errors    map[string]error
}

// ResourcePromptInput is the shared resource model.
type ResourcePromptInput struct {
	Target                             ResourceTarget
	Hash                               string
	Ops                                []definition.Op
	Draft                              *DefinitionDraft
	Pick, Member, Executable, Language string
	WithRuns                           bool
	Runs                               func(domainplace.Project, string, string) ([]PromptRun, error)
	AtProject                          func(domainplace.Project) (*catalog.Catalog, error)
	Catalog                            *catalog.Catalog
	Definition                         *definition.Definition
	Worker                             *ResourceWorkerPrompt
}

// PreviewResourcePrompts chooses the run name and reads the complete draft once.
// All entry points receive complete texts; prompt builders remain private.
func (s ResourceScope) PreviewResourcePrompts(in ResourcePromptInput) (*ResourcePrompts, *ResourceFailure) {
	_, at, n := in.Target.values()
	cat := s.Catalog
	project := cat.Base()
	if in.Catalog != nil {
		cat = in.Catalog
		project = cat.Base()
	}
	out := &ResourcePrompts{Names: ResourcePromptNames{Team: n, Kind: "definition", Status: "none", Runs: []PromptRun{}}, Errors: map[string]error{}}
	if in.WithRuns && in.Runs != nil {
		runs, err := in.Runs(project, n, at.API())
		if err != nil {
			out.Names.Status, out.Names.Reason = "error", err.Error()
		} else {
			out.Names = SelectResourcePromptName(n, runs, in.Pick)
		}
	}
	if out.Names.Kind == "run" {
		out.Names.Naming = domainname.RunningNames(out.Names.Selected.Team)
	} else {
		d, _ := domainname.ParseDefinitionName(n)
		out.Names.Naming = domainname.DefinitionNames(d)
	}
	if in.Catalog == nil && out.Names.Selected != nil && out.Names.Selected.Project.Compare(project) != domainplace.Same && (!out.Names.Selected.Project.IsZero() || !project.IsZero()) {
		if in.AtProject != nil {
			next, err := in.AtProject(out.Names.Selected.Project)
			if err != nil {
				return nil, &ResourceFailure{Text: err.Error()}
			}
			cat = next
			project = cat.Base()
		}
	}
	overlay, bad := resourcePromptOverlay(cat, in, n, at)
	if bad != nil {
		return nil, bad
	}
	out.Project, out.Cat = project, cat
	out.Def = in.Definition
	if out.Def == nil {
		out.Def, out.DefErr = overlay.Definition(n, at.Layer())
	}
	if out.Def == nil || out.Def.Graph == nil {
		return out, nil
	}
	if in.Worker != nil {
		lead, err := resourceHiringMember(out.Def, in.Worker.Purpose, in.Worker.From)
		if err != nil {
			out.WorkerErr = err
			return out, nil
		}
		names := out.Names.Naming
		built, err := prompt.Worker(prompt.WorkerInput{Team: names.String(), Lead: names.Member(lead.Name), LeadRole: lead.Role, Purpose: in.Worker.Purpose, Name: names.Worker(in.Worker.Name), Task: in.Worker.Task})
		if err != nil {
			out.WorkerErr = err
			return out, nil
		}
		out.Worker = &ResourcePromptText{Name: names.Worker(in.Worker.Name), Text: built.Text, Segments: built.Segments}
		return out, nil
	}
	run := prompt.Run{Team: out.Names.Naming, Base: project, Executable: in.Executable, Lang: in.Language, Reader: overlay.Reader(), Sources: overlay, Display: overlay.Display}
	for i := range out.Def.Graph.Members {
		member := &out.Def.Graph.Members[i]
		if in.Member != "" && member.Name.String() != in.Member {
			continue
		}
		built, err := prompt.Member(out.Def, member, run)
		if err != nil {
			out.Errors[member.Name.String()] = err
			continue
		}
		out.Built = append(out.Built, ResourcePromptText{Name: member.Name.String(), Text: built.Text, Segments: built.Segments, Role: built.Role, HasRole: built.HasRole, Read: built.Read})
	}
	return out, nil
}

// SelectResourcePromptName reads or changes the selected resource.
func SelectResourcePromptName(definition string, runs []PromptRun, picked string) ResourcePromptNames {
	out := ResourcePromptNames{Runs: runs, Team: definition, Kind: "definition", Status: "none"}
	if picked != "" {
		for _, run := range runs {
			if run.Team.String() == picked {
				out.Team, out.Kind, out.Status, out.Selected = picked, "run", "picked", &run
				return out
			}
		}
		if len(runs) > 0 {
			out.Status = "choose"
		}
		return out
	}
	if len(runs) == 1 {
		selected := runs[0]
		out.Team, out.Kind, out.Status, out.Selected = selected.Team.String(), "run", "single", &selected
	} else if len(runs) > 1 {
		out.Status = "choose"
	}
	return out
}
func resourceHiringMember(def *definition.Definition, purpose, from string) (*definition.MemberNode, error) {
	if def == nil || def.Graph == nil {
		return nil, i18n.Errorf("error.no_member", from)
	}
	if from != "" {
		for i := range def.Graph.Members {
			m := &def.Graph.Members[i]
			if m.Name.String() == from || domainname.DefinitionNames(def.Name).Member(m.Name) == from {
				return m, nil
			}
		}
		return nil, i18n.Errorf("error.no_member", from)
	}
	var names []string
	for _, m := range def.Graph.Members {
		for _, w := range def.Graph.Workers {
			if w.From == m.Name.String() && w.Purpose.String() == purpose {
				names = append(names, m.Name.String())
				break
			}
		}
	}
	if len(names) == 0 {
		return nil, i18n.Errorf("error.worker_prompt_none", purpose)
	}
	if len(names) == 1 {
		for i := range def.Graph.Members {
			if def.Graph.Members[i].Name.String() == names[0] {
				return &def.Graph.Members[i], nil
			}
		}
	}
	return nil, i18n.Errorf("error.from_required_many", strings.Join(names, i18n.T("list.separator")))
}

func resourcePromptOverlay(cat *catalog.Catalog, in ResourcePromptInput, n string, at domainplace.Place) (*catalog.Catalog, *ResourceFailure) {
	overlay := cat
	if in.Catalog == nil {
		path := cat.TeamYAML(n, at.Layer())
		if in.Draft != nil {
			overlay = cat.With(path, in.Draft.Team).With(filepath.Join(filepath.Dir(path), "agreement.md"), in.Draft.Agreement)
		} else {
			text, err := cat.Reader().Read(source.Doc{Path: path})
			if err != nil {
				return nil, &ResourceFailure{Text: err.Error()}
			}
			if in.Hash != source.Hash(text) {
				return nil, &ResourceFailure{Status: 409, Text: "定義が変わりました。ファイルからリロードしてください"}
			}
			overlay = cat.With(path, text)
		}
		text, err := overlay.Reader().Read(source.Doc{Path: path})
		if err != nil {
			return nil, &ResourceFailure{Text: err.Error()}
		}
		for _, op := range in.Ops {
			edited, err := definition.EditYAML([]byte(text), cat.Display(path), op)
			if err != nil {
				return nil, &ResourceFailure{Text: err.Error()}
			}
			text = string(edited)
		}
		overlay = overlay.With(path, text)
	}
	return overlay, nil
}
