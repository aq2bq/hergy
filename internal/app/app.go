// Package app is the edge of the CLI: arguments, environment, files, output
// and exit codes. Everything it decides comes from the packages below it.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/newteam"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/spawn"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/version"
)

// reportedError marks a failure whose detail was already printed, and carries
// the exit code (1 for a failure, 130 for an interrupt).
type reportedError struct {
	err  error
	code int
}

func (e reportedError) Error() string { return e.err.Error() }

// Run executes one command line and returns the process exit code.
func Run(argv []string, stdout, stderr io.Writer) int {
	// Load the locales and check them before anything can print. The failures
	// below happen before a language is in use, so they keep a plain prefix.
	if err := i18n.Init(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	// validate must still run when the global config.yaml is broken: it is one
	// of the files validate reports. cheat reads no team settings and must
	// print even when the files are broken, so it too goes on. Every other
	// command keeps stopping here.
	tolerant := options.ScanCommand(argv) == "validate" || options.ScanCommand(argv) == "cheat"
	configured, err := team.Lang()
	if err != nil && !tolerant {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	explicit := options.ScanLang(argv)
	lang, err := i18n.Resolve(explicit, configured)
	if err != nil {
		// An unknown --lang is a command-line mistake and always stops. For
		// validate and cheat, an unknown language in config.yaml is a break
		// they tolerate: fall back to the default language and go on.
		if !tolerant || explicit != "" {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		lang = i18n.Fallback
	}
	if err := i18n.Use(lang); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	opts, err := options.Parse(argv)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if opts.Kind == options.Help {
		fmt.Fprint(stdout, opts.HelpText)
		return 0
	}
	if opts.Kind == options.Version {
		fmt.Fprintln(stdout, version.Line())
		return 0
	}
	// cheat is answered before anything touches the environment: it reads no
	// team settings and no Herdr, so it prints even when they are absent.
	if opts.Kind == options.Cheat {
		if opts.AI {
			fmt.Fprint(stdout, cheat.Machine)
		} else {
			fmt.Fprint(stdout, cheat.People())
		}
		return 0
	}

	env, err := team.NewEnv()
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if opts.Dir != "" {
		env, err = env.WithProject(opts.Dir)
		if err != nil {
			fail(stderr, err)
			return 1
		}
	}
	if err := dispatch(ctx, env, opts, stdout, stderr); err != nil {
		if reported, ok := errors.AsType[reportedError](err); ok {
			return reported.code
		}
		fail(stderr, err)
		return 1
	}
	return 0
}

// fail writes one error line in the language in use.
func fail(stderr io.Writer, err error) {
	fmt.Fprintln(stderr, i18n.Tf("error.prefix", err))
}

func dispatch(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	switch opts.Kind {
	case options.Ls:
		return runLs(env, stdout)
	case options.Show:
		return runShow(env, opts, stdout)
	case options.New:
		return runNew(env, opts, stdout)
	case options.Validate:
		return runValidate(env, stdout)
	case options.Serve:
		return runServe(ctx, env, opts, stdout, stderr)
	case options.Up:
		return runUp(ctx, env, opts, stdout, stderr)
	case options.Down:
		return runDown(ctx, env, opts, stdout, stderr)
	case options.Status:
		return runStatus(env, opts, stdout)
	case options.Refresh:
		return runRefresh(ctx, env, opts, stdout, stderr)
	case options.Reprompt:
		return runReprompt(ctx, env, opts, stdout, stderr)
	case options.Focus:
		return runFocus(env, opts, stdout)
	case options.Nudge:
		return runNudge(env, opts, stdout)
	case options.Stop:
		return runStop(env, opts, stdout)
	case options.Tell:
		return runTell(env, opts, stdout)
	case options.Hire:
		return runHire(ctx, env, opts, stdout, stderr)
	default:
		return i18n.Errorf("error.unhandled_command")
	}
}

// resolved is one team definition read for a command: the team (the definition
// and the name it runs under), the members as starts, and where the values came
// from.
type resolved struct {
	team         team.Team
	starts       []definition.Start
	profiles     map[string]profile.Profile
	profilesYAML string
}

// readDefinition reads a team definition by name (up, show) and builds its
// members. layer forces one layer; empty is the one way, local first. runName
// names the team at run time; empty means the definition name.
func readDefinition(cat *catalog.Catalog, base, name, runName string) (*resolved, error) {
	def, err := cat.Definition(name, "")
	if err != nil {
		return nil, err
	}
	if runName == "" {
		runName = def.Name
	}
	return buildResolved(cat, source.Disk{}, base, def, runName)
}

// buildResolved builds the running members of one definition under a team name.
// reader is what the prompt reads the rule files through; a screen's check hands
// an edited file in place of the one on disk.
func buildResolved(cat *catalog.Catalog, reader source.Reader, base string, def *definition.Definition, runName string) (*resolved, error) {
	profiles, profilesYAML, err := cat.Profiles()
	if err != nil {
		return nil, err
	}
	starts, err := startsOf(cat, def, runName, base, profiles, reader)
	if err != nil {
		return nil, err
	}
	return &resolved{
		team:         team.Team{Name: runName, Definition: def},
		starts:       starts,
		profiles:     profiles,
		profilesYAML: profilesYAML,
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
func startsOf(cat *catalog.Catalog, def *definition.Definition, runName, base string, profiles map[string]profile.Profile, reader source.Reader) ([]definition.Start, error) {
	run := prompt.Run{Team: runName, Executable: executable(), Base: base, Reader: reader, Sources: cat, Display: cat.Display}
	starts := make([]definition.Start, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		// herdr sees the member as "<team>-<member>"; the name is checked where
		// the same name is written into the prompt, so the projection and the
		// check refuse it in one place.
		herdr, err := prompt.MemberName(runName, node)
		if err != nil {
			return nil, err
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
			Name:    herdr,
			Short:   node.Name,
			Role:    node.Role,
			Profile: node.Profile,
			Kind:    value.Kind,
			Args:    value.Args,
			Env:     profile.PaneEnv(value.Env, herdr),
			Cwd:     node.Cwd,
			Prompt:  built.Text,
			RoleDoc: built.Role,
			HasRole: built.HasRole,
			Read:    built.Read,
			Workers: workersOf(def, node.Name),
			Turns:   turnsOf(def, node.Name),
		})
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

// member finds one member as a start: by its herdr name first, then by its short
// name.
func (r *resolved) member(name string) (*definition.Start, error) {
	for i := range r.starts {
		if r.starts[i].Name == name || r.starts[i].Short == name {
			return &r.starts[i], nil
		}
	}
	return nil, i18n.Errorf("error.no_member", name)
}

// selectStarts returns every member, or only the named one.
func (r *resolved) selectStarts(name string) ([]definition.Start, error) {
	if name == "" {
		return r.starts, nil
	}
	member, err := r.member(name)
	if err != nil {
		return nil, err
	}
	return []definition.Start{*member}, nil
}

// missionLine is the mission as one line. It is required, so it is never empty
// for a definition that read.
func (r *resolved) missionLine() string {
	line, _, _ := strings.Cut(r.team.Definition.Mission, "\n")
	return strings.TrimSpace(line)
}

// usesProfiles reports whether any member reads a named profile, so the caller
// shows profiles.yaml only when it is actually in effect.
func (r *resolved) usesProfiles() bool {
	return r.profilesYAML != "" && (len(r.starts) > 0 || len(r.team.Definition.Graph.Workers) > 0)
}

// resolveTeam finds a running team by its team name (down, status, refresh,
// reprompt, hire) and builds the team definition it was booted from. An empty
// name picks the one running team, or stops and names the candidates.
func resolveTeam(cat *catalog.Catalog, base string, opts *options.Options) (*resolved, error) {
	running, err := runningTeams()
	if err != nil {
		return nil, err
	}
	chosen, err := pickTeam(running, opts.Team)
	if err != nil {
		return nil, err
	}
	return buildRunning(cat, base, chosen)
}

// resolveTeamDry finds the team a hire --dry plans for without reading herdr, so
// a plan can be made for a team that is not running (r-fe31). --team names a
// team; when this project's launch record knows that name, the record says which
// definition it stood on, and a name the record does not know is taken as a team
// definition name. With no name, the one team the record holds is taken, and
// nothing else is: herdr is not read to choose.
func resolveTeamDry(cat *catalog.Catalog, base string, opts *options.Options) (*resolved, error) {
	recorded := recordedDefinitions(base)
	name := opts.Team
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
	return buildRunning(cat, base, team.Running{Team: name, Definition: definition})
}

// recordedDefinitions is the newest launch of each team name this project has
// had, from this machine's launch record: the team name and the team definition
// it stood on. It reads the record, not herdr, which is what a --dry plan has to
// stand on; a record from another project is not this plan's.
func recordedDefinitions(base string) map[string]string {
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
		if record.Team == "" || record.Definition == "" {
			continue
		}
		if record.Project != "" && !samePath(record.Project, base) {
			continue
		}
		out[record.Team] = record.Definition
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
func buildRunning(cat *catalog.Catalog, base string, running team.Running) (*resolved, error) {
	def, err := cat.Definition(running.Definition, "")
	if err != nil {
		return nil, err
	}
	return buildResolved(cat, source.Disk{}, base, def, running.Team)
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
		out = append(out, team.Launch{Team: record.Team, Definition: record.Definition})
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
	agents, err = herdrAgents()
	if err != nil {
		return teamTabs(list, nil), nil, false, nil
	}
	return teamTabs(list, agents), agents, true, nil
}

// herdrAgents reads the live agents herdr reports.
func herdrAgents() ([]protocol.Agent, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return herdr.New(port).LiveAgents()
}

// teamTabs pairs herdr's tabs with the names of the live agents in each, which
// is what tells hergy's own tabs apart: hergy names a member's agent
// "<team>-<member>".
func teamTabs(tabs []protocol.Tab, agents []protocol.Agent) []team.Tab {
	inTab := map[string][]string{}
	for _, agent := range agents {
		if agent.Name != "" && agent.TabID != "" {
			inTab[agent.TabID] = append(inTab[agent.TabID], agent.Name)
		}
	}
	out := make([]team.Tab, 0, len(tabs))
	for _, tab := range tabs {
		out = append(out, team.Tab{ID: tab.ID, Label: tab.Label, Agents: inTab[tab.ID]})
	}
	return out
}

// pickTeam is the omitted-argument rule for a team command: the one running
// team, or a stop that names the candidates.
func pickTeam(running []team.Running, name string) (team.Running, error) {
	if name != "" {
		if chosen, ok := team.FindRunning(running, name); ok {
			return chosen, nil
		}
		return team.Running{}, teamNotRunning(name)
	}
	switch len(running) {
	case 1:
		return running[0], nil
	case 0:
		return team.Running{}, i18n.Errorf("error.no_running_teams")
	default:
		return team.Running{}, runningCandidates(running)
	}
}

// teamNotRunning is the error for a team name with no tab.
func teamNotRunning(name string) error {
	var b strings.Builder
	fmt.Fprint(&b, i18n.Tf("error.team_not_running", name))
	fmt.Fprintf(&b, "\n%s", i18n.Tf("next", "status"))
	return errors.New(b.String())
}

// runningCandidates lists the teams that are running and stops.
func runningCandidates(running []team.Running) error {
	names := team.RunningNames(running)
	var b strings.Builder
	fmt.Fprint(&b, i18n.Tf("error.need_running_team", strings.Join(names, i18n.T("list.separator"))))
	fmt.Fprintf(&b, "\n%s", i18n.Tf("next", "status"))
	return errors.New(b.String())
}

func runLs(env team.Env, stdout io.Writer) error {
	// The running teams, grouped by the team definition they were booted from.
	// Outside herdr the tabs cannot be read; then only the definitions show.
	running := map[string][]string{}
	if tabs, agentsRead, err := herdrTabs(); err == nil {
		for _, team := range team.RunningTeams(tabs, launchTeams(), agentsRead) {
			running[team.Definition] = append(running[team.Definition], team.Team)
		}
	}
	shown := 0
	for _, row := range env.Catalog(source.Disk{}).Definitions() {
		var path, other string
		if row.Local {
			path = row.LocalPath
			if row.Global {
				other = row.GlobalPath
			}
		} else {
			path = row.GlobalPath
		}
		shown++
		field(stdout, label("definition"), row.Name)
		indentField(stdout, 1, label("path"), env.Display(path))
		if other != "" {
			indentField(stdout, 1, label("note"), i18n.Tf("note.elsewhere", env.Display(other)))
		}
		if teams := running[row.Name]; len(teams) > 0 {
			indentField(stdout, 1, label("running"), strings.Join(teams, i18n.T("list.separator")))
		}
	}
	if shown == 0 {
		field(stdout, label("definition"), i18n.T("value.none"))
	}
	return nil
}

func runShow(env team.Env, opts *options.Options, stdout io.Writer) error {
	cat := env.Catalog(source.Disk{})
	r, err := readDefinition(cat, env.ProjectRoot, opts.Team, opts.As)
	if err != nil {
		return err
	}
	printHeader(stdout, r, env, false)
	docs := agreementDocs(r)
	if len(docs) == 0 {
		field(stdout, label("document"), i18n.T("value.none"))
	}
	for _, doc := range docs {
		field(stdout, label("document"), env.Display(doc.Path))
	}
	for i := range r.starts {
		printMember(stdout, r, &r.starts[i], env, false)
	}
	return nil
}

// agreementDocs is what every member of a definition reads besides its own role
// document: the team's own agreement (段 4). It is read off the built prompts, so
// the list a person sees cannot disagree with what the members were handed. A
// document two members read stands once.
func agreementDocs(r *resolved) []source.Doc {
	var docs []source.Doc
	seen := map[string]bool{}
	for i := range r.starts {
		for _, doc := range r.starts[i].Read {
			if seen[doc.Path] {
				continue
			}
			seen[doc.Path] = true
			docs = append(docs, doc)
		}
	}
	return docs
}

// runNew writes a starter team's files. It never overwrites: an existing team
// directory stops it before anything is written, and a role document that is
// already in the layer is left as it is. The files themselves come from the
// embedded scaffold for the language in use.
func runNew(env team.Env, opts *options.Options, stdout io.Writer) error {
	name := opts.Team
	if !scaffold.NamePattern.MatchString(name) {
		return i18n.Errorf("error.new_name_pattern", name)
	}
	// hergy new writes where the person is (K2): inside a git repository that
	// is the project's own layer, and outside one the global layer. The place
	// comes from the catalog (C13), so a screen that makes a role document or
	// adds a profile writes in the same one.
	layer := source.LayerGlobal
	if env.InRepo() {
		layer = source.LayerLocal
	}
	cat := env.Catalog(source.Disk{})
	dir := cat.NewPlace(catalog.NewTeam, name)
	if _, err := os.Stat(dir); err == nil {
		return i18n.Errorf("error.new_exists", name, env.Display(dir))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := scaffold.Team(i18n.Language())
	if err != nil {
		return err
	}
	// The folder, the layer's role documents a starter team reads, and the
	// starter profile go in one write, which hergy new and the team screen both
	// call (C8). A role document is the layer's, not the team's (K5), so it goes
	// into the layer's roles/ directory and one that is already there is left
	// alone.
	written, err := newteam.Write(newteam.In{
		Catalog: cat, Layer: layer, Name: name, Lang: i18n.Language(),
		Files: files, Starter: true,
	})
	if err != nil {
		return err
	}
	configPath, madeConfig, err := env.EnsureGlobalConfig(i18n.Language())
	if err != nil {
		return err
	}
	field(stdout, label("definition"), name)
	field(stdout, label("dir"), env.Display(dir))
	for _, path := range written.Files {
		field(stdout, label("file"), env.Display(path))
	}
	for _, path := range written.Roles {
		field(stdout, label("role_file"), env.Display(path))
	}
	for _, path := range written.Kept {
		field(stdout, label("note"), i18n.Tf("new.role_kept", env.Display(path)))
	}
	if written.Profile != "" {
		field(stdout, label("profile"), scaffold.ProfileName)
		field(stdout, label("file"), env.Display(written.Profile))
	}
	if madeConfig {
		field(stdout, label("config"), env.Display(configPath))
	}
	printNext(stdout, "up "+name+" --dry")
	return nil
}

// effectiveLayer is the place a definition's name resolves to: the project's
// own when both layers hold it, the global one otherwise (K1).
func effectiveLayer(row catalog.Listing) string {
	if row.Local {
		return source.LayerLocal
	}
	return source.LayerGlobal
}

// teamFindingsFor is what validate prints for one place's definition. The
// effective definition prints every finding. A hidden one prints only what
// stops it and the line that says it is not used: a warning about a guide
// nobody reads would bury the warnings that can be fixed (T13 R3).
func teamFindingsFor(row catalog.Listing, layer string, findings []check.Finding) []check.Finding {
	if layer == effectiveLayer(row) {
		return findings
	}
	out := make([]check.Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Stops || finding.Kind == check.KindHidden {
			out = append(out, finding)
		}
	}
	return out
}

// effectiveTeamSize is one definition's size line for the list at the end of
// validate: the effective place's file, once per name. A definition whose check
// stops still has a size when its team.yaml reads (a missing profile does not
// hide it), and the hidden place of a name is not listed (T13 R1).
func effectiveTeamSize(cat *catalog.Catalog, env team.Env, row catalog.Listing, layer, path string) (teamSize, bool) {
	if layer != effectiveLayer(row) {
		return teamSize{}, false
	}
	def, err := cat.Definition(row.Name, layer)
	if err != nil {
		return teamSize{}, false
	}
	return teamSize{name: env.Display(filepath.Dir(path)), chars: check.TeamSizeChars(cat, def)}, true
}

// validateRoleFile checks one role document as its own unit and returns what
// stops it, its size line when it is the effective document of its name, and
// whether that line is the list's. A document of a name that the other place
// holds is still checked (the screen can open that place), but it is not in the
// size list: the list names the effective document once (T13 R1).
func validateRoleFile(stdout io.Writer, env team.Env, cat *catalog.Catalog, layer, path string) (int, roleSize, bool) {
	name := strings.TrimSuffix(filepath.Base(path), ".yaml")
	text, err := cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		reportFile(stdout, env, path, err)
		return 1, roleSize{}, false
	}
	doc := check.Role(check.RoleInput{Catalog: cat, Place: layer, Name: name, Text: text})
	effective, ok := cat.Role(name)
	if !ok || effective.Path != path {
		// A document of a name the project also holds is not used: only what
		// stops it and the line that says so, not warnings about a guide nobody
		// reads (T13 R4).
		problems := make([]check.RoleProblem, 0, len(doc.Problems)+1)
		for _, problem := range doc.Problems {
			if problem.Sev == rules.SevError {
				problems = append(problems, problem)
			}
		}
		problems = append(problems, check.RoleProblem{
			Sev: rules.SevWarning, Section: -1, Rule: -1,
			Text: i18n.Tf("validate.role_hidden", name),
		})
		return printRoleFindings(stdout, env, path, problems), roleSize{}, false
	}
	return printRoleFindings(stdout, env, path, doc.Problems),
		roleSize{name: env.Display(path), chars: doc.Markdown.Chars}, true
}

// runValidate reads every settings file the way up does, shows one line per
// file, and fails when any of them is broken. It never touches Herdr. After the
// files, it counts what the rules carry: where they came from, why they are
// there, and whether anything shows they were kept. That count is not a break:
// it changes no exit code.
func runValidate(env team.Env, stdout io.Writer) error {
	cat := env.Catalog(source.Disk{})
	checked, problems := 0, 0
	tally := &rules.Tally{}
	var teams []teamSize
	var roles []roleSize
	seenFiles := map[string]bool{}
	ledger := &ledgerCheck{env: env}
	report := func(path string, err error) {
		checked++
		reportFile(stdout, env, path, err)
		if err != nil {
			problems++
		}
	}

	if path := filepath.Join(env.GlobalDir, "config.yaml"); isFile(path) {
		report(path, validateConfig(path))
	}
	for _, dir := range []string{env.GlobalDir, env.LocalDir} {
		if path := filepath.Join(dir, "profiles.yaml"); isFile(path) {
			reportProfileFile(stdout, env, path, report)
		}
	}
	for _, doc := range cat.SharedAgreements() {
		report(doc.Path, sharedAgreementError(cat, []source.Doc{doc}))
	}
	for _, layer := range []string{source.LayerGlobal, source.LayerLocal} {
		reportOldCommon(stdout, env, cat.LayerDir(layer))
		for _, row := range cat.Definitions() {
			path := listingPath(row, layer)
			if path == "" {
				continue
			}
			checked++
			// The team definition's own check: its team.yaml, its rules and the
			// documents its members read together. A role document's own
			// problems are not here; they are the role definition's (d-a575).
			findings := teamFindingsFor(row, layer, check.Definition(check.Input{Catalog: cat, Name: row.Name, Layer: layer}))
			stops := printFindings(stdout, env, path, findings)
			// The size belongs to the definition the screen's list shows: the
			// effective place, once per name, even when the check stops it
			// (T13 R1).
			if size, ok := effectiveTeamSize(cat, env, row, layer, path); ok {
				teams = append(teams, size)
			}
			if stops > 0 {
				problems += stops
				continue
			}
			// The definition reads: the tally is counted from it.
			r, err := validateTeam(cat, env.ProjectRoot, row.Name, layer)
			if err != nil {
				continue
			}
			printLedger(stdout, r.team.Definition, ledger)
			collectRules(tally, seenFiles, r)
		}
	}
	// Every role document is its own unit, checked once however many teams use
	// it (d-a575): the section keeps a role document's own warnings out of the
	// team definitions' sections.
	for _, layer := range []string{source.LayerGlobal, source.LayerLocal} {
		dir := filepath.Join(cat.LayerDir(layer), "roles")
		matches, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
		slices.Sort(matches)
		for _, path := range matches {
			checked++
			stops, size, ok := validateRoleFile(stdout, env, cat, layer, path)
			if stops > 0 {
				problems += stops
			}
			if ok {
				roles = append(roles, size)
			}
		}
	}

	field(stdout, label("checked"), strconv.Itoa(checked))
	field(stdout, label("problems"), strconv.Itoa(problems))
	printTally(stdout, env, tally)
	printSize(stdout, teams, roles)
	if problems > 0 {
		return reportedError{err: errors.New("validate found problems"), code: 1}
	}
	return nil
}

// reportFile writes one settings file's own line: where it is, as a person
// writes it, whether it reads, and why it does not. A team definition's findings
// come from internal/check; this is the config.yaml and profiles.yaml line.
func reportFile(stdout io.Writer, env team.Env, path string, err error) {
	field(stdout, label("file"), env.Display(path))
	if err == nil {
		indentField(stdout, 1, label("result"), i18n.T("value.ok"))
		return
	}
	indentField(stdout, 1, label("result"), i18n.T("value.broken"))
	indentField(stdout, 1, label("detail"), breakLine(env, path, err))
}

// printFindings writes one definition's findings under its file line, the way
// hergy validate has always written them: the result, then the sentences that
// stop it, then the warnings. It returns how many stop the definition, which is
// what the problems count adds.
func printFindings(stdout io.Writer, env team.Env, path string, findings []check.Finding) int {
	field(stdout, label("file"), env.Display(path))
	printed := cliFindings(findings)
	stops := 0
	for _, finding := range printed {
		if finding.Stops {
			stops++
		}
	}
	if stops > 0 {
		indentField(stdout, 1, label("result"), i18n.T("value.broken"))
	} else {
		indentField(stdout, 1, label("result"), i18n.T("value.ok"))
	}
	for _, finding := range printed {
		if finding.Stops {
			indentField(stdout, 1, label("detail"), cliFindingText(finding))
			continue
		}
		indentField(stdout, 1, label("warning"), cliFindingText(finding))
	}
	return stops
}

// cliFindingText is one finding as the command line writes it. A finding that
// comes from a role document's rule names that document before the sentence
// ("writer.yaml の規則 WRITE-7 が…"), so a person reading validate knows which
// document to fix (T13 R2). A screen already stands the file name beside the
// rule, so its sentences are the check's own.
func cliFindingText(finding check.Finding) string {
	if finding.RoleDoc == "" {
		return finding.Text
	}
	return i18n.Tf("validate.rule_doc", filepath.Base(finding.RoleDoc), finding.Text)
}

// printRoleFindings writes one role document's own check under its file line, in
// the section for role definitions: the result, then what stops it, then the
// warnings (d-a575). The sentences are the role check's own and name no team:
// a problem a team's turns cause belongs to that team definition's section.
func printRoleFindings(stdout io.Writer, env team.Env, path string, problems []check.RoleProblem) int {
	field(stdout, label("file"), env.Display(path))
	stops := 0
	for _, problem := range problems {
		if problem.Sev == rules.SevError {
			stops++
		}
	}
	if stops > 0 {
		indentField(stdout, 1, label("result"), i18n.T("value.broken"))
	} else {
		indentField(stdout, 1, label("result"), i18n.T("value.ok"))
	}
	for _, problem := range problems {
		if problem.Sev == rules.SevError {
			indentField(stdout, 1, label("detail"), problem.Text)
			continue
		}
		indentField(stdout, 1, label("warning"), problem.Text)
	}
	return stops
}

// cliFindings is what hergy validate prints of what a check found. It is
// everything, except that a prompt that cannot be built shows its first stop
// alone and the warnings about the files hergy no longer reads (N1): the command
// line's own output stays as it was, while a screen shows every finding a
// definition that reads gets.
func cliFindings(findings []check.Finding) []check.Finding {
	var first *check.Finding
	for i := range findings {
		if findings[i].Kind == check.KindPrompt && findings[i].Stops {
			first = &findings[i]
			break
		}
	}
	if first == nil {
		return findings
	}
	printed := []check.Finding{*first}
	for _, finding := range findings {
		if finding.Kind == check.KindOldFile {
			printed = append(printed, finding)
		}
	}
	return printed
}

// reportOldCommon warns about the layer's common/ directory, where the rules
// every team shared used to stand as a folder of files. Those rules go into a
// team's own agreement or a role document now (段 4, spec/32 D1). It is asked
// once for the layer rather than once per team, because the directory is the
// layer's, and it moves no exit code.
func reportOldCommon(w io.Writer, env team.Env, dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "common", "*.yaml"))
	if len(matches) == 0 {
		return
	}
	field(w, label("warning"), i18n.Tf("validate.old_common",
		env.Display(filepath.Join(dir, "common"))))
}

// reportProfileFile writes one profiles.yaml's line and its warnings, from the
// same reading the WebUI shows: ReadAll gathers every problem in one pass, and
// the command line keeps its own output — the first thing that stops the file
// under the result, and the name warnings under it.
func reportProfileFile(stdout io.Writer, env team.Env, path string, report func(string, error)) {
	text, err := os.ReadFile(path)
	if err != nil {
		report(path, err)
		return
	}
	_, problems := profile.ReadAll(string(text), env.Display(path))
	var stop error
	for _, problem := range problems {
		if problem.Sev == profile.Err {
			stop = errors.New(problem.Text)
			break
		}
	}
	report(path, stop)
	if stop != nil {
		return
	}
	warnings := make([]string, 0, len(problems))
	for _, problem := range problems {
		if problem.Sev == profile.Warn {
			warnings = append(warnings, problem.Text)
		}
	}
	slices.Sort(warnings)
	for _, warning := range warnings {
		indentField(stdout, 1, label("warning"), warning)
	}
}

// collectRules adds the rule files the members of one team read to the tally.
// seen is shared across every team, so a file that two team definitions read is
// counted once. Within one team, a member's role file comes first, then the
// documents it reads.
func collectRules(tally *rules.Tally, seen map[string]bool, r *resolved) {
	for i := range r.starts {
		member := &r.starts[i]
		if member.HasRole {
			countRules(tally, seen, member.RoleDoc)
		}
		for _, doc := range member.Read {
			countRules(tally, seen, doc)
		}
	}
}

// countRules reads and counts one rule file, once. A read that fails here is
// left out: the team this file belongs to was built a moment ago with the same
// reading, so the failure was already reported as a break.
func countRules(tally *rules.Tally, seen map[string]bool, doc source.Doc) {
	if seen[doc.Path] {
		return
	}
	seen[doc.Path] = true
	text, err := os.ReadFile(doc.Path)
	if err != nil {
		return
	}
	parsed, err := rules.Parse(string(text), doc.Path)
	if err != nil {
		return
	}
	tally.Add(parsed)
}

// printTally is the count of what the rules carry, after the per-file lines. It
// prints nothing when no rule file was read (no team definition, or every one
// broken). The words are the locales'; the numbers, the ids and the paths are
// not translated.
func printTally(w io.Writer, env team.Env, tally *rules.Tally) {
	if tally.Files == 0 {
		return
	}
	line := func(text string) { fmt.Fprintf(w, "%s\n", text) }
	line(i18n.Tf("validate.rules_total", tally.Total, tally.Files))
	line(i18n.Tf("validate.rules_origin", tally.WithOrigin,
		tally.Incident, tally.Instruction, tally.Derived, tally.Total-tally.WithOrigin))
	line(i18n.Tf("validate.rules_reason", tally.WithWhy, tally.Total-tally.WithWhy))
	line(i18n.Tf("validate.rules_check", tally.WithCheck,
		tally.Report, tally.Tests, tally.Total-tally.WithCheck))
	for _, missing := range tally.Missing {
		line(i18n.Tf("validate.rules_missing_origin",
			env.Display(missing.Path), strings.Join(missing.IDs, i18n.T("list.separator"))))
	}
}

// teamSize is one team definition's own size (d-00a9): the prompt part of its
// rules and the words of its templates. The per-member totals are gone (d-b87a);
// the unit a person fixes is the definition.
type teamSize struct {
	name  string
	chars int
}

// roleSize is one role document's own size: the part of it that goes into the
// prompt.
type roleSize struct {
	name  string
	chars int
}

// printSize writes what each definition holds, after the rules count: the size
// of the team definition's rules and templates, and of each role document, with
// the provisional limit beside each (d-b87a). A number over its limit is already
// a warning under the definition it belongs to, so this section does not say it
// twice. The numbers move no exit code: a large guide is one to look at, not a
// broken one.
func printSize(w io.Writer, teams []teamSize, roles []roleSize) {
	if len(teams) == 0 && len(roles) == 0 {
		return
	}
	line := func(text string) { fmt.Fprintf(w, "%s\n", text) }
	line(i18n.T("validate.size_heading"))
	for _, team := range teams {
		line(i18n.Tf("validate.size_team", team.name, team.chars, check.TeamCharLimit))
	}
	for _, role := range roles {
		line(i18n.Tf("validate.size_role", role.name, role.chars, check.RoleCharLimit))
	}
	line(i18n.T("validate.size_note"))
}

// validateConfig reads the global config.yaml the way up does, and holds its
// lang to the same check every other command applies when it picks a language.
func validateConfig(path string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lang, err := team.ParseConfig(string(text), path)
	if err != nil {
		return err
	}
	_, err = i18n.Resolve("", lang)
	return err
}

// validateTeam builds one team from one layer the way up does.
func validateTeam(cat *catalog.Catalog, base, name, layer string) (*resolved, error) {
	def, err := cat.Definition(name, layer)
	if err != nil {
		return nil, err
	}
	return buildResolved(cat, source.Disk{}, base, def, def.Name)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// sharedAgreementStops is the error a command that builds a prompt stops with
// when a layer still holds agreement.yaml at its top, the file every team used
// to share (d-31ad): hergy no longer reads it, and the rules it held would
// silently stop reaching the members. It is nil when no layer holds one.
func sharedAgreementStops(cat *catalog.Catalog) error {
	docs := cat.SharedAgreements()
	if len(docs) == 0 {
		return nil
	}
	return sharedAgreementError(cat, docs)
}

// sharedAgreementError is the sentence for one or more leftover shared
// agreements: the paths, and the two places a rule goes now, in the layer the
// first file was found in.
func sharedAgreementError(cat *catalog.Catalog, docs []source.Doc) error {
	return errors.New(check.SharedAgreementText(cat, docs))
}

func runUp(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	r, err := readDefinition(cat, env.ProjectRoot, opts.Team, opts.As)
	if err != nil {
		return err
	}
	if opts.Dry {
		return dryPlan(stdout, env, r, "boot", r.starts, opts, "up "+r.team.Definition.Name+asSuffix(opts))
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := newLauncher(r, opts.Timeout, stdout)
	if err := runLaunch(ctx, stderr, l, l.Boot); err != nil {
		return err
	}
	recordLaunch(stderr, env, r, launchlog.CommandUp, r.starts)
	return nil
}

// runDown closes the panes of a live team's members. --dry prints what would
// close and reads the live agents, but sends no close and needs no Herdr pane.
func runDown(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	running, err := runningTeams()
	if err != nil {
		return err
	}
	chosen, err := pickTeam(running, opts.Team)
	if err != nil {
		return err
	}
	r, buildErr := buildRunning(cat, env.ProjectRoot, chosen)
	if buildErr != nil {
		// The definition is what a prompt is built from; closing a team needs no
		// prompt, so a team whose definition cannot be read is still put away.
		return downByName(ctx, chosen, buildErr, opts, stdout, stderr)
	}
	// The pane hergy runs in is the member that must be closed last.
	selfPane := os.Getenv("HERDR_PANE_ID")
	if opts.Dry {
		printHeader(stdout, r, env, true)
		closed, err := newLauncher(r, opts.Timeout, stdout).Down(r.starts, selfPane, true)
		if err != nil {
			return err
		}
		// A dry run that would close nothing has no next step to name.
		if closed > 0 {
			printNext(stdout, "down "+r.team.Name)
		}
		return nil
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := newLauncher(r, opts.Timeout, stdout)
	return runOperation(ctx, stderr, nil, func() error {
		_, err := l.Down(r.starts, selfPane, false)
		return err
	})
}

// downByName puts a running team away when the team definition behind it cannot
// be read (a boot that failed once its tab was made left no launch record to read
// the definition from). The panes are the ones hergy started, named after the
// team, so closing them needs no definition.
func downByName(ctx context.Context, chosen team.Running, reason error, opts *options.Options, stdout, stderr io.Writer) error {
	field(stdout, label("team"), startedFrom(chosen.Team, chosen.Definition))
	// The reason, on one line, without the "next:" step the resolution error
	// suggests: the step here is the down itself.
	detail := reason.Error()
	if i := strings.Index(detail, "\nnext:"); i >= 0 {
		detail = detail[:i]
	}
	indentField(stdout, 1, label("detail"), strings.Join(strings.Fields(detail), " "))
	indentField(stdout, 1, label("note"), i18n.T("down.closes_by_name"))
	tabs, _, err := herdrTabs()
	if err != nil {
		return err
	}
	selfPane := os.Getenv("HERDR_PANE_ID")
	l := newLauncher(&resolved{team: team.Team{Name: chosen.Team}}, opts.Timeout, stdout)
	if opts.Dry {
		closed, err := l.DownByName(team.MemberAgents(tabs, chosen.Team), selfPane, true)
		if err != nil {
			return err
		}
		// A dry run that would close nothing has no next step to name.
		if closed > 0 {
			printNext(stdout, "down "+chosen.Team)
		}
		return nil
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	return runOperation(ctx, stderr, nil, func() error {
		_, err := l.DownByName(team.MemberAgents(tabs, chosen.Team), selfPane, false)
		return err
	})
}

// runStatus reports each member's and worker's live state, one "name: state"
// line each, then the judgement: who is waiting on the human, or that nobody is
// working. It only reads Herdr, so it runs outside a herdr-managed pane too.
// With no team name it lists every running team, one block each.
func runStatus(env team.Env, opts *options.Options, stdout io.Writer) error {
	if opts.Team == "" {
		return runStatusList(env, opts, stdout)
	}
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return statusOfARunningTeam(opts, err)
	}
	rows, err := newLauncher(r, opts.Timeout, stdout).Status(r.starts)
	if err != nil {
		return err
	}
	markScreens(rows)
	if opts.JSON {
		return writeStatusJSON(stdout, statusJSONTeam{
			Team: r.team.Name, Definition: r.team.Definition.Name, Members: statusJSONRows(rows),
		})
	}
	printHeader(stdout, r, env, true)
	for _, row := range rows {
		field(stdout, row.Name, row.Text())
	}
	return printStatusJudgement(stdout, rows, "")
}

// statusOfARunningTeam keeps the reason a running team's definition cannot be
// read, and adds the one thing that can still be done about it: down closes the
// team's panes by name, so a team is not stuck when its definition is out of
// reach (r-e494's follow-up).
func statusOfARunningTeam(opts *options.Options, reason error) error {
	running, err := runningTeams()
	if err != nil {
		return reason
	}
	if _, ok := team.FindRunning(running, opts.Team); !ok {
		return reason
	}
	return errors.New(reason.Error() + "\n" + i18n.Tf("status.can_down", opts.Team))
}

// runStatusList is status with no team name: every running team, with its team
// definition name, its counts, and the same judgement.
func runStatusList(env team.Env, opts *options.Options, stdout io.Writer) error {
	tabs, agents, agentsRead, err := herdrRead()
	if err != nil {
		return err
	}
	running := readRunningTeams(env, herdrNow{tabs: tabs, agents: agents, agentsRead: agentsRead})
	if len(running) == 0 {
		if opts.JSON {
			return printStatusJSONList(stdout, nil)
		}
		fmt.Fprintln(stdout, i18n.T("status.none"))
		return nil
	}
	var teams []statusJSONTeam
	for i, entry := range running {
		if !opts.JSON && i > 0 {
			fmt.Fprintln(stdout)
		}
		r := entry.Built
		if r == nil {
			// A team booted from another project's definition is still listed,
			// so one such team does not hide the rest. A definition that is here
			// but does not read says why, with its file's path; only a
			// definition that is not here says "another project".
			if opts.JSON {
				teams = append(teams, statusJSONTeam{Team: entry.Name, Definition: entry.Definition, Error: entry.Broken})
				continue
			}
			field(stdout, label("team"), startedFrom(entry.Name, entry.Definition))
			if entry.Elsewhere {
				fmt.Fprintf(stdout, "  %s\n", i18n.T("status.definition_elsewhere"))
			} else {
				indentField(stdout, 1, label("detail"), entry.Broken)
			}
			continue
		}
		rows, err := newLauncher(r, opts.Timeout, stdout).Status(r.starts)
		if err != nil {
			return err
		}
		markScreens(rows)
		if opts.JSON {
			teams = append(teams, statusJSONTeam{
				Team: r.team.Name, Definition: r.team.Definition.Name, Members: statusJSONRows(rows),
			})
			continue
		}
		field(stdout, label("team"), startedFrom(r.team.Name, r.team.Definition.Name))
		membersCount, workers := status.Counts(rows)
		indentField(stdout, 1, label("members"), strconv.Itoa(membersCount))
		indentField(stdout, 1, label("workers"), strconv.Itoa(workers))
		if err := printStatusJudgement(stdout, rows, "  "); err != nil {
			return err
		}
	}
	if opts.JSON {
		return printStatusJSONList(stdout, teams)
	}
	return nil
}

// statusRowJSON is one status row as JSON: the state hergy status prints, and
// how long a working member's screen has stood (absent when hergy has not seen
// it before, or it changed).
type statusRowJSON struct {
	Name             string `json:"name"`
	State            string `json:"state"`
	UnchangedMinutes *int   `json:"unchanged_minutes,omitempty"`
	Live             bool   `json:"live"`
	Worker           bool   `json:"worker,omitempty"`
}

func statusJSONRows(rows []status.Row) []statusRowJSON {
	out := make([]statusRowJSON, 0, len(rows))
	for _, row := range rows {
		state := row.State
		if !row.Live {
			state = i18n.T("status.not_live")
		}
		out = append(out, statusRowJSON{
			Name: row.Name, State: state, UnchangedMinutes: row.UnchangedMinutes,
			Live: row.Live, Worker: row.Worker,
		})
	}
	return out
}

// statusJSONTeam is one team's rows: the team name, the definition it was
// booted from (empty when that cannot be read, with the reason in Error).
type statusJSONTeam struct {
	Team       string          `json:"team"`
	Definition string          `json:"definition,omitempty"`
	Error      string          `json:"error,omitempty"`
	Members    []statusRowJSON `json:"members"`
}

// statusJSONList is status --json with no team name: every running team.
type statusJSONList struct {
	Teams []statusJSONTeam `json:"teams"`
}

func printStatusJSONList(w io.Writer, teams []statusJSONTeam) error {
	if teams == nil {
		teams = []statusJSONTeam{}
	}
	return writeStatusJSON(w, statusJSONList{Teams: teams})
}

func writeStatusJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// screenMarks is what status keeps between runs, for one command line: the
// member screens read last time, and the ones this run will leave behind. now is
// a field, so a test can fix the clock and check the "N min" wording.
type screenMarks struct {
	log  screenlog.Log
	prev map[string]screenlog.Entry
	next map[string]screenlog.Entry
	now  func() time.Time
}

// screenClock is the clock status stamps a fresh screen with. It is a variable
// so a test can fix the time and check the "N min" wording without waiting.
var screenClock = time.Now

// openScreenMarks reads this machine's record of member screens. A record that
// cannot be read leaves the marks off: the extra line is not worth failing the
// command for.
func openScreenMarks() (*screenMarks, error) {
	log, err := screenlog.Open()
	if err != nil {
		return nil, err
	}
	prev, err := log.Read()
	if err != nil {
		return nil, err
	}
	next := make(map[string]screenlog.Entry, len(prev))
	maps.Copy(next, prev)
	return &screenMarks{log: log, prev: prev, next: next, now: screenClock}, nil
}

// mark fills each row's unchanged mark and records this run's screens. A member
// whose screen is the same as last time keeps the time that screen was first
// seen, so the mark counts the whole time it has stood, not the time since the
// last status. A member whose screen is new starts that clock now.
func (m *screenMarks) mark(rows []status.Row) {
	now := m.now()
	for i := range rows {
		row := &rows[i]
		if row.Screen == "" {
			continue
		}
		entry, seen := m.prev[row.Name]
		if seen && entry.Hash == row.Screen {
			if row.Live && row.State == "working" {
				minutes := int(now.Sub(entry.At).Minutes())
				row.UnchangedMinutes = &minutes
			}
			continue
		}
		m.next[row.Name] = screenlog.Entry{Hash: row.Screen, At: now}
	}
}

// save writes the record. A record that cannot be written is left, not reported.
func (m *screenMarks) save() { _ = m.log.Write(m.next) }

// markScreens records the screens of one team's rows, best effort: a machine
// whose state directory cannot be read still prints status.
func markScreens(rows []status.Row) {
	marks, err := openScreenMarks()
	if err != nil {
		return
	}
	marks.mark(rows)
	marks.save()
}

// runFocus moves the focus to one member's pane.
func runFocus(env team.Env, opts *options.Options, stdout io.Writer) error {
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	return newLauncher(r, opts.Timeout, stdout).Focus(member)
}

// runNudge sends one member a short push to carry on, only when it is idle or
// done. --dry prints the message and sends nothing.
func runNudge(env team.Env, opts *options.Options, stdout io.Writer) error {
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	body := opts.Body
	if body == "" {
		body = i18n.T("nudge.default")
	}
	text := sendText(senderName(r, os.Getenv("HERDR_PANE_ID")), body)
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("send.dry"), member.Name, text)
		return nil
	}
	return newLauncher(r, opts.Timeout, stdout).Nudge(member, text)
}

// runStop interrupts one member's current turn. --dry prints what would stop and
// reads nothing.
func runStop(env team.Env, opts *options.Options, stdout io.Writer) error {
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("stop.dry"), member.Name)
		return nil
	}
	stopped, err := newLauncher(r, opts.Timeout, stdout).Stop(member, os.Getenv("HERDR_PANE_ID"))
	if err != nil {
		// esc twice did not end the turn. The next step is the same one a stop
		// that worked names: the member has to be started again.
		if _, ok := errors.AsType[launcher.StopNotStopped](err); ok {
			printNext(stdout, "refresh "+member.Name+" --team "+r.team.Name)
		}
		return err
	}
	if stopped {
		printNext(stdout, "refresh "+member.Name+" --team "+r.team.Name)
	}
	return nil
}

// runTell sends one member a message from a person or another member. --dry
// prints the message and sends nothing.
func runTell(env team.Env, opts *options.Options, stdout io.Writer) error {
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.member(opts.Member)
	if err != nil {
		return err
	}
	from, err := tellSender(r, opts)
	if err != nil {
		return err
	}
	text := sendText(from, opts.Body)
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("send.dry"), member.Name, text)
		return nil
	}
	return newLauncher(r, opts.Timeout, stdout).Tell(member, text)
}

// sendText heads a message with who it is from, the shape a member's own message
// carries (prompt.self_intro_*). An empty from is the human.
func sendText(from, body string) string {
	if from == "" {
		from = i18n.T("send.who_human")
	}
	return i18n.Tf("send.from", from) + " " + body
}

// senderName is which member of this team typed the command, read from the pane
// hergy runs in: the herdr name of the member whose agent runs there, or "" when
// the pane is not one of the team's members (the human).
func senderName(r *resolved, selfPane string) string {
	if selfPane == "" {
		return ""
	}
	agent, err := herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))).Agent(selfPane)
	if err != nil {
		return ""
	}
	for i := range r.starts {
		if r.starts[i].Name == agent.Name {
			return agent.Name
		}
	}
	return ""
}

// tellSender is who a tell's message is from: --from resolved against the team,
// else the pane's member, else the human (""). A --from name the team does not
// have is refused.
func tellSender(r *resolved, opts *options.Options) (string, error) {
	if opts.From == "" {
		return senderName(r, os.Getenv("HERDR_PANE_ID")), nil
	}
	member, err := r.member(opts.From)
	if err != nil {
		return "", i18n.Errorf("error.tell_from_unknown", opts.From)
	}
	return member.Name, nil
}

// brokenTeamLine is why a running team's definition cannot be read, as one line:
// the file as a person writes it and the reason, the way hergy validate writes a
// broken file.
func brokenTeamLine(env team.Env, err error) string {
	if readErr, ok := errors.AsType[*catalog.ReadError](err); ok {
		return breakLine(env, readErr.Path, err)
	}
	return strings.Join(strings.Fields(err.Error()), " ")
}

// printStatusJudgement prints the judgement line for the rows: who is waiting on
// the human, else that nobody is working. A working team with nobody blocked
// gets no line. indent, when set, is written before the line.
func printStatusJudgement(stdout io.Writer, rows []status.Row, indent string) error {
	if line := status.Judge(rows).Text(); line != "" {
		fmt.Fprintf(stdout, "%s%s\n", indent, line)
	}
	return nil
}

func runRefresh(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	r, err := resolveTeam(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := r.selectStarts(memberName)
	if err != nil {
		return err
	}
	if opts.Dry {
		// --dry stops nothing, but it shows the same refusal the real refresh
		// would, so it reads the live agents too (like down --dry).
		if err := newLauncher(r, opts.Timeout, stdout).RefreshDry(selected, os.Getenv("HERDR_PANE_ID")); err != nil {
			return err
		}
		next := "refresh " + memberName
		if opts.All {
			next = "refresh --all"
		}
		next += " --team " + r.team.Name
		return dryPlan(stdout, env, r, "restart", selected, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := newLauncher(r, opts.Timeout, stdout)
	if err := runLaunch(ctx, stderr, l, func() error { return l.Refresh(selected, os.Getenv("HERDR_PANE_ID")) }); err != nil {
		return err
	}
	recordLaunch(stderr, env, r, launchlog.CommandRefresh, selected)
	return nil
}

func runReprompt(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	r, err := resolveTeam(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := r.selectStarts(memberName)
	if err != nil {
		return err
	}
	if opts.Dry {
		next := "reprompt " + memberName
		if opts.All {
			next = "reprompt --all"
		}
		next += " --team " + r.team.Name
		return dryPlan(stdout, env, r, "reprompt", selected, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := newLauncher(r, opts.Timeout, stdout)
	if err := runLaunch(ctx, stderr, l, func() error { return l.Reprompt(selected) }); err != nil {
		return err
	}
	recordLaunch(stderr, env, r, launchlog.CommandReprompt, selected)
	return nil
}

func runHire(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	resolve := resolveTeam
	if opts.Dry {
		// A plan is made from the team definition alone: a team that is not
		// running can be planned for, and herdr is not read (r-fe31).
		resolve = resolveTeamDry
	}
	r, err := resolve(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	worker, err := spawn.Plan(r.team.Name, r.starts, r.profiles, opts.Name, opts.Purpose, opts.Task, opts.From)
	if err != nil {
		return err
	}
	if opts.Dry {
		// A hire has no obvious next step: the worker is named by the member
		// that hires it, so only the plan is shown.
		return dryPlan(stdout, env, r, "spawn", []definition.Start{*worker}, opts, "")
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := newLauncher(r, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Hire(worker) })
}

// dryPlan is the --dry output: the same plan as JSON with --json, and a
// readable form (the prompt verbatim) by default. next, when not empty, is the
// command the plan moves on to, printed last and only in the readable form.
func dryPlan(stdout io.Writer, env team.Env, r *resolved, mode string, members []definition.Start, opts *options.Options, next string) error {
	if opts.JSON {
		return printPlanJSON(stdout, mode, r, members, opts.Timeout)
	}
	printPlanText(stdout, mode, r, members, opts.Timeout, env)
	if next != "" {
		printNext(stdout, next)
	}
	return nil
}

// runLaunch runs one Herdr operation, and on a failure prints the progress
// record and stops.
func runLaunch(ctx context.Context, stderr io.Writer, l *launcher.Launcher, fn func() error) error {
	return runOperation(ctx, stderr, l.ReportFailure, fn)
}

// runOperation runs one Herdr operation, and on Ctrl-C stops with 130. report,
// when set, says where a failed or interrupted operation stopped.
func runOperation(ctx context.Context, stderr io.Writer, report func(io.Writer), fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			fail(stderr, err)
			if report != nil {
				report(stderr)
			}
			return reportedError{err: err, code: 1}
		}
		return nil
	case <-ctx.Done():
		fmt.Fprint(stderr, i18n.T("launcher.interrupted"))
		if report != nil {
			report(stderr)
		}
		return reportedError{err: context.Canceled, code: 130}
	}
}

// printHeader is the short provenance every team command shows: the team's
// mission, the team it runs as, its version, and the files the effective values
// came from. show prints the definition alone; a team command prints the team
// name, and the definition behind it only when the two differ (I2). The mission
// is first: it is the one line that says what the team is for, and it is
// required.
func printHeader(w io.Writer, r *resolved, env team.Env, withTeam bool) {
	def := r.team.Definition
	if mission := r.missionLine(); mission != "" {
		field(w, label("mission"), mission)
	}
	if withTeam {
		// A team is called by its name. The definition it was booted from is
		// named only when the two differ (I2).
		field(w, label("team"), startedFrom(r.team.Name, def.Name))
	} else {
		field(w, label("definition"), def.Name)
	}
	field(w, label("version"), strconv.Itoa(def.Version))
	field(w, label("team_file"), env.Display(def.TeamYAML))
	switch {
	case r.profilesYAML == "":
		field(w, label("profiles"), i18n.T("value.none"))
	case r.usesProfiles():
		field(w, label("profiles"), displayPaths(r.profilesYAML, env))
	default:
		field(w, label("profiles"), i18n.Tf("value.unused", displayPaths(r.profilesYAML, env)))
	}
	if len(def.Ledger) > 0 {
		field(w, label("ledger"), ledgerText(def.Ledger))
	}
}

// startedFrom is how a running team is shown: its name, and the definition it
// was booted from, only when the two differ (I2).
func startedFrom(team, definition string) string {
	if team == definition {
		return team
	}
	return i18n.Tf("note.started_from", team, definition)
}

// printMember is one member: what it runs, where it runs, and the files its
// prompt was built from. With prompt, the expanded prompt is printed verbatim
// after the fields, so up --dry shows exactly what the agent would receive.
func printMember(w io.Writer, r *resolved, member *definition.Start, env team.Env, prompt bool) {
	field(w, label("member"), member.Name)
	indentField(w, 1, label("role"), member.Role)
	indentField(w, 1, label("kind"), member.Kind)
	if member.Profile != "" {
		if value, ok := r.profiles[member.Profile]; ok {
			indentField(w, 1, label("profile"),
				i18n.Tf("value.profile_with_path", member.Profile, env.Display(value.Path)))
		}
	}
	if len(member.Args) > 0 {
		indentField(w, 1, label("args"), strings.Join(member.Args, " "))
	}
	if len(member.Env) > 0 {
		indentField(w, 1, label("env"), envSummary(member.Env))
	}
	indentField(w, 1, label("cwd"), env.Display(member.Cwd))
	if len(member.Turns) > 0 {
		indentField(w, 1, label("turns"), strings.Join(member.Turns, i18n.T("list.separator")))
	}
	if len(member.Workers) > 0 {
		indentField(w, 1, label("workers"), workersText(member.Workers))
	}
	if member.HasRole {
		indentField(w, 1, label("role_file"), env.Display(member.RoleDoc.Path))
	} else {
		indentField(w, 1, label("role_file"), i18n.T("value.none"))
	}
	if len(member.Read) > 0 {
		for _, doc := range member.Read {
			indentField(w, 1, label("reads"), env.Display(doc.Path))
		}
	} else {
		indentField(w, 1, label("reads"), i18n.T("value.none"))
	}
	if prompt {
		printPrompt(w, member.Prompt)
	}
}

// printPrompt prints the expanded prompt unescaped, as a block under the
// prompt field, indented one level past the member.
func printPrompt(w io.Writer, prompt string) {
	indentField(w, 1, label("prompt"), "|")
	for line := range strings.SplitSeq(strings.TrimRight(prompt, "\n"), "\n") {
		if line == "" {
			fmt.Fprint(w, "\n")
			continue
		}
		fmt.Fprintf(w, "    %s\n", line)
	}
}

// printPlanText is the readable --dry plan. It is the same information as the
// JSON, with each member's prompt shown as the text it is.
func printPlanText(w io.Writer, mode string, r *resolved, members []definition.Start, timeout int, env team.Env) {
	field(w, label("mode"), mode)
	printHeader(w, r, env, true)
	field(w, label("timeout"), strconv.Itoa(timeout))
	for i := range members {
		printMember(w, r, &members[i], env, true)
	}
}

// printPlanJSON is the --dry output with --json: the expanded members as JSON.
func printPlanJSON(w io.Writer, mode string, r *resolved, members []definition.Start, timeout int) error {
	plans := make([]map[string]any, len(members))
	for i := range members {
		plans[i] = members[i].ToPlan()
	}
	plan := map[string]any{
		"mode":               mode,
		"team":               r.team.Name,
		"startup_timeout_ms": timeout,
		"members":            plans,
	}
	if def := r.team.Definition; def != nil && len(def.Graph.Turns) > 0 {
		turns := make([]string, len(def.Graph.Turns))
		for i, turn := range def.Graph.Turns {
			turns[i] = turn.String()
		}
		plan["turns"] = turns
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// printNext names the command that moves on from what was just shown. The
// wording is not translated: next: stays next: in every language.
func printNext(w io.Writer, command string) {
	fmt.Fprintln(w, i18n.Tf("next", command))
}

// field writes one "name: value" line. depth is how many two-space levels to
// indent by.
func field(w io.Writer, name, value string) { indentField(w, 0, name, value) }

func indentField(w io.Writer, depth int, name, value string) {
	fmt.Fprintf(w, "%s%s: %s\n", strings.Repeat("  ", depth), name, value)
}

// label is one output field name in the language in use.
func label(key string) string { return i18n.T("label." + key) }

// displayPaths shows a comma-joined list of paths, each in its short form.
func displayPaths(joined string, env team.Env) string {
	paths := strings.Split(joined, ", ")
	for i := range paths {
		paths[i] = env.Display(paths[i])
	}
	return strings.Join(paths, ", ")
}

// workersText is what show and up --dry print for workers: the purpose and
// profile of each worker pair, in the written order.
func workersText(workers []definition.WorkerNode) string {
	parts := make([]string, len(workers))
	for i, worker := range workers {
		parts[i] = fmt.Sprintf(i18n.T("workers.allowed_item"), worker.Purpose, worker.Profile)
	}
	return strings.Join(parts, i18n.T("list.separator"))
}

func newLauncher(r *resolved, timeout int, out io.Writer) *launcher.Launcher {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return launcher.New(r.starts, r.team.Name, herdr.New(port), timeout, out)
}

// requireHerdrPane refuses to touch Herdr outside a Herdr-managed shell pane.
func requireHerdrPane() error {
	if os.Getenv("HERDR_ENV") != "1" || os.Getenv("HERDR_PANE_ID") == "" {
		return i18n.Errorf("error.not_under_herdr")
	}
	return nil
}

func envSummary(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	// deterministic order
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return strings.Join(keys, ", ")
}

// asSuffix repeats --as on the up command a dry run names next.
func asSuffix(opts *options.Options) string {
	if opts.As == "" {
		return ""
	}
	return " --as " + opts.As
}
