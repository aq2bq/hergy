//go:build darwin || linux

package runningstate

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMutationLockSurvivesStateReplacementAndEnd(t *testing.T) {
	var generation Generation
	store := At(t.TempDir())
	name := testName(t, "demo")
	if _, err := store.Begin(name, testLaunch(t)); err != nil {
		t.Fatal(err)
	}
	generation = requireState(t, store, name).Generation()
	path := filepath.Join(store.dir, ".locks", "demo")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.locked(name, func() error {
		other, err := os.OpenFile(path, os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = other.Close() }()
		err = syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("second writer bypassed mutation lock: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Receive(name, generation, []Receipt{{Name: "demo-a", Prompt: "new"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.End(name); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(name, testLaunch(t)); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("replacement unlinked the lock inode")
	}
}
