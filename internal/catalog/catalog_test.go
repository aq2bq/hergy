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
		"version: 1\nmission: a mission for the test\nteam: "+name+"\nturns:\n  - human -> a\n  - a -> human\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
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

// TestGlobalDefinitionReadsTheProjectProfile: a team definition of the global
// layer reads the project's profiles.yaml, so a definition carried from one
// project to another reads what the project it stands in carries (K1).
func TestGlobalDefinitionReadsTheProjectProfile(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.global, "profiles.yaml"), "version: 1\nprofiles:\n  globalprof:\n    kind: claude\n")
	write(t, filepath.Join(l.local, "profiles.yaml"), "version: 1\nprofiles:\n  localprof:\n    kind: codex\n")
	write(t, filepath.Join(l.global, "teams", "g", "team.yaml"),
		"version: 1\nmission: a mission for the test\nteam: g\nmembers:\n  - name: a\n    role: r\n    profile: localprof\nturns:\n  - human -> a\n  - a -> human\n")

	def, err := l.catalog().Definition("g", "")
	if err != nil {
		t.Fatal(err)
	}
	if def.Layer != source.LayerGlobal {
		t.Fatalf("Layer = %q, want global", def.Layer)
	}
	if def.Graph.Members[0].Profile != "localprof" {
		t.Errorf("the member names %q, want the project's profile localprof", def.Graph.Members[0].Profile)
	}
}

// TestDefinitionWithNoNamePicksTheOneOrNamesTheCandidates: a missing name uses
// the one project definition, or stops and names what there is to choose from.
func TestDefinitionWithNoNamePicksTheOneOrNamesTheCandidates(t *testing.T) {
	l := newLayers(t)
	l.writeTeam(t, source.LayerLocal, "one")
	def, err := l.catalog().Definition("", "")
	if err != nil || def.Name != "one" {
		t.Fatalf("Definition(\"\") = %v, %v; want the one project definition", def, err)
	}

	l.writeTeam(t, source.LayerLocal, "two")
	_, err = l.catalog().Definition("", "")
	if err == nil {
		t.Fatal("two project definitions with no name did not stop")
	}
	for _, want := range []string{"one", "two"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error %q does not name the candidate %q", err, want)
		}
	}
}

// TestRolePrefersLocalOverGlobal: a role document is looked up the one way a
// name is (K1): the project's layer first, then the global layer's.
func TestRolePrefersLocalOverGlobal(t *testing.T) {
	cases := []struct {
		name      string
		local     bool
		global    bool
		wantLayer string
	}{
		{"both layers write one", true, true, source.LayerLocal},
		{"only the global layer", false, true, source.LayerGlobal},
		{"only the project's layer", true, false, source.LayerLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayers(t)
			if tc.local {
				write(t, filepath.Join(l.local, "roles", "r.yaml"), "version: 1\n")
			}
			if tc.global {
				write(t, filepath.Join(l.global, "roles", "r.yaml"), "version: 1\n")
			}
			doc, ok := l.catalog().Role("r")
			if !ok {
				t.Fatal("Role(r) found no document")
			}
			want := filepath.Join(l.dir(tc.wantLayer), "roles", "r.yaml")
			if doc.Path != want || doc.Layer != tc.wantLayer {
				t.Errorf("Role(r) = %q (%s), want %q (%s)", doc.Path, doc.Layer, want, tc.wantLayer)
			}
		})
	}
}

// TestRolePlacesNamesBothLayersInOrder: a missing role document names every
// place it was looked for, the project's layer first then the global one (K1),
// so a person moving a definition over sees the whole list (spec/32 D2).
func TestRolePlacesNamesBothLayersInOrder(t *testing.T) {
	l := newLayers(t)
	got := l.catalog().RolePlaces("r")
	want := []string{
		filepath.Join(l.local, "roles", "r.yaml"),
		filepath.Join(l.global, "roles", "r.yaml"),
	}
	if len(got) != len(want) {
		t.Fatalf("RolePlaces(r) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("RolePlaces(r)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRoleWithNoDocumentAndOneStillMarkdown: a role with no .yaml is not a role
// document; a role left as roles/<role>.md is what RoleMarkdown names, for the
// sentence that says how to move it over.
func TestRoleWithNoDocumentAndOneStillMarkdown(t *testing.T) {
	l := newLayers(t)
	cat := l.catalog()
	if _, ok := cat.Role("r"); ok {
		t.Error("Role(r) found a document that is not there")
	}
	if _, ok := cat.RoleMarkdown("r"); ok {
		t.Error("RoleMarkdown(r) found a document that is not there")
	}
	write(t, filepath.Join(l.local, "roles", "r.md"), "- R1: old\n")
	doc, ok := cat.RoleMarkdown("r")
	if !ok || doc.Path != filepath.Join(l.local, "roles", "r.md") {
		t.Errorf("RoleMarkdown(r) = %q, %v; want the project's .md", doc.Path, ok)
	}
	if _, ok := cat.Role("r"); ok {
		t.Error("Role(r) read a Markdown file as a role document")
	}
}

// TestSharedAgreementPrefersLocalOverGlobal: the agreement every team of a layer
// shares is one file, looked up the same way (K1), and writing none is allowed.
func TestSharedAgreementPrefersLocalOverGlobal(t *testing.T) {
	l := newLayers(t)
	if _, ok := l.catalog().SharedAgreement(); ok {
		t.Error("SharedAgreement() found a document that is not there")
	}
	write(t, filepath.Join(l.global, "agreement.yaml"), "version: 1\n")
	doc, ok := l.catalog().SharedAgreement()
	if !ok || doc.Path != filepath.Join(l.global, "agreement.yaml") || doc.Layer != source.LayerGlobal {
		t.Errorf("SharedAgreement() = %q (%s), %v; want the global one", doc.Path, doc.Layer, ok)
	}
	write(t, filepath.Join(l.local, "agreement.yaml"), "version: 1\n")
	doc, ok = l.catalog().SharedAgreement()
	if !ok || doc.Path != filepath.Join(l.local, "agreement.yaml") || doc.Layer != source.LayerLocal {
		t.Errorf("SharedAgreement() = %q (%s), %v; want the project's one", doc.Path, doc.Layer, ok)
	}
}
