package homeview

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// fakePort answers the screens from what a test fills in, so the screens can be
// rendered without a project on disk.
type fakePort struct {
	home Home
	diff Diff
	// layers records what the screens asked for, so a test can see that the
	// layer of the request reached the port.
	layers *[]string
}

func (p fakePort) Home(layer string) Home {
	if p.layers != nil {
		*p.layers = append(*p.layers, layer)
	}
	return p.home
}

func (p fakePort) Diff(layer string) Diff {
	if p.layers != nil {
		*p.layers = append(*p.layers, layer)
	}
	return p.diff
}

func (p fakePort) RuleHref(_, path, id string) string {
	return "/rules?layer=" + path + "&rule=" + id
}

// render runs one screen's page with the query given and returns its body.
func render(t *testing.T, screen web.Screen, target string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	view, err := screen.Routes()[0].Handler(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(view.Body)
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s does not hold %q:\n%s", what, want, body)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, unwanted ...string) {
	t.Helper()
	for _, text := range unwanted {
		if strings.Contains(body, text) {
			t.Errorf("%s holds %q, which it should not:\n%s", what, text, body)
		}
	}
}

// TestTheHomeShowsWhatMovedAndWhatHasNoCheck: one document carries the ids that
// changed since the last launch, and the rules with no check are links into the
// rules screen.
func TestTheHomeShowsWhatMovedAndWhatHasNoCheck(t *testing.T) {
	port := fakePort{home: Home{
		Launched: true,
		Docs: []Doc{
			{
				Path:    ".hergy/teams/demo/protocol.yaml",
				Layer:   web.LayerLocal,
				Rules:   8,
				NoCheck: []Rule{{ID: "C6", Href: "/rules?layer=local&rule=C6"}},
				Added:   []string{"C9"},
				Removed: []string{"C2"},
				Changed: []string{"C6"},
				Bumped:  true,
				Version: 3,
			},
		},
	}}
	screen, err := New(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/?layer=local")
	mustContain(t, "the home", body,
		"protocol.yaml",
		i18n.Tf("web.home.changes.added", "C9"),
		i18n.Tf("web.home.changes.removed", "C2"),
		i18n.Tf("web.home.changes.changed", "C6"),
		i18n.Tf("web.home.changes.bumped", 3),
		i18n.Tf("web.home.nocheck.count", 1, 8),
		`<a href="/rules?layer=local&amp;rule=C6">C6</a>`,
	)
	mustNotContain(t, "the home", body, i18n.T("web.home.changes.no_record"))
}

// TestTheHomeSaysWhenNothingWasLaunched: with no record there is nothing to
// compare with, and the screen names the command that writes one. The rules with
// no check still show: they are read from the files.
func TestTheHomeSaysWhenNothingWasLaunched(t *testing.T) {
	port := fakePort{home: Home{Docs: []Doc{{
		Path:    ".hergy/teams/demo/roles/lead.yaml",
		Layer:   web.LayerLocal,
		Rules:   2,
		NoCheck: []Rule{{ID: "L1", Href: "/rules?layer=local&rule=L1"}},
	}}}}
	screen, err := New(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/?layer=local")
	mustContain(t, "the home", body,
		"hergy up &lt;name&gt;",
		i18n.Tf("web.home.nocheck.count", 1, 2),
		"L1",
	)
}

// TestTheHomeShowsAProblemWithTheRecord: a record that cannot be read is one
// line on the screen, and the rest of the screen still renders.
func TestTheHomeShowsAProblemWithTheRecord(t *testing.T) {
	port := fakePort{home: Home{
		Problem: "~/.local/state/hergy/launches/log.jsonl: line 3: not a JSON line",
		Docs: []Doc{{
			Path:    ".hergy/teams/demo/protocol.yaml",
			Layer:   web.LayerLocal,
			Rules:   1,
			NoCheck: []Rule{{ID: "C1"}},
		}},
	}}
	screen, err := New(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/?layer=local")
	mustContain(t, "the home", body, "line 3: not a JSON line", "protocol.yaml", "C1")
}

// TestTheScreensReadTheLayerOfTheRequest: the layer travels in the query, and
// the screen asks the port for that layer.
func TestTheScreensReadTheLayerOfTheRequest(t *testing.T) {
	var asked []string
	port := fakePort{layers: &asked}
	home, err := New(port)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := NewDiff(port)
	if err != nil {
		t.Fatal(err)
	}

	render(t, home, "/?layer=global")
	render(t, diff, "/diff?layer=global")

	if len(asked) != 2 || asked[0] != web.LayerGlobal || asked[1] != web.LayerGlobal {
		t.Errorf("the screens asked for %v, want both to read the global layer", asked)
	}
}

// TestTheDiffShowsEachMemberAndFoldsTheOnesThatDidNotChange: a member whose
// prompt moved shows the lines, and a member that did not is folded away.
func TestTheDiffShowsEachMemberAndFoldsTheOnesThatDidNotChange(t *testing.T) {
	port := fakePort{diff: Diff{Definitions: []Definition{{
		Name:     "demo",
		Launched: true,
		Members: []Member{
			{
				Name:     "demo-lead",
				Launched: true,
				Lines: []Line{
					{Kind: Same, Text: "# Team"},
					{Kind: Minus, Text: "old rule"},
					{Kind: Plus, Text: "new rule"},
				},
			},
			{
				Name:     "demo-builder",
				Launched: true,
				Lines: []Line{
					{Kind: Same, Text: "# Team"},
					{Kind: Same, Text: "same rule"},
				},
			},
		},
	}}}}
	screen, err := NewDiff(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/diff?layer=local")
	mustContain(t, "the diff", body,
		"demo-lead",
		`class="line minus"`,
		"old rule",
		`class="line plus"`,
		"new rule",
		"<details",
		i18n.Tf("web.diff.member_same", "demo-builder"),
	)
	if strings.Count(body, "<details") != 1 {
		t.Errorf("the diff folds %d members, want only the one that did not change", strings.Count(body, "<details"))
	}
	if strings.Count(body, "<pre") != 2 {
		t.Errorf("the diff shows %d prompts, want both members", strings.Count(body, "<pre"))
	}
}

// TestTheDiffSaysWhenAMemberWasNeverHandedAPrompt: a member the record does not
// know is named, rather than shown as an empty diff.
func TestTheDiffSaysWhenAMemberWasNeverHandedAPrompt(t *testing.T) {
	port := fakePort{diff: Diff{Definitions: []Definition{{
		Name:     "demo",
		Launched: true,
		Members:  []Member{{Name: "demo-lead"}},
	}}}}
	screen, err := NewDiff(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/diff?layer=local")
	mustContain(t, "the diff", body, i18n.Tf("web.diff.member_never", "demo-lead"))
	mustNotContain(t, "the diff", body, "<pre")
}

// TestTheDiffSaysWhenNothingWasLaunched: without a record there is no prompt to
// compare with, and the screen names the command that writes one.
func TestTheDiffSaysWhenNothingWasLaunched(t *testing.T) {
	port := fakePort{diff: Diff{Definitions: []Definition{{Name: "demo"}}}}
	screen, err := NewDiff(port)
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/diff?layer=local")
	mustContain(t, "the diff", body, "hergy up &lt;name&gt;", "demo")
}

// TestTheHomeScreenBringsItsStylesheet: the record's screens draw with a file of
// their own, so no other screen has to share it.
func TestTheHomeScreenBringsItsStylesheet(t *testing.T) {
	screen, err := New(fakePort{})
	if err != nil {
		t.Fatal(err)
	}
	brought, ok := screen.(web.ScreenAssets)
	if !ok {
		t.Fatal("the home screen brings no files of its own")
	}
	assets := brought.Assets()
	if assets.Name != "homeview" {
		t.Errorf("the files are served under %q, want homeview", assets.Name)
	}
	if len(assets.Stylesheets) != 1 || assets.Stylesheets[0] != "homeview.css" {
		t.Fatalf("the home screen brings %v, want homeview.css", assets.Stylesheets)
	}
	text, err := fs.ReadFile(assets.Files, assets.Stylesheets[0])
	if err != nil {
		t.Fatalf("the stylesheet is not in the screen's files: %v", err)
	}
	if !strings.Contains(string(text), ".steps") {
		t.Errorf("the stylesheet does not carry the step bar:\n%s", text)
	}
}

// TestTheDiffScreenBringsNoSecondCopyOfTheFiles: two screens under one name would
// stop hergy serve, so the diff screen has none and the frame links the home's.
func TestTheDiffScreenBringsNoSecondCopyOfTheFiles(t *testing.T) {
	screen, err := NewDiff(fakePort{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := screen.(web.ScreenAssets); ok {
		t.Error("the diff screen brings its own files; the home screen already brings them")
	}
}

// TestDiffLinesMarksWhatOnlyOneSideHas: the diff of two prompts keeps the lines
// both have and marks the ones only one side has.
func TestDiffLinesMarksWhatOnlyOneSideHas(t *testing.T) {
	got := DiffLines(
		[]string{"# Team", "C1 keep", "C2 old", "C3 gone"},
		[]string{"# Team", "C1 keep", "C2 new", "C4 added"},
	)
	want := []Line{
		{Kind: Same, Text: "# Team"},
		{Kind: Same, Text: "C1 keep"},
		{Kind: Minus, Text: "C2 old"},
		{Kind: Minus, Text: "C3 gone"},
		{Kind: Plus, Text: "C2 new"},
		{Kind: Plus, Text: "C4 added"},
	}
	if len(got) != len(want) {
		t.Fatalf("the diff is\n%v\nwant\n%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d is %v, want %v", i, got[i], want[i])
		}
	}
}

// TestDiffLinesKeepsTheCommonLines: a line that did not move is not marked, so a
// prompt that grew by one line shows one added line.
func TestDiffLinesKeepsTheCommonLines(t *testing.T) {
	got := DiffLines([]string{"a", "b"}, []string{"a", "b", "c"})
	if len(got) != 3 {
		t.Fatalf("the diff is %v, want three lines", got)
	}
	for i, want := range []Line{{Kind: Same, Text: "a"}, {Kind: Same, Text: "b"}, {Kind: Plus, Text: "c"}} {
		if got[i] != want {
			t.Errorf("line %d is %v, want %v", i, got[i], want)
		}
	}
}

// TestADiffOfNothingIsNotADiff: the same text on both sides is all "same" lines,
// which is what the screen folds away.
func TestADiffOfNothingIsNotADiff(t *testing.T) {
	prompt := []string{"# Team", "C1 one", "C2 two"}
	for _, line := range DiffLines(prompt, prompt) {
		if line.Kind != Same {
			t.Fatalf("a prompt compared with itself holds %v", line)
		}
	}
}

// TestADiffOfALargeTextStaysBounded: a text far larger than a prompt is compared
// coarsely rather than building a table nobody wants to hold.
func TestADiffOfALargeTextStaysBounded(t *testing.T) {
	before := make([]string, 2000)
	after := make([]string, 2000)
	for i := range before {
		before[i] = "before"
		after[i] = "after"
	}
	got := DiffLines(before, after)
	if len(got) != len(before)+len(after) {
		t.Fatalf("the coarse diff holds %d lines, want %d", len(got), len(before)+len(after))
	}
	if got[0].Kind != Minus || got[len(before)].Kind != Plus {
		t.Errorf("the coarse diff starts %v and turns at %v", got[0], got[len(before)])
	}
}

// TestALineKnowsItsMark: the mark is what the reader scans for.
func TestALineKnowsItsMark(t *testing.T) {
	for kind, want := range map[string]string{Same: " ", Plus: "+", Minus: "-"} {
		if got := (Line{Kind: kind}).Mark(); got != want {
			t.Errorf("a %s line is marked %q, want %q", kind, got, want)
		}
	}
}
