package profile

import (
	"testing"

	"github.com/aq2bq/hergy/internal/scaffold"
)

// TestKindsIsTheChoiceAScreenOffers: the kinds hergy can start, each written
// once, every one a name a profile may hold, and the one hergy's own starter
// profile names among them (C12).
func TestKindsIsTheChoiceAScreenOffers(t *testing.T) {
	kinds := Kinds()
	if len(kinds) == 0 {
		t.Fatal("no kinds are offered")
	}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if seen[kind] {
			t.Errorf("%q is offered twice", kind)
		}
		seen[kind] = true
		if !kindPattern.MatchString(kind) {
			t.Errorf("%q is not a name a profile may hold", kind)
		}
	}
	if !seen[scaffold.ProfileKind] {
		t.Errorf("the kind hergy's own starter profile names (%q) is not offered", scaffold.ProfileKind)
	}
}

// TestKindsHandsOutACopy: a caller that sorts or edits the list it is given does
// not change what the next caller offers.
func TestKindsHandsOutACopy(t *testing.T) {
	first := Kinds()
	if len(first) == 0 {
		t.Fatal("no kinds are offered")
	}
	first[0] = "changed"
	if Kinds()[0] == "changed" {
		t.Error("Kinds hands out the list itself")
	}
}
