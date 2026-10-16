package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheRepositoriesOwnHermyPassesValidate: the project keeps its own team
// definition in .hergy/, and a person writes those files by hand, so a change
// that moves where a document stands has to carry this .hergy over in the same
// change (d-8d89). This reads the repository's own .hergy the way a person
// would, with the config and the state moved into a working directory so the
// check never looks at the machine's own hergy files (d-a861), and asks for
// neither a problem nor a warning: a warning is a file the next change has to
// fix.
func TestTheRepositoriesOwnHermyPassesValidate(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".hergy")); err != nil {
		t.Skip("the repository keeps no .hergy")
	}
	t.Chdir(root)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HERDR_ENV", "")
	t.Setenv("HERDR_PANE_ID", "")

	stdout, stderr, code := run("validate")
	if code != 0 {
		t.Fatalf("hergy validate of the repository's own .hergy: exit %d\n%s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "problems: 0") {
		t.Errorf("the repository's own .hergy has a problem:\n%s", stdout)
	}
	if strings.Contains(stdout, "warning:") {
		t.Errorf("the repository's own .hergy carries a warning:\n%s", stdout)
	}
}

// repoRoot is the root of the repository this test stands in: the nearest
// directory above the test that holds the module's go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's directory")
		}
		dir = parent
	}
}
