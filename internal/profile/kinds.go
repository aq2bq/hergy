package profile

// Kinds is the agent kinds a screen offers when a person picks one for a
// profile, in the order herdr's `agent start --kind` lists them.
//
// hergy keeps no list of its own and no closed list: a profile may name any
// kind the shape allows (kindPattern), and hergy starts whatever herdr can start
// — the cheatsheet says the kind is "herdr's agent kind" and nothing else. This
// list is the choice a screen shows, not a limit, so a name outside it is still
// read from a file. It was copied from herdr 0.9.1; a person picking a kind
// cannot pick one herdr has added since, and the list has to be brought over.
func Kinds() []string {
	out := make([]string, len(kinds))
	copy(out, kinds)
	return out
}

// kinds is herdr's agent kinds, as `herdr agent start --help` prints them.
var kinds = []string{
	"pi",
	"claude",
	"codex",
	"gemini",
	"cursor",
	"devin",
	"agy",
	"cline",
	"omp",
	"mastracode",
	"opencode",
	"copilot",
	"kimi",
	"kiro",
	"droid",
	"amp",
	"grok",
	"hermes",
	"kilo",
	"qodercli",
	"qwen",
	"letta",
	"maki",
	"muse",
}
