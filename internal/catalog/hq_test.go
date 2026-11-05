package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCountsAreOneLayersOwn: a project that shadows a global name counts it on
// each side, because the headquarters shows every project's own size (d-73fc).
func TestCountsAreOneLayersOwn(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	for _, one := range []struct{ dir, place string }{{global, "global"}, {local, "project"}} {
		writeFile(t, filepath.Join(one.dir, "teams", "core", "team.yaml"), "version: 1\n")
		writeFile(t, filepath.Join(one.dir, "roles", "lead.md"), "version: 1\n")
		writeFile(t, filepath.Join(one.dir, "profiles.yaml"), "version: 1\nprofiles:\n  fast:\n    kind: claude\n")
	}
	writeFile(t, filepath.Join(local, "roles", "builder.md"), "# builder\n")
	cat := New(global, local, local, source.Disk{}, func(p string) string { return p })
	defs, roles, profiles := cat.Counts(source.LayerLocal)
	if defs != 1 || roles != 2 || profiles != 1 {
		t.Errorf("Counts(project) = %d, %d, %d, want 1, 2, 1", defs, roles, profiles)
	}
	if !cat.HoldsDefinition("core", source.LayerLocal) || !cat.HoldsRole("builder", source.LayerLocal) || !cat.HoldsProfile("fast", source.LayerLocal) {
		t.Error("a name the project holds was not found there")
	}
	if cat.HoldsDefinition("core", source.LayerGlobal) == false {
		t.Error("the global layer lost its own core")
	}
}

// TestLayerOnlyReadsOneLayer: the headquarters shows the global rows once, not
// once more through every project's catalog (d-73fc).
func TestLayerOnlyReadsOneLayer(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	writeFile(t, filepath.Join(global, "teams", "docs", "team.yaml"), "version: 1\n")
	writeFile(t, filepath.Join(local, "teams", "core", "team.yaml"), "version: 1\n")
	cat := New(global, local, local, source.Disk{}, func(p string) string { return p })
	only := cat.LayerOnly(source.LayerGlobal)
	rows := only.Definitions()
	if len(rows) != 1 || rows[0].Name != "docs" || rows[0].Local {
		t.Errorf("LayerOnly(global).Definitions = %+v", rows)
	}
}

// TestListedRolesCountsTheRowsTheListDraws: the band's number is the list's
// rows (r-929e): a legacy .yaml row and a name only a member names stand on the
// project's side, and a name in both layers is one row on each side.
func TestListedRolesCountsTheRowsTheListDraws(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	writeFile(t, filepath.Join(local, "roles", "lead.md"), "# lead\n")
	writeFile(t, filepath.Join(local, "roles", "builder.md"), "# builder\n")
	writeFile(t, filepath.Join(local, "roles", "old.yaml"), "version: 1\n")
	writeFile(t, filepath.Join(global, "roles", "lead.md"), "# lead\n")
	writeFile(t, filepath.Join(global, "roles", "docs.md"), "# docs\n")
	writeFile(t, filepath.Join(local, "teams", "core", "team.yaml"), "version: 1\nmembers:\n  - name: a\n    role: ghost\n    profile: fast\n")
	cat := New(global, local, local, source.Disk{}, func(p string) string { return p })
	if got := len(cat.ShownRows(KindRole, source.LayerLocal)); got != 4 {
		t.Errorf("ListedRoles(project) = %d, want 4 (lead, builder, old, ghost)", got)
	}
	if got := len(cat.ShownRows(KindRole, source.LayerGlobal)); got != 2 {
		t.Errorf("ListedRoles(global) = %d, want 2 (lead, docs)", got)
	}
	if _, counts, _ := cat.Counts(source.LayerLocal); counts != 4 {
		t.Fatalf("Counts(project).roles = %d, want the row count 4", counts)
	}
}

// TestListedRolesLeavesOutUnreadableDocuments: the list drops a document it
// cannot read, so the count drops it too (r-929e).
func TestListedRolesLeavesOutUnreadableDocuments(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	writeFile(t, filepath.Join(local, "roles", "good.md"), "# good\n")
	bad := filepath.Join(local, "roles", "bad.md")
	writeFile(t, bad, "# bad\n")
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	cat := New(global, local, local, source.Disk{}, func(p string) string { return p })
	if got := len(cat.ShownRows(KindRole, source.LayerLocal)); got != 2 {
		t.Errorf("role rows (project) = %d, want 2 (good and bad)", got)
	}
	if _, counts, _ := cat.Counts(source.LayerLocal); counts != 2 {
		t.Fatalf("Counts(project).roles = %d, want the row count 2", counts)
	}
}

// TestListedRowsAreTheRowsTheListsDraw: one name in one place is one row, for
// each kind; the counts are those rows (r-ea17, d-4d88).
func TestListedRowsAreTheRowsTheListsDraw(t *testing.T) {
	global := t.TempDir()
	local := t.TempDir()
	// Teams: one in each layer, one of them without a readable team.yaml.
	writeFile(t, filepath.Join(local, "teams", "core", "team.yaml"), "version: 1\n")
	writeFile(t, filepath.Join(local, "teams", "broken", "team.yaml"), "not: [a team")
	writeFile(t, filepath.Join(global, "teams", "shared", "team.yaml"), "version: 1\n")
	// Roles: one in each layer, a legacy .yaml, a named-only name, and one
	// document that cannot be read.
	writeFile(t, filepath.Join(local, "roles", "lead.md"), "# lead\n")
	writeFile(t, filepath.Join(local, "roles", "old.yaml"), "version: 1\n")
	writeFile(t, filepath.Join(global, "roles", "shared.md"), "# shared\n")
	writeFile(t, filepath.Join(local, "roles", "bad.md"), "# bad\n")
	if err := os.Chmod(filepath.Join(local, "roles", "bad.md"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(local, "roles", "bad.md"), 0o644) })
	writeFile(t, filepath.Join(local, "teams", "ghost", "team.yaml"), "version: 1\nmembers:\n  - name: a\n    role: ghost\n    profile: fast\n")
	// Profiles: one in each layer.
	writeFile(t, filepath.Join(local, "profiles.yaml"), "version: 1\nprofiles:\n  fast:\n    kind: claude\n")
	writeFile(t, filepath.Join(global, "profiles.yaml"), "version: 1\nprofiles:\n  shared:\n    kind: claude\n")
	cat := New(global, local, local, source.Disk{}, func(p string) string { return p })

	if got := len(cat.ShownRows(KindTeam, source.LayerLocal)); got != 3 {
		t.Errorf("team rows (project) = %d, want 3 (core, broken, ghost)", got)
	}
	if got := len(cat.ShownRows(KindTeam, source.LayerGlobal)); got != 1 {
		t.Errorf("team rows (global) = %d, want 1 (shared)", got)
	}
	// lead, old (legacy), ghost (no document), and bad.md, which cannot be read:
	// every row stands and is counted (d-17ab, answer A to q-a8a3).
	if got := len(cat.ShownRows(KindRole, source.LayerLocal)); got != 4 {
		t.Errorf("role rows (project) = %d, want 4 (lead, old, ghost, bad)", got)
	}
	if got := len(cat.ShownRows(KindRole, source.LayerGlobal)); got != 1 {
		t.Errorf("role rows (global) = %d, want 1 (shared)", got)
	}
	if got := len(cat.ShownRows(KindProfile, source.LayerLocal)); got != 1 {
		t.Errorf("profile rows (project) = %d, want 1 (fast)", got)
	}
	if got := len(cat.ShownRows(KindProfile, source.LayerGlobal)); got != 1 {
		t.Errorf("profile rows (global) = %d, want 1 (shared)", got)
	}
	// The counts are those rows, and nothing else.
	defs, roles, profiles := cat.Counts(source.LayerLocal)
	if defs != 3 || roles != 4 || profiles != 1 {
		t.Errorf("Counts(project) = %d, %d, %d, want the rows 3, 4, 1", defs, roles, profiles)
	}
}
