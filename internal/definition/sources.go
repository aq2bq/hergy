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
// "null", which is what the sentence that says it is gone needs (段 5). Unknown
// keys inside it are checked by hand, since KnownFields does not descend into a
// yaml.Node.
//
// docs: and roles: are gone (段 4). They are still read, only so that a file
// which writes one is stopped with the sentence that says where the documents
// went, rather than with an unknown-key complaint.
type rawTeam struct {
	Version *int        `yaml:"version"`
	Mission *string     `yaml:"mission"`
	Team    string      `yaml:"team"`
	Cwd     string      `yaml:"cwd"`
	Prompt  *string     `yaml:"prompt"`
	Docs    *stringList `yaml:"docs"`
	Roles   *stringList `yaml:"roles"`
	Members []rawMember `yaml:"members"`
	Workers []rawWorker `yaml:"workers"`
	Turns   *stringList `yaml:"turns"`
}

// rawWorker is one worker a member may hire: who hires it, what it is for, and
// the profile it runs (I12c, 段 5).
type rawWorker struct {
	From    string `yaml:"from"`
	Purpose string `yaml:"purpose"`
	Profile string `yaml:"profile"`
}

// rawMember is one member of team.yaml. kind, args and env are not a member's
// to write: they belong to the profile the member names (I7). They are still
// read, only so that a file which writes one is stopped with the sentence that
// says where it goes, rather than with an unknown-key complaint. may_spawn is
// the same: the workers moved to the top level (段 5).
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

// spawnPlaceholder is the old name of the part of a prompt template that lists
// the workers a member may hire. The template that still writes it is stopped
// with the name of the part that replaced it (段 5).
const spawnPlaceholder = "%{spawn}"

// Read reads one team definition: the members (the frames), the workers each
// member may hire, and the team's own agreement. It builds the graph, so the
// readers of a definition share one graph (d-11f8). It does not build a prompt:
// that is a projection (prompt.Member), and this package does not import it.
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
	mission, err := missionOf(raw.Mission, in.TeamYAMLPath)
	if err != nil {
		return nil, err
	}
	if len(raw.Members) == 0 {
		return nil, i18n.Errorf("error.members_empty", in.TeamYAMLPath)
	}
	if raw.Docs != nil {
		return nil, i18n.Errorf("error.docs_gone", in.Name)
	}
	if raw.Roles != nil {
		return nil, i18n.Errorf("error.roles_gone")
	}
	agreement := teamAgreement(in)
	prompt, err := promptOf(raw.Prompt)
	if err != nil {
		return nil, err
	}
	members, err := parseMembers(raw, in)
	if err != nil {
		return nil, err
	}
	workers, err := parseWorkers(raw, in)
	if err != nil {
		return nil, err
	}
	turns, err := parseTurns(raw.Turns)
	if err != nil {
		return nil, err
	}
	graph := NewGraph(members, workers, turns)
	// A graph whose turns the definition cannot hold stops every command that
	// reads it (I12a, I12b, d-edd4): no team runs without a turn out of each
	// member and of the human, and a turn reaching the human.
	if problems := graph.Check(); len(problems) > 0 {
		return nil, &GraphError{Path: in.TeamYAMLPath, Problems: problems}
	}

	return &Definition{
		Name:         in.Name,
		Mission:      mission,
		Graph:        graph,
		Dir:          in.TeamDir,
		TeamYAML:     in.TeamYAMLPath,
		Layer:        in.Layer,
		Version:      version,
		Agreement:    agreement,
		HasAgreement: in.Reader.IsFile(agreement),
		Prompt:       prompt,
	}, nil
}

// teamAgreement is where a team's own agreement sits: agreement.yaml in the
// definition's own folder. It is not read here; the reader is asked whether it
// is there, and prompt.Member reads it when it builds a member's prompt.
func teamAgreement(in Input) source.Doc {
	return source.Doc{Layer: in.Layer, Path: filepath.Join(in.TeamDir, "agreement.yaml")}
}

// promptOf is the team's own prompt template: the text it wrote, or "" when it
// wrote none (the default in the locales is used). An explicitly empty prompt is
// an error rather than a silent fallback, and so is a template that still writes
// the old %{spawn}: expanding it would only stop as an unknown part.
func promptOf(raw *string) (string, error) {
	if raw == nil {
		return "", nil
	}
	if strings.TrimSpace(*raw) == "" {
		return "", i18n.Errorf("error.prompt_empty_team")
	}
	if strings.Contains(*raw, spawnPlaceholder) {
		return "", i18n.Errorf("error.spawn_placeholder_gone")
	}
	return *raw, nil
}

// missionOf is the mission team.yaml writes: one line saying what the team is
// for (I15). It is required, so a file that writes none, writes an empty one, or
// writes one over more than one line is stopped here, with the file's path the
// way the other reading errors carry it. The ends are trimmed first, so a block
// scalar's trailing newline is not a second line.
func missionOf(raw *string, path string) (string, error) {
	if raw == nil {
		return "", i18n.Errorf("error.mission_missing", path)
	}
	mission := strings.TrimSpace(*raw)
	if mission == "" {
		return "", i18n.Errorf("error.mission_missing", path)
	}
	if strings.Contains(mission, "\n") {
		return "", i18n.Errorf("error.mission_multiline", path)
	}
	return mission, nil
}

// parseMembers builds the member frames, refusing the ways a member breaks. Each
// member writes its profile by name (I7); the profile's own fields are read by
// the caller from the profiles it passed in.
func parseMembers(raw rawTeam, in Input) ([]MemberNode, error) {
	members := make([]MemberNode, 0, len(raw.Members))
	for i, rm := range raw.Members {
		label := fmt.Sprintf("members[%d]", i)
		// The team name is added to every member name automatically, so a
		// literal %{team} is a leftover that would only make the herdr name
		// carry the team twice.
		if strings.Contains(rm.Name, teamPlaceholder) {
			return nil, i18n.Errorf("error.name_team_placeholder", label)
		}
		// The workers moved to the top level (段 5): a member that still writes
		// them is stopped here, with the sentence that says where they went.
		if rm.MaySpawn.Kind != 0 {
			return nil, i18n.Errorf("error.may_spawn_gone")
		}
		member, err := parseMember(rm, raw.Cwd, in, i)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	seen := map[string]bool{}
	for _, member := range members {
		if seen[member.Name] {
			return nil, i18n.Errorf("error.duplicate_member", member.Name)
		}
		seen[member.Name] = true
	}
	return members, nil
}

// parseWorkers reads the top-level workers: into the worker nodes the graph
// makes the two edges of each from (I12c). Each one names the member that hires
// it, the purpose, and the profile the worker runs.
func parseWorkers(raw rawTeam, in Input) ([]WorkerNode, error) {
	workers := make([]WorkerNode, 0, len(raw.Workers))
	seen := map[string]bool{}
	for i, rw := range raw.Workers {
		label := fmt.Sprintf("workers[%d]", i)
		if strings.TrimSpace(rw.From) == "" {
			return nil, i18n.Errorf("error.workers_from_empty", label)
		}
		if strings.TrimSpace(rw.Purpose) == "" {
			return nil, i18n.Errorf("error.workers_purpose_empty", label)
		}
		if rw.Profile == "" {
			return nil, i18n.Errorf("error.workers_profile_empty", label)
		}
		if _, ok := in.Profiles[rw.Profile]; !ok {
			return nil, i18n.Errorf("error.workers_no_profile", label, rw.Profile)
		}
		// A worker's identity is the member that hires it and the purpose
		// (I12c): two workers of the same pair are one node twice.
		key := rw.From + "\x00" + rw.Purpose
		if seen[key] {
			return nil, i18n.Errorf("error.workers_duplicate_purpose", label, rw.From, rw.Purpose)
		}
		seen[key] = true
		workers = append(workers, WorkerNode(rw))
	}
	return workers, nil
}

// parseTurns reads the turns: lines, each "- a -> b", into the edges of the
// graph. Both ends are read as d-11f8 writes them: the human, a member's name,
// or "<kind>:<name>" for another kind, which is read and stopped.
func parseTurns(entries *stringList) ([]Edge, error) {
	if entries == nil {
		return nil, nil
	}
	turns := make([]Edge, 0, len(*entries))
	for i, line := range *entries {
		label := fmt.Sprintf("turns[%d]", i)
		from, to, err := parseTurn(line, label)
		if err != nil {
			return nil, err
		}
		turns = append(turns, Edge{From: from, To: to})
	}
	return turns, nil
}

// parseTurn reads one turns: line into its two ends. It is written "a -> b", one
// arrow, with a name on each side.
func parseTurn(line, label string) (NodeID, NodeID, error) {
	before, after, found := strings.Cut(line, arrow)
	if !found {
		return NodeID{}, NodeID{}, i18n.Errorf("error.turns_not_an_edge", label, strings.TrimSpace(line))
	}
	if strings.Contains(after, arrow) {
		return NodeID{}, NodeID{}, i18n.Errorf("error.turns_not_an_edge", label, strings.TrimSpace(line))
	}
	fromName, toName := strings.TrimSpace(before), strings.TrimSpace(after)
	if fromName == "" || toName == "" {
		return NodeID{}, NodeID{}, i18n.Errorf("error.turns_empty_end", label, strings.TrimSpace(line))
	}
	from, err := parseTurnName(fromName)
	if err != nil {
		return NodeID{}, NodeID{}, err
	}
	to, err := parseTurnName(toName)
	if err != nil {
		return NodeID{}, NodeID{}, err
	}
	return from, to, nil
}

// arrow is what a turns: line writes between its two ends. A member's name
// carries no arrow, so one arrow is the whole of it.
const arrow = "->"

// parseTurnName reads one end of a turns: line. A name written as
// "<kind>:<name>" is read (so the shape of the file does not change when a kind
// is added, d-11f8) and stopped: no kind but the human and a member may stand in
// turns: yet. Any other name is a member's; a name the team has no member for is
// kept, and Check reports it.
func parseTurnName(name string) (NodeID, error) {
	if strings.Contains(name, ":") {
		return NodeID{}, i18n.Errorf("error.turns_kind_prefix", name)
	}
	if name == nodeKinds[KindHuman].fixedName {
		return NodeID{Kind: KindHuman, Name: name}, nil
	}
	return memberID(name), nil
}

// memberPrompt is the prompt the member writes over the team's own: an
// explicitly empty one is an error, so a file that would silently fall back is
// stopped, and so is one that still writes the old %{spawn}. The prompt itself
// is the template prompt.Member expands.
func memberPrompt(rm rawMember, label string) (*string, error) {
	if rm.Prompt == nil {
		return nil, nil
	}
	if strings.TrimSpace(*rm.Prompt) == "" {
		return nil, i18n.Errorf("error.prompt_empty_member", label)
	}
	if strings.Contains(*rm.Prompt, spawnPlaceholder) {
		return nil, i18n.Errorf("error.spawn_placeholder_gone")
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
