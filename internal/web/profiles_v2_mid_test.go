package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
)

// This file holds the profile screen's 中 items whose promise stands at the
// 受け口 rather than on a page: the list's own reading of one row (S3), the one
// profile whose env is a whole JSON (T5), the three ways an item is added (P8),
// the sentences a name that cannot be written is told with (V3), and the paths
// a person reads when the shared file lives under the home directory (V7). The
// items whose walk this file's helpers' own file already held stand in
// profiles_v2_test.go, renamed to their new IDs.

// pv2ListBody reads one GET as a profile list.
func pv2ListBody(t *testing.T, f *fixture, server *Server, target string) apiList {
	t.Helper()
	resp, body := f.api(t, server, http.MethodGet, target, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s answers %d: %s", target, resp.StatusCode, body)
	}
	var out apiList
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("GET %s did not answer a list: %v: %s", target, err, body)
	}
	return out
}

// pv2ProfileBody reads one GET as one profile in one place.
func pv2ProfileBody(t *testing.T, f *fixture, server *Server, target string) apiProfileGet {
	t.Helper()
	resp, body := f.api(t, server, http.MethodGet, target, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s answers %d: %s", target, resp.StatusCode, body)
	}
	var out apiProfileGet
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("GET %s did not answer one profile: %v: %s", target, err, body)
	}
	return out
}

// pv2CheckBody reads one check's answer.
func pv2CheckBody(t *testing.T, f *fixture, server *Server, in map[string]any) apiCheck {
	t.Helper()
	resp, body := f.api(t, server, http.MethodPost, "/api/profile/check", in)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answered %d: %s", resp.StatusCode, body)
	}
	var out apiCheck
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("the check did not answer problems: %v: %s", err, body)
	}
	return out
}

// pv2ChangedLines holds two texts of one file against each other and answers the
// lines that differ, by their number in the file. It stops when the two do not
// have the same lines, so a line added or taken away is never counted as one
// changed: the promise it holds is "these lines and no other".
func pv2ChangedLines(t *testing.T, before, after string) []int {
	t.Helper()
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(a) != len(b) {
		t.Fatalf("the file has %d lines before and %d after; want the same lines with some changed", len(a), len(b))
	}
	var out []int
	for i := range a {
		if a[i] != b[i] {
			out = append(out, i+1)
		}
	}
	return out
}

// pv2LineAt is one numbered line of a text, so a test can say what the change
// was rather than only that something changed.
func pv2LineAt(t *testing.T, text string, n int) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	if n < 1 || n > len(lines) {
		t.Fatalf("the text has no line %d", n)
	}
	return lines[n-1]
}

// pv2InsertedBlock is the one block of lines the after has and the before does
// not, with the rest of both left word for word. It stops when the change is not
// one clean insertion, so "the item and no other line" is held whole.
func pv2InsertedBlock(t *testing.T, before, after string) string {
	t.Helper()
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	if p+s != len(a) {
		t.Fatalf("the change is not one block of inserted lines:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	return strings.Join(b[p:len(b)-s], "\n")
}

// pv2RemovedBlock is the one block of lines the before has and the after does
// not, with the rest of both left word for word.
func pv2RemovedBlock(t *testing.T, before, after string) string {
	t.Helper()
	a, b := strings.Split(before, "\n"), strings.Split(after, "\n")
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	if p+s != len(b) {
		t.Fatalf("the change is not one block of removed lines:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	return strings.Join(a[p:len(a)-s], "\n")
}

// TestPROFILES_S3_TheListRowIsOneAndTheProjectIsInEffect: over the real fixture
// GET /api/profile carries sonnet-fast as one row, the project's place in
// effect and the shared one hidden, because the project's file writes the name
// and the shared file writes it too (spec-profiles 1.2).
func TestPROFILES_S3_TheListRowIsOneAndTheProjectIsInEffect(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)

	list := pv2ListBody(t, f, server, "/api/profile")
	rows := 0
	var row *apiItem
	for i := range list.Items {
		if list.Items[i].Name == "sonnet-fast" {
			rows++
			row = &list.Items[i]
		}
	}
	if rows != 1 {
		t.Fatalf("the list carries %d sonnet-fast rows, want 1", rows)
	}
	if row.Effective != p2Project {
		t.Errorf("sonnet-fast is in effect at %q, want %q", row.Effective, p2Project)
	}
	if len(row.Hidden) != 1 || row.Hidden[0] != p2Shared {
		t.Errorf("sonnet-fast hides %v, want the shared place alone", row.Hidden)
	}
}

// TestPROFILES_T5_TheEnvValueIsTheWholeJSON: GET /api/profile/opencode-deepseek
// carries its env value as it stands — the whole 2022-character JSON, not a
// value hidden behind dots (spec-profiles 1.3).
func TestPROFILES_T5_TheEnvValueIsTheWholeJSON(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)

	one := pv2ProfileBody(t, f, server, "/api/profile/opencode-deepseek?place=project")
	var value string
	for _, pair := range one.Doc.Env {
		if pair.Key == "OPENCODE_CONFIG_CONTENT" {
			value = pair.Value
		}
	}
	if value == "" {
		t.Fatalf("opencode-deepseek carries no OPENCODE_CONFIG_CONTENT: %+v", one.Doc.Env)
	}
	if !strings.Contains(value, `"$schema"`) {
		t.Errorf("the env value is not the JSON: %q", value)
	}
	if got := utf8.RuneCountInString(value); got != 2022 {
		t.Errorf("the env value is %d characters, want the whole 2022", got)
	}
	if strings.ContainsAny(value, "•\u2026") {
		t.Errorf("the env value is hidden behind dots: %q", value)
	}
}

// TestPROFILES_P8_AddAndDuplicateAndCopyAddTheItemAlone: an add and a duplicate
// through /api/profile grow the profiles.yaml of the place they stand in, and a
// copy grows the other place's file; only the item's own lines are added, and
// every other line of either file comes back word for word (spec-profiles 2.2).
func TestPROFILES_P8_AddAndDuplicateAndCopyAddTheItemAlone(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	projectPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	globalPath := filepath.Join(f.global, "profiles.yaml")
	projectBefore := readFile(t, projectPath)
	globalBefore := readFile(t, globalPath)

	// 追加: one new name in the project's own file.
	resp, body := f.api(t, server, http.MethodPut, "/api/profile/added-one", map[string]any{
		"place": p2Project, "original": "", "hash": source.Hash(projectBefore),
		"doc": map[string]any{"name": "added-one", "kind": "claude",
			"args": []string{"--model", "claude-haiku-5-5"}, "env": []map[string]string{{"key": "K", "value": "v"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the add answered %d: %s", resp.StatusCode, body)
	}
	projectAfterAdd := readFile(t, projectPath)
	block := pv2InsertedBlock(t, projectBefore, projectAfterAdd)
	for _, want := range []string{"added-one:", "kind: claude", "claude-haiku-5-5", "K: v"} {
		if !strings.Contains(block, want) {
			t.Errorf("the added item does not hold %q:\n%s", want, block)
		}
	}
	if got := readFile(t, globalPath); got != globalBefore {
		t.Errorf("the add wrote the other place's file:\n%s", got)
	}

	// 複製: the item copied under a new name, in the same place.
	resp, body = f.api(t, server, http.MethodPut, "/api/profile/sonnet-fast-2", map[string]any{
		"place": p2Project, "original": "", "hash": source.Hash(projectAfterAdd),
		"doc": map[string]any{"name": "sonnet-fast-2", "kind": "claude",
			"args": []string{"--model", "claude-sonnet-5-5"}, "env": []map[string]string{{"key": "MAX_THINKING_TOKENS", "value": "8000"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the duplicate answered %d: %s", resp.StatusCode, body)
	}
	projectAfterDup := readFile(t, projectPath)
	block = pv2InsertedBlock(t, projectAfterAdd, projectAfterDup)
	for _, want := range []string{"sonnet-fast-2:", "kind: claude", "claude-sonnet-5-5", "MAX_THINKING_TOKENS"} {
		if !strings.Contains(block, want) {
			t.Errorf("the duplicated item does not hold %q:\n%s", want, block)
		}
	}
	if got := readFile(t, globalPath); got != globalBefore {
		t.Errorf("the duplicate wrote the other place's file:\n%s", got)
	}

	// もう一方へのコピー: the project's codex-high also stands in the shared file.
	resp, body = f.api(t, server, http.MethodPut, "/api/profile/codex-high", map[string]any{
		"place": p2Shared, "original": "", "hash": source.Hash(globalBefore),
		"doc": map[string]any{"name": "codex-high", "kind": "codex",
			"args": []string{"-m", "gpt-5.5-codex"}, "env": []map[string]string{{"key": "CODEX_HOME", "value": "~/.codex-team"}}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the copy answered %d: %s", resp.StatusCode, body)
	}
	globalAfterCopy := readFile(t, globalPath)
	block = pv2InsertedBlock(t, globalBefore, globalAfterCopy)
	for _, want := range []string{"codex-high:", "kind: codex", "gpt-5.5-codex", "CODEX_HOME"} {
		if !strings.Contains(block, want) {
			t.Errorf("the copied item does not hold %q:\n%s", want, block)
		}
	}
	if got := readFile(t, projectPath); got != projectAfterDup {
		t.Errorf("the copy wrote the other place's file:\n%s", got)
	}
}

// TestPROFILES_V3_ANameThatCannotBeWritten: an empty name and sonnet-5.5 are
// told on the name's own field with validate's sentence, and a save of either
// is refused, writing nothing (spec-profiles 1.6).
func TestPROFILES_V3_ANameThatCannotBeWritten(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	projectPath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, projectPath)

	cases := []struct {
		name string
		want string
	}{
		{"", "profiles.: プロファイル名は [a-z][a-z0-9_-]{0,31} の形にしてください"},
		{"sonnet-5.5", "profiles.sonnet-5.5: プロファイル名は [a-z][a-z0-9_-]{0,31} の形にしてください"},
	}
	for _, test := range cases {
		check := pv2CheckBody(t, f, server, map[string]any{
			"place": p2Project, "original": "sonnet-fast",
			"doc": map[string]any{"name": test.name, "kind": "claude", "args": []string{}, "env": []map[string]string{}},
		})
		found := false
		for _, problem := range check.Problems {
			if problem.Text != test.want {
				continue
			}
			found = true
			if problem.At != "name" {
				t.Errorf("the check tells %q on %q, want the name field", test.want, problem.At)
			}
		}
		if !found {
			t.Errorf("the check never says %q: %+v", test.want, check.Problems)
		}
	}

	for _, name := range []string{"", "sonnet-5.5"} {
		resp, body := f.api(t, server, http.MethodPut, "/api/profile/sonnet-fast", map[string]any{
			"place": p2Project, "original": "sonnet-fast", "hash": source.Hash(before),
			"doc": map[string]any{"name": name, "kind": "claude", "args": []string{}, "env": []map[string]string{}},
		})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("a save of the name %q answered %d, want 422: %s", name, resp.StatusCode, body)
		}
	}
	if got := readFile(t, projectPath); got != before {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// pv2HomeServer builds the working project and the shared file under the home
// directory, and the server the app would build for it: the environment's own
// display, so a path reads as a person sees it (X1). The machine's own config is
// never read.
func pv2HomeServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "profiles-v2")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(home, ".config")
	proj := filepath.Join(root, "project")
	copyTree(t, filepath.Join(src, "project"), proj)
	copyTree(t, filepath.Join(src, "config", "hergy"), filepath.Join(config, "hergy"))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Chdir(proj)
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	if env.ProjectRoot != proj {
		t.Fatalf("the environment's project root is %q, want the working project %q", env.ProjectRoot, proj)
	}
	server, err := New(env.Catalog(source.Disk{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	return server, proj, root
}

// TestPROFILES_V7_ThePathsReadAsTheProjectAndTheHome: with the shared file
// under ~/.config/hergy, the list, one item and a check's answer write every
// path as .hergy/… or ~/.config/hergy/…, and the working directory's absolute
// path is nowhere (spec-profiles 1.7, X1).
func TestPROFILES_V7_ThePathsReadAsTheProjectAndTheHome(t *testing.T) {
	server, proj, root := pv2HomeServer(t)
	// f.api only carries the token and the Host check; it reads nothing off the
	// fixture, so the home-based project needs no fixture of its own.
	f := &fixture{}

	// 一覧: the two files' paths and every row's place read relative.
	resp, body := f.api(t, server, http.MethodGet, "/api/profile", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/profile answers %d", resp.StatusCode)
	}
	var list apiList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("the list is not JSON: %v: %s", err, body)
	}
	paths := map[string]string{}
	for _, file := range list.Files {
		paths[file.Place] = file.Path
	}
	if paths[p2Project] != ".hergy/profiles.yaml" {
		t.Errorf("the project's file reads %q, want .hergy/profiles.yaml", paths[p2Project])
	}
	if paths[p2Shared] != "~/.config/hergy/profiles.yaml" {
		t.Errorf("the shared file reads %q, want ~/.config/hergy/profiles.yaml", paths[p2Shared])
	}
	pv2NoAbsolutePath(t, "the list", body, proj, root)

	// 1 件: one profile's own reading names its file the same way.
	resp, body = f.api(t, server, http.MethodGet, "/api/profile/sonnet-fast?place=project", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET one profile answers %d", resp.StatusCode)
	}
	var one apiProfileGet
	if err := json.Unmarshal([]byte(body), &one); err != nil {
		t.Fatalf("the one profile is not JSON: %v: %s", err, body)
	}
	if one.Path != ".hergy/profiles.yaml" {
		t.Errorf("one profile's file reads %q, want .hergy/profiles.yaml", one.Path)
	}
	pv2NoAbsolutePath(t, "the one profile", body, proj, root)

	// 検査: a broken shared text is told with the ~/ form of its path.
	resp, body = f.api(t, server, http.MethodPost, "/api/profile/check", map[string]any{
		"place": p2Shared, "text": "version: 1\nprofiles: {}\nbogus: 1\n",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
	}
	var check apiCheck
	if err := json.Unmarshal([]byte(body), &check); err != nil {
		t.Fatalf("the check is not JSON: %v: %s", err, body)
	}
	told := false
	for _, problem := range check.Problems {
		if strings.Contains(problem.Text, "~/.config/hergy/profiles.yaml: field bogus not found") {
			told = true
		}
	}
	if !told {
		t.Errorf("the check does not name the file in its ~/ form: %+v", check.Problems)
	}
	pv2NoAbsolutePath(t, "the check", body, proj, root)
}

// pv2NoAbsolutePath fails while one answer writes the working directory's own
// absolute path.
func pv2NoAbsolutePath(t *testing.T, what, body string, abs ...string) {
	t.Helper()
	for _, one := range abs {
		if one != "" && strings.Contains(body, one) {
			t.Errorf("%s writes the absolute path %q:\n%s", what, one, body)
		}
	}
}
