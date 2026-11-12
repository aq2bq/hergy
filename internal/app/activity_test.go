package app

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

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

var activityNow = time.Date(2026, 10, 21, 13, 52, 0, 0, time.Local)

func inJapanese(t *testing.T) {
	t.Helper()
	useJapanese(t)
	t.Cleanup(func() {
		if err := i18n.Use(i18n.Fallback); err != nil {
			t.Error(err)
		}
	})
}

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

// newActivityFixture writes the project and both fakes, and the running teams'
// records, and points the reader at them.

// activityHerdr writes a fake herdr over this test's own answers, and returns the
// file it records every call in.

// activityGy writes a fake gy over this test's own answers, and returns the file
// it records every call in.

// activityAnswers writes what the two fakes answer: herdr's tabs and agents,
// core-lead's pane, and gy's questions.

// writeHerdr writes one file under the fake herdr's own directory.

// stateFile writes the watcher's memory (6 A2), so a serve that is started while
// the agents are already stopped knows when they stopped.

// reader builds the reading the server would be handed, at the fixed clock.

// entry is one watcher entry as the state file writes it.

// statusError is what hergy status says about one running team's definition, for
// the tests that check the two agree on why a definition does not read (6 A1).
// It is read in Japanese, the language the screen is read in, because the
// sentence is the thing being compared.

// nodeNamed is one node of an opened team by its short name.
