package ops

import "github.com/aq2bq/hergy/internal/ops/internal/team"

// Environment locates the invocation's project and configuration sources.
// It is a read model; it exposes no running-team or herdr operations.
type Environment = team.Env

// Configuration is the parsed global project and language selection.
type Configuration = team.Config

// NewEnvironment resolves this invocation's project and configuration paths.
func NewEnvironment() (Environment, error) { return team.NewEnv() }

// ConfiguredLanguage reads the command line's default language.
func ConfiguredLanguage() (string, error) { return team.Lang() }

// ReadConfiguration checks a global configuration document.
func ReadConfiguration(text, path string) (Configuration, error) { return team.ReadConfig(text, path) }

// RunningDefinitions reads the live team names grouped by their definition.
// An unavailable session preserves ls's definition-only fallback.
func RunningDefinitions() map[string][]string {
	out := map[string][]string{}
	if tabs, _, err := herdrTabs(); err == nil {
		for _, t := range team.RunningTeams(tabs, currentTeamStates()) {
			out[t.Definition] = append(out[t.Definition], t.Team.String())
		}
	}
	return out
}
