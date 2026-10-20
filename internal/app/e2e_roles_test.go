//go:build e2e

package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestE2ERoles walks the rebuilt role screen over the real-scale fixture
// (r-778c): the three tabs, the row that opens in place, the rule that opens
// inside it, the checks the screen draws (a document-level warning and the
// per-rule badges), the filter and the search, one rule saved, one pick in the
// assign tab, the YAML tab, the old URLs, and the three widths in both lights.
// The fixture is the one the design was drawn on (testdata/roles-v2), so the
// numbers are the ones the mock shows: 6 rows, 51 rules (23 with no origin, 36
// with no why, 3 carrying a problem), 4 team definitions and 15 people.
func TestE2ERoles(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	initLocales(t)
	copyRolesFixture(t, f, cwd)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")

	base, stop := startServe(t, f)
	defer stop()

	// The run's one browser, and this test's tab in it (r-a107).
	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 3*time.Minute)
	defer stopTab()

	var mu sync.Mutex
	var console []string
	chromedp.ListenTarget(ctx, func(event any) {
		if called, ok := event.(*runtime.EventConsoleAPICalled); ok && called.Type == "error" {
			var parts []string
			for _, arg := range called.Args {
				parts = append(parts, fmt.Sprintf("%v %v %v", arg.Value, arg.Description, arg.Preview))
			}
			mu.Lock()
			console = append(console, strings.Join(parts, " | "))
			mu.Unlock()
		}
	})

	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	t.Run("structure", func(t *testing.T) { e2eRolesStructure(t, ctx, base) })
	t.Run("open and checks", func(t *testing.T) { e2eRolesOpen(t, ctx, base) })
	t.Run("filter and search", func(t *testing.T) { e2eRolesFilter(t, ctx, base) })
	t.Run("save one rule", func(t *testing.T) { e2eRolesSaveRule(t, ctx, base, f) })
	t.Run("save new and copies", func(t *testing.T) { e2eRolesSaveNew(t, ctx, base, f) })
	t.Run("assign", func(t *testing.T) { e2eRolesAssign(t, ctx, base, f) })
	t.Run("yaml", func(t *testing.T) { e2eRolesYAML(t, ctx, base, f) })
	t.Run("old urls", func(t *testing.T) { e2eRolesOldURLs(t, ctx, base) })
	t.Run("widths and lights", func(t *testing.T) { e2eRolesWidths(t, ctx, base) })

	// The rows a document does not back need a project of their own, because they
	// change the rows and the counts the tests above read: a role a layer holds
	// only as Markdown, and a role no layer holds a document for, which a member
	// names (J2, R11, N2, N3).
	f2 := newFixture(t)
	copyRolesFixture(t, f2, cwd)
	writeFile(t, filepath.Join(f2.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	writeFile(t, filepath.Join(f2.proj, ".hergy", "roles", "mdonly.md"), "# 手書きのロール\n")
	team := filepath.Join(f2.proj, ".hergy", "teams", "research", "team.yaml")
	writeFile(t, team, strings.Replace(readFile(t, team), "role: writer", "role: nosuch", 1))
	f2.runningHerdr(t, "", "")
	base2, stop2 := startServe(t, f2)
	defer stop2()
	t.Run("rows without a document", func(t *testing.T) { e2eRolesNoDoc(t, ctx, base2, f2) })

	mu.Lock()
	defer mu.Unlock()
	if len(console) > 0 {
		t.Errorf("the role screen logged %d console errors: %v", len(console), console)
	}
}

// copyRolesFixture puts the real-scale fixture under the test's project and
// outside settings directory, the way a person's project stands.
func copyRolesFixture(t *testing.T, f *fixture, cwd string) {
	t.Helper()
	src := filepath.Join(cwd, "..", "..", "testdata", "roles-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
}

// e2eRolesValue reads one expression as the text the page draws. The reading
// waits until the page can give it, the way every act does (e2eRead, r-3c41).
func e2eRolesValue(t *testing.T, ctx context.Context, script string) string {
	t.Helper()
	return e2eRead[string](t, ctx, script)
}

// e2eRolesInt reads one expression as the number the page draws.
func e2eRolesInt(t *testing.T, ctx context.Context, script string) int {
	t.Helper()
	return e2eRead[int](t, ctx, script)
}

// e2eRolesFloat reads one expression as the decimal number the page draws.
func e2eRolesFloat(t *testing.T, ctx context.Context, script string) float64 {
	t.Helper()
	return e2eRead[float64](t, ctx, script)
}

// e2eRolesWait waits until one expression is true.
func e2eRolesWait(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	e2eWait(t, ctx, script)
}

// e2eRolesRowJS is a JS expression for the list row one role name stands on.
func e2eRolesRowJS(name string) string {
	return `[...document.querySelectorAll('#rows .role')].find(function(r){var n=r.querySelector('.c-name .nm');return n&&n.textContent.trim()===` + strconv.Quote(name) + `})`
}

// e2eRolesClick presses the element one expression stands for, the way a person
// presses it. It reaches the element by the script it is given, so a row is
// found by its name rather than by a column it may not have.
func e2eRolesClick(t *testing.T, ctx context.Context, find string) {
	t.Helper()
	e2ePressWhenEnabled(t, ctx, find)
}

// e2eRolesSet writes one field's value and fires the input event a person's
// typing fires.
func e2eRolesSet(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eSet(t, ctx, find, value)
}

// e2eRolesSelect picks one option of a select and fires the change event a
// person's picking fires.
func e2eRolesSelect(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eSelect(t, ctx, find, value)
}

// e2eRolesChangedLines is the 1-based numbers of the lines that differ between
// two texts, with the line each became. It is how a save is held to the one line
// it was meant to write.
func e2eRolesChangedLines(before, after string) map[int]string {
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	out := map[int]string{}
	n := len(b)
	if len(a) > n {
		n = len(a)
	}
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(b) {
			l = b[i]
		}
		if i < len(a) {
			r = a[i]
		}
		if l != r {
			out[i+1] = r
		}
	}
	return out
}

// e2eRolesStructure is S1〜S3: the three tabs with their counts, the list's
// columns and six rows with their places and badges, the four team cards with
// their fifteen people, and the YAML tab's two groups and seven files.
func e2eRolesStructure(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))
	e2eRolesWait(t, ctx, `document.fonts.check('14px "Zen Kaku Gothic New"') && document.fonts.check('13px "IBM Plex Mono"')`)

	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#tabs .tab').length`); got != 3 {
		t.Errorf("the page has %d tabs, want 3", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('#tabs .tab')].map(function(b){return b.dataset.t+'='+b.textContent.trim()}).join(',')`); got != "roles=一覧,assign=割り当て15,yaml=YAML" {
		t.Errorf("the tabs read %q", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role').length`); got != 6 {
		t.Errorf("the list has %d rows, want 6", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.head > span')].map(function(e){return e.textContent.trim()}).filter(Boolean).join(',')`); got != "名前,置き場所,規則,由来・なぜ・確かめ,1 人が読む量,読んでいる人" {
		t.Errorf("the head reads %q", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('#rows .role .c-name .nm')].map(function(n){return n.textContent.trim()}).join(',')`); got != "analyst,builder,lead,scout,reviewer,writer" {
		t.Errorf("the rows read %q", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("analyst")+`.querySelector('.c-name .sev').textContent.trim()`); got != "警告 1" {
		t.Errorf("the analyst badge reads %q, want 警告 1", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.c-name .sev').textContent.trim()`); got != "問題 1" {
		t.Errorf("the builder badge reads %q, want 問題 1", got)
	}
	if got := e2eRolesValue(t, ctx, `(function(){var r=`+e2eRolesRowJS("scout")+`;return [r.querySelectorAll('.c-place .pill').length,r.querySelectorAll('.c-place .pill.on').length,r.querySelectorAll('.c-place .pill.shadow').length].join(',')})()`); got != "2,1,1" {
		t.Errorf("the scout row's places read %q, want the project's and the shared one", got)
	}
	if got := e2eRolesValue(t, ctx, `(function(){var r=`+e2eRolesRowJS("reviewer")+`;return r.querySelectorAll('.c-place .pill').length+','+r.querySelectorAll('.c-place .pill.on').length})()`); got != "1,1" {
		t.Errorf("the reviewer row's places read %q, want the shared one alone", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.fchip')].map(function(c){return c.dataset.f+':'+c.textContent.trim()}).join(',')`); got != ":すべて51,noorigin:由来なし23,nowhy:なぜなし36,issues:問題・警告3" {
		t.Errorf("the filter chips read %q", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.c-readers b').textContent`); got != "3" {
		t.Errorf("the builder row is read by %q people, want 3", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("lead")+`.querySelector('.c-readers b').textContent`); got != "4" {
		t.Errorf("the lead row is read by %q people, want 4", got)
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"), chromedp.WaitReady(".team"))
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.team').length`); got != 4 {
		t.Errorf("the assign tab has %d cards, want 4", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.team')].map(function(e){return e.dataset.t}).join(',')`); got != "core,docs,migrate,research" {
		t.Errorf("the cards read %q", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.arow').length`); got != 15 {
		t.Errorf("the assign tab has %d people, want 15", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.team[data-t=core] .arow').length`); got != 5 {
		t.Errorf("the core card has %d people, want 5", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.team[data-t=research] .arow').length`); got != 4 {
		t.Errorf("the research card has %d people, want 4", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.rseg').length`); got != 6 {
		t.Errorf("the role bar has %d segments, want 6", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] select').value`); got != "scout" {
		t.Errorf("scout-2 plays %q, want scout", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.team[data-t=research] [data-act=t-save]').length`); got != 0 {
		t.Errorf("a card nothing changed shows a save button")
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/yaml"), chromedp.WaitReady(".yed .ytext"))
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.fgroup').length`); got != 2 {
		t.Errorf("the YAML tab has %d groups, want 2", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.fgroup .gh b')].map(function(b){return b.textContent}).join(',')`); got != "プロジェクト,共通" {
		t.Errorf("the groups read %q", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.files .file').length`); got != 7 {
		t.Errorf("the YAML tab has %d files, want 7", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.files .file code')].map(function(c){return c.textContent}).join(',')`); got != "analyst.yaml,builder.yaml,lead.yaml,scout.yaml,scout.yaml,reviewer.yaml,writer.yaml" {
		t.Errorf("the files read %q", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.files .file .fs')].map(function(s){return s.textContent.trim()}).join(',')`); got != "警告 1,問題 1,9,4,5,6,8" {
		t.Errorf("the file amounts read %q", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.file.hid').length`); got != 1 {
		t.Errorf("the YAML tab marks %d hidden files, want the shared scout alone", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.file.hid .ft').textContent`); !strings.Contains(got, "隠れている") {
		t.Errorf("the hidden scout file reads %q", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .path').textContent`); !strings.HasSuffix(got, "analyst.yaml") {
		t.Errorf("the YAML tab opens %q, want the first file", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .ytext').value`); !strings.HasPrefix(got, "version: 1") {
		t.Errorf("the YAML editor does not show the document:\n%s", got)
	}
}

// e2eRolesOpen is P1・P3・P5: a row opens under itself with its readers and its
// panel, a rule opens inside it, the two kinds of check the screen draws show on
// the rule and in the Markdown preview, and the rule's own place is in the URL.
func e2eRolesOpen(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))
	e2eRolesClick(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.sum')`)
	e2eRolesWait(t, ctx, e2eRolesRowJS("builder")+`.classList.contains('open')`)
	if got := e2eRolesValue(t, ctx, `window.location.search`); !strings.Contains(got, "open=builder") {
		t.Errorf("opening the builder row left %q in the URL", got)
	}
	if got := e2eRolesInt(t, ctx, e2eRolesRowJS("builder")+`.querySelectorAll('.panel').length`); got != 1 {
		t.Errorf("builder shows %d panels, want the project's alone", got)
	}
	if got := e2eRolesInt(t, ctx, e2eRolesRowJS("builder")+`.querySelectorAll('.usage .uchip').length`); got != 3 {
		t.Errorf("builder is read by %d chips, want 3", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.uchip .g').textContent`); got != "core" {
		t.Errorf("the first reader's chip reads %q, want core", got)
	}
	if got := e2eRolesInt(t, ctx, e2eRolesRowJS("builder")+`.querySelectorAll('.dhvm li').length`); got != 0 {
		t.Errorf("builder shows %d document-level checks, want none", got)
	}

	// P5: the per-rule badge is the rule's own checks. builder has a problem on
	// BUILD-4 (a derived origin that the reader's documents do not hold) and a
	// warning each on BUILD-3 and BUILD-5.
	badge := func(id string) string {
		return e2eRolesValue(t, ctx, `(function(){var x=[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()===`+strconv.Quote(id)+`});var s=x.querySelector('.rid .sev');return s?s.textContent.trim():''})()`)
	}
	for id, want := range map[string]string{"BUILD-1": "", "BUILD-3": "警告 1", "BUILD-4": "問題 1", "BUILD-5": "警告 1"} {
		if got := badge(id); got != want {
			t.Errorf("the %s badge reads %q, want %q", id, got, want)
		}
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.pf .vcount').textContent`); !strings.Contains(got, "問題 1") || !strings.Contains(got, "警告 2") {
		t.Errorf("the panel's count reads %q, want 問題 1 and 警告 2", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .yprev .yl.err').length`); got != 1 {
		t.Errorf("the Markdown preview marks %d lines as a problem, want 1", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .yprev .yl.warn').length`); got != 2 {
		t.Errorf("the Markdown preview marks %d lines as a warning, want 2", got)
	}
	// The layout of a section is one way now (d-f4f6): the section header offers
	// no style to pick — the level picker stands where it did — and every rule's
	// line in the Markdown preview is one bullet item with the id at its head.
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .panel .sec [data-f=level]').length`); got != 2 {
		t.Errorf("builder shows %d level pickers, want the 2 sections'", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .panel .sec [data-f=style]').length`); got != 0 {
		t.Errorf("the section header offers %d style pickers, want none", got)
	}
	if got := e2eRolesInt(t, ctx, `[...document.querySelectorAll('#rows .role.open .yprev .yl .yt')].filter(function(e){return e.textContent.indexOf('- **BUILD-')===0}).length`); got != 7 {
		t.Errorf("the Markdown preview shows %d items with the id at their head, want the 7 rules'", got)
	}
	// R1: the page's CSP stops a style attribute written through innerHTML, so
	// the meter's fill and a kept line's indent are put on the elements from the
	// CSSOM after they are drawn. The meter paints, and a line that stands under
	// a bullet's head is pulled back to it (a negative text-indent).
	if got := e2eRolesFloat(t, ctx, `document.querySelector('#rows .role.open .c-amount .meter i').getBoundingClientRect().width`); got <= 0 {
		t.Errorf("the reader's meter is %vpx wide, want a painted fill", got)
	}
	if got := e2eRolesFloat(t, ctx, `Math.min.apply(null,[...document.querySelectorAll('#rows .role.open .yprev .yt')].map(function(e){return parseFloat(getComputedStyle(e).textIndent)}))`); got >= 0 {
		t.Errorf("the smallest text-indent in the Markdown preview is %v, want a kept line pulled back", got)
	}
	// N1: the origin cell that says — stands one line high, exactly as high as a
	// cell that names an origin. The empty list's sentence has padding of its
	// own, and it must not reach a rule's row (spec-roles 1.2.5).
	cell := func(sel string) string {
		return `(function(){var c=[...document.querySelectorAll(` + strconv.Quote("#rows .role.open .panel .rule "+sel) + `)];return c.length?Math.round(Math.max.apply(null,c.map(function(e){return e.getBoundingClientRect().height}))):-1})()`
	}
	emptyCell, namedCell := e2eRolesFloat(t, ctx, cell(".org.empty")), e2eRolesFloat(t, ctx, cell(".org:not(.empty)"))
	if emptyCell <= 0 || namedCell <= 0 {
		t.Fatalf("the builder shows no origin cells to measure: empty=%v named=%v", emptyCell, namedCell)
	}
	if emptyCell != namedCell {
		t.Errorf("the cell that says — stands %vpx high against %vpx for a named origin, want one line each", emptyCell, namedCell)
	}

	// P3: the rule opens under its own summary, and its place joins the URL.
	e2eRolesClick(t, ctx, `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'}).querySelector('.rsum')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('#rows .role.open .panel .rule.open .redit')`)
	if got := e2eRolesValue(t, ctx, `window.location.search`); !strings.Contains(got, "rule=builder.project.BUILD-5") {
		t.Errorf("opening BUILD-5 left %q in the URL", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`); got != "BUILD-5" {
		t.Errorf("the opened rule reads %q, want BUILD-5", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=text]').value`); !strings.HasPrefix(got, "大きな設計の判断") {
		t.Errorf("the opened rule's text reads %q", got)
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.pedit [data-f=send_why] button.on').textContent`); got != "プロンプトに含める" {
		t.Errorf("the why switch reads %q, want the default プロンプトに含める", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .rule.open .redit [data-act=rule-revert]:not([hidden])').length`); got != 0 {
		t.Errorf("a rule nothing changed shows 元に戻す")
	}
	if got := e2eRolesValue(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.pf [data-act=save]').disabled+''`); got != "true" {
		t.Errorf("a rule nothing changed can be saved (disabled=%s)", got)
	}

	// P1 again: the same row closes.
	e2eRolesClick(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.sum')`)
	e2eRolesWait(t, ctx, `!`+e2eRolesRowJS("builder")+`.classList.contains('open')`)
	if got := e2eRolesValue(t, ctx, `window.location.search`); strings.Contains(got, "open=") {
		t.Errorf("closing the row left %q in the URL", got)
	}

	// A document-level check stands above the sections: the analyst's whole
	// document is one rule over the provisional limit in the core team.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=analyst"), chromedp.WaitReady("#rows .role.open .panel"))
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .panel .dhvm li').length`); got != 1 {
		t.Errorf("analyst shows %d document-level checks, want 1", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .panel .dhvm li span:last-child').textContent`); !strings.Contains(got, "core-analyst に渡る規則が 21 件です") {
		t.Errorf("the analyst check reads %q", got)
	}

	// A deep link opens the row and the rule it names.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder&rule=builder.project.BUILD-5"), chromedp.WaitReady("#rows .role.open .rule.open .redit"))
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`); got != "BUILD-5" {
		t.Errorf("the deep-linked rule reads %q, want BUILD-5", got)
	}

	// P15: a Markdown line opens its own rule and puts the cursor in the field
	// the line stands for — the rule's 本文.
	e2eRolesClick(t, ctx, `[...document.querySelectorAll('#rows .role.open .yprev .yl[data-act=yline]')].find(function(l){return l.textContent.indexOf('BUILD-3')>=0})`)
	e2eRolesWait(t, ctx, `document.activeElement && document.activeElement.getAttribute('data-f')==='text'`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`); got != "BUILD-3" {
		t.Errorf("the Markdown line opened %q, want BUILD-3", got)
	}
}

// e2eRolesFilter is P2: the chips count every rule and thin the list right under
// them, and the search does the same and marks what it found.
func e2eRolesFilter(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	rows := `document.querySelectorAll('#rows .role').length`
	rules := `document.querySelectorAll('#rows .rule').length`
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))

	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=noorigin]')`)
	e2eRolesWait(t, ctx, `document.querySelector('.fchip[data-f=noorigin]').classList.contains('on') && `+rules+`===23`)
	if got := e2eRolesInt(t, ctx, rows); got != 6 {
		t.Errorf("the no-origin filter shows %d rows, want 6", got)
	}
	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=""]')`)
	e2eRolesWait(t, ctx, `document.querySelector('.fchip[data-f=""]').classList.contains('on') && `+rules+`===0`)
	if got := e2eRolesInt(t, ctx, rows); got != 6 {
		t.Errorf("すべて shows %d rows, want 6", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open').length`); got != 0 {
		t.Errorf("clearing the filter leaves %d rows open", got)
	}

	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=nowhy]')`)
	e2eRolesWait(t, ctx, rules+`===36`)
	if got := e2eRolesInt(t, ctx, rows); got != 6 {
		t.Errorf("the no-why filter shows %d rows, want 6", got)
	}
	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=""]')`)
	e2eRolesWait(t, ctx, rules+`===0`)

	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=issues]')`)
	e2eRolesWait(t, ctx, rules+`===3`)
	if got := e2eRolesInt(t, ctx, rows); got != 1 {
		t.Errorf("the issue filter shows %d rows, want the builder alone", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('#rows .rule .rid b')].map(function(b){return b.textContent.trim()}).join(',')`); got != "BUILD-3,BUILD-4,BUILD-5" {
		t.Errorf("the issue filter shows %q", got)
	}
	e2eRolesClick(t, ctx, `document.querySelector('.fchip[data-f=""]')`)
	e2eRolesWait(t, ctx, rules+`===0`)

	// P2: the search looks in the id, the text, the why and the note, and marks
	// what it found in the rule's own summary.
	e2eRolesSet(t, ctx, `document.getElementById('q')`, "テスト")
	e2eRolesWait(t, ctx, rows+`===1 && `+rules+`===1`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role .c-name .nm').textContent.trim()`); got != "builder" {
		t.Errorf("the search for テスト shows %q, want builder", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .rule .rid b').textContent.trim()`); got != "BUILD-4" {
		t.Errorf("the search for テスト shows rule %q, want BUILD-4", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .rule .rtx mark').textContent`); got != "テスト" {
		t.Errorf("the search does not mark what it found: %q", got)
	}
	e2eRolesSet(t, ctx, `document.getElementById('q')`, "")
	e2eRolesWait(t, ctx, rows+`===6 && `+rules+`===0`)
	if got := e2eRolesInt(t, ctx, rows); got != 6 {
		t.Errorf("clearing the search shows %d rows, want 6", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open').length`); got != 0 {
		t.Errorf("clearing the search leaves %d rows open", got)
	}
}

// e2eRolesSaveRule is P4・C4: one rule's text is written on the line it stands
// on, and every other line of the file comes back byte for byte.
func e2eRolesSaveRule(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "roles", "builder.yaml")
	before := readFile(t, path)
	const want = "E2E が書き換えた本文。"

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"), chromedp.WaitReady("#rows .role.open .panel .rule"))
	e2eRolesClick(t, ctx, `[...document.querySelectorAll('#rows .role.open .panel .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'}).querySelector('.rsum')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('#rows .role.open .rule.open .redit [data-f=text]')`)
	e2eRolesSet(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=text]')`, want)
	e2ePressWhenEnabled(t, ctx, e2eRolesRowJS("builder")+`.querySelector('.pf [data-act=save]')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('#rows .role.open .ok')`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .ok').textContent`); got != "保存しました" {
		t.Errorf("the saved line reads %q, want 保存しました", got)
	}

	after := readFile(t, path)
	changed := e2eRolesChangedLines(before, after)
	if len(changed) != 1 {
		t.Errorf("the save changed %d lines, want the rule's text line alone: %v", len(changed), changed)
	}
	for line, text := range changed {
		if text != "        text: "+want {
			t.Errorf("line %d became %q, want the rule's new text", line, text)
		}
	}
	if !strings.Contains(after, "refs: [REQ-5]") {
		t.Errorf("the save lost BUILD-4's derived origin:\n%s", after)
	}
	if got := e2eRolesValue(t, ctx, `(function(){var x=[...document.querySelectorAll('#rows .role.open .rule')].find(function(r){return r.querySelector('.rid b').textContent.trim()==='BUILD-5'});return x.querySelector('.rtx').textContent})()`); !strings.Contains(got, want) {
		t.Errorf("the saved rule's summary reads %q", got)
	}
}

// e2eRolesSaveNew is C4・X1: a role added on the screen, a duplicate and a copy to
// the shared place are written each with its own text — an added document from
// nothing, a duplicate and a copy from the document they came from, so the lines
// a person did not touch travel to the new file (spec-roles 1.2.8, 1.2.9).
func e2eRolesSaveNew(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	projDir := filepath.Join(f.proj, ".hergy", "roles")
	cfgDir := filepath.Join(f.cfg, "hergy", "roles")
	lead := filepath.Join(projDir, "lead.yaml")

	// 追加: the panel's one empty rule takes the id and the text a rule needs, and
	// the save writes the new document from the draft alone.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))
	e2eRolesClick(t, ctx, `document.querySelector('[data-act=new]')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('.panel[data-l=project] .rule.open [data-f=id]')`)
	// R14: the name field takes the cursor once the check's redraw is over.
	e2eRolesWait(t, ctx, `document.activeElement && document.activeElement.getAttribute('data-f')==='name'`)
	panel := `document.querySelector('.panel[data-l=project]')`
	e2eRolesSet(t, ctx, panel+`.querySelector('[data-f=name]')`, "e2e-new")
	e2eRolesSet(t, ctx, panel+`.querySelector('.rule.open [data-f=id]')`, "NEW-1")
	e2eRolesSet(t, ctx, panel+`.querySelector('.rule.open [data-f=text]')`, "新しく足したロールの規則。")
	e2ePressWhenEnabled(t, ctx, panel+`.querySelector('.pf [data-act=save]')`)
	e2eRolesWait(t, ctx, `!!`+panel+`.querySelector('.ok')`)
	added := readFile(t, filepath.Join(projDir, "e2e-new.yaml"))
	for _, want := range []string{"id: NEW-1", "新しく足したロールの規則。"} {
		if !strings.Contains(added, want) {
			t.Errorf("the added role's file does not hold %q:\n%s", want, added)
		}
	}
	if got := e2eRolesValue(t, ctx, panel+`.querySelector('.path').textContent`); !strings.Contains(got, "e2e-new.yaml") {
		t.Errorf("the saved panel's path reads %q", got)
	}

	// 複製: lead's copy comes back with the original's own lines, byte for byte.
	before := readFile(t, lead)
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=lead"), chromedp.WaitReady("#rows .role.open .pf [data-act=dup]"))
	e2eRolesClick(t, ctx, `document.querySelector('#rows .role.open .panel .pf [data-act=dup]')`)
	findDup := `[...document.querySelectorAll('.panel[data-l=project] [data-f=name]')].find(function(i){return i.value==='lead-2'})`
	e2eRolesWait(t, ctx, `!!`+findDup)
	dupUid := e2eRolesValue(t, ctx, findDup+`.closest('.panel').dataset.uid`)
	dupPanel := `.panel[data-uid="` + dupUid + `"][data-l=project]`
	at := func(sel string) string { return `document.querySelector(` + strconv.Quote(dupPanel+" "+sel) + `)` }
	e2eRolesSet(t, ctx, at("[data-f=name]"), "e2e-dup")
	e2ePressWhenEnabled(t, ctx, at(".pf [data-act=save]"))
	e2eRolesWait(t, ctx, `!!`+at(".ok"))
	if got := readFile(t, filepath.Join(projDir, "e2e-dup.yaml")); got != before {
		t.Errorf("the duplicate does not keep the original's lines:\n%s", got)
	}
	if got := readFile(t, lead); got != before {
		t.Errorf("duplicating changed the original:\n%s", got)
	}

	// 共通にコピー: the same lines are written beside the shared documents.
	e2eRolesClick(t, ctx, at(".pf [data-act=copy]"))
	glob := `document.querySelector(` + strconv.Quote(`.panel[data-uid="`+dupUid+`"][data-l=global]`) + `)`
	e2eRolesWait(t, ctx, `!!`+glob)
	e2ePressWhenEnabled(t, ctx, glob+`.querySelector('.pf [data-act=save]')`)
	e2eRolesWait(t, ctx, `!!`+glob+`.querySelector('.ok')`)
	if got := readFile(t, filepath.Join(cfgDir, "e2e-dup.yaml")); got != before {
		t.Errorf("the copy does not keep the original's lines:\n%s", got)
	}

	// R7: a name the pattern refuses stands under the name field, and the footer
	// stays clean (spec-roles 1.2.9).
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=analyst"), chromedp.WaitReady("#rows .role.open .panel[data-l=project] .pf [data-act=save]"))
	analystBefore := readFile(t, filepath.Join(projDir, "analyst.yaml"))
	bad := `.panel[data-l=project]`
	atBad := func(sel string) string {
		return `document.querySelector('#rows .role.open ' + ` + strconv.Quote(bad+" "+sel) + `)`
	}
	e2eRolesSet(t, ctx, atBad("[data-f=name]"), "Bad Name")
	e2ePressWhenEnabled(t, ctx, atBad(".pf [data-act=save]"))
	e2eRolesWait(t, ctx, `!`+atBad(".errline.name")+`.hidden`)
	if got := e2eRolesValue(t, ctx, atBad(".errline.name")+`.textContent`); !strings.Contains(got, "ロール名") {
		t.Errorf("the refusal under the name field reads %q", got)
	}
	if got := e2eRolesValue(t, ctx, atBad(".errline.doc")+`.hidden+''`); got != "true" {
		t.Errorf("a name's refusal stands in the footer too")
	}
	if got := readFile(t, filepath.Join(projDir, "analyst.yaml")); got != analystBefore {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// e2eRolesAssign is P6・V2: one person's role is picked in its card, the card
// says how many changed, and the save writes that one line of team.yaml.
func e2eRolesAssign(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "teams", "research", "team.yaml")
	before := readFile(t, path)

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"), chromedp.WaitReady(".team[data-t=research]"))
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('.team[data-t=research] [data-act=t-save]').length`); got != 0 {
		t.Errorf("a card nothing changed shows a save button")
	}
	e2eRolesSelect(t, ctx, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] select')`, "writer")
	e2eRolesWait(t, ctx, `!!document.querySelector('.team[data-t=research] [data-act=t-save]')`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('.team[data-t=research] .tside .dirty').textContent`); got != "1 人" {
		t.Errorf("the changed card says %q, want 1 人", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] s.was').textContent`); got != "scout" {
		t.Errorf("the old role reads %q, want scout", got)
	}
	e2ePressWhenEnabled(t, ctx, `document.querySelector('.team[data-t=research] [data-act=t-save]')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('.team[data-t=research] .ok')`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('.team[data-t=research] .ok').textContent`); got != "保存しました" {
		t.Errorf("the card's saved line reads %q", got)
	}

	after := readFile(t, path)
	changed := e2eRolesChangedLines(before, after)
	if len(changed) != 1 {
		t.Errorf("the assign save changed %d lines, want scout-2's role alone: %v", len(changed), changed)
	}
	for line, text := range changed {
		if text != "    role: writer" {
			t.Errorf("line %d became %q, want scout-2's new role", line, text)
		}
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.team[data-t=research] .arow[data-m=scout-2] select').value`); got != "writer" {
		t.Errorf("the card reads %q after the save, want writer", got)
	}
}

// e2eRolesYAML is T1・T3・X3: the tab lists the files, the editor marks a broken
// line and refuses the save while a new problem stands, 元に戻す puts the file's
// own text back, and a good change is written as typed.
func e2eRolesYAML(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	orig := readFile(t, path)

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/yaml"), chromedp.WaitReady(".yed .ytext"))
	e2eRolesClick(t, ctx, `[...document.querySelectorAll('.files .file')].find(function(b){return b.querySelector('code').textContent.trim()==='lead.yaml'})`)
	e2eRolesWait(t, ctx, `document.querySelector('.yed .path').textContent.indexOf('lead.yaml')>=0`)
	if got := e2eRolesValue(t, ctx, `window.location.search`); !strings.Contains(got, "file=project.lead") {
		t.Errorf("opening the lead file left %q in the URL", got)
	}

	// T1・T3: a value of the wrong shape is an error on its own line.
	broken := strings.Replace(orig, "- id: LEAD-1\n", "- id: lead-1\n", 1)
	if broken == orig {
		t.Fatalf("the fixture's lead.yaml no longer holds the line the test breaks")
	}
	e2eRolesSet(t, ctx, `document.querySelector('.yed .ytext')`, broken)
	e2eRolesWait(t, ctx, `document.querySelectorAll('.yed .vitem').length>0`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .vitem .lno').textContent`); got != "7 行" {
		t.Errorf("the broken line reads %q, want 7 行", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .vitem .msg').textContent`); !strings.Contains(got, "lead-1") {
		t.Errorf("the broken line says %q", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .gutter span:nth-child(7)').classList.contains('err')+''`); got != "true" {
		t.Errorf("the gutter does not mark line 7 as broken")
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .gutter span:nth-child(6)').classList.contains('err')+''`); got != "false" {
		t.Errorf("the gutter marks line 6 as broken")
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('[data-act=yaml-save]').disabled+''`); got != "true" {
		t.Errorf("a text with a new problem can be saved")
	}

	// 元に戻す puts the file's own text back and leaves nothing to save.
	e2eRolesClick(t, ctx, `document.querySelector('[data-act=yaml-revert]')`)
	e2eRolesWait(t, ctx, `document.querySelector('.yed .ytext').value.indexOf('lead-1')<0`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('[data-act=yaml-save]').disabled+''`); got != "true" {
		t.Errorf("a reverted text can be saved")
	}
	// 元に戻す throws away the findings of the text it threw away: the check
	// goes back to the saved text's own.
	e2eRolesWait(t, ctx, `document.querySelectorAll('.yed .vitem').length===0`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .vlist .vclean') ? document.querySelector('.yed .vlist .vclean').textContent.trim() : ''`); got != "✓ 問題なし" {
		t.Errorf("the reverted text's check reads %q, want ✓ 問題なし", got)
	}
	if got := e2eRolesValue(t, ctx, `[...document.querySelectorAll('.yed .gutter span')].some(function(s){return s.classList.contains('err')})+''`); got != "false" {
		t.Errorf("the gutter still marks a broken line after 元に戻す")
	}

	// A good change is saved as typed, and only its line moves.
	good := strings.Replace(orig, "title: 進行管理", "title: 進行管理E2E", 1)
	if good == orig {
		t.Fatalf("the fixture's lead.yaml no longer holds the title the test changes")
	}
	e2eRolesSet(t, ctx, `document.querySelector('.yed .ytext')`, good)
	e2ePressWhenEnabled(t, ctx, `document.querySelector('[data-act=yaml-save]')`)
	e2eRolesWait(t, ctx, `!!document.querySelector('.yed .ok')`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('.yed .ok').textContent`); got != "保存しました" {
		t.Errorf("the YAML save line reads %q", got)
	}
	after := readFile(t, path)
	changed := e2eRolesChangedLines(orig, after)
	if len(changed) != 1 {
		t.Errorf("the YAML save changed %d lines, want the title alone: %v", len(changed), changed)
	}
	for line, text := range changed {
		if text != "title: 進行管理E2E" {
			t.Errorf("line %d became %q, want the new title", line, text)
		}
	}
}

// e2eRolesOldURLs is 設計 6 節: the old page-per-role URLs move with a 301 to the
// row (and the rule) they name, which the browser then opens.
func e2eRolesOldURLs(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	cases := []struct{ path, want string }{
		{"/roles/lead", "/roles?open=lead"},
		{"/roles/lead/new", "/roles?open=lead"},
		{"/roles/lead/LEAD-1", "/roles?open=lead&rule=lead.project.LEAD-1"},
	}
	for _, c := range cases {
		res, err := client.Get(base + c.path)
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusMovedPermanently {
			t.Errorf("GET %s answers %d, want 301", c.path, res.StatusCode)
		}
		if got := res.Header.Get("Location"); got != c.want {
			t.Errorf("GET %s moves to %q, want %q", c.path, got, c.want)
		}
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/lead/LEAD-1"))
	e2eRolesWait(t, ctx, `document.querySelectorAll('#rows .role.open .rule.open .redit').length===1`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .c-name .nm').textContent.trim()`); got != "lead" {
		t.Errorf("the old URL opens the %q row, want lead", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .rule.open .redit [data-f=id]').value`); got != "LEAD-1" {
		t.Errorf("the old URL opens rule %q, want LEAD-1", got)
	}
}

// e2eRolesContrast is the WCAG contrast ratio of the body's ink against its
// ground, read from the page itself.
const e2eRolesContrast = `(function(){
	var lum=function(c){var m=c.match(/[0-9.]+/g).map(Number).slice(0,3).map(function(v){v=v/255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);});return 0.2126*m[0]+0.7152*m[1]+0.0722*m[2];};
	var s=getComputedStyle(document.body),a=lum(s.color),b=lum(s.backgroundColor);
	var hi=Math.max(a,b),lo=Math.min(a,b);return (hi+0.05)/(lo+0.05);
})()`

// e2eRolesWidths is W1〜W4・D1: the three widths lay the teams, the list and the
// YAML out as the spec says, no tab flows sideways at any width or light, and
// the ink reads against its ground in both lights.
func e2eRolesWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	ready := map[string]string{
		"/roles?open=builder": `!!document.querySelector('#rows .role.open .panel')`,
		"/roles/assign":       `!!document.querySelector('.teams')`,
		"/roles/yaml":         `!!document.querySelector('.yed .ytext')`,
	}
	paths := []string{"/roles?open=builder", "/roles/assign", "/roles/yaml"}

	for _, size := range []int64{2000, 1440, 500} {
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
			for _, path := range paths {
				e2eRun(t, ctx, chromedp.Navigate(base+path))
				e2eRolesWait(t, ctx, ready[path])
				if theme == "" {
					e2eRun(t, ctx, chromedp.Evaluate(`delete document.documentElement.dataset.theme`, nil))
				} else {
					e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.dataset.theme=`+strconv.Quote(theme), nil))
				}
				if over := e2eRolesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
					t.Errorf("%s at %d/%s flows %dpx sideways", path, size, theme, over)
				}
			}
		}
	}

	// W1〜W3: the concrete layout of the three widths.
	tracks := func(sel string) int {
		return e2eRolesInt(t, ctx, `getComputedStyle(document.querySelector(`+strconv.Quote(sel)+`)).gridTemplateColumns.split(' ').filter(Boolean).length`)
	}
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eRolesWait(t, ctx, ready["/roles?open=builder"])
	if got := tracks(".pmain"); got != 2 {
		t.Errorf("2000: the panel and the preview stand in %d columns, want 2", got)
	}
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.querySelector('.pview')).display`); got != "none" {
		t.Errorf("2000: the edit/preview switch is shown beside the preview")
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"))
	e2eRolesWait(t, ctx, ready["/roles/assign"])
	if got := tracks(".teams"); got != 4 {
		t.Errorf("2000: the four cards stand in %d columns, want 4", got)
	}

	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"))
	e2eRolesWait(t, ctx, ready["/roles/assign"])
	if got := tracks(".teams"); got != 2 {
		t.Errorf("1440: the cards stand in %d columns, want 2", got)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eRolesWait(t, ctx, ready["/roles?open=builder"])
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.querySelector('.head')).display`); got == "none" {
		t.Errorf("1440: the list's head row is hidden")
	}

	e2eRun(t, ctx, chromedp.EmulateViewport(500, 900))
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eRolesWait(t, ctx, ready["/roles?open=builder"])
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.querySelector('.head')).display`); got != "none" {
		t.Errorf("500: the list's head row is still shown")
	}
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.querySelector('.c-count')).display`); got != "none" {
		t.Errorf("500: the rule count column is still shown")
	}
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.querySelector('.pview')).display`); got == "none" {
		t.Errorf("500: the edit/preview switch is hidden")
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"))
	e2eRolesWait(t, ctx, ready["/roles/assign"])
	if got := tracks(".teams"); got != 1 {
		t.Errorf("500: the cards stand in %d columns, want 1", got)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/yaml"))
	e2eRolesWait(t, ctx, ready["/roles/yaml"])
	if got := tracks(".ylay"); got != 1 {
		t.Errorf("500: the files and the editor stand in %d columns, want 1", got)
	}
	if got := tracks(".ybody"); got != 1 {
		t.Errorf("500: the editor and the validate list stand in %d columns, want 1", got)
	}

	// D1: the ink reads against its ground in both lights, and the OS's request
	// for dark is honoured when the page is told nothing.
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eRolesWait(t, ctx, ready["/roles?open=builder"])
	for _, theme := range []string{"light", "dark"} {
		e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.dataset.theme=`+strconv.Quote(theme), nil))
		if got := e2eRolesFloat(t, ctx, e2eRolesContrast); got < 4.5 {
			t.Errorf("the body's ink against its ground is %.2f in %s, want at least 4.5", got, theme)
		}
	}
	e2eRun(t, ctx, chromedp.Evaluate(`delete document.documentElement.dataset.theme`, nil))
	e2eRun(t, ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}}).Do(ctx)
	}))
	if got := e2eRolesValue(t, ctx, `getComputedStyle(document.body).backgroundColor`); got != "rgb(15, 18, 24)" {
		t.Errorf("the OS dark ground is %q", got)
	}
}

// e2eRolesNoDoc is J2・R11・N2・N3・N4: the rows a document does not back. A role
// a layer holds only as Markdown is listed with the words that say so and opens
// from the address (J2, N3). A role no layer holds a document for stands as a
// new one: the one empty rule it would write stands open and its check has run
// (N2), and a save under another name puts the new name in the address (N4).
func e2eRolesNoDoc(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	projDir := filepath.Join(f.proj, ".hergy", "roles")
	panel := `document.querySelector('#rows .role.open .panel[data-l=project]')`

	// J2・N3: the address opens the row the .md leaves, and the row says what
	// hergy does about it. No panel: hergy does not write over a Markdown file.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=mdonly"), chromedp.WaitReady("#rows .role.open .dhvm li"))
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .c-name .nm').textContent.trim()`); got != "mdonly" {
		t.Errorf("open=mdonly opened the row %q", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .dhvm li span:last-child').textContent`); !strings.Contains(got, "Markdown") {
		t.Errorf("the Markdown row's words read %q", got)
	}
	if got := e2eRolesInt(t, ctx, `document.querySelectorAll('#rows .role.open .panel').length`); got != 0 {
		t.Errorf("a Markdown row shows %d editable panels, want none", got)
	}

	// R11: the row no document holds shows no place, counts no rules and no
	// amount, and carries the number of the people who read it.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))
	row := e2eRolesRowJS("nosuch")
	if got := e2eRolesValue(t, ctx, row+`.querySelector('.c-name .sev').textContent.trim()`); got != "問題 1" {
		t.Errorf("the row no document holds carries %q, want the reader's count", got)
	}
	if got := e2eRolesValue(t, ctx, row+`.querySelector('.c-place').textContent.trim()`); got != "" {
		t.Errorf("the row no document holds shows the place %q, want none", got)
	}
	if got := e2eRolesValue(t, ctx, row+`.querySelector('.c-count').textContent.trim()`); got != "—" {
		t.Errorf("the row no document holds counts %q rules, want —", got)
	}
	if got := e2eRolesInt(t, ctx, row+`.querySelectorAll('.c-name .tag').length`); got != 1 {
		t.Errorf("the row no document holds shows %d 新規 tags, want 1", got)
	}

	// N2: opening it opens the one empty rule a new role starts with, and its
	// check has run by then — the preview reads the server's own reading and not
	// the 0 行 a check that never ran would leave.
	e2eRolesClick(t, ctx, row+`.querySelector('.sum')`)
	e2eRolesWait(t, ctx, `!!`+row+`.querySelector('.panel .rule.open')`)
	e2eRolesWait(t, ctx, `!document.querySelector('#rows .role.open .yprev .yinfo .vt').textContent.startsWith('0 行')`)
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .yprev .yinfo .vt').textContent`); got == "" {
		t.Errorf("the opened row's preview reads nothing at all")
	}
	if got := e2eRolesValue(t, ctx, panel+`.querySelector('.rule.open [data-f=id]').value`); got != "" {
		t.Errorf("the one empty rule a new role starts with reads the id %q", got)
	}

	// N4: the save writes the document under the new name, and the address opens
	// the row by the name it now shows.
	e2eRolesSet(t, ctx, panel+`.querySelector('[data-f=name]')`, "nosuch-2")
	e2eRolesSet(t, ctx, panel+`.querySelector('.rule.open [data-f=id]')`, "NOSUCH-1")
	e2eRolesSet(t, ctx, panel+`.querySelector('.rule.open [data-f=text]')`, "文書の無いロールに足した規則。")
	e2ePressWhenEnabled(t, ctx, panel+`.querySelector('.pf [data-act=save]')`)
	e2eRolesWait(t, ctx, `!!`+panel+`.querySelector('.ok')`)
	written := readFile(t, filepath.Join(projDir, "nosuch-2.yaml"))
	for _, want := range []string{"NOSUCH-1", "文書の無いロールに足した規則。"} {
		if !strings.Contains(written, want) {
			t.Errorf("the saved document does not hold %q:\n%s", want, written)
		}
	}
	if got := e2eRolesValue(t, ctx, `window.location.search`); !strings.Contains(got, "open=nosuch-2") {
		t.Errorf("the save under another name left %q in the address, want the new name", got)
	}
	if got := e2eRolesValue(t, ctx, `document.querySelector('#rows .role.open .c-name .nm').textContent.trim()`); got != "nosuch-2" {
		t.Errorf("the saved row shows the name %q", got)
	}
}
