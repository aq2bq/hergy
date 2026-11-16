package turns

import (
	"path/filepath"
	"testing"
)

func TestLookupDistinguishesMissingStateFromHumanHolder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "demo.json")
	if st, err := Lookup(path); err != nil || st != nil {
		t.Fatalf("missing state = %+v, %v", st, err)
	}
	if err := Save(path, &State{Team: fixtureSeen("demo"), Holder: Human}); err != nil {
		t.Fatal(err)
	}
	if st, err := Lookup(path); err != nil || st == nil || st.Holder != Human {
		t.Fatalf("human state = %+v, %v", st, err)
	}
}
