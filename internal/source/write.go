package source

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Hash is the hash a screen carries from a read to a write: the content, hashed
// with SHA-256 and written as hex. The same function hashes the text a screen
// read and the file it is about to write over, so a file that moved under the
// screen is refused (WriteIfUnchanged).
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ErrChanged says a file is not the one that was read: its content hashes to
// something other than the hash the caller carried. A caller turns it into the
// sentence a person reads, because that sentence names the file the way its own
// screen does.
var ErrChanged = errors.New("the file is not the one that was read")

// WriteIfUnchanged writes text to path, but only when the file it finds hashes
// to prevHash: a file that moved under the screen since it was read is not
// written over.
//
// A file that is not there yet is being made (a layer whose profiles.yaml does
// not exist gets one on the first save, r-daf8): it is written when prevHash is
// the hash of the empty text. A file that is not there and was read as something
// else is not this function's to name, so the read's own error is returned and
// the caller says what it means.
func WriteIfUnchanged(path, prevHash, text string) error {
	before, err := os.ReadFile(path)
	switch {
	case err == nil && Hash(string(before)) != prevHash:
		return ErrChanged
	case err != nil && errors.Is(err, fs.ErrNotExist) && prevHash == Hash(""):
		// Not there yet, and the caller read nothing: it is being made.
	case err != nil:
		return err
	}
	return WriteBeside(path, text)
}

// WriteBeside writes text to a file beside the target and renames it over, so a
// reader sees either the old file or the new one, never half of either.
//
// The target may be a link: a dotfiles setup keeps the files under
// ~/.config/hergy as links, so the link is followed and the file it points at is
// what is written. The link stays a link, and the mode stays the one the file
// already had rather than a fresh 0644.
//
// A target that is not there yet is written at its own path: this is how a layer
// whose profiles.yaml does not exist gets one (r-daf8). The directory must be
// there; the caller checks that where it matters.
func WriteBeside(path, text string) error {
	target, err := filepath.EvalSymlinks(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		target = path
	case err != nil:
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	// A temp file that does not reach the rename is removed. A removal that
	// fails has nothing to say: the target file is untouched either way.
	defer func() {
		if name != "" {
			_ = os.Remove(name)
		}
	}()
	if _, err := temp.WriteString(text); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if err := os.Rename(name, target); err != nil {
		return err
	}
	name = "" // renamed: there is nothing left to remove
	return nil
}
