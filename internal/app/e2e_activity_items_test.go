//go:build e2e

package app

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/i18n"
)

// TestE2EActivityItems walks checks/activity.md's items that TestE2EActivity
// does not name one by one: the tab alone and the words that are gone (S2, T3),
// the definition under a row's name and the rows that stand open at first (S4,
// S5), the mock's classes (S7), the two tables of words the screen says (T1,
// T2), what a row says at a glance (C1), what the opened rows of the team that
// waits for the human and the one that waits for a permission say (C2, C4, C5,
// C6), letting the pane's held lines out (P2), the widths and the four lights
// (W2, D1).
//
// It runs over the same real-scale fixture as TestE2EActivity — testdata/teams-v2
// with testdata/activity over it, a running mock herdr and gy — and the serve a
// person runs, with the run's own clock and zone (r-f647). No walk here changes
// the fixture, so the order they stand in is not their own.
func TestE2EActivityItems(t *testing.T) {
	// The run's clock and zone are its own (r-f647), as in TestE2EActivity: what
	// the rows say does not depend on when or where the run happens.
	began := time.Now()
	wasClock, wasZone := serveClock, time.Local
	serveClock = func() time.Time { return e2eActivityNow.Add(time.Since(began)) }
	time.Local = e2eActivityZone
	t.Cleanup(func() { serveClock, time.Local = wasClock, wasZone })

	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	initLocales(t)
	f := newFixture(t)
	at := copyActivityFixture(t, f, cwd)
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	base, stop := startServe(t, f)
	defer stop()

	browser := e2eChrome(t, chrome)
	ctx, closeTab := e2eTab(t, browser, 30*time.Minute)
	defer closeTab()
	console := e2eListen(ctx, false)
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	// The shape of one screen: its tab, the definition under a name, the rows
	// that stand open, and the classes the design's prototype names.
	t.Run("ACTIVITY-S2 the screen's own tab", func(t *testing.T) { e2eActivityItemTab(t, ctx, base) })
	t.Run("ACTIVITY-S4 the definition under the name", func(t *testing.T) { e2eActivityItemDefinition(t, ctx, base) })
	t.Run("ACTIVITY-S5 the rows that stand open at first", func(t *testing.T) { e2eActivityItemFirstOpen(t, ctx, base) })
	t.Run("ACTIVITY-S7 the mock's classes", func(t *testing.T) { e2eActivityItemClasses(t, ctx, base) })

	// The words.
	t.Run("ACTIVITY-T1 the list's words", func(t *testing.T) { e2eActivityItemListWords(t, ctx, base) })
	t.Run("ACTIVITY-T2 the opened row's words", func(t *testing.T) { e2eActivityItemOpenedWords(t, ctx, base) })
	t.Run("ACTIVITY-T3 the words that are gone", func(t *testing.T) { e2eActivityItemNoOldWords(t, ctx, base) })

	// The scenes.
	t.Run("ACTIVITY-C1 the two who wait", func(t *testing.T) { e2eActivityItemAtAGlance(t, ctx, base) })
	t.Run("ACTIVITY-C2 the questions of the team that waits", func(t *testing.T) { e2eActivityItemQuestions(t, ctx, base, f) })
	t.Run("ACTIVITY-C4 the members that wait", func(t *testing.T) { e2eActivityItemWaiting(t, ctx, base) })
	t.Run("ACTIVITY-C5 the row's いつから", func(t *testing.T) { e2eActivityItemSince(t, ctx, base, at) })
	t.Run("ACTIVITY-C6 the crew's order", func(t *testing.T) { e2eActivityItemCrewOrder(t, ctx, base) })

	// The presses and the widths and lights.
	t.Run("ACTIVITY-P2 letting the pane's lines out", func(t *testing.T) { e2eActivityItemPane(t, ctx, base) })
	t.Run("ACTIVITY-W2 no width flows sideways", func(t *testing.T) { e2eActivityItemWidths(t, ctx, base) })
	t.Run("ACTIVITY-D1 the four lights read", func(t *testing.T) { e2eActivityItemLights(t, ctx, base) })

	if lines := console.Lines(); len(lines) != 0 {
		t.Errorf("the screens wrote to the console: %v", lines)
	}
}

// e2eActivityItemTab is S2: the screen's own tab is 一覧 alone, with no YAML tab
// and no number of its own.
func e2eActivityItemTab(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	if got := e2eRead[string](t, ctx, `Array.from(document.querySelectorAll('.top .tab')).map(function(e){return e.textContent}).join(',')`); got != "一覧" {
		t.Errorf("the screen's tabs read %q, want 一覧 alone", got)
	}
}

// e2eActivityItemDefinition is S4: the row of a team that was booted from a
// definition of another name writes 定義 <名前> under its name, and no other row
// does.
func e2eActivityItemDefinition(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	got := e2eRead[string](t, ctx, `(function(){var out=[];document.querySelectorAll('#list .act').forEach(function(a){`+
		`var d=a.querySelector('.c-name .def');if(d)out.push(a.dataset.team+':'+d.textContent)});return out.join(',')})()`)
	if got != "core-hotfix:定義 core" {
		t.Errorf("the rows naming their definition read %q, want core-hotfix alone", got)
	}
	// And the words the list writes are 定義 <名前>, not just 定義 (T2's own part
	// is checked by ACTIVITY-T2).
	if !strings.Contains(got, "定義 core") {
		t.Errorf("the definition reads %q, want it under the name as 定義 core", got)
	}
}

// e2eActivityItemFirstOpen is S5: opening the screen with no open in the address
// leaves the teams waiting for the human or stopped open, and the working one
// shut.
func e2eActivityItemFirstOpen(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	if got := e2eRead[string](t, ctx, activityStands); got != "core=open,migrate=open,core-hotfix=open,research=shut" {
		t.Errorf("the rows stand %q, want core, migrate and core-hotfix open and research shut", got)
	}
}

// activityMockClasses is the class names the design's prototype (mock-activity
// 版 1) writes, which spec-activity 0 章 says the screen copies as they stand
// (S7). The prototype's two states that the screen draws are the ones under a
// team or a member (open, last, answer, turn, stalled, working, blocked, handed,
// active, idle, absent) as well as the ones it names in its markup. It is the
// whole vocabulary the two sides share: the screen may write no class outside it
// (with the two carve-outs below).
var activityMockClasses = []string{
	"abody", "act", "ago", "ahead", "ar", "ask", "at", "bar",
	"c-name", "c-since", "c-st", "c-who", "chev", "crew", "crumb",
	"def", "dot", "hm", "lb", "len", "list", "mark", "more", "mrow",
	"n", "nm", "nodes", "on", "opts", "q", "qh", "qid", "qt", "rest",
	"rl", "scr", "screens", "sep", "st", "sub", "sw", "t", "tab",
	"tabs", "tail", "term", "th", "top", "wait", "who2", "wk", "wrap",
	"open", "last", "answer", "turn", "stalled", "working",
	"blocked", "handed", "active", "idle", "absent",
}

// activityMockStanding is the part of that vocabulary the fixture itself draws,
// so S7 can hold the screen to writing it. absent is left out: it is a member
// the definition names and no agent runs, and this fixture runs an agent for
// every member (the mock's own DATA hand-writes one), so no honest check of that
// class exists over this fixture — the reverse check below still holds the
// screen to not inventing a class in its place.
var activityMockStanding = []string{
	"abody", "act", "ago", "ahead", "ar", "ask", "at", "bar",
	"c-name", "c-since", "c-st", "c-who", "chev", "crew", "crumb",
	"def", "dot", "hm", "lb", "len", "list", "mark", "more", "mrow",
	"n", "nm", "nodes", "on", "opts", "q", "qh", "qid", "qt", "rest",
	"rl", "scr", "screens", "sep", "st", "sub", "sw", "t", "tab",
	"tabs", "tail", "term", "th", "top", "wait", "who2", "wk", "wrap",
	"open", "last", "answer", "turn", "stalled", "working",
	"blocked", "handed", "active", "idle",
}

// activityMockSampleOnly is the prototype's own terminal-sample styling: it
// paints its hand-written tail lines with cur and dim (mock-activity.html の
// DATA). The real screen gets text only — the answer carries lines of text and no
// class (V1) — and 2.4 says herdr's own text goes out as it stands, so the screen
// neither must nor can write those two.
var activityMockSampleOnly = []string{"cur", "dim"}

// activityScreenOwn is the class the screen names that the prototype does not: the
// one line it says when no team runs or herdr cannot be read (2.7, `gone`). The
// prototype always has four teams, so it never draws it.
var activityScreenOwn = []string{"gone"}

// e2eActivityItemClasses is S7: standing beside the design's prototype, the
// screen's class names are the same. Every class the prototype names that this
// fixture draws stands somewhere on the page, and the page writes no class the
// prototype does not (the prototype's own terminal-sample styling and the
// screen's own one line for an empty list are the two names the two sides do not
// share, as above).
func e2eActivityItemClasses(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	// Every row open at 2000, so every part the prototype draws — the chosen
	// member (last), every state, the workers and the panes — stands somewhere.
	activityOpen(t, ctx, base, "?open=core,migrate,core-hotfix,research")
	classes := e2eRead[[]string](t, ctx, `(function(){var out={};`+
		`document.querySelectorAll('*').forEach(function(e){if(e.classList)e.classList.forEach(function(c){out[c]=1})});`+
		`return Object.keys(out).sort()})()`)
	mine := map[string]bool{}
	for _, class := range classes {
		mine[class] = true
	}
	for _, want := range activityMockStanding {
		if !mine[want] {
			t.Errorf("the screen writes no %q, which the mock names", want)
		}
	}
	allowed := map[string]bool{}
	for _, class := range activityMockClasses {
		allowed[class] = true
	}
	for _, class := range append(append([]string{}, activityMockSampleOnly...), activityScreenOwn...) {
		allowed[class] = true
	}
	for _, got := range classes {
		if !allowed[got] {
			t.Errorf("the screen writes %q, which the mock does not name", got)
		}
	}
}

// e2eActivityItemListWords is T1: the words the list itself says. から, より前
// and 以上 are the server's いつから and stand on the rows that show one.
func e2eActivityItemListWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	text := e2eRead[string](t, ctx, `document.querySelector('#list').textContent`)
	for _, word := range []string{
		"許可待ち", "人間の番", "止まっている", "仕事中", "最後のターン", "人間", "から", "より前", "以上",
	} {
		if !strings.Contains(text, word) {
			t.Errorf("the list never says %q, which 7 章 lists", word)
		}
	}
}

// e2eActivityItemOpenedWords is T2: the words the opened rows say — the heading
// of the left column, the pane and its held lines, the workers, and the states
// and the definition the prototype names.
func e2eActivityItemOpenedWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=core,migrate,core-hotfix,research")
	text := e2eRead[string](t, ctx, `document.querySelector('#list').textContent`)
	for _, word := range []string{
		"ロール lead", "画面の最後", "▸ 上に 2 行", "ワーカー", "人間に渡した", "動いていない", "定義 core",
	} {
		if !strings.Contains(text, word) {
			t.Errorf("the opened rows never say %q, which 7 章 lists", word)
		}
	}
}

// e2eActivityItemNoOldWords is T3: none of the words the old screens used is
// anywhere on this one, with every row open.
func e2eActivityItemNoOldWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=core,migrate,core-hotfix,research")
	text := e2eRead[string](t, ctx, `document.body.textContent`)
	for _, word := range []string{"層", "layer", "担当", "役割", "版", "台帳"} {
		if strings.Contains(text, word) {
			t.Errorf("the screen says %q, which this screen never says", word)
		}
	}
}

// e2eActivityItemAtAGlance is C1: without opening a row, its いま and 誰 alone
// say that core's lead and migrate's tester are the ones the human is waited by.
func e2eActivityItemAtAGlance(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	for _, one := range []struct{ team, state, who, reading string }{
		{"core", "人間の番", "最後のターンlead→人間", "core の lead が人間の番"},
		{"migrate", "許可待ち", "tester", "migrate の tester が許可待ち"},
	} {
		parts := strings.Split(activityRowText(t, ctx, one.team), "|")
		if parts[0] != one.state || parts[1] != one.who {
			t.Errorf("%s reads %q|%q, want %q|%q (%s)", one.team, parts[0], parts[1], one.state, one.who, one.reading)
		}
	}
}

// e2eActivityItemQuestions is C2: core's opened part shows its lead's newest
// question with its two options, then its two older ones in one line each, with
// the server's 何分前. Of the fixture's 55 open questions the screen shows four,
// and the 51 the rows' own members did not ask stand nowhere.
func e2eActivityItemQuestions(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	ask := `document.querySelector('#list .act[data-team="core"] .q')`

	// q-7c2e first: its id, its title, and its two options.
	if got := e2eRead[string](t, ctx, ask+`.querySelector('.qh .qid').textContent`); got != "q-7c2e" {
		t.Errorf("core's first question reads %q, want q-7c2e", got)
	}
	if got := e2eRead[string](t, ctx, ask+`.querySelector('.qt').textContent`); got != "注文 API のページ送りを cursor に替えるか" {
		t.Errorf("core's first question's title reads %q", got)
	}
	wantOptions := "cursor に替える。古いクライアントのために offset を 2 つ先のバージョンまで残す / offset のままにし、1 回に返す件数の上限を 1,000 件にする"
	if got := e2eRead[string](t, ctx, `Array.from(`+ask+`.querySelectorAll('.opts>li')).map(function(e){return e.textContent}).join(' / ')`); got != wantOptions {
		t.Errorf("core's first question's options read %q, want the two", got)
	}
	if got := e2eRead[string](t, ctx, `String(`+ask+`.querySelectorAll('.opts>li').length)`); got != "2" {
		t.Errorf("core's first question shows %s options, want 2", got)
	}
	// q-51b9 and q-0d4a one line each, with the server's 何分前 (1500 and 2900
	// minutes before the run's own clock: one day and two days).
	if got := e2eRead[string](t, ctx, `Array.from(`+ask+`.querySelectorAll('.more .mrow')).map(function(e){`+
		`return e.querySelector('code').textContent+' '+e.querySelector('.ago').textContent}).join(' / ')`); got != "q-51b9 1 日前 / q-0d4a 2 日前" {
		t.Errorf("the questions after core's first read %q, want q-51b9 1 日前 and q-0d4a 2 日前", got)
	}
	// The screen shows four questions: core's three and core-hotfix's one.
	shown := e2eRead[string](t, ctx, `(function(){var out=[];document.querySelectorAll('#list .q').forEach(function(q){`+
		`var id=q.querySelector('.qid');if(id)out.push(id.textContent);`+
		`q.querySelectorAll('.more .mrow code').forEach(function(c){out.push(c.textContent)})});return out.join(',')})()`)
	if shown != "q-7c2e,q-51b9,q-0d4a,q-9e10" {
		t.Errorf("the screen shows %q, want the four the rows' own members asked", shown)
	}
	// Of the fixture's 55 open questions, the 51 that are not among those stand
	// nowhere on the screen.
	questions := activityQuestions(t, f)
	if len(questions) != 55 {
		t.Fatalf("the fixture holds %d open questions, want 55", len(questions))
	}
	here := map[string]bool{"q-7c2e": true, "q-51b9": true, "q-0d4a": true, "q-9e10": true}
	text := e2eRead[string](t, ctx, `document.querySelector('#list').textContent`)
	hidden := 0
	for _, question := range questions {
		if here[question.ID] {
			continue
		}
		hidden++
		if strings.Contains(text, question.ID) || strings.Contains(text, question.Title) {
			t.Errorf("the screen says %s %q, which the rows' own members did not ask", question.ID, question.Title)
		}
	}
	if hidden != 51 {
		t.Errorf("the fixture holds %d questions the screen must hide, want 51", hidden)
	}
}

// e2eActivityItemWaiting is C4: in the members' column, core's lead alone is
// 人間に渡した and every other row is 止まっている, while migrate's impl is
// 仕事中.
func e2eActivityItemWaiting(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=core,migrate")
	handed := map[string]bool{}
	for _, row := range e2eActivityItemCrew(t, ctx, "core") {
		name, state, _ := strings.Cut(row, "|")
		if name == "" {
			continue // the ワーカー heading
		}
		switch state {
		case "人間に渡した":
			handed[name] = true
		case "止まっている":
		default:
			t.Errorf("core's %s reads %q, want 止まっている", name, state)
		}
	}
	if len(handed) != 1 || !handed["lead"] {
		t.Errorf("core's member that is 人間に渡した is %v, want its lead alone", handed)
	}
	working := map[string]bool{}
	for _, row := range e2eActivityItemCrew(t, ctx, "migrate") {
		name, state, _ := strings.Cut(row, "|")
		if name == "" {
			continue
		}
		if state == "仕事中" {
			working[name] = true
		}
	}
	if !working["impl"] {
		t.Errorf("migrate's impl does not read 仕事中: %v", working)
	}
}

// e2eActivityItemSince is C5: core's row says 最後のターン lead → 人間 with 2 時間
// 40 分 from a clock, and core-hotfix's says 最後のターン impl-a with 4 時間 5
// 分以上 from a moment that is not today's — the words the server writes.
func e2eActivityItemSince(t *testing.T, ctx context.Context, base string, at time.Time) {
	t.Helper()
	activityOpen(t, ctx, base, "")
	for _, one := range []struct {
		team, who string
		waited    time.Duration
		lower     bool
	}{
		{"core", "最後のターンlead→人間", 160 * time.Minute, false},
		{"core-hotfix", "最後のターンimpl-a", 245 * time.Minute, true},
	} {
		parts := strings.Split(activityRowText(t, ctx, one.team), "|")
		if parts[1] != one.who {
			t.Errorf("%s reads %q, want %q", one.team, parts[1], one.who)
		}
		length, moment := activitySpan(t, ctx, one.team)
		if !strings.Contains(strings.Join(activityLengths(one.waited, one.lower), ","), length) {
			t.Errorf("%s has waited %q, want %v", one.team, length, activityLengths(one.waited, one.lower))
		}
		if want := activity.SinceAt(at, at.Add(-one.waited), one.lower); moment != want {
			t.Errorf("%s has waited since %q, want %q", one.team, moment, want)
		}
	}
}

// e2eActivityItemCrewOrder is C6: core's members stand in the order of the
// moment they stopped, the newest first.
func e2eActivityItemCrewOrder(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=core")
	if got := strings.Join(e2eActivityItemMembers(t, ctx, "core"), ","); got != "lead,impl-a,impl-b,reviewer,analyst" {
		t.Errorf("core's members stand %q, want the newest stop first", got)
	}
}

// e2eActivityItemMembers is one row's members (not its workers) in the order the
// crew draws them (C4・C6).
func e2eActivityItemMembers(t *testing.T, ctx context.Context, team string) []string {
	t.Helper()
	q := strconv.Quote(team)
	return e2eRead[[]string](t, ctx, `(function(){var out=[],workers=false;`+
		`document.querySelectorAll('#list .act[data-team='+JSON.stringify(`+q+`)+'] .crew .nodes>li').forEach(function(li){`+
		`if(li.classList.contains('sub')){workers=true;return}if(workers)return;`+
		`var n=li.querySelector('.nm');if(n)out.push(n.textContent)});return out})()`)
}

// e2eActivityItemPane is P2: pressing ▸ 上に N 行 lets the last two lines of
// migrate's pane out and its heading becomes ▾ 上の行をしまう, and pressing it
// puts them back — the pane alone, the rows around it untouched.
func e2eActivityItemPane(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	activityOpen(t, ctx, base, "?open=migrate")
	tail := `document.querySelector('#list .act[data-team="migrate"] .tail')`
	lines := func() string {
		return e2eRead[string](t, ctx, `String(`+tail+`.querySelector('.term').textContent.split('\n').length)`)
	}
	if got := lines(); got != "6" {
		t.Fatalf("migrate's pane shows %s lines before the press, want the six the server sent", got)
	}
	others := e2eRead[string](t, ctx, activityOthersScript("migrate"))
	e2ePressWhenEnabled(t, ctx, tail+`.querySelector('.th .rest')`)
	e2eWait(t, ctx, `(`+tail+`.querySelector('.term').textContent.split('\n').length === 8)`)
	if got := e2eRead[string](t, ctx, tail+`.querySelector('.th').textContent`); got != "画面の最後▾ 上の行をしまう" {
		t.Errorf("the extended pane stands under %q, want 上の行をしまう", got)
	}
	_, pane := activityTail(t, ctx, "migrate")
	if !strings.Contains(strings.Join(pane, "\n"), "tell Codex what to do differently") || !strings.Contains(strings.Join(pane, "\n"), "分ける SQL") {
		t.Errorf("the two lines that were let out are not the pane's own first two")
	}
	if got := e2eRead[string](t, ctx, activityOthersScript("migrate")); got != others {
		t.Errorf("extending one pane moved what the screen says elsewhere")
	}
	e2ePressWhenEnabled(t, ctx, tail+`.querySelector('.th .rest')`)
	e2eWait(t, ctx, `(`+tail+`.querySelector('.term').textContent.split('\n').length === 6)`)
	if got := e2eRead[string](t, ctx, tail+`.querySelector('.th').textContent`); got != "画面の最後▸ 上に 2 行" {
		t.Errorf("the pane put back stands under %q, want 上に 2 行", got)
	}
}

// e2eActivityItemWidths is W2: at every width the page takes the window's own
// width and no more, and at 500 the pane's long psql line wraps instead of
// running off its box.
func e2eActivityItemWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, size := range []int64{2000, 1440, 500} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
		activityOpen(t, ctx, base, "?open=core,migrate")
		if over := e2eRead[string](t, ctx, `String(document.documentElement.scrollWidth - window.innerWidth)`); over != "0" && over != "1" {
			t.Errorf("at %d the page flows %s px sideways", size, over)
		}
		if over := e2eRead[string](t, ctx, `String(document.body.scrollWidth - window.innerWidth)`); over != "0" && over != "1" {
			t.Errorf("at %d the body flows %s px sideways", size, over)
		}
	}
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 1000))
	activityOpen(t, ctx, base, "?open=migrate")
	pane := `document.querySelector('#list .act[data-team="migrate"] .term')`
	if !strings.Contains(e2eRead[string](t, ctx, pane+`.textContent`), `$ psql "$STAGING_URL" -f migrate/0042_split_ledger.sql`) {
		t.Errorf("migrate's pane does not show the psql line at 500")
	}
	if got := e2eRead[string](t, ctx, `String(`+pane+`.scrollWidth-`+pane+`.clientWidth)`); got != "0" && got != "1" {
		t.Errorf("at 500 the pane's long line runs %s px past its box, want it wrapped", got)
	}
	if got := e2eRead[string](t, ctx, `getComputedStyle(`+pane+`).whiteSpace`); got != "pre-wrap" {
		t.Errorf("the pane reads %q, want its lines wrapped", got)
	}
}

// e2eActivityItemLights is D1: the four ways the screen can be lit — the OS's
// light, the OS's dark, and the theme written into the page — all read, the ink
// of the rows, the state pills and the pane staying legible against its ground.
func e2eActivityItemLights(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	activityOpen(t, ctx, base, "?open=core,migrate")
	lights := []struct {
		name  string
		dark  bool
		theme string
	}{
		{"light", false, ""},
		{"dark", true, ""},
		{`data-theme="light"`, true, "light"},
		{`data-theme="dark"`, false, "dark"},
	}
	for _, light := range lights {
		if light.dark {
			e2eDark(t, ctx)
		} else {
			e2eActivityLight(t, ctx)
		}
		e2eTheme(t, ctx, light.theme)
		if got := e2eRead[float64](t, ctx, e2eRolesContrast); got < 4.5 {
			t.Errorf("in %s the body's ink against its ground is %.2f, want at least 4.5", light.name, got)
		}
		for _, one := range []struct{ what, find string }{
			{"the row's name", `#list .act[data-team="core"] .c-name .nm`},
			{"the waiting pill", `#list .act[data-team="migrate"] .st.answer`},
			{"the pane", `#list .act[data-team="core"] .term`},
		} {
			if got := e2eRead[float64](t, ctx, e2eActivityItemInk(one.find)); got < 4.5 {
				t.Errorf("in %s %s reads at %.2f against its ground, want at least 4.5", light.name, one.what, got)
			}
		}
	}
	e2eTheme(t, ctx, "")
}

// e2eActivityItemInk is the contrast of one element's text against the ground it
// stands on: the colour of its own text, and of the first ancestor that paints a
// ground (D1). It is e2eRolesContrast made to work for an element inside the page.
func e2eActivityItemInk(find string) string {
	q := strconv.Quote(find)
	return `(function(){` +
		`var lum=function(c){var m=c.match(/[0-9.]+/g).map(Number).slice(0,3).map(function(v){v=v/255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);});return 0.2126*m[0]+0.7152*m[1]+0.0722*m[2];};` +
		`var el=document.querySelector(` + q + `);if(!el) throw new Error('no element for '+` + q + `);` +
		`var fg=lum(getComputedStyle(el).color),node=el,bg=null;` +
		`while(node){var s=getComputedStyle(node).backgroundColor;if(s&&s!=='rgba(0, 0, 0, 0)'){bg=lum(s);break;}node=node.parentElement;}` +
		`if(bg===null)bg=lum(getComputedStyle(document.body).backgroundColor);` +
		`var hi=Math.max(fg,bg),lo=Math.min(fg,bg);return (hi+0.05)/(lo+0.05);})()`
}

// e2eActivityItemCrew is the crew reader the walks above share: one row's crew as
// lines of name and state.
func e2eActivityItemCrew(t *testing.T, ctx context.Context, team string) []string {
	t.Helper()
	q := strconv.Quote(team)
	return e2eRead[[]string](t, ctx, `(function(){var c=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+'] .crew');`+
		`if(!c) throw new Error('no crew: '+`+q+`);`+
		`return Array.from(c.querySelectorAll('.nodes>li')).map(function(li){`+
		`var n=li.querySelector('.nm'),s=li.querySelector('.sw');`+
		`return (n?n.textContent:'')+'|'+(s?s.textContent:'')})})()`)
}
