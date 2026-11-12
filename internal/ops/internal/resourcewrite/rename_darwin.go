package resourcewrite

import "golang.org/x/sys/unix"

// RenameDirectory moves a definition without replacing a concurrently created
// destination, including an empty directory.
func RenameDirectory(from, to string) error { return unix.RenamexNp(from, to, unix.RENAME_EXCL) }
