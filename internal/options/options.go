// Package options turns the hergy command line into a command and its flags.
package options

import (
	"fmt"
	"strconv"
	"strings"
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

	HelpText string

	seen map[string]bool
}

const (
	// DefaultTimeoutMS is the startup timeout when --timeout is not given.
	DefaultTimeoutMS = 30000
	minTimeoutMS     = 3001
	maxTimeoutMS     = 300000
)

// Usage is the help text.
const Usage = `hergy - チームと役割を Herdr の一段上で扱う

使い方:
  hergy up [チーム] [--dry] [--global|--local] [--timeout ms] [-C プロジェクト]
  hergy refresh <担当>|--all [--team チーム] [--global|--local] [--dry] [-C プロジェクト]
  hergy hire <名前> --profile P --purpose X --task "…" [--from 担当] [--team チーム] [--global|--local] [--timeout ms] [--dry] [-C プロジェクト]
  hergy ls [--global|--local] [-C プロジェクト]
  hergy show <チーム> [--global|--local] [-C プロジェクト]

説明:
  up        チームを起動する。--dry なら Herdr に触れず展開済みの計画を出す
  refresh   idle な担当へ初期プロンプトを再投入する（--all で全員）
  hire      ワーカーを1人、別のタブに起動する。may_spawn の外でも注意を出して続ける
  ls        見えるチームを層つきで一覧する
  show      層を重ねた最終形と、各値・各文書がどの層のどのファイルから来たかを出す

チームの解決:
  名前だけで呼ぶ。ローカル（<プロジェクト>/.hergy/teams/<名前>/team.yaml）が先、
  グローバル（$XDG_CONFIG_HOME/hergy/teams/<名前>/team.yaml、未設定なら ~/.config/hergy/）が後。
  同名はローカルが勝つ。引数なしはローカルにチームが1つだけならそれ、それ以外は候補を示して止まる。
  プロジェクトの直下は git のルート（無ければカレント）。-C で直下を指定できる。

  起動に HERGY_HERDR を指定すると、herdr の実行ファイルを差し替える（偽の herdr で確かめるため）。
`

// allowed is the flags each command accepts. Anything else is an error, so a
// flag that belongs to another command never gets silently ignored.
var allowed = map[Kind]map[string]bool{
	Up:      {"dry": true, "global": true, "local": true, "timeout": true, "dir": true, "help": true},
	Refresh: {"all": true, "team": true, "global": true, "local": true, "dry": true, "dir": true, "help": true},
	Hire:    {"profile": true, "purpose": true, "task": true, "from": true, "team": true, "global": true, "local": true, "timeout": true, "dry": true, "dir": true, "help": true},
	Ls:      {"global": true, "local": true, "dir": true, "help": true},
	Show:    {"global": true, "local": true, "dir": true, "help": true},
}

// Parse reads argv (without the program name).
func Parse(argv []string) (*Options, error) {
	if len(argv) == 0 {
		return &Options{Kind: Help, HelpText: Usage}, nil
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
	case "help", "-h", "--help":
		return &Options{Kind: Help, HelpText: Usage}, nil
	default:
		return nil, fmt.Errorf("unknown command %q（hergy help を見てください）", cmd)
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
				return "", fmt.Errorf("--%s には値が必要です", canonical)
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
		case "team", "profile", "purpose", "task", "from", "timeout":
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
			case "timeout":
				ms, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("--timeout はミリ秒の整数です: %q", v)
				}
				o.Timeout = ms
			}
		default:
			return nil, fmt.Errorf("unknown option %q（hergy help を見てください）", arg)
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
	case Up, Show:
		if len(positionals) > 1 {
			return fmt.Errorf("位置引数が多すぎます: %v", positionals)
		}
		if len(positionals) == 1 {
			o.Team = positionals[0]
		}
	case Refresh:
		if len(positionals) > 1 {
			return fmt.Errorf("位置引数が多すぎます: %v", positionals)
		}
		if len(positionals) == 1 {
			o.Member = positionals[0]
		}
	case Hire:
		if len(positionals) != 1 {
			return fmt.Errorf("hergy hire にはワーカー名が1つ必要です")
		}
		o.Name = positionals[0]
	case Ls:
		if len(positionals) != 0 {
			return fmt.Errorf("hergy ls は位置引数を取りません: %v", positionals)
		}
	}
	return nil
}

func validate(o *Options) error {
	if o.seen["global"] && o.seen["local"] {
		return fmt.Errorf("--global と --local は同時に使えません")
	}
	for name := range o.seen {
		if !allowed[o.Kind][name] {
			return fmt.Errorf("--%s は hergy %s では使えません", name, commandName(o.Kind))
		}
	}
	if o.Timeout < minTimeoutMS || o.Timeout > maxTimeoutMS {
		return fmt.Errorf("--timeout は %d..%d ミリ秒です", minTimeoutMS, maxTimeoutMS)
	}
	switch o.Kind {
	case Refresh:
		if o.All && o.Member != "" {
			return fmt.Errorf("refresh は <担当> と --all を同時に使えません")
		}
		if !o.All && o.Member == "" {
			return fmt.Errorf("refresh には <担当> か --all が必要です")
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
			return fmt.Errorf("hire には %s が必要です", strings.Join(missing, ", "))
		}
	case Show:
		if o.Team == "" {
			return fmt.Errorf("hergy show にはチーム名が必要です")
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
	default:
		return "help"
	}
}
