package team

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func writeTeam(t *testing.T, env Env, layer, name string) {
	t.Helper()
	write(t, env.TeamDir(name, layer)+"/team.yaml", "version: 1\nteam: "+name+"\nmembers:\n  - name: a\n    role: r\n    kind: claude\n")
}

func TestResolvePrefersLocalOverGlobal(t *testing.T) {
	cases := []struct {
		name      string
		force     string
		wantLayer string
	}{
		{"by default", "", source.LayerLocal},
		{"forced local", "local", source.LayerLocal},
		{"forced global", "global", source.LayerGlobal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newEnv(t)
			writeTeam(t, env, source.LayerGlobal, "t")
			writeTeam(t, env, source.LayerLocal, "t")

			got, err := env.Resolve("t", tc.force)
			if err != nil {
				t.Fatal(err)
			}
			if got.Layer != tc.wantLayer {
				t.Errorf("Layer = %q, want %q", got.Layer, tc.wantLayer)
			}
			wantPath := filepath.Join(env.TeamDir("t", tc.wantLayer), "team.yaml")
			if got.TeamYAMLPath != wantPath {
				t.Errorf("TeamYAMLPath = %q, want %q", got.TeamYAMLPath, wantPath)
			}
			if got.TeamDir != filepath.Dir(wantPath) {
				t.Errorf("TeamDir = %q, want %q", got.TeamDir, filepath.Dir(wantPath))
			}
		})
	}
}

func TestResolveRecordsWhereTheNameAlsoExists(t *testing.T) {
	env := newEnv(t)
	writeTeam(t, env, source.LayerGlobal, "t")
	writeTeam(t, env, source.LayerLocal, "t")
	writeTeam(t, env, source.LayerGlobal, "only-global")

	got, err := env.Resolve("t", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.GlobalTeamYAML != filepath.Join(env.GlobalDir, "teams", "t", "team.yaml") {
		t.Errorf("GlobalTeamYAML = %q", got.GlobalTeamYAML)
	}
	if got.LocalTeamYAML != filepath.Join(env.LocalDir, "teams", "t", "team.yaml") {
		t.Errorf("LocalTeamYAML = %q", got.LocalTeamYAML)
	}

	only, err := env.Resolve("only-global", "")
	if err != nil {
		t.Fatal(err)
	}
	if only.Layer != source.LayerGlobal {
		t.Errorf("Layer = %q, want %q", only.Layer, source.LayerGlobal)
	}
	if only.LocalTeamYAML != "" {
		t.Errorf("LocalTeamYAML = %q, want empty", only.LocalTeamYAML)
	}
	if _, err := env.Resolve("absent", ""); err == nil {
		t.Error("Resolve of an unknown name succeeded, want an error")
	}
}

func TestLoadProfilesOverlaysLocalOnGlobal(t *testing.T) {
	env := newEnv(t)
	write(t, filepath.Join(env.GlobalDir, "profiles.yaml"), `version: 1
profiles:
  global_only:
    kind: claude
  shared:
    kind: claude
    args: ["--global"]
`)
	write(t, filepath.Join(env.LocalDir, "profiles.yaml"), `version: 1
profiles:
  local_only:
    kind: codex
  shared:
    kind: codex
    args: ["--local"]
`)

	globalProfiles := filepath.Join(env.GlobalDir, "profiles.yaml")
	localProfiles := filepath.Join(env.LocalDir, "profiles.yaml")

	cases := []struct {
		name       string
		layer      string
		wantShared string
		wantArgs   string
		wantGlobal bool
		wantLocal  bool
		wantPaths  []string
	}{
		{
			name:       "a local team gets both layers, local winning",
			layer:      source.LayerLocal,
			wantShared: "codex",
			wantArgs:   "--local",
			wantGlobal: true,
			wantLocal:  true,
			wantPaths:  []string{localProfiles, globalProfiles},
		},
		{
			name:       "a global team never reads the project's profiles",
			layer:      source.LayerGlobal,
			wantShared: "claude",
			wantArgs:   "--global",
			wantGlobal: true,
			wantLocal:  false,
			wantPaths:  []string{globalProfiles},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profiles, paths, err := env.LoadProfiles(tc.layer)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := profiles["global_only"]; ok != tc.wantGlobal {
				t.Errorf("global_only present = %v, want %v", ok, tc.wantGlobal)
			}
			if _, ok := profiles["local_only"]; ok != tc.wantLocal {
				t.Errorf("local_only present = %v, want %v", ok, tc.wantLocal)
			}
			shared, ok := profiles["shared"]
			if !ok {
				t.Fatal("shared profile is missing")
			}
			if shared.Kind != tc.wantShared {
				t.Errorf("shared.Kind = %q, want %q", shared.Kind, tc.wantShared)
			}
			if got := strings.Join(shared.Args, " "); got != tc.wantArgs {
				t.Errorf("shared.Args = %q, want %q", got, tc.wantArgs)
			}
			if len(paths) != len(tc.wantPaths) {
				t.Fatalf("paths = %v, want %v", paths, tc.wantPaths)
			}
			for i := range paths {
				if paths[i] != tc.wantPaths[i] {
					t.Errorf("paths[%d] = %q, want %q", i, paths[i], tc.wantPaths[i])
				}
			}
		})
	}
}

func TestLoadProfilesWithoutFilesIsEmpty(t *testing.T) {
	env := newEnv(t)
	profiles, paths, err := env.LoadProfiles(source.LayerLocal)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 || len(paths) != 0 {
		t.Errorf("got %d profiles and %d paths, want none", len(profiles), len(paths))
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
