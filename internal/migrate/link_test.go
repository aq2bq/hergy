package migrate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestUnsupportedHardLinkPreservesSourceAndNamesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "role.yaml")
	original := "version: 1\nsections:\n  - heading: Work\n    rules:\n      - text: Keep this.\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	err = f.apply(func(string, string) error { return syscall.EOPNOTSUPP })
	if !errors.Is(err, syscall.EOPNOTSUPP) || !strings.Contains(err.Error(), f.Backup) {
		t.Fatalf("error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Fatalf("source changed: %q, %v", got, err)
	}
	for _, p := range []string{f.Backup, f.Destination} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial file: %s: %v", p, err)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".migrate-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files: %v", leftovers)
	}
	if err := f.Apply(); err != nil {
		t.Fatal(err)
	}
}
