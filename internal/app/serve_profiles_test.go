package app

import (
	"os"
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

// TestServePortProfilesCarriesTheEnvValues: the env of a profile reaches the
// screen with its values too (d-c60f: env holds no API key, so the screen shows
// them and edits them).
func TestServePortProfilesCarriesTheEnvValues(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}

	local := p.Profiles(web.LayerLocal)
	if len(local) == 0 {
		t.Fatal("the local layer holds no profile")
	}
	if got := local[0].Env; len(got) != 1 || got[0].Name != "OPENAI_API_KEY" || got[0].Value != "sk-secret" {
		t.Errorf("the env = %+v, want the name and the value of the profile's variable", got)
	}
	// The profiles a local team definition uses carry them the same way.
	for _, profile := range p.Uses(web.LayerLocal) {
		if profile.Name != "impl" {
			continue
		}
		if len(profile.Env) != 1 || profile.Env[0].Value != "sk-secret" {
			t.Errorf("impl in the uses list = %+v, want its env value", profile.Env)
		}
	}
}

// TestServePortReadProfilesReadsTheFileForTheScreen: the screen is drawn from
// and saved against what this reads: the text, the hash of that text, whether
// the file is there, and, when it does not read, the reason hergy validate gives.
func TestServePortReadProfilesReadsTheFileForTheScreen(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}

	local := p.ReadProfiles(web.LayerLocal)
	if !local.Exists || local.Text == "" || local.Broken != "" {
		t.Fatalf("the local file = %+v, want the text of a file that reads", local)
	}
	if local.Hash != hashText(local.Text) {
		t.Errorf("the hash = %q, want the hash of the text the screen shows", local.Hash)
	}

	// A layer without the file: the empty text with the hash of it, and no
	// reason. This is the layer the first save writes the file for.
	if err := os.Remove(filepath.Join(f.cfg, "hergy", "profiles.yaml")); err != nil {
		t.Fatal(err)
	}
	missing := p.ReadProfiles(web.LayerGlobal)
	if missing.Exists || missing.Text != "" || missing.Broken != "" || missing.Hash != hashText("") {
		t.Errorf("a layer with no file = %+v, want the empty text, its hash and no reason", missing)
	}

	// A file that does not read: the reason is what hergy validate would say,
	// and it names the file the way a person writes it.
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "profiles:\n  impl:\n    kind: claude\n")
	broken := p.ReadProfiles(web.LayerLocal)
	if broken.Broken == "" {
		t.Fatal("a file with no version reads as sound, want the reason")
	}
	if !strings.Contains(broken.Broken, ".hergy/profiles.yaml") {
		t.Errorf("the reason = %q, want it to name the file", broken.Broken)
	}
}

// TestServePortCheckProfilesIsTheCheckValidateRuns: what the screen asks before a
// save is the check hergy validate runs on the file the screen would write, in
// the words a person reads.
func TestServePortCheckProfilesIsTheCheckValidateRuns(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")

	sound := "version: 1\nprofiles:\n  impl:\n    kind: claude\n"
	if reason := p.CheckProfiles(web.LayerLocal, sound); reason != "" {
		t.Errorf("a profiles.yaml that reads = %q, want no reason", reason)
	}

	broken := "version: 1\nprofiles:\n  impl:\n    kind: Claude Code\n"
	reason := p.CheckProfiles(web.LayerLocal, broken)
	if reason == "" {
		t.Fatal("a kind hergy refuses reads as sound")
	}
	// The same text on disk is what hergy validate refuses, so the screen and
	// the command agree on what is broken.
	writeFile(t, path, broken)
	if _, err := validateProfiles(path); err == nil {
		t.Error("hergy validate reads the text the screen refused as sound")
	}
	if !strings.Contains(reason, ".hergy/profiles.yaml") {
		t.Errorf("the reason = %q, want it to name the file", reason)
	}
}

// TestServePortCheckProfilesRefusesASaveThatBreaksATeamDefinition: the check the
// screen runs before a save builds the team definitions that read the layer's
// profiles.yaml, so a save that takes away a profile a definition names — by
// deleting it or by renaming it — is refused, and a profile no definition names
// can go (r-daf8 の P1). Without this check the save would land and hergy
// validate would then refuse the project.
func TestServePortCheckProfilesRefusesASaveThatBreaksATeamDefinition(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	local := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	// The local layer holds profiles the global layer does not, so dropping one
	// of those takes it away for good: impl is in both layers and would fall
	// back on the global one.
	writeFile(t, local, "version: 1\nprofiles:\n  impl:\n    kind: claude\n  solo:\n    kind: opencode\n  seeker:\n    kind: codex\n")
	// One member that runs solo, and one worker it may hire that runs seeker:
	// both names are named by the definition, impl is not.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
prompt: "%{spawn}"
members:
  - name: lead
    role: lead
    profile: solo
    may_spawn:
      workers:
        - purpose: research
          profile: seeker
`)
	p := servePort{env: serveEnv(t)}

	// The file as it stands reads: nothing to refuse.
	if reason := p.CheckProfiles(web.LayerLocal, readFile(t, local)); reason != "" {
		t.Errorf("the file the project holds = %q, want no reason", reason)
	}

	// Dropping solo leaves demo's lead without the profile it runs.
	reason := p.CheckProfiles(web.LayerLocal, "version: 1\nprofiles:\n  impl:\n    kind: claude\n  seeker:\n    kind: codex\n")
	if reason == "" {
		t.Fatal("a save that drops a profile a team definition uses reads as sound")
	}
	for _, want := range []string{"demo", "lead", "solo"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the reason = %q, want it to name %q", reason, want)
		}
	}

	// Renaming it is the same thing: the name the member runs is gone.
	if reason := p.CheckProfiles(web.LayerLocal, "version: 1\nprofiles:\n  impl:\n    kind: claude\n  seeker:\n    kind: codex\n  solo2:\n    kind: opencode\n"); reason == "" {
		t.Error("a save that renames a profile a team definition uses reads as sound")
	}

	// The profile a worker is hired with is watched the same way, and the
	// reason names the member and the purpose it hires for.
	reason = p.CheckProfiles(web.LayerLocal, "version: 1\nprofiles:\n  impl:\n    kind: claude\n  solo:\n    kind: opencode\n")
	if reason == "" {
		t.Fatal("a save that drops the profile of a worker reads as sound")
	}
	if !strings.Contains(reason, "research") {
		t.Errorf("the reason = %q, want it to name the purpose the worker is hired for", reason)
	}

	// A profile no definition names can go: impl is used by nothing here.
	sound := "version: 1\nprofiles:\n  solo:\n    kind: opencode\n  seeker:\n    kind: codex\n"
	if reason := p.CheckProfiles(web.LayerLocal, sound); reason != "" {
		t.Errorf("a save that drops impl = %q, want no reason: no team definition names it", reason)
	}

	// The same file is what hergy validate builds the definition from: a save
	// the screen refuses is a project hergy would refuse.
	writeFile(t, local, "version: 1\nprofiles:\n  impl:\n    kind: claude\n  solo:\n    kind: opencode\n")
	if _, err := validateTeam(p.env, "demo", web.LayerLocal); err == nil {
		t.Error("hergy validate builds the definition the screen refused")
	}
	writeFile(t, local, sound)
	if _, err := validateTeam(p.env, "demo", web.LayerLocal); err != nil {
		t.Errorf("hergy validate refuses the file the screen allowed: %v", err)
	}
}

// TestServePortCheckProfilesWatchesTheLocalLayerForAGlobalSave: a local team
// definition reads the global layer's profiles under its own, so a global save
// that drops a profile a local definition names is refused too, and the reason
// names that definition (r-daf8 の P1).
func TestServePortCheckProfilesWatchesTheLocalLayerForAGlobalSave(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	global := filepath.Join(f.cfg, "hergy", "profiles.yaml")
	// The local definition runs the global layer's shared profile: the project's
	// own profiles.yaml does not hold that name.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nprompt: \"%{spawn}\"\nmembers:\n  - name: lead\n    role: lead\n    profile: shared\n")
	p := servePort{env: serveEnv(t)}

	// The global file as it stands reads for the local definition.
	if reason := p.CheckProfiles(web.LayerGlobal, readFile(t, global)); reason != "" {
		t.Errorf("the global file the project holds = %q, want no reason", reason)
	}

	// Dropping the global shared leaves the local definition without it.
	reason := p.CheckProfiles(web.LayerGlobal, "version: 1\nprofiles:\n  impl:\n    kind: opencode\n")
	if reason == "" {
		t.Fatal("a global save that drops a profile a local team definition uses reads as sound")
	}
	if !strings.Contains(reason, "demo") {
		t.Errorf("the reason = %q, want it to name the definition that reads the name", reason)
	}

	// Keeping the name reads, so a global save is not refused for a name the
	// global layer holds.
	if reason := p.CheckProfiles(web.LayerGlobal, "version: 1\nprofiles:\n  impl:\n    kind: opencode\n  shared:\n    kind: claude\n"); reason != "" {
		t.Errorf("a global save that keeps every name = %q, want no reason", reason)
	}
}

// TestServePortWriteProfilesWritesOneLayer: the one write of the profiles screen
// writes the layer's own profiles.yaml, makes the file when the layer has none
// yet, and refuses a file that is not the one the screen read.
func TestServePortWriteProfilesWritesOneLayer(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeProfiles(t, f)
	p := servePort{env: serveEnv(t)}
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	global := filepath.Join(f.cfg, "hergy", "profiles.yaml")

	before, err := os.ReadFile(global)
	if err != nil {
		t.Fatal(err)
	}
	read := p.ReadProfiles(web.LayerLocal)
	text := "version: 1\nprofiles:\n  solo:\n    kind: opencode\n"
	if err := p.WriteProfiles(web.LayerLocal, text, read.Hash); err != nil {
		t.Fatalf("WriteProfiles: %v", err)
	}
	if got := readFile(t, path); got != text {
		t.Errorf("the local file holds %q, want %q", got, text)
	}
	if got := readFile(t, global); got != string(before) {
		t.Errorf("the global file changed to %q, want it untouched", got)
	}

	// A hash that is not the one the screen read: nothing is written, and the
	// reason is the one the screen shows.
	err = p.WriteProfiles(web.LayerLocal, "version: 1\n", hashText("something else"))
	if err == nil {
		t.Fatal("a write from an older file was accepted")
	}
	if !strings.Contains(err.Error(), "変わりました") && !strings.Contains(err.Error(), "changed on disk") {
		t.Errorf("the refusal = %q, want the words about a file that moved", err)
	}
	if got := readFile(t, path); got != text {
		t.Errorf("a refused write left %q, want %q", got, text)
	}

	// A layer whose profiles.yaml is not there yet: the save makes it.
	if err := os.Remove(global); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteProfiles(web.LayerGlobal, text, hashText("")); err != nil {
		t.Fatalf("WriteProfiles of a layer with no file: %v", err)
	}
	if got := readFile(t, global); got != text {
		t.Errorf("the global file holds %q, want the text of the first save", got)
	}
}

// TestServePortWriteProfilesRefusesALayerThatIsNotThere: a project with no
// .hergy has no local layer to write, and the reason says which directory is
// missing rather than making one.
func TestServePortWriteProfilesRefusesALayerThatIsNotThere(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	p := servePort{env: serveEnv(t)}

	err := p.WriteProfiles(web.LayerLocal, "version: 1\n", hashText(""))
	if err == nil {
		t.Fatal("a write into a layer that is not there was accepted")
	}
	if !strings.Contains(err.Error(), ".hergy") {
		t.Errorf("the refusal = %q, want it to name the directory that is not there", err)
	}
	if _, statErr := os.Stat(filepath.Join(f.proj, ".hergy")); statErr == nil {
		t.Error("the refused write made the layer's directory")
	}
}
