package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
)

func TestMain(m *testing.M) {
	if err := Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func catalog(t *testing.T, lang string) map[string]string {
	t.Helper()
	raw, err := files.ReadFile("locales/" + lang + ".json")
	if err != nil {
		t.Fatalf("cannot read locale %s: %v", lang, err)
	}
	messages := map[string]string{}
	if err := json.Unmarshal(raw, &messages); err != nil {
		t.Fatalf("locale %s is not valid JSON: %v", lang, err)
	}
	return messages
}

func TestEachTranslationHasAnEnglishMessage(t *testing.T) {
	base := Available[0]
	reference := catalog(t, base)
	for _, lang := range Available[1:] {
		got := catalog(t, lang)
		var extra []string
		for key := range got {
			if _, ok := reference[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Errorf("keys in %s not in %s: %v", lang, base, extra)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name       string
		explicit   string
		configured string
		want       string
		wantErr    bool
	}{
		{"explicit wins over configured", "ja", "en", "ja", false},
		{"configured is used without an explicit value", "", "ja", "ja", false},
		{"fallback when nothing says a language", "", "", Fallback, false},
		{"unknown language stops", "fr", "", "", true},
		{"unknown configured language stops", "", "fr", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.explicit, tc.configured)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Resolve(%q, %q) = %q, want an error", tc.explicit, tc.configured, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q, %q): %v", tc.explicit, tc.configured, err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q, %q) = %q, want %q", tc.explicit, tc.configured, got, tc.want)
			}
		})
	}
}

// TestUseChoosesTheHandedLanguageAndLeavesTEnglish is d-8e53: Use picks the
// language of the text hergy hands to an agent, and what a person reads through
// T stays English whatever was chosen.
func TestUseChoosesTheHandedLanguageAndLeavesTEnglish(t *testing.T) {
	const key = "prompt.actor"
	english, japanese := catalogs["en"][key], catalogs["ja"][key]
	if english == "" || japanese == "" || english == japanese {
		t.Fatalf("%s reads %q in en and %q in ja: the test needs two different sentences", key, english, japanese)
	}
	if err := Use("ja"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Use(Fallback) })
	if got := HandedLanguage(); got != "ja" {
		t.Errorf("HandedLanguage = %q, want %q", got, "ja")
	}
	if got := T(key); got != english {
		t.Errorf("T(%q) = %q with ja chosen, want the English %q", key, got, english)
	}
	if got := Tf(key, "x"); got != fmt.Sprintf(english, "x") {
		t.Errorf("Tf(%q) = %q, want the English sentence", key, got)
	}
	if got := Errorf(key, "x").Error(); got != fmt.Sprintf(english, "x") {
		t.Errorf("Errorf(%q) = %q, want the English sentence", key, got)
	}
	if got := Handed(key); string(got) != japanese {
		t.Errorf("Handed(%q) = %q, want the Japanese %q", key, got, japanese)
	}
	if got := Handedf(key, "x"); string(got) != fmt.Sprintf(japanese, "x") {
		t.Errorf("Handedf(%q) = %q, want the Japanese sentence", key, got)
	}
	// Text names its language; an empty one is the handed one, and a language
	// hergy does not ship reads English.
	for lang, want := range map[string]string{"en": english, "ja": japanese, "": japanese, "fr": english} {
		if got := Text(lang, key); string(got) != want {
			t.Errorf("Text(%q, %q) = %q, want %q", lang, key, got, want)
		}
	}
	if err := Use("fr"); err == nil {
		t.Error("Use of an unloaded locale succeeded, want an error")
	}
}

func TestInitReturnsToTheFallback(t *testing.T) {
	if err := Use("ja"); err != nil {
		t.Fatal(err)
	}
	if err := Init(); err != nil {
		t.Fatalf("Init after Use: %v", err)
	}
	if got := HandedLanguage(); got != Fallback {
		t.Errorf("HandedLanguage after Init = %q, want %q", got, Fallback)
	}
}

func TestTUnknownKey(t *testing.T) {
	const key = "no.such.key"
	if got, want := T(key), "[["+key+"]]"; got != want {
		t.Errorf("T(%q) = %q, want %q", key, got, want)
	}
}

func TestTextUsesTheLanguageItIsGiven(t *testing.T) {
	if err := Use(Fallback); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Use(Fallback) })
	const key = "prompt.actor"
	ja := catalog(t, "ja")[key]
	en := catalog(t, Fallback)[key]
	if ja == en {
		t.Fatalf("%s reads the same in %s and ja; the test cannot tell them apart", key, Fallback)
	}
	if got := Text("ja", key); string(got) != ja {
		t.Errorf("Text(ja, %s) = %q, want %q", key, got, ja)
	}
	if got := Text("", key); string(got) != en {
		t.Errorf("Text with no language = %q, want the language in use %q", got, en)
	}
	if got := Text("fr", key); string(got) != en {
		t.Errorf("Text(fr, %s) = %q, want the language in use %q", key, got, en)
	}
}

func TestNumber(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", -999: "-999", 1200: "1,200", 1234567: "1,234,567", -1200: "-1,200"} {
		if got := Number(n); got != want {
			t.Errorf("Number(%d) = %q, want %q", n, got, want)
		}
	}
}
