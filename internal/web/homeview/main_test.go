package homeview

import (
	"fmt"
	"os"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestMain loads the locales, so the screens render the words a person reads
// rather than the keys.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
