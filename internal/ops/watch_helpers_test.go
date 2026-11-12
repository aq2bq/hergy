package ops

import (
	"github.com/aq2bq/hergy/internal/i18n"
	"testing"
)

func inJapanese(t *testing.T) {
	t.Helper()
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
	old := i18n.Language()
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Use(old) })
}
