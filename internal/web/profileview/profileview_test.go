package profileview

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// envSample is the env a real profiles.yaml holds. Its value is what the screen
// shows and edits (d-c60f: env holds no API key).
var envSample = []EnvVar{{Name: "OPENAI_API_KEY", Value: "sk-test"}}

// hashOf is the hash the screen hands back at save time, the way the app
// computes it.
func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// fakePort is the app behind the screen in a test: one layer's profiles, the
// profiles a local team definition uses, where each layer's files live, and one
// profiles.yaml per layer, kept in memory. A save goes through the same three
// steps the app takes: the check hergy validate runs over the text, the hash
// that says the file has not moved, and the write.
type fakePort struct {
	profiles map[string][]Profile
	uses     map[string][]Profile
	roots    map[string]string
	file     map[string]Source
	// wrote is the text the last save wrote, per layer, and writeErr what a
	// write answers instead of writing.
	wrote    map[string]string
	writeErr error
}

func (p *fakePort) Profiles(layer string) []Profile { return p.profiles[layer] }
func (p *fakePort) Uses(layer string) []Profile     { return p.uses[layer] }
func (p *fakePort) LayerRoot(layer string) string   { return p.roots[layer] }

func (p *fakePort) ReadProfiles(layer string) Source { return p.file[layer] }

func (p *fakePort) CheckProfiles(_, text string) string {
	if _, err := config.ParseProfiles(text, "profiles.yaml"); err != nil {
		return err.Error()
	}
	return ""
}

func (p *fakePort) WriteProfiles(layer, text, hash string) error {
	if p.writeErr != nil {
		return p.writeErr
	}
	if hash != p.file[layer].Hash {
		return i18n.Errorf("error.profiles_file_changed", "profiles.yaml")
	}
	if p.wrote == nil {
		p.wrote = map[string]string{}
	}
	p.wrote[layer] = text
	// The file is what was written, and the layer's profiles are what it holds:
	// the page drawn after a save is drawn from the file, the way it is by the
	// app.
	profiles, err := config.ParseProfiles(text, "profiles.yaml")
	if err != nil {
		return err
	}
	rows := make([]Profile, 0, len(profiles))
	for _, name := range slices.Sorted(maps.Keys(profiles)) {
		profile := profiles[name]
		env := make([]EnvVar, 0, len(profile.Env))
		for _, variable := range slices.Sorted(maps.Keys(profile.Env)) {
			env = append(env, EnvVar{Name: variable, Value: profile.Env[variable]})
		}
		rows = append(rows, Profile{Name: profile.Name, Kind: profile.Kind, Args: profile.Args, Env: env, Layer: layer})
	}
	p.profiles[layer] = rows
	p.file[layer] = Source{Text: text, Hash: hashOf(text), Exists: true}
	return nil
}

// demoPort is the port the tests read: a local profile that overrides a global
// one of the same name, a local profile the global layer does not hold, and a
// global profile the local layer does not hold.
func demoPort() *fakePort {
	return &fakePort{
		profiles: map[string][]Profile{
			LayerLocal: {
				{Name: "impl", Kind: "claude", Args: []string{"--model", "opus"}, Env: envSample, Layer: LayerLocal, Over: true},
				{Name: "solo", Kind: "opencode", Layer: LayerLocal},
			},
			LayerGlobal: {
				{Name: "impl", Kind: "opencode", Layer: LayerGlobal, Shadowed: true},
				{Name: "shared", Kind: "claude", Layer: LayerGlobal},
			},
		},
		uses: map[string][]Profile{
			LayerLocal: {
				{Name: "impl", Kind: "claude", Args: []string{"--model", "opus"}, Env: envSample, Layer: LayerLocal, Over: true},
				{Name: "shared", Kind: "claude", Layer: LayerGlobal},
				{Name: "solo", Kind: "opencode", Layer: LayerLocal},
			},
		},
		roots: map[string]string{LayerLocal: ".hergy", LayerGlobal: "~/.config/hergy"},
		file: map[string]Source{
			LayerLocal:  {Text: "version: 1\nprofiles:\n  impl:\n    kind: claude\n", Hash: hashOf("version: 1\nprofiles:\n  impl:\n    kind: claude\n"), Exists: true},
			LayerGlobal: {Text: "version: 1\nprofiles:\n  impl:\n    kind: opencode\n", Hash: hashOf("version: 1\nprofiles:\n  impl:\n    kind: opencode\n"), Exists: true},
		},
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

// mustContain and mustNotContain read the page as a person does: the words are
// compared as text, so an apostrophe the template wrote as &#39; is the same
// character here.
func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	text := html.UnescapeString(body)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not contain %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	text := html.UnescapeString(body)
	for _, bad := range unwanted {
		if strings.Contains(text, bad) {
			t.Errorf("%s contains %q:\n%s", what, bad, body)
		}
	}
}

// TestTheScreenShowsTheProfilesAsEditableFields: the table carries a field for
// each profile's name, its kind, its args one to a line and its env one
// name=value to a line, with what the profile holds in it. The value of an env
// variable is on the page like the rest (d-c60f).
func TestTheScreenShowsTheProfilesAsEditableFields(t *testing.T) {
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
		i18n.T("web.profiles.edit.note"),
		`name="name" value="impl"`,                               // the profile's name, in a field
		`name="kind" value="claude"`,                             // its kind
		"--model\nopus",                                          // its args, one to a line
		"OPENAI_API_KEY=sk-test",                                 // its env, name=value
		i18n.T("web.profiles.save"),                              // the save button
		i18n.T("web.profiles.remove"),                            // the per-row delete
		i18n.Tf("web.profiles.add", i18n.T("label."+LayerLocal)), // the add button
		i18n.T("web.profiles.state.over"),                        // the mark the row keeps
		i18n.T("web.profiles.effective"),                         // the card of what a team definition uses
		i18n.T("web.profiles.lede.global"),                       // what the layers read
		`action="`+SavePath+`?layer=`+LayerLocal+`"`,             // the form saves at the save path
		`formaction="`+EditPath+`?layer=`+LayerLocal+`"`,         // add and delete go to the edit path
		`<input type="hidden" name="hash"`,                       // the form carries the version it read
		`/profiles.yaml`,                                         // the file the heading names
	)
	// The sentence that said the env values are not shown is gone with the
	// mechanism that kept them out (d-c60f).
	mustNotContain(t, "the profiles body", body, "画面に出しません")
}

// TestTheFormCarriesTheLayerItIsOn: a write is answered for the layer in the
// URL, so the form a page draws posts its own layer, and the page of the global
// layer never writes the local one.
func TestTheFormCarriesTheLayerItIsOn(t *testing.T) {
	body, err := newScreen(t, demoPort()).ProfileBody(LayerGlobal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustContain(t, "the global profiles body", body,
		`action="`+SavePath+`?layer=`+LayerGlobal+`"`,
		`formaction="`+EditPath+`?layer=`+LayerGlobal+`"`,
	)
	mustNotContain(t, "the global profiles body", body, `?layer=`+LayerLocal)
}

// TestTheScreenDrawsTheFormTheSaveReads: the fields of the page are the fields
// SaveBody reads: reading the page back as a form gives the profiles the port
// handed in.
func TestTheScreenDrawsTheFormTheSaveReads(t *testing.T) {
	body, err := newScreen(t, demoPort()).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	form := url.Values{}
	for _, match := range fieldRE.FindAllStringSubmatch(body, -1) {
		// An input carries its text in the value, a textarea between the tags.
		form[match[1]] = append(form[match[1]], html.UnescapeString(match[2]+match[3]))
	}
	rows := rowsOf(form)
	if len(rows) != 2 {
		t.Fatalf("the page holds %d rows, want the two profiles: %+v", len(rows), rows)
	}
	if rows[0].Name != "impl" || rows[0].Kind != "claude" || rows[0].Args != "--model\nopus" || rows[0].Env != "OPENAI_API_KEY=sk-test" {
		t.Errorf("the first row = %+v, want the impl profile with its args and env", rows[0])
	}
	if rows[1].Name != "solo" || rows[1].Kind != "opencode" {
		t.Errorf("the second row = %+v, want the solo profile", rows[1])
	}
}

// fieldRE is one field of the form as the page draws it: an input or a textarea
// under one of the names the handler reads.
var fieldRE = regexp.MustCompile(`<(?:input|textarea)[^>]*\bname="(name|kind|args|env)"[^>]*?(?:value="([^"]*)"|>([^<]*)</)`)

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
// the empty table, the sentence that says so, and the buttons to add one.
func TestTheScreenSaysWhenALayerHasNoProfiles(t *testing.T) {
	port := &fakePort{
		profiles: map[string][]Profile{LayerLocal: nil},
		roots:    map[string]string{LayerLocal: ".hergy"},
		file:     map[string]Source{LayerLocal: {Hash: hashOf("")}},
	}
	body, err := newScreen(t, port).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustContain(t, "an empty profiles table", body,
		i18n.Tf("web.profiles.empty", i18n.T("label."+LayerLocal)),
		`.hergy/profiles.yaml`,
		i18n.Tf("web.profiles.add", i18n.T("label."+LayerLocal)),
	)
}

// TestTheScreenShowsAFileItCannotReadAndDoesNotEditIt: a profiles.yaml that does
// not read is shown with the reason hergy validate gives, and with no form: hergy
// does not write over a file it cannot read.
func TestTheScreenShowsAFileItCannotReadAndDoesNotEditIt(t *testing.T) {
	port := demoPort()
	port.file[LayerLocal] = Source{Text: "profiles: [", Hash: hashOf("profiles: ["), Exists: true,
		Broken: "profiles.yaml: version is missing"}
	port.profiles[LayerLocal] = nil
	body, err := newScreen(t, port).ProfileBody(LayerLocal)
	if err != nil {
		t.Fatalf("ProfileBody: %v", err)
	}
	mustContain(t, "the broken profiles body", body,
		i18n.T("web.profiles.broken"),
		"version is missing",
	)
	mustNotContain(t, "the broken profiles body", body, `name="name"`, i18n.T("web.profiles.save"))
}

// TestTheScreenIsNotATabAndAnswersThePageAndTwoWrites: the profiles screen is
// opened from the left list, not from the navigation, and its paths are the page
// and the two writes the form makes.
func TestTheScreenIsNotATabAndAnswersThePageAndTwoWrites(t *testing.T) {
	screen := newScreen(t, demoPort())
	if screen.InNav() {
		t.Error("the profiles screen says the navigation lists it")
	}
	if got := screen.Name(); got != i18n.T("web.profiles.title") {
		t.Errorf("Name = %q, want %q", got, i18n.T("web.profiles.title"))
	}
	routes := screen.Routes()
	want := []web.Route{{Path: ProfilePath}, {Path: EditPath, Write: true}, {Path: SavePath, Write: true}}
	if len(routes) != len(want) {
		t.Fatalf("Routes = %+v, want the page and the two writes", routes)
	}
	for i, route := range routes {
		if route.Path != want[i].Path || route.Write != want[i].Write {
			t.Errorf("route %d = %+v, want %+v", i, route, want[i])
		}
	}
	if ProfilePath != "/profiles" {
		t.Errorf("ProfilePath = %q, want /profiles", ProfilePath)
	}
}

// formOf is one form the page would send: the rows as they stand, under the
// names the handler reads.
func formOf(rows ...Row) url.Values {
	values := url.Values{"hash": {"hash"}, "token": {"token"}}
	for _, row := range rows {
		values["name"] = append(values["name"], row.Name)
		values["kind"] = append(values["kind"], row.Kind)
		values["args"] = append(values["args"], row.Args)
		values["env"] = append(values["env"], row.Env)
	}
	return values
}

// TestEditBodyAddsAndDropsARow: the add button opens one more empty row and the
// delete button takes the row it names out, with everything the person typed
// kept. Neither writes a file.
func TestEditBodyAddsAndDropsARow(t *testing.T) {
	port := demoPort()
	screen := newScreen(t, port)

	added, err := screen.EditBody(LayerLocal, "tok", url.Values{
		"name": {"a", "b"}, "kind": {"claude", "codex"}, "args": {"--x", ""}, "env": {"FOO=bar", ""},
		"add": {"1"}, "hash": {"h"},
	})
	if err != nil {
		t.Fatalf("EditBody: %v", err)
	}
	mustContain(t, "the page with a row added", added, `name="name" value="a"`, `name="name" value="b"`, `name="name" value=""`)
	mustContain(t, "the page with a row added", added, `name="hash" value="h"`)

	dropped, err := screen.EditBody(LayerLocal, "tok", url.Values{
		"name": {"a", "b"}, "kind": {"claude", "codex"}, "args": {"--x", ""}, "env": {"FOO=bar", ""},
		"drop": {"0"}, "hash": {"h"},
	})
	if err != nil {
		t.Fatalf("EditBody: %v", err)
	}
	mustContain(t, "the page with a row dropped", dropped, `name="name" value="b"`)
	mustNotContain(t, "the page with a row dropped", dropped, `name="name" value="a"`)

	if len(port.wrote) != 0 {
		t.Errorf("adding and dropping wrote a file: %v", port.wrote)
	}
}

// TestSaveBodyWritesTheRowsAsTheOneFormat: a save writes the rows of the form
// into the layer's profiles.yaml: a name changed, args and env values in, a
// profile added, and a profile the form no longer holds deleted.
func TestSaveBodyWritesTheRowsAsTheOneFormat(t *testing.T) {
	port := demoPort()
	// The page was drawn with two rows, and the person renamed one, gave the
	// other args and env values, dropped the first and added a new profile.
	values := formOf(
		Row{Name: "impl", Kind: "claude", Args: "--model\nopus", Env: "OPENAI_API_KEY=sk-test\nHOME_HINT=here"},
		Row{Name: "solo", Kind: "opencode", Args: "--foo bar"},
		Row{Name: "fresh", Kind: "codex"},
	)
	values.Set("hash", port.file[LayerLocal].Hash)
	body, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	want := `version: 1
profiles:
  fresh:
    kind: codex
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
    args:
      - --foo bar
`
	if got := port.wrote[LayerLocal]; got != want {
		t.Errorf("the save wrote:\n%s\nwant:\n%s", got, want)
	}
	mustContain(t, "the page after a save", body, i18n.T("web.profiles.saved"), "fresh")
	mustNotContain(t, "the page after a save", body, i18n.T("web.profiles.error"))
}

// TestSaveBodyDeletesAProfileTheFormNoLongerHolds: a profile whose row the
// person deleted is gone from the file, and the rest of the file is what it was.
func TestSaveBodyDeletesAProfileTheFormNoLongerHolds(t *testing.T) {
	port := demoPort()
	values := formOf(Row{Name: "solo", Kind: "opencode"})
	values.Set("hash", port.file[LayerLocal].Hash)
	if _, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values); err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	want := "version: 1\nprofiles:\n  solo:\n    kind: opencode\n"
	if got := port.wrote[LayerLocal]; got != want {
		t.Errorf("the save wrote %q, want %q", got, want)
	}
}

// TestSaveBodyKeepsTheRowsAndWritesNothingWhenTheyCannotBeRead: a name written
// twice, a profile with no name and an env line that is not name=value are
// refused rather than written: the page comes back with the reason, the rows as
// they were typed, and no file touched.
func TestSaveBodyKeepsTheRowsAndWritesNothingWhenTheyCannotBeRead(t *testing.T) {
	cases := []struct {
		name   string
		rows   []Row
		reason string
	}{
		{
			name:   "two profiles with the same name",
			rows:   []Row{{Name: "impl", Kind: "claude"}, {Name: "impl", Kind: "codex"}},
			reason: i18n.Tf("web.profiles.error.duplicate", "impl"),
		},
		{
			name:   "a profile with no name",
			rows:   []Row{{Name: "", Kind: "claude"}},
			reason: i18n.Tf("web.profiles.error.no_name", 1),
		},
		{
			name:   "an env line with no =",
			rows:   []Row{{Name: "impl", Kind: "claude", Env: "OPENAI_API_KEY"}},
			reason: i18n.Tf("web.profiles.error.env_line", "OPENAI_API_KEY"),
		},
		{
			name:   "two env variables with the same name",
			rows:   []Row{{Name: "impl", Kind: "claude", Env: "FOO=1\nFOO=2"}},
			reason: i18n.Tf("web.profiles.error.env_duplicate", "FOO"),
		},
		{
			name:   "a name the file refuses",
			rows:   []Row{{Name: "Impl Runtime", Kind: "claude"}},
			reason: "profile names",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port := demoPort()
			values := formOf(tc.rows...)
			values.Set("hash", port.file[LayerLocal].Hash)
			body, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values)
			if err != nil {
				t.Fatalf("SaveBody: %v", err)
			}
			if got := port.wrote[LayerLocal]; got != "" {
				t.Errorf("a refused save wrote %q", got)
			}
			mustContain(t, "the page of a refused save", body, tc.reason)
			for _, row := range tc.rows {
				if row.Name != "" && !strings.Contains(body, row.Name) {
					t.Errorf("the page lost the row %q the person typed", row.Name)
				}
			}
		})
	}
}

// TestSaveBodyRefusesAFormWithNoHash: a form that does not carry the version of
// the file that was read is not a form hergy drew, so nothing is written.
func TestSaveBodyRefusesAFormWithNoHash(t *testing.T) {
	port := demoPort()
	values := formOf(Row{Name: "impl", Kind: "claude"})
	values.Del("hash")
	body, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if got := port.wrote[LayerLocal]; got != "" {
		t.Errorf("a save with no hash wrote %q", got)
	}
	mustContain(t, "the page of a save with no hash", body, i18n.T("web.profiles.error.no_hash"))
}

// TestSaveBodyRefusesAFormThatReadAnOlderFile: a form whose hash is not the one
// the file holds now is refused by the write, and the page says what the write
// said. The rows the person typed stay on the page.
func TestSaveBodyRefusesAFormThatReadAnOlderFile(t *testing.T) {
	port := demoPort()
	values := formOf(Row{Name: "impl", Kind: "codex"})
	values.Set("hash", hashOf("version: 1\nprofiles:\n  impl:\n    kind: claude\n    args: [\"--old\"]\n"))
	body, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if got := port.wrote[LayerLocal]; got != "" {
		t.Errorf("a save from an older file wrote %q", got)
	}
	mustContain(t, "the page of a refused write", body,
		i18n.Tf("error.profiles_file_changed", "profiles.yaml"),
		`name="kind" value="codex"`,
	)
}

// TestSaveBodySaysWhenTheFileMovedUnderTheScreen: a file that changed on disk
// since it was read is refused by the Port, and the page says what it said.
func TestSaveBodySaysWhenTheFileMovedUnderTheScreen(t *testing.T) {
	port := demoPort()
	port.writeErr = i18n.Errorf("error.profiles_file_changed", "profiles.yaml")
	values := formOf(Row{Name: "impl", Kind: "claude"})
	values.Set("hash", port.file[LayerLocal].Hash)
	body, err := newScreen(t, port).SaveBody(LayerLocal, "tok", values)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	mustContain(t, "the page of a refused write", body, i18n.Tf("error.profiles_file_changed", "profiles.yaml"))
}
