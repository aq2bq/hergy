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
			Place   string `json:"place"`
			Path    string `json:"path"`
			Hash    string `json:"hash"`
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
	Targets   int    `json:"rename_targets"`
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
	for _, want := range []string{"プロファイル名", "kind", "プロジェクト", "引数", "環境変数", "使っている人", "＋ プロファイルを追加"} {
		if !strings.Contains(body, want) {
			t.Errorf("the list does not hold %q", want)
		}
	}
	data := island(t, body, "profiles-data")
	if got := len(data.Profile.Items); got != 2 {
		t.Errorf("the list carries %d rows, want the fixture's 2", got)
	}
}

// TestPROFILES_P7_TheSaveChangesTheTwoLinesAlone: a save of sonnet-fast with one
// argument and one env value changed grows the real fixture's profiles.yaml by
// exactly those two lines and leaves every other line word for word
// (spec-profiles 2.2).
func TestPROFILES_P7_TheSaveChangesTheTwoLinesAlone(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)

	resp, body := f.api(t, server, http.MethodPut, "/api/profile/sonnet-fast", map[string]any{
		"place": p2Project, "original": "sonnet-fast", "hash": source.Hash(before),
		"doc": map[string]any{"name": "sonnet-fast", "kind": "claude",
			"args": []string{"--model", "claude-sonnet-5-5", "--effort", "high"},
			"env":  []map[string]string{{"key": "MAX_THINKING_TOKENS", "value": "16000"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answered %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	changed := pv2ChangedLines(t, before, after)
	if len(changed) != 2 {
		t.Fatalf("the save changed the lines %v, want the args line and the env line alone", changed)
	}
	for _, n := range changed {
		was, now := pv2LineAt(t, before, n), pv2LineAt(t, after, n)
		switch {
		case strings.Contains(was, "args:") && strings.Contains(now, "claude-sonnet-5-5") && strings.Contains(now, "--effort"):
			// the args line, with the one argument added.
		case strings.Contains(was, "MAX_THINKING_TOKENS") && strings.Contains(now, "16000"):
			// the env line, with the value changed.
		default:
			t.Errorf("line %d changed from %q to %q, which is neither the args nor the env line", n, was, now)
		}
	}
}

// TestPROFILES_P4_TheRenameChangesTheNameAndThePeopleAlone: renaming the used
// fable-review to fable-review-2, with the box ticked, changes the name's line
// in the project's profiles.yaml and the one `profile:` line of the definition
// that named it, and nothing else — the other three team.yaml files come back
// byte for byte (spec-profiles 2.2).
func TestPROFILES_P4_TheRenameChangesTheNameAndThePeopleAlone(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	profilesPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, profilesPath)
	teamsDir := filepath.Join(f.proj, ".hergy", "teams")
	corePath := filepath.Join(teamsDir, "core", "team.yaml")
	coreBefore := readFile(t, corePath)
	untouched := map[string]string{}
	for _, name := range []string{"docs", "migrate", "research"} {
		path := filepath.Join(teamsDir, name, "team.yaml")
		untouched[path] = readFile(t, path)
	}

	resp, body := f.api(t, server, http.MethodPut, "/api/profile/fable-review", map[string]any{
		"place": p2Project, "original": "fable-review", "rename_users": true, "hash": source.Hash(before),
		"doc": map[string]any{"name": "fable-review-2", "kind": "claude",
			"args": []string{"--model", "claude-fable-5-1", "--effort", "high"}, "env": []map[string]string{}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the rename answered %d: %s", resp.StatusCode, body)
	}

	changed := pv2ChangedLines(t, before, readFile(t, profilesPath))
	if len(changed) != 1 || !strings.Contains(pv2LineAt(t, before, changed[0]), "fable-review:") {
		t.Errorf("the name's file changed the lines %v, want the fable-review line alone", changed)
	}

	coreChanged := pv2ChangedLines(t, coreBefore, readFile(t, corePath))
	if len(coreChanged) != 1 {
		t.Fatalf("the definition that named it changed the lines %v, want its profile line alone", coreChanged)
	}
	was, now := pv2LineAt(t, coreBefore, coreChanged[0]), pv2LineAt(t, readFile(t, corePath), coreChanged[0])
	if was != "    profile: fable-review" || now != "    profile: fable-review-2" {
		t.Errorf("the definition's line changed from %q to %q, want the profile renamed", was, now)
	}

	for path, want := range untouched {
		if got := readFile(t, path); got != want {
			t.Errorf("the rename wrote the definition %s:\n%s", path, got)
		}
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
	got, _ := decoded["file"].(map[string]any)
	if got == nil || got["changed"] == nil || got["changed"] == "" {
		t.Fatalf("the refusal carries no file with a changed hour: %v", decoded["file"])
	}
	want := i18n.Tf("error.file_changed", f.proj+"/.hergy/profiles.yaml", got["changed"])
	if decoded["text"] != want {
		t.Errorf("the refusal is %q, want %q", decoded["text"], want)
	}
	if readFile(t, path) != before {
		t.Errorf("the refused save wrote the file")
	}
}

// TestPROFILES_V5_ProfileCheckSaysValidatesSentences: the checks a field sends while it is
// being typed are the sentences validate prints, not a screen's own.
func TestPROFILES_V5_ProfileCheckSaysValidatesSentences(t *testing.T) {
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

// TestPROFILES_F1_TheUsedProfileGoesAndTheAssignShowsTheLoss: deleting the used
// fable-review from the project's profiles.yaml takes that profile's block alone,
// and the 割り当て tab then puts the missing-profile sentence on core's reviewer
// and counts core one problem (spec-profiles 2.2).
func TestPROFILES_F1_TheUsedProfileGoesAndTheAssignShowsTheLoss(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)

	resp, body := f.api(t, server, http.MethodDelete, "/api/profile/fable-review?place="+p2Project+"&hash="+source.Hash(before), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deleting a used profile answered %d: %s", resp.StatusCode, body)
	}
	removed := pv2RemovedBlock(t, before, readFile(t, path))
	for _, want := range []string{"fable-review:", "kind: claude", "claude-fable-5-1"} {
		if !strings.Contains(removed, want) {
			t.Errorf("the delete took away %q, which is not the profile's own block:\n%s", removed, want)
		}
	}

	assign := f.islandOf(t, server, "/profiles/assign")
	core := -1
	for i := range assign.Assign.Teams {
		if assign.Assign.Teams[i].Name == "core" {
			core = i
		}
	}
	if core < 0 {
		t.Fatalf("the assign tab carries no core card: %+v", assign.Assign.Teams)
	}
	if got := assign.Assign.Teams[core].Err; got != 1 {
		t.Errorf("core counts %d problems, want the one the delete left", got)
	}
	want := "members[4].profile: fable-review というプロファイルはありません。profiles.yaml の profiles: にある名前を profile: に書いてください"
	found := false
	for _, row := range assign.Assign.Teams[core].Rows {
		if row.ID == "m-4" && row.Error == want {
			found = true
		}
	}
	if !found {
		t.Errorf("core does not tell the reviewer %q: %+v", want, assign.Assign.Teams[core].Rows)
	}
}

// TestPROFILES_V4_TheYAMLSaveWritesTheTypedTextAndRefusesABrokenOne: the YAML
// tab's save writes the text exactly as typed, and a text with a problem is
// refused with that problem and its line, writing nothing (spec-profiles 2.2,
// spec-boundary 5).
func TestPROFILES_V4_TheYAMLSaveWritesTheTypedTextAndRefusesABrokenOne(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	text := before + "  added-typed:\n    kind: opencode\n"

	resp, body := f.api(t, server, http.MethodPut, "/api/profile/project", map[string]any{
		"place": p2Project, "text": text, "hash": source.Hash(before),
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the YAML save answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); got != text {
		t.Errorf("the YAML save wrote %q, want the typed text", got)
	}

	// A text with a problem: the check names the problem and the line it stands
	// on, and a save of the same text is refused and writes nothing.
	broken := "version: 1\nprofiles:\n  bad:\n    kind: \"\"\n"
	check := pv2CheckBody(t, f, server, map[string]any{"place": p2Project, "text": broken})
	told := false
	for _, problem := range check.Problems {
		if problem.Line == 4 && strings.Contains(problem.Text, "profiles.bad.kind") {
			told = true
		}
	}
	if !told {
		t.Errorf("the check does not name the broken kind and its line 4: %+v", check.Problems)
	}

	resp, body = f.api(t, server, http.MethodPut, "/api/profile/project", map[string]any{
		"place": p2Project, "text": broken, "hash": source.Hash(text),
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a broken text answered %d, want 422: %s", resp.StatusCode, body)
	}
	if got := readFile(t, path); got != text {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// TestPROFILES_P12_TheAssignSaveChangesOneLineAndTheDocsProblem: the 割り当て
// save that changes migrate's impl changes that `profile:` line alone in
// migrate's team.yaml; docs, whose 翻訳 names the missing gemini-flash, is not
// refused by that problem it already had, but is refused when a save would add a
// problem, writing nothing (spec-profiles 2.2).
func TestPROFILES_P12_TheAssignSaveChangesOneLineAndTheDocsProblem(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	migratePath := filepath.Join(f.proj, ".hergy", "teams", "migrate", "team.yaml")
	before := readFile(t, migratePath)

	resp, body := f.api(t, server, http.MethodPut, "/api/team/migrate", map[string]any{
		"place": p2Project, "part": "team", "hash": source.Hash(before),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberProfile", "member": "impl", "profile": "opus-plan"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the assign save answered %d: %s", resp.StatusCode, body)
	}
	changed := pv2ChangedLines(t, before, readFile(t, migratePath))
	if len(changed) != 1 {
		t.Fatalf("the assign save changed the lines %v, want the impl's profile line alone", changed)
	}
	was, now := pv2LineAt(t, before, changed[0]), pv2LineAt(t, readFile(t, migratePath), changed[0])
	if was != "    profile: codex-high" || now != "    profile: opus-plan" {
		t.Errorf("the impl's line changed from %q to %q, want the profile changed", was, now)
	}

	// docs: the problem it already had does not stop a save that adds none.
	docsPath := filepath.Join(f.proj, ".hergy", "teams", "docs", "team.yaml")
	docsBefore := readFile(t, docsPath)
	resp, body = f.api(t, server, http.MethodPut, "/api/team/docs", map[string]any{
		"place": p2Project, "part": "team", "hash": source.Hash(docsBefore),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberProfile", "member": "writer", "profile": "codex-high"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a docs save that adds no problem was answered %d, want the existing 翻訳 problem to pass: %s", resp.StatusCode, body)
	}
	docsAfter := readFile(t, docsPath)
	if !strings.Contains(docsAfter, "    profile: codex-high\n") {
		t.Errorf("the docs save did not write the writer's profile:\n%s", docsAfter)
	}

	// docs: a save that would add a problem is refused, writing nothing.
	resp, body = f.api(t, server, http.MethodPut, "/api/team/docs", map[string]any{
		"place": p2Project, "part": "team", "hash": source.Hash(docsAfter),
		"doc": map[string]any{"ops": []map[string]any{{"kind": "memberProfile", "member": "checker", "profile": "nosuch-profile"}}},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a docs save that adds a problem answered %d, want 422: %s", resp.StatusCode, body)
	}
	if got := readFile(t, docsPath); got != docsAfter {
		t.Errorf("a refused save wrote the file:\n%s", got)
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
	if project != nil && project.Targets != 3 {
		t.Errorf("the project's place counts %d readers of its file, want 3", project.Targets)
	}
	if shared == nil {
		t.Fatalf("the row carries no shared place")
	}
	if shared.Targets != 0 {
		t.Errorf("the shared place counts %d readers it does not have", shared.Targets)
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

// TestTheAssignCardSavesThePlaceItStandsIn: a definition that is only in the
// global layer stands there — the card carries that place and that file's hash —
// and its save writes the global file, which every project reading the name
// changes with it (d-0f32, r-f8b9). The project's layer gets nothing.
func TestTheAssignCardSavesThePlaceItStandsIn(t *testing.T) {
	f := newFixture(t)
	shared := filepath.Join(f.global, "teams", "shared", "team.yaml")
	if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "version: 1\nmission: 共通のチーム\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> human\n"
	if err := os.WriteFile(shared, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)

	assign := f.islandOf(t, server, "/profiles/assign")
	card := -1
	for i := range assign.Assign.Teams {
		if assign.Assign.Teams[i].Name == "shared" {
			card = i
		}
	}
	if card < 0 {
		t.Fatalf("the assign tab carries no shared card: %+v", assign.Assign.Teams)
	}
	if got := assign.Assign.Teams[card].Place; got != "global" {
		t.Fatalf("shared's card stands in %q, want global", got)
	}
	if got := assign.Assign.Teams[card].Hash; got != source.Hash(text) {
		t.Errorf("shared's card carries %q, want the global file's hash", got)
	}

	resp, body := f.api(t, server, http.MethodPut, "/api/team/shared", map[string]any{
		"place": "global", "hash": source.Hash(text),
		"doc": map[string]any{
			"ops": []map[string]string{{"kind": "memberProfile", "member": "lead", "profile": "planner"}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save in the card's place answered %d: %s", resp.StatusCode, body)
	}
	if got := readFile(t, shared); !strings.Contains(got, "profile: planner\n") {
		t.Errorf("the save did not write the global file:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "shared", "team.yaml")); !os.IsNotExist(err) {
		t.Errorf("the save made a project definition of the global name: %v", err)
	}
}

// TestPROFILES_R430B_ADeleteWorksForEveryShapeTheFileWrites: DELETE
// /api/profile takes one profile out of a profiles.yaml written the two ways the
// line edit could not rewrite — the last profile of a block mapping, and a flow
// mapping on the key's own line — and answers the list with the profile gone. The
// file keeps every line the delete did not have to change, and no sentence about
// hergy's own bug is said (ac-adb6, r-430b).
func TestPROFILES_R430B_ADeleteWorksForEveryShapeTheFileWrites(t *testing.T) {
	cases := []struct{ name, text, target, want string }{
		{
			name:   "the only profile of a block mapping",
			text:   "version: 1\nprofiles:\n  coder:\n    kind: claude\n",
			target: "coder",
			want:   "version: 1\nprofiles: {}\n",
		},
		{
			name:   "the only profile, with a document start and a comment",
			text:   "---\nversion: 1\nprofiles: # 使うエンジン\n  coder:\n    kind: claude\n",
			target: "coder",
			want:   "---\nversion: 1\nprofiles: {} # 使うエンジン\n",
		},
		{
			name:   "a flow mapping on the key's line",
			text:   "version: 1\nprofiles: {coder: {kind: claude}, planner: {kind: codex}}\n",
			target: "coder",
			want:   "version: 1\nprofiles: {planner: {kind: codex}}\n",
		},
		{
			name:   "the last of a flow mapping",
			text:   "version: 1\nprofiles: {planner: {kind: codex}, coder: {kind: claude}}\n",
			target: "coder",
			want:   "version: 1\nprofiles: {planner: {kind: codex}}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.write(t, "profiles.yaml", tc.text)
			server := f.server(t)
			path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
			resp, decoded := f.apiJSON(t, server, http.MethodDelete,
				"/api/profile/"+tc.target+"?place=project&hash="+source.Hash(tc.text), nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("the delete answered %d: %v", resp.StatusCode, decoded)
			}
			if got := readFile(t, path); got != tc.want {
				t.Errorf("the delete wrote\n%q\nwant\n%q", got, tc.want)
			}
			if names := pv2ItemNames(t, decoded); names[tc.target] {
				t.Errorf("the list the delete answered still holds %s: %v", tc.target, names)
			}
		})
	}
}

// pv2ItemNames is the names one list answer holds.
func pv2ItemNames(t *testing.T, decoded map[string]any) map[string]bool {
	t.Helper()
	raw, err := json.Marshal(decoded["items"])
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item.Name] = true
	}
	return out
}
