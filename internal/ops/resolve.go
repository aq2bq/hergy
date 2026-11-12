// Package ops owns operations shared by the CLI and screens.
package ops

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

// ResolvedTeam is the selected identity and its definition.
type ResolvedTeam team.Team

// Resolved is one team definition read for a command: the team (the definition
// and the name it runs under), the members as starts, and where the values came
// from.
// ResolvedTeam is a definition and the identity under which it runs.
type Resolved struct {
	Project      domainplace.Project
	Team         ResolvedTeam
	Starts       []definition.Start
	Profiles     map[string]profile.Profile
	ProfilesYAML string
}

// ReadDefinition reads a team definition by name (up, show) and builds its
// members. layer forces one layer; empty is the one way, local first. runName
// names the team at run time; empty means the definition name.
func ReadDefinition(cat *catalog.Catalog, base domainplace.Project, name, runName string) (*Resolved, error) {
	def, err := cat.Definition(name, "")
	if err != nil {
		return nil, err
	}
	if runName == "" {
		runName = def.Name.String()
	}
	return BuildResolved(cat, base, def, runName)
}

// BuildResolved builds the running members of one definition under a team name.
// The prompt reads the rule files from disk: a screen's check reads its edited
// text through its own catalog, not through this.
func BuildResolved(cat *catalog.Catalog, base domainplace.Project, def *definition.Definition, runName string) (*Resolved, error) {
	profiles, profilesYAML, err := cat.Profiles()
	if err != nil {
		return nil, err
	}
	starts, err := startsOf(cat, def, runName, base, profiles, cat.Reader())
	if err != nil {
		return nil, err
	}
	model, _ := domainname.ParseTeamName(runName)
	return &Resolved{
		Project:      base,
		Team:         ResolvedTeam{Name: model.Ref(), Definition: def},
		Starts:       starts,
		Profiles:     profiles,
		ProfilesYAML: profilesYAML,
	}, nil
}

// executable is the hergy binary a member's prompt names in its hire command:
// this process's own path, or "hergy" when it cannot be read. The launch record
// keeps it, so a prompt rebuilt later names the same binary (C9).
func executable() string {
	path, err := os.Executable()
	if err != nil || path == "" {
		return "hergy"
	}
	return path
}

// startsOf builds the running members: the frame of each member, the engine its
// profile names, and the prompt hergy hands it (a projection of the definition
// and the catalog).
func startsOf(cat *catalog.Catalog, def *definition.Definition, runName string, base domainplace.Project, profiles map[string]profile.Profile, reader source.Reader) ([]definition.Start, error) {
	d, _ := domainname.ParseDefinitionName(runName)
	names := domainname.DefinitionNames(d)
	if model, err := d.Team(); err == nil {
		names = domainname.RunningNames(model.Ref())
	}
	run := prompt.Run{Team: names, Executable: executable(), Base: base, Reader: reader, Sources: cat, Display: cat.Display}
	starts := make([]definition.Start, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		// A member's role is a name, and a name that is not a role identifier
		// is a broken document: the same sentence validate prints stops the run
		// before a prompt is built (d-841e, r-7ea6).
		if err := catalog.RoleNameProblem(node.Role); err != nil {
			return nil, err
		}
		// herdr sees the member as "<team>-<member>"; the name is checked where
		// the same name is written into the prompt, so the projection and the
		// check refuse it in one place.
		herdr, err := names.MemberTarget(node.Name)
		if err != nil {
			return nil, i18n.Errorf("error.herdr_name_pattern", names.Member(node.Name))
		}
		built, err := prompt.Member(def, node, run)
		if err != nil {
			return nil, err
		}
		value, ok := profiles[node.Profile]
		if !ok {
			// A definition reads with a profile profiles.yaml does not hold, so
			// a screen can draw it and the name can be fixed (spec-teams 1.9):
			// this is where the name stops a run, since the member has no engine
			// to start with.
			return nil, i18n.Errorf("error.no_profile", fmt.Sprintf("members[%d]", i), node.Profile)
		}
		starts = append(starts, definition.Start{
			Name:     herdr,
			Short:    node.Name.Short(),
			Role:     node.Role,
			Profile:  node.Profile,
			Kind:     value.Kind,
			Args:     value.Args,
			Env:      profile.PaneEnv(value.Env, herdr.String()),
			Cwd:      node.Cwd,
			Prompt:   built.Text,
			Segments: built.Segments,
			RoleDoc:  built.Role,
			HasRole:  built.HasRole,
			Read:     built.Read,
			Workers:  workersOf(def, node.Name.String()),
			Turns:    turnsOf(def, node.Name.String()),
		})
		wireStart(&starts[len(starts)-1], runName)
	}
	return starts, nil
}

// turnsOf is who one member hands the turn to, from the graph's edges out of it
// (I13): the human or another member, in the order turns: writes them. A
// worker's edge is made by workers: and is not a turn out of the member (I12c).
func turnsOf(def *definition.Definition, member string) []string {
	var out []string
	for _, edge := range def.Graph.Out(definition.NodeID{Kind: definition.KindMember, Name: member}) {
		if edge.To.Kind != definition.KindHuman && edge.To.Kind != definition.KindMember {
			continue
		}
		out = append(out, edge.To.String())
	}
	return out
}

// workersOf is what one member may hire, from the graph's worker nodes.
func workersOf(def *definition.Definition, member string) []definition.WorkerNode {
	var out []definition.WorkerNode
	for _, worker := range def.Graph.Workers {
		if worker.From == member {
			out = append(out, worker)
		}
	}
	return out
}

// Member finds one member as a start: by its herdr name first, then by its short
// name.
func (r *Resolved) Member(name string) (*definition.Start, error) {
	for i := range r.Starts {
		if r.Starts[i].Name.String() == name || r.Starts[i].Short.String() == name {
			return &r.Starts[i], nil
		}
	}
	return nil, i18n.Errorf("error.no_member", name)
}

// SelectStarts returns every member, or only the named one.
func (r *Resolved) SelectStarts(name string) ([]definition.Start, error) {
	if name == "" {
		return r.Starts, nil
	}
	member, err := r.Member(name)
	if err != nil {
		return nil, err
	}
	return []definition.Start{*member}, nil
}

// DescriptionLine is the description as one line. It is required, so it is
// never empty for a definition that read. Only the places a person picks or
// reads a definition print it (hergy show and the screens): a team command's
// header leaves it out (d-5f55).
func (r *Resolved) DescriptionLine() string {
	line, _, _ := strings.Cut(r.Team.Definition.Description, "\n")
	return strings.TrimSpace(line)
}

// UsesProfiles reports whether any member reads a named profile, so the caller
// shows profiles.yaml only when it is actually in effect.
func (r *Resolved) UsesProfiles() bool {
	return r.ProfilesYAML != "" && (len(r.Starts) > 0 || len(r.Team.Definition.Graph.Workers) > 0)
}

// ModelTeamName is the boundary from the existing operation read model to a
// checked name. A resolved team's starts have already passed the checked member constructors.
func (r *Resolved) ModelTeamName() domainname.TeamName {
	model, _ := r.Team.Name.Model()
	return model
}

// resolveTeam finds a running team by its team name (down, status, refresh,
// reprompt, hire) and builds the team definition it was booted from. An empty
// name picks the one running team, or stops and names the candidates.
func resolveTeam(cat *catalog.Catalog, base domainplace.Project, opts *options.Options) (*Resolved, error) {
	running, err := runningTeams()
	if err != nil {
		return nil, err
	}
	var inferred domainname.Seen
	if opts.Team.String() == "" && len(running) > 1 {
		inferred = teamOfPane(running, CurrentPane())
	}
	names := make([]domainname.TeamRef, len(running))
	for i, one := range running {
		names[i] = one.Team
	}
	selected, err := opts.Team.ResolveTeam(names, inferred)
	if err != nil {
		return nil, err
	}
	chosen, _ := team.FindRunning(running, selected.String())
	return buildRunning(cat, base, chosen)
}

// teamOfPane is the running team whose member or worker runs in the pane hergy
// was typed in, or "" from any other pane. A member that types hergy tell while
// several teams run names no team: its own pane says which (r-103d).
func teamOfPane(running []team.Running, selfPane domainname.PaneID) domainname.Seen {
	if selfPane == "" {
		return domainname.Seen{}
	}
	agent, err := herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))).Agent(selfPane)
	if err != nil {
		return domainname.Seen{}
	}
	return teamOfAgent(running, agent.Name)
}

// teamOfAgent is the running team an agent's herdr name belongs to, or "".
func teamOfAgent(running []team.Running, agentName domainname.Seen) domainname.Seen {
	known := make([]domainname.Seen, len(running))
	for i, one := range running {
		known[i] = one.Team.Observation()
	}
	teamName, _, ok := domainname.Resolve(agentName, known)
	if !ok {
		return domainname.Seen{}
	}
	return teamName
}

// resolveTeamDry finds the team a hire --dry plans for without reading herdr, so
// a plan can be made for a team that is not running (r-fe31). --team names a
// team; when this project's launch record knows that name, the record says which
// definition it stood on, and a name the record does not know is taken as a team
// definition name. With no name, the one team the record holds is taken, and
// nothing else is: herdr is not read to choose.
func resolveTeamDry(cat *catalog.Catalog, base domainplace.Project, opts *options.Options) (*Resolved, error) {
	recorded := recordedDefinitions(base)
	name := opts.Team.String()
	if name == "" {
		if len(recorded) != 1 {
			return nil, dryTeamError(recorded)
		}
		name = slices.Sorted(maps.Keys(recorded))[0]
	}
	definition := name
	if one, ok := recorded[name]; ok {
		definition = one
	}
	def, err := cat.Definition(definition, "")
	if err != nil {
		return nil, err
	}
	return BuildResolved(cat, base, def, name)
}

// recordedDefinitions is the newest launch of each team name this project has
// had, from this machine's launch record: the team name and the team definition
// it stood on. It reads the record, not herdr, which is what a --dry plan has to
// stand on; a record from another project is not this plan's.
func recordedDefinitions(base domainplace.Project) map[string]string {
	out := map[string]string{}
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	records, err := log.Read()
	if err != nil {
		return out
	}
	for _, record := range records {
		if record.Team.String() == "" || record.Definition == "" || record.IsHire() {
			continue
		}
		if !record.Project.Project().IsZero() && record.Project.Project().Compare(base) != domainplace.Same {
			continue
		}
		out[record.Team.String()] = record.Definition
	}
	return out
}

// dryTeamError is the sentence a --dry with no team name stops on: it names the
// teams this project's launch record holds, so a person can pick one without
// herdr.
func dryTeamError(recorded map[string]string) error {
	if len(recorded) == 0 {
		return i18n.Errorf("error.dry_team_needed")
	}
	names := slices.Sorted(maps.Keys(recorded))
	return i18n.Errorf("error.dry_team_candidates", strings.Join(names, i18n.T("list.separator")))
}

// buildRunning resolves one running team's definition and builds the members
// under the team's run name.
func buildRunning(cat *catalog.Catalog, base domainplace.Project, running team.Running) (*Resolved, error) {
	return buildRunningIn(cat, base, running, "")
}

// buildRunningIn builds a running team from the layer its launch record names: a
// global definition started in a project whose layer holds a same-named one is
// still the global definition (d-73fc, M1).
func buildRunningIn(cat *catalog.Catalog, base domainplace.Project, running team.Running, layer string) (*Resolved, error) {
	def, err := cat.Definition(running.Definition, layer)
	if err != nil {
		return nil, err
	}
	return BuildResolved(cat, base, def, running.Team.String())
}

// runningTeams reads herdr's tabs and this machine's launch record, and returns
// the running teams.
func runningTeams() ([]team.Running, error) {
	tabs, agentsRead, err := herdrTabs()
	if err != nil {
		return nil, err
	}
	return team.RunningTeams(tabs, launchTeams(), agentsRead), nil
}

// launchTeams is this machine's launch record as the team name and the team
// definition of each launch, oldest first. It is how a running team is
// recognized (a tab is labelled with the team's name alone) and how the
// definition behind it is found (d-a72d). A record that cannot be read leaves
// the older "team:<team>@<definition>" labels as the only way a team is found.
func launchTeams() []team.Launch {
	log, err := launchlog.Open()
	if err != nil {
		return nil
	}
	records, err := log.Read()
	if err != nil {
		return nil
	}
	out := make([]team.Launch, 0, len(records))
	for _, record := range records {
		if record.IsHire() {
			continue
		}
		out = append(out, team.Launch{Team: record.Team, Definition: record.Definition, Project: record.Project.Project()})
	}
	return out
}

// herdrTabs reads the tabs herdr reports, each with the live agents in it. It is
// how a team command tells hergy's own tabs apart and finds the team definition
// behind a running team name; herdrRead is the one read behind it and behind the
// activity screen.
func herdrTabs() (tabs []team.Tab, agentsRead bool, err error) {
	tabs, _, agentsRead, err = herdrRead()
	return tabs, agentsRead, err
}

// herdrRead is one read of herdr: the tabs, each with the live agents in it, and
// the live agents themselves, with the state they are in. It is what hergy status
// reads a running team from and what the activity screen reads, so both see the
// same herdr (6 A1).
//
// A herdr that cannot list its agents still lists its tabs: the tabs an older
// hergy labelled are found from their labels, and the ones this machine recorded
// are guessed at from the record, rather than lost. agentsRead says whether the
// agent list could be read at all — with no agent names, the launch record is the
// best guess left at which tab is a team's.
func herdrRead() (tabs []team.Tab, agents []protocol.Agent, agentsRead bool, err error) {
	list, err := herdrTabList()
	if err != nil {
		return nil, nil, false, err
	}
	panes, _ := herdrPanes()
	agents, err = herdrAgents()
	if err != nil {
		return teamTabs(list, nil, panes), nil, false, nil
	}
	return teamTabs(list, agents, panes), agents, true, nil
}

// herdrPanes reads the labels hergy gave the panes, the way down finds a team
// whose members are not running (d-da92). A read that fails leaves the panes
// unknown: the live agents still say whose a tab is.
func herdrPanes() ([]protocol.Pane, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Panes()
}

// herdrSpaceList reads herdr's workspaces, with their labels and ids.
func herdrSpaceList() ([]herdr.Workspace, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Workspaces()
}

// herdrAgents reads the live agents herdr reports.
func herdrAgents() ([]protocol.Agent, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).LiveAgents()
}

// teamTabs pairs herdr's tabs with the names of the live agents and of the named
// panes in each, which is what tells hergy's own tabs apart: hergy names a
// member's agent and pane "<team>-<member>". A pane that is there without its
// agent still says the tab is the team's (d-da92).
func teamTabs(tabs []protocol.Tab, agents []protocol.Agent, panes []protocol.Pane) []team.Tab {
	inTab := map[string][]domainname.Seen{}
	for _, agent := range agents {
		if agent.Name.String() != "" && agent.TabID != "" {
			inTab[agent.TabID] = append(inTab[agent.TabID], agent.Name)
		}
	}
	panesInTab := map[string][]domainname.Seen{}
	dirsInTab := map[string][]domainplace.Seen{}
	for _, pane := range panes {
		if pane.Label.String() != "" && pane.TabID != "" {
			panesInTab[pane.TabID] = append(panesInTab[pane.TabID], pane.Label)
			dirsInTab[pane.TabID] = append(dirsInTab[pane.TabID], pane.Cwd)
		}
	}
	out := make([]team.Tab, 0, len(tabs))
	for _, tab := range tabs {
		out = append(out, team.Tab{
			ID: tab.ID, Label: tab.Label, WorkspaceID: tab.WorkspaceID,
			Agents: inTab[tab.ID], Panes: panesInTab[tab.ID], PaneDirs: dirsInTab[tab.ID],
		})
	}
	return out
}

// herdrTabList lists the tabs herdr reports, with the labels a team is found by.
func herdrTabList() ([]protocol.Tab, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).Tabs()
}

// CurrentPane is the pane ID inherited from herdr.
func CurrentPane() domainname.PaneID { return domainname.PaneID(os.Getenv("HERDR_PANE_ID")) }
