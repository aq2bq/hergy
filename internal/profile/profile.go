// Package profile names the AI engines hergy starts: one profile is a name, a
// kind, the arguments the kind is started with and the environment it runs in,
// kept in a profiles.yaml. A member names a profile and never writes a kind
// itself (I7).
//
// The reading that every settings YAML shares — the format version first, then a
// decode that refuses an unknown key — lives here too (settings.go): profiles.yaml
// is the lowest settings file hergy reads, and team.yaml and config.yaml read
// through the same functions. One copy, and the import between the settings
// packages goes one way.
package profile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// NamePattern is the shape of a name hergy writes in a settings file: a profile,
// a team definition, a member. It is one pattern, so a name that is legal in one
// place is legal in the others.
var NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// kindPattern is the shape of a profile's kind: the agent hergy starts.
var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// envPattern is the shape of one environment variable's name.
var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Profile is one engine choice, named, kept in a profiles.yaml.
type Profile struct {
	Name string
	Kind string
	Args []string
	Env  map[string]string
	Path string // the profiles.yaml it came from
}

type rawProfiles struct {
	Version  *int                  `yaml:"version"`
	Profiles map[string]rawProfile `yaml:"profiles"`
}

type rawProfile struct {
	Kind string         `yaml:"kind"`
	Args []any          `yaml:"args"`
	Env  map[string]any `yaml:"env"`
}

// ParseProfiles reads one profiles.yaml, checking its version before anything
// else and then rejecting unknown keys.
func ParseProfiles(text, path string) (map[string]Profile, error) {
	if _, err := ReadVersion(text, "profiles.yaml", path); err != nil {
		return nil, err
	}
	var raw rawProfiles
	if err := StrictDecode(text, &raw); err != nil {
		return nil, fmt.Errorf("%s: %s", path, CleanError(err))
	}
	out := make(map[string]Profile, len(raw.Profiles))
	for name, value := range raw.Profiles {
		label := "profiles." + name
		if !NamePattern.MatchString(name) {
			return nil, i18n.Errorf("error.profile_name_pattern", label)
		}
		kind, err := kindOf(value.Kind, label+".kind")
		if err != nil {
			return nil, err
		}
		args, err := argsOf(value.Args, label+".args")
		if err != nil {
			return nil, err
		}
		env, err := envOf(value.Env, label+".env")
		if err != nil {
			return nil, err
		}
		out[name] = Profile{Name: name, Kind: kind, Args: args, Env: env, Path: path}
	}
	return out, nil
}

func kindOf(value, label string) (string, error) {
	if value == "" || !kindPattern.MatchString(value) {
		return "", i18n.Errorf("error.not_plain_kind", label)
	}
	return value, nil
}

// argsOf accepts only strings, so an unquoted number is an error; quote numbers.
func argsOf(value []any, label string) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	out := make([]string, 0, len(value))
	for i, item := range value {
		text, ok := item.(string)
		if !ok {
			return nil, i18n.Errorf("error.arg_not_string", label, i)
		}
		if strings.ContainsRune(text, 0) {
			return nil, i18n.Errorf("error.arg_nul", label, i)
		}
		out = append(out, text)
	}
	return out, nil
}

// envOf accepts only a mapping of variable name to string value.
func envOf(value map[string]any, label string) (map[string]string, error) {
	if len(value) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(value))
	for key, item := range value {
		if !envPattern.MatchString(key) {
			return nil, i18n.Errorf("error.not_variable_name", label, key)
		}
		text, ok := item.(string)
		if !ok {
			return nil, i18n.Errorf("error.env_not_string", label, key)
		}
		out[key] = text
	}
	return out, nil
}
