package resourcewrite

import "golang.org/x/sys/unix"

// RenameDirectory moves a definition only when its destination does not exist.
func RenameDirectory(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}
