package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// fixtureNav is the band's fixture: the profile screen's fixture with the role
// fixture over it, and the band's own two shared definitions on top
// (spec-nav.md 5.1). testdata/roles-v2 already carries the profile screen's
// profiles.yaml, so it stands in for testdata/profiles-v2 — the same way the
// browser test's copyNavFixture lays the two down, so the numbers a screen reads
// here are the ones the e2e walks see.
func fixtureNav(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config")
	f := &fixture{root: root, proj: proj, global: global}
	for _, one := range []string{"roles-v2", "nav"} {
		src := filepath.Join("..", "..", "testdata", one)
		for _, half := range []struct{ from, to string }{
			{filepath.Join(src, "project", ".hergy"), filepath.Join(proj, ".hergy")},
			{filepath.Join(src, "config", "hergy"), global},
		} {
			if _, err := os.Stat(half.from); err != nil {
				continue
			}
			copyTree(t, half.from, half.to)
		}
	}
	return f
}

// bandOf is the band of one page as it stands, from its one <nav> to the end of
// it, so a check looks at the band and not at a tab that happens to say the same
// number.
func bandOf(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<nav class="screens"`)
	if start < 0 {
		t.Fatalf("the page carries no band:\n%s", body)
	}
	rest := body[start:]
	end := strings.Index(rest, "</nav>")
	if end < 0 {
		t.Fatalf("the band is not closed:\n%s", body)
	}
	return rest[:end+len("</nav>")]
}

// TestNAV_N11_TheBandNumberFollowsASave: a save that adds one role document
// answers with the whole list grown by one, and the next page a person draws
// counts the same — so the band's number never lags the save behind it. The
// profile screen keeps its number the same way, from its list of rows
// (spec-nav.md 1.1, spec-boundary 5).
func TestNAV_N11_TheBandNumberFollowsASave(t *testing.T) {
	f := fixtureNav(t)
	server := f.server(t)

	// A role the fixture does not hold yet, saved through the boundary the way
	// the screen's new row saves one: no file stands behind the name, so the
	// save writes a fresh document (spec-roles 1.2.4).
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/nav-one", map[string]any{
		"place": "project", "original": "", "hash": "",
		"doc": map[string]any{
			"name": "nav-one", "revision": 1,
			"sections": []map[string]any{{
				"heading": "使命",
				"rules":   []map[string]any{{"id": "NAV-1", "text": "帯の数のために足した規則。", "send_why": true, "in_prompt": true}},
			}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("adding one role answered %d, want 200: %v", resp.StatusCode, decoded)
	}
	items, _ := decoded["items"].([]any)
	if len(items) != 7 {
		t.Fatalf("the save's list carries %d roles, want the fixture's 6 and one more", len(items))
	}
	// The next page a person draws counts the same 7.
	resp, body := f.get(t, server, "/roles")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /roles answers %d, want 200", resp.StatusCode)
	}
	if band := bandOf(t, body); !strings.Contains(band, `data-screen="roles">ロール<span class="n">7</span>`) {
		t.Errorf("the band after the save does not count 7 roles:\n%s", band)
	}

	// The profile screen: one more row of the project's profiles.yaml, and the
	// band counts the rows the list now holds (spec-profiles 1.2).
	profilesPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, profilesPath)
	resp, decoded = f.apiJSON(t, server, http.MethodPut, "/api/profile/nav-one", map[string]any{
		"place": "project", "original": "", "hash": source.Hash(before),
		"doc": map[string]any{"name": "nav-one", "kind": "claude", "args": []string{}, "env": []map[string]string{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("adding one profile answered %d, want 200: %v", resp.StatusCode, decoded)
	}
	items, _ = decoded["items"].([]any)
	if len(items) != 12 {
		t.Fatalf("the save's list carries %d profiles, want the fixture's 11 and one more", len(items))
	}
	resp, body = f.get(t, server, "/profiles")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profiles answers %d, want 200", resp.StatusCode)
	}
	if band := bandOf(t, body); !strings.Contains(band, `data-screen="profiles">プロファイル<span class="n">12</span>`) {
		t.Errorf("the band after the save does not count 12 profiles:\n%s", band)
	}
}
