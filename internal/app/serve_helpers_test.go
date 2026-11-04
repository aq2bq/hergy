package app

import (
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// initLocales loads the locales for the tests that call the pieces of a run
// rather than Run itself: a team's built-in prompt template comes from the
// locale, so without it the members read no document and count no rule.
func initLocales(t *testing.T) {
	t.Helper()
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
}
