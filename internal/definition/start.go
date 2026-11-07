package definition

import (
	"maps"

	"github.com/aq2bq/hergy/internal/source"
)

// Start is one member as a team runs it: the frame team.yaml writes, the engine
// its profile names, and the prompt hergy hands it. The launcher works on these;
// the definition is what they were built from (I1, I2). Building a Start is a
// projection (prompt.Member), done where the run's facts are known.
type Start struct {
	// Name is the name herdr sees: "<team>-<member>".
	Name string
	// Short is the short name team.yaml writes.
	Short string
	// From is the short name of the member that hires this worker, for a worker
	// a member hired (d-b4b9). Empty for a member of the definition.
	From string
	// Purpose is what a hired worker is for, as the member's workers: names it
	// (d-b4b9). Empty for a member of the definition.
	Purpose string
	Role    string
	// Profile is the profile the member names; Kind and Args are that profile's
	// (I7), and Env is the profile's own env with hergy's GY_ACTOR set to Name
	// (r-1c42), which is the environment the member's pane is made with.
	Profile string
	Kind    string
	Args    []string
	Env     map[string]string
	Cwd     string
	Prompt  string
	// RoleDoc is the role document the prompt read, with HasRole saying whether
	// there was one.
	RoleDoc source.Doc
	HasRole bool
	// Read is the documents the prompt read, in order.
	Read []source.Doc
	// Turns is who this member hands the turn to, in the order turns: writes
	// them: "human" or another member's short name (段 5).
	Turns []string
	// Workers is what this member may hire: the graph's edges out of it.
	Workers []WorkerNode
	// Segments is the prompt cut into the pieces it was built from, in the
	// order they stand in Prompt: joining their texts gives Prompt exactly
	// (r-4cf9). A prompt built before this existed has none.
	Segments []PromptSegment
	// Signal is what hergy adds to the start so the harness says when a turn
	// ends (r-378e): kept apart from Args and Env, which stay the profile's,
	// so a fingerprint, a plan and apply read the profile's alone.
	Signal Signal
}

// Signal is the harness's turn-end signal wired into one start: the arguments
// and environment added to the profile's own, or why none were.
type Signal struct {
	Wired bool
	Args  []string
	Env   map[string]string
	// Skipped is why nothing was wired (a harness.Skipped constant); empty
	// when Wired.
	Skipped string
}

// StartArgs is what the harness is started with: the profile's arguments and
// the signal's.
func (s *Start) StartArgs() []string {
	if len(s.Signal.Args) == 0 {
		return s.Args
	}
	return append(append([]string(nil), s.Args...), s.Signal.Args...)
}

// PaneEnv is the environment the pane is made with: the profile's (with
// GY_ACTOR) and the signal's.
func (s *Start) PaneEnv() map[string]string {
	if len(s.Signal.Env) == 0 {
		return s.Env
	}
	out := make(map[string]string, len(s.Env)+len(s.Signal.Env))
	maps.Copy(out, s.Env)
	maps.Copy(out, s.Signal.Env)
	return out
}

// PromptOrigin is where one prompt segment came from: hergy's own words, the
// team definition (its prompt or its roster/turns/workers), one member's own
// prompt, a role document, the team's rules, or the hire that names a worker's
// task.
type PromptOrigin struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Path  string `json:"path,omitempty"`
	Place string `json:"place,omitempty"`
}

// PromptSegment is one contiguous piece of a prompt and where it came from. The
// texts joined in order are the prompt, character for character.
type PromptSegment struct {
	// Kind is what the piece is: template, actor, role_doc, team_rules,
	// roster, turns, workers, herdr or task.
	Kind   string       `json:"kind"`
	Text   string       `json:"text"`
	Origin PromptOrigin `json:"origin"`
}

// ToPlan is the member as the --dry output shows it: the expanded prompt, cwd,
// args, what it may hire and the documents it was built from. Empty lists are []
// rather than null.
func (s Start) ToPlan() map[string]any {
	out := map[string]any{
		"name":     s.Name,
		"role":     s.Role,
		"kind":     s.Kind,
		"args":     orEmpty(s.Args),
		"cwd":      s.Cwd,
		"prompt":   s.Prompt,
		"segments": orSegments(s.Segments),
	}
	if s.Profile != "" {
		out["profile"] = s.Profile
	}
	if len(s.Env) > 0 {
		out["env"] = s.Env
	}
	if len(s.Turns) > 0 {
		out["turns"] = s.Turns
	}
	if len(s.Workers) > 0 {
		workers := make([]map[string]any, len(s.Workers))
		for i, worker := range s.Workers {
			workers[i] = map[string]any{"from": worker.From, "purpose": worker.Purpose, "profile": worker.Profile}
		}
		out["workers"] = workers
	}
	if sources := s.sourceDisplays(); len(sources) > 0 {
		out["sources"] = sources
	}
	return out
}

// sourceDisplays is the role file, then the documents, in the order they appear.
func (s Start) sourceDisplays() []string {
	var out []string
	if s.HasRole {
		out = append(out, s.RoleDoc.Display())
	}
	for _, doc := range s.Read {
		out = append(out, doc.Display())
	}
	return out
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func orSegments(values []PromptSegment) []PromptSegment {
	if values == nil {
		return []PromptSegment{}
	}
	return values
}
