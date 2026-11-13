package team

import (
	"os"
	"os/exec" //nolint:depguard // the git commands are fixed and reading a worktree is not a herdr call (d-ddd2 D2)
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// Worktree is one git worktree of the project, other than the project's own
// root, with what removing it would lose.
type Worktree struct {
	Path   domainplace.Project
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
// the current run state. hergy cannot tell which worktree a team made — an agent
// keeps its pane in the project root and cds into a worktree for each command —
// so it says when the worktree came instead of whose it is (d-970c).
//
// dirs are where the panes stood, read before they closed: a worktree one of
// them stood in comes back with PaneCwd, and one whose making time hergy cannot
// read is listed only when a pane stood in it. A directory that is not in a git
// worktree of this project is skipped: another repository a pane happened to
// stand in is not the team's leftover.
func GitLeftovers(projectRoot domainplace.Project, since time.Time, dirs []domainplace.Seen) Leftovers {
	root := projectRoot
	if root.IsZero() {
		return Leftovers{}
	}
	common := gitCommon(root)
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
	sort.Slice(out.Worktrees, func(i, j int) bool { return out.Worktrees[i].Path.String() < out.Worktrees[j].Path.String() })
	out.Stashes = len(gitLines(root.String(), "stash", "list", "--format=%gd"))
	return out
}

// paneWorktrees is the worktrees of this project that the given directories
// stand in, by resolved path, other than the project's own root.
func paneWorktrees(root, common domainplace.Project, dirs []domainplace.Seen) map[domainplace.Project]bool {
	out := map[domainplace.Project]bool{}
	for _, dir := range dirs {
		if strings.TrimSpace(dir.String()) == "" {
			continue
		}
		worktree, _ := domainplace.NewProject(gitLine(dir.String(), "rev-parse", "--show-toplevel"))
		if worktree.IsZero() || worktree == root {
			continue
		}
		if common.IsZero() || gitCommon(worktree) != common {
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
func gitWorktrees(root domainplace.Project) []domainplace.Project {
	var out []domainplace.Project
	for _, line := range gitLines(root.String(), "worktree", "list", "--porcelain") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		project, _ := domainplace.NewProject(strings.TrimSpace(path))
		if project.IsZero() || project == root {
			continue
		}
		if info, err := os.Stat(project.String()); err != nil || !info.IsDir() {
			continue
		}
		out = append(out, project)
	}
	return out
}

// worktreeAdmins maps a worktree of the project to the directory git keeps for
// it, <common>/worktrees/<id>. git writes the worktree's own path into that
// directory's gitdir file, so the mapping is read from git's files rather than
// guessed from the worktree's name.
func worktreeAdmins(common domainplace.Project) map[domainplace.Project]string {
	out := map[domainplace.Project]string{}
	if common.IsZero() {
		return out
	}
	entries, err := os.ReadDir(filepath.Join(common.String(), "worktrees"))
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		admin := filepath.Join(common.String(), "worktrees", entry.Name())
		text, err := os.ReadFile(filepath.Join(admin, "gitdir"))
		if err != nil {
			continue
		}
		// The file holds "<worktree>/.git", which is the worktree's own path.
		path := strings.TrimSuffix(strings.TrimSpace(string(text)), "/.git")
		if path == "" {
			continue
		}
		project, _ := domainplace.NewProject(strings.TrimSpace(path))
		out[project] = admin
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
func worktreeOf(path domainplace.Project) Worktree {
	worktree := Worktree{Path: path}
	worktree.Branch = gitLine(path.String(), "rev-parse", "--abbrev-ref", "HEAD")
	if worktree.Branch == "HEAD" {
		worktree.Branch = ""
	}
	worktree.Dirty = gitLine(path.String(), "status", "--porcelain") != ""
	switch {
	case gitOK(path.String(), "rev-parse", "--verify", "--quiet", "main"):
		worktree.Base = "main"
	case gitOK(path.String(), "rev-parse", "--verify", "--quiet", "@{upstream}"):
		worktree.Base = gitLine(path.String(), "rev-parse", "--abbrev-ref", "@{upstream}")
	}
	if worktree.Base != "" {
		if n, err := strconv.Atoi(gitLine(path.String(), "rev-list", "--count", "HEAD", "^"+worktree.Base)); err == nil {
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

// gitCommon reads git's directory relative to the worktree at the git boundary.
func gitCommon(worktree domainplace.Project) domainplace.Project {
	raw := gitLine(worktree.String(), "rev-parse", "--git-common-dir")
	if raw == "" {
		return domainplace.Project{}
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(worktree.String(), raw)
	}
	project, _ := domainplace.NewProject(strings.TrimSpace(raw))
	return project
}
