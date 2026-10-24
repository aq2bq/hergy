package app

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails when a test leaves a goroutine behind. hergy serve starts a
// team in the background (the headquarters' start), and a test that goes away
// must not leave that goroutine running (d-b28a).
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
