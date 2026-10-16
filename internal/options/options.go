// Package options turns the hergy command line into a command and its flags.
package options

import (
	"errors"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/scaffold"
)

// Kind is the command.
type Kind int

const (
	// Up boots (or, with --dry, prints) a team.
	Up Kind = iota
	// Down closes the panes of a live team's members.
	Down
	// Status reports each member's and worker's live state, and who is stuck.
	Status
	// Refresh restarts a member: it ends the agent and starts it again.
	Refresh
	// Reprompt re-submits a bootstrap prompt, keeping the conversation.
	Reprompt
	// Hire starts one worker.
	Hire
	// Ls lists visible teams.
	Ls
	// Show prints the effective config and where it came from.
	Show
	// New writes a starter team's files.
	New
	// Validate checks every settings file and reports each broken one.
	Validate
	// Serve opens the web UI: a small server on 127.0.0.1 for this machine.
	Serve
	// Help prints usage.
	Help
	// Version prints which hergy this is.
	Version
	// Cheat prints a cheatsheet: for people, or, with --ai, for an agent.
	Cheat

	// kindEnd is not a command: it ends the list, so a test can count the kinds.
	kindEnd
)

// Options is a parsed command line.
type Options struct {
	Kind Kind

	Team   string // up/show/new: a team definition name; down/status/refresh/reprompt/hire: a team name
	Member string // refresh/reprompt positional
	All    bool   // refresh/reprompt --all
	Name   string // hire positional
	As     string // up only: the team name to boot under (defaults to the team definition name)

	Profile string
	Purpose string
	Task    string
	From    string

	Dry     bool
	JSON    bool   // --json: print the --dry plan as JSON instead of text
	AI      bool   // --ai: cheat prints the agent cheatsheet
	Layer   string // "", "global", "local"
	Dir     string // -C: the project root to resolve against
	Timeout int    // milliseconds
	Lang    string // --lang: the language for this run
	Port    int    // serve: the port to listen on (0: the operating system picks one)
	NoOpen  bool   // serve --no-open: print the URL and open no browser

	HelpText string

	seen map[string]bool
}

const (
	// DefaultTimeoutMS is the startup timeout when --timeout is not given.
	DefaultTimeoutMS = 30000
	// DefaultPort is where hergy serve listens when --port is not given.
	DefaultPort  = 8787
	minTimeoutMS = 3001
	maxTimeoutMS = 300000
	maxPort      = 65535
)

// Usage is the help text in the language in use.
func Usage() string { return i18n.T("usage") }

// ScanLang finds --lang in argv before the full parse, so messages printed
// during the parse already use the chosen language. The full parse still
// rejects a malformed --lang.
func ScanLang(argv []string) string {
	for i := range argv {
		arg := argv[i]
		if arg == "--lang" {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		if value, ok := strings.CutPrefix(arg, "--lang="); ok {
			return value
		}
	}
	return ""
}

// ScanCommand returns the command word, argv[0], before the full parse. It lets
// the caller recognize a command whose own settings file may be broken, so the
// command can run instead of stopping on that file.
func ScanCommand(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return argv[0]
}

// Parse reads argv (without the program name).
func Parse(argv []string) (*Options, error) {
	if len(argv) == 0 {
		return &Options{Kind: Help, HelpText: Usage()}, nil
	}
	cmd := argv[0]
	o := &Options{Timeout: DefaultTimeoutMS, Port: DefaultPort, seen: map[string]bool{}}
	switch cmd {
	case "help", "-h", "--help":
		return &Options{Kind: Help, HelpText: Usage()}, nil
	case "version", "--version":
		return &Options{Kind: Version}, nil
	}
	spec := specNamed(cmd)
	if spec == nil {
		return nil, i18n.Errorf("error.unknown_command", cmd)
	}
	o.Kind = spec.Kind

	positionals, err := parseFlags(argv[1:], o)
	if errors.Is(err, errHelp) {
		return &Options{Kind: Help, HelpText: CommandHelp(o.Kind)}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := assignPositionals(o, positionals); err != nil {
		return nil, err
	}
	if err := validate(o); err != nil {
		return nil, err
	}
	return o, nil
}

func parseFlags(args []string, o *Options) ([]string, error) {
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		value := ""
		hasValue := false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			value = name[eq+1:]
			name = name[:eq]
			hasValue = true
		}
		f := lookupFlag(name)
		if f == nil {
			return nil, i18n.Errorf("error.unknown_option", arg, commandName(o.Kind))
		}
		if f.Value != nil && !hasValue {
			i++
			if i >= len(args) {
				return nil, i18n.Errorf("error.flag_needs_value", f.Name)
			}
			value = args[i] //nolint:gosec // G602: i < len(args) is checked just above
		}
		if err := f.Set(o, value); err != nil {
			return nil, err
		}
		o.seen[f.Name] = true
	}
	return positionals, nil
}

func assignPositionals(o *Options, positionals []string) error {
	switch o.Kind {
	case Up, Down, Status, Show, New:
		if len(positionals) > 1 {
			return i18n.Errorf("error.too_many_args", positionals)
		}
		if len(positionals) == 1 {
			o.Team = positionals[0]
		}
	case Refresh, Reprompt:
		if len(positionals) > 1 {
			return i18n.Errorf("error.too_many_args", positionals)
		}
		if len(positionals) == 1 {
			o.Member = positionals[0]
		}
	case Hire:
		if len(positionals) != 1 {
			return i18n.Errorf("error.hire_needs_name")
		}
		o.Name = positionals[0]
	case Ls:
		if len(positionals) != 0 {
			return i18n.Errorf("error.ls_no_args", positionals)
		}
	case Validate:
		if len(positionals) != 0 {
			return i18n.Errorf("error.validate_no_args", positionals)
		}
	case Serve:
		if len(positionals) != 0 {
			return i18n.Errorf("error.serve_no_args", positionals)
		}
	case Cheat:
		if len(positionals) != 0 {
			return i18n.Errorf("error.cheat_no_args", positionals)
		}
	default:
		// help and version take no positionals; they are handled before parsing.
	}
	return nil
}

func validate(o *Options) error {
	if err := validateValues(o); err != nil {
		return err
	}
	return validateKind(o)
}

// validateValues checks what every command's flags must hold, whatever the
// command is.
func validateValues(o *Options) error {
	if o.seen["global"] && o.seen["local"] {
		return i18n.Errorf("error.global_local_conflict")
	}
	for name := range o.seen {
		if !allowed[o.Kind][name] {
			return i18n.Errorf("error.flag_not_allowed", name, commandName(o.Kind))
		}
	}
	if o.Timeout < minTimeoutMS || o.Timeout > maxTimeoutMS {
		return i18n.Errorf("error.timeout_range", minTimeoutMS, maxTimeoutMS)
	}
	if o.Port < 0 || o.Port > maxPort {
		return i18n.Errorf("error.port_range", maxPort)
	}
	if o.JSON && !o.Dry {
		return i18n.Errorf("error.json_needs_dry")
	}
	if o.As != "" && !scaffold.NamePattern.MatchString(o.As) {
		return i18n.Errorf("error.as_name_pattern", o.As)
	}
	return nil
}

// validateKind checks what only some commands need: the member a refresh or a
// reprompt acts on, the flags hire needs, and the name a show or a new needs.
func validateKind(o *Options) error {
	switch o.Kind {
	case Refresh, Reprompt:
		switch {
		case o.All && o.Member != "":
			return i18n.Errorf("error.member_and_all", commandName(o.Kind))
		case !o.All && o.Member == "":
			return i18n.Errorf("error.needs_member", commandName(o.Kind))
		}
	case Hire:
		return validateHire(o)
	case Show:
		if o.Team == "" {
			return i18n.Errorf("error.show_needs_team")
		}
	case New:
		if o.Team == "" {
			return i18n.Errorf("error.new_needs_name")
		}
	default:
		// the other commands have nothing to check beyond the shared flags above.
	}
	return nil
}

// validateHire checks the two flags hire cannot work without.
func validateHire(o *Options) error {
	var missing []string
	if o.Purpose == "" {
		missing = append(missing, "--purpose")
	}
	if o.Task == "" {
		missing = append(missing, "--task")
	}
	if len(missing) > 0 {
		return i18n.Errorf("error.hire_needs_flags", strings.Join(missing, ", "))
	}
	return nil
}
