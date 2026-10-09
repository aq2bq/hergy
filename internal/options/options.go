// Package options turns the hergy command line into a command and its flags.
package options

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Kind is the command.
type Kind int

const (
	// Up boots (or, with --dry, prints) a team.
	Up Kind = iota
	// Refresh re-submits a bootstrap prompt.
	Refresh
	// Hire starts one worker.
	Hire
	// Ls lists visible teams.
	Ls
	// Show prints the effective config and where it came from.
	Show
	// New writes a starter team's files.
	New
	// Help prints usage.
	Help
)

// Options is a parsed command line.
type Options struct {
	Kind Kind

	Team   string // up/show positional, or refresh/hire --team
	Member string // refresh positional
	All    bool   // refresh --all
	Name   string // hire positional

	Profile string
	Purpose string
	Task    string
	From    string

	Dry     bool
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
	Up:      {"dry": true, "global": true, "local": true, "timeout": true, "dir": true, "lang": true, "help": true},
	Refresh: {"all": true, "team": true, "global": true, "local": true, "dry": true, "dir": true, "lang": true, "help": true},
	Hire:    {"profile": true, "purpose": true, "task": true, "from": true, "team": true, "global": true, "local": true, "timeout": true, "dry": true, "dir": true, "lang": true, "help": true},
	Ls:      {"global": true, "local": true, "dir": true, "lang": true, "help": true},
	Show:    {"global": true, "local": true, "dir": true, "lang": true, "help": true},
	New:     {"local": true, "lang": true, "dir": true, "help": true},
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
	case "refresh":
		o.Kind = Refresh
	case "hire":
		o.Kind = Hire
	case "ls":
		o.Kind = Ls
	case "show":
		o.Kind = Show
	case "new":
		o.Kind = New
	case "help", "-h", "--help":
		return &Options{Kind: Help, HelpText: Usage()}, nil
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
		case "team", "profile", "purpose", "task", "from", "timeout", "lang":
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
	case Up, Show, New:
		if len(positionals) > 1 {
			return i18n.Errorf("error.too_many_args", positionals)
		}
		if len(positionals) == 1 {
			o.Team = positionals[0]
		}
	case Refresh:
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
	switch o.Kind {
	case Refresh:
		if o.All && o.Member != "" {
			return i18n.Errorf("error.refresh_all_and_member")
		}
		if !o.All && o.Member == "" {
			return i18n.Errorf("error.refresh_needs_member")
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
	case Refresh:
		return "refresh"
	case Hire:
		return "hire"
	case Ls:
		return "ls"
	case Show:
		return "show"
	case New:
		return "new"
	default:
		return "help"
	}
}
