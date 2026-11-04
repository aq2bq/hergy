package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateIsExplicitAndSelectsOneLayer(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	const legacy = "version: 1\ntitle: Legacy\nsections:\n  - rules:\n      - id: ABCD\n        text: Kept text.\n      - text: Disabled text.\n        in_prompt: false\n        note: Private note.\n"
	local := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	global := filepath.Join(f.cfg, "hergy", "roles", "shared.yaml")
	md := strings.TrimSuffix(local, ".yaml") + ".md"
	if err := os.Remove(md); err != nil {
		t.Fatal(err)
	}
	writeFile(t, local, legacy)
	writeFile(t, global, legacy)
	if _, _, code := run("up", "demo", "--dry"); code == 0 {
		t.Fatal("legacy definition booted")
	}
	if out, _, code := run("validate"); code == 0 || !strings.Contains(out, "lead.yaml") {
		t.Fatalf("validate did not name legacy: %s", out)
	}
	plan := mustRun(t, "migrate", "--dry")
	if !strings.Contains(plan, "lead.yaml") || strings.Contains(plan, "shared.yaml") {
		t.Fatalf("wrong layer: %s", plan)
	}
	if _, err := os.Stat(md); !os.IsNotExist(err) {
		t.Fatal("dry run wrote Markdown")
	}
	mustRun(t, "migrate")
	if got := readFile(t, local+".bak"); got != legacy {
		t.Fatal("backup changed bytes")
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatal("legacy file remains active")
	}
	got := readFile(t, md)
	if got != "# Legacy\n\n- Kept text.\n" {
		t.Fatalf("converted text: %q", got)
	}
	if readFile(t, global) != legacy {
		t.Fatal("project conversion changed global")
	}
	mustRun(t, "migrate", "--global")
	if readFile(t, global+".bak") != legacy {
		t.Fatal("global backup changed")
	}
	mustRun(t, "validate")
	mustRun(t, "up", "demo", "--dry")
	writeFile(t, md, "Edited after migration.\n")
	mustRun(t, "migrate")
	if readFile(t, md) != "Edited after migration.\n" {
		t.Fatal("rerun overwrote edited prose")
	}
}
