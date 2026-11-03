//go:build darwin || linux

package source

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// LockTeams excludes cooperating starts and renames across processes. The lock
// files stay in the teams directory: removing a locked inode would let another
// process acquire a different inode for the same name. Closing releases a lock,
// including when its process exits. Each path is a team.yaml, before or after a
// rename; unrelated definitions do not exclude one another.
func LockTeams(paths ...string) (func(), error) {
	paths = slices.Clone(paths)
	slices.Sort(paths)
	var held []*os.File
	release := func() {
		for _, file := range held {
			_ = file.Close()
		}
	}
	for _, path := range slices.Compact(paths) {
		name := fmt.Sprintf(".lock-%x", sha256.Sum256([]byte(filepath.Base(filepath.Dir(path)))))
		//nolint:gosec // G703: the directory is the caller's catalog path; the lock filename is a hash, not input text.
		file, err := os.OpenFile(filepath.Join(filepath.Dir(filepath.Dir(path)), name), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			release()
			return nil, err
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = file.Close()
			release()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		held = append(held, file)
	}
	return release, nil
}
