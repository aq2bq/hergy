package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// roleDoc is one role document with no title, the shape a person who wrote the
// Japanese name into the file name leaves behind.
const jpRoleDoc = "version: 1\nsections:\n  - heading: 進め方\n    rules:\n      - id: PM-1\n        text: 今日決めることを 1 つ書く。\n"

// TestValidateARoleNameThatIsNotAnIdentifierIsBroken: a role's name is an
// identifier (d-841e, r-7ea6). validate stands the role's own file broken, and
// says which name cannot be used and how to fix it.
func TestValidateARoleNameThatIsNotAnIdentifierIsBroken(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "進行役.yaml"), jpRoleDoc)

	out, stderr, code := run("validate")
	if code == 0 {
		t.Fatalf("validate accepts a role named 進行役:\n%s", out)
	}
	if stderr != "" {
		t.Errorf("validate wrote to stderr: %s", stderr)
	}
	block := ""
	for one := range strings.SplitSeq(out, "\nfile: ") {
		if strings.HasPrefix(one, "file: .hergy/roles/進行役.yaml\n") || strings.HasPrefix(one, ".hergy/roles/進行役.yaml\n") {
			block = one
		}
	}
	if block == "" {
		t.Fatalf("validate does not name the role's file:\n%s", out)
	}
	if !strings.Contains(block, "result: broken") {
		t.Errorf("the role's file is not broken:\n%s", block)
	}
	want := i18n.Tf("error.role_name_pattern", "進行役")
	if !strings.Contains(block, "detail: "+want) {
		t.Errorf("the role's file does not say why (want %q):\n%s", want, block)
	}
}

// TestUpStopsOnARoleNameThatIsNotAnIdentifier: up stops before it builds a
// prompt when a member reads a role whose name is not an identifier, with the
// same sentence validate prints.
func TestUpStopsOnARoleNameThatIsNotAnIdentifier(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "進行役.yaml"), jpRoleDoc)
	team := filepath.Join(".hergy", "teams", "demo", "team.yaml")
	writeFile(t, team, strings.Replace(readFile(t, team), "    role: lead\n", "    role: 進行役\n", 1))

	stdout, stderr, code := run("up", "demo", "--dry")
	if code == 0 {
		t.Fatalf("up builds a team whose member reads a role named 進行役:\n%s", stdout)
	}
	want := i18n.Tf("error.role_name_pattern", "進行役")
	if !strings.Contains(stderr, want) {
		t.Errorf("up's refusal does not say why (want %q):\n%s", want, stderr)
	}
}
