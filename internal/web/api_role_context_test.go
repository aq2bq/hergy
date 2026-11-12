package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

func TestRoleContextResolutionAndReadStates(t *testing.T) {
	f := newFixture(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.root, "state"))
	s := f.server(t)
	read := func(name, query string) apiRoleContext {
		t.Helper()
		resp, body := f.api(t, s, http.MethodGet, "/api/role/"+name+"/context"+query, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("%d %s", resp.StatusCode, body)
		}
		var out apiRoleContext
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	local := read("lead", "")
	if local.State != "readable" || local.Place != "project" || len(local.Uses) != 1 || local.Uses[0].Name != "demo" {
		t.Fatalf("local: %+v", local)
	}
	if got := read("missing", ""); got.State != "missing" || got.Doc != nil {
		t.Fatalf("missing: %+v", got)
	}
	f.write(t, "roles/broken.md", "[")
	if got := read("broken", ""); got.State != "readable" || got.Doc == nil || got.Doc.Text != "[" {
		t.Fatalf("broken: %+v", got)
	}
	globalPath := filepath.Join(f.global, "roles", "lead.md")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("- global role\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A catalog whose project layer holds nothing: the global document is the
	// one that reads, without a layer-only catalog (r-ea17).
	empty := t.TempDir()
	global := catalog.New(f.global, empty, projectPath(empty), source.Disk{}, func(p string) string { return p })
	hq := fakeHQ{global: global, cats: map[domainplace.Project]*catalog.Catalog{projectPath(f.proj): f.catalog()}, projects: HQProjects{Mode: "current", Current: HQProject{Path: projectPath(f.proj), Name: "sample"}}}
	s.hq = hq
	s.projects = hq
	got := read("lead", "?context=global")
	if got.Place != "global" || got.State != "readable" || len(got.Uses) != 0 || len(got.Errors) != 0 {
		t.Fatalf("shadowed global: %+v", got)
	}
}

func TestRoleModalSaveRefusesANewlyPreferredLocalFile(t *testing.T) {
	f := newFixture(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.root, "state"))
	path := filepath.Join(f.global, "roles", "shared.md")
	text := "- shared content\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	s := f.server(t)
	ctx := s.roleContext("shared", s.cat, projectPath(f.proj), false)
	if ctx.Place != "global" || ctx.Doc == nil {
		t.Fatalf("context: %+v", ctx)
	}
	// Equal contents must not make another place's hash an acceptable target.
	f.write(t, "roles/shared.md", text)
	resp, body := f.api(t, s, http.MethodPut, "/api/role/shared", map[string]any{"place": "global", "role_context": "project", "original": "shared", "hash": ctx.Hash, "doc": ctx.Doc})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	before, err := os.ReadFile(path)
	if err != nil || string(before) != text {
		t.Fatal("refusal changed global file")
	}
}

func TestGlobalRoleContextKeepsUnknownProject(t *testing.T) {
	f := newFixture(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.root, "state"))
	s := f.server(t)
	resp, body := f.api(t, s, http.MethodGet, "/api/role/lead/context?context=global", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	var out apiRoleContext
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Project.IsZero() {
		t.Fatalf("global context invented project %q", out.Project)
	}
}
