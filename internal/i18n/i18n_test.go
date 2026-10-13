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

func TestLocaleKeySetsAreEqual(t *testing.T) {
	base := Available[0]
	reference := catalog(t, base)
	for _, lang := range Available[1:] {
		got := catalog(t, lang)
		var missing, extra []string
		for key := range reference {
			if _, ok := got[key]; !ok {
				missing = append(missing, key)
			}
		}
		for key := range got {
			if _, ok := reference[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		if len(missing) > 0 {
			t.Errorf("keys in %s missing from %s: %v", base, lang, missing)
		}
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

func TestUseAndT(t *testing.T) {
	if err := Use("ja"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Use(Fallback) })
	if got := Language(); got != "ja" {
		t.Errorf("Language = %q, want %q", got, "ja")
	}
	if err := Use("fr"); err == nil {
		t.Error("Use of an unloaded locale succeeded, want an error")
	}
	if key := "error.unknown_language"; T(key) == "" || T(key) == "[["+key+"]]" {
		t.Errorf("T(%q) = %q, want the message", key, T(key))
	}
}
