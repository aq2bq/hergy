package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// These are the check units' promises (d-a575, r-2f83), moved from the browser
// walks CHECK-standard and CHECK-running: every list a screen draws counts one
// team definition's own check, a role document's answer names no team, a
// combination problem stands on the team definition and names the role document
// it came from, and hergy validate splits the same units and sizes. The two
// scenes differed in herdr alone, so one fixture (testdata/teams-v2, the sample
// both stood on) holds both here.

// checkFixture lays the team screen's real-scale fixture out and starts the
// serve the checks are read through.
func checkFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	useJapanese(t)
	f := newFixture(t)
	src := filepath.Join(cwd, "..", "..", "testdata", "teams-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	base, stop := startServe(t, f)
	t.Cleanup(stop)
	base = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+`).FindString(base)
	if base == "" {
		t.Fatal("the serve printed no address")
	}
	checkToken = canvasToken(t, base, "/teams")
	return f, base
}

// checkToken is the token the serve's writes carry: it is made for one run, so
// the test reads it from the page that carries it.
var checkToken string

// checkProblems is one check answer's problems as the screens count them.
type checkProblems struct {
	Sev    string `json:"sev"`
	Text   string `json:"text"`
	At     string `json:"at"`
	Field  string `json:"field"`
	Role   string `json:"role"`
	RuleID string `json:"rule_id"`
}

type checkAnswerBody struct {
	Problems []checkProblems `json:"problems"`
}

// countOf counts one answer's problems the way a screen's badge does.
func countOf(problems []checkProblems) string {
	errs, warns := 0, 0
	for _, one := range problems {
		if one.Sev == "err" {
			errs++
		} else {
			warns++
		}
	}
	return strconv.Itoa(errs) + "/" + strconv.Itoa(warns)
}

// apiBody sends one request to the serve and returns the answer's body.
func apiBody(t *testing.T, base, method, path string, payload any) []byte {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if checkToken != "" {
		req.Header.Set("X-Hergy-Token", checkToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	out, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s answers %d: %s", method, path, resp.StatusCode, out)
	}
	return out
}

// checkProblemsOf sends one check and returns its problems.
func checkProblemsOf(t *testing.T, base, path string, payload any) []checkProblems {
	t.Helper()
	var answer checkAnswerBody
	if err := json.Unmarshal(apiBody(t, base, http.MethodPost, path, payload), &answer); err != nil {
		t.Fatalf("%s does not read: %v", path, err)
	}
	return answer.Problems
}

type teamListItem struct {
	Name      string `json:"name"`
	Effective string `json:"effective"`
	Problems  struct {
		Err  int `json:"err"`
		Warn int `json:"warn"`
	} `json:"problems"`
	Summary struct {
		Size int `json:"size"`
	} `json:"summary"`
	Places map[string]struct {
		Path string `json:"path"`
		Text string `json:"text"`
	} `json:"places"`
}

func teamList(t *testing.T, base string) []teamListItem {
	t.Helper()
	var list struct {
		Items []teamListItem `json:"items"`
	}
	if err := json.Unmarshal(apiBody(t, base, http.MethodGet, "/api/team", nil), &list); err != nil {
		t.Fatalf("the team list does not read: %v", err)
	}
	return list.Items
}

// TestCHECK_TheCountsAreOnePerDefinition: the row's badge, the validate column
// and the YAML tab's team.yaml head count the same team definition's check — for
// the saved file and for the unsaved text the YAML tab carries (d-a575).
func TestCHECK_TheCountsAreOnePerDefinition(t *testing.T) {
	_, base := checkFixture(t)
	items := teamList(t, base)
	if len(items) == 0 {
		t.Fatal("the fixture lists no teams")
	}
	for _, item := range items {
		want := strconv.Itoa(item.Problems.Err) + "/" + strconv.Itoa(item.Problems.Warn)
		place := item.Places[item.Effective]
		panel := countOf(checkProblemsOf(t, base, "/api/team/check", map[string]any{
			"place": item.Effective, "name": item.Name, "part": "team",
			"doc": map[string]any{"ops": []map[string]any{}},
		}))
		if panel != want {
			t.Errorf("%s: the row counts %s and its validate column %s", item.Name, want, panel)
		}
		text := countOf(checkProblemsOf(t, base, "/api/team/check", map[string]any{
			"place": item.Effective, "name": item.Name, "part": "team", "text": place.Text,
		}))
		if text != want {
			t.Errorf("%s: the row counts %s and its YAML head %s", item.Name, want, text)
		}
	}
}

// TestCHECK_ALeftoverAgreementIsOnEveryUnit: a leftover shared agreement.yaml
// is one break on every definition (d-31ad), counted the same by the row, the
// validate column and the YAML head.
func TestCHECK_ALeftoverAgreementIsOnEveryUnit(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	useJapanese(t)
	f := newFixture(t)
	src := filepath.Join(cwd, "..", "..", "testdata", "teams-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	base, stop := startServe(t, f)
	t.Cleanup(stop)
	base = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+`).FindString(base)
	if base == "" {
		t.Fatal("the serve printed no address")
	}
	checkToken = canvasToken(t, base, "/teams")

	baseline := map[string]int{}
	for _, item := range teamList(t, base) {
		baseline[item.Name] = item.Problems.Err + item.Problems.Warn
	}
	// The file every team of the layer used to share, left where it used to
	// stand: it is one break on every definition (d-31ad).
	writeFile(t, filepath.Join(f.proj, ".hergy", "agreement.yaml"), "version: 1\ntitle: 共通\nsections:\n  - heading: 台帳\n    rules:\n      - id: S1\n        text: 全員が読む。\n")

	items := teamList(t, base)
	if len(items) == 0 {
		t.Fatal("the fixture lists no teams")
	}
	for _, item := range items {
		total := item.Problems.Err + item.Problems.Warn
		if total <= baseline[item.Name] {
			t.Errorf("%s counts %d problems with the leftover, %d without: the leftover is on no row", item.Name, total, baseline[item.Name])
		}
		want := strconv.Itoa(item.Problems.Err) + "/" + strconv.Itoa(item.Problems.Warn)
		panel := countOf(checkProblemsOf(t, base, "/api/team/check", map[string]any{
			"place": item.Effective, "name": item.Name, "part": "team",
			"doc": map[string]any{"ops": []map[string]any{}},
		}))
		if panel != want {
			t.Errorf("%s: with the leftover, the row counts %s and the validate column %s", item.Name, want, panel)
		}
		text := countOf(checkProblemsOf(t, base, "/api/team/check", map[string]any{
			"place": item.Effective, "name": item.Name, "part": "team", "text": item.Places[item.Effective].Text,
		}))
		if text != want {
			t.Errorf("%s: with the leftover, the row counts %s and the YAML head %s", item.Name, want, text)
		}
	}
}

// TestCHECK_EachAnswerNamesItsOwnUnit: a role document's own problems name no
// team or member (ac-be51 1); a team definition's answer does not carry a role
// document's own warning (ac-be51 2); and a combination problem stands on the
// team definition, naming the role document and rule it came from (ac-be51 3).
func TestCHECK_EachAnswerNamesItsOwnUnit(t *testing.T) {
	f, base := checkFixture(t)

	// 1: the role document's own answer.
	lead := readFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"))
	role := checkProblemsOf(t, base, "/api/role/check", map[string]any{
		"place": "project", "name": "lead", "text": lead,
	})
	var roleText strings.Builder
	for _, one := range role {
		roleText.WriteString(one.Text)
		roleText.WriteString("\n")
	}
	if strings.Contains(roleText.String(), "core-lead") || strings.Contains(roleText.String(), "に渡る規則") {
		t.Errorf("the role's answer names a team's member:\n%s", roleText.String())
	}

	// 2: core's team answer, with its own single size warning, does not carry
	// builder.yaml's own model-name warning.
	core := checkProblemsOf(t, base, "/api/team/check", map[string]any{
		"place": "project", "name": "core", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	var coreText strings.Builder
	for _, one := range core {
		coreText.WriteString(one.Text)
		coreText.WriteString("\n")
	}
	if len(core) != 1 {
		t.Errorf("core's team answer holds %d problems, want its one size warning: %s", len(core), coreText.String())
	}
	if strings.Contains(coreText.String(), "BUILD-5") || strings.Contains(coreText.String(), "本文にモデルの名前") {
		t.Errorf("core's team answer carries builder.yaml's own warning:\n%s", coreText.String())
	}

	// 3: draft's rule WRITE-7 names lead, which draft does not have; the
	// problem stands on the team definition and names writer.yaml's rule.
	draft := checkProblemsOf(t, base, "/api/team/check", map[string]any{
		"place": "global", "name": "draft", "part": "team",
		"doc": map[string]any{"ops": []map[string]any{}},
	})
	found := false
	for _, one := range draft {
		if one.Role != "writer" || one.RuleID != "WRITE-7" {
			continue
		}
		found = true
		// The sentence names the rule; the file it stands in is the field the
		// screen's own column shows (teams/view.js vlistParts).
		if !strings.Contains(one.Text, "WRITE-7") || !strings.Contains(one.Field, "writer.yaml") {
			t.Errorf("the combination problem reads %+v, want writer.yaml's WRITE-7", one)
		}
	}
	if !found {
		t.Errorf("draft's team answer does not name writer.yaml's WRITE-7: %+v", draft)
	}
}

// teamSize is one line of validate's size list.
var teamSizeLine = regexp.MustCompile(`^  team (.+): (\d+) characters \(the provisional budget is \d+\)$`)
var roleSizeLine = regexp.MustCompile(`^  role (.+): (\d+) characters \(the provisional budget is \d+\)$`)

// TestCHECK_ValidateSplitsTheUnitsAndSizes: hergy validate keeps a role
// document's own warning in its own section, checked once however many teams
// read it, gives a hidden document the not-used line alone, and its size list
// is the screens' list (T13 R1, T13 R3, T13 R4).
func TestCHECK_ValidateSplitsTheUnitsAndSizes(t *testing.T) {
	f, base := checkFixture(t)
	out, stderr, code := run("validate", "--lang", "en")
	if out == "" {
		t.Fatalf("validate wrote nothing (exit %d): %s", code, stderr)
	}
	blocks := []string{}
	for one := range strings.SplitSeq(out, "\nfile: ") {
		if one == "" {
			continue
		}
		if !strings.HasPrefix(one, "file: ") {
			one = "file: " + one
		}
		blocks = append(blocks, one)
	}
	block := func(path string) string {
		for _, one := range blocks {
			if strings.HasPrefix(one, "file: "+path+"\n") || one == "file: "+path {
				return one
			}
		}
		return ""
	}

	core := block(".hergy/teams/core/team.yaml")
	if strings.Contains(core, "BUILD-5") || strings.Contains(core, "names a model") {
		t.Errorf("core's team section carries builder.yaml's own warning:\n%s", core)
	}
	builder := block(".hergy/roles/builder.yaml")
	if !strings.Contains(builder, "BUILD-5") || !strings.Contains(builder, "names a model") {
		t.Errorf("builder.yaml's own warning is not in its role section:\n%s", builder)
	}
	if n := strings.Count(out, "file: .hergy/roles/builder.yaml\n"); n != 1 {
		t.Errorf("builder.yaml is checked %d times, want once", n)
	}
	globalDir := filepath.Join(f.cfg, "hergy")
	draft := block(filepath.Join(globalDir, "teams", "draft", "team.yaml"))
	if !regexp.MustCompile(`writer\.yaml: rule WRITE-7|reviewer\.yaml: rule REV-4`).MatchString(draft) {
		t.Errorf("draft's team section does not name the role document's rule:\n%s", draft)
	}
	hiddenDocs := block(filepath.Join(globalDir, "teams", "docs", "team.yaml"))
	if !strings.Contains(hiddenDocs, "is not used") {
		t.Errorf("the hidden docs does not say it is not used:\n%s", hiddenDocs)
	}
	if regexp.MustCompile(`WRITE-7|REV-4`).MatchString(hiddenDocs) {
		t.Errorf("the hidden docs shows a warning:\n%s", hiddenDocs)
	}
	if hiddenScout := block(filepath.Join(globalDir, "roles", "scout.yaml")); !strings.Contains(hiddenScout, "is not used") {
		t.Errorf("the hidden scout does not say it is not used:\n%s", hiddenScout)
	}

	// The size list is the screens' list: the effective place once per name, a
	// definition that does not read included, the hidden place left out.
	teamSizes := map[string]int{}
	roleSizes := map[string]int{}
	for line := range strings.SplitSeq(out, "\n") {
		if m := teamSizeLine.FindStringSubmatch(line); m != nil {
			teamSizes[m[1]], _ = strconv.Atoi(m[2])
		}
		if m := roleSizeLine.FindStringSubmatch(line); m != nil {
			roleSizes[m[1]], _ = strconv.Atoi(m[2])
		}
	}
	wantTeams := map[string]int{}
	for _, item := range teamList(t, base) {
		wantTeams[strings.TrimSuffix(item.Places[item.Effective].Path, "/team.yaml")] = item.Summary.Size
	}
	for path, size := range wantTeams {
		if got, ok := teamSizes[path]; !ok || got != size {
			t.Errorf("the size list reads %q = %d (%v), want %d", path, got, ok, size)
		}
	}
	if len(teamSizes) != len(wantTeams) {
		t.Errorf("the size list names %d definitions, the screen lists %d: %v", len(teamSizes), len(wantTeams), teamSizes)
	}

	var roles struct {
		Items []struct {
			Effective string `json:"effective"`
			Summary   struct {
				Chars int `json:"chars"`
			} `json:"summary"`
			Places map[string]struct {
				Path string `json:"path"`
			} `json:"places"`
		} `json:"items"`
	}
	if err := json.Unmarshal(apiBody(t, base, http.MethodGet, "/api/role", nil), &roles); err != nil {
		t.Fatalf("the role list does not read: %v", err)
	}
	wantRoles := map[string]int{}
	for _, item := range roles.Items {
		path := item.Places[item.Effective].Path
		if path != "" {
			wantRoles[path] = item.Summary.Chars
		}
	}
	for path, size := range wantRoles {
		if got, ok := roleSizes[path]; !ok || got != size {
			t.Errorf("the role size list reads %q = %d (%v), want %d", path, got, ok, size)
		}
	}
	if len(roleSizes) != len(wantRoles) {
		t.Errorf("the role size list names %d documents, the screen lists %d: %v", len(roleSizes), len(wantRoles), roleSizes)
	}
}
