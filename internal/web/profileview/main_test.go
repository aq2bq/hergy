package profileview

import (
	"fmt"
	"os"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestMain loads the locales, so the screen renders the words a person reads and
// the tests can say which words they expect.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
