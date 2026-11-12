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

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/newteam"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/spawn"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
	"github.com/aq2bq/hergy/internal/version"
)

// reportedError marks a failure whose detail was already printed, and carries
// the exit code (1 for a failure, 130 for an interrupt).
type reportedError struct {
	err  error
	code int
}

func (e reportedError) Error() string { return e.err.Error() }

func (e reportedError) Unwrap() error { return e.err }

// Run executes one command line and returns the process exit code.
func Run(argv []string, stdout, stderr io.Writer) int {
	// Load the locales and check them before anything can print. The failures
	// below happen before a language is in use, so they keep a plain prefix.
	if err := i18n.Init(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	// The global config.yaml names the language, but the usage (help, version,
	// cheat) does not stand on it: a person reads the usage to get out of a
	// broken file, so it prints anyway and says the file is broken at its end
	// (d-dfdd).
	ok, problem := chooseLanguage(argv, stderr)
	if !ok {
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
		if problem != nil {
			fmt.Fprintf(stdout, "\n%s\n", i18n.Tf("usage.config_unreadable", problem))
		}
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
	// Every other command stands on the settings, so a broken config.yaml stops
	// it here, as it always has. validate is the one that reports the file
	// itself, and the usage above has already answered (d-dfdd).
	if problem != nil && opts.Kind != options.Validate {
		fail(stderr, problem)
		return 1
	}

	env, err := team.NewEnv()
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if opts.Dir != "" {
		project, pathErr := domainplace.NewProject(opts.Dir)
		if pathErr != nil {
			fail(stderr, pathErr)
			return 1
		}
		env, err = env.WithProject(project)
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

// chooseLanguage picks the language this run prints in, and says what keeps the
// settings from standing: the global config.yaml cannot be read, or it names a
// language hergy does not know. The language is --lang, then the config.yaml,
// then English (d-dfdd). An unknown --lang is a command-line mistake and stops
// the run: ok is false and the reason is already on stderr.
func chooseLanguage(argv []string, stderr io.Writer) (ok bool, problem error) {
	configured, configErr := team.Lang()
	explicit := options.ScanLang(argv)
	lang, err := i18n.Resolve(explicit, configured)
	if err != nil && explicit != "" {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return false, nil
	}
	// An unknown language in config.yaml is one way the file is broken: the run
	// goes on in the default language and says so where it prints usage.
	unknownLang := err != nil
	if unknownLang {
		lang = i18n.Fallback
	}
	if err := i18n.Use(lang); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return false, nil
	}
	if configErr == nil && !unknownLang {
		return true, nil
	}
	// The reads above happen before a language is chosen, so their sentences are
	// in the default one. Read them again, so the line a person reads speaks the
	// same language as the usage it stands under.
	if _, configErr = team.Lang(); configErr != nil {
		return true, configErr
	}
	_, err = i18n.Resolve(explicit, configured)
	return true, err
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
	case options.Migrate:
		return runMigrate(env, opts, stdout)
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
	case options.Apply:
		return runApply(ctx, env, opts, stdout, stderr)
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
	case options.Turns:
		return runTurns(ctx, env, opts, stdout)
	case options.TurnDone:
		return runTurnDone(opts, os.Stdin)
	default:
		return i18n.Errorf("error.unhandled_command")
	}
}

func runLs(env team.Env, stdout io.Writer) error {
	// The running teams, grouped by the team definition they were booted from.
	// Outside herdr the tabs cannot be read; then only the definitions show.
	running := map[string][]string{}
	if tabs, agentsRead, err := ops.HerdrTabs(); err == nil {
		for _, team := range team.RunningTeams(tabs, ops.LaunchTeams(), agentsRead) {
			running[team.Definition] = append(running[team.Definition], team.Team.String())
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
	r, err := ops.ReadDefinition(cat, env.ProjectRoot, opts.Definition.DefinitionName().String(), opts.As)
	if err != nil {
		return err
	}
	// show is where a person reads a definition, so it prints the description
	// the team list and the start sheet read (d-5f55).
	if description := r.DescriptionLine(); description != "" {
		field(stdout, label("description"), description)
	}
	printHeader(stdout, r, env, false)
	docs := agreementDocs(r)
	if len(docs) == 0 {
		field(stdout, label("document"), i18n.T("value.none"))
	}
	for _, doc := range docs {
		field(stdout, label("document"), env.Display(doc.Path))
	}
	for i := range r.Starts {
		printMember(stdout, r, &r.Starts[i], env, false)
	}
	return nil
}

// agreementDocs is what every member of a definition reads besides its own role
// document: the team's own agreement (段 4). It is read off the built prompts, so
// the list a person sees cannot disagree with what the members were handed. A
// document two members read stands once.
func agreementDocs(r *ops.Resolved) []source.Doc {
	var docs []source.Doc
	seen := map[string]bool{}
	for i := range r.Starts {
		for _, doc := range r.Starts[i].Read {
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
	name := opts.Definition.String()
	if _, err := domainname.NewDefinitionName(name); err != nil {
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
	// A starter team's roles and profile are the ones the person uses only when
	// neither place holds any: a role or profile a place already holds is chosen
	// from, not written again into the place the definition goes to (r-54b8,
	// d-86b1). The role documents are the layer's, not the team's (K5), so one
	// that is written goes into the layer's roles/ directory.
	hasRoles := newteam.HasRoles(cat)
	hasProfiles, err := newteam.HasProfiles(cat)
	if err != nil {
		return err
	}
	written, err := newteam.Write(newteam.In{
		Catalog: cat, Layer: layer, Name: name, Lang: i18n.Language(),
		Files: files, Starter: true, StarterRoles: !hasRoles, StarterProfile: !hasProfiles,
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
		field(stdout, label("file"), cat.Display(path))
	}
	for _, path := range written.Roles {
		field(stdout, label("role_file"), cat.Display(path))
	}
	for _, path := range written.Kept {
		field(stdout, label("note"), i18n.Tf("new.role_kept", cat.Display(path)))
	}
	if written.Profile != "" {
		field(stdout, label("profile"), scaffold.ProfileName)
		field(stdout, label("file"), env.Display(written.Profile))
	}
	// A definition whose members name a role or profile no place holds is kept as
	// it stands: the files are the person's to write, and the sentence says which
	// member keys to fill from what the places hold (r-54b8).
	missing := false
	if len(written.Missing) > 0 {
		field(stdout, label("note"), i18n.Tf("new.roles_missing", joinNames(written.Missing), joinNames(existingRoleNames(cat))))
		missing = true
	}
	if written.ProfileMissing {
		field(stdout, label("note"), i18n.Tf("new.profile_missing", scaffold.ProfileName, joinNames(existingProfileNames(cat))))
		missing = true
	}
	if madeConfig {
		field(stdout, label("config"), env.Display(configPath))
	}
	if missing {
		printNext(stdout, "validate")
	} else {
		printNext(stdout, "up "+name+" --dry")
	}
	return nil
}

// joinNames writes the names one sentence lists, in the language in use.
func joinNames(names []string) string {
	return strings.Join(names, i18n.T("list.separator"))
}

// existingRoleNames is the role documents either place holds, by name: what a
// definition's members choose from (r-54b8).
func existingRoleNames(cat *catalog.Catalog) []string {
	var out []string
	for _, role := range cat.Roles() {
		if role.HasDocument {
			out = append(out, role.Name)
		}
	}
	return out
}

// existingProfileNames is the profiles either place defines, by name, sorted.
func existingProfileNames(cat *catalog.Catalog) []string {
	profiles, _, err := cat.Profiles()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(profiles))
	for name := range profiles {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
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
	name := strings.TrimSuffix(filepath.Base(path), ".md")
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
			if problem.Sev == "err" {
				problems = append(problems, problem)
			}
		}
		problems = append(problems, check.RoleProblem{
			Sev:  "warn",
			Text: i18n.Tf("validate.role_hidden", name),
		})
		return printRoleFindings(stdout, env, path, problems), roleSize{}, false
	}
	return printRoleFindings(stdout, env, path, doc.Problems),
		roleSize{name: cat.Display(path), chars: doc.Markdown.Chars}, true
}

// runValidate reads every settings file the way up does, shows one line per
// file, and fails when any of them is broken. It never touches Herdr. After the
// files, it counts what the rules carry: where they came from, why they are
// there, and whether anything shows they were kept. That count is not a break:
// it changes no exit code.
func runValidate(env team.Env, stdout io.Writer) error {
	cat := env.Catalog(source.Disk{})
	checked, problems := 0, 0
	var teams []teamSize
	var roles []roleSize
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
	reportLegacyProse(env, cat, report)
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
			printLedger(stdout, r.Team.Definition, ledger)
		}
	}
	// Every role document is its own unit, checked once however many teams use
	// it (d-a575): the section keeps a role document's own warnings out of the
	// team definitions' sections.
	for _, layer := range []string{source.LayerGlobal, source.LayerLocal} {
		dir := filepath.Join(cat.LayerDir(layer), "roles")
		matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
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
		if problem.Sev == "err" {
			stops++
		}
	}
	if stops > 0 {
		indentField(stdout, 1, label("result"), i18n.T("value.broken"))
	} else {
		indentField(stdout, 1, label("result"), i18n.T("value.ok"))
	}
	for _, problem := range problems {
		if problem.Sev == "err" {
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
		line(i18n.Tf("validate.size_team", team.name, i18n.Number(team.chars), i18n.Number(check.TeamCharLimit)))
	}
	for _, role := range roles {
		line(i18n.Tf("validate.size_role", role.name, i18n.Number(role.chars), i18n.Number(check.RoleCharLimit)))
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
	cfg, err := team.ReadConfig(string(text), path)
	if err != nil {
		return err
	}
	if _, err := i18n.Resolve("", cfg.Lang); err != nil {
		return err
	}
	env, err := team.NewEnv()
	if err != nil {
		return err
	}
	_, _, err = env.ProjectPaths(cfg)
	return err
}

// validateTeam builds one team from one layer the way up does.
func validateTeam(cat *catalog.Catalog, base domainplace.Project, name, layer string) (*ops.Resolved, error) {
	def, err := cat.Definition(name, layer)
	if err != nil {
		return nil, err
	}
	return ops.BuildResolved(cat, base, def, def.Name.String())
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
	r, err := ops.ReadDefinition(cat, env.ProjectRoot, opts.Definition.DefinitionName().String(), opts.As)
	if err != nil {
		return err
	}
	if opts.Dry {
		return dryPlan(stdout, env, r, "boot", r.Starts, opts, "up "+r.Team.Definition.Name.String()+asSuffix(opts))
	}
	release, err := source.LockTeamStart(r.Team.Definition.TeamYAML.String())
	if err != nil {
		return i18n.Errorf("web.rename.busy")
	}
	defer release()
	// A rename may have finished after the first read, before this lock.
	r, err = ops.ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, opts.Definition.DefinitionName().String(), opts.As)
	if err != nil {
		return err
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	printSignalNotes(stdout, r.Starts)
	l := launcherFor(r, opts.Timeout, stdout).WithWatcher(watcherCommand(r.Team.Name.String(), env.ProjectRoot))
	up := hergyCommand(opts)
	// A boot that stops partway says the same thing the screen's start sheet
	// says: the member, herdr's reason, what remains, and the command to go on
	// with, in the project it is typed in (d-5f02).
	say := func(err error) error {
		if launcher.IsUndelivered(err) {
			return err
		}
		failure := l.Failure(err)
		return errors.New(startFailureText(failure, filepath.Base(env.ProjectRoot.String()),
			failedStartCommand(failure, r.Team.Name.String(), up)))
	}
	err = runLaunch(ctx, stderr, l, say, func() error { return l.Boot(up) })
	if err != nil && !launcher.IsUndelivered(err) {
		return err
	}
	recordReceivedLaunch(stderr, env, r, launchlog.CommandUp, r.Starts, l, launchlog.Workspace{Label: l.WorkspaceLabel(), ID: l.WorkspaceID()})
	return err
}

// runDown turns the CLI word into an operation input and displays its result.
func runDown(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	target, err := ops.ResolveStop(opts.Target)
	if err != nil {
		return err
	}
	in := ops.StopInput{Env: env, Target: target, Timeout: opts.Timeout, SelfPane: ops.CurrentPane(),
		RequirePane: true, ManagedPane: os.Getenv("HERDR_ENV") == "1", Dry: opts.Dry, PlanOnly: opts.JSON}
	in.Observe = func(event ops.StopEvent) {
		if event.Plan != nil {
			printStopPlan(stdout, *event.Plan, env)
		}
		if event.Text != "" {
			fmt.Fprint(stdout, event.Text)
		}
		if event.Failure != nil {
			fail(stderr, event.Failure)
		}
	}
	var result ops.StopResult
	if target.IsWorker() {
		result, err = ops.StopWorker(ctx, in)
	} else {
		result, err = ops.StopTeam(ctx, in)
	}
	plan := result.Plan
	if opts.JSON && err == nil {
		return writeDownPlan(stdout, downPlan{Team: plan.Team.Team.String(), Members: plan.Members, Workers: plan.Workers, Leftovers: hqLeftoversOf(result.Leftovers)})
	}
	if result.HasLeftovers {
		printLeftovers(stdout, result.Leftovers, plan.Owner, opts.Dry)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprint(stderr, i18n.T("launcher.interrupted"))
			return reportedError{err: context.Canceled, code: 130}
		}
		if result.OperationErr != nil {
			if !errors.Is(err, result.OperationErr) {
				return err
			}
			return reportedError{err: err, code: 1}
		}
		return err
	}
	if opts.Dry && (plan.Worker != nil || result.Closed.Members+result.Closed.Workers+result.Closed.Watchers > 0) {
		name := plan.Team.Team.String()
		if plan.Worker != nil {
			name = plan.Worker.Herdr.String()
		}
		printNext(stdout, "down "+name)
	}
	return nil
}

func printStopPlan(stdout io.Writer, plan ops.StopPlan, env team.Env) {
	switch {
	case plan.Worker != nil:
		field(stdout, label("worker"), plan.Worker.Herdr.String())
		indentField(stdout, 1, label("team"), plan.Worker.Team.String())
	case plan.Stopped:
		field(stdout, label("team"), plan.Team.Team.String())
	case plan.Reason != nil:
		field(stdout, label("team"), startedFrom(plan.Team.Team.String(), plan.Team.Definition))
		detail := plan.Reason.Error()
		if i := strings.Index(detail, "\nnext:"); i >= 0 {
			detail = detail[:i]
		}
		indentField(stdout, 1, label("detail"), strings.Join(strings.Fields(detail), " "))
		indentField(stdout, 1, label("note"), i18n.T("down.closes_by_name"))
	default:
		printHeader(stdout, plan.Built, env, true)
	}
}

// runStatus reports each member's and worker's live state, one "name: state"
// line each, then the judgement: who is waiting on the human, or that nobody is
// working. It only reads Herdr, so it runs outside a herdr-managed pane too.
// With no team name it lists every running team, one block each.
func runStatus(env team.Env, opts *options.Options, stdout io.Writer) error {
	if opts.Team.String() == "" {
		return runStatusList(env, opts, stdout)
	}
	r, err := ops.ResolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return statusOfARunningTeam(opts, err)
	}
	rows, err := launcherFor(r, opts.Timeout, stdout).Status(r.Starts)
	if err != nil {
		return err
	}
	markScreens(rows)
	if opts.JSON {
		return writeStatusJSON(stdout, statusJSONTeam{
			Team: r.Team.Name.String(), Definition: r.Team.Definition.Name.String(), Members: statusJSONRows(rows),
		})
	}
	printHeader(stdout, r, env, true)
	for _, row := range rows {
		field(stdout, row.Name, row.Text())
	}
	printTurn(stdout, r.Team.Name.Observation())
	return printStatusJudgement(stdout, rows, "")
}

// watcherCommand is what a boot runs in the watcher's strip: this hergy, hergy
// turns <team>, in the project, in the language the command line runs in.
func watcherCommand(teamName string, projectRoot domainplace.Project) []string {
	return []string{ops.Executable(), "turns", teamName, "-C", projectRoot.String(), "--lang", i18n.Language()}
}

// printTurn is the status lines about the turn: who holds it, from whom, why
// the human does when hergy handed it over, whether the holder is being asked
// who its result is for, who waits for whose answer; and whether the watcher's
// pane is there at all (r-bc26, r-103d).
func printTurn(stdout io.Writer, teamName domainname.Seen) {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return
	}
	st, err := turns.Load(turns.Path(dir, teamName.String()), teamName)
	if err != nil {
		return
	}
	if st.Since.IsZero() {
		// The watcher has not seen this team: there is no turn to speak of.
		if !watcherPresent(teamName) {
			fmt.Fprint(stdout, i18n.Tf("status.no_watcher", teamName))
		}
		return
	}
	switch {
	case st.Holder == turns.Human:
		why := ""
		if st.Why != "" {
			why = i18n.T("status.turn_why_" + st.Why)
		}
		field(stdout, i18n.T("status.turn"), i18n.Tf("status.turn_human", st.From, why))
	case st.Asked[st.Holder].Seq != 0:
		field(stdout, i18n.T("status.turn"), i18n.Tf("status.turn_holder_asked", st.Holder, st.From))
	default:
		field(stdout, i18n.T("status.turn"), i18n.Tf("status.turn_holder", st.Holder, st.From))
	}
	if waits := st.Waits(); len(waits) > 0 {
		field(stdout, i18n.T("status.turn_waits"), strings.Join(waits, i18n.T("list.separator")))
	}
	if !watcherPresent(teamName) {
		fmt.Fprint(stdout, i18n.Tf("status.no_watcher", teamName))
	}
}

// watcherPresent is whether a pane named for the team's watcher is live in herdr.
func watcherPresent(teamName domainname.Seen) bool {
	panes, err := ops.HerdrPanes()
	if err != nil {
		return true // nothing to say when herdr cannot be asked
	}
	for _, pane := range panes {
		if pane.Label.String() == launcher.WatcherPane(teamName).String() {
			return true
		}
	}
	return false
}

// statusOfARunningTeam keeps the reason a running team's definition cannot be
// read, and adds the one thing that can still be done about it: down closes the
// team's panes by name, so a team is not stuck when its definition is out of
// reach (r-e494's follow-up).
func statusOfARunningTeam(opts *options.Options, reason error) error {
	running, err := ops.RunningTeams()
	if err != nil {
		return reason
	}
	if _, ok := team.FindRunning(running, opts.Team.String()); !ok {
		return reason
	}
	return errors.New(reason.Error() + "\n" + i18n.Tf("status.can_down", opts.Team.String()))
}

// runStatusList is status with no team name: every running team, with its team
// definition name, its counts, and the same judgement.
func runStatusList(env team.Env, opts *options.Options, stdout io.Writer) error {
	tabs, agents, agentsRead, err := ops.HerdrRead()
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
				teams = append(teams, statusJSONTeam{Team: entry.Name.String(), Definition: entry.Definition, Error: entry.Broken})
				continue
			}
			field(stdout, label("team"), startedFrom(entry.Name.String(), entry.Definition))
			if entry.Elsewhere {
				fmt.Fprintf(stdout, "  %s\n", i18n.T("status.definition_elsewhere"))
			} else {
				indentField(stdout, 1, label("detail"), entry.Broken)
			}
			continue
		}
		rows, err := launcherFor(r, opts.Timeout, stdout).Status(r.Starts)
		if err != nil {
			return err
		}
		markScreens(rows)
		if opts.JSON {
			teams = append(teams, statusJSONTeam{
				Team: r.Team.Name.String(), Definition: r.Team.Definition.Name.String(), Members: statusJSONRows(rows),
			})
			continue
		}
		field(stdout, label("team"), startedFrom(r.Team.Name.String(), r.Team.Definition.Name.String()))
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
	r, err := ops.ResolveTeam(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.Member(opts.Member)
	if err != nil {
		return err
	}
	return launcherFor(r, opts.Timeout, stdout).Focus(member)
}

// runNudge sends one member a short push to carry on, only when it is idle or
// done. --dry prints the message and sends nothing.
func runNudge(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolve := ops.ResolveTeam
	if opts.Dry {
		resolve = ops.ResolveTeamDry
	}
	r, err := resolve(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.Member(opts.Member)
	if err != nil {
		return err
	}
	body := opts.Body
	if body == "" {
		body = i18n.T("nudge.default")
	}
	from := domainname.Seen{}
	if !opts.Dry {
		from = senderName(r, ops.CurrentPane())
	}
	text := sendText(from.String(), body)
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("send.dry"), member.Name.String(), text)
		return nil
	}
	return launcherFor(r, opts.Timeout, stdout).Nudge(member, text)
}

// runStop interrupts one member's current turn. --dry prints what would stop and
// reads nothing.
func runStop(env team.Env, opts *options.Options, stdout io.Writer) error {
	resolve := ops.ResolveTeam
	if opts.Dry {
		resolve = ops.ResolveTeamDry
	}
	r, err := resolve(env.Catalog(source.Disk{}), env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	member, err := r.Member(opts.Member)
	if err != nil {
		return err
	}
	if opts.Dry {
		fmt.Fprintf(stdout, i18n.T("stop.dry"), member.Name.String())
		return nil
	}
	stopped, err := launcherFor(r, opts.Timeout, stdout).Stop(member, ops.CurrentPane())
	if err != nil {
		// esc twice did not end the turn. The next step is the same one a stop
		// that worked names: the member has to be started again.
		if _, ok := errors.AsType[launcher.StopNotStopped](err); ok {
			printNext(stdout, "refresh "+member.Name.String()+" --team "+r.Team.Name.String())
		}
		return err
	}
	if stopped {
		printNext(stdout, "refresh "+member.Name.String()+" --team "+r.Team.Name.String())
	}
	return nil
}

// sendText heads a message with who it is from, the shape a member's own message
// carries. An empty from is the human.
func sendText(from, body string) string {
	if from == "" {
		from = i18n.T("send.who_human")
	}
	return i18n.Tf("send.from", from) + " " + body
}

// senderName is which member of this team typed the command, read from the pane
// hergy runs in: the herdr name of the member whose agent runs there, or "" when
// the pane is not one of the team's members (the human).
func senderName(r *ops.Resolved, selfPane domainname.PaneID) domainname.Seen {
	if selfPane == "" {
		return domainname.Seen{}
	}
	agent, err := herdr.New(herdrcli.New(os.Getenv("HERGY_HERDR"))).Agent(selfPane)
	if err != nil {
		return domainname.Seen{}
	}
	for i := range r.Starts {
		if r.Starts[i].Name.String() == agent.Name.String() {
			return agent.Name
		}
	}
	return domainname.Seen{}
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
	r, err := ops.ResolveTeam(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := r.SelectStarts(memberName)
	if err != nil {
		return err
	}
	// A member the team has no pane for is added by apply now (d-36ef): refresh
	// restarts a member that is already running, or one whose pane is still
	// there at its shell. The check reads herdr the same way the real refresh
	// does, so --dry refuses what a real run would.
	newPanes, err := launcherFor(r, opts.Timeout, stdout).RefreshDry(selected, ops.CurrentPane())
	if err != nil {
		return err
	}
	if len(newPanes.Members) > 0 {
		shorts := make([]string, 0, len(newPanes.Members))
		for _, name := range newPanes.Members {
			shorts = append(shorts, domainname.ShortIn(name.Observation(), r.Team.Name.Observation()).String())
		}
		return i18n.Errorf("refresh.use_apply", strings.Join(shorts, i18n.T("list.separator")))
	}
	if opts.Dry {
		// --dry stops and makes nothing, but it shows the same refusal the real
		// refresh would, so it reads the live agents, tabs and panes too (like
		// down --dry).
		next := "refresh " + memberName
		if opts.All {
			next = "refresh --all"
		}
		next += " --team " + r.Team.Name.String()
		return dryPlan(stdout, env, r, "restart", selected, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := launcherFor(r, opts.Timeout, stdout)
	err = runLaunch(ctx, stderr, l, nil, func() error { return l.Refresh(selected, ops.CurrentPane()) })
	if err != nil && !launcher.IsUndelivered(err) {
		return err
	}
	recordReceivedLaunch(stderr, env, r, launchlog.CommandRefresh, selected, l, launchlog.Workspace{})
	return err
}

func runReprompt(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	resolve := ops.ResolveTeam
	if opts.Dry {
		resolve = ops.ResolveTeamDry
	}
	r, err := resolve(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	memberName := opts.Member
	if opts.All {
		memberName = ""
	}
	selected, err := r.SelectStarts(memberName)
	if err != nil {
		return err
	}
	if opts.Dry {
		next := "reprompt " + memberName
		if opts.All {
			next = "reprompt --all"
		}
		next += " --team " + r.Team.Name.String()
		return dryPlan(stdout, env, r, "reprompt", selected, opts, next)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	printHeader(stdout, r, env, true)
	l := launcherFor(r, opts.Timeout, stdout)
	sent := promptsToResend(cat, env.ProjectRoot, r, selected)
	err = runLaunch(ctx, stderr, l, nil, func() error { return l.Reprompt(sent) })
	if err != nil && !launcher.IsUndelivered(err) {
		return err
	}
	recordReceivedLaunch(stderr, env, r, launchlog.CommandReprompt, selected, l, launchlog.Workspace{})
	return err
}

func runHire(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	resolve := ops.ResolveTeam
	if opts.Dry {
		// A plan is made from the team definition alone: a team that is not
		// running can be planned for, and herdr is not read (r-fe31).
		resolve = ops.ResolveTeamDry
	}
	r, err := resolve(cat, env.ProjectRoot, opts)
	if err != nil {
		return err
	}
	worker, err := spawn.Plan(r.ModelTeamName(), r.Starts, r.Profiles, opts.Name, opts.Purpose, opts.Task, opts.From)
	if err == nil {
		ops.WireStart(worker, r.Team.Name.String())
		printSignalNotes(stdout, []definition.Start{*worker})
	}
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
	l := launcherFor(r, opts.Timeout, stdout)
	err = runLaunch(ctx, stderr, l, nil, func() error { return l.Hire(worker, opts.Task) })
	if err != nil && !launcher.IsUndelivered(err) {
		return err
	}
	// The worker is up, so the record says who added it (d-b4b9). The activity
	// screen reads the line; the launch itself does not stand on it.
	recordHire(stderr, env, r, *worker)
	return err
}

// dryPlan is the --dry output: the same plan as JSON with --json, and a
// readable form (the prompt verbatim) by default. next, when not empty, is the
// command the plan moves on to, printed last and only in the readable form.
func dryPlan(stdout io.Writer, env team.Env, r *ops.Resolved, mode string, members []definition.Start, opts *options.Options, next string) error {
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
// record and stops. say, when set, turns the error of a failure into the sentence
// a person reads before the record: a boot speaks the screen's own words
// (d-5f02), and the other operations keep their error.
func runLaunch(ctx context.Context, stderr io.Writer, l *launcher.Launcher, say func(error) error, fn func() error) error {
	run := fn
	if say != nil {
		run = func() error {
			err := fn()
			if err == nil {
				return nil
			}
			return say(err)
		}
	}
	return runOperation(ctx, stderr, l.ReportFailure, l.ReportInterrupted, run)
}

// runOperation runs one Herdr operation, and on Ctrl-C stops with 130. failure,
// when set, says where a failed operation stopped; interrupted, when set, says
// where an interrupted one stopped. The two differ: an interruption while hergy
// waits on a member's question adds how to go on.
func runOperation(ctx context.Context, stderr io.Writer, failure, interrupted func(io.Writer), fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			fail(stderr, err)
			if failure != nil {
				failure(stderr)
			}
			return reportedError{err: err, code: 1}
		}
		return nil
	case <-ctx.Done():
		fmt.Fprint(stderr, i18n.T("launcher.interrupted"))
		if interrupted != nil {
			interrupted(stderr)
		}
		return reportedError{err: context.Canceled, code: 130}
	}
}

// printHeader is the short provenance every team command shows: the team it
// runs as, its version, and the files the effective values came from. show
// prints the definition alone; a team command prints the team name, and the
// definition behind it only when the two differ (I2). The description is not
// here: a team command shows a running team, and the description is read where
// a person picks or reads a definition (d-5f55).
func printHeader(w io.Writer, r *ops.Resolved, env team.Env, withTeam bool) {
	def := r.Team.Definition
	if withTeam {
		// A team is called by its name. The definition it was booted from is
		// named only when the two differ (I2).
		field(w, label("team"), startedFrom(r.Team.Name.String(), def.Name.String()))
	} else {
		field(w, label("definition"), def.Name.String())
	}
	field(w, label("version"), strconv.Itoa(def.Version))
	field(w, label("team_file"), env.Display(def.TeamYAML.String()))
	switch {
	case r.ProfilesYAML == "":
		field(w, label("profiles"), i18n.T("value.none"))
	case r.UsesProfiles():
		field(w, label("profiles"), displayPaths(r.ProfilesYAML, env))
	default:
		field(w, label("profiles"), i18n.Tf("value.unused", displayPaths(r.ProfilesYAML, env)))
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
func printMember(w io.Writer, r *ops.Resolved, member *definition.Start, env team.Env, prompt bool) {
	field(w, label("member"), member.Name.String())
	indentField(w, 1, label("role"), member.Role)
	indentField(w, 1, label("kind"), member.Kind)
	if member.Profile != "" {
		if value, ok := r.Profiles[member.Profile]; ok {
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
func printPlanText(w io.Writer, mode string, r *ops.Resolved, members []definition.Start, timeout int, env team.Env) {
	field(w, label("mode"), mode)
	printHeader(w, r, env, true)
	field(w, label("timeout"), strconv.Itoa(timeout))
	for i := range members {
		printMember(w, r, &members[i], env, true)
	}
}

// printPlanJSON is the --dry output with --json: the expanded members as JSON.
func printPlanJSON(w io.Writer, mode string, r *ops.Resolved, members []definition.Start, timeout int) error {
	plans := make([]map[string]any, len(members))
	for i := range members {
		plans[i] = members[i].ToPlan()
	}
	plan := map[string]any{
		"mode":               mode,
		"team":               r.Team.Name,
		"startup_timeout_ms": timeout,
		"members":            plans,
	}
	if def := r.Team.Definition; def != nil && len(def.Graph.Turns) > 0 {
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

// hergyCommand is a command line as it was typed, program name first. A message
// that says to run the same command again names it as it was typed, so a person
// can copy it.
func hergyCommand(opts *options.Options) string {
	return strings.Join(append([]string{"hergy"}, opts.Argv...), " ")
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

// requireHerdrPane refuses to touch Herdr outside a Herdr-managed shell pane.
func requireHerdrPane() error {
	if os.Getenv("HERDR_ENV") != "1" || ops.CurrentPane() == "" {
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

func reportLegacyProse(env team.Env, cat *catalog.Catalog, report func(string, error)) {
	for _, dir := range []string{env.GlobalDir, env.LocalDir} {
		for _, pattern := range []string{"roles/*.yaml", "teams/*/agreement.yaml"} {
			paths, _ := filepath.Glob(filepath.Join(dir, pattern))
			for _, path := range paths {
				report(path, i18n.Errorf("error.legacy_prose", cat.Display(path)))
			}
		}
	}
}

func seenStrings(names []domainname.Seen) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = n.String()
	}
	return out
}
