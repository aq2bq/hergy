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
		writeFile(t, filepath.Join(one.dir, "roles", "lead.yaml"), "version: 1\n")
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
