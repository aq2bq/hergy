package options

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Text is one sentence in every language hergy ships, keyed by the language
// code. The tests walk i18n.Available, so a language added there fails until
// every Text has it.
type Text map[string]string

func text(en, ja string) Text { return Text{"en": en, "ja": ja} }

// Flag is how one flag is read: its names, whether it takes a value, and what
// it sets. A flag is defined here once; which commands accept it, and what it
// means for each, is the table in spec.go.
type Flag struct {
	Name  string   // the long spelling, without the dashes
	Short string   // the one-letter spelling ("C"), or ""
	Alias []string // other long spellings
	Value Text     // how --help writes the value ("<team>"); nil for a flag with no value
	Set   func(o *Options, value string) error
}

// names are every spelling the parser takes for the flag.
func (f *Flag) names() []string {
	names := []string{f.Name}
	if f.Short != "" {
		names = append(names, f.Short)
	}
	return append(names, f.Alias...)
}

// errHelp signals that -h or --help was seen: the flag stops the parse where it
// stands, so nothing after it is read.
var errHelp = fmt.Errorf("help")

func sets(set func(o *Options)) func(*Options, string) error {
	return func(o *Options, _ string) error {
		set(o)
		return nil
	}
}

func setsValue(set func(o *Options, value string)) func(*Options, string) error {
	return func(o *Options, value string) error {
		set(o, value)
		return nil
	}
}

var (
	flagDry  = &Flag{Name: "dry", Alias: []string{"dry-run"}, Set: sets(func(o *Options) { o.Dry = true })}
	flagJSON = &Flag{Name: "json", Set: sets(func(o *Options) { o.JSON = true })}
	flagAI   = &Flag{Name: "ai", Set: sets(func(o *Options) { o.AI = true })}
	flagAll  = &Flag{Name: "all", Set: sets(func(o *Options) { o.All = true })}
	// --global and --local are gone (段 3): the parser still knows them, so a
	// person who types one is stopped with the sentence that says so, rather
	// than with "unknown option". Nothing is set; validateValues refuses them.
	flagGlobal = &Flag{Name: "global", Set: sets(func(*Options) {})}
	flagLocal  = &Flag{Name: "local", Set: sets(func(*Options) {})}
	// --profile is gone too (段 6b): the profile comes from the workers: entry,
	// so a hire does not choose one. It still takes a value, so the word after
	// it is not read as the worker's name.
	flagProfile = &Flag{Name: "profile", Value: text("<profile>", "<プロファイル>"), Set: sets(func(*Options) {})}

	flagDir = &Flag{Name: "dir", Short: "C", Value: text("<dir>", "<ディレクトリ>"),
		Set: setsValue(func(o *Options, v string) { o.Dir = v })}
	flagTeam = &Flag{Name: "team", Value: text("<team>", "<チーム名>"),
		Set: setsValue(func(o *Options, v string) { o.Team = v })}
	flagAs = &Flag{Name: "as", Value: text("<team>", "<チーム名>"),
		Set: setsValue(func(o *Options, v string) { o.As = v })}
	flagPurpose = &Flag{Name: "purpose", Value: text("<purpose>", "<目的>"),
		Set: setsValue(func(o *Options, v string) { o.Purpose = v })}
	flagTask = &Flag{Name: "task", Value: text("<task>", "<依頼>"),
		Set: setsValue(func(o *Options, v string) { o.Task = v })}
	flagFrom = &Flag{Name: "from", Value: text("<member>", "<メンバー>"),
		Set: setsValue(func(o *Options, v string) { o.From = v })}
	flagLang = &Flag{Name: "lang", Value: text("en|ja", "en|ja"),
		Set: setsValue(func(o *Options, v string) { o.Lang = v })}
	flagTimeout = &Flag{Name: "timeout", Value: text("<ms>", "<ミリ秒>"),
		Set: func(o *Options, v string) error {
			ms, err := strconv.Atoi(v)
			if err != nil {
				return i18n.Errorf("error.timeout_not_int", v)
			}
			o.Timeout = ms
			return nil
		}}
	flagPort = &Flag{Name: "port", Value: text("<port>", "<ポート>"),
		Set: func(o *Options, v string) error {
			port, err := strconv.Atoi(v)
			if err != nil {
				return i18n.Errorf("error.port_not_int", v)
			}
			o.Port = port
			return nil
		}}
	flagNoOpen = &Flag{Name: "no-open", Set: sets(func(o *Options) { o.NoOpen = true })}

	flagHelp = &Flag{Name: "help", Short: "h", Set: func(*Options, string) error { return errHelp }}
)

// allFlags is every flag hergy reads, in the order the cheatsheet lists them.
var allFlags = []*Flag{
	flagAI, flagAs, flagDry, flagJSON, flagAll, flagGlobal, flagLocal, flagTimeout, flagPort, flagNoOpen,
	flagTeam, flagProfile, flagPurpose, flagTask, flagFrom, flagLang, flagDir, flagHelp,
}

// lookupFlag finds the flag a spelling on the command line names, or nil.
func lookupFlag(name string) *Flag {
	for _, f := range allFlags {
		if slices.Contains(f.names(), name) {
			return f
		}
	}
	return nil
}
