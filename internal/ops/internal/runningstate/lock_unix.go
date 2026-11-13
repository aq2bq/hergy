//go:build darwin || linux

package runningstate

import (
	"os"
	"path/filepath"
	"syscall"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
)

// The lock inode survives End and Begin. Locking the JSON inode would lose the
// lock on rename; unlinking a lock file would let a second writer pass it.
func (s Store) locked(name domainname.TeamName, change func() error) error {
	if _, err := domainname.ParseTeamName(name.String()); err != nil {
		return err
	}
	dir := filepath.Join(s.dir, ".locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, name.String()), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	return change()
}
