package name_test

import (
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
)

func TestPublicTextConstructorsCheckSpelling(t *testing.T) {
	for _, raw := range []string{"Upper", "worker:demo/a", "bad/name"} {
		var seen name.Seen
		if err := seen.UnmarshalText([]byte(raw)); err == nil {
			t.Fatalf("unchecked observation: %q", raw)
		}
		var team name.TeamRef
		if err := team.UnmarshalText([]byte(raw)); err == nil {
			t.Fatalf("unchecked team: %q", raw)
		}
	}
	var seen name.Seen
	if err := seen.UnmarshalText([]byte("demo-lead")); err != nil || seen.String() != "demo-lead" {
		t.Fatalf("checked name: %v %s", err, seen)
	}
}
