package homeview

import (
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
	// bump is what a write answers with, and bumped records what the screen
	// asked to raise (layer and path), so a test can see the form reach the app.
	bump   Answer
	bumped *[]string
	// bumpErr is what a write answers with instead, when a test asks for a
	// refused write.
	bumpErr error
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

func (p fakePort) Bump(layer, path string) (Answer, error) {
	if p.bumped != nil {
		*p.bumped = append(*p.bumped, layer+" "+path)
	}
	if p.bumpErr != nil {
		return Answer{}, p.bumpErr
	}
	return p.bump, nil
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

// TestTheHomeShowsWhatMovedAndWhatHasNoCheck: one document carries the rules
// that changed since the last launch, each with the parts that moved and their
// text before and after, and the rules with no check are links into the rules
// screen.
func TestTheHomeShowsWhatMovedAndWhatHasNoCheck(t *testing.T) {
	port := fakePort{home: Home{
		Launched: true,
		Docs: []Doc{
			{
				Path:    ".hergy/teams/demo/protocol.yaml",
				File:    "/proj/.hergy/teams/demo/protocol.yaml",
				Layer:   web.LayerLocal,
				Rules:   8,
				NoCheck: []Rule{{ID: "C6", Href: "/rules?layer=local&rule=C6"}},
				Added: []Rule{
					{ID: "C9", Section: "報告", Diffs: []Part{{After: "足した本文"}}},
				},
				Removed: []Rule{
					{ID: "C2", Diffs: []Part{{Before: "消した本文"}}},
				},
				Changed: []Rule{
					{
						ID:      "C6",
						Section: "基本",
						Fields:  []string{"text"},
						Diffs:   []Part{{Before: "前の本文", After: "後の本文"}},
					},
				},
				Bumped:     true,
				Version:    3,
				Definition: "demo",
				Launched:   "2026-10-15 14:03",
				Hash:       "abc1234",
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
		i18n.T("web.home.changes.added"),
		"<span class=\"mono\">C9</span>",
		`class="ck added"`,
		i18n.T("web.home.changes.removed"),
		"<span class=\"mono\">C2</span>",
		`class="ck removed"`,
		i18n.T("web.home.changes.changed"),
		"<span class=\"mono\">C6</span>",
		`class="ck changed"`,
		// The section a rule stands in, and the parts that moved.
		`<span class="tag">報告</span>`,
		i18n.T("web.home.changes.fields.text"),
		// What each rule read before and after.
		`<div class="diff"><ins>足した本文</ins></div>`,
		`<div class="diff"><del>消した本文</del></div>`,
		`<div class="diff"><del>前の本文</del><ins>後の本文</ins></div>`,
		// Which launch the changes are against.
		i18n.T("web.home.changes.launched")+`: demo · 2026-10-15 14:03 · <span class="mono">abc1234</span>`,
		i18n.Tf("web.home.changes.bumped", 3),
		i18n.Tf("web.home.nocheck.count", 1, 8),
		`<a class="ref" href="/rules?layer=local&amp;rule=C6">C6</a>`,
	)
	mustNotContain(t, "the home", body, i18n.T("web.home.changes.no_record"))
}

// TestTheHomeShowsTheStepBarWithTheCurrentStep: the home opens with the four
// steps of the daily flow, numbered, each a link into its screen, and the step
// this screen stands on is marked as the current one (試作 5 版).
func TestTheHomeShowsTheStepBarWithTheCurrentStep(t *testing.T) {
	screen, err := New(fakePort{})
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/?layer=local")
	mustContain(t, "the home", body,
		`aria-label="`+esc(i18n.T("web.guide.label"))+`"`,
		`<a class="gstep cur" href="/?layer=local" aria-current="step"><span class="gnum">1</span>`+esc(i18n.T("web.guide.step.changes"))+`</a>`,
		`<span class="gnum">2</span>`+esc(i18n.T("web.guide.step.fix")),
		`<span class="gnum">3</span>`+esc(i18n.T("web.guide.step.diff")),
		`<span class="gnum">4</span>`+esc(i18n.T("web.guide.step.bump")),
	)
	if want := strings.Count(body, `class="garrow"`); want != 3 {
		t.Errorf("the step bar holds %d arrows, want one between each pair of steps", want)
	}
	if got := strings.Count(body, `aria-current="step"`); got != 1 {
		t.Errorf("the step bar marks %d steps as the current one, want 1", got)
	}
}

// TestTheHomeShowsTheOccasionalWork: the home ends with one card for the work a
// person does now and then: a link to the team list and a link to the profiles.
// The map row and the "getting started" card are not shown (試作 homeView の
// div.two を 1 枚にしたもの).
func TestTheHomeShowsTheOccasionalWork(t *testing.T) {
	screen, err := New(fakePort{})
	if err != nil {
		t.Fatal(err)
	}

	body := render(t, screen, "/?layer=local")
	mustContain(t, "the home", body,
		`<div class="two">`,
		i18n.T("web.home.aside.title"),
		`href="/teams?layer=local"`,
		i18n.T("web.home.aside.teams"),
		`href="/profiles?layer=local"`,
		i18n.T("web.home.aside.profiles"),
	)
	if got := strings.Count(body, "<li>"); got != 2 {
		t.Errorf("the occasional work card holds %d rows, want the team list and the profiles", got)
	}
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

	// The home reads the layer once; the diff reads the diff and, for the step
	// bar's count, the home of the layer too.
	if len(asked) != 3 {
		t.Fatalf("the screens asked for %v, want three reads of the layer", asked)
	}
	for i, layer := range asked {
		if layer != web.LayerGlobal {
			t.Errorf("read %d asked for %q, want the global layer", i, layer)
		}
	}
}

// TestTheDiffShowsAMemberChipAndTheLinesThatMoved: every member gets a chip,
// the member whose prompt moved is shown in the panel with its lines marked, and
// the member whose prompt did not move is still reachable by its chip.
func TestTheDiffShowsAMemberChipAndTheLinesThatMoved(t *testing.T) {
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
		"demo-builder",
		`class="dl del"`,
		"old rule",
		`class="dl add"`,
		"new rule",
	)
	if got := strings.Count(body, `class="chip"`); got != 2 {
		t.Errorf("the diff draws %d member chips, want one per member", got)
	}

	// The panel shows the member whose prompt moved; the member that did not
	// change is still readable by asking for it.
	picked := render(t, screen, "/diff?layer=local&member=demo%2Fdemo-builder")
	mustContain(t, "the diff", picked, i18n.Tf("web.diff.member_same", "demo-builder"))
	mustNotContain(t, "the diff", picked, `class="dl del"`)
}

// TestFoldFoldsLongRunsOfUnchangedLines: a run of unchanged lines longer than
// four keeps its first two and its last two, with one row naming what it folded;
// shorter runs and changed lines are drawn as they are (試作 diffHtml の flush).
func TestFoldFoldsLongRunsOfUnchangedLines(t *testing.T) {
	lines := []Line{{Kind: Plus, Text: "new"}}
	for range 10 {
		lines = append(lines, Line{Kind: Same, Text: "same"})
	}
	lines = append(lines, Line{Kind: Minus, Text: "gone"})

	got := Fold(lines)
	if len(got) != 7 {
		t.Fatalf("the folded diff holds %d rows, want the two changed lines, four kept, and one fold: %+v", len(got), got)
	}
	if got[0].Kind != Plus || got[len(got)-1].Kind != Minus {
		t.Errorf("the folded diff starts %v and ends %v, want the changed lines", got[0], got[len(got)-1])
	}
	if skip := got[3]; skip.Kind != Skip || skip.N != 6 {
		t.Errorf("the folded run is %+v, want one row folding 6 unchanged lines", skip)
	}
	for _, i := range []int{1, 2, 4, 5} {
		if got[i].Kind != Same {
			t.Errorf("row %d is %v, want an unchanged line kept around the fold", i, got[i])
		}
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
	mustNotContain(t, "the diff", body, `class="code"`)
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
