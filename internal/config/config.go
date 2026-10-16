// Package config is on its way out (段 3). It used to own the reading of
// team.yaml; that is now definition.Read, and the prompt is prompt.Member. What
// is left here is the temporary shape the rest of hergy still takes (Config,
// Member): Parse is a thin wrapper over definition.Read that fills it in, and
// step 5 removes this package.
package config

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

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
	Profiles map[string]profile.Profile

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
	Profiles     map[string]profile.Profile
	ProfilesYAML string
	Base         string // the project root, for cwd values and the embedded hire command
	Reader       source.Reader
	Executable   string // the hergy binary, embedded in the hire command shown to members

	GlobalTeamYAML string
	LocalTeamYAML  string
}

type rawConfig struct {
	Version *int   `yaml:"version"`
	Lang    string `yaml:"lang"`
}

// ParseConfig reads the global config.yaml and returns the language it names, or
// "" when it names none. It is held to the same strictness as team.yaml: the
// version must be present and 1, and any unknown key is an error with a reason.
func ParseConfig(text, path string) (string, error) {
	if _, err := profile.ReadVersion(text, "config.yaml", path); err != nil {
		return "", err
	}
	var raw rawConfig
	if err := profile.StrictDecode(text, &raw); err != nil {
		return "", fmt.Errorf("%s: %s", path, profile.CleanError(err))
	}
	return raw.Lang, nil
}

// GlobalYAML is the global config.yaml hergy new writes when the file is
// missing: the version every settings file carries, and the language that run
// displayed in. It holds exactly the two keys ParseConfig reads.
func GlobalYAML(lang string) string {
	return fmt.Sprintf("version: 1\nlang: %s\n", lang)
}

// Parse builds a Config from team.yaml text: definition.Read reads the
// definition, and prompt.Member builds each member's prompt. It is the
// migration shape until step 5 removes Config and Member.
func Parse(in Input) (*Config, error) {
	if in.RunName == "" {
		in.RunName = in.TeamName
	}
	def, err := definition.Read(definition.Input{
		YAML:         in.YAML,
		Name:         in.TeamName,
		Layer:        in.Layer,
		TeamYAMLPath: in.TeamYAMLPath,
		TeamDir:      in.TeamDir,
		Profiles:     in.Profiles,
		Base:         in.Base,
		Reader:       in.Reader,
	})
	if err != nil {
		return nil, err
	}
	run := prompt.Run{Team: in.RunName, Executable: in.Executable, Base: in.Base, Reader: in.Reader}
	members := make([]Member, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		// herdr sees the member as "<team>-<member>"; the team name is checked
		// short enough for this to fit, but a long member name can still
		// overflow the limit.
		herdr := in.RunName + "-" + node.Name
		if !profile.NamePattern.MatchString(herdr) {
			return nil, i18n.Errorf("error.herdr_name_pattern", herdr)
		}
		built, err := prompt.Member(def, node, run)
		if err != nil {
			return nil, err
		}
		p := in.Profiles[node.Profile]
		members = append(members, Member{
			Name:          herdr,
			ShortName:     node.Name,
			Role:          node.Role,
			Kind:          p.Kind,
			Args:          p.Args,
			Env:           p.Env,
			Cwd:           node.Cwd,
			Prompt:        built.Text,
			Profile:       node.Profile,
			MaySpawn:      maySpawnOf(def, node.Name),
			RoleSource:    built.Role,
			HasRoleSource: built.HasRole,
			DocsRead:      built.Read,
		})
	}
	return &Config{
		Team:           in.RunName,
		Definition:     in.TeamName,
		Version:        def.Version,
		Mission:        def.Mission,
		Members:        members,
		Profiles:       in.Profiles,
		TeamYAML:       source.Doc{Layer: in.Layer, Path: in.TeamYAMLPath},
		ProfilesYAML:   in.ProfilesYAML,
		Layer:          in.Layer,
		Docs:           def.Docs,
		GlobalTeamYAML: in.GlobalTeamYAML,
		LocalTeamYAML:  in.LocalTeamYAML,
	}, nil
}

// maySpawnOf is one member's worker pairs, from the graph's worker nodes, in the
// written order; nil when the member hires nobody.
func maySpawnOf(def *definition.Definition, member string) *MaySpawn {
	var workers []MaySpawnWorker
	for _, worker := range def.Graph.Workers {
		if worker.From == member {
			workers = append(workers, MaySpawnWorker{Purpose: worker.Purpose, Profile: worker.Profile})
		}
	}
	if len(workers) == 0 {
		return nil
	}
	return &MaySpawn{Workers: workers}
}
