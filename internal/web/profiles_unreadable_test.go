package web

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPROFILES_TheFileThatCannotBeReadIsOneRowWithTheReason: a profiles.yaml
// that cannot be read stands as one row named after the file, with the reason at
// its place, so the list still draws it and the band still counts it (r-ea17,
// d-17ab).
func TestPROFILES_TheFileThatCannotBeReadIsOneRowWithTheReason(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	server := f.server(t)

	list := pv2ListBody(t, f, server, "/api/profile")
	var row *apiItem
	for i := range list.Items {
		if list.Items[i].Name == "profiles.yaml" {
			row = &list.Items[i]
		}
	}
	if row == nil {
		t.Fatalf("the list carries no row for the file it could not read: %+v", list.Items)
	}
	place, ok := row.Places[p2Project]
	if !ok {
		t.Fatalf("the row has no project place: %+v", row.Places)
	}
	if place.Problem == "" {
		t.Errorf("the project place carries no reason")
	}
	if row.Problems.Err == 0 {
		t.Errorf("the row carries no problem")
	}
}
