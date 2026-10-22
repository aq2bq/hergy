//go:build e2e

package app

import (
	"fmt"
	"os"
	"time"
)

// The browser walks drive the serve a person runs — a binary of its own, not an
// in-process server — so the moment its screens are read at has to reach it from
// outside. Only the binary built with -tags e2e reads it (d-f978): HERGY_E2E_NOW
// is the moment the run begins, as RFC 3339, and the clock goes on from there the
// way the Go walks' own clock did (e2eActivityNow.Add(time.Since(began))). The
// published binary has neither the variable nor this reading.
//
// The zone is the run's own, through TZ (r-f647): a moment that is not today's is
// written with its date in that zone, so what a screen says does not depend on
// where the run happens.
func init() {
	raw := os.Getenv("HERGY_E2E_NOW")
	if raw == "" {
		return
	}
	began, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(fmt.Sprintf("HERGY_E2E_NOW=%q is not an RFC 3339 moment: %v", raw, err))
	}
	start := time.Now()
	serveClock = func() time.Time { return began.Add(time.Since(start)) }
}
