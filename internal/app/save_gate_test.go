package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestATurnProblemStopsUpDryAndShowsInValidate: a team definition with a member
// that hands the turn to nobody saves, but up --dry stops and validate says the
// same sentence with the fix (d-9abb, r-df77).
func TestATurnProblemStopsUpDryAndShowsInValidate(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
description: a description for the test
members:
  - name: lead
    role: lead
    profile: claude-default
  - name: builder
    role: builder
    profile: claude-default
turns:
  - human -> lead
  - lead -> human
`)
	want := i18n.Tf("error.graph_member_no_turn", "builder", "builder", "builder", "lead")

	_, stderr, code := run("up", "demo", "--dry")
	if code == 0 {
		t.Fatalf("up --dry accepted a member with no turn:\n%s", stderr)
	}
	if !strings.Contains(stderr, want) {
		t.Errorf("up --dry does not say %q:\n%s", want, stderr)
	}
	stdout, _, code := run("validate")
	if code == 0 {
		t.Fatalf("validate accepted a member with no turn:\n%s", stdout)
	}
	if !strings.Contains(stdout, want) {
		t.Errorf("validate does not say %q:\n%s", want, stdout)
	}
}
