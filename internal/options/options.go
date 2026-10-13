// Package options turns the hergy command line into a command and its flags.
package options

import (
	"fmt"
	"sort"
	"strconv"
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
	// Help prints usage.
	Help
	// Version prints which hergy this is.
	Version
	// Cheat prints a cheatsheet: for people, or, with --ai, for an agent.
	Cheat
)

// Options is a parsed command line.
type Options struct {
	Kind Kind

	Team   string // up/show positional, or refresh/reprompt/hire --team
	Member string // refresh/reprompt positional
	All    bool   // refresh/reprompt --all
	Name   string // hire positional
	As     string // --as: the team name to boot under (defaults to the settings name)

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

	HelpText string

	seen map[string]bool
}

const (
	// DefaultTimeoutMS is the startup timeout when --timeout is not given.
	DefaultTimeoutMS = 30000
	minTimeoutMS     = 3001
	maxTimeoutMS     = 300000
)

// Usage is the help text in the language in use.
func Usage() string { return i18n.T("usage") }

// allowed is the flags each command accepts. Anything else is an error, so a
// flag that belongs to another command never gets silently ignored.
var allowed = map[Kind]map[string]bool{
	Up:       {"dry": true, "json": true, "global": true, "local": true, "timeout": true, "dir": true, "lang": true, "as": true, "help": true},
	Down:     {"dry": true, "global": true, "local": true, "dir": true, "lang": true, "as": true, "help": true},
	Refresh:  {"all": true, "team": true, "global": true, "local": true, "dry": true, "json": true, "timeout": true, "dir": true, "lang": true, "as": true, "help": true},
	Reprompt: {"all": true, "team": true, "global": true, "local": true, "dry": true, "json": true, "dir": true, "lang": true, "as": true, "help": true},
	Hire:     {"profile": true, "purpose": true, "task": true, "from": true, "team": true, "global": true, "local": true, "timeout": true, "dry": true, "json": true, "dir": true, "lang": true, "as": true, "help": true},
	Ls:       {"global": true, "local": true, "dir": true, "lang": true, "help": true},
	Show:     {"global": true, "local": true, "dir": true, "lang": true, "help": true},
	New:      {"local": true, "lang": true, "dir": true, "help": true},
	Validate: {"dir": true, "lang": true, "help": true},
	Cheat:    {"ai": true, "lang": true, "help": true},
}

// Kinds lists every command kind, Help aside, in the order the cheatsheet shows
// them. It is the one place a new command is added for the cheatsheet tests to
// pick up.
var Kinds = []Kind{Up, Down, Refresh, Reprompt, Hire, Ls, Show, New, Validate, Version, Cheat}

// Commands returns every command word, in the order Kinds lists them.
func Commands() []string {
	names := make([]string, len(Kinds))
	for i, kind := range Kinds {
		names[i] = commandName(kind)
	}
	return names
}

// Flags returns the flags one command accepts, read from the table the parser
// enforces, so a cheatsheet test can check it names every flag. The names are
// the long spellings, without the leading dashes, sorted.
func Flags(kind Kind) []string {
	out := make([]string, 0, len(allowed[kind]))
	for name := range allowed[kind] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ScanLang finds --lang in argv before the full parse, so messages printed
// during the parse already use the chosen language. The full parse still
// rejects a malformed --lang.
func ScanLang(argv []string) string {
	for i := 0; i < len(argv); i++ {
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
	o := &Options{Timeout: DefaultTimeoutMS, seen: map[string]bool{}}
	switch cmd {
	case "up":
		o.Kind = Up
	case "down":
		o.Kind = Down
	case "refresh":
		o.Kind = Refresh
	case "reprompt":
		o.Kind = Reprompt
	case "hire":
		o.Kind = Hire
	case "ls":
		o.Kind = Ls
	case "show":
		o.Kind = Show
	case "new":
		o.Kind = New
	case "validate":
		o.Kind = Validate
	case "cheat":
		o.Kind = Cheat
	case "help", "-h", "--help":
		return &Options{Kind: Help, HelpText: Usage()}, nil
	case "version", "--version":
		return &Options{Kind: Version}, nil
	default:
		return nil, i18n.Errorf("error.unknown_command", cmd)
	}

	positionals, err := parseFlags(argv[1:], o)
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
		takeValue := func(canonical string) (string, error) {
			if hasValue {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", i18n.Errorf("error.flag_needs_value", canonical)
			}
			return args[i], nil
		}
		switch name {
		case "h", "help":
			return nil, errHelp
		case "dry", "dry-run":
			o.Dry = true
			o.seen["dry"] = true
		case "json":
			o.JSON = true
			o.seen["json"] = true
		case "ai":
			o.AI = true
			o.seen["ai"] = true
		case "all":
			o.All = true
			o.seen["all"] = true
		case "global":
			o.Layer = "global"
			o.seen["global"] = true
		case "local":
			o.Layer = "local"
			o.seen["local"] = true
		case "C", "dir":
			v, err := takeValue("dir")
			if err != nil {
				return nil, err
			}
			o.Dir = v
			o.seen["dir"] = true
		case "team", "profile", "purpose", "task", "from", "timeout", "lang", "as":
			v, err := takeValue(name)
			if err != nil {
				return nil, err
			}
			o.seen[name] = true
			switch name {
			case "team":
				o.Team = v
			case "profile":
				o.Profile = v
			case "purpose":
				o.Purpose = v
			case "task":
				o.Task = v
			case "from":
				o.From = v
			case "as":
				o.As = v
			case "lang":
				o.Lang = v
			case "timeout":
				ms, err := strconv.Atoi(v)
				if err != nil {
					return nil, i18n.Errorf("error.timeout_not_int", v)
				}
				o.Timeout = ms
			}
		default:
			return nil, i18n.Errorf("error.unknown_option", arg)
		}
	}
	return positionals, nil
}

// errHelp signals that --help was seen mid-line.
var errHelp = fmt.Errorf("help")

// IsHelp reports whether Parse returned a Help request via errHelp.
func IsHelp(err error) bool { return err == errHelp }

func assignPositionals(o *Options, positionals []string) error {
	switch o.Kind {
	case Up, Down, Show, New:
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
	case Cheat:
		if len(positionals) != 0 {
			return i18n.Errorf("error.cheat_no_args", positionals)
		}
	}
	return nil
}

func validate(o *Options) error {
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
	if o.JSON && !o.Dry {
		return i18n.Errorf("error.json_needs_dry")
	}
	if o.As != "" && !scaffold.NamePattern.MatchString(o.As) {
		return i18n.Errorf("error.as_name_pattern", o.As)
	}
	switch o.Kind {
	case Refresh, Reprompt:
		if o.All && o.Member != "" {
			return i18n.Errorf("error.seat_all_and_member", commandName(o.Kind))
		}
		if !o.All && o.Member == "" {
			return i18n.Errorf("error.needs_seat", commandName(o.Kind))
		}
	case Hire:
		var missing []string
		if o.Profile == "" {
			missing = append(missing, "--profile")
		}
		if o.Purpose == "" {
			missing = append(missing, "--purpose")
		}
		if o.Task == "" {
			missing = append(missing, "--task")
		}
		if len(missing) > 0 {
			return i18n.Errorf("error.hire_needs_flags", strings.Join(missing, ", "))
		}
	case Show:
		if o.Team == "" {
			return i18n.Errorf("error.show_needs_team")
		}
	case New:
		if o.Team == "" {
			return i18n.Errorf("error.new_needs_name")
		}
	}
	return nil
}

func commandName(kind Kind) string {
	switch kind {
	case Up:
		return "up"
	case Down:
		return "down"
	case Refresh:
		return "refresh"
	case Reprompt:
		return "reprompt"
	case Hire:
		return "hire"
	case Ls:
		return "ls"
	case Show:
		return "show"
	case New:
		return "new"
	case Validate:
		return "validate"
	case Version:
		return "version"
	case Cheat:
		return "cheat"
	default:
		return "help"
	}
}
