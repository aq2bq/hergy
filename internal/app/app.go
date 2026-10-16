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

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/spawn"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/topology"
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
		var reported reportedError
		if errors.As(err, &reported) {
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
func readDefinition(cat *catalog.Catalog, base, name, layer, runName string) (*resolved, error) {
	def, err := cat.Definition(name, layer)
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

// startsOf builds the running members: the frame of each member, the engine its
// profile names, and the prompt hergy hands it (a projection of the definition
// and the catalog).
func startsOf(cat *catalog.Catalog, def *definition.Definition, runName, base string, profiles map[string]profile.Profile, reader source.Reader) ([]definition.Start, error) {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = "hergy"
	}
	run := prompt.Run{Team: runName, Executable: executable, Base: base, Reader: reader, Sources: cat, Display: cat.Display}
	starts := make([]definition.Start, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		// herdr sees the member as "<team>-<member>"; the team name is checked
		// short enough for this to fit, but a long member name can still
		// overflow the limit.
		herdr := runName + "-" + node.Name
		if !profile.NamePattern.MatchString(herdr) {
			return nil, i18n.Errorf("error.herdr_name_pattern", herdr)
		}
		built, err := prompt.Member(def, node, run)
		if err != nil {
			return nil, err
		}
		value := profiles[node.Profile]
		starts = append(starts, definition.Start{
			Name:    herdr,
			Short:   node.Name,
			Role:    node.Role,
			Profile: node.Profile,
			Kind:    value.Kind,
			Args:    value.Args,
			Env:     value.Env,
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
// behind a running team name. agentsRead says whether the agent list could be
// read at all: with no agent names, the launch record is the best guess left at
// which tab is a team's.
func herdrTabs() (tabs []team.Tab, agentsRead bool, err error) {
	list, err := herdrTabList()
	if err != nil {
		return nil, false, err
	}
	agents, err := herdrAgents()
	if err != nil {
		// A herdr that cannot list its agents still lists its tabs: the tabs an
		// older hergy labelled are found from their labels, and the ones this
		// machine recorded are guessed at from the record, rather than lost.
		return teamTabs(list, nil), false, nil
	}
	return teamTabs(list, agents), true, nil
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
	r, err := readDefinition(cat, env.ProjectRoot, opts.Team, "", opts.As)
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
// document, in the order a prompt joins them: the agreement every team of the
// layer shares, then the team's own (段 4). It is read off the built prompts, so
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
	// is the project's own layer, and outside one the global layer.
	layer := source.LayerGlobal
	if env.InRepo() {
		layer = source.LayerLocal
	}
	dir := env.TeamDir(name, layer)
	if _, err := os.Stat(dir); err == nil {
		return i18n.Errorf("error.new_exists", name, env.Display(dir))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := scaffold.Team(i18n.Language())
	if err != nil {
		return err
	}
	if err := writeScaffold(dir, files); err != nil {
		return err
	}
	// A role document is the layer's, not the team's (K5), so it goes into the
	// layer's roles/ directory, and one that is already there is left alone.
	roles, err := scaffold.Roles(i18n.Language())
	if err != nil {
		return err
	}
	roleDir := filepath.Join(env.Catalog(source.Disk{}).LayerDir(layer), "roles")
	wroteRoles, keptRoles, err := writeScaffoldRoles(roleDir, roles)
	if err != nil {
		return err
	}
	configPath, madeConfig, err := env.EnsureGlobalConfig(i18n.Language())
	if err != nil {
		return err
	}
	profilePath, err := ensureScaffoldProfile(env, layer)
	if err != nil {
		return err
	}
	field(stdout, label("definition"), name)
	field(stdout, label("dir"), env.Display(dir))
	for _, file := range files {
		field(stdout, label("file"), env.Display(filepath.Join(dir, file.Path)))
	}
	for _, path := range wroteRoles {
		field(stdout, label("role_file"), env.Display(path))
	}
	for _, path := range keptRoles {
		field(stdout, label("note"), i18n.Tf("new.role_kept", env.Display(path)))
	}
	if profilePath != "" {
		field(stdout, label("profile"), scaffold.ProfileName)
		field(stdout, label("file"), env.Display(profilePath))
	}
	if madeConfig {
		field(stdout, label("config"), env.Display(configPath))
	}
	printNext(stdout, "up "+name+" --dry")
	return nil
}

// ensureScaffoldProfile makes sure the layer a starter team is written to can
// resolve the profile its members name (I7). The name is looked up the way a
// team of that layer resolves it: when it is already there, in the layer's own
// profiles.yaml or in a global one a local team reads, nothing is written and
// the path is "". A layer that cannot resolve the name gets it added to its own
// profiles.yaml, which is made when the layer has none yet. An existing file the
// layer cannot read is an error rather than something to write over.
func ensureScaffoldProfile(env team.Env, layer string) (string, error) {
	profiles, _, err := env.Catalog(source.Disk{}).Profiles()
	if err != nil {
		return "", err
	}
	if _, ok := profiles[scaffold.ProfileName]; ok {
		return "", nil
	}
	dir := env.GlobalDir
	if layer == source.LayerLocal {
		dir = env.LocalDir
	}
	path := filepath.Join(dir, "profiles.yaml")
	all := map[string]profile.Profile{}
	switch text, err := os.ReadFile(path); {
	case err == nil:
		if all, err = profile.ParseProfiles(string(text), path); err != nil {
			return "", err
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	default:
		return "", err
	}
	all[scaffold.ProfileName] = profile.Profile{
		Name: scaffold.ProfileName,
		Kind: scaffold.ProfileKind,
		Path: path,
	}
	text, err := profile.EmitProfiles(all)
	if err != nil {
		return "", err
	}
	if err := writeFileBeside(path, text); err != nil {
		return "", err
	}
	return path, nil
}

// writeScaffold writes the starter files under dir, creating the directories
// they need.
func writeScaffold(dir string, files []scaffold.File) error {
	for _, file := range files {
		path := filepath.Join(dir, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, file.Data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeScaffoldRoles writes the starter role documents into one layer's roles/
// directory. A document that is already there is left as it is: it belongs to
// the layer, not to this team, and a person's own file is never written over
// (K5). The paths it wrote and the paths it left come back for the output line.
func writeScaffoldRoles(dir string, files []scaffold.File) (wrote, kept []string, err error) {
	for _, file := range files {
		path := filepath.Join(dir, file.Path)
		_, statErr := os.Stat(path)
		switch {
		case statErr == nil:
			kept = append(kept, path)
		case !errors.Is(statErr, os.ErrNotExist):
			return nil, nil, statErr
		default:
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(path, file.Data, 0o644); err != nil {
				return nil, nil, err
			}
			wrote = append(wrote, path)
		}
	}
	return wrote, kept, nil
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
	seenFiles := map[string]bool{}
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
			profiles, err := validateProfiles(path)
			report(path, err)
			if err == nil {
				reportProfileNames(stdout, profiles)
			}
		}
	}
	for _, layer := range []string{source.LayerGlobal, source.LayerLocal} {
		reportOldCommon(stdout, env, cat.LayerDir(layer))
		for _, row := range cat.Definitions() {
			path := listingPath(row, layer)
			if path == "" {
				continue
			}
			checked++
			size, fileProblems, ok := validateDefinition(cat, env, stdout, tally, seenFiles, row, layer)
			problems += fileProblems
			if !ok {
				continue
			}
			teams = append(teams, size)
		}
	}

	field(stdout, label("checked"), strconv.Itoa(checked))
	field(stdout, label("problems"), strconv.Itoa(problems))
	printTally(stdout, env, tally)
	printSize(stdout, teams)
	if problems > 0 {
		return reportedError{err: errors.New("validate found problems"), code: 1}
	}
	return nil
}

// reportFile writes one team definition's own file: where it is, as a person
// writes it, whether it reads, and why it does not. A graph the definition
// cannot hold is one detail line per problem (I12a, I12b), the same sentences
// validate has always printed (d-edd4); each is a separate thing to fix, so each
// is counted. It returns how many problems it found.
func reportFile(stdout io.Writer, env team.Env, path string, err error) int {
	field(stdout, label("file"), env.Display(path))
	if gerr, ok := graphErrorOf(err); ok {
		indentField(stdout, 1, label("result"), i18n.T("value.broken"))
		for _, detail := range gerr.Messages() {
			indentField(stdout, 1, label("detail"), breakLine(env, path, errors.New(detail)))
		}
		return len(gerr.Problems)
	}
	if err == nil {
		indentField(stdout, 1, label("result"), i18n.T("value.ok"))
		return 0
	}
	indentField(stdout, 1, label("result"), i18n.T("value.broken"))
	indentField(stdout, 1, label("detail"), breakLine(env, path, err))
	return 1
}

// graphErrorOf is the graph problem a definition's reading stopped on, when that
// is what it stopped on (d-edd4). validate writes a detail line per problem, so
// it wants the problems rather than the one message Error() joins them into.
func graphErrorOf(err error) (*definition.GraphError, bool) {
	var gerr *definition.GraphError
	if err == nil || !errors.As(err, &gerr) {
		return nil, false
	}
	return gerr, true
}

// validateDefinition checks one definition of one layer the way up reads it,
// writes its result line and its warnings, and returns the size to print after
// the per-file lines, and how many problems it found. ok is false when the
// definition does not read.
func validateDefinition(cat *catalog.Catalog, env team.Env, stdout io.Writer, tally *rules.Tally, seen map[string]bool, row catalog.Listing, layer string) (size teamSize, problems int, ok bool) {
	path := listingPath(row, layer)
	r, err := validateTeam(cat, env.ProjectRoot, row.Name, layer)
	problems = reportFile(stdout, env, path, err)
	reportOldTeamFiles(stdout, env, cat, row.Name, layer)
	if err != nil {
		return teamSize{}, problems, false
	}
	if scaffold.IsPlaceholderMission(r.team.Definition.Mission) {
		// The starter team's mission is an example: the definition reads, but
		// its team has not said what it is for (d-3cae).
		indentField(stdout, 1, label("warning"), i18n.T("validate.mission_placeholder"))
	}
	// A rule text that still says "a → b" is where the turns used to live; the
	// warning says when the text and turns: disagree.
	for _, warning := range turnWarnings(cat, r) {
		indentField(stdout, 1, label("warning"), warning)
	}
	if layer == source.LayerGlobal && row.Local {
		// The project's definition of this name is the one that is used, so the
		// global one is hidden (I11); validate says so, where a person reading
		// the global file finds it.
		indentField(stdout, 1, label("warning"), i18n.Tf("validate.global_hidden", row.Name))
	}
	collectRules(tally, seen, r)
	size = sizeOf(r)
	size.name = env.Display(filepath.Dir(path))
	return size, 0, true
}

// reportOldTeamFiles warns about files a team definition used to keep where
// hergy no longer reads them (段 4, spec/32 D1): a protocol.yaml that was not
// renamed to agreement.yaml, and role documents that were not moved out of the
// definition's folder. The warning stands under the team's own file line, names
// the file and the place it goes now, and moves no exit code: the definition
// still reads, but the rules those files hold are read no more.
func reportOldTeamFiles(w io.Writer, env team.Env, cat *catalog.Catalog, name, layer string) {
	dir := cat.LayerDir(layer)
	old := filepath.Join(dir, "teams", name, "protocol.yaml")
	if isFile(old) {
		indentField(w, 1, label("warning"), i18n.Tf("validate.old_team_agreement",
			env.Display(old), env.Display(filepath.Join(dir, "teams", name, "agreement.yaml"))))
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "teams", name, "roles", "*.yaml"))
	slices.Sort(matches)
	for _, path := range matches {
		role := strings.TrimSuffix(filepath.Base(path), ".yaml")
		indentField(w, 1, label("warning"), i18n.Tf("validate.old_team_role",
			env.Display(path), env.Display(filepath.Join(dir, "roles", role+".yaml"))))
	}
}

// reportOldCommon warns about the layer's common/ directory, where the agreement
// every team shares used to stand as a folder of files: it is one agreement.yaml
// at the layer's root now (段 4, spec/32 D1). It is asked once for the layer
// rather than once per team, because the directory is the layer's, and it moves
// no exit code.
func reportOldCommon(w io.Writer, env team.Env, dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "common", "*.yaml"))
	if len(matches) == 0 {
		return
	}
	field(w, label("warning"), i18n.Tf("validate.old_common",
		env.Display(filepath.Join(dir, "common")), env.Display(filepath.Join(dir, "agreement.yaml"))))
}

// turnWarnings is the turn directions a team's rule texts claim and the graph
// does not have (段 5). A rule text writes a turn as "a → b"; the turns live in
// turns: now, and this says when the two disagree. A text that names a party the
// graph has no node for claims no edge the graph should hold — a rule names a
// worker by its purpose, and workers: already ties that worker to its member — so
// those are left alone. Nothing here stops a team from booting.
func turnWarnings(cat *catalog.Catalog, r *resolved) []string {
	members, docs := turnReading(cat, r)
	graph := r.team.Definition.Graph
	var out []string
	seen := map[string]bool{}
	for _, edge := range topology.Read(members, docs).Edges {
		for _, pair := range edgePairs(edge, members) {
			if graphHasTurn(graph, pair[0], pair[1]) {
				continue
			}
			key := edge.RuleID + "\x00" + pair[0].String() + "\x00" + pair[1].String()
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, i18n.Tf("validate.turn_not_in_turns", edge.RuleID,
				pair[0].String()+" → "+pair[1].String()))
		}
	}
	return out
}

// edgePairs is the turns one rule-text arrow claims: one, or both ways for "⇄".
// An arrow with an end the graph has no node for claims none of them.
func edgePairs(edge topology.Edge, members []topology.Member) [][2]definition.NodeID {
	from, okFrom := turnNode(edge.From, members)
	to, okTo := turnNode(edge.To, members)
	if !okFrom || !okTo {
		return nil
	}
	pairs := [][2]definition.NodeID{{from, to}}
	if edge.Both {
		pairs = append(pairs, [2]definition.NodeID{to, from})
	}
	return pairs
}

// turnNode is one end of a rule-text arrow as a node of the graph: a member the
// team has, or the human. A name that matched no member is not a node.
func turnNode(side topology.Side, members []topology.Member) (definition.NodeID, bool) {
	switch side.Kind {
	case topology.SideHuman:
		return definition.NodeID{Kind: definition.KindHuman, Name: "human"}, true
	case topology.SideMember:
		if side.Member >= 0 && side.Member < len(members) {
			return definition.NodeID{Kind: definition.KindMember, Name: members[side.Member].Name}, true
		}
	case topology.SideStranger:
		// A worker's purpose, or a name the team has no node for: not a node.
	}
	return definition.NodeID{}, false
}

// graphHasTurn is whether the graph already holds this turn.
func graphHasTurn(g *definition.Graph, from, to definition.NodeID) bool {
	for _, edge := range g.Turns {
		if edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}

// turnReading is the members and the rule files the reading of a rule text's
// turn arrows needs: the members of the team, and the documents a member reads
// (段 4: the agreement every team of the layer shares, the team's own agreement,
// and each member's role document), with the rules parsed. It reads what the
// definition names, not what one prompt happened to ask for, so a team whose
// prompt drops %{docs} still has its rule texts compared with turns:. A file that
// cannot be read is left out: the definition was built a moment ago with the same
// reading, so the break was already reported.
func turnReading(cat *catalog.Catalog, r *resolved) ([]topology.Member, []topology.Doc) {
	def := r.team.Definition
	members := make([]topology.Member, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		var workers []topology.Worker
		for _, worker := range def.Graph.Workers {
			if worker.From == member.Name {
				workers = append(workers, topology.Worker{Purpose: worker.Purpose, Profile: worker.Profile})
			}
		}
		members = append(members, topology.Member{Name: member.Name, Role: member.Role, Workers: workers})
	}
	var docs []topology.Doc
	seen := map[string]bool{}
	add := func(doc source.Doc, role string) {
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
		entry := topology.Doc{Key: doc.Path, Label: parsed.Title, Role: role}
		for _, rule := range parsed.Rules() {
			entry.Rules = append(entry.Rules, topology.Rule{ID: rule.ID, Text: rule.Text})
		}
		docs = append(docs, entry)
	}
	if cat != nil {
		if shared, ok := cat.SharedAgreement(); ok {
			add(shared, "")
		}
	}
	if def.HasAgreement {
		add(def.Agreement, "")
	}
	if cat != nil {
		for i := range def.Graph.Members {
			member := &def.Graph.Members[i]
			if doc, ok := cat.Role(member.Role); ok {
				add(doc, member.Role)
			}
		}
	}
	return members, docs
}

// reportProfileNames is the one warning a profiles.yaml gets: a profile whose
// name is its kind (d-a477). A name that is the kind makes a profile look like a
// choice of kind, where I7 makes it a choice of engine settings; the name should
// say the model or the settings instead. It stands under the file's own result
// line, and moves no exit code: such a profile runs the same.
func reportProfileNames(w io.Writer, profiles map[string]profile.Profile) {
	for _, name := range slices.Sorted(maps.Keys(profiles)) {
		if profiles[name].Kind == name {
			indentField(w, 1, label("warning"), i18n.Tf("validate.profile_named_after_kind", name))
		}
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

// memberSize is one member's size, with the name the output writes.
type memberSize struct {
	name string
	size rules.MemberSize
}

// teamSize is one team definition's size, with where it stands and the members
// it hands its rules to. The members stand under the definition in the output,
// so a definition name that stands in both layers is told apart by the block it
// is in.
type teamSize struct {
	name    string
	size    rules.TeamSize
	members []memberSize
}

// sizeOf counts what one team definition hands each member and what the team
// keeps. Each rule file is read and parsed once; a file two members read is
// counted in the team's sources once, because a person maintains it once.
func sizeOf(r *resolved) teamSize {
	parsed := map[string]*rules.Document{}
	texts := map[string]string{}
	load := func(doc source.Doc) *rules.Document {
		if document, ok := parsed[doc.Path]; ok {
			return document
		}
		text, err := os.ReadFile(doc.Path)
		if err != nil {
			parsed[doc.Path] = nil
			return nil
		}
		texts[doc.Path] = string(text)
		document, err := rules.Parse(string(text), doc.Path)
		if err != nil {
			parsed[doc.Path] = nil
			return nil
		}
		parsed[doc.Path] = document
		return document
	}
	var team teamSize
	var size rules.TeamSize
	for i := range r.starts {
		member := &r.starts[i]
		var read rules.MemberSize
		files := make([]source.Doc, 0, len(member.Read)+1)
		if member.HasRole {
			files = append(files, member.RoleDoc)
		}
		files = append(files, member.Read...)
		for _, doc := range files {
			if document := load(doc); document != nil {
				read.Add(document)
			}
			if text, ok := texts[doc.Path]; ok {
				size.AddSource(doc.Path, text)
			}
		}
		team.members = append(team.members, memberSize{name: member.Name, size: read})
	}
	team.size = size
	return team
}

// printSize is the size of what each team definition keeps and of what each of
// its members is handed, after the rules count. A definition comes first and its
// members stand under it, so a name that stands in both layers says which file
// its warnings are about. A member or a definition over one of the provisional
// limits (d-f2bc) gets a warning line; the numbers move no exit code, because a
// large team is a team to look at, not a broken one.
func printSize(w io.Writer, teams []teamSize) {
	if len(teams) == 0 {
		return
	}
	line := func(text string) { fmt.Fprintf(w, "%s\n", text) }
	warned := false
	for _, team := range teams {
		line(i18n.Tf("validate.size_team", team.name, team.size.Sources, team.size.Chars))
		if team.size.OverTeamSources() {
			line(i18n.Tf("validate.size_warn_team_sources", team.name, team.size.Sources, rules.TeamSourceLimit))
			warned = true
		}
		if team.size.OverTeamChars() {
			line(i18n.Tf("validate.size_warn_team_chars", team.name, team.size.Chars, rules.TeamSourceCharLimit))
			warned = true
		}
		for _, member := range team.members {
			line(i18n.Tf("validate.size_member", member.name, member.size.Rules, member.size.Chars))
			if member.size.OverMemberRules() {
				line(i18n.Tf("validate.size_warn_member_rules", member.name, member.size.Rules, rules.MemberRuleLimit))
				warned = true
			}
			if member.size.OverMemberChars() {
				line(i18n.Tf("validate.size_warn_member_chars", member.name, member.size.Chars, rules.MemberCharLimit))
				warned = true
			}
		}
	}
	if warned {
		line(i18n.T("validate.size_note"))
	}
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

// validateProfiles reads one profiles.yaml the way up does, and returns the
// profiles it holds for the name warning (d-a477).
func validateProfiles(path string) (map[string]profile.Profile, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return profile.ParseProfiles(string(text), path)
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

func runUp(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	r, err := readDefinition(cat, env.ProjectRoot, opts.Team, "", opts.As)
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
	printHeader(stdout, r, env, true)
	rows, err := newLauncher(r, opts.Timeout, stdout).Status(r.starts)
	if err != nil {
		return err
	}
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
	running, err := runningTeams()
	if err != nil {
		return err
	}
	if len(running) == 0 {
		fmt.Fprintln(stdout, i18n.T("status.none"))
		return nil
	}
	cat := env.Catalog(source.Disk{})
	for i, entry := range running {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		r, err := buildRunning(cat, env.ProjectRoot, entry)
		if err != nil {
			// A team booted from another project's definition is still listed,
			// so one such team does not hide the rest. A definition that is here
			// but does not read says why, with its file's path; only a
			// definition that is not here says "another project".
			field(stdout, label("team"), startedFrom(entry.Team, entry.Definition))
			if errors.Is(err, catalog.ErrNotFound) {
				fmt.Fprintf(stdout, "  %s\n", i18n.T("status.definition_elsewhere"))
			} else {
				indentField(stdout, 1, label("detail"), brokenTeamLine(env, err))
			}
			continue
		}
		field(stdout, label("team"), startedFrom(r.team.Name, r.team.Definition.Name))
		rows, err := newLauncher(r, opts.Timeout, stdout).Status(r.starts)
		if err != nil {
			return err
		}
		membersCount, workers := status.Counts(rows)
		indentField(stdout, 1, label("members"), strconv.Itoa(membersCount))
		indentField(stdout, 1, label("workers"), strconv.Itoa(workers))
		if err := printStatusJudgement(stdout, rows, "  "); err != nil {
			return err
		}
	}
	return nil
}

// brokenTeamLine is why a running team's definition cannot be read, as one line:
// the file as a person writes it and the reason, the way hergy validate writes a
// broken file.
func brokenTeamLine(env team.Env, err error) string {
	var readErr *catalog.ReadError
	if errors.As(err, &readErr) {
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
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
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
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
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
	r, err := resolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
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
