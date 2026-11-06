package web

import (
	"fmt"
	"github.com/aq2bq/hergy/internal/catalog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// TestPROFILES_DangerLossNamesWhatDeleteTakes: the words of the confirmation and
// the entry Delete removes come from the same target (ac-ddb6).
func TestPROFILES_DangerLossNamesWhatDeleteTakes(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	lines, refusal := server.dangerLossProfile(p2Project, "sonnet-fast")
	if refusal != nil {
		t.Fatalf("the loss: %+v", refusal)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "sonnet-fast") || !strings.Contains(joined, "ファイルは残ります。") {
		t.Errorf("the loss does not name the entry and the file: %v", lines)
	}
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	text := readFile(t, path)
	resp, body := f.api(t, server, http.MethodDelete, "/api/profile/sonnet-fast?place=project&hash="+source.Hash(text), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE answers %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	if strings.Contains(after, "sonnet-fast") {
		t.Errorf("the entry the loss named is still there:\n%s", after)
	}
	if _, err := profile.ReadAll(after, path); err != nil {
		_ = err
	}
	_ = fmt.Sprint()
}

// TestROLES_DangerLossNamesWhatDeleteTakes: a used role's loss names the
// document and its readers, and Delete takes that same document.
func TestROLES_DangerLossNamesWhatDeleteTakes(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	lines, refusal := server.dangerLossRole(catalog.PlaceProject, "builder")
	if refusal != nil {
		t.Fatalf("the loss: %+v", refusal)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "builder.md") {
		t.Errorf("the loss does not name the document: %v", lines)
	}
	hash := source.Hash(readFile(t, path))
	resp, body := f.api(t, server, http.MethodDelete, "/api/role/builder?place=project&hash="+url.QueryEscape(hash), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE answers %d: %s", resp.StatusCode, body)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the document the loss named is still there: %v", err)
	}
	// A role nobody names carries no readers line.
	unused, _ := server.dangerLossRole(catalog.PlaceGlobal, "writer")
	if strings.Contains(strings.Join(unused, "\n"), "読んでいます") {
		t.Errorf("an unused role names readers: %v", unused)
	}
}

// TestTEAMS_DangerLossNamesWhatDeleteTakes: the loss names the files Delete
// removes and, honestly, what stays in the folder (ac-ddb6).
func TestTEAMS_DangerLossNamesWhatDeleteTakes(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	dir := filepath.Dir(f.teamYAML())
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, refusal := server.dangerLossTeam(catalog.PlaceProject, "demo")
	if refusal != nil {
		t.Fatalf("the loss: %+v", refusal)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "team.yaml") || !strings.Contains(joined, "notes.txt") {
		t.Errorf("the loss does not name the files and what stays: %v", lines)
	}
	hash := source.Hash(readFile(t, f.teamYAML()))
	agree := filepath.Join(dir, "agreement.md")
	q := url.Values{}
	q.Set("place", "project")
	q.Set("hash", hash)
	if _, err := os.Stat(agree); err == nil {
		q.Set("agreement_hash", source.Hash(readFile(t, agree)))
	}
	resp, body := f.api(t, server, http.MethodDelete, "/api/team/demo?"+q.Encode(), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE answers %d: %s", resp.StatusCode, body)
	}
	if _, err := os.Stat(f.teamYAML()); !os.IsNotExist(err) {
		t.Errorf("team.yaml the loss named is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("the file the loss said stays is gone: %v", err)
	}
}
