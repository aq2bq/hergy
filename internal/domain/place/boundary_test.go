package place_test

import (
	"github.com/aq2bq/hergy/internal/domain/place"

	"testing"
)

func TestPublicTextConstructorCanonicalizesPath(t *testing.T) {
	raw := t.TempDir() + "/"
	var seen place.Seen
	if err := seen.UnmarshalText([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	expected, err := place.NewProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if seen.String() != expected.String() {
		t.Fatalf("raw spelling escaped the boundary: %q", seen.String())
	}
}
