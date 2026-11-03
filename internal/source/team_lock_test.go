//go:build darwin || linux

package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTeamLockAcrossProcesses(t *testing.T) {
	if path := os.Getenv("HERGY_TEST_TEAM_LOCK"); path != "" {
		release, err := LockTeams(path)
		if err == nil {
			release()
			os.Exit(3)
		}
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "demo", "team.yaml")
	release, err := LockTeams(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	cmd := exec.Command(os.Args[0], "-test.run=^TestTeamLockAcrossProcesses$")
	cmd.Env = append(os.Environ(), "HERGY_TEST_TEAM_LOCK="+path, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child acquired held lock: %v %s", err, out)
	}
	other, err := LockTeams(filepath.Join(dir, "other", "team.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	other()
	release()
	again, err := LockTeams(path)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
