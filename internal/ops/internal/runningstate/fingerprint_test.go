package runningstate

import "testing"

// TestFingerprintsTellAStartAndAnEnv: the start fingerprint follows the kind and
// args and nothing else, and the env fingerprint follows the keys and values in
// any order, so a profile change is seen and an env change is a different
// question (d-36ef).
func TestFingerprintsTellAStartAndAnEnv(t *testing.T) {
	if StartFingerprint("claude", []string{"--model", "opus"}) == StartFingerprint("claude", []string{"--model", "sonnet"}) {
		t.Error("two starts hashed alike")
	}
	if StartFingerprint("claude", nil) == StartFingerprint("codex", nil) {
		t.Error("two kinds hashed alike")
	}
	one := EnvFingerprint(map[string]string{"A": "1", "B": "2"})
	if one != EnvFingerprint(map[string]string{"B": "2", "A": "1"}) {
		t.Error("the same environment hashed two ways")
	}
	if one == EnvFingerprint(map[string]string{"A": "1", "B": "3"}) {
		t.Error("a changed value hashed like the value it changed from")
	}
	if one == StartFingerprint("claude", nil) {
		t.Error("an environment hashed like a start")
	}
}

// Existing state files must keep comparing equal after the function moves.
func TestFingerprintEncodingRemainsStable(t *testing.T) {
	if got := StartFingerprint("claude", nil); got != "b8306bdea4eae62d30fa9e6bf7fa3daf059a0bf15f87abc81f8bc5ca9b4c4d8f" {
		t.Fatalf("start encoding changed: %s", got)
	}
	if got := EnvFingerprint(nil); got != "74d1a7d4427536309b77a92b31f1f2a683674475974b35faf122f1f07e4a3039" {
		t.Fatalf("env encoding changed: %s", got)
	}
}
