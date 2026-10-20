package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// These are the boundary's own promises (r-2cc2, spec-boundary 5): the three
// tabs as pages, a list and a save that answer JSON, the refusals said with
// validate's own sentences, and a profile in use that is not taken away. Every
// read and write goes through /api/, and the pages carry the server's reading as
// one island of JSON.

// api runs one call through the /api/ boundary with the token of the run on the
// header, and answers the raw body.
func (f *fixture) api(t *testing.T, server *Server, method, target string, payload any) (*http.Response, string) {
	t.Helper()
	server.addr = "127.0.0.1:8787"
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1:8787"
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if server.token != "" {
		req.Header.Set("X-Hergy-Token", server.token)
	}
	rec := httptest.NewRecorder()
	server.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	out, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(out)
}

// apiJSON runs one call and decodes the JSON answer.
func (f *fixture) apiJSON(t *testing.T, server *Server, method, target string, payload any) (*http.Response, map[string]any) {
	t.Helper()
	resp, body := f.api(t, server, method, target, payload)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("%s %s did not answer JSON: %v: %s", method, target, err, body)
	}
	return resp, decoded
}

// p2PageIsland is the reading the profile page carries inline, in the shape the
// screen's script reads it (spec-boundary 5): the profile kind's list and, on
// the 割り当て tab, the assignment cards. Only the parts the tests look at.
type p2PageIsland struct {
	Profile struct {
		Items  []p2TestItem      `json:"items"`
		Hashes map[string]string `json:"hashes"`
		Files  []p2TestYAMLFile  `json:"files"`
	} `json:"profile"`
	Assign struct {
		Empty bool `json:"empty"`
		Teams []struct {
			Name    string `json:"name"`
			Err     int    `json:"err"`
			Members int    `json:"members"`
			Workers int    `json:"workers"`
			Rows    []struct {
				ID      string `json:"id"`
				Missing bool   `json:"missing"`
				Error   string `json:"error"`
				Profile string `json:"profile"`
			} `json:"rows"`
		} `json:"teams"`
	} `json:"assign"`
}

// p2TestItem is one item of the profile list as the boundary carries it.
type p2TestItem struct {
	Name      string                 `json:"name"`
	Effective string                 `json:"effective"`
	Hidden    []string               `json:"hidden"`
	Places    map[string]p2TestPlace `json:"places"`
	Summary   struct {
		Kind      string   `json:"kind"`
		KindClass string   `json:"kindClass"`
		Args      string   `json:"args"`
		Env       []string `json:"env"`
		More      int      `json:"more"`
		Users     int      `json:"users"`
		Roles     string   `json:"roles"`
	} `json:"summary"`
	Problems struct {
		Err  int `json:"err"`
		Warn int `json:"warn"`
	} `json:"problems"`
}

// p2TestPlace is one place's reading of one item.
type p2TestPlace struct {
	Doc struct {
		Name string   `json:"name"`
		Kind string   `json:"kind"`
		Args []string `json:"args"`
	} `json:"doc"`
	Path      string `json:"path"`
	Hash      string `json:"hash"`
	Readers   int    `json:"readers"`
	SavedLine bool   `json:"savedLine"`
	Problems  struct {
		Err  int `json:"err"`
		Warn int `json:"warn"`
	} `json:"problems"`
}

// p2TestYAMLFile is one file of the profile list, as the YAML tab reads it.
type p2TestYAMLFile struct {
	Place    string `json:"place"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Text     string `json:"text"`
	Hash     string `json:"hash"`
	Problems []struct {
		Sev  string `json:"sev"`
		At   string `json:"at"`
		Line int    `json:"line"`
		Text string `json:"text"`
	} `json:"problems"`
}

// island reads the JSON one screen's page carries in its script element.
func island(t *testing.T, body, id string) p2PageIsland {
	t.Helper()
	marker := `<script id="` + id + `" type="application/json">`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("the page carries no %s island:\n%s", id, body)
	}
	rest := body[i+len(marker):]
	before, _, ok := strings.Cut(rest, "</script>")
	if !ok {
		t.Fatalf("the %s island is not closed", id)
	}
	var decoded p2PageIsland
	if err := json.Unmarshal([]byte(strings.TrimSpace(before)), &decoded); err != nil {
		t.Fatalf("the %s island is not JSON: %v: %s", id, err, before)
	}
	return decoded
}

// islandOf runs one GET and reads the profile page's island.
func (f *fixture) islandOf(t *testing.T, server *Server, target string) p2PageIsland {
	t.Helper()
	resp, body := f.get(t, server, target)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s answers %d", target, resp.StatusCode)
	}
	return island(t, body, "profiles-data")
}

// TestProfilesTabsAreRealPages: every tab answers a whole page with its own
// address, and the list's reading carries the rows the fixture has.
func TestProfilesTabsAreRealPages(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	for _, tab := range []string{"profiles", "assign", "yaml"} {
		path := "/profiles"
		if tab != "profiles" {
			path = "/profiles/" + tab
		}
		resp, body := f.get(t, server, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, resp.StatusCode)
		}
		if !strings.Contains(body, `data-tab="`+tab+`"`) {
			t.Errorf("GET %s does not say which tab it is", path)
		}
		for _, gone := range []string{"hergy.css", "[[", "層"} {
			if strings.Contains(body, gone) {
				t.Errorf("GET %s holds %q", path, gone)
			}
		}
	}
	_, body := f.get(t, server, "/profiles")
	for _, want := range []string{"名前", "kind", "置き場所", "引数", "環境変数", "使っている人", "＋ プロファイルを追加"} {
		if !strings.Contains(body, want) {
			t.Errorf("the list does not hold %q", want)
		}
	}
	data := island(t, body, "profiles-data")
	if got := len(data.Profile.Items); got != 2 {
		t.Errorf("the list carries %d rows, want the fixture's 2", got)
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

	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/profile/coder", map[string]any{
		"place": "project", "original": "coder", "hash": source.Hash(hand),
		"doc": map[string]any{"name": "coder", "kind": "gemini", "args": []string{"--x", "--y"}, "env": []map[string]string{{"key": "A", "value": "one"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d: %v", resp.StatusCode, decoded)
	}
	items, _ := decoded["items"].([]any)
	found := false
	for _, one := range items {
		item, _ := one.(map[string]any)
		if item["name"] != "coder" {
			continue
		}
		if places, ok := item["places"].(map[string]any); ok {
			if place, ok := places["project"].(map[string]any); ok && place["savedLine"] == true {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("the answer does not carry the written place with its saved line: %v", decoded)
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

	resp, body := f.api(t, server, http.MethodPut, "/api/profile/coder", map[string]any{
		"place": "project", "original": "coder", "rename_users": true, "hash": source.Hash(profilesBefore),
		"doc": map[string]any{"name": "coder-2", "kind": "claude", "args": []string{"--x"}, "env": []map[string]string{{"key": "A", "value": "one"}}},
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
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/profile/coder", map[string]any{
		"place": "project", "original": "coder", "hash": source.Hash("something else"),
		"doc": map[string]any{"name": "coder", "kind": "gemini", "args": []string{}, "env": []map[string]string{}},
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
	cases := []struct {
		doc  map[string]any
		want string
		at   string
	}{
		{map[string]any{"name": "sonnet-5.5", "kind": "claude", "args": []string{}, "env": []map[string]string{}}, i18n.Tf("error.profile_name_pattern", "profiles.sonnet-5.5"), "name"},
		{map[string]any{"name": "coder", "kind": "claude", "args": []string{}, "env": []map[string]string{{"key": "A", "value": "1"}, {"key": "A", "value": "2"}}}, i18n.Tf("error.env_key_twice", "profiles.coder", "A"), "env"},
	}
	for _, test := range cases {
		resp, decoded := f.apiJSON(t, server, http.MethodPost, "/api/profile/check", map[string]any{
			"place": "project", "original": "coder", "doc": test.doc,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the check answered %d", resp.StatusCode)
		}
		problems, _ := decoded["problems"].([]any)
		found, placed := false, false
		for _, one := range problems {
			problem, _ := one.(map[string]any)
			if problem["text"] == test.want {
				found = true
				if at, _ := problem["at"].(string); strings.HasPrefix(at, test.at) {
					placed = true
				}
			}
		}
		if !found {
			t.Errorf("the check did not say %q: %v", test.want, problems)
		}
		if !placed {
			t.Errorf("the check did not stand %q on its field %q: %v", test.want, test.at, problems)
		}
	}
}

// TestProfileInUseIsDeletedAfterConfirm: a profile a team names goes once the
// person has seen the count (J1), and the assign tab then shows the people it
// left naming nothing.
func TestProfileInUseIsDeletedAfterConfirm(t *testing.T) {
	f := newFixture(t)
	f.write(t, "profiles.yaml", "version: 1\nprofiles:\n  coder:\n    kind: claude\n  planner:\n    kind: codex\n")
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)

	resp, body := f.api(t, server, http.MethodDelete, "/api/profile/coder?place=project&hash="+source.Hash(before), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deleting a used profile answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); got != "version: 1\nprofiles:\n  planner:\n    kind: codex\n" {
		t.Errorf("the delete took more than the profile:\n%q", got)
	}
	assign := f.islandOf(t, server, "/profiles/assign")
	if len(assign.Assign.Teams) != 1 {
		t.Fatalf("the assign tab carries %d teams, want 1", len(assign.Assign.Teams))
	}
	team := assign.Assign.Teams[0]
	if team.Err != 4 {
		t.Errorf("the card counts %d people the delete left, want 4", team.Err)
	}
	missing := false
	for _, row := range team.Rows {
		if strings.Contains(row.Error, "members[0].profile: coder というプロファイルはありません") {
			missing = true
		}
	}
	if !missing {
		t.Errorf("the assign tab does not show the people left without a profile: %+v", team.Rows)
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
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/profile/project", map[string]any{
		"place": "project", "text": text, "hash": source.Hash(before),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the YAML save answered %d: %v", resp.StatusCode, decoded)
	}
	if decoded["hash"] != source.Hash(text) {
		t.Errorf("the YAML save answers hash %v, want the written text's", decoded["hash"])
	}
	if got := readFile(t, path); got != text {
		t.Errorf("the YAML save wrote %q, want the typed text", got)
	}

	resp, decoded = f.apiJSON(t, server, http.MethodPut, "/api/profile/project", map[string]any{
		"place": "project", "text": "version: 1\nprofiles:\n  bad:\n    kind: \"\"\n", "hash": source.Hash(text),
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
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberProfile", "member": "builder", "profile": "planner"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the assign save answered %d: %v", resp.StatusCode, decoded)
	}
	if decoded["hash"] != source.Hash(readFile(t, path)) {
		t.Errorf("the card's answer does not carry the written file's hash")
	}
	want := strings.Replace(before, "  - name: builder\n    role: builder\n    profile: coder", "  - name: builder\n    role: builder\n    profile: planner", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the assign save changed more than the one line:\n%s", got)
	}
}

// TestAssignSaveKeepsTheProfilesItWasNotToldAbout: an op that carries a role and
// no profile leaves the profiles alone, so the role screen's save cannot blank
// them (spec-boundary 6).
func TestAssignSaveKeepsTheProfilesItWasNotToldAbout(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberRole", "member": "builder", "role": "lead"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the role-only save answered %d: %s", resp.StatusCode, body)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "  - name: builder\n    role: lead\n    profile: coder\n") {
		t.Errorf("the role-only save changed the profile as well:\n%s", got)
	}
}

// TestAssignSaveTakesThePicksAsOps: the picks the assign tabs send as the team
// kind's own ops write the one line each names, and no other (r-8e85).
func TestAssignSaveTakesThePicksAsOps(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "hash": source.Hash(before),
		"doc": map[string]any{
			"ops": []map[string]string{{"kind": "memberProfile", "member": "builder", "profile": "planner"}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the ops save answered %d: %v", resp.StatusCode, decoded)
	}
	if decoded["hash"] != source.Hash(readFile(t, path)) {
		t.Errorf("the answer does not carry the written file's hash")
	}
	want := strings.Replace(before, "  - name: builder\n    role: builder\n    profile: coder", "  - name: builder\n    role: builder\n    profile: planner", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the ops write changed more than the one line:\n%s", got)
	}
}

// TestAssignSaveTakesARoleOp: a role pick is one op too, and the profiles of the
// members it does not name stay as the file had them (r-8e85).
func TestAssignSaveTakesARoleOp(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "hash": source.Hash(before),
		"doc": map[string]any{
			"ops": []map[string]string{{"kind": "memberRole", "member": "builder", "role": "lead"}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the role op answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); !strings.Contains(got, "  - name: builder\n    role: lead\n    profile: coder\n") {
		t.Errorf("the role op did not write the one role line:\n%s", got)
	}
}

// TestAssignSaveRefusesAProfileTheTeamDoesNotRead: a picked profile that is not
// one the definition reads is refused, whether the pick came as an op or a row
// (r-8e85).
func TestAssignSaveRefusesAProfileTheTeamDoesNotRead(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "hash": source.Hash(before),
		"doc": map[string]any{
			"ops": []map[string]string{{"kind": "memberProfile", "member": "builder", "profile": "nosuch"}},
		},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a pick of a profile the team does not read answered %d, want 422: %v", resp.StatusCode, decoded)
	}
	if got := readFile(t, path); got != before {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// TestAssignSaveRefusesAnOpTheTeamDoesNotKnow: an op the team kind does not know
// is refused rather than written (r-8e85).
func TestAssignSaveRefusesAnOpTheTeamDoesNotKnow(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, _ := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "hash": source.Hash(before),
		"doc": map[string]any{
			"ops": []map[string]string{{"kind": "nosuch", "text": "別の使命"}},
		},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("an op the team kind does not know answered %d, want 422", resp.StatusCode)
	}
	if got := readFile(t, path); got != before {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// TestAssignSaveKeepsTheRolesItWasNotToldAbout: a doc that carries a role and no
// profile leaves the profiles alone, so the role screen's save cannot blank them
// (spec-boundary 6).
func TestAssignSaveKeepsTheRolesItWasNotToldAbout(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := f.teamYAML()
	before := readFile(t, path)
	resp, body := f.api(t, server, http.MethodPut, "/api/team/demo", map[string]any{
		"place": "project", "hash": source.Hash(before),
		"doc": map[string]any{
			"name":    "demo",
			"members": []map[string]string{{"name": "builder", "role": "lead"}},
			"workers": []map[string]string{},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the role-only save answered %d: %s", resp.StatusCode, body)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "  - name: builder\n    role: lead\n    profile: coder\n") {
		t.Errorf("the role-only save changed the profile as well:\n%s", got)
	}
}

// TestProfilesPageCarriesTheFixtureScale: the page the screen renders over the
// real-scale fixture reads the numbers the mock has.
func TestProfilesPageCarriesTheFixtureScale(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)

	profiles := f.islandOf(t, server, "/profiles")
	if len(profiles.Profile.Items) != 11 {
		t.Errorf("the list reads %d rows, want 11", len(profiles.Profile.Items))
	}
	warned := false
	for _, row := range profiles.Profile.Items {
		if row.Name == "codex" && row.Problems.Warn == 1 {
			warned = true
		}
	}
	if !warned {
		t.Errorf("the shared codex profile does not carry its warning")
	}

	assign := f.islandOf(t, server, "/profiles/assign")
	people := 0
	for _, team := range assign.Assign.Teams {
		people += team.Members + team.Workers
	}
	if people != 18 {
		t.Errorf("the 割り当て tab reads %d people, want 18", people)
	}
	if len(assign.Assign.Teams) != 4 {
		t.Errorf("the assign tab reads %d cards, want 4", len(assign.Assign.Teams))
	}

	yaml := f.islandOf(t, server, "/profiles/yaml")
	opencode := false
	for _, file := range yaml.Profile.Files {
		if strings.Contains(file.Text, "OPENCODE_CONFIG_CONTENT") {
			opencode = true
		}
	}
	if !opencode {
		t.Errorf("the YAML tab does not carry the shared file's text")
	}
}

// TestMissingProfilesFileStartsAtVersionOne is E1: a file that is not there
// reads as version: 1 alone, which validate reads and an add can grow.
func TestMissingProfilesFileStartsAtVersionOne(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	yaml := f.islandOf(t, server, "/profiles/yaml")
	if len(yaml.Profile.Files) == 0 || yaml.Profile.Files[0].Text != "version: 1\n" {
		t.Fatalf("the missing file does not read as version: 1: %+v", yaml.Profile.Files)
	}
	resp, body := f.api(t, server, http.MethodPut, "/api/profile/fresh", map[string]any{
		"place": "project", "original": "", "hash": source.Hash(""),
		"doc": map[string]any{"name": "fresh", "kind": "claude", "args": []string{}, "env": []map[string]string{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("making the file answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); !strings.HasPrefix(got, "version: 1\nprofiles:\n  fresh:\n    kind: claude\n") {
		t.Errorf("the made file reads:\n%s", got)
	}
}

// TestProfileEnvKeepsTheScreenOrder is R7: the rows are written in the order a
// person put them, new names after the ones the file already had.
func TestProfileEnvKeepsTheScreenOrder(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	resp, body := f.api(t, server, http.MethodPut, "/api/profile/coder", map[string]any{
		"place": "project", "original": "coder", "hash": source.Hash(before),
		"doc": map[string]any{"name": "coder", "kind": "claude", "args": []string{"--x"},
			"env": []map[string]string{{"key": "B_KEY", "value": "2"}, {"key": "A", "value": "one"}, {"key": "A_KEY", "value": "1"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the env save answered %d: %s", resp.StatusCode, body)
	}
	got := readFile(t, path)
	b, a, aKey := strings.Index(got, "B_KEY"), strings.Index(got, "A: one"), strings.Index(got, "A_KEY")
	if b < 0 || a < 0 || aKey < 0 || b >= a || a >= aKey {
		t.Errorf("the env is not in the screen's order (B=%d A=%d A_KEY=%d):\n%s", b, a, aKey, got)
	}
}

// TestProfilesPageNamesTheFileAndItsReaders is R1・R2: every panel says the file
// it writes, and a rename on a name in both files counts the readers of that
// panel's file (not zero because the other file has the name too).
func TestProfilesPageNamesTheFileAndItsReaders(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	data := f.islandOf(t, server, "/profiles?open=sonnet-fast")
	projectPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	var project, shared *p2TestPlace
	for i := range data.Profile.Items {
		if data.Profile.Items[i].Name != "sonnet-fast" {
			continue
		}
		for name, place := range data.Profile.Items[i].Places {
			one := place
			switch name {
			case "project":
				project = &one
			case "global":
				shared = &one
			}
		}
	}
	if project == nil || project.Path != projectPath {
		t.Errorf("the project's place does not name its file: %+v", project)
	}
	if project != nil && project.Readers != 3 {
		t.Errorf("the project's place counts %d readers of its file, want 3", project.Readers)
	}
	if shared == nil {
		t.Fatalf("the row carries no shared place")
	}
	if shared.Readers != 0 {
		t.Errorf("the shared place counts %d readers it does not have", shared.Readers)
	}
}

// TestRenameThatSplitsAnswersWithBothRows is N1: renaming the project's profile
// while the shared file keeps the old name answers with both rows, in the order
// a reload shows them.
func TestRenameThatSplitsAnswersWithBothRows(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/profile/sonnet-fast", map[string]any{
		"place": "project", "original": "sonnet-fast", "hash": source.Hash(before),
		"doc": map[string]any{"name": "sonnet-fast2", "kind": "claude", "args": []string{"--model", "claude-sonnet-5-5"},
			"env": []map[string]string{{"key": "MAX_THINKING_TOKENS", "value": "8000"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the rename answered %d: %v", resp.StatusCode, decoded)
	}
	items, _ := decoded["items"].([]any)
	if len(items) != 12 {
		t.Fatalf("the answer carries %d rows, want the split's 12", len(items))
	}
	names := make([]string, 0, len(items))
	for _, one := range items {
		item, _ := one.(map[string]any)
		name, _ := item["name"].(string)
		names = append(names, name)
	}
	at := func(name string) int {
		for i, got := range names {
			if got == name {
				return i
			}
		}
		return -1
	}
	newRow, lastProject, oldRow, firstShared := at("sonnet-fast2"), at("gemini-pro"), at("sonnet-fast"), at("opus-plan")
	if newRow < 0 || oldRow < 0 || newRow >= lastProject || lastProject >= oldRow || oldRow >= firstShared {
		t.Errorf("the split rows are not in the reload's order (%v)", names)
	}
	if got := readFile(t, path); !strings.Contains(got, "  sonnet-fast2:\n") {
		t.Errorf("the project's file was not renamed:\n%s", got)
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
	f := &fixture{root: root, proj: proj, global: global}
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
