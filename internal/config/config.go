// Package config turns a team's YAML into the values a boot or a hire needs.
//
// YAML owns topology and engines; the role files own the meaning of each role.
// The parser does not know about layers or the filesystem layout: it is handed
// the already-resolved team directory and profiles, and reads what it needs
// through the Reader.
package config

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// Patterns the config enforces.
var (
	NamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	envPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	typeNameRE  = regexp.MustCompile(` in type \S+|\s*(?:\[\])?config\.raw\w+|\s*into\s+(?:map\[string\]interface \{\}|(?:\[\])?interface \{\}|\[\](?:string|int|bool|any|float64))`)
	dangleRE    = regexp.MustCompile(`(?m)\s+into\s*$`)
)

// teamPlaceholder is a leftover from the form where a member name had to carry
// the team name itself. The team name is now added automatically, so a member
// name that still contains it is refused.
const teamPlaceholder = "%{team}"

// Profile is one engine choice, named, kept in a profiles.yaml.
type Profile struct {
	Name string
	Kind string
	Args []string
	Env  map[string]string
	Path string // the profiles.yaml it came from
}

// MaySpawnWorker is one purpose-to-profile pair: what a member may hire a worker
// for, and the profile that worker runs.
type MaySpawnWorker struct {
	Purpose string
	Profile string
}

// MaySpawn is what a member may hire. It is stated in the prompt, never enforced.
type MaySpawn struct {
	Workers []MaySpawnWorker
}

// ProfileFor returns the profile may_spawn pairs with a purpose.
func (m *MaySpawn) ProfileFor(purpose string) (string, bool) {
	if m == nil {
		return "", false
	}
	for _, worker := range m.Workers {
		if worker.Purpose == purpose {
			return worker.Profile, true
		}
	}
	return "", false
}

// Purposes lists the purposes a member may hire for, in the written order.
func (m *MaySpawn) Purposes() []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m.Workers))
	for i, worker := range m.Workers {
		out[i] = worker.Purpose
	}
	return out
}

// Member is one member of the roster, with its prompt already built.
//
// Name is the name herdr sees: "<team>-<member>", where team is the name the team
// runs under (--as, else the settings name). ShortName is the short name written in
// team.yaml; the commands that take a member (refresh, reprompt, hire --from)
// take the short name or the herdr name.
type Member struct {
	Name      string
	ShortName string
	Role      string
	Kind      string
	Args      []string
	Env       map[string]string
	Cwd       string
	Prompt    string
	Profile   string
	MaySpawn  *MaySpawn

	// Provenance: the role file, and the documents this member's prompt read.
	RoleSource    source.Doc
	HasRoleSource bool
	DocsRead      []source.Doc
}

// Config is a resolved team.
type Config struct {
	Team       string
	Definition string // the team definition name: the teams/<name>/ directory
	Version    int
	// Mission is what team.yaml's mission: says the team is for, as written:
	// one line, and empty when the file writes none. A mission written over
	// several lines keeps them here; MissionLine is what the output shows.
	Mission  string
	Members  []Member
	Profiles map[string]Profile

	// Provenance: where the effective values came from.
	TeamYAML     source.Doc
	ProfilesYAML string // the profiles.yaml files read, local first; empty when none
	Layer        string
	Docs         []source.Doc // the documents the team lists, in order

	// Which layers define this team name, for ls/show.
	GlobalTeamYAML string
	LocalTeamYAML  string
}

// MissionLine is the mission as one line: the first line of what team.yaml
// writes, or "" when the file writes no mission. A mission written over several
// lines is not joined: hergy validate says it should be one line, and the
// output shows the first line rather than a paragraph.
func (c *Config) MissionLine() string {
	line, _, _ := strings.Cut(c.Mission, "\n")
	return strings.TrimSpace(line)
}

// UsesProfiles reports whether any member reads a named profile, so the caller
// shows profiles.yaml only when it is actually in effect.
func (c *Config) UsesProfiles() bool {
	if c.ProfilesYAML == "" {
		return false
	}
	for i := range c.Members {
		if c.Members[i].Profile != "" {
			return true
		}
		if c.Members[i].MaySpawn != nil {
			return true
		}
	}
	return false
}

// Member returns the named member: by its herdr name first, then by its short name,
// so both the internal lookups (which use the herdr name) and the command-line
// member names (the short member name) resolve.
func (c *Config) Member(name string) (*Member, error) {
	for i := range c.Members {
		if c.Members[i].Name == name {
			return &c.Members[i], nil
		}
	}
	for i := range c.Members {
		if c.Members[i].ShortName == name {
			return &c.Members[i], nil
		}
	}
	return nil, i18n.Errorf("error.no_member", name)
}

// Select returns every member, or only the named one.
func (c *Config) Select(name string) ([]*Member, error) {
	if name == "" {
		out := make([]*Member, len(c.Members))
		for i := range c.Members {
			out[i] = &c.Members[i]
		}
		return out, nil
	}
	member, err := c.Member(name)
	if err != nil {
		return nil, err
	}
	return []*Member{member}, nil
}

// Input is everything Parse needs.
type Input struct {
	YAML         string
	TeamName     string // the name the team was resolved by (its directory name)
	RunName      string // the team name at run time: --as, else TeamName
	Layer        string
	TeamYAMLPath string
	TeamDir      string
	Profiles     map[string]Profile
	ProfilesYAML string
	Base         string // the project root, for cwd values and the embedded hire command
	Reader       source.Reader
	Executable   string // the hergy binary, embedded in the hire command shown to members

	GlobalTeamYAML string
	LocalTeamYAML  string
}

type stringList []string

func (s *stringList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var one string
		if err := value.Decode(&one); err != nil {
			return err
		}
		*s = []string{one}
		return nil
	}
	var many []string
	if err := value.Decode(&many); err != nil {
		return err
	}
	*s = many
	return nil
}

// The may_spawn key is kept as a raw node, because yaml.v3 does not call a
// custom unmarshaler for `may_spawn:` (null); the node tells "absent" apart from
// "null". Unknown keys inside it are checked by hand, since KnownFields does not
// descend into a yaml.Node.
type rawTeam struct {
	Version *int        `yaml:"version"`
	Mission *string     `yaml:"mission"`
	Team    string      `yaml:"team"`
	Cwd     string      `yaml:"cwd"`
	Prompt  *string     `yaml:"prompt"`
	Docs    *stringList `yaml:"docs"`
	Roles   *stringList `yaml:"roles"`
	Members []rawMember `yaml:"members"`
}

// rawMember is one member of team.yaml. kind, args and env are not a member's
// to write: they belong to the profile the member names (I7). They are still
// read, only so that a file which writes one is stopped with the sentence that
// says where it goes, rather than with an unknown-key complaint.
type rawMember struct {
	Name     string         `yaml:"name"`
	Role     string         `yaml:"role"`
	Profile  string         `yaml:"profile"`
	Kind     string         `yaml:"kind"`
	Args     []any          `yaml:"args"`
	Env      map[string]any `yaml:"env"`
	Cwd      string         `yaml:"cwd"`
	Prompt   *string        `yaml:"prompt"`
	MaySpawn yaml.Node      `yaml:"may_spawn"`
}

type rawMaySpawn struct {
	Workers []rawMaySpawnWorker `yaml:"workers"`
}

type rawMaySpawnWorker struct {
	Purpose string `yaml:"purpose"`
	Profile string `yaml:"profile"`
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

type rawConfig struct {
	Version *int   `yaml:"version"`
	Lang    string `yaml:"lang"`
}

// ParseProfiles reads one profiles.yaml, checking its version
// before anything else and then rejecting unknown keys.
func ParseProfiles(text, path string) (map[string]Profile, error) {
	if _, err := readVersion(text, "profiles.yaml", path); err != nil {
		return nil, err
	}
	var raw rawProfiles
	if err := strictDecode(text, &raw); err != nil {
		return nil, fmt.Errorf("%s: %s", path, cleanYAMLError(err))
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

// ParseConfig reads the global config.yaml and returns the language it names, or
// "" when it names none. It is held to the same strictness as team.yaml: the
// version must be present and 1, and any unknown key is an error with a reason.
func ParseConfig(text, path string) (string, error) {
	if _, err := readVersion(text, "config.yaml", path); err != nil {
		return "", err
	}
	var raw rawConfig
	if err := strictDecode(text, &raw); err != nil {
		return "", fmt.Errorf("%s: %s", path, cleanYAMLError(err))
	}
	return raw.Lang, nil
}

// GlobalYAML is the global config.yaml hergy new writes when the file is
// missing: the version every settings file carries, and the language that run
// displayed in. It holds exactly the two keys ParseConfig reads.
func GlobalYAML(lang string) string {
	return fmt.Sprintf("version: 1\nlang: %s\n", lang)
}

// readVersion decodes only the version, so a missing or wrong version is
// reported before any unknown-key complaint.
func readVersion(text, what, path string) (int, error) {
	var only struct {
		Version *int `yaml:"version"`
	}
	dec := yaml.NewDecoder(strings.NewReader(text))
	if err := dec.Decode(&only); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, i18n.Errorf("error.file_empty", what, path)
		}
		return 0, fmt.Errorf("%s: %s", path, cleanYAMLError(err))
	}
	if err := checkVersion(only.Version, what, path); err != nil {
		return 0, err
	}
	return *only.Version, nil
}

func strictDecode(text string, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	return dec.Decode(out)
}

func checkVersion(version *int, what, path string) error {
	if version == nil {
		return i18n.Errorf("error.version_missing", what, path)
	}
	if *version != 1 {
		return i18n.Errorf("error.version_wrong", what, *version, path)
	}
	return nil
}

// Parse builds a Config from team.yaml text.
func Parse(in Input) (*Config, error) {
	if in.RunName == "" {
		in.RunName = in.TeamName
	}
	version, err := readVersion(in.YAML, "team.yaml", in.TeamYAMLPath)
	if err != nil {
		return nil, err
	}
	var raw rawTeam
	if err := strictDecode(in.YAML, &raw); err != nil {
		return nil, fmt.Errorf("%s: %s", in.TeamYAMLPath, cleanYAMLError(err))
	}
	if raw.Team != "" && raw.Team != in.TeamName {
		return nil, i18n.Errorf("error.team_name_mismatch",
			in.TeamYAMLPath, raw.Team, in.TeamName)
	}
	if len(raw.Members) == 0 {
		return nil, i18n.Errorf("error.members_empty", in.TeamYAMLPath)
	}

	docs, err := expandDocs(raw.Docs, in)
	if err != nil {
		return nil, err
	}
	roleDirs, err := expandRoleDirs(raw.Roles, in)
	if err != nil {
		return nil, err
	}

	members, err := parseMembers(raw, in, docs, roleDirs)
	if err != nil {
		return nil, err
	}

	return &Config{
		Team:           in.RunName,
		Definition:     in.TeamName,
		Version:        version,
		Mission:        missionOf(raw.Mission),
		Members:        members,
		Profiles:       in.Profiles,
		TeamYAML:       source.Doc{Layer: in.Layer, Path: in.TeamYAMLPath},
		ProfilesYAML:   in.ProfilesYAML,
		Layer:          in.Layer,
		Docs:           docs,
		GlobalTeamYAML: in.GlobalTeamYAML,
		LocalTeamYAML:  in.LocalTeamYAML,
	}, nil
}

// missionOf is the mission team.yaml writes, with the ends of what it wrote
// trimmed: a block scalar ends in a newline, which is not part of the line. The
// lines inside it stay as they are; validate says when there is more than one.
func missionOf(raw *string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(*raw)
}

// expandDocs: with no docs key, no documents go in. With a docs key, exactly
// what it lists goes in, in order. A file entry is a rule file; a directory
// entry is the *.yaml files directly inside it, by name. hergy has no built-in
// layer, so a "builtin:" entry is an error, and a document left as Markdown
// stops here with the way to move it over.
func expandDocs(entries *stringList, in Input) ([]source.Doc, error) {
	if entries == nil {
		return nil, nil
	}
	var docs []source.Doc
	for _, entry := range *entries {
		doc, err := resolveEntry(entry, in.TeamDir, in.Layer)
		if err != nil {
			return nil, i18n.Errorf("error.docs_prefix", err)
		}
		switch {
		case in.Reader.IsDir(doc):
			more, err := docsIn(in.Reader, doc, entry)
			if err != nil {
				return nil, err
			}
			docs = append(docs, more...)
		case in.Reader.IsFile(doc):
			if isMarkdown(doc.Path) {
				return nil, i18n.Errorf("error.docs_markdown", entry)
			}
			docs = append(docs, doc)
		default:
			return nil, i18n.Errorf("error.docs_missing", entry)
		}
	}
	return docs, nil
}

// docsIn expands one directory entry: its *.yaml files, or the error that says
// the directory is still Markdown.
func docsIn(reader source.Reader, dir source.Doc, entry string) ([]source.Doc, error) {
	more, err := reader.YAMLIn(dir)
	if err != nil {
		return nil, err
	}
	if len(more) > 0 {
		return more, nil
	}
	if markdown, err := reader.MarkdownIn(dir); err == nil && len(markdown) > 0 {
		return nil, i18n.Errorf("error.docs_dir_markdown", entry)
	}
	return nil, nil
}

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

// expandRoleDirs is the directories a team lists in roles; there is no built-in
// fallback. With no roles key, the team's own roles/ directory is used.
func expandRoleDirs(entries *stringList, in Input) ([]source.Doc, error) {
	if entries == nil {
		entries = &stringList{"roles"}
	}
	dirs := make([]source.Doc, 0, len(*entries))
	seen := map[string]bool{}
	for _, entry := range *entries {
		dir, err := resolveEntry(entry, in.TeamDir, in.Layer)
		if err != nil {
			return nil, i18n.Errorf("error.roles_prefix", err)
		}
		if seen[dir.Path] {
			continue
		}
		seen[dir.Path] = true
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

// resolveEntry turns a docs/roles entry into a Doc. hergy keeps no rules of its
// own, so a "builtin:" entry is refused with the reason.
func resolveEntry(entry, base, layer string) (source.Doc, error) {
	if strings.HasPrefix(entry, "builtin:") {
		return source.Doc{}, i18n.Errorf("error.builtin_not_allowed", entry)
	}
	return source.EntryDoc(entry, base, layer), nil
}

// parseMembers builds every member, then fills each prompt once the roster is known.
func parseMembers(raw rawTeam, in Input, docs, roleDirs []source.Doc) ([]Member, error) {
	members := make([]Member, 0, len(raw.Members))
	templates := make([]string, 0, len(raw.Members))
	for i, rm := range raw.Members {
		label := fmt.Sprintf("members[%d]", i)
		// The team name is added to every member name automatically, so a
		// literal %{team} is a leftover that would only make the herdr name
		// carry the team twice.
		if strings.Contains(rm.Name, teamPlaceholder) {
			return nil, i18n.Errorf("error.name_team_placeholder", label)
		}
		member, err := parseMember(rm, raw.Cwd, in, i)
		if err != nil {
			return nil, err
		}
		template, err := memberTemplate(rm, raw, label)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
		templates = append(templates, template)
	}
	seen := map[string]bool{}
	for _, member := range members {
		if seen[member.Name] {
			return nil, i18n.Errorf("error.duplicate_member", member.Name)
		}
		seen[member.Name] = true
	}

	roster := make([]prompt.RosterEntry, len(members))
	for i, member := range members {
		roster[i] = prompt.RosterEntry{Name: member.Name, Role: member.Role}
	}
	for i := range members {
		if err := buildPrompt(&members[i], templates[i], in, docs, roleDirs, roster); err != nil {
			return nil, err
		}
	}
	return members, nil
}

// memberTemplate is the member's prompt, else the team's, else the default. An
// explicitly empty prompt is an error rather than a silent fallback.
func memberTemplate(rm rawMember, raw rawTeam, label string) (string, error) {
	if rm.Prompt != nil {
		if strings.TrimSpace(*rm.Prompt) == "" {
			return "", i18n.Errorf("error.prompt_empty_member", label)
		}
		return *rm.Prompt, nil
	}
	if raw.Prompt != nil {
		if strings.TrimSpace(*raw.Prompt) == "" {
			return "", i18n.Errorf("error.prompt_empty_team")
		}
		return *raw.Prompt, nil
	}
	return prompt.MemberTemplate(), nil
}

func parseMember(rm rawMember, teamCwd string, in Input, index int) (Member, error) {
	label := fmt.Sprintf("members[%d]", index)
	if !NamePattern.MatchString(rm.Name) {
		return Member{}, i18n.Errorf("error.name_pattern", label)
	}
	// herdr sees the member as "<team>-<member>". The team name is checked short
	// enough for this to fit, but a long member name can still overflow the limit.
	name := in.RunName + "-" + rm.Name
	if !NamePattern.MatchString(name) {
		return Member{}, i18n.Errorf("error.herdr_name_pattern", name)
	}
	if strings.TrimSpace(rm.Role) == "" {
		return Member{}, i18n.Errorf("error.role_empty", label)
	}
	kind, args, env, profile, err := engine(rm, label, in.Profiles)
	if err != nil {
		return Member{}, err
	}
	cwd := rm.Cwd
	if cwd == "" {
		cwd = teamCwd
	}
	if cwd == "" {
		cwd = "."
	}
	if !isAbs(cwd) {
		cwd = joinPath(in.Base, cwd)
	}
	var maySpawn *MaySpawn
	if rm.MaySpawn.Kind != 0 {
		maySpawn, err = parseMaySpawnNode(rm.MaySpawn, label+".may_spawn", in.Profiles)
		if err != nil {
			return Member{}, err
		}
	}
	return Member{
		Name:      name,
		ShortName: rm.Name,
		Role:      rm.Role,
		Kind:      kind,
		Args:      args,
		Env:       env,
		Cwd:       cwd,
		Profile:   profile,
		MaySpawn:  maySpawn,
	}, nil
}

// engine is the profile a member names. kind, args and env are the profile's
// (I7): a member that writes one of them is stopped here, with the sentence that
// says where they go, so the file is fixed before a team boots from it.
func engine(rm rawMember, label string, profiles map[string]Profile) (string, []string, map[string]string, string, error) {
	if key := inlineEngineKey(rm); key != "" {
		return "", nil, nil, "", i18n.Errorf("error.member_writes_engine", rm.Name, key, key)
	}
	if rm.Profile == "" {
		return "", nil, nil, "", i18n.Errorf("error.member_needs_profile", rm.Name)
	}
	profile, ok := profiles[rm.Profile]
	if !ok {
		return "", nil, nil, "", i18n.Errorf("error.no_profile", label, rm.Profile)
	}
	return profile.Kind, profile.Args, profile.Env, profile.Name, nil
}

// inlineEngineKey is the first of kind, args and env a member writes itself, or
// "" when it writes none. Each of the three is refused the same way.
func inlineEngineKey(rm rawMember) string {
	switch {
	case rm.Kind != "":
		return "kind"
	case rm.Args != nil:
		return "args"
	case rm.Env != nil:
		return "env"
	}
	return ""
}

// parseMaySpawnNode reads may_spawn by hand: the node tells "absent" apart from
// "null", and each worker's keys are checked because KnownFields does not descend
// into a yaml.Node. The old purposes/profiles form is refused with the new one.
func parseMaySpawnNode(node yaml.Node, label string, profiles map[string]Profile) (*MaySpawn, error) {
	if node.Tag == "!!null" {
		return nil, i18n.Errorf("error.must_be_mapping_not_null", label)
	}
	if node.Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.must_be_mapping", label)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch key := node.Content[i].Value; key {
		case "workers":
		case "purposes", "profiles":
			return nil, i18n.Errorf("error.mayspawn_old_form", label)
		default:
			return nil, i18n.Errorf("error.unknown_keys", label, key)
		}
	}
	workers := mappingValue(&node, "workers")
	if workers == nil || workers.Tag == "!!null" {
		return nil, i18n.Errorf("error.mayspawn_workers_empty", label)
	}
	if workers.Kind != yaml.SequenceNode {
		return nil, i18n.Errorf("error.mayspawn_workers_list", label)
	}
	if len(workers.Content) == 0 {
		return nil, i18n.Errorf("error.mayspawn_workers_empty", label)
	}
	out := &MaySpawn{}
	seen := map[string]bool{}
	for i, worker := range workers.Content {
		item := fmt.Sprintf("%s.workers[%d]", label, i)
		if worker.Kind != yaml.MappingNode {
			return nil, i18n.Errorf("error.must_be_mapping", item)
		}
		for j := 0; j+1 < len(worker.Content); j += 2 {
			if key := worker.Content[j].Value; key != "purpose" && key != "profile" {
				return nil, i18n.Errorf("error.unknown_keys", item, key)
			}
		}
		var raw rawMaySpawnWorker
		if err := worker.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%s: %s", item, cleanYAMLError(err))
		}
		if strings.TrimSpace(raw.Purpose) == "" {
			return nil, i18n.Errorf("error.mayspawn_purpose_empty", item)
		}
		if raw.Profile == "" {
			return nil, i18n.Errorf("error.mayspawn_profile_empty", item)
		}
		if _, ok := profiles[raw.Profile]; !ok {
			return nil, i18n.Errorf("error.mayspawn_no_profile", item, raw.Profile)
		}
		if seen[raw.Purpose] {
			return nil, i18n.Errorf("error.mayspawn_duplicate_purpose", label, raw.Purpose)
		}
		seen[raw.Purpose] = true
		out.Workers = append(out.Workers, MaySpawnWorker(raw))
	}
	return out, nil
}

// mappingValue returns the value node for a key in a mapping node, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// buildPrompt expands one member's template and records what it read. The rule
// files that go in are parsed and rendered to Markdown, and the set they form is
// checked as a whole: a rule id, a report name and a reference are held to the
// documents this member actually reads.
func buildPrompt(member *Member, template string, in Input, docs, roleDirs []source.Doc, roster []prompt.RosterEntry) error {
	var buildErr error
	var roleSource *source.Doc
	var docsRead []source.Doc
	var readDocs []*rules.Document

	readDoc := func(doc source.Doc) string {
		parsed, err := readRuleFile(in.Reader, doc)
		if err != nil {
			if buildErr == nil {
				buildErr = err
			}
			return ""
		}
		readDocs = append(readDocs, parsed)
		return strings.TrimRight(parsed.Markdown(), "\n")
	}

	parts := map[string]func() string{
		"team": func() string { return in.RunName },
		"role": func() string { return member.Role },
		"name": func() string { return member.Name },
		"role_doc": func() string {
			if buildErr != nil {
				return ""
			}
			doc, err := roleFile(member.Role, roleDirs, in.Reader)
			if err != nil {
				buildErr = err
				return ""
			}
			found := doc
			roleSource = &found
			return readDoc(doc)
		},
		"docs": func() string {
			if buildErr != nil {
				return ""
			}
			texts := make([]string, 0, len(docs))
			for _, doc := range docs {
				texts = append(texts, readDoc(doc))
			}
			docsRead = append(docsRead, docs...)
			if len(texts) == 0 {
				return ""
			}
			return "\n" + strings.Join(texts, "\n\n") + "\n"
		},
		"roster": func() string { return prompt.Roster(roster, member.Name) },
		"herdr": func() string {
			return prompt.HerdrMember(i18n.Tf("prompt.self_intro_member", in.RunName, member.Role, member.Name))
		},
		"spawn": func() string {
			if member.MaySpawn == nil {
				return ""
			}
			workers := make([]prompt.SpawnPair, len(member.MaySpawn.Workers))
			for i, worker := range member.MaySpawn.Workers {
				workers[i] = prompt.SpawnPair{Purpose: worker.Purpose, Profile: worker.Profile}
			}
			return prompt.SpawnNote(member.Name, workers, hireCommand(in, member.ShortName))
		},
	}
	text, err := prompt.Expand(template, parts)
	if err != nil {
		return err
	}
	if buildErr != nil {
		return buildErr
	}
	if err := rules.Check(readDocs); err != nil {
		return err
	}
	member.Prompt = text
	member.DocsRead = docsRead
	if roleSource != nil {
		member.RoleSource = *roleSource
		member.HasRoleSource = true
	}
	return nil
}

// readRuleFile reads one rule file and renders it to the Markdown a member's
// prompt carries.
func readRuleFile(reader source.Reader, doc source.Doc) (*rules.Document, error) {
	text, err := reader.Read(doc)
	if err != nil {
		return nil, err
	}
	return rules.Parse(text, doc.Path)
}

// hireCommand is embedded in a member's prompt. It names the exact team, layer
// and project, so a hire works even when the member runs outside the project or
// the same name exists in both layers. --team carries the team name the member
// runs under (hire takes a team name, not a team definition name), so the hire
// finds the running team even when --as named it. The --from value is the
// hiring member's short name: the command line takes the short name, and hergy
// adds the team name to the herdr name itself.
func hireCommand(in Input, shortName string) string {
	layerFlag := "--local"
	if in.Layer == source.LayerGlobal {
		layerFlag = "--global"
	}
	executable := in.Executable
	if executable == "" {
		executable = "hergy"
	}
	return i18n.Tf("hire.command",
		shellQuote(executable), in.RunName, layerFlag, i18n.Language(), shellQuote(in.Base), shortName)
}

func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'\\$&|;<>()*?[]#~") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// roleFile is the first role directory that has the file; files are never merged.
func roleFile(role string, dirs []source.Doc, reader source.Reader) (source.Doc, error) {
	for _, dir := range dirs {
		doc := source.Doc{Layer: dir.Layer, Path: joinPath(dir.Path, role+".yaml")}
		if reader.IsFile(doc) {
			return doc, nil
		}
	}
	return source.Doc{}, missingRole(role, dirs, reader)
}

// missingRole says which it is: a role file that is still Markdown, or no role
// file at all.
func missingRole(role string, dirs []source.Doc, reader source.Reader) error {
	for _, dir := range dirs {
		doc := source.Doc{Layer: dir.Layer, Path: joinPath(dir.Path, role+".md")}
		if reader.IsFile(doc) {
			return i18n.Errorf("error.role_markdown", role, doc.Path)
		}
	}
	if len(dirs) == 0 {
		return i18n.Errorf("error.no_role_file_no_dirs", role)
	}
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = dir.Path
	}
	return i18n.Errorf("error.no_role_file_paths", role, strings.Join(paths, ", "))
}

func isAbs(p string) bool         { return filepath.IsAbs(p) }
func joinPath(a, b string) string { return filepath.Join(a, b) }

// cleanYAMLError drops the Go type names yaml puts in its messages, and points
// the removed team-level `profiles` at where profiles went.
func cleanYAMLError(err error) string {
	msg := typeNameRE.ReplaceAllString(err.Error(), "")
	msg = dangleRE.ReplaceAllString(msg, "")
	msg = strings.ReplaceAll(msg, "field profiles not found", "field profiles not found"+i18n.T("error.profiles_moved"))
	return strings.TrimSpace(msg)
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
