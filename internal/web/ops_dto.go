package web

import "github.com/aq2bq/hergy/internal/ops"

func dtoHQProjects(in ops.HQProjects) HQProjects {
	out := HQProjects{}
	out.RemovedAt = in.RemovedAt
	out.Config = in.Config
	out.Mode = in.Mode
	out.Current = dtoHQProject(in.Current)
	if in.Projects != nil {
		out.Projects = make([]HQProject, len(in.Projects))
		for i, v := range in.Projects {
			out.Projects[i] = dtoHQProject(v)
		}
	}
	out.Global = dtoHQGlobal(in.Global)
	return out
}
func dtoHQProject(in ops.HQProject) HQProject {
	out := HQProject{}
	out.Path = in.Path
	out.Name = in.Name
	out.Display = in.Display
	out.Listed = in.Listed
	out.Current = in.Current
	out.Exists = in.Exists
	out.HasHergy = in.HasHergy
	out.Defs = in.Defs
	out.Roles = in.Roles
	out.Profiles = in.Profiles
	out.Running = in.Running
	out.LastLaunch = in.LastLaunch
	if in.LastWorkspace != nil {
		v := dtoHQWorkspace(*in.LastWorkspace)
		out.LastWorkspace = &v
	}
	out.Source = in.Source
	return out
}
func dtoHQGlobal(in ops.HQGlobal) HQGlobal {
	out := HQGlobal{}
	out.Path = in.Path
	out.Display = in.Display
	out.Defs = in.Defs
	out.Roles = in.Roles
	out.Profiles = in.Profiles
	return out
}
func dtoHQRef(in ops.HQRef) HQRef {
	out := HQRef{}
	out.Path = in.Path
	out.Name = in.Name
	out.Display = in.Display
	return out
}
func dtoHQRunning(in ops.HQRunning) HQRunning {
	out := HQRunning{}
	out.Herdr = dtoActivityNote(in.Herdr)
	if in.Teams != nil {
		out.Teams = make([]HQRunningTeam, len(in.Teams))
		for i, v := range in.Teams {
			out.Teams[i] = dtoHQRunningTeam(v)
		}
	}
	return out
}
func dtoHQRunningTeam(in ops.HQRunningTeam) HQRunningTeam {
	out := HQRunningTeam{}
	out.Team = in.Team
	out.Definition = in.Definition
	out.Place = in.Place
	out.Project = in.Project
	out.ProjectName = in.ProjectName
	out.Workspace = dtoHQWorkspace(in.Workspace)
	out.Tab = dtoHQTab(in.Tab)
	out.Starting = in.Starting
	out.Launch = in.Launch
	if in.Members != nil {
		out.Members = make([]HQMember, len(in.Members))
		for i, v := range in.Members {
			out.Members[i] = dtoHQMember(v)
		}
	}
	if in.Workers != nil {
		out.Workers = make([]HQWorker, len(in.Workers))
		for i, v := range in.Workers {
			out.Workers[i] = dtoHQWorker(v)
		}
	}
	out.Waiting = in.Waiting
	out.NotUp = in.NotUp
	out.Error = in.Error
	return out
}
func dtoHQWorkspace(in ops.HQWorkspace) HQWorkspace {
	out := HQWorkspace{}
	out.ID = in.ID
	out.Label = in.Label
	out.Focused = in.Focused
	out.Team = in.Team
	out.TeamProject = in.TeamProject
	out.LastTeam = in.LastTeam
	return out
}
func dtoHQTab(in ops.HQTab) HQTab {
	out := HQTab{}
	out.ID = in.ID
	out.Label = in.Label
	return out
}
func dtoHQMember(in ops.HQMember) HQMember {
	out := HQMember{}
	out.Name = in.Name
	out.Herdr = in.Herdr
	out.Live = in.Live
	out.State = in.State
	return out
}
func dtoHQWorker(in ops.HQWorker) HQWorker {
	out := HQWorker{}
	out.Name = in.Name
	out.Tab = in.Tab
	out.Live = in.Live
	return out
}
func dtoHQWorkspaces(in ops.HQWorkspaces) HQWorkspaces {
	out := HQWorkspaces{}
	if in.Workspaces != nil {
		out.Workspaces = make([]HQWorkspace, len(in.Workspaces))
		for i, v := range in.Workspaces {
			out.Workspaces[i] = dtoHQWorkspace(v)
		}
	}
	return out
}
func dtoHQLaunchOptions(in ops.HQLaunchOptions) HQLaunchOptions {
	out := HQLaunchOptions{}
	out.Definition = in.Definition
	out.Description = in.Description
	out.Place = in.Place
	if in.Project != nil {
		v := dtoHQRef(*in.Project)
		out.Project = &v
	}
	if in.Members != nil {
		out.Members = make([]HQMember, len(in.Members))
		for i, v := range in.Members {
			out.Members[i] = dtoHQMember(v)
		}
	}
	if in.Projects != nil {
		out.Projects = make([]HQLaunchProject, len(in.Projects))
		for i, v := range in.Projects {
			out.Projects[i] = dtoHQLaunchProject(v)
		}
	}
	if in.Clash != nil {
		out.Clash = make([]HQClash, len(in.Clash))
		for i, v := range in.Clash {
			out.Clash[i] = dtoHQClash(v)
		}
	}
	out.SuggestedName = in.SuggestedName
	if in.LastWorkspace != nil {
		v := dtoHQWorkspace(*in.LastWorkspace)
		out.LastWorkspace = &v
	}
	out.NewWorkspaceName = in.NewWorkspaceName
	if in.Workspaces != nil {
		out.Workspaces = make([]HQWorkspaceChoice, len(in.Workspaces))
		for i, v := range in.Workspaces {
			out.Workspaces[i] = dtoHQWorkspaceChoice(v)
		}
	}
	return out
}
func dtoHQLaunchProject(in ops.HQLaunchProject) HQLaunchProject {
	out := HQLaunchProject{}
	out.Path = in.Path
	out.Name = in.Name
	out.Available = in.Available
	out.Reason = in.Reason
	return out
}
func dtoHQClash(in ops.HQClash) HQClash {
	out := HQClash{}
	out.Team = in.Team
	out.Project = in.Project
	return out
}
func dtoHQWorkspaceChoice(in ops.HQWorkspaceChoice) HQWorkspaceChoice {
	out := HQWorkspaceChoice{}
	out.HQWorkspace = dtoHQWorkspace(in.HQWorkspace)
	out.Selectable = in.Selectable
	out.Why = in.Why
	return out
}

func dtoHQLaunch(in ops.HQLaunch) HQLaunch {
	out := HQLaunch{}
	out.ID = in.ID
	out.Action = in.Action
	out.Team = in.Team
	out.Definition = in.Definition
	out.Place = in.Place
	out.Project = in.Project
	out.ProjectName = in.ProjectName
	out.Workspace = dtoHQWorkspace(in.Workspace)
	out.Tab = dtoHQTab(in.Tab)
	out.Phase = in.Phase
	if in.Steps != nil {
		out.Steps = make([]HQLaunchStep, len(in.Steps))
		for i, v := range in.Steps {
			out.Steps[i] = dtoHQLaunchStep(v)
		}
	}
	if in.Waiting != nil {
		v := dtoHQWaiting(*in.Waiting)
		out.Waiting = &v
	}
	out.Error = in.Error
	out.Members = in.Members
	return out
}
func dtoHQLaunchStep(in ops.HQLaunchStep) HQLaunchStep {
	out := HQLaunchStep{}
	out.At = in.At
	out.Kind = in.Kind
	out.Text = in.Text
	return out
}
func dtoHQWaiting(in ops.HQWaiting) HQWaiting {
	out := HQWaiting{}
	out.Member = in.Member
	out.Herdr = in.Herdr
	out.Workspace = in.Workspace
	out.Tab = in.Tab
	out.Pane = in.Pane
	out.Command = in.Command
	return out
}
func dtoHQStopped(in ops.HQStopped) HQStopped {
	out := HQStopped{}
	out.Team = in.Team
	out.Definition = in.Definition
	out.Project = in.Project
	out.Workspace = dtoHQWorkspace(in.Workspace)
	out.Closed = dtoHQClosed(in.Closed)
	out.StoppedAt = in.StoppedAt
	out.Leftovers = dtoHQLeftovers(in.Leftovers)
	return out
}
func dtoHQLeftovers(in ops.HQLeftovers) HQLeftovers {
	out := HQLeftovers{}
	if in.Worktrees != nil {
		out.Worktrees = make([]HQWorktree, len(in.Worktrees))
		for i, v := range in.Worktrees {
			out.Worktrees[i] = dtoHQWorktree(v)
		}
	}
	out.Stashes = in.Stashes
	return out
}
func dtoHQWorktree(in ops.HQWorktree) HQWorktree {
	out := HQWorktree{}
	out.Path = in.Path
	out.Branch = in.Branch
	out.Dirty = in.Dirty
	out.Ahead = in.Ahead
	out.Base = in.Base
	out.PaneCwd = in.PaneCwd
	return out
}
func dtoHQClosed(in ops.HQClosed) HQClosed {
	out := HQClosed{}
	out.Members = in.Members
	out.Workers = in.Workers
	return out
}
func dtoActivityList(in ops.ActivityList) ActivityList {
	out := ActivityList{}
	if in.Items != nil {
		out.Items = make([]ActivityItem, len(in.Items))
		for i, v := range in.Items {
			out.Items[i] = dtoActivityItem(v)
		}
	}
	out.Running = in.Running
	out.Waiting = in.Waiting
	out.Herdr = dtoActivityNote(in.Herdr)
	return out
}
func dtoActivityItem(in ops.ActivityItem) ActivityItem {
	out := ActivityItem{}
	out.Name = in.Name
	out.Summary = dtoActivitySummary(in.Summary)
	out.Error = in.Error
	out.Place = in.Place
	out.Project = in.Project
	out.ProjectName = in.ProjectName
	out.Workspace = dtoActivitySpace(in.Workspace)
	return out
}
func dtoActivitySpace(in ops.ActivitySpace) ActivitySpace {
	out := ActivitySpace{}
	out.ID = in.ID
	out.Label = in.Label
	return out
}
func dtoActivitySummary(in ops.ActivitySummary) ActivitySummary {
	out := ActivitySummary{}
	out.Definition = in.Definition
	out.State = in.State
	if in.Since != nil {
		v := dtoActivitySince(*in.Since)
		out.Since = &v
	}
	if in.Turn != nil {
		v := dtoActivityTurn(*in.Turn)
		out.Turn = &v
	}
	return out
}
func dtoActivityTurn(in ops.ActivityTurn) ActivityTurn {
	out := ActivityTurn{}
	out.Holder = in.Holder
	out.From = in.From
	out.Why = in.Why
	out.Text = in.Text
	return out
}
func dtoActivitySince(in ops.ActivitySince) ActivitySince {
	out := ActivitySince{}
	out.Len = in.Len
	out.At = in.At
	out.Lower = in.Lower
	return out
}
func dtoActivityNote(in ops.ActivityNote) ActivityNote {
	out := ActivityNote{}
	out.OK = in.OK
	out.Text = in.Text
	return out
}
func dtoActivityDoc(in ops.ActivityDoc) ActivityDoc {
	out := ActivityDoc{}
	if in.Nodes != nil {
		out.Nodes = make([]ActivityNode, len(in.Nodes))
		for i, v := range in.Nodes {
			out.Nodes[i] = dtoActivityNode(v)
		}
	}
	if in.Hired != nil {
		out.Hired = make([]ActivityNode, len(in.Hired))
		for i, v := range in.Hired {
			out.Hired[i] = dtoActivityNode(v)
		}
	}
	out.Question = in.Question
	return out
}
func dtoActivityNode(in ops.ActivityNode) ActivityNode {
	out := ActivityNode{}
	out.Question = in.Question
	out.Name = in.Name
	out.Herdr = in.Herdr
	out.State = in.State
	out.At = in.At
	out.SinceAt = in.SinceAt
	out.Doing = in.Doing
	out.From = in.From
	return out
}
func dtoPromptRun(in ops.PromptRun) PromptRun {
	out := PromptRun{}
	out.Team = in.Team
	out.Project = in.Project
	return out
}
func dtoRecordedPrompt(in ops.RecordedPrompt) RecordedPrompt {
	out := RecordedPrompt{}
	out.Text = in.Text
	out.Command = in.Command
	out.Time = in.Time
	return out
}
func dtoPromptHistory(in ops.PromptHistory) PromptHistory {
	out := PromptHistory{}
	if in.Runs != nil {
		out.Runs = make([]PromptRun, len(in.Runs))
		for i, v := range in.Runs {
			out.Runs[i] = dtoPromptRun(v)
		}
	}
	if in.Selected != nil {
		v := dtoPromptRun(*in.Selected)
		out.Selected = &v
	}
	out.Status = in.Status
	out.Reason = in.Reason
	if in.Recorded != nil {
		v := dtoRecordedPrompt(*in.Recorded)
		out.Recorded = &v
	}
	return out
}
