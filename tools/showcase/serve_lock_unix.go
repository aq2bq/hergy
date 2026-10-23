//go:build unix

// The hold one serve takes on its work directory.
//
// Two serves over one work directory would lay it out twice: the second
// prepare() clears the directory the first is serving from (2026-10-23, the
// owner's running showcase was rewritten under them). A serve takes an
// exclusive flock on a file beside the work directory before it lays anything
// out, so a second serve stops and asks for --work instead. The operating
// system frees the hold when the process ends, even on a crash, so a stale
// hold cannot lock a person out.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// errLocked says another serve holds the work directory.
var errLocked = errors.New("work directory in use")

// serveLock is one process's hold on a work directory. The file stays open for
// the life of the process: closing it, or ending the process, frees the hold.
type serveLock struct {
	file *os.File
}

// lockWork takes the hold on work, or says errLocked when another serve has it.
// The hold is a file beside the work directory (work + ".lock"), so laying the
// directory out does not delete the hold itself.
func lockWork(work string) (*serveLock, error) {
	if err := os.MkdirAll(filepath.Dir(work), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(work+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("%s を確保できません: %w", work, err)
	}
	return &serveLock{file: file}, nil
}

// release frees the hold. The serve keeps it until it has stopped; releasing it
// earlier would let a second serve lay the directory out under the first.
func (l *serveLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}
