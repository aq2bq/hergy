package team

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadConfigKnowsProjectsFromNoProjects: a config.yaml without projects: and
// one with an empty list mean different things (d-73fc).
func TestReadConfigKnowsProjectsFromNoProjects(t *testing.T) {
	cases := []struct {
		name string
		text string
		has  bool
		n    int
	}{
		{"no key", "version: 1\nlang: ja\n", false, 0},
		{"empty list", "version: 1\nprojects: []\n", true, 0},
		{"block", "version: 1\nprojects:\n  - /a\n  - /b\n", true, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ReadConfig(tc.text, "config.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.HasProjects != tc.has || len(cfg.Projects) != tc.n {
				t.Errorf("ReadConfig = %+v, want HasProjects %v with %d projects", cfg, tc.has, tc.n)
			}
		})
	}
}

// TestProjectPathsExpandsHomeAndRefusesRelative: ~ is the home directory, and a
// relative entry is a broken setting that names itself (api.md 10 節).
func TestProjectPathsExpandsHomeAndRefusesRelative(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := Env{}
	paths, present, err := env.ProjectPaths(Config{HasProjects: true, Projects: []string{"~/src/shop", "~/src/shop"}})
	if err != nil {
		t.Fatal(err)
	}
	if !present || len(paths) != 1 || paths[0] != filepath.Join(home, "src", "shop") {
		t.Errorf("ProjectPaths = %v, %v, want the one expanded path", paths, present)
	}
	if _, _, err := env.ProjectPaths(Config{HasProjects: true, Projects: []string{"src/shop"}}); err == nil {
		t.Error("a relative project path did not stop")
	} else if !strings.Contains(err.Error(), "src/shop") {
		t.Errorf("the refusal does not name the entry: %v", err)
	}
}

// TestAddProjectWritesHomeAsTilde: a project under the home directory is written
// as ~/…, so the person reading config.yaml sees it (lead's addition 3), and
// every other line stays as it was.
func TestAddProjectWritesHomeAsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := Env{GlobalDir: filepath.Join(t.TempDir(), "hergy")}
	write(t, env.ConfigPath(), "version: 1\nlang: ja\n# the machine's settings\n")
	dir := filepath.Join(home, "src", "shop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	written, err := env.AddProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if written != "~/src/shop" {
		t.Errorf("the written entry = %q, want ~/src/shop", written)
	}
	text, err := os.ReadFile(env.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "projects:\n  - ~/src/shop\n") {
		t.Fatalf("projects: was not written as a block:\n%s", text)
	}
	if !strings.Contains(string(text), "# the machine's settings") {
		t.Errorf("the comment was lost:\n%s", text)
	}
	if again, err := env.AddProject(dir); err != nil || again != written {
		t.Errorf("adding the same project again = %q, %v", again, err)
	}
	cfg, err := env.ReadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	paths, _, err := env.ProjectPaths(cfg)
	resolved, _ := filepath.EvalSymlinks(dir)
	if err != nil || len(paths) != 1 || paths[0] != resolved {
		t.Errorf("the written file reads as %v, %v", paths, err)
	}
}

// TestAddProjectToAConfigWithNoProjects: the entry gets its key at the end, and
// the language line above it is untouched.
func TestAddProjectToAConfigWithNoProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := Env{GlobalDir: filepath.Join(t.TempDir(), "hergy")}
	write(t, env.ConfigPath(), "version: 1\nlang: ja\n")
	dir := filepath.Join(home, "work", "atlas")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := env.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(env.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "version: 1\nlang: ja\nprojects:\n  - ~/work/atlas\n") {
		t.Fatalf("the file is not the expected lines:\n%s", text)
	}
}

// TestRemoveProjectLeavesAnEmptyList: the last entry leaves projects: [] and
// every other line alone, and a path that is not listed stops.
func TestRemoveProjectLeavesAnEmptyList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := Env{GlobalDir: filepath.Join(t.TempDir(), "hergy")}
	write(t, env.ConfigPath(), "version: 1\n# before\nprojects:\n  - ~/src/shop\n  - ~/src/ledger\n# after\nlang: ja\n")
	if err := env.RemoveProject(filepath.Join(home, "src", "shop")); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(env.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "projects:\n  - ~/src/ledger\n") || !strings.Contains(string(text), "# after") {
		t.Fatalf("remove changed more than the one item:\n%s", text)
	}
	if err := env.RemoveProject(filepath.Join(home, "src", "ledger")); err != nil {
		t.Fatal(err)
	}
	text, _ = os.ReadFile(env.ConfigPath())
	if !strings.Contains(string(text), "projects: []\n") {
		t.Fatalf("the last item did not leave an empty list:\n%s", text)
	}
	if err := env.RemoveProject(filepath.Join(home, "src", "ledger")); err == nil {
		t.Error("removing a path that is not listed did not stop")
	}
}

// TestAtProjectReadsAnotherLayer: the global directory and language stay, and
// the project root and .hergy/ become the other project's.
func TestAtProjectReadsAnotherLayer(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	env := Env{ProjectRoot: root, GlobalDir: filepath.Join(root, "cfg"), LocalDir: filepath.Join(root, ".hergy")}
	got, err := env.AtProject(other)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectRoot != other || got.LocalDir != filepath.Join(other, ".hergy") || got.GlobalDir != env.GlobalDir {
		t.Errorf("AtProject = %+v", got)
	}
}

// TestAddProjectKeepsAFlowList: a projects: written on one line as [a, b] keeps
// both entries when one more is added, and becomes a block list.
func TestAddProjectKeepsAFlowList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := Env{GlobalDir: filepath.Join(t.TempDir(), "hergy")}
	one, two := filepath.Join(home, "one"), filepath.Join(home, "two")
	for _, dir := range []string{one, two} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, env.ConfigPath(), "version: 1\nprojects: [~/one]\n")
	if _, err := env.AddProject(two); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(env.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadConfig(string(text), "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	paths, _, err := env.ProjectPaths(cfg)
	if err != nil || len(paths) != 2 {
		t.Fatalf("paths = %v, %v\n%s", paths, err, text)
	}
}
