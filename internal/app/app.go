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
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/options"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	opts, err := options.Parse(argv)
	if err != nil {
		if options.IsHelp(err) {
			fmt.Fprint(stdout, options.Usage)
			return 0
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if opts.Kind == options.Help {
		fmt.Fprint(stdout, opts.HelpText)
		return 0
	}

	env, err := team.NewEnv()
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if opts.Dir != "" {
		env, err = env.WithProject(opts.Dir)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	}
	if err := dispatch(ctx, env, opts, stdout, stderr); err != nil {
		var reported reportedError
		if errors.As(err, &reported) {
			return reported.code
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func dispatch(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	switch opts.Kind {
	case options.Ls:
		return runLs(env, opts, stdout)
	case options.Show:
		return runShow(env, opts, stdout)
	case options.Up:
		return runUp(ctx, env, opts, stdout, stderr)
	case options.Refresh:
		return runRefresh(ctx, env, opts, stdout, stderr)
	case options.Hire:
		return runHire(ctx, env, opts, stdout, stderr)
	default:
		return fmt.Errorf("unhandled command")
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
	fmt.Fprintf(stdout, "チーム一覧（ローカル: %s / グローバル: %s）\n", env.LocalDir, env.GlobalDir)
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
				note = "（ローカルに上書きされている）"
			}
		case "local":
			if !row.Local {
				continue
			}
			label, path = "local", row.LocalPath
			if row.Global {
				note = "（グローバルにもある）"
			}
		default:
			switch {
			case row.Local && row.Global:
				label, path = "local（global を上書き）", row.LocalPath
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
		fmt.Fprintln(stdout, "  （見えるチームがありません）")
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
	fmt.Fprintf(stdout, "層: %s\n", cfg.Layer)
	fmt.Fprintf(stdout, "  team.yaml : %s\n", cfg.TeamYAML.Display())
	if resolved.GlobalTeamYAML != "" && resolved.LocalTeamYAML != "" {
		switch cfg.Layer {
		case source.LayerLocal:
			fmt.Fprintf(stdout, "  グローバル: %s （ローカルが上書き）\n", resolved.GlobalTeamYAML)
		case source.LayerGlobal:
			fmt.Fprintf(stdout, "  ローカル: %s （グローバルを使用）\n", resolved.LocalTeamYAML)
		}
	}
	if cfg.ProfilesYAML == "" {
		fmt.Fprintln(stdout, "profiles: なし（members が kind/args を直に書いている場合のみ可）")
	} else if cfg.UsesProfiles() {
		fmt.Fprintf(stdout, "profiles: %s\n", cfg.ProfilesYAML)
	} else {
		fmt.Fprintf(stdout, "profiles: %s（どの担当も使っていません）\n", cfg.ProfilesYAML)
	}

	fmt.Fprintln(stdout, "文書（チームが宣言した順）:")
	if len(cfg.Docs) == 0 {
		fmt.Fprintln(stdout, "  （なし）")
	}
	for i, doc := range cfg.Docs {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, doc.Display())
	}

	fmt.Fprintln(stdout, "名簿（担当ごとに実際に読むもの）:")
	for i := range cfg.Members {
		member := &cfg.Members[i]
		engineLabel := fmt.Sprintf("kind=%s", member.Kind)
		if member.Profile != "" {
			engineLabel = fmt.Sprintf("profile=%s kind=%s", member.Profile, member.Kind)
		}
		fmt.Fprintf(stdout, "  - %-10s 役割=%s  %s\n", member.Name, member.Role, engineLabel)
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
			fmt.Fprintf(stdout, "      役割ファイル: %s\n", member.RoleSource.Display())
		} else {
			fmt.Fprintln(stdout, "      役割ファイル: なし")
		}
		if len(member.DocsRead) > 0 {
			fmt.Fprintf(stdout, "      読み込む文書: %s\n", displaysOf(member.DocsRead))
		} else {
			fmt.Fprintln(stdout, "      読み込む文書: なし")
		}
	}
	return nil
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
		fmt.Fprintf(stderr, "Note: %s (spawning anyway)\n", note)
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
		fmt.Fprintln(stderr, "Interrupted.")
		l.ReportFailure(stderr)
		return reportedError{err: context.Canceled, code: 130}
	}
}

// printResolution says which layer's which file each value and document came
// from, per member. Only documents a member actually read are listed.
func printResolution(w io.Writer, resolved *team.Resolved, cfg *config.Config) {
	fmt.Fprintf(w, "hergy: team %q 層=%s\n", cfg.Team, cfg.Layer)
	fmt.Fprintf(w, "  team.yaml : %s\n", cfg.TeamYAML.Display())
	if resolved.GlobalTeamYAML != "" && resolved.LocalTeamYAML != "" {
		switch cfg.Layer {
		case source.LayerLocal:
			fmt.Fprintf(w, "  グローバルにも同名チームがあります（ローカルが上書き）: %s\n", resolved.GlobalTeamYAML)
		case source.LayerGlobal:
			fmt.Fprintf(w, "  ローカルにも同名チームがあります（グローバルを使用）: %s\n", resolved.LocalTeamYAML)
		}
	}
	switch {
	case cfg.ProfilesYAML == "":
		fmt.Fprintln(w, "  profiles  : なし")
	case cfg.UsesProfiles():
		fmt.Fprintf(w, "  profiles  : %s\n", cfg.ProfilesYAML)
	default:
		fmt.Fprintf(w, "  profiles  : %s（どの担当も使っていません）\n", cfg.ProfilesYAML)
	}
	fmt.Fprintln(w, "担当ごとの出どころ:")
	for i := range cfg.Members {
		member := &cfg.Members[i]
		fmt.Fprintf(w, "  %s 役割=%s\n", member.Name, member.Role)
		if member.HasRoleSource {
			fmt.Fprintf(w, "    役割ファイル: %s\n", member.RoleSource.Display())
		} else {
			fmt.Fprintln(w, "    役割ファイル: なし")
		}
		if len(member.DocsRead) > 0 {
			fmt.Fprintf(w, "    文書: %s\n", displaysOf(member.DocsRead))
		} else {
			fmt.Fprintln(w, "    文書: なし")
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
	fmt.Fprintf(stderr, "Error: %v\n", err)
	l.ReportFailure(stderr)
	return reportedError{err: err, code: 1}
}

// requireHerdrPane refuses to touch Herdr outside a Herdr-managed shell pane.
func requireHerdrPane() error {
	if os.Getenv("HERDR_ENV") != "1" || os.Getenv("HERDR_PANE_ID") == "" {
		return fmt.Errorf("Herdr 管理下のシェルペインで実行してください（HERDR_ENV=1 と HERDR_PANE_ID が必要です）")
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
