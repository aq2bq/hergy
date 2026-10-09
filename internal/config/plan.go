package config

// ToPlan is the member as the --dry output shows it: the expanded prompt, cwd,
// args and the documents it was built from. Empty lists are [] rather than null.
func (m Member) ToPlan() map[string]any {
	out := map[string]any{
		"name":   m.Name,
		"role":   m.Role,
		"kind":   m.Kind,
		"args":   orEmpty(m.Args),
		"cwd":    m.Cwd,
		"prompt": m.Prompt,
	}
	if m.Profile != "" {
		out["profile"] = m.Profile
	}
	if len(m.Env) > 0 {
		out["env"] = m.Env
	}
	if m.MaySpawn != nil {
		out["may_spawn"] = map[string]any{
			"purposes": orEmpty(m.MaySpawn.Purposes),
			"profiles": orEmpty(m.MaySpawn.Profiles),
		}
	}
	if sources := m.sourceDisplays(); len(sources) > 0 {
		out["sources"] = sources
	}
	return out
}

// sourceDisplays is the role file, then the documents, in the order they appear.
func (m Member) sourceDisplays() []string {
	var out []string
	if m.HasRoleSource {
		out = append(out, m.RoleSource.Display())
	}
	for _, doc := range m.DocsRead {
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
