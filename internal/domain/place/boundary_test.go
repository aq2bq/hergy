package place_test

import (
	"encoding"
	"encoding/json"

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

// Project is serialized in API response fields (HQProject.Path and role-context
// Project). JSON decoding must rebuild the same canonical checked value.
func TestProjectTextAndJSONBoundaryCanonicalizesPath(t *testing.T) {
	root := t.TempDir()
	raw := root + "/child/.."
	want, err := place.NewProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	var project place.Project
	var text encoding.TextUnmarshaler = &project
	if err := text.UnmarshalText([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if project != want {
		t.Fatalf("text decoded %q want %q", project.String(), want.String())
	}
	var response struct {
		Project place.Project `json:"project"`
	}
	payload, err := json.Marshal(map[string]string{"project": raw})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if response.Project != want {
		t.Fatalf("JSON decoded %q want %q", response.Project.String(), want.String())
	}
}
