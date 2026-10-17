package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// These are the handler's own promises (r-c683): the three tabs, a save that
// changes only what was edited, the refusals said with validate's own
// sentences, and a profile in use that is not taken away.

// p2PostJSON runs one JSON-carrying POST and decodes its JSON answer.
func p2PostJSON(t *testing.T, f *fixture, server *Server, target string, form url.Values) (*http.Response, map[string]any) {
	t.Helper()
	resp, body := f.post(t, server, target, form)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("%s did not answer JSON: %v: %s", target, err, body)
	}
	return resp, decoded
}

// TestProfilesTabsAreRealPages: every tab answers a whole page with its own
// address, and the list holds the columns the mock has.
func TestProfilesTabsAreRealPages(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	for _, test := range []struct{ path, want string }{
		{"/profiles", "置き場所"},
		{"/profiles/assign", "チーム定義"},
		{"/profiles/yaml", "validate"},
	} {
		resp, body := f.get(t, server, test.path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", test.path, resp.StatusCode)
		}
		if !strings.Contains(body, test.want) {
			t.Errorf("GET %s does not hold %q", test.path, test.want)
		}
		for _, gone := range []string{"hergy.css", "[[", "層"} {
			if strings.Contains(body, gone) {
				t.Errorf("GET %s holds %q", test.path, gone)
			}
		}
	}
	_, body := f.get(t, server, "/profiles")
	for _, want := range []string{"名前", "kind", "置き場所", "引数", "環境変数", "使っている人", "＋ プロファイルを追加", "プロファイルはありません"} {
		if want == "プロファイルはありません" {
			continue
		}
		if !strings.Contains(body, want) {
			t.Errorf("the list does not hold %q", want)
		}
	}
	if got := strings.Count(body, `<div class="row`); got != 2 {
		t.Errorf("the list holds %d rows, want the fixture's 2", got)
	}
}

// TestProfileSaveChangesOnlyWhatWasEdited: a save writes the kind and the args
// that changed, and every other line — the comment, the order, the shared
// profile's lines — stays as it was.
func TestProfileSaveChangesOnlyWhatWasEdited(t *testing.T) {
	f := newFixture(t)
	hand := "version: 1\n# 手で書いたファイル\n\nprofiles:\n  coder:\n    kind: claude\n    args: [--x]\n    env:\n      A: one\n  planner:\n    kind: codex\n"
	f.write(t, "profiles.yaml", hand)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")

	resp, body := f.post(t, server, "/profiles/save", url.Values{
		"file": {"project"}, "original": {"coder"},
		"profile": {`{"name":"coder","kind":"gemini","args":["--x","--y"],"env":[{"key":"A","value":"one"}]}`},
		"hash":    {source.Hash(hand)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "保存しました") {
		t.Errorf("the save does not say it was saved:\n%s", body)
	}
	got := readFile(t, path)
	for _, want := range []string{"# 手で書いたファイル", "  coder:\n", "    kind: gemini\n", "  planner:\n    kind: codex\n", "A: one"} {
		if !strings.Contains(got, want) {
			t.Errorf("the file does not hold %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, `args: [--x, --y]`) {
		t.Errorf("the args were not written as a flow sequence:\n%s", got)
	}
}

// TestProfileRenameWritesThePeopleToo: a rename with the box ticked writes the
// new name into the members that used it, and leaves every other line of
// team.yaml alone.
func TestProfileRenameWritesThePeopleToo(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	profilesPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	teamPath := f.teamYAML()
	profilesBefore := readFile(t, profilesPath)
	teamBefore := readFile(t, teamPath)

	resp, body := f.post(t, server, "/profiles/save", url.Values{
		"file": {"project"}, "original": {"coder"}, "renameUsers": {"1"},
		"profile": {`{"name":"coder-2","kind":"claude","args":["--x"],"env":[{"key":"A","value":"one"}]}`},
		"hash":    {source.Hash(profilesBefore)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the rename answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, profilesPath); got != strings.Replace(profilesBefore, "  coder:", "  coder-2:", 1) {
		t.Errorf("the rename changed more than the key:\n%s", got)
	}
	want := strings.ReplaceAll(teamBefore, "profile: coder", "profile: coder-2")
	if got := readFile(t, teamPath); got != want {
		t.Errorf("the people were not written:\n%s", got)
	}
}

// TestProfileSaveRefusesAChangedFile: a page that read one text does not write
// over another, and says it with the locale's sentence.
func TestProfileSaveRefusesAChangedFile(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	resp, decoded := p2PostJSON(t, f, server, "/profiles/save", url.Values{
		"file": {"project"}, "original": {"coder"},
		"profile": {`{"name":"coder","kind":"gemini","args":[],"env":[]}`},
		"hash":    {source.Hash("something else")},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("the save answered %d, want 409", resp.StatusCode)
	}
	want := i18n.Tf("error.profiles_file_changed", f.proj+"/.hergy/profiles.yaml")
	if decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if readFile(t, path) != before {
		t.Errorf("the refused save wrote the file")
	}
}

// TestProfileCheckSaysValidatesSentences: the checks a field sends while it is
// being typed are the sentences validate prints, not a screen's own.
func TestProfileCheckSaysValidatesSentences(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	cases := []struct{ draft, want string }{
		{`{"name":"sonnet-5.5","kind":"claude","args":[],"env":[]}`, i18n.Tf("error.profile_name_pattern", "profiles.sonnet-5.5")},
		{`{"name":"coder","kind":"claude","args":[],"env":[{"key":"A","value":"1"},{"key":"A","value":"2"}]}`, i18n.Tf("error.env_key_twice", "profiles.coder", "A")},
	}
	for _, test := range cases {
		resp, decoded := p2PostJSON(t, f, server, "/profiles/check", url.Values{
			"file": {"project"}, "original": {"coder"}, "profile": {test.draft},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the check answered %d", resp.StatusCode)
		}
		problems, _ := decoded["problems"].([]any)
		found := false
		for _, one := range problems {
			problem, _ := one.(map[string]any)
			if problem["text"] == test.want {
				found = true
			}
		}
		if !found {
			t.Errorf("the check did not say %q: %v", test.want, problems)
		}
	}
}

// TestProfileInUseIsNotDeleted: a profile the fixture's team names is refused,
// and the file stays as it was; one nobody names goes.
func TestProfileInUseIsNotDeleted(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n  spare:\n    kind: codex\n")
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)

	resp, decoded := p2PostJSON(t, f, server, "/profiles/delete", url.Values{
		"file": {"project"}, "name": {"coder"}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("deleting a used profile answered %d, want 422", resp.StatusCode)
	}
	if decoded["text"] != i18n.Tf("web.profile.in_use", "coder") {
		t.Errorf("the refusal is %q", decoded["text"])
	}
	if readFile(t, path) != before {
		t.Errorf("the refused delete wrote the file")
	}

	resp, _ = f.post(t, server, "/profiles/delete", url.Values{
		"file": {"project"}, "name": {"spare"}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deleting an unused profile answered %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != "version: 1\nprofiles:\n  coder:\n    kind: claude\n" {
		t.Errorf("the delete took more than the profile:\n%q", got)
	}
}

// TestYAMLSaveWritesTheTypedText: the YAML tab writes the text as typed, and
// refuses a text with a problem.
func TestYAMLSaveWritesTheTypedText(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	text := "version: 1\nprofiles:\n  coder:\n    kind: opencode\n  planner:\n    kind: codex\n"
	resp, _ := f.post(t, server, "/profiles/yaml/save", url.Values{
		"file": {"project"}, "text": {text}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the YAML save answered %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != text {
		t.Errorf("the YAML save wrote %q, want the typed text", got)
	}

	resp, decoded := p2PostJSON(t, f, server, "/profiles/yaml/save", url.Values{
		"file": {"project"}, "text": {"version: 1\nprofiles:\n  bad:\n    kind: \"\"\n"}, "hash": {source.Hash(text)},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a broken text answered %d, want 422", resp.StatusCode)
	}
	if decoded["text"] == "" {
		t.Errorf("the refusal has no sentence")
	}
}

// TestAssignSaveWritesOneProfileLine: one pick writes that member's profile and
// leaves every other line of team.yaml as it was.
func TestAssignSaveWritesOneProfileLine(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	changes, _ := json.Marshal([]map[string]string{{"member": "builder", "profile": "planner"}})
	resp, body := f.post(t, server, "/profiles/assign/save", url.Values{
		"definition": {"demo"}, "changes": {string(changes)}, "hash": {source.Hash(before)},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the assign save answered %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "保存しました") {
		t.Errorf("the card does not say it was saved:\n%s", body)
	}
	want := strings.Replace(before, "  - name: builder\n    role: builder\n    profile: coder", "  - name: builder\n    role: builder\n    profile: planner", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the assign save changed more than the one line:\n%s", got)
	}
}

// TestProfilesPageCarriesTheFixtureScale: the page the screen renders over the
// real-scale fixture holds the numbers the mock does.
func TestProfilesPageCarriesTheFixtureScale(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	for _, test := range []struct{ path, want string }{
		{"/profiles", `<span class="n">11</span>`},
		{"/profiles/assign", `<span class="n">18</span>`},
		{"/profiles/yaml", "OPENCODE_CONFIG_CONTENT"},
	} {
		resp, body := f.get(t, server, test.path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", test.path, resp.StatusCode)
		}
		if !strings.Contains(body, test.want) {
			t.Errorf("GET %s does not hold %q", test.path, test.want)
		}
	}
	_, body := f.get(t, server, "/profiles")
	if got := strings.Count(body, `<div class="row`); got != 11 {
		t.Errorf("the list holds %d rows, want 11", got)
	}
	if !strings.Contains(body, "警告 1") {
		t.Errorf("the shared codex profile does not carry its warning")
	}
}

// fixtureProject copies the real-scale fixture into a working project and an
// outside settings directory, and answers a fixture whose server reads it.
func fixtureProject(t *testing.T) (*fixture, func()) {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "profiles-v2")
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config")
	copyTree(t, filepath.Join(src, "project"), proj)
	copyTree(t, filepath.Join(src, "config", "hergy"), global)
	f := &fixture{root: root, proj: proj, global: global, agents: map[string][]activity.Agent{}}
	f.port = &fakePort{f: f}
	return f, func() {}
}

// copyTree copies a directory as it stands.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
