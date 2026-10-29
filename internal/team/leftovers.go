package team

import (
	"os/exec" //nolint:depguard // the git commands are fixed and reading a pane's worktree is not a herdr call (d-ddd2 D2)
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Worktree is one git worktree a closed pane stood in, other than the project's
// own root, with what removing it would lose.
type Worktree struct {
	Path   string
	Branch string
	Dirty  bool
	// Ahead is how many commits the worktree's HEAD has that Base does not:
	// main when the repository has it, else the branch's upstream.
	Ahead int
	Base  string
}

// Leftovers is what a team left in git after its panes closed: the worktrees its
// panes stood in, and the project's stash count. hergy only lists them; a person
// or an AI decides what goes (d-778e).
type Leftovers struct {
	Worktrees []Worktree
	Stashes   int
}

// Empty reports whether nothing is left.
func (l Leftovers) Empty() bool { return len(l.Worktrees) == 0 && l.Stashes == 0 }

// GitLeftovers reads the worktrees of the project that the given directories
// stand in (the project's own root is not one of them), and the number of stashes
// the project holds. A directory that is not in a git worktree of this project is
// skipped: another repository a pane happened to stand in is not the team's
// leftover.
func GitLeftovers(projectRoot string, dirs []string) Leftovers {
	root := resolvePath(projectRoot)
	if root == "" {
		return Leftovers{}
	}
	common := gitLine(root, "rev-parse", "--git-common-dir")
	common = resolveAgainst(root, common)
	seen := map[string]bool{}
	out := Leftovers{}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		worktree := resolvePath(gitLine(dir, "rev-parse", "--show-toplevel"))
		if worktree == "" || worktree == root || seen[worktree] {
			continue
		}
		if common == "" || resolveAgainst(worktree, gitLine(worktree, "rev-parse", "--git-common-dir")) != common {
			continue
		}
		seen[worktree] = true
		out.Worktrees = append(out.Worktrees, worktreeOf(worktree))
	}
	sort.Slice(out.Worktrees, func(i, j int) bool { return out.Worktrees[i].Path < out.Worktrees[j].Path })
	out.Stashes = len(gitLines(root, "stash", "list", "--format=%gd"))
	return out
}

// worktreeOf reads one worktree's branch, whether it has uncommitted changes, and
// how far it is ahead of main or its upstream.
func worktreeOf(path string) Worktree {
	worktree := Worktree{Path: path}
	worktree.Branch = gitLine(path, "rev-parse", "--abbrev-ref", "HEAD")
	if worktree.Branch == "HEAD" {
		worktree.Branch = ""
	}
	worktree.Dirty = gitLine(path, "status", "--porcelain") != ""
	switch {
	case gitOK(path, "rev-parse", "--verify", "--quiet", "main"):
		worktree.Base = "main"
	case gitOK(path, "rev-parse", "--verify", "--quiet", "@{upstream}"):
		worktree.Base = gitLine(path, "rev-parse", "--abbrev-ref", "@{upstream}")
	}
	if worktree.Base != "" {
		if n, err := strconv.Atoi(gitLine(path, "rev-list", "--count", "HEAD", "^"+worktree.Base)); err == nil {
			worktree.Ahead = n
		}
	}
	return worktree
}

// gitLine runs one git command in dir and answers its first line of output, or
// empty when the command fails.
func gitLine(dir string, args ...string) string {
	lines := gitLines(dir, args...)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

// gitLines runs one git command in dir and answers its output lines. A command
// that fails answers nothing: a directory git does not know is not a worktree.
func gitLines(dir string, args ...string) []string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec // G204: the git command is fixed; dir is a pane's cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// gitOK reports whether one git revision resolves in dir.
func gitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec // G204: the git command is fixed; dir is a pane's cwd
	return cmd.Run() == nil
}

// resolvePath is a path with symbolic links resolved, or the cleaned path when it
// is not there.
func resolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// resolveAgainst is a git path made absolute: git answers the common directory
// relative to the worktree in the main one, and absolute in a linked worktree.
func resolveAgainst(dir, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return resolvePath(path)
}
