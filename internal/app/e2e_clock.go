//go:build e2e

package app

import (
	"fmt"
	"os"
	"time"
)

// The e2e run's own clock (d-f978, r-f647).
//
// The run's screens are read at one moment, and the fixture's times are counted
// back from that moment, so a run around midnight does not want "22:50 より前"
// where the screen, rightly, writes "10/20 22:50 より前". The Go e2e tests used
// to set serveClock themselves; a binary driven from outside (the Playwright
// walk) has no such handle, so the e2e build reads the starting moment from
// HERGY_E2E_NOW and moves serveClock on from it, the way e2eActivityNow.Add(
// time.Since(began)) did.
//
// Only a binary built with -tags e2e has this file: the published binary reads
// no clock from its environment.
func init() {
	raw := os.Getenv("HERGY_E2E_NOW")
	if raw == "" {
		return
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HERGY_E2E_NOW=%q is not an RFC 3339 moment: %v\n", raw, err)
		os.Exit(2)
	}
	began := time.Now()
	serveClock = func() time.Time { return at.Add(time.Since(began)) }
}
