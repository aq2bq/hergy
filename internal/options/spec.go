package options

import (
	"sort"

	"github.com/aq2bq/hergy/internal/words"
)

// Use says one command accepts one flag, and what the flag means there. The
// same flag reads differently from command to command (up --dry prints the boot
// plan, down --dry prints what would close), so the description belongs to the
// command, and there is no description to fall back on: a flag added to a
// command without one fails the tests.
type Use struct {
	Flag     *Flag
	Desc     Text
	Required bool // the command refuses to run without it (hire --purpose, --task)
}

// Arg is one positional argument.
type Arg struct {
	Name     Text // without the brackets: "definition"
	Desc     Text
	Optional bool  // written [name] rather than <name>
	Or       *Flag // a flag that stands in for it: refresh <member>|--all
}

// Spec owns command syntax: arguments and accepted flags. Public words are
// references to internal/words. The parser, hergy <command> --help and the cheatsheet's
// command lines all read it.
type Spec struct {
	Kind    Kind
	Name    string // the command word
	Summary Text   // one line; the same sentence as the command's row in hergy --help
	Args    []Arg
	Flags   []Use // the flags of this command, in the order --help shows them; never --lang or --help
	Bare    bool  // version: answers before any flag is read, has no help page of its own
	// Variadic says the last argument may be repeated (apply: any number of
	// members), so the synopsis writes [<member>...] and the parser takes more
	// than the table lists.
	Variadic bool
}

// common is what every command but version accepts. It is written once, so a
// command's Flags leaves it out.
var common = []Use{
	{Flag: flagLang, Desc: words.Sentences("flag.lang")},
	{Flag: flagHelp, Desc: words.Sentences("flag.help")},
}

func use(f *Flag, desc Text) Use { return Use{Flag: f, Desc: desc} }

func required(f *Flag, desc Text) Use { return Use{Flag: f, Desc: desc, Required: true} }

var (
	descDir      = words.Sentences("flag.dir")
	descJSON     = words.Sentences("flag.json")
	descTimeout  = words.Sentences("startup.timeout")
	descTeam     = words.Sentences("team.selection")
	descHireTeam = words.Sentences("hire.team-selection")
)

// specs is every command, in the order the cheatsheet lists them.
var specs = []Spec{
	{
		Kind: Up, Name: "up",
		Summary: words.Label("up.summary"),
		Args: []Arg{{
			Name: words.Label("argument.definition"), Optional: true,
			Desc: words.Label("argument.up-definition"),
		}},
		Flags: []Use{
			use(flagAs, words.Sentences("flag.as.up")),
			use(flagDry, words.Sentences("flag.dry.up")),
			use(flagJSON, descJSON),
			use(flagTimeout, descTimeout),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Down, Name: "down",
		Summary: words.Label("down.summary"),
		Args: []Arg{{
			Name: words.Label("argument.team-or-worker"), Optional: true,
			Desc: words.Label("argument.down-target"),
		}},
		Flags: []Use{
			use(flagDry, words.Sentences("flag.dry.down")),
			use(flagJSON, descJSON),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Status, Name: "status",
		Summary: words.Label("status.summary"),
		Args: []Arg{{
			Name: words.Label("argument.team"), Optional: true,
			Desc: words.Label("argument.status-team"),
		}},
		Flags: []Use{
			use(flagJSON, words.Sentences("flag.json.status")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Refresh, Name: "refresh",
		Summary: words.Label("refresh.summary"),
		Args: []Arg{{
			Name: words.Label("argument.member"), Or: flagAll,
			Desc: words.Label("argument.refresh-member"),
		}},
		Flags: []Use{
			use(flagAll, words.Sentences("flag.all.refresh")),
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.refresh")),
			use(flagJSON, descJSON),
			use(flagTimeout, descTimeout),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Reprompt, Name: "reprompt",
		Summary: words.Label("reprompt.summary"),
		Args: []Arg{{
			Name: words.Label("argument.member"), Or: flagAll,
			Desc: words.Label("argument.reprompt-member"),
		}},
		Flags: []Use{
			use(flagAll, words.Sentences("flag.all.reprompt")),
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.reprompt")),
			use(flagJSON, descJSON),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Apply, Name: "apply", Variadic: true,
		Summary: words.Label("apply.summary"),
		Args: []Arg{{
			Name: words.Label("argument.member"), Optional: true,
			Desc: words.Label("argument.apply-members"),
		}},
		Flags: []Use{
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.apply")),
			use(flagJSON, descJSON),
			use(flagTimeout, descTimeout),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Focus, Name: "focus",
		Summary: words.Label("focus.summary"),
		Args: []Arg{{
			Name: words.Label("argument.member"),
			Desc: words.Label("argument.focus-member"),
		}},
		Flags: []Use{
			use(flagTeam, descTeam),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Nudge, Name: "nudge",
		Summary: words.Label("nudge.summary"),
		Args: []Arg{
			{
				Name: words.Label("argument.member"),
				Desc: words.Label("argument.nudge-member"),
			},
			{
				Name: words.Label("argument.text"), Optional: true,
				Desc: words.Label("argument.nudge-text"),
			},
		},
		Flags: []Use{
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.nudge")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Stop, Name: "stop",
		Summary: words.Label("stop.summary"),
		Args: []Arg{{
			Name: words.Label("argument.member"),
			Desc: words.Label("argument.stop-member"),
		}},
		Flags: []Use{
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.stop")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Tell, Name: "tell",
		Summary: words.Label("tell.summary"),
		Args: []Arg{
			{
				Name: words.Label("argument.name"),
				Desc: words.Label("argument.tell-recipient"),
			},
			{
				Name: words.Label("argument.text"), Optional: true,
				Desc: words.Label("argument.tell-text"),
			},
		},
		Flags: []Use{
			use(flagFrom, words.Sentences("flag.from.tell")),
			use(flagTeam, descTeam),
			use(flagDry, words.Sentences("flag.dry.tell")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Hire, Name: "hire",
		Summary: words.Label("hire.summary"),
		Args: []Arg{{
			Name: words.Label("argument.name"),
			Desc: words.Label("argument.hire-name"),
		}},
		Flags: []Use{
			required(flagPurpose, words.Sentences("flag.purpose.hire")),
			required(flagTask, words.Sentences("flag.task.hire")),
			use(flagFrom, words.Sentences("flag.from.hire")),
			use(flagTeam, descHireTeam),
			use(flagTimeout, descTimeout),
			use(flagDry, words.Sentences("flag.dry.hire")),
			use(flagJSON, descJSON),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Turns, Name: "turns",
		Summary: words.Label("turns.summary"),
		Args: []Arg{{
			Name: words.Label("argument.team"), Optional: true,
			Desc: words.Label("argument.turns-team"),
		}},
		Flags: []Use{
			use(flagLog, words.Sentences("flag.log.turns")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: TurnDone, Name: "turn-done",
		Summary: words.Label("turn-done.summary"),
		Args: []Arg{
			{Name: words.Label("argument.team"), Desc: words.Label("argument.team-name")},
			{Name: words.Label("argument.member"), Desc: words.Label("argument.turn-done-member")},
			{Name: words.Label("argument.payload"), Optional: true, Desc: words.Label("argument.turn-done-payload")},
		},
	},
	{
		Kind: Ls, Name: "ls",
		Summary: words.Label("ls.summary"),
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Show, Name: "show",
		Summary: words.Label("show.summary"),
		Args: []Arg{{
			Name: words.Label("argument.definition"),
			Desc: words.Label("argument.show-definition"),
		}},
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: New, Name: "new",
		Summary: words.Label("new.summary"),
		Args: []Arg{{
			Name: words.Label("argument.definition"),
			Desc: words.Label("argument.new-definition"),
		}},
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{Kind: Migrate, Name: "migrate", Summary: words.Label("migrate.summary"),
		Flags: []Use{use(flagDry, words.Sentences("flag.dry.migrate")), use(flagGlobal, words.Sentences("flag.global.migrate")), use(flagDir, descDir)}},
	{
		Kind: Validate, Name: "validate",
		Summary: words.Label("validate.summary"),
		Flags: []Use{
			use(flagDir, descDir),
		},
	},
	{
		Kind: Serve, Name: "serve",
		Summary: words.Label("serve.summary"),
		Flags: []Use{
			use(flagPort, words.Sentences("flag.port.serve")),
			use(flagNoOpen, words.Sentences("flag.no-open.serve")),
			use(flagDir, descDir),
		},
	},
	{
		Kind: Version, Name: "version", Bare: true,
		Summary: words.Label("version.summary"),
	},
	{
		Kind: Cheat, Name: "cheat",
		Summary: words.Label("cheat.summary"),
		Flags: []Use{
			use(flagAI, words.Sentences("flag.ai.cheat")),
		},
	},
}

// allowed is the flags each command accepts, read from specs and kept as a
// lookup. Anything else is an error, so a flag that belongs to another command
// never gets silently ignored.
var allowed = buildAllowed()

func buildAllowed() map[Kind]map[string]bool {
	out := map[Kind]map[string]bool{}
	for _, s := range specs {
		if s.Bare {
			continue
		}
		names := map[string]bool{}
		for _, u := range s.Flags {
			names[u.Flag.Name] = true
		}
		for _, u := range common {
			names[u.Flag.Name] = true
		}
		out[s.Kind] = names
	}
	return out
}

// Kinds lists every command kind, Help aside, in the order the cheatsheet shows
// them. It comes from specs, so a command added there is picked up by the
// cheatsheet tests.
var Kinds = kindsOf(specs)

func kindsOf(list []Spec) []Kind {
	kinds := make([]Kind, len(list))
	for i, s := range list {
		kinds[i] = s.Kind
	}
	return kinds
}

// specOf is the table row of a command kind, or nil for Help.
func specOf(kind Kind) *Spec {
	for i := range specs {
		if specs[i].Kind == kind {
			return &specs[i]
		}
	}
	return nil
}

// specNamed is the table row for a command word, or nil.
func specNamed(name string) *Spec {
	for i := range specs {
		if specs[i].Name == name {
			return &specs[i]
		}
	}
	return nil
}

// Commands returns every command word, in the order Kinds lists them.
func Commands() []string {
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}
	return names
}

// Flags returns the flags one command accepts, --lang and --help included: the
// names the parser lets through. A cheatsheet test checks it names every flag.
// The names are the long spellings, without the leading dashes, sorted.
func Flags(kind Kind) []string {
	out := make([]string, 0, len(allowed[kind]))
	for name := range allowed[kind] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// OwnFlags returns the flags that belong to one command, without the --lang
// and --help every command has. It is what the command's own line in a
// cheatsheet has to name.
func OwnFlags(kind Kind) []string {
	s := specOf(kind)
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Flags))
	for _, u := range s.Flags {
		out = append(out, u.Flag.Name)
	}
	sort.Strings(out)
	return out
}

func commandName(kind Kind) string {
	if s := specOf(kind); s != nil {
		return s.Name
	}
	return "help"
}
