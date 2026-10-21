//go:build e2e

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// TestE2EActivity walks the activity screen over the real-scale fixture the mock
// was drawn from (spec-activity 5 章). It holds the screen to 7 章: the band's
// two numbers (S1), the screen's own tab (S2), the four rows and the order they
// stand in (S3〜S5), what each row says at a glance (C1・C4・C5、T1・T2), the two
// opened parts (S6, C2・C3・C6), the presses that must move only what stands
// under them (P1〜P4), the three widths and both lights (W1・W2、D1・D2), the
// words the server makes rather than the script (V1〜V3), and the ways herdr and
// gy do not answer (E1〜E5).
//
// Two readings here are deliberate, and the run's record says so as well. The
// rows stand in activity.Sort's order (2.2: the longest wait for the human
// first), which puts core's two hours and forty minutes above migrate's eighteen
// — the mock's DATA is a hand-written order and the mock does not sort. And
// ほか N 件 (P3) needs five questions for one member, which the fixture's own
// numbers never reach (5.3 gives the most of them to core's lead: three), so
// that one press is walked on a gy that answers with three more of the same
// member's questions.
func TestE2EActivity(t *testing.T) {
	// The run's clock and zone are its own (r-f647): the serve reads its moment
	// from the first, and a moment is written in the zone of the second, so what
	// the rows say does not depend on when or where the run happens. Both are put
	// back when the test is done, after the serves it started have stopped.
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
	copyActivityFixture(t, f, cwd)
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

	// The 大 item: reading again while the fixture changes under the page
	// (ACTIVITY-P4). Every other item is held by a non-browser test at its own
	// size.
	t.Run("ACTIVITY-P4 reading again", func(t *testing.T) { e2eActivityAgain(t, ctx, base, f) })

	if lines := console.Lines(); len(lines) != 0 {
		t.Errorf("the screens wrote to the console: %v", lines)
	}
}

// ---- reading the screen ----------------------------------------------------------------

// activityDrawn is the list having been drawn: rows, or the one line a list with
// nothing to show carries.
const activityDrawn = `document.querySelectorAll('#list .act').length > 0 || !!document.querySelector('#list .gone')`

// activityNames is the teams the screen lists, in the order it lists them (2.2).
const activityNames = `Array.from(document.querySelectorAll('#list .act')).map(function(a){return a.dataset.team}).join(',')`

// activityStands is every row with whether it is open, in the order drawn (2.2・2.3).
const activityStands = `Array.from(document.querySelectorAll('#list .act')).map(function(a){return a.dataset.team+'='+(a.classList.contains('open')?'open':'shut')}).join(',')`

// activityBandScript is the band's two numbers as the reading leaves them: the
// 活動 number and the 待ち word, or none where they are not written (2.1・2.7).
const activityBandScript = `(function(){var s=document.querySelector('.screens .scr[data-screen="activity"]');` +
	`if(!s) throw new Error('no activity band');` +
	`var n=s.querySelector('.n'),w=s.querySelector('.wait');` +
	`return [n?n.textContent:'none',w?w.textContent:'none'].join('|')})()`

// activityOthers is every row but one, as the screen drew them: what a press on
// that row must leave alone (P1〜P3).
func activityOthersScript(team string) string {
	return `Array.from(document.querySelectorAll('#list .act')).filter(function(a){return a.dataset.team!==` + strconv.Quote(team) + `})` +
		`.map(function(a){return a.dataset.team+':'+a.textContent}).join('\n')`
}

// activityRow is one row as it reads at a glance: the word of いま, who the row
// is about, and いつから (2.2).
func activityRowScript(team string) string {
	q := strconv.Quote(team)
	return `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(` + q + `)+']');` +
		`if(!a) throw new Error('no row for '+` + q + `);` +
		`return [a.querySelector('.c-st .st').textContent,a.querySelector('.c-who').textContent,a.querySelector('.c-since').textContent].join('|');})()`
}

// activityRowText is one row as it reads at a glance.
func activityRowText(t *testing.T, ctx context.Context, team string) string {
	t.Helper()
	return e2eRead[string](t, ctx, activityRowScript(team))
}

// activitySpan is one row's いつから in two parts: the length and the moment it
// began, or an empty word for a row that carries none (a working team).
func activitySpan(t *testing.T, ctx context.Context, team string) (string, string) {
	t.Helper()
	q := strconv.Quote(team)
	got := e2eRead[string](t, ctx, `(function(){var r=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+']'),s=r&&r.querySelector('.c-since');`+
		`if(!s) return '';`+
		`return [(s.querySelector('.len')||{}).textContent||'',(s.querySelector('.at')||{}).textContent||''].join('|');})()`)
	if got == "" {
		return "", ""
	}
	parts := strings.SplitN(got, "|", 2)
	return parts[0], parts[1]
}

// activityShape is one row's opened part as a shape: the two columns, the left
// column's own parts, and the right column's rows with their states (S6, C6).
func activityShape(t *testing.T, ctx context.Context, team string) string {
	t.Helper()
	q := strconv.Quote(team)
	return e2eRead[string](t, ctx, `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+']'),ab=a&&a.querySelector('.abody');`+
		`if(!ab) throw new Error('not open: '+`+q+`);`+
		`var own=function(el){return el?Array.from(el.children).map(function(e){return (e.className||e.tagName.toLowerCase()).split(' ')[0]}).join('>'):'none'};`+
		`var ask=ab.querySelector('.ask'),crew=ab.querySelector('.crew');`+
		`return [own(ab),own(ask),own(crew),crew?Array.from(crew.querySelectorAll('.nodes>li')).map(function(e){return e.className}).join(','):''].join(' | ');})()`)
}

// activityCrew is one row's right column, one line each: the class, the member's
// name, the state's word, and the moment (2.5).
func activityCrew(t *testing.T, ctx context.Context, team string) []string {
	t.Helper()
	q := strconv.Quote(team)
	return e2eRead[[]string](t, ctx, `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+']'),c=a&&a.querySelector('.crew');`+
		`if(!c) throw new Error('no members: '+`+q+`);`+
		`return Array.from(c.querySelectorAll('.nodes>li')).map(function(e){`+
		`var nm=e.querySelector('.nm'),sw=e.querySelector('.sw'),t=e.querySelector('.t');`+
		`return [e.className,nm?nm.textContent:'',sw?sw.textContent:'',t?t.textContent:''].join('|')})})()`)
}

// activityQuestion is one row's question section: the first question's id, its
// title, its options, the lines after it, and the word of the rest (2.4).
func activityQuestion(t *testing.T, ctx context.Context, team string) string {
	t.Helper()
	q := strconv.Quote(team)
	return e2eRead[string](t, ctx, `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+']'),s=a&&a.querySelector('.q');`+
		`if(!s) throw new Error('no question: '+`+q+`);`+
		`var rest=s.querySelector('.more .rest');`+
		`return [s.querySelector('.qh .qid').textContent,s.querySelector('.qt').textContent,`+
		`Array.from(s.querySelectorAll('.opts>li')).map(function(e){return e.textContent}).join(' / '),`+
		`Array.from(s.querySelectorAll('.more .mrow')).map(function(e){return e.querySelector('code').textContent+' '+e.querySelector('span').textContent}).join(' / '),`+
		`rest?rest.textContent:''].join('§');})()`)
}

// activityTail is one row's pane as it reads: the heading, and the lines the
// screen shows of it (2.4).
func activityTail(t *testing.T, ctx context.Context, team string) (string, []string) {
	t.Helper()
	q := strconv.Quote(team)
	got := e2eRead[string](t, ctx, `(function(){var a=document.querySelector('#list .act[data-team='+JSON.stringify(`+q+`)+']'),t=a&&a.querySelector('.tail');`+
		`if(!t) throw new Error('no pane: '+`+q+`);`+
		`return t.querySelector('.th').textContent+'\u0000'+t.querySelector('.term').textContent;})()`)
	head, body, _ := strings.Cut(got, "\u0000")
	if body == "" {
		return head, nil
	}
	return head, strings.Split(body, "\n")
}

// activityAwait waits for one expression beyond one act's own deadline: the
// screen reads again every ten seconds (2.6), so a change made behind it lands a
// poll later.
func activityAwait(t *testing.T, ctx context.Context, script string, span time.Duration) {
	t.Helper()
	deadline := time.Now().Add(span)
	for time.Now().Before(deadline) {
		if ok, _, answered := e2eLook[bool](t, ctx, script); answered && ok {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("never became true within %s: %s", span, script)
}

// activitySettles loads the screen again until it stands as asked. Two things
// make this different from waiting inside one page: the reading of herdr is kept
// for five seconds (3 章), so a fixture or an environment changed just now is not
// in the server's answer yet, and the page's shell — the band's two numbers with
// it — is written when the page is made and never read again by the script.
func activitySettles(t *testing.T, ctx context.Context, base, script string, span time.Duration) {
	t.Helper()
	deadline := time.Now().Add(span)
	for {
		e2eRun(t, ctx, chromedp.Navigate(base+"/activity"))
		e2eWait(t, ctx, activityDrawn)
		if ok, _, answered := e2eLook[bool](t, ctx, script); answered && ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the screen never stood as asked within %s: %s", span, script)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// activityOpen opens the screen on one address and waits for the answer to be
// drawn.
func activityOpen(t *testing.T, ctx context.Context, base, query string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/activity"+query))
	e2eWait(t, ctx, activityDrawn)
}

// activityLengths is what the screen may write for a state that began at one
// moment. The words are the server's (V1), and a minute of drift is accepted:
// the screen is read a moment after the fixture was counted from, while the
// server writes whole minutes.
func activityLengths(d time.Duration, lower bool) []string {
	return []string{activity.Length(d, lower), activity.Length(d+time.Minute, lower)}
}

// activityMoment is one moment as the screen writes it (C5, V1): the clock while
// it is today's and the date before it when it is another day's — the server's
// own rule, read at the run's own clock. The clock and the zone are the run's
// (e2eActivityNow, e2eActivityZone), so which moments carry a date is the same
// wherever and whenever the run happens (r-f647).
func activityMoment(moment time.Time) string {
	return activity.Clock(e2eActivityNow, moment)
}

// ---- the walks --------------------------------------------------------------------------

// e2eActivityBand is S1〜S2 and T3: 活動 carries the two numbers and leads to the
// screen, the screen the person is on is marked, its own tab is 一覧 alone, and
// none of the words of the old screens is here.
func e2eActivityBand(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	// S1: from another screen, 活動 4 and 待ち 2 stand on a link to the activity.
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"))
	e2eWait(t, ctx, `!!document.querySelector('.screens .scr[data-screen="activity"]')`)
	link := `document.querySelector('.screens .scr[data-screen="activity"]')`
	got := e2eRead[string](t, ctx, `(function(){var a=`+link+`;return a.tagName+'|'+a.getAttribute('href')+'|'+a.textContent})()`)
	if got != "A|/activity|活動4待ち 2" {
		t.Errorf("the band's 活動 reads %q, want a link to /activity with 活動 4 and 待ち 2", got)
	}
	e2ePressWhenEnabled(t, ctx, link)
	e2eWait(t, ctx, `location.pathname === '/activity' && document.querySelectorAll('#list .act').length === 4`)
	// 居ると濃い地: here it is the screen the person is on, and it is no link.
	got = e2eRead[string](t, ctx, `(function(){var s=document.querySelector('.screens .scr[data-screen="activity"]');`+
		`return s.tagName+'|'+s.className+'|'+(s.getAttribute('aria-current')||'')})()`)
	if got != "SPAN|scr on|page" {
		t.Errorf("the band's 活動 on its own screen reads %q, want the screen the person is on", got)
	}
	// S2: the screen's own tab is 一覧 alone.
	if got := e2eRead[string](t, ctx, `Array.from(document.querySelectorAll('.top .tab')).map(function(e){return e.textContent}).join(',')`); got != "一覧" {
		t.Errorf("the screen's tabs read %q, want 一覧 alone", got)
	}
	// T3: none of the words the old screens used is here.
	text := e2eRead[string](t, ctx, `document.body.textContent`)
	for _, word := range []string{"層", "layer", "担当", "役割", "版", "台帳"} {
		if strings.Contains(text, word) {
			t.Errorf("the screen says %q, which this screen never says", word)
		}
	}
}

// e2eActivityRows is S3〜S5, C1, C4, C5 and T1・T2: the four rows of this
// project's running teams, the order they stand in, what each one says at a
// glance, and which of them are open at first.
func e2eActivityRows(t *testing.T, ctx context.Context, base string, at time.Time) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	activityOpen(t, ctx, base, "")

	// S3: the four teams of this project, in activity.Sort's order (2.2): the
	// longest wait for the human first, so core's 2 時間 40 分 stands above
	// migrate's 18 分. The mock's DATA is a hand-written order and the mock does
	// not sort; the record says the same.
	want := "core,migrate,core-hotfix,research"
	if got := e2eRead[string](t, ctx, activityNames); got != want {
		t.Errorf("the rows are %q, want %q", got, want)
	}
	// S5: at first the rows waiting or stopped are open, the working one is not.
	if got := e2eRead[string](t, ctx, activityStands); got != "core=open,migrate=open,core-hotfix=open,research=shut" {
		t.Errorf("the rows stand %q", got)
	}
	// S4: only core-hotfix was booted from a definition of another name.
	got := e2eRead[string](t, ctx, `(function(){var out=[];document.querySelectorAll('#list .act').forEach(function(a){`+
		`var d=a.querySelector('.c-name .def');if(d)out.push(a.dataset.team+':'+d.textContent)});return out.join(',')})()`)
	if got != "core-hotfix:定義 core" {
		t.Errorf("the rows naming their definition read %q, want core-hotfix alone", got)
	}
	// C1・C4: いま and 誰 alone say who the human is waiting for.
	for _, one := range []struct{ team, state, who string }{
		{"core", "人間の番", "最後のターンlead→人間"},
		{"migrate", "許可待ち", "tester"},
		{"core-hotfix", "止まっている", "最後のターンimpl-a"},
		{"research", "仕事中", "leadワーカー 3"},
	} {
		parts := strings.Split(activityRowText(t, ctx, one.team), "|")
		if parts[0] != one.state || parts[1] != one.who {
			t.Errorf("%s reads %q|%q, want %q|%q", one.team, parts[0], parts[1], one.state, one.who)
		}
	}
	// C5: いつから says how long and from when, and a working team has none.
	core, coreAt := activitySpan(t, ctx, "core")
	if !slices.Contains(activityLengths(160*time.Minute, false), core) {
		t.Errorf("core has waited %q, want %v", core, activityLengths(160*time.Minute, false))
	}
	if want := activity.SinceAt(at, at.Add(-160*time.Minute), false); coreAt != want {
		t.Errorf("core has waited since %q, want %q", coreAt, want)
	}
	stuck, stuckAt := activitySpan(t, ctx, "core-hotfix")
	if !slices.Contains(activityLengths(245*time.Minute, true), stuck) {
		t.Errorf("core-hotfix is stuck %q, want %v", stuck, activityLengths(245*time.Minute, true))
	}
	if want := activity.SinceAt(at, at.Add(-245*time.Minute), true); stuckAt != want {
		t.Errorf("core-hotfix is stuck since %q, want %q", stuckAt, want)
	}
	if len, moment := activitySpan(t, ctx, "research"); len != "" || moment != "" {
		t.Errorf("a working row reads いつから %q/%q, and must read none", len, moment)
	}
	// T1・T2: the words the screen says, in the row's own language.
	text := e2eRead[string](t, ctx, `document.querySelector('#list').textContent`)
	for _, word := range []string{
		"許可待ち", "人間の番", "止まっている", "仕事中", "最後のターン", "人間", "から", "より前", "以上",
		"ワーカー", "人間に渡した", "ロール", "画面の最後", "▸ 上に 2 行", "定義 core",
	} {
		if !strings.Contains(text, word) {
			t.Errorf("the screen never says %q, which 7 章 lists", word)
		}
	}
}

// e2eActivityCore is S6, C2, C4 and C6: the opened part of the team that waits
// for the human — the heading, the one question the row's own member asked, the
// pane, and the members from the newest stop.
func e2eActivityCore(t *testing.T, ctx context.Context, base string, at time.Time, f *fixture) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=core")

	// S6: the left column is the heading, the question and the pane, and the
	// right column is the members and the workers.
	shape := "ask>crew | who2>q>tail | nodes | handed last,idle,idle,idle,idle,sub,idle"
	if got := activityShape(t, ctx, "core"); got != shape {
		t.Errorf("core's opened part is %q, want %q", got, shape)
	}
	if got := e2eRead[string](t, ctx, `(function(){var s=document.querySelector('#list .act[data-team="core"] .ask');`+
		`return s.querySelector('.who2 code').textContent+'|'+s.querySelector('.who2 .rl').textContent})()`); got != "core-lead|ロール lead" {
		t.Errorf("core's opened part stands under %q, want the herdr name and the role", got)
	}
	// C2: the newest question of core's lead, its two options, then one line
	// each for the two older ones, and ほか N 件 is not there to press.
	wantQuestion := "q-7c2e§注文 API のページ送りを cursor に替えるか" +
		"§cursor に替える。古いクライアントのために offset を 2 つ先のバージョンまで残す / offset のままにし、1 回に返す件数の上限を 1,000 件にする" +
		"§q-51b9 注文の取り消しを、注文から 24 時間までに限るか / q-0d4a impl-b の結合テストを夜の CI にまわしてよいか" +
		"§"
	if got := activityQuestion(t, ctx, "core"); got != wantQuestion {
		t.Errorf("core's question reads\n%q\nwant\n%q", got, wantQuestion)
	}
	if got := e2eRead[string](t, ctx, `String(/前$/.test(document.querySelector('#list .act[data-team="core"] .qh .ago').textContent))`); got != "true" {
		t.Errorf("the question stands with a moment that is not in the server's words")
	}
	// C2: of the fixture's 55 open questions, the 52 that core's lead did not
	// ask stand nowhere on the screen.
	questions := activityQuestions(t, f)
	if len(questions) != 55 {
		t.Fatalf("the fixture holds %d open questions, want 55", len(questions))
	}
	hidden := 0
	text := e2eRead[string](t, ctx, `document.querySelector('#list').textContent`)
	for _, question := range questions {
		if question.Actor == "core-lead" {
			continue
		}
		hidden++
		if strings.Contains(text, question.ID) || strings.Contains(text, question.Title) {
			t.Errorf("the screen says %s %q, which core's lead did not ask", question.ID, question.Title)
		}
	}
	if hidden != 52 {
		t.Errorf("the fixture holds %d questions of other members, want 52", hidden)
	}
	// 画面の最後: core-lead's own pane, and all of it (six lines, nothing held).
	head, lines := activityTail(t, ctx, "core")
	if head != "画面の最後" {
		t.Errorf("core's pane stands under %q, want 画面の最後 with nothing to press", head)
	}
	if len(lines) != 6 {
		t.Errorf("core's pane shows %d lines, want its own six", len(lines))
	}
	if len(lines) > 0 && !strings.Contains(strings.Join(lines, "\n"), "impl-a 12 ファイル") {
		t.Errorf("core's pane is not herdr's own: %q", strings.Join(lines, "\n"))
	}
	// C6・C4: the members, the newest stop first, with 人間に渡した on the member
	// the line is about alone; then the workers under ワーカー.
	want := []string{
		"handed last|lead|人間に渡した|" + activityMoment(at.Add(-160*time.Minute)),
		"idle|impl-a|止まっている|" + activityMoment(at.Add(-167*time.Minute)),
		"idle|impl-b|止まっている|" + activityMoment(at.Add(-174*time.Minute)),
		"idle|reviewer|止まっている|" + activityMoment(at.Add(-192*time.Minute)),
		"idle|analyst|止まっている|" + activityMoment(at.Add(-276*time.Minute)),
		"sub|||",
		"idle|deps|止まっている|" + activityMoment(at.Add(-210*time.Minute)),
	}
	if got := activityCrew(t, ctx, "core"); !slices.Equal(got, want) {
		t.Errorf("core's members read\n%v\nwant\n%v", got, want)
	}
}

// e2eActivityMigrate is C3 and C4: the opened part of the team that waits for a
// permission — no question at all, the pane with herdr's own psql line in it,
// and the worker that is working.
func e2eActivityMigrate(t *testing.T, ctx context.Context, base string, at time.Time) {
	t.Helper()
	activityOpen(t, ctx, base, "?open=migrate")
	ask := `document.querySelector('#list .act[data-team="migrate"] .ask')`

	// C3: the section has no question, and nothing says it could not read one.
	if got := e2eRead[string](t, ctx, `String(!!`+ask+`.querySelector('.q'))`); got != "false" {
		t.Errorf("migrate's opened part carries a question section")
	}
	if got := e2eRead[string](t, ctx, ask+`.querySelector('.who2 code').textContent+'|'+`+ask+`.querySelector('.who2 .rl').textContent`); got != "migrate-tester|ロール reviewer" {
		t.Errorf("migrate's opened part stands under %q", got)
	}
	// 画面の最後: herdr's own lines, the psql question and all. The server sends
	// the last six of herdr's eight, the first of those six being empty, and the
	// screen keeps that empty line (the newline it writes under its own <pre> is
	// the one the parser drops), so all six stand, and the two held back are the
	// two before them (R3).
	head, lines := activityTail(t, ctx, "migrate")
	if head != "画面の最後▸ 上に 2 行" {
		t.Errorf("migrate's pane stands under %q, want 画面の最後 and 上に 2 行", head)
	}
	if len(lines) != 6 {
		t.Errorf("migrate's pane shows %d lines, want the six it shows first", len(lines))
	}
	if len(lines) > 0 && lines[0] != "" {
		t.Errorf("migrate's pane starts %q, want the empty line the server sent first", lines[0])
	}
	pane := strings.Join(lines, "\n")
	for _, want := range []string{"Would you like to run the following command?", `$ psql "$STAGING_URL" -f migrate/0042_split_ledger.sql`} {
		if !strings.Contains(pane, want) {
			t.Errorf("migrate's pane never says %q", want)
		}
	}
	if strings.Contains(pane, "分ける SQL") {
		t.Errorf("migrate's pane shows a line the 上に 2 行 holds back")
	}
	// C4: impl is the one working, the tester is who the line is about, and the
	// workers stand under ワーカー.
	want := []string{
		"blocked last|tester|許可待ち|" + activityMoment(at.Add(-18*time.Minute)),
		"active|impl|仕事中|",
		"idle|lead|止まっている|" + activityMoment(at.Add(-50*time.Minute)),
		"sub|||",
		"active|draft|仕事中|",
		"idle|match|止まっている|" + activityMoment(at.Add(-22*time.Minute)),
	}
	if got := activityCrew(t, ctx, "migrate"); !slices.Equal(got, want) {
		t.Errorf("migrate's members read\n%v\nwant\n%v", got, want)
	}
}

// e2eActivityPresses is P1〜P2: pressing a row's head opens and closes the part
// under it and moves the address, and pressing 上に N 行 lets the pane's held
// lines out and back — and neither press moves what stands anywhere else.
func e2eActivityPresses(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	activityOpen(t, ctx, base, "")
	row := func(team, inside string) string {
		return `document.querySelector('#list .act[data-team=' + JSON.stringify(` + strconv.Quote(team) + `) + ']` + inside + `')`
	}

	// P1（R1）: opening a working team reads it here and then, not at the next
	// re-reading (2.3). research runs and stands shut; its opened part is the
	// right column alone (2.5). The screen is fresh — the press is a second or
	// two after it was drawn — so a part that waited for the ten-second
	// re-reading would still be empty inside the five seconds allowed here.
	e2ePressWhenEnabled(t, ctx, row("research", " .ahead"))
	activityAwait(t, ctx, `!!`+row("research", " .abody .crew .nodes li"), 5*time.Second)
	if got := e2eRead[string](t, ctx, `String(!!`+row("research", " .abody .ask")+`)`); got != "false" {
		t.Errorf("a working team's opened part carries the left column")
	}
	if names := activityCrew(t, ctx, "research"); len(names) < 2 {
		t.Errorf("research's opened part reads %v, want its members", names)
	}
	// The row is put back shut, so the presses below start where the screen did.
	e2ePressWhenEnabled(t, ctx, row("research", " .ahead"))
	e2eWait(t, ctx, `!`+row("research", " .abody"))

	// P1: pressing core-hotfix's head closes it, its part alone going away, and
	// the address writes the names that are open.
	others := e2eRead[string](t, ctx, activityOthersScript("core-hotfix"))
	e2ePressWhenEnabled(t, ctx, row("core-hotfix", " .ahead"))
	e2eWait(t, ctx, `!`+row("core-hotfix", " .abody"))
	if got := e2eRead[string](t, ctx, activityStands); got != "core=open,migrate=open,core-hotfix=shut,research=shut" {
		t.Errorf("after shutting core-hotfix the rows stand %q", got)
	}
	if got := e2eRead[string](t, ctx, `location.search`); got != "?open=core%2Cmigrate" {
		t.Errorf("the address reads %q, want the names that are open", got)
	}
	if got := e2eRead[string](t, ctx, activityOthersScript("core-hotfix")); got != others {
		t.Errorf("shutting one row moved what the other rows say")
	}
	// P1: pressing it again opens it again, and the address follows.
	e2ePressWhenEnabled(t, ctx, row("core-hotfix", " .ahead"))
	e2eWait(t, ctx, `!!`+row("core-hotfix", " .abody"))
	if got := e2eRead[string](t, ctx, `location.search`); got != "?open=core%2Cmigrate%2Ccore-hotfix" {
		t.Errorf("the address reads %q after opening it again", got)
	}

	// P2: 上に 2 行 lets the last two lines of migrate's pane out, and 上の行を
	// しまう puts them back; the other rows do not move. The pane reads all six
	// lines the server sent, the first of them empty (R3).
	tail := row("migrate", " .tail")
	lines := func() string {
		return e2eRead[string](t, ctx, `String(`+tail+`.querySelector('.term').textContent.split('\n').length)`)
	}
	if got := lines(); got != "6" {
		t.Fatalf("migrate's pane shows %s lines before the press, want the six the server sent", got)
	}
	others = e2eRead[string](t, ctx, activityOthersScript("migrate"))
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

// e2eActivityMore is P3: ほか N 件 lets the questions the section held back out.
// The fixture's own numbers never reach it (5.3: three questions for core's
// lead, the most one member has), so a gy that answers with three more of the
// same member's questions stands in for this one press.
func e2eActivityMore(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	work := filepath.Join(f.cfg, "activity")
	more := filepath.Join(work, "bin", "gy-more")
	// Three more questions of core-lead, older than the three the fixture holds,
	// so they stand after them (2.4: newest first).
	var open []map[string]any
	readJSONFile(t, filepath.Join(work, "gy", "open.json"), &open)
	var writes []map[string]any
	readJSONFile(t, filepath.Join(work, "gy", "writes.json"), &writes)
	for i, old := range []int{4000, 4200, 4400} {
		id := "q-500" + strconv.Itoa(i+1)
		when := e2eActivityNow.Add(-time.Duration(old) * time.Minute).Format(time.RFC3339)
		open = append(open, map[string]any{"id": id, "kind": "Question", "status": "open", "title": "作り物の問い " + strconv.Itoa(i+1), "scope": "common", "created": when})
		writes = append(writes, map[string]any{"actor": "core-lead", "node": id, "what": "created", "why": "", "at": when})
		writeFile(t, filepath.Join(work, "gy", "show", id+".json"),
			`[{"id":"`+id+`","kind":"Question","title":"作り物の問い `+strconv.Itoa(i+1)+`","data":{"Question":{"closure":"Decision","decider":"master","options":["案 A","案 B"]}}}]`)
	}
	writeActivityJSON(t, filepath.Join(work, "gy", "open-more.json"), open)
	writeActivityJSON(t, filepath.Join(work, "gy", "writes-more.json"), writes)
	writeFile(t, more, "#!/bin/sh\n"+
		`d="$(cd "$(dirname "$0")/.." && pwd)"`+"\n"+
		`case "$*" in`+"\n"+
		`  "list --type question --status open --json") cat "$d/gy/open-more.json" ;;`+"\n"+
		`  "list --since 0 --type question --json") cat "$d/gy/writes-more.json" ;;`+"\n"+
		`  *) exec "$d/bin/gy" "$@" ;;`+"\n"+
		"esac\n")
	if err := os.Chmod(more, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", more)

	activityOpen(t, ctx, base, "?open=core")
	rest := `document.querySelector('#list .act[data-team="core"] .more .rest')`
	if got := e2eRead[string](t, ctx, rest+`.textContent`); got != "ほか 2 件" {
		t.Fatalf("the section's rest reads %q, want ほか 2 件", got)
	}
	rows := `document.querySelectorAll('#list .act[data-team="core"] .more .mrow').length`
	if got := e2eRead[string](t, ctx, `String(`+rows+`)`); got != "3" {
		t.Fatalf("the section shows %s lines, want the three it shows first", got)
	}
	e2ePressWhenEnabled(t, ctx, rest)
	e2eWait(t, ctx, `(`+rows+` === 5)`)
	if got := e2eRead[string](t, ctx, `String(!!document.querySelector('#list .act[data-team="core"] .more .rest'))`); got != "false" {
		t.Errorf("the rest is still there after every question was let out")
	}
	if got := e2eRead[string](t, ctx, `Array.from(document.querySelectorAll('#list .act[data-team="core"] .more .mrow code')).map(function(e){return e.textContent}).join(',')`); got != "q-51b9,q-0d4a,q-5001,q-5002,q-5003" {
		t.Errorf("the lines read %q, want the three older ones and the three made up ones", got)
	}
}

// e2eActivityNoHerdr is E1: a herdr that cannot be read leaves one line in the
// list and no number in the band's 活動.
func e2eActivityNoHerdr(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	// E1（R2）: the screen stands first while herdr reads, with the band's two
	// numbers. herdr is then pointed at a path that is not there, behind the
	// standing page, so the screen's own re-reading is what takes the numbers off
	// the band (2.1・2.7) — not a page made again.
	activityOpen(t, ctx, base, "")
	if got := e2eRead[string](t, ctx, activityBandScript); got != "4|待ち 2" {
		t.Fatalf("the band stands %q while herdr reads, want 活動 4 and 待ち 2", got)
	}
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-herdr"))
	// The reading of herdr stands for five seconds (3 章) and the screen asks
	// again every ten, so the line and the numbers land a re-reading later.
	activityAwait(t, ctx, `document.querySelectorAll('#list .gone').length === 1`, 40*time.Second)
	if got := e2eRead[string](t, ctx, activityBandScript); got != "none|none" {
		t.Errorf("with a herdr that cannot be read the band stands %q, want 活動 alone", got)
	}
	if got := e2eRead[string](t, ctx, `document.querySelector('#list .gone').textContent`); got == "" {
		t.Errorf("the line a herdr that could not be read leaves is empty")
	}
	// The one line is not 動いているチームはありません: a herdr that answered with
	// no team says that, and one that cannot be read says why.
	if got := e2eRead[string](t, ctx, `document.querySelector('#list .gone').textContent`); got == i18n.T("status.none") {
		t.Errorf("an unreadable herdr says there is no team running")
	}
	// herdr is put back, and the screen is waited out until the rows stand
	// again: the five seconds are the reading's, not this walk's, and the walks
	// after this one start where the fixture stands.
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "activity", "bin", "herdr"))
	e2eRun(t, ctx, chromedp.Navigate(base+"/activity"))
	activityAwait(t, ctx, `document.querySelectorAll('#list .act').length === 4`, 30*time.Second)
}

// e2eActivityNoGy is E2: a gy that is not there leaves the question section out
// and says nothing about it — the pane and the members are there as before.
func e2eActivityNoGy(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	t.Setenv("HERGY_GY", filepath.Join(f.cfg, "no-gy"))
	activityOpen(t, ctx, base, "?open=core")
	// The five-second reading of herdr is waited out, so the row with its own
	// pane stands whatever the walk before this one left behind.
	activityAwait(t, ctx, `!!document.querySelector('#list .act[data-team="core"] .ask')`, 30*time.Second)
	ask := `document.querySelector('#list .act[data-team="core"] .ask')`
	if got := e2eRead[string](t, ctx, `String(!!`+ask+`.querySelector('.q'))`); got != "false" {
		t.Errorf("a gy that is not there still leaves a question section")
	}
	if got := e2eRead[string](t, ctx, `String(`+ask+`.textContent.indexOf('警告')>=0)`); got != "false" {
		t.Errorf("a gy that is not there says something about it")
	}
	if got := e2eRead[string](t, ctx, ask+`.querySelector('.who2 code').textContent`); got != "core-lead" {
		t.Errorf("the section stands under %q, want the herdr name still", got)
	}
	if got := e2eRead[string](t, ctx, `String(!!`+ask+`.querySelector('.tail .term'))`); got != "true" {
		t.Errorf("the pane is gone although only gy is not there")
	}
}

// e2eActivityGyFails is E3: a gy that is there and fails leaves 警告 and one
// line where the questions stand, and the pane is still there.
func e2eActivityGyFails(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	bad := filepath.Join(f.cfg, "activity", "bin", "gy-bad")
	writeFile(t, bad, "#!/bin/sh\necho 'gy: the ledger could not be read' >&2\nexit 1\n")
	if err := os.Chmod(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", bad)
	activityOpen(t, ctx, base, "?open=core")
	activityAwait(t, ctx, `!!document.querySelector('#list .act[data-team="core"] .ask')`, 30*time.Second)
	ask := `document.querySelector('#list .act[data-team="core"] .ask')`
	if got := e2eRead[string](t, ctx, `(function(){var w=`+ask+`.querySelector('.q .qh');`+
		`return Array.from(w.querySelectorAll('span')).map(function(e){return e.textContent}).join('|')})()`); !strings.HasPrefix(got, "警告|") || strings.Trim(strings.TrimPrefix(got, "警告|"), "|") == "" {
		t.Errorf("the place of the questions reads %q, want 警告 and the sentence", got)
	}
	if got := e2eRead[string](t, ctx, `String(!!`+ask+`.querySelector('.tail .term'))`); got != "true" {
		t.Errorf("the pane is gone although gy alone failed")
	}
}

// e2eActivityAPI is V1〜V3 and E5: the screen's script writes no length, no
// moment and no 何分前 of its own, GET /api/activity reads no gy at all while
// GET /api/activity/{team} reads each question's own details once, and GET /
// carries a person to the screen that has something to show. It stands on a
// serve of its own, with a gy that writes down every call, so the screen's own
// re-reading cannot stand between a call and its reading.
func e2eActivityAPI(t *testing.T, cwd string) {
	t.Helper()
	f := newFixture(t)
	copyActivityFixture(t, f, cwd)
	work := filepath.Join(f.cfg, "activity")
	log := filepath.Join(work, "gy-calls.log")
	wrap := filepath.Join(work, "bin", "gy-log")
	writeFile(t, wrap, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> "+strconv.Quote(log)+"\nexec "+strconv.Quote(filepath.Join(work, "bin", "gy"))+" \"$@\"\n")
	if err := os.Chmod(wrap, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", wrap)
	base, stop := startServe(t, f)
	defer stop()

	// V1: the script the screen runs makes no length, no moment and no 何分前.
	_, script := navGet(t, base, "/static/activity-v2.js")
	code := activityCode(script)
	for _, word := range []string{"時間", "分", "から", "秒", "以上", "より前", "日", "時"} {
		if strings.Contains(code, word) {
			t.Errorf("the screen's own script says %q, which the server is the one to say", word)
		}
	}

	// V2: the list reads herdr and the launch record, and never gy.
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	resp, page := navGet(t, base, "/api/activity")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity answers %d", resp.StatusCode)
	}
	var list activityListAnswer
	readJSONText(t, "GET /api/activity", page, &list)
	if list.Running != 4 || list.Waiting != 2 || !list.Herdr.OK {
		t.Errorf("the list answers running=%d waiting=%d herdr.ok=%v", list.Running, list.Waiting, list.Herdr.OK)
	}
	if got := activityItemNames(list.Items); got != "core,migrate,core-hotfix,research" {
		t.Errorf("the list's items are %q, want activity.Sort's order", got)
	}
	if got := calls(t, log); got != "" {
		t.Errorf("GET /api/activity called gy: %s", strings.TrimSpace(got))
	}

	// V3: the opened part reads each question's own details once, and only the
	// three core's lead asked.
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	resp, page = navGet(t, base, "/api/activity/core")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/activity/core answers %d", resp.StatusCode)
	}
	var doc activityDocAnswer
	readJSONText(t, "GET /api/activity/core", page, &doc)
	if doc.Doc.Name != "core" || doc.Doc.Herdr != "core-lead" || doc.Doc.Role != "lead" || !doc.Doc.Gy.OK {
		t.Errorf("the opened part answers %q/%q/%q gy.ok=%v", doc.Doc.Name, doc.Doc.Herdr, doc.Doc.Role, doc.Doc.Gy.OK)
	}
	if got := activityQuestionIDs(doc.Doc.Questions); got != "q-7c2e,q-51b9,q-0d4a" {
		t.Errorf("the opened part carries %q, want the row's own member's three", got)
	}
	if doc.Doc.Tail.Shown != 6 || len(doc.Doc.Tail.Lines) != 6 {
		t.Errorf("the pane carries %d lines with showing %d", len(doc.Doc.Tail.Lines), doc.Doc.Tail.Shown)
	}
	shown := []string{}
	for _, line := range strings.Split(strings.TrimSpace(calls(t, log)), "\n") {
		if strings.HasPrefix(line, "show ") {
			shown = append(shown, strings.Fields(line)[1])
		}
	}
	if !slices.Equal(shown, []string{"q-7c2e", "q-51b9", "q-0d4a"}) {
		t.Errorf("the questions' details were read for %v, want each of the three once", shown)
	}
	// A team that is not running is not there (3 章).
	if resp, _ := navGet(t, base, "/api/activity/nope"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /api/activity/nope answers %d, want 404", resp.StatusCode)
	}
	// The kind is one someone reads: the three writes are refused with 405 and
	// the word the boundary writes for that (3 章).
	for _, one := range []struct{ method, path string }{
		{"POST", "/api/activity/check"},
		{"PUT", "/api/activity/core"},
		{"DELETE", "/api/activity/core"},
	} {
		resp, body := activityWrite(t, base, one.method, one.path)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s answers %d, want 405", one.method, one.path, resp.StatusCode)
		}
		if got := strings.TrimSpace(body); got != i18n.T("web.error.read_only") {
			t.Errorf("%s %s says %q, want the read-only sentence", one.method, one.path, got)
		}
	}

	// E5: with teams running, GET / carries a person to the activity screen.
	resp, _ = navGet(t, base, "/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/activity" {
		t.Errorf("GET / answers %d -> %q, want a move to /activity", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// e2eActivityAgain is P4: the screen reads again every ten seconds (2.6), and
// the re-reading moves only what the answer changed. core-lead is made to work,
// so core's row turns 仕事中 and the order moves, while the cursor, the scroll,
// the open rows and the let-out pane lines stay where the person left them. The
// band's two numbers are the re-reading's own too (2.1): 待ち follows the members
// waiting, down to nothing.
func e2eActivityAgain(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 900))
	activityOpen(t, ctx, base, "?open=core,migrate,core-hotfix,research")
	// The person lets migrate's held lines out and stands the cursor on them.
	tail := `document.querySelector('#list .act[data-team="migrate"] .tail')`
	e2ePressWhenEnabled(t, ctx, tail+`.querySelector('.th .rest')`)
	e2eWait(t, ctx, `(`+tail+`.querySelector('.term').textContent.split('\n').length === 8)`)
	e2eScript(t, ctx, `document.getElementById('tail-migrate').focus();window.scrollTo(0,120)`)
	before := e2eRead[string](t, ctx, `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`)
	if !strings.HasPrefix(before, "tail-migrate|120|?open=core,migrate,core-hotfix,research") {
		t.Fatalf("the person stands at %q before the re-reading", before)
	}

	activityWork(t, f, "core-lead")
	activityAwait(t, ctx, `(function(){var a=document.querySelector('#list .act[data-team="core"]');`+
		`return !!a && a.querySelector('.c-st .st').textContent === '仕事中'})()`, 40*time.Second)

	if got := e2eRead[string](t, ctx, activityNames); got != "migrate,core-hotfix,core,research" {
		t.Errorf("after core started working the rows are %q, want the working teams last", got)
	}
	parts := strings.Split(activityRowText(t, ctx, "core"), "|")
	if parts[0] != "仕事中" || parts[1] != "lead" || parts[2] != "" {
		t.Errorf("core's row reads %q, want 仕事中 and the member working with no いつから", parts)
	}
	if got := e2eRead[string](t, ctx, `(document.activeElement.id||'')+'|'+String(window.scrollY)+'|'+location.search`); got != before {
		t.Errorf("the re-reading moved the person: %q, was %q", got, before)
	}
	if got := e2eRead[string](t, ctx, `(function(){var a=document.querySelector('#list .act[data-team="migrate"] .tail');`+
		`return String(a.querySelector('.term').textContent.split('\n').length)+'|'+a.querySelector('.th').textContent})()`); got != "8|画面の最後▾ 上の行をしまう" {
		t.Errorf("the let-out pane reads %q after the re-reading", got)
	}
	// A working team's opened part is the right column alone (2.5).
	if got := e2eRead[string](t, ctx, `String(!!document.querySelector('#list .act[data-team="core"] .ask'))`); got != "false" {
		t.Errorf("a team that started working still shows the left column")
	}

	// P4（R2）: the band's two numbers are the re-reading's own (2.1). core-lead
	// works now, so the members waiting for a human or a permission fall from
	// two to one, 活動 staying at four.
	if got := e2eRead[string](t, ctx, activityBandScript); got != "4|待ち 1" {
		t.Errorf("after the re-reading the band stands %q, want 活動 4 and 待ち 1", got)
	}
	// P4（R2）: with the last member waiting gone, 待ち falls off the band (2.1).
	// The walk then puts the fixture back as the walks after it expect it.
	activityAgentState(t, f, "migrate-tester", "working")
	activityAwait(t, ctx, activityBandScript+` === '4|none'`, 40*time.Second)
	activityAgentState(t, f, "migrate-tester", "blocked")
	e2eRun(t, ctx, chromedp.Navigate(base+"/activity"))
	activityAwait(t, ctx, `(function(){var l=document.querySelector('#list .act[data-team="migrate"] .c-since .len');return !!l && l.textContent.length > 0})()`, 30*time.Second)
}

// e2eActivityWidths is W1〜W2 and D1〜D2: the rows lie as 4 章 says at 2000, 1440
// and 500, no width or light flows sideways, the pane's long psql line wraps at
// 500, the ink reads against its ground in the four lights, and the sheet names
// no colour of its own.
func e2eActivityWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	row := `document.querySelector('#list .act[data-team="migrate"]`

	// W1: at 2000 and 1440 one row stands on one line, with いつから at the row's
	// right edge. 誰's box is aligned to the name's to within a pixel, since the
	// two words are set in letters of different heights.
	for _, size := range []int64{2000, 1440} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1100))
		activityOpen(t, ctx, base, "?open=migrate")
		if shift := e2eRead[float64](t, ctx, `(function(){var a=`+row+` .ahead'),n=a.querySelector('.c-name').getBoundingClientRect(),w=a.querySelector('.c-who').getBoundingClientRect();`+
			`return Math.abs(w.top-n.top)})()`); shift > 2 {
			t.Errorf("at %d 誰 stands %.1f px off the name's line, want them on one line", size, shift)
		}
		if gap := e2eRead[float64](t, ctx, `(function(){var a=`+row+` .ahead'),s=a.querySelector('.c-since').getBoundingClientRect();`+
			`return a.getBoundingClientRect().right - s.right})()`); gap < 1 || gap > 20 {
			t.Errorf("at %d いつから stands %.1f px from the row's right edge, want it at the edge", size, gap)
		}
		// The opened part's two columns stand side by side, the left one first.
		if got := e2eRead[string](t, ctx, `(function(){var b=`+row+` .abody'),a=b.querySelector('.ask').getBoundingClientRect(),c=b.querySelector('.crew').getBoundingClientRect();`+
			`return String(c.left > a.right - 1)})()`); got != "true" {
			t.Errorf("at %d the members do not stand beside the questions", size)
		}
	}

	// W1: at 500 the row stands on two lines and hides the moment, keeping the
	// length, and the members stand under the questions.
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 1000))
	activityOpen(t, ctx, base, "?open=migrate")
	if shift := e2eRead[float64](t, ctx, `(function(){var a=`+row+` .ahead'),n=a.querySelector('.c-name').getBoundingClientRect(),w=a.querySelector('.c-who').getBoundingClientRect();`+
		`return w.top - n.top})()`); shift <= 0 {
		t.Errorf("at 500 誰 stands %.1f px below the name, want the row on two lines", shift)
	}
	got := e2eRead[string](t, ctx, `(function(){var a=`+row+` .ahead');return [getComputedStyle(a.querySelector('.c-since .at')).display,a.querySelector('.c-since .len').textContent.length>0].join('|')})()`)
	if got != "none|true" {
		t.Errorf("at 500 the row reads %q, want the moment hidden and the length kept", got)
	}
	if got := e2eRead[string](t, ctx, `(function(){var b=`+row+` .abody'),a=b.querySelector('.ask').getBoundingClientRect(),c=b.querySelector('.crew').getBoundingClientRect();`+
		`return String(c.top > a.bottom - 1)})()`); got != "true" {
		t.Errorf("at 500 the members do not stand under the questions")
	}

	// W2: no width and no light flows sideways, and at 500 the pane's long psql
	// line wraps rather than running off the page.
	for _, size := range []int64{2000, 1440, 500} {
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
			activityOpen(t, ctx, base, "?open=core,migrate")
			e2eTheme(t, ctx, theme)
			if over := e2eRead[string](t, ctx, `String(document.documentElement.scrollWidth - window.innerWidth)`); over != "0" && over != "1" {
				t.Errorf("at %d in %q the page flows %s px sideways", size, theme, over)
			}
			if over := e2eRead[string](t, ctx, `String(document.body.scrollWidth - window.innerWidth)`); over != "0" && over != "1" {
				t.Errorf("at %d in %q the body flows %s px sideways", size, theme, over)
			}
		}
	}
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 1000))
	activityOpen(t, ctx, base, "?open=migrate")
	if got := e2eRead[string](t, ctx, `(function(){var p=document.querySelector('#list .act[data-team="migrate"] .term');return String(p.scrollWidth-p.clientWidth)})()`); got != "0" && got != "1" {
		t.Errorf("at 500 the pane's long line runs %s px past its box, want it wrapped", got)
	}
	if got := e2eRead[string](t, ctx, `getComputedStyle(document.querySelector('#list .act[data-team="migrate"] .term')).whiteSpace`); got != "pre-wrap" {
		t.Errorf("the pane reads %q, want its lines wrapped", got)
	}

	// D1: the four ways the screen can be lit all read, and the OS's request for
	// dark is honoured when the page is told nothing.
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	activityOpen(t, ctx, base, "?open=core")
	lights := []struct {
		name, ground string
		dark         bool
		theme        string
	}{
		{"light", "rgb(236, 239, 243)", false, ""},
		{"dark", "rgb(15, 18, 24)", true, ""},
		{`data-theme="light"`, "rgb(236, 239, 243)", true, "light"},
		{`data-theme="dark"`, "rgb(15, 18, 24)", false, "dark"},
	}
	for _, light := range lights {
		if light.dark {
			e2eDark(t, ctx)
		} else {
			e2eActivityLight(t, ctx)
		}
		e2eTheme(t, ctx, light.theme)
		if got := e2eRead[string](t, ctx, `getComputedStyle(document.body).backgroundColor`); got != light.ground {
			t.Errorf("in %s the ground is %q, want %q", light.name, got, light.ground)
		}
		if got := e2eRead[float64](t, ctx, e2eRolesContrast); got < 4.5 {
			t.Errorf("in %s the body's ink against its ground is %.2f, want at least 4.5", light.name, got)
		}
	}
	e2eTheme(t, ctx, "")

	// D2: the sheet names no colour of its own, apart from where it defines the
	// tokens (the mock's :root and its two dark blocks).
	_, css := navGet(t, base, "/static/activity-v2.css")
	if hard := activityHardColours(css); len(hard) != 0 {
		t.Errorf("the sheet names colours of its own: %v", hard)
	}
}

// e2eActivityNoTeams is E4: with no team running the list says so in one line,
// the band's 活動 carries no number, and GET / carries a person to the screen
// that has something to show. It goes last: it empties the fixture's herdr.
func e2eActivityNoTeams(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	writeFile(t, filepath.Join(f.cfg, "activity", "herdr", "tabs.json"), `{"result":{"tabs":[]}}`+"\n")
	// As with an unreadable herdr: the sentence the fixture just made stands
	// only after the five seconds of the reading before it.
	activitySettles(t, ctx, base,
		`!!document.querySelector('#list .gone') && !document.querySelector('.screens .scr[data-screen="activity"] .n')`, 30*time.Second)
	if got := e2eRead[string](t, ctx, `document.querySelector('#list .gone').textContent`); got != i18n.T("status.none") {
		t.Errorf("the list says %q with no team running, want %q", got, i18n.T("status.none"))
	}
	// GET / is read by the same server, so it settles with the screen: with no
	// team running it carries a person to the team definitions.
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, _ := navGet(t, base, "/")
		if resp.StatusCode == http.StatusFound && resp.Header.Get("Location") == "/teams" {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("GET / answers %d -> %q with no team running, want a move to /teams", resp.StatusCode, resp.Header.Get("Location"))
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ---- the fixture's own files ------------------------------------------------------------

// activityAsked is one open question of the fixture: its id, its title, and who
// asked it.
type activityAsked struct{ ID, Title, Actor string }

// activityQuestions is the fixture's open questions with who asked each, so a
// walk can hold the screen to showing none but the row's own member's (C2).
func activityQuestions(t *testing.T, f *fixture) []activityAsked {
	t.Helper()
	var open []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	readJSONFile(t, filepath.Join(f.cfg, "activity", "gy", "open.json"), &open)
	var writes []struct {
		Actor string `json:"actor"`
		Node  string `json:"node"`
	}
	readJSONFile(t, filepath.Join(f.cfg, "activity", "gy", "writes.json"), &writes)
	by := map[string]string{}
	for _, write := range writes {
		by[write.Node] = write.Actor
	}
	out := make([]activityAsked, 0, len(open))
	for _, question := range open {
		out = append(out, activityAsked{ID: question.ID, Title: question.Title, Actor: by[question.ID]})
	}
	return out
}

// activityWork writes herdr's agent list again with one agent working: how the
// fixture shows a member starting to work between two readings (P4).
func activityWork(t *testing.T, f *fixture, name string) {
	t.Helper()
	activityAgentState(t, f, name, "working")
}

// activityAgentState writes herdr's agent list again with one agent in one
// state, its change's seq moved up so the reading takes it as the newest. A walk
// that changes the fixture between two readings puts it back the same way.
func activityAgentState(t *testing.T, f *fixture, name, state string) {
	t.Helper()
	path := filepath.Join(f.cfg, "activity", "herdr", "agents.json")
	var doc struct {
		Result struct {
			Agents []map[string]any `json:"agents"`
		} `json:"result"`
	}
	readJSONFile(t, path, &doc)
	found := false
	for _, agent := range doc.Result.Agents {
		if agent["name"] != name {
			continue
		}
		seq, _ := agent["state_change_seq"].(float64)
		agent["agent_status"] = state
		agent["state_change_seq"] = int(seq) + 1
		found = true
	}
	if !found {
		t.Fatalf("the fixture has no agent %s", name)
	}
	writeActivityJSON(t, path, doc)
}

// ---- small readers --------------------------------------------------------------------

// readJSONFile reads one JSON file of the fixture into one value.
func readJSONFile(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// writeActivityJSON writes one value as the fixture's files are written.
func writeActivityJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(raw))
}

// e2eActivityLight asks the page to read prefers-color-scheme as light, so each
// of the four ways the screen can be lit is read on its own (D1).
func e2eActivityLight(t *testing.T, ctx context.Context) {
	t.Helper()
	e2eCall(t, ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "light"}},
	})
}

// activityHardColours is the colours one sheet names of its own (D2): the sheet
// may name them where it defines the tokens, so every declaration that sets one
// is taken out first, and what is left may read var(...) and nothing else.
func activityHardColours(css string) []string {
	text := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	text = regexp.MustCompile(`--[A-Za-z0-9-]+\s*:[^;]*;`).ReplaceAllString(text, "")
	var out []string
	for _, find := range []*regexp.Regexp{
		regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`),
		regexp.MustCompile(`\brgba?\(`),
		regexp.MustCompile(`\bhsla?\(`),
	} {
		out = append(out, find.FindAllString(text, -1)...)
	}
	return out
}
