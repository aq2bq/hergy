//go:build darwin || linux

package source

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTeamLockAcrossProcesses(t *testing.T) {
	if path := os.Getenv("HERGY_TEST_TEAM_LOCK"); path != "" {
		lock := LockTeamStart
		if os.Getenv("HERGY_TEST_TEAM_MODE") == "rename" {
			lock = func(path string) (func(), error) { return LockTeams(path) }
		}
		release, err := lock(path)
		if release != nil {
			release()
		}
		wantBusy := os.Getenv("HERGY_TEST_TEAM_BUSY") == "true"
		if errors.Is(err, errTeamLockBusy) != wantBusy || (!wantBusy && err != nil) {
			t.Fatalf("lock error = %v, want busy = %v", err, wantBusy)
		}
		return
	}
	for _, parent := range []string{"start", "rename"} {
		for _, child := range []string{"start", "rename"} {
			t.Run(parent+"/"+child, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("XDG_CONFIG_HOME", dir)
				t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
				// Both projects can start the same global definition concurrently.
				path := filepath.Join(dir, "demo", "team.yaml")
				lock := LockTeamStart
				if parent == "rename" {
					lock = func(path string) (func(), error) { return LockTeams(path) }
				}
				release, err := lock(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(release)
				cmd := exec.Command(os.Args[0], "-test.run=^TestTeamLockAcrossProcesses$")
				cmd.Dir = t.TempDir() // A different project's process.
				cmd.Env = append(os.Environ(), "HERGY_TEST_TEAM_LOCK="+path, "HERGY_TEST_TEAM_MODE="+child,
					fmt.Sprintf("HERGY_TEST_TEAM_BUSY=%t", parent == "rename" || child == "rename"))
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("child: %v %s", err, out)
				}
				release()
				again, err := LockTeams(path)
				if err != nil {
					t.Fatal(err)
				}
				again()
			})
		}
	}
}

func TestTeamLocksUseStateAndAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	state := filepath.Join(dir, "state")
	t.Setenv("XDG_STATE_HOME", state)
	for _, root := range []string{filepath.Join(dir, "project", ".hergy"), filepath.Join(dir, "config", "hergy")} {
		teams := filepath.Join(root, "teams")
		if err := os.MkdirAll(teams, 0o755); err != nil {
			t.Fatal(err)
		}
		// Configuration can be read-only: the lock must never be written here.
		if err := os.Chmod(teams, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(teams, 0o755) })
		path := filepath.Join(teams, "demo", "team.yaml")
		release, err := LockTeams(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(release)
		// Different absolute paths with the same definition name are independent.
		resolved, err := filepath.EvalSymlinks(teams)
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Join(resolved, "demo", "team.yaml"))))
		if _, err := os.Stat(filepath.Join(state, "hergy", "team-locks", expected)); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(teams)
		if err != nil || len(entries) != 0 {
			t.Fatalf("lock changed configuration: %v %v", entries, err)
		}
	}
}

func TestTeamStartContinuesWithoutState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "blocked"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "blocked"))
	path := filepath.Join(dir, "demo", "team.yaml")
	release, err := LockTeamStart(path)
	if err != nil {
		t.Fatalf("state error refused a start: %v", err)
	}
	release()
	if _, err := LockTeams(path); err == nil {
		t.Fatal("rename accepted unavailable state")
	}
}
