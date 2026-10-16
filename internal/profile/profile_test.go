package profile

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/i18n"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestParseProfilesHoldsTheFileToTheSettingsRules: the version first and no
// unknown key, the way every settings file is read.
func TestParseProfilesHoldsTheFileToTheSettingsRules(t *testing.T) {
	cases := []struct{ name, text string }{
		{"without a version", "profiles:\n  p:\n    kind: claude\n"},
		{"with a version that is not 1", "version: 2\nprofiles:\n  p:\n    kind: claude\n"},
		{"with an unknown key", "version: 1\nprofiles:\n  p:\n    kind: claude\n    unknown_key: 1\n"},
		{"with a name that is not a hergy name", "version: 1\nprofiles:\n  P:\n    kind: claude\n"},
		{"with a kind that is not a kind", "version: 1\nprofiles:\n  p:\n    kind: P\n"},
		{"with an argument that is not a string", "version: 1\nprofiles:\n  p:\n    kind: claude\n    args: [1]\n"},
		{"with an environment name that is not one", "version: 1\nprofiles:\n  p:\n    kind: claude\n    env:\n      'A B': x\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseProfiles(tc.text, "profiles.yaml"); err == nil {
				t.Error("a broken profiles.yaml did not stop the parse")
			}
		})
	}
}

// TestCheatMachineNamesEveryProfileKey: the keys a profiles.yaml reads are named
// in the cheatsheet for an agent, so a key added later cannot be left out. The
// keys of the other settings files are checked by the package that reads them.
func TestCheatMachineNamesEveryProfileKey(t *testing.T) {
	keys := map[string]bool{}
	collectYAMLKeys(keys, reflect.TypeFor[rawProfiles]())
	if len(keys) == 0 {
		t.Fatal("no yaml keys were collected; the reflection walk is broken")
	}
	for key := range keys {
		if !strings.Contains(cheat.Machine, key) {
			t.Errorf("cheat --ai does not name the profiles.yaml key %q", key)
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
		if typ.PkgPath() != reflect.TypeFor[rawProfile]().PkgPath() {
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

func FuzzParseProfiles(f *testing.F) {
	f.Add("version: 1\nprofiles:\n  seeker:\n    kind: claude\n")
	f.Add("profiles: {a: {kind: x}}")
	f.Add("- 1")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		profiles, err := ParseProfiles(text, "profiles.yaml")
		if err != nil {
			return
		}
		// A profiles.yaml hergy writes is one it reads back as the same
		// profiles: EmitProfiles is what the profiles screen saves with
		// (d-cbac), so a text it writes that reads as something else would be a
		// save that changes the file it was shown.
		written, err := EmitProfiles(profiles)
		if err != nil {
			t.Fatalf("EmitProfiles(%+v): %v", profiles, err)
		}
		again, err := ParseProfiles(written, "profiles.yaml")
		if err != nil {
			t.Fatalf("ParseProfiles of what hergy wrote (%q): %v", written, err)
		}
		if !reflect.DeepEqual(again, profiles) {
			t.Errorf("reading %q back gave %+v, want %+v", written, again, profiles)
		}
	})
}
