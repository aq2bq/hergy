package profile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// This file reads a profiles.yaml into the entries a screen shows and the
// problems a person has to fix, without stopping at the first one. The command
// line reads its first problem from here too, so a screen and `hergy validate`
// say the same sentence (r-c683).

// Severity is how hard one problem is: an error stops the file from reading, a
// warning does not.
type Severity int

const (
	// Err is a problem that stops the file from reading.
	Err Severity = iota
	// Warn is a problem the file reads through.
	Warn
)

// Word is the severity as a screen and a JSON answer write it.
func (s Severity) Word() string {
	if s == Warn {
		return "warn"
	}
	return "err"
}

// Problem is one thing wrong with a profiles.yaml. Field is where it stands in
// the profile (name, kind, args.<i>, env.<i>.key, env.<i>.value), empty for the
// file as a whole. Line is the YAML line it is on, 1-based.
type Problem struct {
	Sev   Severity
	Field string
	Line  int
	Text  string
}

// EnvPair is one environment variable as the file writes it: the order of the
// file is kept, so a screen shows the pairs in the order a person wrote them.
type EnvPair struct {
	Key   string
	Value string
}

// Entry is one profile as a file writes it: the values a screen shows, and the
// lines they stand on. Problems are the ones this entry has.
type Entry struct {
	Name     string
	Kind     string
	Args     []string
	Env      []EnvPair
	Line     int
	KindLine int
	ArgsLine int
	EnvLines []int
	Problems []Problem
}

// ReadAll reads one profiles.yaml into the entries it holds and every problem
// it has, in the order the file writes them. A file that does not read at all
// (empty, broken YAML, a version hergy does not read) gives no entries and one
// file-level problem.
func ReadAll(text, path string) ([]Entry, []Problem) {
	if strings.TrimSpace(text) == "" {
		return nil, []Problem{{Sev: Err, Line: 1, Text: i18n.Tf("error.file_empty", "profiles.yaml", path)}}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, []Problem{{Sev: Err, Line: errorLine(err, text), Text: fmt.Sprintf("%s: %s", path, CleanError(err))}}
	}
	if len(doc.Content) == 0 {
		return nil, []Problem{{Sev: Err, Line: 1, Text: i18n.Tf("error.file_empty", "profiles.yaml", path)}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, []Problem{{Sev: Err, Line: root.Line, Text: i18n.Tf("error.version_missing", "profiles.yaml", path)}}
	}

	problems := versionProblems(root, path)
	if hasError(problems) {
		return nil, problems
	}
	problems = append(problems, unknownKeys(root, path, "version", "profiles")...)
	profiles := yamlMappingValue(root, "profiles")
	if profiles == nil {
		return nil, problems
	}
	if profiles.Kind != yaml.MappingNode {
		return nil, append(problems, Problem{Sev: Err, Line: profiles.Line, Text: i18n.Tf("error.edit_not_mapping", path)})
	}

	entries := make([]Entry, 0, len(profiles.Content)/2)
	seen := map[string]bool{}
	for i := 0; i+1 < len(profiles.Content); i += 2 {
		key, value := profiles.Content[i], profiles.Content[i+1]
		name := strings.TrimSpace(key.Value)
		if seen[name] {
			problems = append(problems, Problem{Sev: Err, Field: "name", Line: key.Line, Text: i18n.Tf("error.edit_profile_twice", name)})
			continue
		}
		seen[name] = true
		entry, found := readEntry(name, key, value, path)
		entries = append(entries, entry)
		problems = append(problems, found...)
	}
	return entries, problems
}

// versionProblems is the one check that comes before any profile: the file has
// to say the version hergy reads.
func versionProblems(root *yaml.Node, path string) []Problem {
	key := yamlMappingKey(root, "version")
	if key == nil {
		return []Problem{{Sev: Err, Line: 1, Text: i18n.Tf("error.version_missing", "profiles.yaml", path)}}
	}
	value := yamlMappingValue(root, "version")
	number, err := strconv.Atoi(value.Value)
	if value.Kind != yaml.ScalarNode || err != nil {
		return []Problem{{Sev: Err, Line: key.Line, Text: i18n.Tf("error.version_missing", "profiles.yaml", path)}}
	}
	if number != 1 {
		return []Problem{{Sev: Err, Line: key.Line, Text: i18n.Tf("error.version_wrong", "profiles.yaml", number, path)}}
	}
	return nil
}

// unknownKeys names every key a settings file does not hold, the way the strict
// decode the command line reads with names it.
func unknownKeys(mapping *yaml.Node, path string, known ...string) []Problem {
	allowed := map[string]bool{}
	for _, key := range known {
		allowed[key] = true
	}
	var out []Problem
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if allowed[key] {
			continue
		}
		line := mapping.Content[i].Line
		out = append(out, Problem{Sev: Err, Line: line, Text: fmt.Sprintf("%s: field %s not found", path, key)})
	}
	return out
}

// readEntry reads one profile and every problem its fields have.
func readEntry(name string, key, value *yaml.Node, path string) (Entry, []Problem) {
	entry := Entry{Name: name, Line: key.Line}
	label := "profiles." + name
	var problems []Problem
	add := func(sev Severity, field string, line int, text string) {
		problems = append(problems, Problem{Sev: sev, Field: field, Line: line, Text: text})
	}
	if !NamePattern.MatchString(name) {
		add(Err, "name", key.Line, i18n.Tf("error.profile_name_pattern", label))
	}
	if value.Kind != yaml.MappingNode {
		add(Err, "kind", value.Line, i18n.Tf("error.not_plain_kind", label+".kind"))
		return entry, problems
	}
	problems = append(problems, unknownKeys(value, path, "kind", "args", "env")...)

	kindNode := yamlMappingValue(value, "kind")
	entry.KindLine = value.Line
	if kindNode != nil {
		entry.KindLine = kindNode.Line
		if kind := readKind(kindNode); kind != "" {
			entry.Kind = kind
		} else {
			add(Err, "kind", kindNode.Line, i18n.Tf("error.not_plain_kind", label+".kind"))
		}
	} else {
		add(Err, "kind", value.Line, i18n.Tf("error.not_plain_kind", label+".kind"))
	}
	entry.Args, entry.ArgsLine, problems = readArgs(entry.Args, value, label, problems)
	entry.Env, entry.EnvLines, problems = readEnv(entry.Env, value, label, problems)
	if entry.Kind != "" && name == entry.Kind {
		add(Warn, "name", key.Line, i18n.Tf("validate.profile_named_after_kind", name))
	}
	entry.Problems = problems
	return entry, problems
}

// readKind is a kind a profile can run, or "" when the file wrote none hergy
// can read.
func readKind(node *yaml.Node) string {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || !kindPattern.MatchString(node.Value) {
		return ""
	}
	return node.Value
}

// readArgs reads args: and the problems of its items. A value that is not a
// list is one problem, and no args.
func readArgs(args []string, body *yaml.Node, label string, problems []Problem) ([]string, int, []Problem) {
	key := yamlMappingKey(body, "args")
	value := yamlMappingValue(body, "args")
	if key == nil || value == nil || value.Tag == "!!null" {
		return args, 0, problems
	}
	line := key.Line
	if value.Kind != yaml.SequenceNode {
		return args, line, append(problems, Problem{Sev: Err, Field: "args.0", Line: line, Text: i18n.Tf("error.arg_not_string", label+".args", 0)})
	}
	for i, item := range value.Content {
		if item.Tag != "!!str" {
			problems = append(problems, Problem{Sev: Err, Field: fmt.Sprintf("args.%d", i), Line: item.Line, Text: i18n.Tf("error.arg_not_string", label+".args", i)})
		} else if strings.ContainsRune(item.Value, 0) {
			problems = append(problems, Problem{Sev: Err, Field: fmt.Sprintf("args.%d", i), Line: item.Line, Text: i18n.Tf("error.arg_nul", label+".args", i)})
		}
		args = append(args, item.Value)
	}
	return args, line, problems
}

// readEnv reads env: in the order the file wrote it, and the problems of its
// names and values, a name written twice included.
func readEnv(env []EnvPair, body *yaml.Node, label string, problems []Problem) ([]EnvPair, []int, []Problem) {
	key := yamlMappingKey(body, "env")
	value := yamlMappingValue(body, "env")
	if key == nil || value == nil || value.Tag == "!!null" {
		return env, nil, problems
	}
	if value.Kind != yaml.MappingNode {
		return env, []int{key.Line}, append(problems, Problem{Sev: Err, Field: "env.0.key", Line: key.Line, Text: i18n.Tf("error.edit_not_mapping", label+".env")})
	}
	var lines []int
	seen := map[string]bool{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		name, item := value.Content[i], value.Content[i+1]
		line := name.Line
		lines = append(lines, line)
		field := fmt.Sprintf("env.%d", i/2)
		if !envPattern.MatchString(name.Value) {
			problems = append(problems, Problem{Sev: Err, Field: field + ".key", Line: line, Text: i18n.Tf("error.not_variable_name", label+".env", name.Value)})
		} else if seen[name.Value] {
			problems = append(problems, Problem{Sev: Err, Field: field + ".key", Line: line, Text: i18n.Tf("error.env_key_twice", label, name.Value)})
		}
		seen[name.Value] = true
		if item.Tag != "!!str" {
			problems = append(problems, Problem{Sev: Err, Field: field + ".value", Line: item.Line, Text: i18n.Tf("error.env_not_string", label+".env", name.Value)})
		}
		env = append(env, EnvPair{Key: name.Value, Value: item.Value})
	}
	return env, lines, problems
}

// CheckDraft checks one profile a screen is editing against the entries the
// file holds. original is the saved name the edit started from, empty for a new
// profile. A name that is the kind is a warning; everything else stops a save.
func CheckDraft(draft Draft, saved []Entry, original string) []Problem {
	label := "profiles." + draft.Name
	var out []Problem
	add := func(sev Severity, field, text string) {
		out = append(out, Problem{Sev: sev, Field: field, Text: text})
	}
	if !NamePattern.MatchString(draft.Name) {
		add(Err, "name", i18n.Tf("error.profile_name_pattern", label))
	} else {
		for _, entry := range saved {
			if entry.Name != original && entry.Name == draft.Name {
				add(Err, "name", i18n.Tf("error.edit_profile_twice", draft.Name))
				break
			}
		}
	}
	if draft.Kind != "" && draft.Name == draft.Kind {
		add(Warn, "name", i18n.Tf("validate.profile_named_after_kind", draft.Name))
	}
	if draft.Kind == "" || !kindPattern.MatchString(draft.Kind) {
		add(Err, "kind", i18n.Tf("error.not_plain_kind", label+".kind"))
	}
	for i, arg := range draft.Args {
		if strings.ContainsRune(arg, 0) {
			add(Err, fmt.Sprintf("args.%d", i), i18n.Tf("error.arg_nul", label+".args", i))
		}
	}
	seen := map[string]bool{}
	for i, pair := range draft.Env {
		field := fmt.Sprintf("env.%d", i)
		switch {
		case !envPattern.MatchString(pair.Key):
			add(Err, field+".key", i18n.Tf("error.not_variable_name", label+".env", pair.Key))
		case seen[pair.Key]:
			add(Err, field+".key", i18n.Tf("error.env_key_twice", label, pair.Key))
		}
		seen[pair.Key] = true
	}
	return out
}

// Draft is one profile as a screen holds it before it is written.
type Draft struct {
	Name string
	Kind string
	Args []string
	Env  []EnvPair
}

// hasError reports whether any problem stops the file.
func hasError(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Sev == Err {
			return true
		}
	}
	return false
}

// errorLine is the line a YAML reading error names, or 1 when it names none.
func errorLine(err error, _ string) int {
	found := lineRE.FindStringSubmatch(err.Error())
	if len(found) < 2 {
		return 1
	}
	number, convErr := strconv.Atoi(found[1])
	if convErr != nil || number < 1 {
		return 1
	}
	return number
}

// lineRE is the "line N" a yaml.v3 error carries.
var lineRE = regexp.MustCompile(`line (\d+)`)

// yamlMappingKey is the key node a mapping writes, or nil.
func yamlMappingKey(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i]
		}
	}
	return nil
}

// yamlMappingValue is the value a mapping writes for a key, or nil.
func yamlMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if keyNode := yamlMappingKey(mapping, key); keyNode != nil {
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if mapping.Content[i] == keyNode {
				return mapping.Content[i+1]
			}
		}
	}
	return nil
}
