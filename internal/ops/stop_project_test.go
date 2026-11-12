package ops

import (
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

func TestStopProjectUsesRunProjectAndFallsBackOnlyWhenUnknown(t *testing.T) {
	team, err := domainplace.NewProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	caller, err := domainplace.NewProject(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := stopProject(team, caller); got != team {
		t.Fatalf("known run: %s", got)
	}
	if got := stopProject(domainplace.Project{}, caller); got != caller {
		t.Fatalf("unknown run: %s", got)
	}
}
