//go:build darwin || linux

package source

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

var errTeamLockBusy = errors.New("team definition is locked")

// LockTeams excludes renames from other renames and cooperating starts. Lock
// files stay in state: removing a locked inode would permit a second lock for
// the same path. Closing releases a lock, including when its process exits.
func LockTeams(paths ...string) (func(), error) {
	return lockTeams(syscall.LOCK_EX, paths...)
}

// LockTeamStart allows concurrent starts. Only a held rename lock refuses a
// start; unsupported locking or unavailable state must not prevent booting.
func LockTeamStart(path string) (func(), error) {
	release, err := lockTeams(syscall.LOCK_SH, path)
	if err != nil && !errors.Is(err, errTeamLockBusy) {
		return func() {}, nil
	}
	return release, err
}

func lockTeams(mode int, paths ...string) (func(), error) {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	var held []*os.File
	release := func() {
		for _, file := range held {
			_ = file.Close()
		}
	}
	for _, path := range slices.Compact(paths) {
		file, err := teamLockFile(path)
		if err != nil {
			release()
			return nil, err
		}
		if err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB); err != nil {
			_ = file.Close()
			release()
			if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
				err = fmt.Errorf("%w: %w", errTeamLockBusy, err)
			}
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		held = append(held, file)
	}
	return release, nil
}

func teamLockFile(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// Resolve the teams directory, which exists for both old and new names.
	// /var and /private/var must not produce different locks on macOS.
	parentPath := filepath.Dir(filepath.Dir(absolute))
	_, err = filepath.EvalSymlinks(parentPath)
	if err != nil {
		return nil, err
	}
	parent, err := domainplace.NewProject(parentPath)
	if err != nil {
		return nil, err
	}
	absolute = filepath.Join(parent.String(), filepath.Base(filepath.Dir(absolute)), filepath.Base(absolute))
	state := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(state) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		state = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(state, "hergy", "team-locks")
	//nolint:gosec // G703: XDG_STATE_HOME or the user's home selects the state directory, not a requested team path.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%x", sha256.Sum256([]byte(absolute)))
	//nolint:gosec // G703: the state directory is configured by the user; the filename is a hash, not input text.
	return os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0o600)
}
