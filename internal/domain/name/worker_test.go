package name_test

import (
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
)

func TestWorkerTabs(t *testing.T) {
	team, _ := name.ParseTeamName("a-b")
	worker, _ := name.ParseWorkerName("c-d")
	label := team.WorkerTab(worker)
	if label.String() != "worker:a-b/c-d" {
		t.Fatal(label)
	}
	gotTeam, gotWorker, ok := name.ParseWorkerTab(label.String())
	if !ok || gotTeam != team || gotWorker != worker {
		t.Fatalf("%v %v %v", gotTeam, gotWorker, ok)
	}
	for _, invalid := range []string{"a-b/c-d", "worker:/a", "worker:a/", "worker:a/b/c", "worker:A/b"} {
		if _, _, ok := name.ParseWorkerTab(invalid); ok {
			t.Errorf("accepted %q", invalid)
		}
	}
}
