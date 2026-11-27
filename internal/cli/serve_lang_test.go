package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestServeKeepsTheHandedLanguageAndAnswersInEnglish is d-8e53: serve no
// longer picks a language of its own. The screens' pages keep their Japanese
// labels, the sentences the server answers them with are English, and the
// language chosen for the text handed to agents is the one the settings chose.
func TestServeKeepsTheHandedLanguageAndAnswersInEnglish(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
	url, stop := startServe(t, f)
	defer stop()
	if got := i18n.HandedLanguage(); got != "ja" {
		t.Errorf("serve changed the handed language to %q, want ja", got)
	}
	resp, err := http.Get(url + "/teams")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "チーム定義") {
		t.Errorf("the teams page lost its Japanese labels:\n%s", body)
	}
	if got, want := i18n.T("web.activity.herdr_unavailable"), string(i18n.Text("en", "web.activity.herdr_unavailable")); got != want {
		t.Errorf("a sentence the screens are answered with reads %q, want the English %q", got, want)
	}
}
