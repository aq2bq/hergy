package app

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestServeMakesTheWebUIJapaneseWhateverLangSays: the screens are written in
// Japanese, so serve makes their pages in Japanese even when --lang says en, and
// the command line's language comes back when serve stops (r-85b3).
func TestServeMakesTheWebUIJapaneseWhateverLangSays(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	if err := i18n.Use("en"); err != nil {
		t.Fatal(err)
	}
	url, stop := startServe(t, f)
	if got := i18n.Language(); got != screenLanguage {
		t.Errorf("serve runs the screens in %q, want %q", got, screenLanguage)
	}
	resp, err := http.Get(url + "/teams")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "チーム定義") {
		t.Errorf("the teams page is not Japanese:\n%s", body)
	}
	stop()
	if got := i18n.Language(); got != "en" {
		t.Errorf("the command line's language after serve is %q, want en", got)
	}
}
