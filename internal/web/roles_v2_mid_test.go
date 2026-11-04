package web

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// This file holds the 小・中 checks of the role screen that are held without a
// browser: the preview a member's prompt carries (ROLES-M1) and the writes the
// role screen sends (ROLES-F1, F2, F4, F5, F6, F7). The fixture is the
// real-scale one (testdata/roles-v2), the same one the mock and the e2e walks
// were drawn on.
//
// Every helper here is this file's own: another worker's test file holds its own
// copier and its own block readers, and the names never meet.

// ruFixtureRoles copies the real-scale roles fixture into a working project and
// an outside settings directory, and answers a fixture whose server reads it.
func ruFixtureRoles(t *testing.T) *fixture {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "roles-v2")
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config")
	copyTree(t, filepath.Join(src, "project"), proj)
	copyTree(t, filepath.Join(src, "config", "hergy"), global)
	return &fixture{root: root, proj: proj, global: global}
}

// ruRolePath is where one layer keeps one role document, the way the catalog
// reads it.
func ruRolePath(f *fixture, place, name string) string {
	if place == catalog.PlaceGlobal {
		return filepath.Join(f.global, "roles", name+".md")
	}
	return filepath.Join(f.proj, ".hergy", "roles", name+".md")
}

// ruLoadDraft reads one role file and answers the draft a panel would send for
// it, with the text it was read from.
func ruLoadDraft(t *testing.T, path string) (*r2Doc, string) {
	t.Helper()
	text := readFile(t, path)
	return r2DocOf(text, path), text
}

// TestROLES_F4_RenamingWritesTheReadingTeamsRoleLine: a save that renames a role
// and rewrites the definitions that read it changes the file's name, leaves the
// new file byte for byte the old one, and changes only the role: lines of the
// team.yaml files whose members played the old name.
func TestROLES_F4_RenamingWritesTheReadingTeamsRoleLine(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	old := ruRolePath(f, catalog.PlaceProject, "scout")
	before := readFile(t, old)
	teams := map[string]string{}
	for _, name := range []string{"core", "docs", "migrate", "research"} {
		teams[name] = readFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml"))
	}
	sharedScout := ruRolePath(f, catalog.PlaceGlobal, "scout")
	sharedBefore := readFile(t, sharedScout)

	draft, text := ruLoadDraft(t, old)
	draft.Name = "scout-3"
	resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/scout", map[string]any{
		"place": catalog.PlaceProject, "original": "scout", "hash": source.Hash(text),
		"rename_users": true, "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d", resp.StatusCode)
	}

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the old file is still there: %v", err)
	}
	next := ruRolePath(f, catalog.PlaceProject, "scout-3")
	if got := readFile(t, next); got != before {
		t.Errorf("the renamed file is not the old one byte for byte:\n%s", got)
	}
	research := filepath.Join(f.proj, ".hergy", "teams", "research", "team.yaml")
	want := strings.ReplaceAll(teams["research"], "    role: scout\n", "    role: scout-3\n")
	if got := readFile(t, research); got != want {
		t.Errorf("research's team.yaml is not the fixture with only the role: lines changed:\n%s\nwant\n%s", got, want)
	}
	if n := strings.Count(teams["research"], "    role: scout\n"); n != 2 {
		t.Fatalf("research reads scav's role on %d lines, want 2", n)
	}
	for _, name := range []string{"core", "docs", "migrate"} {
		path := filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml")
		if got := readFile(t, path); got != teams[name] {
			t.Errorf("%s's team.yaml changed, and no member of it plays scout:\n%s", name, got)
		}
	}
	if got := readFile(t, sharedScout); got != sharedBefore {
		t.Errorf("renaming the project's scout changed the shared one:\n%s", got)
	}
}

// TestROLES_F5_AStaleHashIsRefused: a save whose page read a text another place
// has since changed is refused with error.file_changed — the sentence that keeps
// the draft and names the file — and writes nothing.
func TestROLES_F5_AStaleHashIsRefused(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	read := readFile(t, path)
	draft, _ := ruLoadDraft(t, path)

	changed := read + "\n# ほかの所で変わった\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
		"place": catalog.PlaceProject, "original": "builder", "hash": source.Hash(read), "doc": draft,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the save answered %d, want 409", resp.StatusCode)
	}
	got, _ := decoded["file"].(map[string]any)
	if got == nil || got["changed"] == nil || got["changed"] == "" {
		t.Fatalf("the refusal carries no file with a changed hour: %v", decoded["file"])
	}
	if want := i18n.Tf("error.file_changed", path, got["changed"]); decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if got := readFile(t, path); got != changed {
		t.Errorf("the refused save wrote the file:\n%s", got)
	}
}

// TestROLES_F7_TheFiveWrites: a role added, a duplicate and a copy to the shared
// place write new files (the duplicate and the copy byte for byte from the
// document they came from), a delete takes that one file away, and a role no
// layer holds a document for gets a new one.
func TestROLES_F7_TheFiveWrites(t *testing.T) {
	t.Run("a role added", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		draft := r2NewRole("ra-new")

		draft.Text = "新しく足したロールの規則。"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
			"place": catalog.PlaceProject, "hash": "", "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		written := readFile(t, ruRolePath(f, catalog.PlaceProject, "ra-new"))
		if !strings.Contains(written, "新しく足したロールの規則。") {
			t.Errorf("the added file does not hold the rule's text:\n%s", written)
		}
		if written != draft.Text {
			t.Fatalf("saved prose differs: %q", written)
		}
	})

	t.Run("builder duplicated", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		draft.Name = "builder-2"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder-2", map[string]any{
			"place": catalog.PlaceProject, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceProject, "builder-2")); got != before {
			t.Errorf("the duplicate does not keep the original's lines:\n%s", got)
		}
		if got := readFile(t, src); got != before {
			t.Errorf("duplicating changed the original:\n%s", got)
		}
	})

	t.Run("copied to the shared place", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != before {
			t.Errorf("the copy does not keep the original's lines:\n%s", got)
		}
		if got := readFile(t, src); got != before {
			t.Errorf("copying changed the original:\n%s", got)
		}
	})

	t.Run("a role taken away", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		path := ruRolePath(f, catalog.PlaceProject, "builder")
		hash := source.Hash(readFile(t, path))
		resp, _ := f.api(t, server, http.MethodDelete, "/api/role/builder?place="+catalog.PlaceProject+"&hash="+url.QueryEscape(hash), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the delete answered %d", resp.StatusCode)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the file is still there: %v", err)
		}
		if _, err := os.Stat(ruRolePath(f, catalog.PlaceProject, "lead")); err != nil {
			t.Errorf("the delete took another file away: %v", err)
		}
		if _, err := os.Stat(ruRolePath(f, catalog.PlaceGlobal, "reviewer")); err != nil {
			t.Errorf("the delete took the shared reviewer away: %v", err)
		}
	})

	t.Run("a role no layer holds", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		dir := filepath.Join(f.proj, ".hergy", "teams", "ghost")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		team := "version: 1\ndescription: \"文書の無いロールを読む\"\n" +
			"members:\n  - name: lead\n    role: lead\n    profile: opus-deep\n" +
			"  - name: ghost-writer\n    role: ghost\n    profile: gemini-pro\n" +
			"turns:\n  - human -> lead\n  - lead -> ghost-writer\n  - ghost-writer -> lead\n"
		if err := os.WriteFile(filepath.Join(dir, "team.yaml"), []byte(team), 0o644); err != nil {
			t.Fatal(err)
		}

		_, list := f.apiJSON(t, server, http.MethodGet, "/api/role", nil)
		found := false
		for _, item := range list["items"].([]any) {
			one, _ := item.(map[string]any)
			if one["name"] != "ghost" {
				continue
			}
			summary, _ := one["summary"].(map[string]any)
			found = summary["noDoc"] == true
		}
		if !found {
			t.Fatalf("the list has no 文書の無い row for ghost: %v", list["items"])
		}

		draft := r2NewRole("ghost")

		draft.Text = "文書の無いロールに足した規則。"
		resp, _ := f.apiJSON(t, server, http.MethodPut, "/api/role/ghost", map[string]any{
			"place": catalog.PlaceProject, "hash": "", "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d", resp.StatusCode)
		}
		written := readFile(t, ruRolePath(f, catalog.PlaceProject, "ghost"))
		if !strings.Contains(written, "文書の無いロールに足した規則。") {
			t.Errorf("the new file does not hold the rule's text:\n%s", written)
		}
		if written != draft.Text {
			t.Fatalf("saved prose differs: %q", written)
		}
	})
}

// TestROLES_F8_ALayerWithoutRolesGetsOne: a layer that is there but holds no
// roles/ yet gets one on the first save, whether the save adds a role, copies one
// to the shared place or writes the YAML tab's text; a layer that is not there is
// still refused with the sentence that names it (n-86c7, d-4cba).
func TestROLES_F8_ALayerWithoutRolesGetsOne(t *testing.T) {
	// noRoles answers a fixture whose one layer has lost its roles/.
	noRoles := func(t *testing.T, place string) *fixture {
		t.Helper()
		f := ruFixtureRoles(t)
		if err := os.RemoveAll(filepath.Dir(ruRolePath(f, place, "x"))); err != nil {
			t.Fatal(err)
		}
		return f
	}

	t.Run("a role added", func(t *testing.T) {
		for _, place := range []string{catalog.PlaceProject, catalog.PlaceGlobal} {
			f := noRoles(t, place)
			server := f.server(t)
			draft := r2NewRole("ra-new")
			draft.Text = "roles/ の無い層に足したロールの規則。"
			resp, body := f.api(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
				"place": place, "hash": "", "doc": draft,
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: the save answered %d: %s", place, resp.StatusCode, body)
			}
			if got := readFile(t, ruRolePath(f, place, "ra-new")); !strings.Contains(got, "roles/ の無い層に足したロールの規則。") {
				t.Errorf("%s: the added file does not hold the rule:\n%s", place, got)
			}
		}
	})

	t.Run("copied to the shared place", func(t *testing.T) {
		f := noRoles(t, catalog.PlaceGlobal)
		server := f.server(t)
		src := ruRolePath(f, catalog.PlaceProject, "builder")
		before := readFile(t, src)
		draft, _ := ruLoadDraft(t, src)
		resp, body := f.api(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "",
			"base": map[string]any{"place": catalog.PlaceProject, "name": "builder"}, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != before {
			t.Errorf("the copy does not keep the original's lines:\n%s", got)
		}
	})

	t.Run("the YAML tab's text", func(t *testing.T) {
		f := noRoles(t, catalog.PlaceGlobal)
		server := f.server(t)
		text := readFile(t, ruRolePath(f, catalog.PlaceProject, "builder"))
		resp, body := f.api(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceGlobal, "name": "builder", "hash": source.Hash(""), "text": text,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
		}
		if got := readFile(t, ruRolePath(f, catalog.PlaceGlobal, "builder")); got != text {
			t.Errorf("the save wrote %q, want the typed text", got)
		}
	})

	t.Run("a layer that is not there", func(t *testing.T) {
		f := ruFixtureRoles(t)
		server := f.server(t)
		if err := os.RemoveAll(f.global); err != nil {
			t.Fatal(err)
		}
		resp, body := f.api(t, server, http.MethodPut, "/api/role/ra-new", map[string]any{
			"place": catalog.PlaceGlobal, "hash": "", "doc": r2NewRole("ra-new"),
		})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("the save answered %d, want 422: %s", resp.StatusCode, body)
		}
		if !strings.Contains(body, "roles/") {
			t.Errorf("the refusal does not name roles/: %s", body)
		}
		if _, err := os.Stat(f.global); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the refused save made the layer: %v", err)
		}
	})
}

// TestROLES_F4b_ARenameWithTheMarkOffLeavesTheTeamsAlone: the same save with the
// rename's mark off renames the role document and nothing else — the definitions
// that name the old role keep it, and are listed as broken until the person
// rewrites them (r-b0bc, ac-2abf).
func TestROLES_F4b_ARenameWithTheMarkOffLeavesTheTeamsAlone(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	old := ruRolePath(f, catalog.PlaceProject, "scout")
	research := filepath.Join(f.proj, ".hergy", "teams", "research", "team.yaml")
	before := readFile(t, research)

	draft, text := ruLoadDraft(t, old)
	draft.Name = "scout-3"
	resp, body := f.apiJSON(t, server, http.MethodPut, "/api/role/scout", map[string]any{
		"place": catalog.PlaceProject, "original": "scout", "hash": source.Hash(text),
		"rename_users": false, "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d: %v", resp.StatusCode, body)
	}
	if _, err := os.Stat(ruRolePath(f, catalog.PlaceProject, "scout-3")); err != nil {
		t.Errorf("the renamed role document was not written: %v", err)
	}
	if got := readFile(t, research); got != before {
		t.Errorf("a save with the mark off changed the team that names the role:\n%s\nwant\n%s", got, before)
	}
}
