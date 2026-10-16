package config

import (
	"bytes"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The keys of a profiles.yaml, in the order hergy writes them (d-cbac, spec/10
// の 6 節): the format version, then the profiles, and under one profile its
// engine, its arguments and its environment. The order is the one ParseProfiles
// reads a file with, so a key cannot move in one place and stay in the other.
var (
	profilesFileKeys = []string{"version", "profiles"}
	profileFileKeys  = []string{"kind", "args", "env"}
)

// EmitProfiles writes a profiles.yaml back in the one format hergy writes
// (d-cbac): the version first, then the profiles by name, and under each one its
// kind, its args and its env, leaving out an args and an env the profile does
// not hold. A file with no profile holds the version alone.
//
// The profiles are written by name, so the order a file happened to have is not
// kept. A file hergy writes is the file it reads back: ParseProfiles of what
// EmitProfiles wrote returns the profiles it was given, and EmitProfiles of
// those returns the same text again.
func EmitProfiles(profiles map[string]Profile) (string, error) {
	names := slices.Sorted(maps.Keys(profiles))
	values := map[string]*yaml.Node{"version": emitProfilesInt(1)}
	if len(names) > 0 {
		bodies := make(map[string]*yaml.Node, len(names))
		for _, name := range names {
			bodies[name] = emitProfileBody(profiles[name])
		}
		values["profiles"] = emitProfilesMapping(names, bodies)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(emitProfilesMapping(profilesFileKeys, values)); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// emitProfileBody is one profile as the keys ParseProfiles reads it with.
func emitProfileBody(profile Profile) *yaml.Node {
	values := map[string]*yaml.Node{"kind": emitProfilesText(profile.Kind)}
	if len(profile.Args) > 0 {
		values["args"] = emitProfilesList(profile.Args)
	}
	if len(profile.Env) > 0 {
		names := slices.Sorted(maps.Keys(profile.Env))
		env := make(map[string]*yaml.Node, len(names))
		for _, name := range names {
			env[name] = emitProfilesText(profile.Env[name])
		}
		values["env"] = emitProfilesMapping(names, env)
	}
	return emitProfilesMapping(profileFileKeys, values)
}

// The helpers below write a value the way rules.Emit writes it
// (internal/rules/emit.go): a mapping whose keys are written in the order given,
// a list as a block sequence, and a text with the string tag set so a value that
// looks like a number still comes back as text. The two files write the one
// format (d-cbac), and config cannot borrow rules' helpers: config reads
// team.yaml through rules, so the import between the two goes one way.

// emitProfilesMapping is a mapping whose keys are written in the order given. A
// key with no value is left out, and a key given twice is left out the second
// time.
func emitProfilesMapping(keys []string, values map[string]*yaml.Node) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil || seen[key] {
			continue
		}
		seen[key] = true
		n.Content = append(n.Content, emitProfilesText(key), value)
	}
	return n
}

func emitProfilesList(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, value := range values {
		items = append(items, emitProfilesText(value))
	}
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

// emitProfilesText is one text value. A value with a line break is a literal
// block; any other value is one line and yaml.v3 adds quotes when the plain form
// would read as something else.
func emitProfilesText(text string) *yaml.Node {
	style := yaml.Style(0)
	if strings.Contains(text, "\n") {
		style = yaml.LiteralStyle
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: text, Style: style}
}

func emitProfilesInt(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}
