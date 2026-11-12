package cli

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	// activityNow is the clock the tests read the screen at: a fixed moment, so
	// every length and every time is one a test can write down.
	// inJapanese reads the screen in Japanese: the words this screen writes are the
	// ones the fixture and the spec spell out (3 V1). The language is put back, so a
	// test in this package that reads something else is not left reading Japanese.
	// useJapanese loads the locales and reads in Japanese from here on. Every command
	// picks the language again when it runs (app.go), so a test that runs one asks
	// for Japanese again afterwards.
	// calls is every call a fake was given, and empty when the fake was never called
	// (a script that is never run writes no log).
)

func useJapanese(t *testing.T) {
	t.Helper()
	initLocales(t)
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
}
func calls(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}
