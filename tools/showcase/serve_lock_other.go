//go:build !unix

// The hold one serve takes on its work directory. The hold is a flock, which
// only the unix systems hergy's tools run on have; elsewhere this file keeps the
// tool building and leaves the hold empty.
package main

import "errors"

// errLocked says another serve holds the work directory.
var errLocked = errors.New("work directory in use")

// serveLock is one process's hold on a work directory.
type serveLock struct{}

// lockWork takes the hold on work, which this system cannot take: no hold is
// better than a tool that does not build.
func lockWork(string) (*serveLock, error) { return &serveLock{}, nil }

// release frees the hold.
func (l *serveLock) release() {}
