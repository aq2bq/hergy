package web

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/goleak"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestMain loads the locales, so the screens render the words a person reads,
// and then fails when a test leaves a goroutine behind: the server starts one
// for every connection, and a test that stops the server must not leave one
// running (d-b28a).
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	goleak.VerifyTestMain(m)
}
