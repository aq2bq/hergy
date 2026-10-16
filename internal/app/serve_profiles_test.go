package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/profileview"
)

// writeProfiles writes one layer's profiles.yaml. The two files the tests use
// share the name "impl": the local one overrides the global one.
func writeProfiles(t *testing.T, f *fixture) {
	t.Helper()
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), `version: 1
profiles:
  impl:
    kind: claude
    args: ["--model", "opus"]
    env:
      OPENAI_API_KEY: sk-secret
  solo:
    kind: opencode
`)
	writeFile(t, filepath.Join(f.cfg, "hergy", "profiles.yaml"), `version: 1
profiles:
  impl:
    kind: opencode
  shared:
    kind: claude
`)
}

// profileNames is the names of a list of profiles, in the order they come.
func profileNames(profiles []profileview.Profile) []string {
	out := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.Name)
	}
	return out
}

// TestServePortProfilesReadsOneLayerAndMarksTheOther: Profiles is one layer's
// own profiles.yaml, by name, and it marks each name the other layer also holds
// (a global profile a local one takes the place of, and the local one that does
// it).
func TestServePortProfilesReadsOneLayerAndMarksTheOther(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}

	local := p.Profiles(web.LayerLocal)
	if got := profileNames(local); strings.Join(got, ",") != "impl,solo" {
		t.Fatalf("the local profiles = %v, want impl and solo by name", got)
	}
	impl := local[0]
	if impl.Kind != "claude" || strings.Join(impl.Args, " ") != "--model opus" {
		t.Errorf("the local impl = %+v, want its kind and args", impl)
	}
	if !impl.Over || impl.Shadowed {
		t.Errorf("the local impl = %+v, want it marked as overriding the global one", impl)
	}
	if local[1].Over || local[1].Shadowed {
		t.Errorf("a profile the other layer does not hold is marked: %+v", local[1])
	}

	global := p.Profiles(web.LayerGlobal)
	if got := profileNames(global); strings.Join(got, ",") != "impl,shared" {
		t.Fatalf("the global profiles = %v, want impl and shared by name", got)
	}
	if !global[0].Shadowed || global[0].Over {
		t.Errorf("the global impl = %+v, want it marked as overridden by the local one", global[0])
	}
	if global[1].Shadowed || global[1].Over {
		t.Errorf("a global profile the local layer does not hold is marked: %+v", global[1])
	}
}

// TestServePortUsesOverlaysLocalOnGlobalLikeLoadProfiles: Uses is the profiles
// a team definition ends up with, by the rule Env.LoadProfiles reads: a local
// definition lays the local layer over the global one, and a global definition
// reads the global layer alone. Each profile says which layer it came from.
func TestServePortUsesOverlaysLocalOnGlobalLikeLoadProfiles(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	env := serveEnv(t)
	p := servePort{env: env}

	merged, _, err := env.LoadProfiles(web.LayerLocal)
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	local := p.Uses(web.LayerLocal)
	// The project's own profiles come first and the global layer's after, so a
	// person reads what its team definitions use before what they fall back on.
	if got := profileNames(local); strings.Join(got, ",") != "impl,solo,shared" {
		t.Fatalf("the local uses = %v, want the project's own first and then the global layer's (%v)", got, merged)
	}
	from := map[string]profileview.Profile{}
	for _, profile := range local {
		from[profile.Name] = profile
	}
	if from["impl"].Layer != web.LayerLocal || !from["impl"].Over {
		t.Errorf("impl = %+v, want the local layer's, overriding the global one", from["impl"])
	}
	// The local file's profile is the one that wins, so its kind is the local
	// "claude" and not the global "opencode": the overlay is LoadProfiles's.
	if from["impl"].Kind != "claude" {
		t.Errorf("impl kind = %q, want the local profile's claude", from["impl"].Kind)
	}
	if from["solo"].Layer != web.LayerLocal || from["solo"].Over {
		t.Errorf("solo = %+v, want the local layer's, overriding nothing", from["solo"])
	}
	if from["shared"].Layer != web.LayerGlobal || from["shared"].Over {
		t.Errorf("shared = %+v, want the global layer's, overriding nothing", from["shared"])
	}

	global := p.Uses(web.LayerGlobal)
	if got := profileNames(global); strings.Join(got, ",") != "impl,shared" {
		t.Fatalf("the global uses = %v, want the global layer's profiles", got)
	}
	for _, profile := range global {
		if profile.Layer != web.LayerGlobal || profile.Over {
			t.Errorf("a global team definition reads %+v, want the global layer's profile with no override", profile)
		}
	}
}

// TestServePortProfilesNeverCarriesAnEnvValue: the env of a profile reaches the
// screen as names alone. A value may hold an API key (r-bb9c), so it must not be
// anywhere in what Profiles or Uses hands over.
func TestServePortProfilesNeverCarriesAnEnvValue(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}

	local := p.Profiles(web.LayerLocal)
	if got := strings.Join(local[0].EnvNames, ","); got != "OPENAI_API_KEY" {
		t.Errorf("the env names = %q, want the one name of the profile", got)
	}
	for _, profile := range append(local, p.Uses(web.LayerLocal)...) {
		if dump := fmt.Sprintf("%+v", profile); strings.Contains(dump, "sk-secret") {
			t.Errorf("a profile carries an env value:\n%s", dump)
		}
	}
}
