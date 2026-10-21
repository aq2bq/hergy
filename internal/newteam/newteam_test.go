package newteam

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// These are newteam's own promises (spec-teams C8): hergy new and the team
// screen write a definition's folder the same way, so what making a definition
// writes is one decision in one place.

// starterIn is one starter write into a layer that holds nothing yet.
func starterIn(t *testing.T) (In, string) {
	t.Helper()
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	global := filepath.Join(root, "global")
	for _, dir := range []string{filepath.Join(proj, ".hergy"), global} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cat := catalog.New(global, filepath.Join(proj, ".hergy"), proj, source.Disk{}, func(path string) string { return path })
	files, err := scaffold.Team("ja")
	if err != nil {
		t.Fatal(err)
	}
	return In{Catalog: cat, Layer: source.LayerLocal, Name: "ops", Lang: "ja", Files: files, Starter: true}, proj
}

// TestPlanSaysWhatAWriteWouldMake: the plan names the definition's own files, the
// layer's role documents it would write, the ones it would leave, and the starter
// profile (spec-teams B2, C8, I7, K5).
func TestPlanSaysWhatAWriteWouldMake(t *testing.T) {
	in, proj := starterIn(t)
	written, err := Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(proj, ".hergy")
	if len(written.Files) != 2 {
		t.Errorf("the plan names %v, want the definition's own two files", written.Files)
	}
	if len(written.Roles) != 2 || !slices.Contains(written.Roles, filepath.Join(base, "roles", "lead.yaml")) {
		t.Errorf("the plan writes roles %v, want the starter's two", written.Roles)
	}
	if len(written.Kept) != 0 {
		t.Errorf("the plan leaves %v, want none in a layer that holds no role documents", written.Kept)
	}
	if want := filepath.Join(base, "profiles.yaml"); written.Profile != want {
		t.Errorf("the plan adds the starter profile to %q, want %q", written.Profile, want)
	}
	// The plan writes nothing (B2: the screen shows it before a person saves).
	if _, err := os.Stat(filepath.Join(base, "teams", "ops", "team.yaml")); !os.IsNotExist(err) {
		t.Error("the plan wrote the definition")
	}
}

// TestWriteMakesTheStarterFolder: a starter write makes the definition's files,
// the layer's role documents, and the starter profile, and a role document the
// layer already holds is left as it is (spec-teams C8, I7, K5).
func TestWriteMakesTheStarterFolder(t *testing.T) {
	in, proj := starterIn(t)
	base := filepath.Join(proj, ".hergy")
	// The layer's own lead.yaml, with a rule of its own.
	if err := os.MkdirAll(filepath.Join(base, "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "version: 1\nsections:\n  - rules:\n      - id: MINE-1\n        text: これは私のリード。\n"
	if err := os.WriteFile(filepath.Join(base, "roles", "lead.yaml"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := Write(in)
	if err != nil {
		t.Fatal(err)
	}
	files, err := scaffold.Team("ja")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		got, err := os.ReadFile(filepath.Join(base, "teams", "ops", file.Path))
		if err != nil {
			t.Fatalf("the written %s does not read: %v", file.Path, err)
		}
		if string(got) != string(file.Data) {
			t.Errorf("the written %s reads\n%s\nwant the scaffold's own\n%s", file.Path, got, file.Data)
		}
	}
	if got, err := os.ReadFile(filepath.Join(base, "roles", "lead.yaml")); err != nil || string(got) != mine {
		t.Errorf("the layer's own lead.yaml was written over: %v\n%s", err, got)
	}
	if len(written.Kept) != 1 || len(written.Roles) != 1 {
		t.Errorf("the write left %v and wrote %v, want one of each", written.Kept, written.Roles)
	}
	if got, err := os.ReadFile(filepath.Join(base, "roles", "builder.yaml")); err != nil || !strings.Contains(string(got), "version: 1") {
		t.Errorf("the starter's builder.yaml was not written: %v\n%s", err, got)
	}
	if got, err := os.ReadFile(filepath.Join(base, "profiles.yaml")); err != nil || !strings.Contains(string(got), scaffold.ProfileName) {
		t.Errorf("profiles.yaml does not hold the starter profile: %v\n%s", err, got)
	}
}

// TestWriteTakesBackWhatItCouldNotFinish: a write that cannot finish removes what
// it wrote, so a refused save leaves the layer as it was (spec-teams B6).
func TestWriteTakesBackWhatItCouldNotFinish(t *testing.T) {
	in, proj := starterIn(t)
	base := filepath.Join(proj, ".hergy")
	// A profiles.yaml that cannot be read stops the last step, after the
	// definition's files and the role documents are written.
	if err := os.WriteFile(filepath.Join(base, "profiles.yaml"), []byte("this: [is not: a profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(in); err == nil {
		t.Fatal("the write answered no error for a profiles.yaml it cannot read")
	}
	if _, err := os.Stat(filepath.Join(base, "teams", "ops", "team.yaml")); !os.IsNotExist(err) {
		t.Error("the failed write left the definition's team.yaml")
	}
	if _, err := os.Stat(filepath.Join(base, "teams", "ops")); !os.IsNotExist(err) {
		t.Error("the failed write left the definition's directory")
	}
	if _, err := os.Stat(filepath.Join(base, "roles", "lead.yaml")); !os.IsNotExist(err) {
		t.Error("the failed write left the role document it wrote")
	}
	if got, err := os.ReadFile(filepath.Join(base, "profiles.yaml")); err != nil || string(got) != "this: [is not: a profile\n" {
		t.Errorf("the failed write changed the profiles.yaml it could not read: %v\n%s", err, got)
	}
}
