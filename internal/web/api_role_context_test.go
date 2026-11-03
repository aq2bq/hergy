package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
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
	f.write(t, "roles/broken.yaml", "[")
	if got := read("broken", ""); got.State != "unreadable" || got.Doc != nil {
		t.Fatalf("broken: %+v", got)
	}
	globalPath := filepath.Join(f.global, "roles", "lead.yaml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("version: 1\nsections:\n  - rules:\n      - text: global role\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	global := f.catalog().LayerOnly("global")
	hq := fakeHQ{global: global, cats: map[string]*catalog.Catalog{f.proj: f.catalog()}, projects: HQProjects{Mode: "current", Current: HQProject{Path: f.proj, Name: "sample"}}}
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
	path := filepath.Join(f.global, "roles", "shared.yaml")
	text := "version: 1\nsections:\n  - rules:\n      - text: shared content\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	s := f.server(t)
	ctx := s.roleContext("shared", s.cat, f.proj, false)
	if ctx.Place != "global" || ctx.Doc == nil {
		t.Fatalf("context: %+v", ctx)
	}
	// Equal contents must not make another place's hash an acceptable target.
	f.write(t, "roles/shared.yaml", text)
	resp, body := f.api(t, s, http.MethodPut, "/api/role/shared", map[string]any{"place": "global", "role_context": "project", "original": "shared", "hash": ctx.Hash, "doc": ctx.Doc})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	before, err := os.ReadFile(path)
	if err != nil || string(before) != text {
		t.Fatal("refusal changed global file")
	}
}
