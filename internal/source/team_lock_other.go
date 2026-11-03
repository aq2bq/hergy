//go:build !darwin && !linux

package source

import "errors"

// LockTeams fails closed on platforms without the supported file-lock port.
func LockTeams(_ ...string) (func(), error) {
	return nil, errors.New("team file locking is supported on macOS and Linux")
}

// RenameDirectory refuses platforms without an exclusive directory move.
func RenameDirectory(_, _ string) error {
	return errors.New("exclusive directory rename is supported on macOS and Linux")
}
