package source

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash is the hash a screen carries from a read to a write: the content, hashed
// with SHA-256 and written as hex. The same function hashes the text a screen
// read and the file it is about to write over, so a file that moved under the
// screen is refused by the conditional writer inside ops.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
