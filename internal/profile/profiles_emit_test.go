package profile

import (
	"maps"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// TestEmitProfilesWritesTheOneFormat: a profiles.yaml hergy writes holds the
// version first, then the profiles by name, and under each one its kind, its
// args as a block sequence and its env by variable name, with an args and an env
// the profile does not hold left out (d-cbac).
func TestEmitProfilesWritesTheOneFormat(t *testing.T) {
	profiles := map[string]Profile{
		"solo": {Name: "solo", Kind: "opencode"},
		"impl": {
			Name: "impl",
			Kind: "claude",
			Args: []string{"--model", "opus"},
			Env:  map[string]string{"OPENAI_API_KEY": "sk-test", "HOME_HINT": "here"},
		},
	}
	got, err := EmitProfiles(profiles)
	if err != nil {
		t.Fatalf("EmitProfiles: %v", err)
	}
	want := `version: 1
profiles:
  impl:
    kind: claude
    args:
      - --model
      - opus
    env:
      HOME_HINT: here
      OPENAI_API_KEY: sk-test
  solo:
    kind: opencode
`
	if got != want {
		t.Errorf("EmitProfiles wrote:\n%s\nwant:\n%s", got, want)
	}
}

// TestEmitProfilesWritesNoProfiles: a layer whose file holds no profile keeps
// the version and nothing else, which is a file hergy reads back as no profile.
func TestEmitProfilesWritesNoProfiles(t *testing.T) {
	got, err := EmitProfiles(nil)
	if err != nil {
		t.Fatalf("EmitProfiles: %v", err)
	}
	if got != "version: 1\n" {
		t.Errorf("EmitProfiles(nil) = %q, want the version alone", got)
	}
	if profiles, err := ParseProfiles(got, "profiles.yaml"); err != nil || len(profiles) != 0 {
		t.Errorf("ParseProfiles of %q = %v, %v, want no profile and no error", got, profiles, err)
	}
}

// TestEmitProfilesRoundTrips: a file hergy writes is the file it reads back. The
// names come back in the order EmitProfiles writes them, and writing the same
// profiles twice gives the same text, so a save of an unedited file changes
// nothing.
func TestEmitProfilesRoundTrips(t *testing.T) {
	for _, text := range []string{
		"version: 1\nprofiles:\n  impl:\n    kind: claude\n",
		"version: 1\nprofiles:\n  b:\n    kind: codex\n    args: [\"--x\", \"--y\"]\n  a:\n    kind: claude\n    env:\n      FOO: bar\n      NUMBER: \"1\"\n",
		"version: 1\nprofiles:\n  impl:\n    kind: claude\n    args:\n      - \"--model\"\n      - \"opus\"\n    env: {}\n",
	} {
		profiles, err := ParseProfiles(text, "profiles.yaml")
		if err != nil {
			t.Fatalf("ParseProfiles(%q): %v", text, err)
		}
		first, err := EmitProfiles(profiles)
		if err != nil {
			t.Fatalf("EmitProfiles: %v", err)
		}
		again, err := ParseProfiles(first, "profiles.yaml")
		if err != nil {
			t.Fatalf("ParseProfiles of what hergy wrote (%q): %v", first, err)
		}
		if !reflect.DeepEqual(again, profiles) {
			t.Errorf("reading %q back gave %+v, want %+v", first, again, profiles)
		}
		second, err := EmitProfiles(again)
		if err != nil {
			t.Fatalf("EmitProfiles: %v", err)
		}
		if first != second {
			t.Errorf("writing the same profiles twice gave %q then %q", first, second)
		}
		if got := slices.Sorted(maps.Keys(again)); !slices.Equal(got, slices.Sorted(maps.Keys(profiles))) {
			t.Errorf("the profiles came back as %v, want %v", got, slices.Sorted(maps.Keys(profiles)))
		}
	}
}

// TestEmitProfilesRoundTripsEnvValues: an env value comes back exactly as it
// was, including one that holds a line break (r-daf8 の P2 は、これが成り立つ
// ことを前提にしている), so a screen that saves a profile without touching its
// env changes no value.
func TestEmitProfilesRoundTripsEnvValues(t *testing.T) {
	values := []string{
		"plain",
		"",
		" lead and trail ",
		"a=b",
		"line one\nline two",
		"line one\nline two\n",
		"\n",
		"\n\n",
		"1",
		"true",
		`{"a": 1}`,
		"tab\tx",
		"# not a comment",
		"- a list item",
	}
	for _, value := range values {
		t.Run(strconv.Quote(value), func(t *testing.T) {
			profiles := map[string]Profile{"impl": {Name: "impl", Kind: "claude", Env: map[string]string{"CFG": value}}}
			text, err := EmitProfiles(profiles)
			if err != nil {
				t.Fatalf("EmitProfiles: %v", err)
			}
			back, err := ParseProfiles(text, "profiles.yaml")
			if err != nil {
				t.Fatalf("ParseProfiles of what hergy wrote (%q): %v", text, err)
			}
			if got := back["impl"].Env["CFG"]; got != value {
				t.Errorf("the value came back as %q, want %q (hergy wrote:\n%s)", got, value, text)
			}
		})
	}
}
