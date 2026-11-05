package web

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

// TestTheSaveOverAnUnreadableFileIsRefused: a write never replaces a
// profiles.yaml that is there and cannot be read, and the file keeps what it
// held (r-ea17, d-0bf1).
func TestTheSaveOverAnUnreadableFileIsRefused(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	for _, body := range []map[string]any{
		{"place": p2Project, "text": "version: 1\nprofiles: {}\n", "hash": ""},
		{"place": p2Project, "hash": "", "doc": map[string]any{"name": "sonnet-fast", "kind": "claude", "args": []string{}, "env": []any{}}},
	} {
		resp, answer := f.api(t, server, http.MethodPut, "/api/profile/sonnet-fast", body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("the save answers %d, want 422: %s", resp.StatusCode, answer)
		}
		if !strings.Contains(answer, "permission denied") {
			t.Errorf("the refusal does not say why: %s", answer)
		}
	}
	resp, _ := f.api(t, server, http.MethodDelete, "/api/profile/sonnet-fast?place=project&hash=", nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("the delete answers %d, want 422", resp.StatusCode)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("the file changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestTheDeleteOfAnUnreadableTeamIsRefused: a team.yaml that cannot be read is
// not deleted (r-ea17, d-0bf1).
func TestTheDeleteOfAnUnreadableTeamIsRefused(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	hash := source.Hash(readFile(t, path))
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	resp, body := f.api(t, server, http.MethodDelete, "/api/team/demo?place=project&hash="+url.QueryEscape(hash), nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("the delete answers %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "permission denied") {
		t.Errorf("the refusal does not say why: %s", body)
	}
}

// TestTheDeleteOfAnUnreadableRoleIsRefused: a role document that cannot be read
// is not deleted (r-ea17, d-0bf1).
func TestTheDeleteOfAnUnreadableRoleIsRefused(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	hash := source.Hash(readFile(t, path))
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	resp, body := f.api(t, server, http.MethodDelete, "/api/role/builder?place="+catalog.PlaceProject+"&hash="+url.QueryEscape(hash), nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("the delete answers %d, want 422: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "permission denied") {
		t.Errorf("the refusal does not say why: %s", body)
	}
}
