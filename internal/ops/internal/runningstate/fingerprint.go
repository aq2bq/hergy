package runningstate

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// StartFingerprint is the hash of how a member is started: the kind and the
// profile's args, which are the command hergy starts the agent with. The env is
// not part of it: env is fixed when the pane is made, so a change to it is a
// different question (EnvFingerprint). A person reads this fingerprint only as
// "changed", never as a command.
func StartFingerprint(kind string, args []string) string {
	var b strings.Builder
	field(&b, "kind", kind)
	field(&b, "args", strconv.Itoa(len(args)))
	for _, arg := range args {
		field(&b, "arg", arg)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// EnvFingerprint is the hash of one member's environment: the keys, in order, and
// their values. The values are folded in here and written nowhere else, so a
// change to one can be seen without the record holding a secret.
func EnvFingerprint(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	field(&b, "env", strconv.Itoa(len(keys)))
	for _, key := range keys {
		field(&b, "env key", key)
		field(&b, "env value", env[key])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// field writes one labelled field: its name, its length, then its bytes. The
// length is what keeps two fields from running into each other, so a member
// whose prompt is "ab" and whose kind is "c" cannot hash like one whose prompt
// is "a" and whose kind is "bc".
func field(b *strings.Builder, name, value string) {
	b.WriteString(name)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(len(value)))
	b.WriteByte(':')
	b.WriteString(value)
}
