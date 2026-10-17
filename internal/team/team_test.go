package team

import (
	"os"
	"os/exec" // the test makes a real git repository, to see K2's answer
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newEnv is an Env whose global and local layers are fresh temp directories.
func newEnv(t *testing.T) Env {
	t.Helper()
	root := t.TempDir()
	return Env{
		ProjectRoot: root,
		GlobalDir:   filepath.Join(root, "global"),
		LocalDir:    filepath.Join(root, "local"),
	}
}

func TestLangReadsTheGlobalConfig(t *testing.T) {
	env := newEnv(t)
	t.Setenv("XDG_CONFIG_HOME", env.GlobalDir)

	if got, err := Lang(); err != nil || got != "" {
		t.Fatalf("Lang() = %q, %v, want no language and no error", got, err)
	}

	write(t, filepath.Join(env.GlobalDir, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	got, err := Lang()
	if err != nil {
		t.Fatal(err)
	}
	if got != "ja" {
		t.Errorf("Lang() = %q, want %q", got, "ja")
	}
}

// TestInRepoFollowsTheGitRoot: hergy new writes to the project's layer inside a
// git repository and to the global one outside (K2), so InRepo must follow what
// git reports.
func TestInRepoFollowsTheGitRoot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if env, err := NewEnv(); err != nil {
		t.Fatal(err)
	} else if env.InRepo() {
		t.Error("InRepo() = true in a directory that is not a git repository")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if env, err := NewEnv(); err != nil {
		t.Fatal(err)
	} else if !env.InRepo() {
		t.Error("InRepo() = false inside a git repository")
	}
}

// TestNewPlaceFollowsK2: where the catalog puts something new is the layer K2
// reads, and the same place the environment names (C13). The environment is
// what tells the catalog whether the project is inside a repository, so the two
// answers agree.
func TestNewPlaceFollowsK2(t *testing.T) {
	env := newEnv(t)
	in := env
	in.inRepo = true
	out := env
	out.inRepo = false

	if got, want := in.Catalog(source.Disk{}).NewPlace(catalog.NewTeam, "t"), in.TeamDir("t", source.LayerLocal); got != want {
		t.Errorf("inside a repository NewPlace = %q, want %q", got, want)
	}
	if got, want := out.Catalog(source.Disk{}).NewPlace(catalog.NewTeam, "t"), out.TeamDir("t", source.LayerGlobal); got != want {
		t.Errorf("outside a repository NewPlace = %q, want %q", got, want)
	}
	cat := in.Catalog(source.Disk{})
	if got, want := cat.NewPlace(catalog.NewRole, "lead"), filepath.Join(env.LocalDir, "roles", "lead.yaml"); got != want {
		t.Errorf("NewPlace(NewRole, lead) = %q, want %q", got, want)
	}
	if got, want := cat.NewPlace(catalog.NewProfile, "lead"), filepath.Join(env.LocalDir, "profiles.yaml"); got != want {
		t.Errorf("NewPlace(NewProfile, lead) = %q, want %q", got, want)
	}
}
