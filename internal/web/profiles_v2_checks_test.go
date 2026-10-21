package web

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/profile"
)

// The profile list's items the gate reads at the 受け口 layer: D3 (the screen's
// sheet) and V6 (the screen's checks are `hergy validate`'s checks, and only
// those). The screen's own walks of the same IDs live in internal/app
// (e2e_profiles_items_test.go); these hold the two promises that belong to the
// boundary rather than to the page.

// TestPROFILES_D3_TheScreenWritesNoColours: the profile screen's sheet names no
// colour of its own. The one place it may name one is where it defines the
// design's tokens — the mock's :root and its two dark blocks — so every
// declaration that sets a token is taken out first, and what is left may read
// var(...), a keyword, or a length, and nothing else (spec-profiles 7.5 D3).
func TestPROFILES_D3_TheScreenWritesNoColours(t *testing.T) {
	raw, err := files.ReadFile("static/profiles-v2.css")
	if err != nil {
		t.Fatal(err)
	}
	text := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(raw), "")
	text = regexp.MustCompile(`--[A-Za-z0-9-]+\s*:[^;]*;`).ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "!important", "")
	for _, find := range []*regexp.Regexp{
		regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`),
		regexp.MustCompile(`\brgba?\(`),
		regexp.MustCompile(`\bhsla?\(`),
		regexp.MustCompile(`\boklch\(`),
		regexp.MustCompile(`\bcolor\(`),
	} {
		if found := find.FindAllString(text, -1); len(found) != 0 {
			t.Errorf("profiles-v2.css names the colour %v of its own", found)
		}
	}
	colours := map[string]bool{
		"color": true, "background": true, "background-color": true,
		"border": true, "border-color": true, "border-top": true, "border-right": true,
		"border-bottom": true, "border-left": true, "border-top-color": true,
		"border-right-color": true, "border-bottom-color": true, "border-left-color": true,
		"outline": true, "outline-color": true, "fill": true, "stroke": true,
		"box-shadow": true, "text-decoration-color": true, "caret-color": true,
	}
	// What a colour property may carry without one of the page's tokens: the
	// tokens themselves (var), the words that name no colour, and the lengths a
	// border or a shadow writes.
	plain := map[string]bool{
		"none": true, "0": true, "inherit": true, "unset": true, "initial": true,
		"transparent": true, "currentColor": true, "currentcolor": true,
		"solid": true, "dashed": true, "dotted": true, "double": true, "inset": true, "outset": true,
		"line-through": true, "underline": true, "overline": true,
	}
	length := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(px|em|rem|%|vw|vh|ch|pt)?$`)
	flat := strings.NewReplacer("{", ";", "}", ";").Replace(text)
	for decl := range strings.SplitSeq(flat, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		prop := strings.TrimSpace(name)
		if !colours[prop] {
			continue
		}
		v := strings.TrimSpace(value)
		if strings.Contains(v, "var(") {
			continue
		}
		ok = true
		for word := range strings.FieldsSeq(v) {
			if !plain[word] && !length.MatchString(word) {
				ok = false
			}
		}
		if !ok {
			t.Errorf("profiles-v2.css sets %s to %q, which is not one of the page's tokens", prop, v)
		}
	}
}

// TestPROFILES_V6_TheScreenInventsNoCheck: every sentence the profile screen's
// checks write is one `hergy validate` writes too — the assign tab's missing
// profile, the YAML tab's problems, and the field check a panel sends while it
// is being typed. A check the screen added of its own would show up here as a
// sentence the reading behind validate does not make (spec-profiles 7.6 V6).
func TestPROFILES_V6_TheScreenInventsNoCheck(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)

	v6AssignMatchesValidate(t, f, server)
	v6YAMLMatchesValidate(t, f, server)
	v6DraftMatchesValidate(t, f, server)
}

// v6AssignMatchesValidate is the assign tab's part: the sentence shown for a
// person with a profile that is not there is the sentence check.Definition —
// what `hergy validate` prints for a team — writes for that same row.
func v6AssignMatchesValidate(t *testing.T, f *fixture, server *Server) {
	t.Helper()
	assign := f.islandOf(t, server, "/profiles/assign")
	if len(assign.Assign.Teams) == 0 {
		t.Fatal("the assign tab carries no cards")
	}
	shown := 0
	for _, team := range assign.Assign.Teams {
		// The findings validate writes for this team's rows, keyed by the field
		// they stand beside: members[i].profile and workers[i].
		want := map[string]string{}
		for _, finding := range check.Definition(check.Input{Catalog: f.catalog(), Name: team.Name}) {
			if strings.Contains(finding.Text, "というプロファイルはありません") {
				want[finding.Field] = finding.Text
			}
		}
		for _, row := range team.Rows {
			if !row.Missing {
				if row.Error != "" {
					t.Errorf("team %s row %s is not missing but the tab says %q", team.Name, row.ID, row.Error)
				}
				continue
			}
			shown++
			field := p6AssignField(row.ID)
			text, ok := want[field]
			if !ok {
				t.Errorf("the assign tab says %q for team %s %s, which hergy validate's check does not say", row.Error, team.Name, field)
				continue
			}
			delete(want, field)
			if !strings.HasSuffix(text, row.Error) {
				t.Errorf("the assign tab says %q for team %s %s, hergy validate says %q", row.Error, team.Name, field, text)
			}
		}
		for field, text := range want {
			t.Errorf("hergy validate's check says %q for team %s %s, which the assign tab does not show", text, team.Name, field)
		}
	}
	if shown == 0 {
		t.Error("the assign tab shows no missing profile, so the comparison is empty")
	}
}

// p6AssignField is one assign row's id in the field names check uses: m-3 stands
// for members[3].profile, w-0 for workers[0].
func p6AssignField(id string) string {
	if rest, ok := strings.CutPrefix(id, "m-"); ok {
		return "members[" + rest + "].profile"
	}
	if rest, ok := strings.CutPrefix(id, "w-"); ok {
		return "workers[" + rest + "]"
	}
	return id
}

// v6YAMLMatchesValidate is the YAML tab's part: the problems shown beside each
// file are the problems profile.ReadAll — what `hergy validate` reads
// profiles.yaml with — finds in that same text, and no others.
func v6YAMLMatchesValidate(t *testing.T, f *fixture, server *Server) {
	t.Helper()
	yaml := f.islandOf(t, server, "/profiles/yaml")
	if len(yaml.Profile.Files) != 2 {
		t.Fatalf("the YAML tab carries %d files, want 2", len(yaml.Profile.Files))
	}
	for _, file := range yaml.Profile.Files {
		_, problems := profile.ReadAll(file.Text, file.Path)
		want := map[string]string{}
		for _, problem := range problems {
			want[p6Key(problem.Sev.Word(), problem.Line, problem.Text)] = problem.Text
		}
		for _, problem := range file.Problems {
			key := p6Key(problem.Sev, problem.Line, problem.Text)
			if _, ok := want[key]; !ok {
				t.Errorf("the YAML tab shows %q on line %d of %s, which hergy validate's reading does not", problem.Text, problem.Line, file.Path)
				continue
			}
			delete(want, key)
		}
		for _, text := range want {
			t.Errorf("hergy validate's reading of %s finds %q, which the YAML tab does not show", file.Path, text)
		}
	}
}

// v6DraftMatchesValidate is the field check's part: what a panel sends while it
// is being typed is a subset of what profile.ReadAll would find in the file a
// save of that draft would write, so a draft check can never be stricter than
// the gate the written file goes through.
func v6DraftMatchesValidate(t *testing.T, f *fixture, server *Server) {
	t.Helper()
	yaml := f.islandOf(t, server, "/profiles/yaml")
	var project *p2TestYAMLFile
	for i := range yaml.Profile.Files {
		if yaml.Profile.Files[i].Place == "project" {
			project = &yaml.Profile.Files[i]
		}
	}
	if project == nil {
		t.Fatal("the YAML tab carries no project file")
	}
	entries, _ := profile.ReadAll(project.Text, project.Path)

	cases := []struct {
		original string
		draft    p2Draft
	}{
		{"sonnet-fast", p2Draft{Name: "sonnet-5.5", Kind: ""}},
		{"haiku-chore", p2Draft{Name: "haiku-chore", Kind: ""}},
		{"", p2Draft{Name: "Bad Name", Kind: "claude"}},
	}
	checked := 0
	for _, test := range cases {
		resp, decoded := f.apiJSON(t, server, http.MethodPost, "/api/profile/check", map[string]any{
			"place": "project", "original": test.original, "doc": test.draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("the check answered %d", resp.StatusCode)
		}
		problems, _ := decoded["problems"].([]any)
		if len(problems) == 0 {
			continue
		}
		written, err := p2EditProfile(project.Text, project.Path, entries, test.draft.profileOf(), test.original)
		if err != nil {
			t.Fatalf("the draft's save could not be built: %v", err)
		}
		_, found := profile.ReadAll(written, project.Path)
		have := map[string]bool{}
		for _, problem := range found {
			have[p6Key(problem.Sev.Word(), 0, problem.Text)] = true
		}
		for _, one := range problems {
			problem, _ := one.(map[string]any)
			sev, _ := problem["sev"].(string)
			text, _ := problem["text"].(string)
			checked++
			if !have[p6Key(sev, 0, text)] {
				t.Errorf("the field check says %q (%s) for %q, which hergy validate's reading of the written file does not", text, sev, test.original)
			}
		}
	}
	if checked == 0 {
		t.Error("no draft check made a problem, so the comparison is empty")
	}
}

// p6Key is one problem as the two sides compare it: how bad it is, the line it
// stands on (0 when a draft's problems carry no line), and the sentence.
func p6Key(sev string, line int, text string) string {
	return fmt.Sprintf("%s\x00%d\x00%s", sev, line, text)
}
