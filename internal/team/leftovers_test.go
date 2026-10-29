package team

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeLeftover(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGitLeftoversReadsThePanesWorktrees: a pane that stood in one of the
// project's worktrees leaves that worktree in the list, with its branch,
// uncommitted changes and commits not in main; the project's own root is not a
// leftover, another repository's worktree is not one, and the project's stashes
// are counted (r-b65e, d-778e).
func TestGitLeftoversReadsThePanesWorktrees(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "test")
	writeLeftover(t, filepath.Join(root, "base.txt"), "base\n")
	gitRun(t, root, "add", "base.txt")
	gitRun(t, root, "commit", "-q", "-m", "base")

	// A worktree on its own branch, one commit ahead of main, with an uncommitted
	// file: both are what removing it would lose.
	worktree := filepath.Join(t.TempDir(), "lane-1")
	gitRun(t, root, "worktree", "add", "-q", "-b", "lane-1", worktree, "main")
	writeLeftover(t, filepath.Join(worktree, "lane.txt"), "lane\n")
	gitRun(t, worktree, "add", "lane.txt")
	gitRun(t, worktree, "commit", "-q", "-m", "lane")
	writeLeftover(t, filepath.Join(worktree, "dirty.txt"), "dirty\n")

	// The project's stash.
	writeLeftover(t, filepath.Join(root, "stashed.txt"), "stash\n")
	gitRun(t, root, "add", "stashed.txt")
	gitRun(t, root, "stash", "push", "-q", "-m", "left for later")

	// Another repository a pane happened to stand in is not this project's.
	other := t.TempDir()
	gitRun(t, other, "init", "-q")
	gitRun(t, other, "config", "user.email", "test@example.com")
	gitRun(t, other, "config", "user.name", "test")
	writeLeftover(t, filepath.Join(other, "other.txt"), "other\n")
	gitRun(t, other, "add", "other.txt")
	gitRun(t, other, "commit", "-q", "-m", "other")

	left := GitLeftovers(root, []string{root, worktree, other, filepath.Join(worktree, "..")})
	if len(left.Worktrees) != 1 {
		t.Fatalf("leftovers = %+v, want the one worktree", left)
	}
	one := left.Worktrees[0]
	if resolvePath(one.Path) != resolvePath(worktree) {
		t.Errorf("worktree path = %q, want %q", one.Path, worktree)
	}
	if one.Branch != "lane-1" || !one.Dirty || one.Ahead != 1 || one.Base != "main" {
		t.Errorf("worktree = %+v, want lane-1, dirty, 1 ahead of main", one)
	}
	if left.Stashes != 1 {
		t.Errorf("stashes = %d, want the one the project holds", left.Stashes)
	}
}

// TestGitLeftoversIsEmptyForTheProjectRoot: a team that worked in the project
// itself leaves no worktree, so down says nothing about one.
func TestGitLeftoversIsEmptyForTheProjectRoot(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "test")
	writeLeftover(t, filepath.Join(root, "only.txt"), "only\n")
	gitRun(t, root, "add", "only.txt")
	gitRun(t, root, "commit", "-q", "-m", "only")

	if left := GitLeftovers(root, []string{root, filepath.Join(root, "sub")}); !left.Empty() {
		t.Errorf("leftovers = %+v, want none", left)
	}
}
