package app

import (
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/web" // waitLaunch waits for one launch to end, as the screens' polling does, and
	// answers its last state.
)

func waitLaunch(t *testing.T, read *activityRead, id string) web.HQLaunch {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		launch, ok := read.LaunchState(id)
		if !ok {
			t.Fatalf("launch %s is not there", id)
		}
		if launch.Phase != "starting" && launch.Phase != "awaiting" {
			return launch
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("launch %s did not end: %+v", id, launch)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
