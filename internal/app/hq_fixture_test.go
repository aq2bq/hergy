package app

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// TestHQ_FixtureListsThreeProjectsAndGlobal: testdata/hq is the headquarters'
// sample, the one the screens are looked at with (e2e/scripts/look.mjs). It is
// a machine's own shape: config.yaml lists three projects under ~/src, each
// project holds its own .hergy, and the settings directory holds the global
// layer. Every list a screen draws stands under the three projects' heads and
// the global layer's one, so the sample cannot go stale without this failing.
func TestHQ_FixtureListsThreeProjectsAndGlobal(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(cwd, "..", "..", "testdata", "hq")
	f := newFixture(t)
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
	home := os.Getenv("HOME")
	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		dir := filepath.Join(home, "src", name)
		copyDir(t, filepath.Join(src, "projects", name, ".hergy"), filepath.Join(dir, ".hergy"))
		// A project is a git repository, the way hergy finds one; the sample's
		// projects stand where a person's do, under the home directory.
		if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v: %s", name, err, out)
		}
	}

	base, stop := startServe(t, f)
	defer stop()
	base = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+`).FindString(base)
	if base == "" {
		t.Fatal("the serve printed no address")
	}
	read := func(path string, into any) {
		t.Helper()
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s answers %d: %s", path, resp.StatusCode, body)
		}
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("GET %s does not read: %v\n%s", path, err, body)
		}
	}

	var projects struct {
		Mode     string `json:"mode"`
		Projects []struct {
			Name     string `json:"name"`
			Path     string `json:"path"`
			Listed   bool   `json:"listed"`
			HasHergy bool   `json:"has_hergy"`
		} `json:"projects"`
		Global struct {
			Defs, Roles, Profiles int
		} `json:"global"`
	}
	read("/api/hq/projects", &projects)
	if projects.Mode != "listed" {
		t.Fatalf("the sample is read in mode %q, want the listed projects", projects.Mode)
	}
	if len(projects.Projects) != len(names) {
		t.Fatalf("the sample lists %d projects, want %d", len(projects.Projects), len(names))
	}
	for i, one := range projects.Projects {
		if one.Name != names[i] || !one.Listed || !one.HasHergy {
			t.Errorf("project %d reads %+v, want %s listed with its .hergy", i, one, names[i])
		}
	}

	// Every list's groups: the three projects in the config's order, then the
	// global layer once.
	for _, kind := range []string{"teams", "roles", "profiles"} {
		var groups struct {
			Groups []struct {
				Project *struct {
					Name string `json:"name"`
				} `json:"project"`
				Global bool `json:"global"`
				List   struct {
					Items []struct {
						Name string `json:"name"`
					} `json:"items"`
				} `json:"list"`
			} `json:"groups"`
		}
		read("/api/hq/"+kind, &groups)
		if len(groups.Groups) != len(names)+1 {
			t.Fatalf("%s has %d groups, want the three projects and the global layer", kind, len(groups.Groups))
		}
		for i, name := range names {
			group := groups.Groups[i]
			if group.Project == nil || group.Project.Name != name || group.Global {
				t.Errorf("%s's group %d reads %+v, want %s", kind, i, group.Project, name)
			}
			if len(group.List.Items) == 0 {
				t.Errorf("%s's %s group is empty; the sample should show one", kind, name)
			}
		}
		if global := groups.Groups[len(names)]; !global.Global || global.Project != nil {
			t.Errorf("%s's last group is not the global layer: %+v", kind, global)
		}
	}
}
