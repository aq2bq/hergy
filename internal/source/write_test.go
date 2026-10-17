package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestHashIsTheHashTheScreenCarries: the same text hashes the same, different
// texts do not, and the empty text has a hash of its own: a screen that read
// nothing carries that one (C11).
func TestHashIsTheHashTheScreenCarries(t *testing.T) {
	sum := Hash("a team.yaml")
	if sum != Hash("a team.yaml") {
		t.Error("the same text hashed differently")
	}
	if Hash("before") == Hash("after") {
		t.Error("two texts hashed the same")
	}
	if Hash("") == Hash("before") {
		t.Error("the empty text hashed like a file")
	}
}

// TestWriteIfUnchangedWritesWhenTheFileIsTheOneRead writes over the file the
// screen read, and leaves nothing beside it.
func TestWriteIfUnchangedWritesWhenTheFileIsTheOneRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.yaml")
	write(t, path, "before")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteIfUnchanged(path, Hash("before"), "after"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "after" {
		t.Errorf("the file holds %q, want %q", b, "after")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want the file's own 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the directory holds %d files, want the one: %v", len(entries), entries)
	}
}

// TestWriteIfUnchangedRefusesAFileThatMoved: a file that changed since the read
// is not written over, and the caller is told which of the two happened (C11).
func TestWriteIfUnchangedRefusesAFileThatMoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.yaml")
	write(t, path, "someone else's text")

	err := WriteIfUnchanged(path, Hash("what the screen read"), "the screen's text")
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	b, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(b) != "someone else's text" {
		t.Errorf("the file holds %q, want it untouched", b)
	}
}

// TestWriteIfUnchangedMakesAFileThatIsNotThere: a file a screen read as nothing
// is being made (a layer without a profiles.yaml yet, r-daf8).
func TestWriteIfUnchangedMakesAFileThatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")

	if err := WriteIfUnchanged(path, Hash(""), "version: 1\n"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "version: 1\n" {
		t.Errorf("the file holds %q, want the text", b)
	}
}

// TestWriteIfUnchangedReportsAFileThatIsNotThereAndWasReadAsSomething: a file
// that stood there and is gone is not the one the screen read, and the read's
// own error is what the caller gets to name (C11).
func TestWriteIfUnchangedReportsAFileThatIsNotThereAndWasReadAsSomething(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.yaml")

	err := WriteIfUnchanged(path, Hash("what the screen read"), "the screen's text")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the file's own not-exist error", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("the file was written: %v", statErr)
	}
}

// TestWriteBesideFollowsALink: a dotfiles setup keeps the files under
// ~/.config/hergy as links, so the write follows the link, and the link stays a
// link.
func TestWriteBesideFollowsALink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.yaml")
	write(t, target, "before")
	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this filesystem has no links: %v", err)
	}

	if err := WriteBeside(link, "after"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link is no longer a link: %v, %v", info, err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "after" {
		t.Errorf("the file it points at holds %q, want %q", b, "after")
	}
}
