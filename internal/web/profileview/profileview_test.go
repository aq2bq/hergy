package profileview

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// envSample is the env a real profiles.yaml holds: OPENAI_API_KEY has a value
// that must not leave the app. The Port carries the names alone (Profile.
// EnvNames), so the value cannot reach the screen; the tests below say so.
var envSample = map[string]string{"OPENAI_API_KEY": "sk-secret"}

// envNames is the names of an env map, in order, the way the app hands them in.
func envNames(env map[string]string) []string {
	return slices.Sorted(maps.Keys(env))
}

// fakePort is the app behind the screen in a test: one layer's profiles, the
// profiles a local team definition uses, and where each layer's files live.
type fakePort struct {
	profiles map[string][]Profile
	uses     map[string][]Profile
	roots    map[string]string
}

func (p fakePort) Profiles(layer string) []Profile { return p.profiles[layer] }
func (p fakePort) Uses(layer string) []Profile     { return p.uses[layer] }
func (p fakePort) LayerRoot(layer string) string   { return p.roots[layer] }

// demoPort is the port the tests read: a local profile that overrides a global
// one of the same name, a local profile the global layer does not hold, and a
// global profile the local layer does not hold.
func demoPort() fakePort {
	return fakePort{
		profiles: map[string][]Profile{
			LayerLocal: {
				{Name: "impl", Kind: "claude", Args: []string{"--model", "opus"}, EnvNames: envNames(envSample), Layer: LayerLocal, Over: true},
				{Name: "solo", Kind: "opencode", Layer: LayerLocal},
			},
			LayerGlobal: {
				{Name: "impl", Kind: "opencode", Layer: LayerGlobal, Shadowed: true},
				{Name: "shared", Kind: "claude", Layer: LayerGlobal},
			},
		},
		uses: map[string][]Profile{
			LayerLocal: {
				{Name: "impl", Kind: "claude", Args: []string{"--model", "opus"}, EnvNames: envNames(envSample), Layer: LayerLocal, Over: true},
				{Name: "shared", Kind: "claude", Layer: LayerGlobal},
				{Name: "solo", Kind: "opencode", Layer: LayerLocal},
			},
		},
		roots: map[string]string{LayerLocal: ".hergy", LayerGlobal: "~/.config/hergy"},
	}
}

func newScreen(t *testing.T, port Port) *Screen {
	t.Helper()
	screen, err := New(port)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return screen
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s does not contain %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(body, bad) {
			t.Errorf("%s contains %q:\n%s", what, bad, body)
		}
	}
}

// TestTheScreenShowsTheProfilesAndTheEnvNamesOnly: the table carries each
// profile's name, its kind, its args one to a line and the names of its env
// variables one to a line, and the value of an env variable never reaches the
// body (a value may hold an API key, r-bb9c).
func TestTheScreenShowsTheProfilesAndTheEnvNamesOnly(t *testing.T) {
	body, err := newScreen(t, demoPort()).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustContain(t, "the profiles body", body,
		i18n.T("web.profiles.title"),
		i18n.T("web.profiles.column.name"),
		i18n.T("web.profiles.column.kind"),
		i18n.T("web.profiles.column.args"),
		i18n.T("web.profiles.column.env"),
		i18n.T("web.profiles.env.note"),
		"impl",           // the profile's name
		"claude",         // its kind
		"--model\nopus",  // its args, one to a line
		"OPENAI_API_KEY", // the name of its env variable
	)
	// The value of the env variable is nowhere: the Port has no field for it.
	mustNotContain(t, "the profiles body", body, "sk-secret")
}

// TestTheScreenMarksTheOverrideOnBothLayers: a global profile a local one of
// the same name takes the place of is marked shadowed and says what that means;
// a local profile that overrides a global one says what that means. A profile
// the other layer does not hold carries no sentence.
func TestTheScreenMarksTheOverrideOnBothLayers(t *testing.T) {
	global, err := newScreen(t, demoPort()).ProfileBody(LayerGlobal)
	if err != nil {
		t.Fatalf("ProfileBody global: %v", err)
	}
	mustContain(t, "the global profiles body", global,
		`class="mgroup shadowed"`,
		i18n.T("web.profiles.state.shadowed"),
	)
	mustNotContain(t, "the global profiles body", global, i18n.T("web.profiles.state.over"))

	local, err := newScreen(t, demoPort()).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody local: %v", err)
	}
	mustContain(t, "the local profiles body", local, i18n.T("web.profiles.state.over"))
	mustNotContain(t, "the local profiles body", local,
		i18n.T("web.profiles.state.shadowed"),
		`class="mgroup shadowed"`,
	)
	// Only the two profiles whose name is in both layers carry a sentence: the
	// local "solo" has none.
	if got := strings.Count(local, `class="wrows"`); got != 1 {
		t.Errorf("the local table carries %d state lines, want the one for the overriding profile:\n%s", got, local)
	}
}

// TestTheEffectiveCardIsDrawnForTheLocalLayerAlone: the card that says what a
// local team definition ends up with is drawn for the local layer only, and it
// names which layer each profile came from and which one overrides the global.
func TestTheEffectiveCardIsDrawnForTheLocalLayerAlone(t *testing.T) {
	local, err := newScreen(t, demoPort()).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody local: %v", err)
	}
	mustContain(t, "the local profiles body", local,
		i18n.T("web.profiles.effective"),
		i18n.Tf("web.profiles.effective.row", i18n.Tf("web.layer.name", i18n.T("label."+LayerLocal))),
		i18n.Tf("web.profiles.effective.row", i18n.Tf("web.layer.name", i18n.T("label."+LayerGlobal))),
		i18n.T("web.profiles.over.global"),
		"shared", // the global profile a local definition still reads
	)

	global, err := newScreen(t, demoPort()).ProfileBody(LayerGlobal)
	if err != nil {
		t.Fatalf("ProfileBody global: %v", err)
	}
	mustNotContain(t, "the global profiles body", global, i18n.T("web.profiles.effective"))
}

// TestTheScreenSaysWhenALayerHasNoProfiles: a layer with no profiles.yaml shows
// the profiles table with the one sentence that says so.
func TestTheScreenSaysWhenALayerHasNoProfiles(t *testing.T) {
	port := fakePort{
		profiles: map[string][]Profile{LayerLocal: nil},
		roots:    map[string]string{LayerLocal: ".hergy"},
	}
	body, err := newScreen(t, port).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustContain(t, "an empty profiles table", body,
		i18n.Tf("web.profiles.empty", i18n.T("label."+LayerLocal)),
		`.hergy/profiles.yaml`,
	)
}

// TestTheScreenOffersNoEditingChrome: this screen reads the profiles alone. It
// carries no field to edit a profile, no delete, no add, and no details block
// that would dump profiles.yaml (an env value lives in that file, r-bb9c).
func TestTheScreenOffersNoEditingChrome(t *testing.T) {
	body, err := newScreen(t, demoPort()).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustNotContain(t, "the read-only profiles body", body,
		"<input", "<textarea", "<details",
		"削除", "＋ プロファイルを追加",
	)
}

// TestTheScreenIsNotATabAndAnswersOnePath: the profiles screen is opened from
// the left list, not from the navigation, and its one path is where the left
// list and the home screen point.
func TestTheScreenIsNotATabAndAnswersOnePath(t *testing.T) {
	screen := newScreen(t, demoPort())
	if screen.InNav() {
		t.Error("the profiles screen says the navigation lists it")
	}
	if got := screen.Name(); got != i18n.T("web.profiles.title") {
		t.Errorf("Name = %q, want %q", got, i18n.T("web.profiles.title"))
	}
	routes := screen.Routes()
	if len(routes) != 1 || routes[0].Path != ProfilePath || routes[0].Write {
		t.Errorf("Routes = %+v, want the read-only page at %s", routes, ProfilePath)
	}
	if ProfilePath != "/profiles" {
		t.Errorf("ProfilePath = %q, want /profiles", ProfilePath)
	}
}
