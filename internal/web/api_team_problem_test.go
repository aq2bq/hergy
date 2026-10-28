package web

import (
	"testing"

	"github.com/aq2bq/hergy/internal/check"
)

// TestFigureProblemsNameThePlaceAndTheField: a figure finding says where it
// stands (at, the place in team.yaml) and which field it belongs beside (field),
// the way a role document's problems do: the same value is not written twice
// (n-3f6c).
func TestFigureProblemsNameThePlaceAndTheField(t *testing.T) {
	problems := t2FigureProblems([]check.Finding{
		{Field: "members[2].role", Text: "role?", Stops: true},
		{Field: "members[2]", Text: "member?"},
		{Field: "mission", Text: "mission?"},
		{Field: "workers[0]", Text: "worker?"},
		{Field: "node:lead", Text: "node?"},
		{Field: "rule:demo/agreement.yaml#C1", Text: "rule?"},
	}, "demo")
	want := []struct{ at, field string }{
		{"members[2].role", "role"},
		{"members[2]", "members"},
		{"mission", "mission"},
		{"workers[0]", "workers"},
		{"node:lead", "node:lead"},
		{"rule:demo/agreement.yaml#C1", "rule:demo/agreement.yaml#C1"},
	}
	if len(problems) != len(want) {
		t.Fatalf("problems = %+v, want %d", problems, len(want))
	}
	for i := range want {
		if problems[i].At != want[i].at || problems[i].Field != want[i].field {
			t.Errorf("problem %d = at %q field %q, want at %q field %q",
				i, problems[i].At, problems[i].Field, want[i].at, want[i].field)
		}
	}
}
