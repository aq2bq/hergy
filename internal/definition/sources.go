package definition

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// teamPlaceholder is a leftover from the form where a member name had to carry
// the team name itself. The team name is now added automatically, so a member
// name that still contains it is refused.
const teamPlaceholder = "%{team}"

// Input is everything Read needs. The reader decides what a path is: the
// definition is read from the filesystem, and a screen may read an edit that is
// not written yet.
type Input struct {
	YAML         string
	Name         string // the definition name: the teams/<name>/ directory
	Layer        string
	TeamYAMLPath string
	TeamDir      string
	Profiles     map[string]profile.Profile
	Base         string // the project root, for cwd values
	Reader       source.Reader
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

// Read reads one team definition: the members (the frames), the workers each
// member may hire, the documents every member reads and where a role document
// is looked for. It builds the graph, so the readers of a definition share one
// graph (d-11f8). It does not build a prompt: that is a projection
// (prompt.Member), and this package does not import it.
func Read(in Input) (*Definition, error) {
	version, err := profile.ReadVersion(in.YAML, "team.yaml", in.TeamYAMLPath)
	if err != nil {
		return nil, err
	}
	var raw rawTeam
	if err := profile.StrictDecode(in.YAML, &raw); err != nil {
		return nil, fmt.Errorf("%s: %s", in.TeamYAMLPath, profile.CleanError(err))
	}
	if raw.Team != "" && raw.Team != in.Name {
		return nil, i18n.Errorf("error.team_name_mismatch", in.TeamYAMLPath, raw.Team, in.Name)
	}
	mission, err := missionOf(raw.Mission)
	if err != nil {
		return nil, err
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
	prompt, err := promptOf(raw.Prompt)
	if err != nil {
		return nil, err
	}
	members, workers, err := parseMembers(raw, in)
	if err != nil {
		return nil, err
	}

	return &Definition{
		Name:     in.Name,
		Mission:  mission,
		Graph:    NewGraph(members, workers),
		Dir:      in.TeamDir,
		TeamYAML: in.TeamYAMLPath,
		Layer:    in.Layer,
		Version:  version,
		Docs:     docs,
		RoleDirs: roleDirs,
		Prompt:   prompt,
	}, nil
}

// promptOf is the team's own prompt template: the text it wrote, or "" when it
// wrote none (the default in the locales is used). An explicitly empty prompt is
// an error rather than a silent fallback.
func promptOf(raw *string) (string, error) {
	if raw == nil {
		return "", nil
	}
	if strings.TrimSpace(*raw) == "" {
		return "", i18n.Errorf("error.prompt_empty_team")
	}
	return *raw, nil
}

// missionOf is the mission team.yaml writes: one line saying what the team is
// for (I15). It is required, so a file that writes none, writes an empty one, or
// writes one over more than one line is stopped here. The ends are trimmed
// first, so a block scalar's trailing newline is not a second line.
func missionOf(raw *string) (string, error) {
	if raw == nil {
		return "", i18n.Errorf("error.mission_missing")
	}
	mission := strings.TrimSpace(*raw)
	if mission == "" {
		return "", i18n.Errorf("error.mission_missing")
	}
	if strings.Contains(mission, "\n") {
		return "", i18n.Errorf("error.mission_multiline")
	}
	return mission, nil
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

// parseMembers builds the member frames and the worker nodes, refusing the ways
// a member breaks. Each member writes its profile by name (I7); the profile's
// own fields are read by the caller from the profiles it passed in.
func parseMembers(raw rawTeam, in Input) ([]MemberNode, []WorkerNode, error) {
	members := make([]MemberNode, 0, len(raw.Members))
	var workers []WorkerNode
	for i, rm := range raw.Members {
		label := fmt.Sprintf("members[%d]", i)
		// The team name is added to every member name automatically, so a
		// literal %{team} is a leftover that would only make the herdr name
		// carry the team twice.
		if strings.Contains(rm.Name, teamPlaceholder) {
			return nil, nil, i18n.Errorf("error.name_team_placeholder", label)
		}
		member, err := parseMember(rm, raw.Cwd, in, i)
		if err != nil {
			return nil, nil, err
		}
		more, err := parseMaySpawn(rm, in, i)
		if err != nil {
			return nil, nil, err
		}
		members = append(members, member)
		workers = append(workers, more...)
	}
	seen := map[string]bool{}
	for _, member := range members {
		if seen[member.Name] {
			return nil, nil, i18n.Errorf("error.duplicate_member", member.Name)
		}
		seen[member.Name] = true
	}
	return members, workers, nil
}

// memberPrompt is the prompt the member writes over the team's own: an
// explicitly empty one is an error, so a file that would silently fall back is
// stopped. The prompt itself is the template prompt.Member expands.
func memberPrompt(rm rawMember, label string) (*string, error) {
	if rm.Prompt == nil {
		return nil, nil
	}
	if strings.TrimSpace(*rm.Prompt) == "" {
		return nil, i18n.Errorf("error.prompt_empty_member", label)
	}
	return rm.Prompt, nil
}

func parseMember(rm rawMember, teamCwd string, in Input, index int) (MemberNode, error) {
	label := fmt.Sprintf("members[%d]", index)
	if !profile.NamePattern.MatchString(rm.Name) {
		return MemberNode{}, i18n.Errorf("error.name_pattern", label)
	}
	if strings.TrimSpace(rm.Role) == "" {
		return MemberNode{}, i18n.Errorf("error.role_empty", label)
	}
	name, err := engineProfile(rm, label, in.Profiles)
	if err != nil {
		return MemberNode{}, err
	}
	cwd := rm.Cwd
	if cwd == "" {
		cwd = teamCwd
	}
	if cwd == "" {
		cwd = "."
	}
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(in.Base, cwd)
	}
	prompt, err := memberPrompt(rm, label)
	if err != nil {
		return MemberNode{}, err
	}
	node := MemberNode{Name: rm.Name, Role: rm.Role, Profile: name, Cwd: cwd}
	if prompt != nil {
		node.Prompt = *prompt
		node.HasPrompt = true
	}
	return node, nil
}

// engineProfile is the profile a member names. kind, args and env are the
// profile's (I7): a member that writes one of them is stopped here, with the
// sentence that says where they go, so the file is fixed before a team boots
// from it.
func engineProfile(rm rawMember, label string, profiles map[string]profile.Profile) (string, error) {
	if key := inlineEngineKey(rm); key != "" {
		return "", i18n.Errorf("error.member_writes_engine", rm.Name, key, key)
	}
	if rm.Profile == "" {
		return "", i18n.Errorf("error.member_needs_profile", rm.Name)
	}
	if _, ok := profiles[rm.Profile]; !ok {
		return "", i18n.Errorf("error.no_profile", label, rm.Profile)
	}
	return rm.Profile, nil
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

// parseMaySpawn reads one member's may_spawn into the worker nodes it hires, so
// the graph can make the two edges of each pair (I12c). It is read by hand: the
// node tells "absent" apart from "null", and each worker's keys are checked
// because KnownFields does not descend into a yaml.Node.
func parseMaySpawn(rm rawMember, in Input, index int) ([]WorkerNode, error) {
	if rm.MaySpawn.Kind == 0 {
		return nil, nil
	}
	label := fmt.Sprintf("members[%d].may_spawn", index)
	node := rm.MaySpawn
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
	out := make([]WorkerNode, 0, len(workers.Content))
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
			return nil, fmt.Errorf("%s: %s", item, profile.CleanError(err))
		}
		if strings.TrimSpace(raw.Purpose) == "" {
			return nil, i18n.Errorf("error.mayspawn_purpose_empty", item)
		}
		if raw.Profile == "" {
			return nil, i18n.Errorf("error.mayspawn_profile_empty", item)
		}
		if _, ok := in.Profiles[raw.Profile]; !ok {
			return nil, i18n.Errorf("error.mayspawn_no_profile", item, raw.Profile)
		}
		if seen[raw.Purpose] {
			return nil, i18n.Errorf("error.mayspawn_duplicate_purpose", label, raw.Purpose)
		}
		seen[raw.Purpose] = true
		out = append(out, WorkerNode{From: rm.Name, Purpose: raw.Purpose, Profile: raw.Profile})
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
