// Package app is the edge of the CLI: arguments, environment, files, output
// and exit codes. Everything it decides comes from the packages below it.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/spawn"
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
		if options.IsHelp(err) {
			fmt.Fprint(stdout, options.Usage())
			return 0
		}
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
		return runLs(env, opts, stdout)
	case options.Show:
		return runShow(env, opts, stdout)
	case options.New:
		return runNew(env, opts, stdout)
	case options.Validate:
		return runValidate(env, stdout)
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

// resolveAndBuild is the shared path: find the team, then parse it.
func resolveAndBuild(env team.Env, opts *options.Options) (*team.Resolved, *config.Config, error) {
	resolved, err := env.Resolve(opts.Team, opts.Layer)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := env.BuildConfig(resolved, source.Disk{}, opts.As)
	if err != nil {
		return nil, nil, err
	}
	return resolved, cfg, nil
}

// asSuffix is the --as part of a next command, empty when no --as was given.
func asSuffix(opts *options.Options) string {
	if opts.As == "" {
		return ""
	}
	return " --as " + opts.As
}

func runLs(env team.Env, opts *options.Options, stdout io.Writer) error {
	field(stdout, label("local"), env.Display(env.LocalDir))
	field(stdout, label("global"), env.Display(env.GlobalDir))
	shown := 0
	for _, row := range env.List() {
		var layer, path, other, note string
		switch opts.Layer {
		case "global":
			if !row.Global {
				continue
			}
			layer, path = source.LayerGlobal, row.GlobalPath
			if row.Local {
				note = i18n.T("value.overridden_local")
			}
		case "local":
			if !row.Local {
				continue
			}
			layer, path = source.LayerLocal, row.LocalPath
			if row.Global {
				note = i18n.T("value.also_global")
			}
		default:
			if row.Local {
				layer, path = source.LayerLocal, row.LocalPath
				if row.Global {
					other = row.GlobalPath
				}
			} else {
				layer, path = source.LayerGlobal, row.GlobalPath
			}
		}
		shown++
		field(stdout, label("team"), row.Name)
		indentField(stdout, 1, label("layer"), layer)
		indentField(stdout, 1, label("path"), env.Display(path))
		if other != "" {
			indentField(stdout, 1, label("global"), env.Display(other))
		}
		if note != "" {
			indentField(stdout, 1, label("note"), note)
		}
	}
	if shown == 0 {
		field(stdout, label("team"), i18n.T("value.none"))
	}
	return nil
}

func runShow(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	if len(cfg.Docs) == 0 {
		field(stdout, label("document"), i18n.T("value.none"))
	}
	for _, doc := range cfg.Docs {
		field(stdout, label("document"), env.Display(doc.Path))
	}
	for i := range cfg.Members {
		printMember(stdout, cfg, &cfg.Members[i], env, false)
	}
	return nil
}

// runNew writes a starter team's files. It never overwrites: an existing team
// directory stops it before anything is written. The files themselves come from
// the embedded scaffold for the language in use.
func runNew(env team.Env, opts *options.Options, stdout io.Writer) error {
	name := opts.Team
	if !scaffold.NamePattern.MatchString(name) {
		return i18n.Errorf("error.new_name_pattern", name)
	}
	layer := source.LayerGlobal
	if opts.Layer == "local" {
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
	configPath, madeConfig, err := env.EnsureGlobalConfig(i18n.Language())
	if err != nil {
		return err
	}
	field(stdout, label("team"), name)
	field(stdout, label("layer"), layer)
	field(stdout, label("dir"), env.Display(dir))
	for _, file := range files {
		field(stdout, label("file"), env.Display(filepath.Join(dir, file.Path)))
	}
	if madeConfig {
		field(stdout, label("config"), env.Display(configPath))
	}
	printNext(stdout, "up "+name+" --dry")
	return nil
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

// runValidate reads every settings file the way up does, shows one line per
// file, and fails when any of them is broken. It never touches Herdr.
func runValidate(env team.Env, stdout io.Writer) error {
	checked, problems := 0, 0
	report := func(path string, err error) {
		checked++
		field(stdout, label("file"), env.Display(path))
		if err == nil {
			indentField(stdout, 1, label("result"), i18n.T("value.ok"))
			return
		}
		problems++
		msg := strings.ReplaceAll(err.Error(), path, env.Display(path))
		// Some team-file errors do not name the file; the file-level parser
		// errors already do. Show the location once either way, and keep the
		// whole reason on one line.
		if !strings.Contains(msg, env.Display(path)) {
			msg = fmt.Sprintf("%s: %s", env.Display(path), msg)
		}
		msg = strings.Join(strings.Fields(msg), " ")
		indentField(stdout, 1, label("result"), i18n.T("value.broken"))
		indentField(stdout, 1, label("detail"), msg)
	}

	if path := filepath.Join(env.GlobalDir, "config.yaml"); isFile(path) {
		report(path, validateConfig(path))
	}
	for _, dir := range []string{env.GlobalDir, env.LocalDir} {
		if path := filepath.Join(dir, "profiles.yaml"); isFile(path) {
			report(path, validateProfiles(path))
		}
	}
	rows := env.List()
	for _, layer := range []string{source.LayerGlobal, source.LayerLocal} {
		for _, row := range rows {
			var path string
			switch layer {
			case source.LayerGlobal:
				if !row.Global {
					continue
				}
				path = row.GlobalPath
			case source.LayerLocal:
				if !row.Local {
					continue
				}
				path = row.LocalPath
			}
			report(path, validateTeam(env, row.Name, layer))
		}
	}

	field(stdout, label("checked"), strconv.Itoa(checked))
	field(stdout, label("problems"), strconv.Itoa(problems))
	if problems > 0 {
		return reportedError{err: errors.New("validate found problems"), code: 1}
	}
	return nil
}

// validateConfig reads the global config.yaml the way up does, and holds its
// lang to the same check every other command applies when it picks a language.
func validateConfig(path string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lang, err := config.ParseConfig(string(text), path)
	if err != nil {
		return err
	}
	_, err = i18n.Resolve("", lang)
	return err
}

// validateProfiles reads one profiles.yaml the way up does.
func validateProfiles(path string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = config.ParseProfiles(string(text), path)
	return err
}

// validateTeam builds one team from one layer the way up does.
func validateTeam(env team.Env, name, layer string) error {
	resolved, err := env.Resolve(name, layer)
	if err != nil {
		return err
	}
	_, err = env.BuildConfig(resolved, source.Disk{}, "")
	return err
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func runUp(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	if opts.Dry {
		return dryPlan(stdout, env, resolved, cfg, "boot", cfg.Members, opts, "up "+resolved.Name+asSuffix(opts))
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, l.Boot)
}

// runDown closes the panes of a live team's members. --dry prints what would
// close and reads the live agents, but sends no close and needs no Herdr pane.
func runDown(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	members, err := cfg.Select("")
	if err != nil {
		return err
	}
	// The pane hergy runs in is the member that must be closed last.
	selfPane := os.Getenv("HERDR_PANE_ID")
	if opts.Dry {
		printHeader(stdout, resolved, cfg, env)
		closed, err := newLauncher(cfg, opts.Timeout, stdout).Down(members, selfPane, true)
		if err != nil {
			return err
		}
		// A dry run that would close nothing has no next step to name.
		if closed > 0 {
			printNext(stdout, "down "+resolved.Name+asSuffix(opts))
		}
		return nil
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runOperation(ctx, stderr, nil, func() error {
		_, err := l.Down(members, selfPane, false)
		return err
	})
}

// runStatus reports each member's and worker's live state, one "name: state"
// line each, then the judgement: who is waiting on the human, or that nobody is
// working. It only reads Herdr, so it runs outside a herdr-managed pane too.
func runStatus(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	members, err := cfg.Select("")
	if err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	rows, err := newLauncher(cfg, opts.Timeout, stdout).Status(members)
	if err != nil {
		return err
	}
	var blocked []string
	working := 0
	for _, row := range rows {
		state := row.State
		if !row.Live {
			state = i18n.T("status.not_live")
		}
		if row.Worker {
			state += i18n.T("status.worker_suffix")
		}
		field(stdout, row.Name, state)
		if !row.Live {
			continue
		}
		switch row.State {
		case "working":
			working++
		case "blocked":
			blocked = append(blocked, row.Name)
		}
	}
	switch {
	case len(blocked) > 0:
		fmt.Fprintln(stdout, i18n.Tf("status.human_waiting", strings.Join(blocked, i18n.T("list.separator"))))
	case working == 0:
		fmt.Fprintln(stdout, i18n.T("status.nobody_working"))
	}
	return nil
}

func runRefresh(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := cfg.Select(memberName)
	if err != nil {
		return err
	}
	if opts.Dry {
		members := make([]config.Member, len(selected))
		for i, member := range selected {
			members[i] = *member
		}
		// --dry stops nothing, but it shows the same refusal the real refresh
		// would, so it reads the live agents too (like down --dry).
		if err := newLauncher(cfg, opts.Timeout, stdout).RefreshDry(selected, os.Getenv("HERDR_PANE_ID")); err != nil {
			return err
		}
		next := "refresh " + memberName
		if opts.All {
			next = "refresh --all"
		}
		next += asSuffix(opts)
		return dryPlan(stdout, env, resolved, cfg, "restart", members, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Refresh(selected, os.Getenv("HERDR_PANE_ID")) })
}

func runReprompt(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := cfg.Select(memberName)
	if err != nil {
		return err
	}
	if opts.Dry {
		members := make([]config.Member, len(selected))
		for i, member := range selected {
			members[i] = *member
		}
		next := "reprompt " + memberName
		if opts.All {
			next = "reprompt --all"
		}
		next += asSuffix(opts)
		return dryPlan(stdout, env, resolved, cfg, "reprompt", members, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Reprompt(selected) })
}

func runHire(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	worker, notes, err := spawn.Plan(cfg, opts.Name, opts.Profile, opts.Purpose, opts.Task, opts.From)
	if err != nil {
		return err
	}
	for _, note := range notes {
		fmt.Fprintf(stderr, i18n.T("note.spawn_outside"), note)
	}
	if opts.Dry {
		// A hire has no obvious next step: the worker is named by the member
		// that hires it, so only the plan is shown.
		return dryPlan(stdout, env, resolved, cfg, "spawn", []config.Member{*worker}, opts, "")
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, resolved, cfg, env)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Hire(worker) })
}

// dryPlan is the --dry output: the same plan as JSON with --json, and a
// readable form (the prompt verbatim) by default. next, when not empty, is the
// command the plan moves on to, printed last and only in the readable form.
func dryPlan(stdout io.Writer, env team.Env, resolved *team.Resolved, cfg *config.Config, mode string, members []config.Member, opts *options.Options, next string) error {
	if opts.JSON {
		return printPlanJSON(stdout, mode, cfg, members, opts.Timeout)
	}
	printPlanText(stdout, mode, resolved, cfg, members, opts.Timeout, env)
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

// printHeader is the short provenance every team command shows: the team, its
// version, the layer in use, and the files the effective values came from. The
// per-member list is left to show and the plan.
func printHeader(w io.Writer, resolved *team.Resolved, cfg *config.Config, env team.Env) {
	field(w, label("team"), cfg.Team)
	field(w, label("version"), strconv.Itoa(cfg.Version))
	field(w, label("layer"), cfg.Layer)
	field(w, label("team_file"), env.Display(cfg.TeamYAML.Path))
	if resolved.GlobalTeamYAML != "" && resolved.LocalTeamYAML != "" {
		switch cfg.Layer {
		case source.LayerLocal:
			field(w, label("note"), i18n.Tf("note.local_overrides", env.Display(resolved.GlobalTeamYAML)))
		case source.LayerGlobal:
			field(w, label("note"), i18n.Tf("note.global_used", env.Display(resolved.LocalTeamYAML)))
		}
	}
	switch {
	case cfg.ProfilesYAML == "":
		field(w, label("profiles"), i18n.T("value.none"))
	case cfg.UsesProfiles():
		field(w, label("profiles"), displayPaths(cfg.ProfilesYAML, env))
	default:
		field(w, label("profiles"), i18n.Tf("value.unused", displayPaths(cfg.ProfilesYAML, env)))
	}
}

// printMember is one seat: what it runs, where it runs, and the files its
// prompt was built from. With prompt, the expanded prompt is printed verbatim
// after the fields, so up --dry shows exactly what the agent would receive.
func printMember(w io.Writer, cfg *config.Config, member *config.Member, env team.Env, prompt bool) {
	field(w, label("member"), member.Name)
	indentField(w, 1, label("role"), member.Role)
	indentField(w, 1, label("kind"), member.Kind)
	if member.Profile != "" {
		indentField(w, 1, label("profile"),
			i18n.Tf("value.profile_with_path", member.Profile, env.Display(cfg.Profiles[member.Profile].Path)))
	}
	if len(member.Args) > 0 {
		indentField(w, 1, label("args"), strings.Join(member.Args, " "))
	}
	if len(member.Env) > 0 {
		indentField(w, 1, label("env"), envSummary(member.Env))
	}
	indentField(w, 1, label("cwd"), env.Display(member.Cwd))
	if member.MaySpawn != nil {
		separator := i18n.T("list.separator")
		indentField(w, 1, label("may_spawn_purposes"), strings.Join(member.MaySpawn.Purposes, separator))
		indentField(w, 1, label("may_spawn_profiles"), strings.Join(member.MaySpawn.Profiles, separator))
	}
	if member.HasRoleSource {
		indentField(w, 1, label("role_file"), env.Display(member.RoleSource.Path))
	} else {
		indentField(w, 1, label("role_file"), i18n.T("value.none"))
	}
	if len(member.DocsRead) > 0 {
		for _, doc := range member.DocsRead {
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
	for _, line := range strings.Split(strings.TrimRight(prompt, "\n"), "\n") {
		if line == "" {
			fmt.Fprint(w, "\n")
			continue
		}
		fmt.Fprintf(w, "    %s\n", line)
	}
}

// printPlanText is the readable --dry plan. It is the same information as the
// JSON, with each member's prompt shown as the text it is.
func printPlanText(w io.Writer, mode string, resolved *team.Resolved, cfg *config.Config, members []config.Member, timeout int, env team.Env) {
	field(w, label("mode"), mode)
	printHeader(w, resolved, cfg, env)
	field(w, label("timeout"), strconv.Itoa(timeout))
	for i := range members {
		printMember(w, cfg, &members[i], env, true)
	}
}

// printPlanJSON is the --dry output with --json: the expanded members as JSON.
func printPlanJSON(w io.Writer, mode string, cfg *config.Config, members []config.Member, timeout int) error {
	plans := make([]map[string]any, len(members))
	for i := range members {
		plans[i] = members[i].ToPlan()
	}
	plan := map[string]any{
		"mode":               mode,
		"team":               cfg.Team,
		"startup_timeout_ms": timeout,
		"members":            plans,
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

func newLauncher(cfg *config.Config, timeout int, out io.Writer) *launcher.Launcher {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return launcher.New(cfg, herdr.New(port), timeout, out)
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
