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

// starterAll is starterIn, with the person having asked for the starter roles
// and the starter profile: the write a person chose in full.
func starterAll(t *testing.T) (In, string) {
	t.Helper()
	in, proj := starterIn(t)
	in.StarterRoles, in.StarterProfile = true, true
	return in, proj
}

// TestPlanSaysWhatAWriteWouldMake: the plan names the definition's own files, the
// layer's role documents it would write, the ones it would leave, and the starter
// profile (spec-teams B2, C8, I7, K5).
func TestPlanSaysWhatAWriteWouldMake(t *testing.T) {
	in, proj := starterAll(t)
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
	if len(written.Missing) != 0 || written.ProfileMissing {
		t.Errorf("the plan misses %v / %v, want nothing when the person asked for the starter ones", written.Missing, written.ProfileMissing)
	}
	// The plan writes nothing (B2: the screen shows it before a person saves).
	if _, err := os.Stat(filepath.Join(base, "teams", "ops", "team.yaml")); !os.IsNotExist(err) {
		t.Error("the plan wrote the definition")
	}
}

// TestPlanWritesNothingThePersonDidNotChoose: a starter row a person did not
// choose roles or a profile for writes its own two files alone, and says which
// starter roles and profile a place does not hold (r-54b8, d-86b1).
func TestPlanWritesNothingThePersonDidNotChoose(t *testing.T) {
	in, proj := starterIn(t)
	written, err := Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Files) != 2 {
		t.Errorf("the plan names %v, want the definition's own two files", written.Files)
	}
	if len(written.Roles) != 0 || len(written.Kept) != 0 {
		t.Errorf("the plan writes roles %v and leaves %v, want neither", written.Roles, written.Kept)
	}
	if want := []string{"lead", "builder"}; !slices.Equal(written.Missing, want) {
		t.Errorf("the plan misses %v, want %v", written.Missing, want)
	}
	if written.Profile != "" || !written.ProfileMissing {
		t.Errorf("the plan writes the profile to %q (missing %v), want it left for the person", written.Profile, written.ProfileMissing)
	}
	if _, err := os.Stat(filepath.Join(proj, ".hergy", "roles")); !os.IsNotExist(err) {
		t.Errorf("the plan's writing left a roles/ directory: %v", err)
	}
}

// TestPlanLooksForARoleInBothPlaces: a role document the global place holds is
// the one a definition reads, so a starter team added to the project writes no
// copy of it (r-54b8, K1).
func TestPlanLooksForARoleInBothPlaces(t *testing.T) {
	in, proj := starterIn(t)
	global := in.Catalog.LayerDir(source.LayerGlobal)
	if err := os.MkdirAll(filepath.Join(global, "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "roles", "lead.yaml"), []byte("version: 1\nsections: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(global, "roles", "lead.yaml"); len(written.Kept) != 1 || written.Kept[0] != want {
		t.Errorf("the plan leaves %v, want the global lead.yaml alone", written.Kept)
	}
	if want := []string{"builder"}; !slices.Equal(written.Missing, want) {
		t.Errorf("the plan misses %v, want %v", written.Missing, want)
	}
	if slices.Contains(written.Roles, filepath.Join(proj, ".hergy", "roles", "lead.yaml")) {
		t.Errorf("the plan writes the project's lead.yaml over the global one: %v", written.Roles)
	}
}

// TestWriteMakesTheStarterFolder: a starter write makes the definition's files,
// the layer's role documents, and the starter profile, and a role document the
// layer already holds is left as it is (spec-teams C8, I7, K5).
func TestWriteMakesTheStarterFolder(t *testing.T) {
	in, proj := starterAll(t)
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
	in, proj := starterAll(t)
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

// TestWriteAddsTheStarterProfileLineByLine: a layer that already holds a
// profiles.yaml gets the starter profile with the one edit the profiles screen
// makes, so every other line — the order of the profiles, a comment, the way a
// person wrote a value — comes back as it was (spec-boundary 5, 監査 R3).
func TestWriteAddsTheStarterProfileLineByLine(t *testing.T) {
	in, proj := starterAll(t)
	base := filepath.Join(proj, ".hergy")
	before := "version: 1\n" +
		"# a comment a person wrote\n" +
		"profiles:\n" +
		"  zeta:\n" +
		"    kind: codex\n" +
		"    args: ['-m', 'gpt-5']\n" +
		"    env:\n" +
		"      CODEX_HOME: '~/.codex-team'\n" +
		"  alpha:\n" +
		"    kind: claude\n"
	path := filepath.Join(base, "profiles.yaml")
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(in); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := before + "  " + scaffold.ProfileName + ":\n    kind: " + scaffold.ProfileKind + "\n"
	if string(after) != want {
		t.Errorf("the write left the profiles.yaml as\n%s\nwant only the starter profile added:\n%s", after, want)
	}
}

// TestWriteLeavesTheRolesAndProfileThePersonDidNotChoose: a starter write the
// person asked nothing more for writes the definition's own files alone, so the
// two places' roles/ and profiles.yaml stand as they were (r-54b8, ac-ff73).
func TestWriteLeavesTheRolesAndProfileThePersonDidNotChoose(t *testing.T) {
	in, proj := starterIn(t)
	base := filepath.Join(proj, ".hergy")
	// The global place holds a role of its own: it is not the project's to copy.
	global := in.Catalog.LayerDir(source.LayerGlobal)
	if err := os.MkdirAll(filepath.Join(global, "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "roles", "analyst.yaml"), []byte("version: 1\nsections: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := Write(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{filepath.Join(base, "teams", "ops", "team.yaml"), filepath.Join(base, "teams", "ops", "agreement.yaml")} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("the write did not make %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "roles")); !os.IsNotExist(err) {
		t.Errorf("the write made the project's roles/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "profiles.yaml")); !os.IsNotExist(err) {
		t.Errorf("the write made the project's profiles.yaml: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(global, "roles", "analyst.yaml")); err != nil || !strings.Contains(string(got), "sections: []") {
		t.Errorf("the write changed the global place's role document: %v\n%s", err, got)
	}
	if len(written.Roles) != 0 || len(written.Kept) != 0 || written.Profile != "" {
		t.Errorf("the write made %v, left %v, wrote the profile to %q, want none of them", written.Roles, written.Kept, written.Profile)
	}
}
