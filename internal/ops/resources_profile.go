package ops

import (
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
)

func editResourceProfile(text, display string, entries []profile.Entry, draft profile.Draft, original string) (string, error) {
	if strings.TrimSpace(text) == "" {
		text = "version: 1\n"
	}
	ops, err := resourceProfileOps(entries, draft, original)
	if err != nil {
		return "", err
	}
	out := text
	for _, op := range ops {
		edited, err := profile.EditYAML([]byte(out), display, op)
		if err != nil {
			return "", err
		}
		out = string(edited)
	}
	return out, nil
}

// resourceProfileOps is the ops a panel's save means.
func resourceProfileOps(entries []profile.Entry, draft profile.Draft, original string) ([]profile.Op, error) {
	saved, found := resourceProfileEntry(entries, original)
	switch {
	case original == "":
		ops := []profile.Op{profile.Add(draft.Name, draft.Kind)}
		if len(draft.Args) > 0 {
			ops = append(ops, profile.SetArgs(draft.Name, draft.Args))
		}
		if len(draft.Env) > 0 {
			ops = append(ops, profile.SetEnvInOrder(draft.Name, draft.Env))
		}
		return ops, nil
	case !found:
		return nil, i18n.Errorf("error.edit_no_profile", original)
	}
	var ops []profile.Op
	name := original
	if draft.Name != original {
		ops = append(ops, profile.Rename(original, draft.Name))
		name = draft.Name
	}
	if draft.Kind != saved.Kind {
		ops = append(ops, profile.SetKind(name, draft.Kind))
	}
	if !slices.Equal(draft.Args, saved.Args) {
		ops = append(ops, profile.SetArgs(name, draft.Args))
	}
	if !sameResourceEnv(draft.Env, saved.Env) {
		ops = append(ops, profile.SetEnvInOrder(name, draft.Env))
	}
	return ops, nil
}

// resourceProfileEntry is one name's entry in a file.
func resourceProfileEntry(entries []profile.Entry, name string) (profile.Entry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return profile.Entry{}, false
}

// sameResourceEnv reports whether the pairs and the file's env are the same values,
// whatever order the screen shows them in.
func sameResourceEnv(pairs []profile.EnvPair, saved []profile.EnvPair) bool {
	if len(pairs) != len(saved) {
		return false
	}
	want := make(map[string]string, len(saved))
	for _, pair := range saved {
		want[pair.Key] = pair.Value
	}
	for _, pair := range pairs {
		if value, ok := want[pair.Key]; !ok || value != pair.Value {
			return false
		}
	}
	return true
}
