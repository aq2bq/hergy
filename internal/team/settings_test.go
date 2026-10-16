package team

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
)

// TestConfigYAMLRoundTrips: the config.yaml hergy writes is one ParseConfig
// reads back to the same language.
func TestConfigYAMLRoundTrips(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		text := GlobalYAML(lang)
		got, err := ParseConfig(text, "config.yaml")
		if err != nil {
			t.Fatalf("ParseConfig(%q) = %v", text, err)
		}
		if got != lang {
			t.Errorf("ParseConfig(%q) lang = %q, want %q", text, got, lang)
		}
	}
}

// TestConfigYAMLStopsOnBrokenInput: config.yaml is held to the same strictness as
// team.yaml: the version must be there and 1, and an unknown key stops.
func TestConfigYAMLStopsOnBrokenInput(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"without a version", "lang: ja\n"},
		{"with an unknown key", "version: 1\nlang: ja\nunknown_key: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseConfig(tc.text, "config.yaml"); err == nil {
				t.Error("broken config.yaml did not stop the parse")
			}
		})
	}
}

// TestCheatMachineNamesEveryConfigKey collects every yaml key config.yaml's
// settings struct reads, by reflection over its yaml tags, and checks the agent
// cheatsheet names each one, so a key added later cannot be left out. The keys
// team.yaml reads are checked where that struct lives (internal/definition).
func TestCheatMachineNamesEveryConfigKey(t *testing.T) {
	keys := map[string]bool{}
	collectYAMLKeys(keys, reflect.TypeFor[rawConfig]())
	if len(keys) == 0 {
		t.Fatal("no yaml keys were collected; the reflection walk is broken")
	}
	for key := range keys {
		if !strings.Contains(cheat.Machine, key) {
			t.Errorf("cheat --ai does not name the settings key %q", key)
		}
	}
}

func collectYAMLKeys(keys map[string]bool, typ reflect.Type) {
	if typ == nil {
		return
	}
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		collectYAMLKeys(keys, typ.Elem())
	case reflect.Struct:
		if typ.PkgPath() != reflect.TypeFor[rawConfig]().PkgPath() {
			return
		}
		for field := range typ.Fields() {
			if tag, _, _ := strings.Cut(field.Tag.Get("yaml"), ","); tag != "" && tag != "-" {
				keys[tag] = true
			}
			collectYAMLKeys(keys, field.Type)
		}
	default:
		// scalars carry no yaml keys.
	}
}

// The config.yaml reader must never panic, whatever the file holds. The seeds run
// with go test; the nightly workflow (go run ./tools/check nightly) runs them
// with -fuzz.
func FuzzParseConfig(f *testing.F) {
	f.Add("version: 1\nlang: ja\n")
	f.Add("lang: [1]")
	f.Add("")
	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = ParseConfig(text, filepath.Join(os.TempDir(), "config.yaml"))
	})
}
