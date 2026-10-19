package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// TestWithCallsAddUp checks two texts read at once (spec/38 N2): a screen that
// checks a definition's role change and the document the new role needs reads
// both, and the change is not lost.
func TestWithCallsAddUp(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	teamYAML := filepath.Join(l.local, "teams", "t", "team.yaml")
	roleDoc := filepath.Join(l.local, "roles", "newrole.yaml")
	after := strings.Replace(readFile(t, teamYAML), "role: impl-role", "role: newrole", 1)
	document := "newrole's document\n"

	cat := l.catalog().With(teamYAML, after).With(roleDoc, document)

	def, err := cat.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if def.Graph.Members[1].Role != "newrole" {
		t.Errorf("the member's role = %q, want the edit's", def.Graph.Members[1].Role)
	}
	found, ok := cat.Role("newrole")
	if !ok {
		t.Fatal("the document the second edit makes is not found")
	}
	if found.Path != roleDoc {
		t.Errorf("the document is %q, want %q", found.Path, roleDoc)
	}
	text, err := cat.Reader().Read(found)
	if err != nil {
		t.Fatal(err)
	}
	if text != document {
		t.Errorf("the document reads %q, want the edit's", text)
	}
	if refs := cat.RoleUsers("newrole"); len(refs) != 1 {
		t.Errorf("the role is played by %d members, want 1: %v", len(refs), refs)
	}
}

// TestWithLeavesTheCatalogItWasCalledOn: a text is added to the catalog it
// returns, so the one it was called on reads what it read before (spec/38 N2).
func TestWithLeavesTheCatalogItWasCalledOn(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	teamYAML := filepath.Join(l.local, "teams", "t", "team.yaml")
	roleDoc := filepath.Join(l.local, "roles", "newrole.yaml")
	after := strings.Replace(readFile(t, teamYAML), "role: impl-role", "role: newrole", 1)

	was := l.catalog()
	one := was.With(teamYAML, after)
	two := one.With(roleDoc, "newrole's document\n")

	if _, ok := one.Role("newrole"); ok {
		t.Error("the catalog before the second edit reads the second text")
	}
	if _, ok := two.Role("newrole"); !ok {
		t.Error("the catalog after the second edit does not read the second text")
	}
	if _, ok := was.Role("newrole"); ok {
		t.Error("the catalog the first edit was made from reads the first text")
	}
	def, err := was.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if def.Graph.Members[1].Role != "impl-role" {
		t.Errorf("the untouched catalog reads %q from disk, want the file's", def.Graph.Members[1].Role)
	}
}

// TestWithOnTheSamePathTakesTheLaterText: one path carried twice reads the later
// text (spec/38 N2).
func TestWithOnTheSamePathTakesTheLaterText(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	teamYAML := filepath.Join(l.local, "teams", "t", "team.yaml")
	onDisk := readFile(t, teamYAML)

	cat := l.catalog().With(teamYAML, strings.Replace(onDisk, "mission: m", "mission: first", 1)).
		With(teamYAML, strings.Replace(onDisk, "mission: m", "mission: second", 1))
	def, err := cat.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if def.Mission != "second" {
		t.Errorf("mission = %q, want the later text", def.Mission)
	}
}

// TestRolesIsOneRowPerRole: the roles list is one row per name, with the
// document the one way to a name picks (3.5, spec/38 T2).
func TestRolesIsOneRowPerRole(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")

	roles := l.catalog().Roles()
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2: %+v", len(roles), roles)
	}
	if roles[0].Name != "impl-role" || roles[1].Name != "lead-role" {
		t.Fatalf("roles are not by name: %+v", roles)
	}
	for _, role := range roles {
		want := filepath.Join(l.local, "roles", role.Name+".yaml")
		if role.Path != want || !role.HasDocument {
			t.Errorf("%s = %+v, want the document at %q", role.Name, role, want)
		}
		if role.Outside || role.UnusedPath != "" {
			t.Errorf("%s = %+v, want the project's document and nothing unused", role.Name, role)
		}
	}
}

// TestRolesKeepsARoleWithoutADocument: a role a definition names stays a row
// when no layer holds a document for it, so a screen can offer to make one
// (3.5, spec/38 T2).
func TestRolesKeepsARoleWithoutADocument(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	teamYAML := filepath.Join(l.local, "teams", "t", "team.yaml")
	after := strings.Replace(readFile(t, teamYAML), "role: impl-role", "role: absent-role", 1)
	write(t, teamYAML, after)

	roles := l.catalog().Roles()
	if len(roles) != 3 {
		t.Fatalf("roles = %d, want 3: %+v", len(roles), roles)
	}
	byName := map[string]RoleEntry{}
	for _, role := range roles {
		byName[role.Name] = role
	}
	absent, ok := byName["absent-role"]
	if !ok {
		t.Fatalf("the role a definition names is not a row: %+v", roles)
	}
	if absent.HasDocument || absent.Path != "" || absent.Outside || absent.UnusedPath != "" {
		t.Errorf("absent-role = %+v, want a row with no document", absent)
	}
	// The document the renamed member left is still the layer's, so it is a row
	// of its own even though no definition names it now.
	if left := byName["impl-role"]; !left.HasDocument || left.Path == "" {
		t.Errorf("impl-role = %+v, want the document the layer still holds", left)
	}
	// The documents come first, the row no document holds last (spec-roles 1.2「並び」).
	if roles[0].Name != "impl-role" || roles[1].Name != "lead-role" || roles[2].Name != "absent-role" {
		t.Errorf("order = %s, %s, %s, want impl-role, lead-role, absent-role", roles[0].Name, roles[1].Name, roles[2].Name)
	}
}

// TestRolesTakesARoleLeftAsMarkdown: a role document a layer still holds as
// Markdown is a row too, with no .yaml, so a screen says the .yaml is not there
// yet (spec-roles 1.2.10, J2).
func TestRolesTakesARoleLeftAsMarkdown(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.local, "roles", "noted.md"), "# noted\n")

	roles := l.catalog().Roles()
	if len(roles) != 1 || roles[0].Name != "noted" {
		t.Fatalf("roles = %+v, want the one row noted", roles)
	}
	if roles[0].HasDocument || roles[0].Path != "" {
		t.Errorf("noted = %+v, want a row with no .yaml document", roles[0])
	}
}

// TestRolesTakesTheDocumentsTheLayersHold: a document a layer holds is a row
// even when no definition names it, and the project's document wins where both
// layers hold one (3.5, K1, spec/38 T2).
func TestRolesTakesTheDocumentsTheLayersHold(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "roles", "orphan.yaml"), "a document nobody names\n")
	write(t, filepath.Join(l.global, "roles", "watched.yaml"), "the shared one\n")
	write(t, filepath.Join(l.local, "roles", "watched.yaml"), "the project's one\n")
	write(t, filepath.Join(l.global, "profiles.yaml"), "version: 1\nprofiles:\n  p:\n    kind: claude\n")
	write(t, filepath.Join(l.local, "teams", "t", "team.yaml"), `version: 1
mission: m
turns:
  - human -> a
members:
  - name: a
    role: watched
    profile: p
`)

	roles := l.catalog().Roles()
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2: %+v", len(roles), roles)
	}
	// The project's document comes first, then the shared one (spec-roles 1.2「並び」).
	if roles[0].Name != "watched" || roles[1].Name != "orphan" {
		t.Fatalf("order = %s, %s, want watched, orphan", roles[0].Name, roles[1].Name)
	}
	orphan := roles[1]
	if !orphan.Outside || !orphan.HasDocument {
		t.Errorf("orphan = %+v, want the outside document", orphan)
	}
	if want := filepath.Join(l.global, "roles", "orphan.yaml"); orphan.Path != want {
		t.Errorf("orphan path = %q, want %q", orphan.Path, want)
	}
	watched := roles[0]
	if want := filepath.Join(l.local, "roles", "watched.yaml"); watched.Path != want {
		t.Errorf("watched path = %q, want %q", watched.Path, want)
	}
	if want := filepath.Join(l.global, "roles", "watched.yaml"); watched.UnusedPath != want {
		t.Errorf("watched UnusedPath = %q, want %q", watched.UnusedPath, want)
	}
	if watched.Outside {
		t.Error("watched is marked as outside the project")
	}
}

// TestRolesReadsTheTextAScreenIsEditing: a document a screen is about to make is
// the row's document (C3b, 3.5).
func TestRolesReadsTheTextAScreenIsEditing(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	path := filepath.Join(l.local, "roles", "impl-role.yaml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	before := l.catalog()
	for _, role := range before.Roles() {
		if role.Name == "impl-role" && role.HasDocument {
			t.Fatalf("impl-role has a document before the edit: %+v", role)
		}
	}
	after := before.With(path, "the document the screen wrote\n")
	roles := after.Roles()
	if len(roles) != 2 {
		t.Fatalf("roles = %d, want 2: %+v", len(roles), roles)
	}
	if !roles[0].HasDocument || roles[0].Path != path {
		t.Errorf("impl-role = %+v, want the edit's document at %q", roles[0], path)
	}
}

// TestProfileEntriesIsOneRowPerName: the profiles list is one row per name, with
// the profiles.yaml the one way to a name picks (3.7, K1, spec/38 T2).
func TestProfileEntriesIsOneRowPerName(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerGlobal, "other", "another team")
	l.writeFullTeam(t, source.LayerLocal, "t", "m")

	entries := l.catalog().ProfileEntries()
	if len(entries) != 3 {
		t.Fatalf("profiles = %d, want 3: %+v", len(entries), entries)
	}
	if entries[0].Name != "builder" || entries[1].Name != "claude-default" || entries[2].Name != "researcher" {
		t.Fatalf("profiles are not by name: %+v", entries)
	}
	localProfiles := filepath.Join(l.local, "profiles.yaml")
	unusedProfiles := filepath.Join(l.global, "profiles.yaml")
	for _, entry := range entries {
		if entry.Path != localProfiles || entry.Outside {
			t.Errorf("%s = %+v, want the project's profiles.yaml", entry.Name, entry)
		}
		if entry.UnusedPath != unusedProfiles {
			t.Errorf("%s UnusedPath = %q, want %q", entry.Name, entry.UnusedPath, unusedProfiles)
		}
	}
}

// TestProfileEntriesTakesWhatOneLayerHolds: a profile only the outside
// profiles.yaml defines is a row there, with nothing unused (3.7, spec/38 T2).
func TestProfileEntriesTakesWhatOneLayerHolds(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "profiles.yaml"), "version: 1\nprofiles:\n  outside:\n    kind: claude\n")
	write(t, filepath.Join(l.local, "profiles.yaml"), "version: 1\nprofiles:\n  inside:\n    kind: codex\n")

	entries := l.catalog().ProfileEntries()
	if len(entries) != 2 {
		t.Fatalf("profiles = %d, want 2: %+v", len(entries), entries)
	}
	if got := entries[0]; got.Name != "inside" || got.Outside || got.Path != filepath.Join(l.local, "profiles.yaml") || got.UnusedPath != "" {
		t.Errorf("inside = %+v", got)
	}
	if got := entries[1]; got.Name != "outside" || !got.Outside || got.Path != filepath.Join(l.global, "profiles.yaml") || got.UnusedPath != "" {
		t.Errorf("outside = %+v", got)
	}
}

// TestProfileEntriesCarriesTheKind: a row holds the kind the name resolves to,
// read from the same file the name was read from (spec/41 N2, spec/40 T4).
func TestProfileEntriesCarriesTheKind(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "profiles.yaml"), "version: 1\nprofiles:\n  shared:\n    kind: codex\n  outside:\n    kind: claude\n")
	write(t, filepath.Join(l.local, "profiles.yaml"), "version: 1\nprofiles:\n  shared:\n    kind: claude\n")

	entries := l.catalog().ProfileEntries()
	if len(entries) != 2 {
		t.Fatalf("profiles = %d, want 2: %+v", len(entries), entries)
	}
	if got := entries[0]; got.Name != "outside" || got.Kind != "claude" {
		t.Errorf("outside = %+v, want kind claude", got)
	}
	// The project's file is the one the name resolves to, so its kind is the
	// row's, and the other file is the unused one.
	if got := entries[1]; got.Name != "shared" || got.Kind != "claude" || got.UnusedPath == "" {
		t.Errorf("shared = %+v, want the project's kind and the other file unused", got)
	}
}

// TestProfileEntriesReadsTheTextAScreenIsEditing: a layer whose profiles.yaml is
// not there yet contributes the rows of the text the screen is writing (C3b).
func TestProfileEntriesReadsTheTextAScreenIsEditing(t *testing.T) {
	l := newLayers(t)
	path := filepath.Join(l.local, "profiles.yaml")
	text := "version: 1\nprofiles:\n  made:\n    kind: claude\n"

	entries := l.catalog().With(path, text).ProfileEntries()
	if len(entries) != 1 {
		t.Fatalf("profiles = %d, want 1: %+v", len(entries), entries)
	}
	if got := entries[0]; got.Name != "made" || got.Path != path || got.Outside {
		t.Errorf("the edit's profile = %+v, want %q in the project's layer", got, path)
	}
}

// TestProfileEntriesSkipsALayerThatDoesNotRead: a profiles.yaml that does not
// read has no names to show (3.7, spec/38 T2).
func TestProfileEntriesSkipsALayerThatDoesNotRead(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "profiles.yaml"), "version: 1\nprofiles:\n  good:\n    kind: claude\n")
	write(t, filepath.Join(l.local, "profiles.yaml"), "version: 1\nprofiles:\n  bad:\n    kind: NotAKind\n")

	entries := l.catalog().ProfileEntries()
	if len(entries) != 1 || entries[0].Name != "good" {
		t.Fatalf("entries = %+v, want the one the layer that reads holds", entries)
	}
}
