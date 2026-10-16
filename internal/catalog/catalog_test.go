package catalog

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

type layers struct {
	global string
	local  string
}

func newLayers(t *testing.T) layers {
	t.Helper()
	root := t.TempDir()
	return layers{global: filepath.Join(root, "global"), local: filepath.Join(root, "local")}
}

func (l layers) catalog() *Catalog {
	return New(l.global, l.local, l.global, source.Disk{}, func(path string) string { return path })
}

func (l layers) writeTeam(t *testing.T, layer, name string) {
	t.Helper()
	profiles := "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n"
	write(t, filepath.Join(l.dir(layer), "profiles.yaml"), profiles)
	write(t, filepath.Join(l.dir(layer), "teams", name, "team.yaml"),
		"version: 1\nteam: "+name+"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
}

func (l layers) dir(layer string) string {
	if layer == source.LayerLocal {
		return l.local
	}
	return l.global
}

func TestDefinitionPrefersLocalOverGlobal(t *testing.T) {
	cases := []struct {
		name      string
		layer     string
		wantLayer string
	}{
		{"by default", "", source.LayerLocal},
		{"forced local", source.LayerLocal, source.LayerLocal},
		{"forced global", source.LayerGlobal, source.LayerGlobal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayers(t)
			l.writeTeam(t, source.LayerGlobal, "t")
			l.writeTeam(t, source.LayerLocal, "t")

			def, err := l.catalog().Definition("t", tc.layer)
			if err != nil {
				t.Fatal(err)
			}
			if def.Layer != tc.wantLayer {
				t.Errorf("Layer = %q, want %q", def.Layer, tc.wantLayer)
			}
			wantPath := filepath.Join(l.dir(tc.wantLayer), "teams", "t", "team.yaml")
			if def.TeamYAML != wantPath {
				t.Errorf("TeamYAML = %q, want %q", def.TeamYAML, wantPath)
			}
			if def.Dir != filepath.Dir(wantPath) {
				t.Errorf("Dir = %q, want %q", def.Dir, filepath.Dir(wantPath))
			}
		})
	}
}

func TestDefinitionRecordsTheOtherLayer(t *testing.T) {
	l := newLayers(t)
	l.writeTeam(t, source.LayerGlobal, "t")
	l.writeTeam(t, source.LayerLocal, "t")
	l.writeTeam(t, source.LayerGlobal, "only-global")

	got, err := l.catalog().Definition("t", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.OtherYAML != filepath.Join(l.global, "teams", "t", "team.yaml") {
		t.Errorf("OtherYAML = %q", got.OtherYAML)
	}

	only, err := l.catalog().Definition("only-global", "")
	if err != nil {
		t.Fatal(err)
	}
	if only.Layer != source.LayerGlobal {
		t.Errorf("Layer = %q, want %q", only.Layer, source.LayerGlobal)
	}
	if only.OtherYAML != "" {
		t.Errorf("OtherYAML = %q, want empty", only.OtherYAML)
	}
	if _, err := l.catalog().Definition("absent", ""); err == nil {
		t.Error("Definition of an unknown name succeeded, want an error")
	}
}

// TestProfilesOverlaysLocalOnGlobal: there is one way to a name (K1). A team
// definition reads the global layer's profiles with the local layer's laid over
// them, whatever layer the definition itself sits in.
func TestProfilesOverlaysLocalOnGlobal(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "profiles.yaml"), `version: 1
profiles:
  global_only:
    kind: claude
  shared:
    kind: claude
    args: ["--global"]
`)
	write(t, filepath.Join(l.local, "profiles.yaml"), `version: 1
profiles:
  local_only:
    kind: codex
  shared:
    kind: codex
    args: ["--local"]
`)

	globalProfiles := filepath.Join(l.global, "profiles.yaml")
	localProfiles := filepath.Join(l.local, "profiles.yaml")
	profiles, paths, err := l.catalog().Profiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"global_only", "local_only", "shared"} {
		if _, ok := profiles[name]; !ok {
			t.Errorf("%s is missing", name)
		}
	}
	shared := profiles["shared"]
	if shared.Kind != "codex" {
		t.Errorf("shared.Kind = %q, want codex (local wins)", shared.Kind)
	}
	if got := strings.Join(shared.Args, " "); got != "--local" {
		t.Errorf("shared.Args = %q, want --local", got)
	}
	if want := []string{localProfiles, globalProfiles}; !strings.Contains(paths, localProfiles) || !strings.Contains(paths, globalProfiles) {
		t.Errorf("paths = %q, want %v", paths, want)
	}
}

func TestProfilesWithoutFilesIsEmpty(t *testing.T) {
	l := newLayers(t)
	profiles, paths, err := l.catalog().Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 || paths != "" {
		t.Errorf("got %d profiles and %q paths, want none", len(profiles), paths)
	}
}
