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
	"strings"

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
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	// validate must still run when the global config.yaml is broken: it is one
	// of the files validate reports. Every other command keeps stopping here.
	validating := options.ScanCommand(argv) == "validate"
	configured, err := team.Lang()
	if err != nil && !validating {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	explicit := options.ScanLang(argv)
	lang, err := i18n.Resolve(explicit, configured)
	if err != nil {
		// An unknown --lang is a command-line mistake and always stops. For
		// validate, an unknown language in config.yaml is one of the breaks it
		// reports: fall back to the default language and go on.
		if !validating || explicit != "" {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		lang = i18n.Fallback
	}
	if err := i18n.Use(lang); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
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
	case options.Refresh:
		return runRefresh(ctx, env, opts, stdout, stderr)
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
	cfg, err := env.BuildConfig(resolved, source.Disk{})
	if err != nil {
		return nil, nil, err
	}
	return resolved, cfg, nil
}

func runLs(env team.Env, opts *options.Options, stdout io.Writer) error {
	rows := env.List()
	fmt.Fprintf(stdout, i18n.T("ls.header"), env.LocalDir, env.GlobalDir)
	shown := 0
	for _, row := range rows {
		var label, path, note string
		switch opts.Layer {
		case "global":
			if !row.Global {
				continue
			}
			label, path = "global", row.GlobalPath
			if row.Local {
				note = i18n.T("ls.note.overridden")
			}
		case "local":
			if !row.Local {
				continue
			}
			label, path = "local", row.LocalPath
			if row.Global {
				note = i18n.T("ls.note.also_global")
			}
		default:
			switch {
			case row.Local && row.Global:
				label, path = i18n.T("ls.label.both"), row.LocalPath
			case row.Local:
				label, path = "local", row.LocalPath
			default:
				label, path = "global", row.GlobalPath
			}
		}
		shown++
		fmt.Fprintf(stdout, "  %-20s %-24s %s %s\n", row.Name, label, path, note)
	}
	if shown == 0 {
		fmt.Fprint(stdout, i18n.T("ls.empty"))
	}
	return nil
}

func runShow(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolved, cfg, err := resolveAndBuild(env, opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "team: %s\n", cfg.Team)
	fmt.Fprintf(stdout, "version: %d\n", cfg.Version)
	fmt.Fprintf(stdout, i18n.T("show.layer"), cfg.Layer)
	fmt.Fprintf(stdout, "  team.yaml : %s\n", cfg.TeamYAML.Display())
	if resolved.GlobalTeamYAML != "" && resolved.LocalTeamYAML != "" {
		switch cfg.Layer {
		case source.LayerLocal:
			fmt.Fprintf(stdout, i18n.T("show.global_overridden"), resolved.GlobalTeamYAML)
		case source.LayerGlobal:
			fmt.Fprintf(stdout, i18n.T("show.local_used"), resolved.LocalTeamYAML)
		}
	}
	if cfg.ProfilesYAML == "" {
		fmt.Fprint(stdout, i18n.T("show.profiles_none"))
	} else if cfg.UsesProfiles() {
		fmt.Fprintf(stdout, "profiles: %s\n", cfg.ProfilesYAML)
	} else {
		fmt.Fprintf(stdout, i18n.T("show.profiles_unused"), cfg.ProfilesYAML)
	}

	fmt.Fprint(stdout, i18n.T("show.docs_header"))
	if len(cfg.Docs) == 0 {
		fmt.Fprint(stdout, i18n.T("show.docs_none"))
	}
	for i, doc := range cfg.Docs {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, doc.Display())
	}

	fmt.Fprint(stdout, i18n.T("show.roster_header"))
	for i := range cfg.Members {
		member := &cfg.Members[i]
		engineLabel := fmt.Sprintf("kind=%s", member.Kind)
		if member.Profile != "" {
			engineLabel = fmt.Sprintf("profile=%s kind=%s", member.Profile, member.Kind)
		}
		fmt.Fprintf(stdout, i18n.T("show.member_line"), member.Name, member.Role, engineLabel)
		if len(member.Args) > 0 {
			fmt.Fprintf(stdout, "      args=%s\n", strings.Join(member.Args, " "))
		}
		if len(member.Env) > 0 {
			fmt.Fprintf(stdout, "      env=%s\n", envSummary(member.Env))
		}
		fmt.Fprintf(stdout, "      cwd=%s\n", member.Cwd)
		if member.MaySpawn != nil {
			fmt.Fprintf(stdout, "      may_spawn: purposes=[%s] profiles=[%s]\n",
				strings.Join(member.MaySpawn.Purposes, ", "), strings.Join(member.MaySpawn.Profiles, ", "))
		}
		if member.HasRoleSource {
			fmt.Fprintf(stdout, i18n.T("show.role_file"), member.RoleSource.Display())
		} else {
			fmt.Fprint(stdout, i18n.T("show.role_file_none"))
		}
		if len(member.DocsRead) > 0 {
			fmt.Fprintf(stdout, i18n.T("show.docs_read"), displaysOf(member.DocsRead))
		} else {
			fmt.Fprint(stdout, i18n.T("show.docs_read_none"))
		}
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
		return i18n.Errorf("error.new_exists", name, dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	files, err := scaffold.Team(i18n.Language(), name)
	if err != nil {
		return err
	}
	if err := writeScaffold(dir, files); err != nil {
		return err
	}
	fmt.Fprintf(stdout, i18n.T("new.created"), name, layer, dir)
	for _, file := range files {
		fmt.Fprintf(stdout, i18n.T("new.file"), filepath.Join(dir, file.Path))
	}
	fmt.Fprint(stdout, i18n.Tf("new.next", name))
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
		if err == nil {
			fmt.Fprint(stdout, i18n.Tf("validate.ok", path))
			return
		}
		problems++
		msg := err.Error()
		// Some team-file errors do not name the file; the file-level parser
		// errors already do. Show the location once either way, and keep the
		// whole reason on one line.
		if !strings.Contains(msg, path) {
			msg = fmt.Sprintf("%s: %s", path, msg)
		}
		msg = strings.Join(strings.Fields(msg), " ")
		fmt.Fprint(stdout, i18n.Tf("validate.bad", msg))
	}

	if path := filepath.Join(env.GlobalDir, "config.yaml"); isFile(path) {
		report(path, validateConfig(path))
	}
	if path := filepath.Join(env.GlobalDir, "profiles.yaml"); isFile(path) {
		report(path, validateProfiles(path))
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

	fmt.Fprint(stdout, i18n.Tf("validate.summary", checked, problems))
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

// validateProfiles reads the global profiles.yaml the way up does.
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
	_, err = env.BuildConfig(resolved, source.Disk{})
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
		// --dry touches no Herdr: provenance to stderr, the plan alone on stdout.
		printResolution(stderr, resolved, cfg)
		return printPlan(stdout, "boot", cfg, cfg.Members, opts.Timeout)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printResolution(stdout, resolved, cfg)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, l.Boot)
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
		printResolution(stderr, resolved, cfg)
		members := make([]config.Member, len(selected))
		for i, member := range selected {
			members[i] = *member
		}
		return printPlan(stdout, "reseed", cfg, members, opts.Timeout)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printResolution(stdout, resolved, cfg)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Refresh(selected) })
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
		printResolution(stderr, resolved, cfg)
		return printPlan(stdout, "spawn", cfg, []config.Member{*worker}, opts.Timeout)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printResolution(stdout, resolved, cfg)
	l := newLauncher(cfg, opts.Timeout, stdout)
	return runLaunch(ctx, stderr, l, func() error { return l.Hire(worker) })
}

// runLaunch runs one Herdr operation, and on Ctrl-C prints the failure report
// and stops with 130.
func runLaunch(ctx context.Context, stderr io.Writer, l *launcher.Launcher, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			return reportLaunch(stderr, l, err)
		}
		return nil
	case <-ctx.Done():
		fmt.Fprint(stderr, i18n.T("launcher.interrupted"))
		l.ReportFailure(stderr)
		return reportedError{err: context.Canceled, code: 130}
	}
}

// printResolution says which layer's which file each value and document came
// from, per member. Only documents a member actually read are listed.
func printResolution(w io.Writer, resolved *team.Resolved, cfg *config.Config) {
	fmt.Fprintf(w, i18n.T("resolution.header"), cfg.Team, cfg.Layer)
	fmt.Fprintf(w, "  team.yaml : %s\n", cfg.TeamYAML.Display())
	if resolved.GlobalTeamYAML != "" && resolved.LocalTeamYAML != "" {
		switch cfg.Layer {
		case source.LayerLocal:
			fmt.Fprintf(w, i18n.T("resolution.global_overridden"), resolved.GlobalTeamYAML)
		case source.LayerGlobal:
			fmt.Fprintf(w, i18n.T("resolution.local_used"), resolved.LocalTeamYAML)
		}
	}
	switch {
	case cfg.ProfilesYAML == "":
		fmt.Fprint(w, i18n.T("resolution.profiles_none"))
	case cfg.UsesProfiles():
		fmt.Fprintf(w, "  profiles  : %s\n", cfg.ProfilesYAML)
	default:
		fmt.Fprintf(w, i18n.T("resolution.profiles_unused"), cfg.ProfilesYAML)
	}
	fmt.Fprint(w, i18n.T("resolution.members_header"))
	for i := range cfg.Members {
		member := &cfg.Members[i]
		fmt.Fprintf(w, i18n.T("resolution.member_line"), member.Name, member.Role)
		if member.HasRoleSource {
			fmt.Fprintf(w, i18n.T("resolution.role_file"), member.RoleSource.Display())
		} else {
			fmt.Fprint(w, i18n.T("resolution.role_file_none"))
		}
		if len(member.DocsRead) > 0 {
			fmt.Fprintf(w, i18n.T("resolution.docs"), displaysOf(member.DocsRead))
		} else {
			fmt.Fprint(w, i18n.T("resolution.docs_none"))
		}
	}
}

// printPlan is the --dry output: the expanded members as JSON.
func printPlan(w io.Writer, mode string, cfg *config.Config, members []config.Member, timeout int) error {
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

func newLauncher(cfg *config.Config, timeout int, out io.Writer) *launcher.Launcher {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	return launcher.New(cfg, herdr.New(port), timeout, out)
}

// reportLaunch prints where the operation stopped, then the error.
func reportLaunch(stderr io.Writer, l *launcher.Launcher, err error) error {
	fail(stderr, err)
	l.ReportFailure(stderr)
	return reportedError{err: err, code: 1}
}

// requireHerdrPane refuses to touch Herdr outside a Herdr-managed shell pane.
func requireHerdrPane() error {
	if os.Getenv("HERDR_ENV") != "1" || os.Getenv("HERDR_PANE_ID") == "" {
		return i18n.Errorf("error.not_under_herdr")
	}
	return nil
}

func displaysOf(docs []source.Doc) string {
	displays := make([]string, len(docs))
	for i, doc := range docs {
		displays[i] = doc.Display()
	}
	return strings.Join(displays, ", ")
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
