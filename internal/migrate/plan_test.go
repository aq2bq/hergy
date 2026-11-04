package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanPreflightsWholeLayerAndIgnoresCompletedBackups(t *testing.T) {
	layer := t.TempDir()
	for _, dir := range []string{"roles", "teams/sample"} {
		if err := os.MkdirAll(filepath.Join(layer, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	role := filepath.Join(layer, "roles/lead.yaml")
	agreement := filepath.Join(layer, "teams/sample/agreement.yaml")
	put(t, role, legacy)
	put(t, agreement, "invalid")
	put(t, filepath.Join(layer, "teams/sample/team.yaml"), "not a rule document")
	if _, err := Plan(layer); err == nil {
		t.Fatal("accepted invalid agreement")
	}
	if _, err := os.Stat(role + ".bak"); !os.IsNotExist(err) {
		t.Fatal("preflight wrote a file")
	}
	put(t, agreement, legacy)
	files, err := Plan(layer)
	if err != nil || len(files) != 2 {
		t.Fatalf("plan: %v %v", files, err)
	}
	for _, file := range files {
		if err := file.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(layer, "roles/lead.md"), "# Edited after migration\n")
	files, err = Plan(layer)
	if err != nil || len(files) != 0 {
		t.Fatalf("completed: %v %v", files, err)
	}
}
