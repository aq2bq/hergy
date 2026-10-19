package definition

import "github.com/aq2bq/hergy/internal/source"

// Start is one member as a team runs it: the frame team.yaml writes, the engine
// its profile names, and the prompt hergy hands it. The launcher works on these;
// the definition is what they were built from (I1, I2). Building a Start is a
// projection (prompt.Member), done where the run's facts are known.
type Start struct {
	// Name is the name herdr sees: "<team>-<member>".
	Name string
	// Short is the short name team.yaml writes.
	Short string
	Role  string
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
}

// ToPlan is the member as the --dry output shows it: the expanded prompt, cwd,
// args, what it may hire and the documents it was built from. Empty lists are []
// rather than null.
func (s Start) ToPlan() map[string]any {
	out := map[string]any{
		"name":   s.Name,
		"role":   s.Role,
		"kind":   s.Kind,
		"args":   orEmpty(s.Args),
		"cwd":    s.Cwd,
		"prompt": s.Prompt,
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
