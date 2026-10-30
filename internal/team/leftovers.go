package team

import (
	"os"
	"os/exec" //nolint:depguard // the git commands are fixed and reading a worktree is not a herdr call (d-ddd2 D2)
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Worktree is one git worktree of the project, other than the project's own
// root, with what removing it would lose.
type Worktree struct {
	Path   string
	Branch string
	Dirty  bool
	// Ahead is how many commits the worktree's HEAD has that Base does not:
	// main when the repository has it, else the branch's upstream.
	Ahead int
	Base  string
	// PaneCwd is where a pane hergy closed stood. hergy reads the panes before
	// they close, so a worktree an agent worked in by cd-ing to it is still
	// found after its pane's own directory was the project root (d-970c).
	PaneCwd bool
}

// Leftovers is what a team left in git after its panes closed: the worktrees
// that came while it was running, and the project's stash count. hergy only
// lists them; a person or an AI decides what goes (d-778e).
type Leftovers struct {
	Worktrees []Worktree
	Stashes   int
}

// Empty reports whether nothing is left.
func (l Leftovers) Empty() bool { return len(l.Worktrees) == 0 && l.Stashes == 0 }

// GitLeftovers reads the worktrees of the project that came at or after since
// (every one of them when since is the zero time), other than the project's own
// root, and the number of stashes the project holds.
//
// since is when the team was started, or when the worker was hired, read from
// the launch record. hergy cannot tell which worktree a team made — an agent
// keeps its pane in the project root and cds into a worktree for each command —
// so it says when the worktree came instead of whose it is (d-970c).
//
// dirs are where the panes stood, read before they closed: a worktree one of
// them stood in comes back with PaneCwd, and one whose making time hergy cannot
// read is listed only when a pane stood in it. A directory that is not in a git
// worktree of this project is skipped: another repository a pane happened to
// stand in is not the team's leftover.
func GitLeftovers(projectRoot string, since time.Time, dirs []string) Leftovers {
	root := resolvePath(projectRoot)
	if root == "" {
		return Leftovers{}
	}
	common := resolveAgainst(root, gitLine(root, "rev-parse", "--git-common-dir"))
	panes := paneWorktrees(root, common, dirs)
	admins := worktreeAdmins(common)
	out := Leftovers{}
	for _, path := range gitWorktrees(root) {
		made, known := worktreeMadeAt(admins[path])
		switch {
		case !since.IsZero() && known && made.Before(since):
			// It was there before the team started, so it is not what the team
			// left behind.
			continue
		case !since.IsZero() && !known && !panes[path]:
			// hergy cannot say when it was made and no pane stood in it.
			continue
		}
		worktree := worktreeOf(path)
		worktree.PaneCwd = panes[path]
		out.Worktrees = append(out.Worktrees, worktree)
	}
	sort.Slice(out.Worktrees, func(i, j int) bool { return out.Worktrees[i].Path < out.Worktrees[j].Path })
	out.Stashes = len(gitLines(root, "stash", "list", "--format=%gd"))
	return out
}

// paneWorktrees is the worktrees of this project that the given directories
// stand in, by resolved path, other than the project's own root.
func paneWorktrees(root, common string, dirs []string) map[string]bool {
	out := map[string]bool{}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		worktree := resolvePath(gitLine(dir, "rev-parse", "--show-toplevel"))
		if worktree == "" || worktree == root {
			continue
		}
		if common == "" || resolveAgainst(worktree, gitLine(worktree, "rev-parse", "--git-common-dir")) != common {
			continue
		}
		out[worktree] = true
	}
	return out
}

// gitWorktrees is every worktree git lists for the project, other than the
// project's own root, by resolved path. A worktree whose directory is gone is
// left out: it is not a place to remove, and the branch and changes a person
// would check before removing it cannot be read there.
func gitWorktrees(root string) []string {
	var out []string
	for _, line := range gitLines(root, "worktree", "list", "--porcelain") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		path = resolvePath(path)
		if path == "" || path == root {
			continue
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		out = append(out, path)
	}
	return out
}

// worktreeAdmins maps a worktree of the project to the directory git keeps for
// it, <common>/worktrees/<id>. git writes the worktree's own path into that
// directory's gitdir file, so the mapping is read from git's files rather than
// guessed from the worktree's name.
func worktreeAdmins(common string) map[string]string {
	out := map[string]string{}
	if common == "" {
		return out
	}
	entries, err := os.ReadDir(filepath.Join(common, "worktrees"))
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		admin := filepath.Join(common, "worktrees", entry.Name())
		text, err := os.ReadFile(filepath.Join(admin, "gitdir"))
		if err != nil {
			continue
		}
		// The file holds "<worktree>/.git", which is the worktree's own path.
		path := strings.TrimSuffix(strings.TrimSpace(string(text)), "/.git")
		if path == "" {
			continue
		}
		out[resolvePath(path)] = admin
	}
	return out
}

// worktreeMadeAt is when git made a worktree, read from the mtime of the admin
// directory's gitdir file.
//
// git writes gitdir once, when it makes the worktree. Ordinary work in the
// worktree does not rewrite it: a commit moves HEAD, a checkout rewrites HEAD,
// and every command that reads the worktree rewrites index, so HEAD and index
// date the last work rather than the making. The directory itself is touched
// whenever a lock file comes or goes.
//
// A stat's mtime reads the same on macOS and on Linux. A creation (birth) time
// does not: APFS keeps one, and Go answers it on macOS alone, because the
// syscall package has no birth time on Linux and not every Linux filesystem
// keeps one. Moving a worktree or repairing it rewrites gitdir, which dates that
// move: the worktree is still one the team used, so a move after the start keeps
// it in the list.
func worktreeMadeAt(admin string) (time.Time, bool) {
	if admin == "" {
		return time.Time{}, false
	}
	info, err := os.Stat(filepath.Join(admin, "gitdir"))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
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
