package screenlog

import (
	"testing"
	"time"
)

// TestHashIsLowercaseHexOfTheScreen: the hash is stable and depends on the
// screen, so the same screen written twice reads back the same.
func TestHashIsLowercaseHexOfTheScreen(t *testing.T) {
	first := Hash("hello")
	if first != Hash("hello") {
		t.Fatal("Hash is not stable for the same screen")
	}
	if first == Hash("hello ") {
		t.Fatal("Hash does not depend on the screen")
	}
	if len(first) != 64 {
		t.Fatalf("Hash = %q, want 64 hex characters", first)
	}
	for _, r := range first {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("Hash = %q, want lowercase hex", first)
		}
	}
}

// TestReadOfAnEmptyRecordIsEmpty: no status has run yet, so the record reads as
// an empty map rather than an error.
func TestReadOfAnEmptyRecordIsEmpty(t *testing.T) {
	log := At(t.TempDir())
	entries, err := log.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Read of a missing record = %v, want empty", entries)
	}
}

// TestWriteThenReadRoundTrips: what was written is what is read back, keyed by
// member name.
func TestWriteThenReadRoundTrips(t *testing.T) {
	log := At(t.TempDir())
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	want := map[string]Entry{
		"demo-lead":    {Hash: Hash("screen"), At: at},
		"demo-builder": {Hash: Hash("other"), At: at.Add(time.Minute)},
	}
	if err := log.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := log.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Read = %v, want %v", got, want)
	}
	for name, entry := range want {
		if got[name] != entry {
			t.Fatalf("Read[%q] = %+v, want %+v", name, got[name], entry)
		}
	}
}

// TestWriteReplacesTheWholeRecord: a member that is not in the new map is gone,
// so the record does not grow without bound.
func TestWriteReplacesTheWholeRecord(t *testing.T) {
	log := At(t.TempDir())
	if err := log.Write(map[string]Entry{"a": {Hash: Hash("a")}}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := log.Write(map[string]Entry{"b": {Hash: Hash("b")}}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := log.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if _, ok := got["a"]; ok {
		t.Fatalf("Write kept a member the new record dropped: %v", got)
	}
	if _, ok := got["b"]; !ok {
		t.Fatalf("Write dropped the member it was handed: %v", got)
	}
}

// TestDefaultDirFollowsXDGStateHome: the record is state, so it sits under
// $XDG_STATE_HOME/hergy, next to the launch record.
func TestDefaultDirFollowsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/hergy-state")
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if want := "/tmp/hergy-state/hergy"; dir != want {
		t.Fatalf("DefaultDir = %q, want %q", dir, want)
	}
}
