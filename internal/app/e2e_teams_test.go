//go:build e2e

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestE2ETeams walks the team-definition screen the way a person uses it, over
// the real-scale fixture the mock was drawn from (spec-teams 4 章). It holds the
// screen to spec-teams 5 章: the list and its columns (S1〜S8), the words (T1・
// T5), the presses that must move only what stands near them (P1〜P11), the one
// line a save writes (F1・F2・F4・F6・F7), the three widths and both lights (W1〜
// D2), and the sentences `hergy validate` wrote (V1). The read-only walks use the
// fixture as it stands; the writes stand in a project of their own.
func TestE2ETeams(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	initLocales(t)
	copyTeamsFixture(t, f, cwd)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")

	base, stop := startServe(t, f)
	defer stop()

	// The run's one browser, and this test's tab in it (r-a107).
	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 4*time.Minute)
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

	t.Run("structure", func(t *testing.T) { e2eTeamsStructure(t, ctx, base) })
	t.Run("open and checks", func(t *testing.T) { e2eTeamsOpen(t, ctx, base) })
	t.Run("yaml", func(t *testing.T) { e2eTeamsYAML(t, ctx, base) })
	t.Run("widths and lights", func(t *testing.T) { e2eTeamsWidths(t, ctx, base) })

	// The writes stand in a project of their own, so the counts and the files the
	// walks above read stay as the fixture wrote them.
	f2 := newFixture(t)
	copyTeamsFixture(t, f2, cwd)
	writeFile(t, filepath.Join(f2.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f2.runningHerdr(t, "", "")
	base2, stop2 := startServe(t, f2)
	defer stop2()
	t.Run("mission and turns", func(t *testing.T) { e2eTeamsSaveShape(t, ctx, base2, f2) })
	t.Run("agreement and yaml", func(t *testing.T) { e2eTeamsSaveAgreement(t, ctx, base2, f2) })

	mu.Lock()
	defer mu.Unlock()
	if len(console) > 0 {
		t.Errorf("the team screen logged %d console errors: %v", len(console), console)
	}
}

// copyTeamsFixture puts the real-scale fixture under the test's project and
// outside settings directory, the way a person's project stands.
func copyTeamsFixture(t *testing.T, f *fixture, cwd string) {
	t.Helper()
	src := filepath.Join(cwd, "..", "..", "testdata", "teams-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
}

// e2eTeamsValue reads one expression as the text the page draws.
func e2eTeamsValue(t *testing.T, ctx context.Context, script string) string {
	t.Helper()
	return e2eRead[string](t, ctx, script)
}

// e2eTeamsInt reads one expression as the number the page draws.
func e2eTeamsInt(t *testing.T, ctx context.Context, script string) int {
	t.Helper()
	return e2eRead[int](t, ctx, script)
}

// e2eTeamsWait waits until one expression is true.
func e2eTeamsWait(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	e2eWait(t, ctx, script)
}

// e2eTeamsRowJS is a JS expression for the list row one definition name stands on.
func e2eTeamsRowJS(name string) string {
	return `[...document.querySelectorAll('#view .tdef')].find(function(r){return r.dataset.name===` + strconv.Quote(name) + `})`
}

// e2eTeamsPanelJS is a JS expression for one row's shape panel.
func e2eTeamsPanelJS(name string) string {
	return `document.querySelector(` + strconv.Quote(`.tdef[data-name="`+name+`"] .panel.team-p`) + `)`
}

// e2eTeamsAgreeJS is a JS expression for one row's agreement panel.
func e2eTeamsAgreeJS(name string) string {
	return `document.querySelector(` + strconv.Quote(`.tdef[data-name="`+name+`"] .panel.agree-p`) + `)`
}

// e2eTeamsClick presses the element one expression stands for, the way a person
// presses it. A point or a line of the figure is an SVG element, which has no
// click() of its own, so the press is a mouse event that bubbles to the panel's
// one listener (the way a real press does).
func e2eTeamsClick(t *testing.T, ctx context.Context, find string) {
	t.Helper()
	e2eReady(t, ctx)
	script := `(function(){var el=` + find + `;if(!el){return 'missing';}` +
		`if(el.disabled){return 'shut';}` +
		`if(typeof el.click==='function'){el.click();}else{el.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,view:window}));}` +
		`return 'pressed';})()`
	deadline := time.Now().Add(e2eDeadline)
	word := ""
	for time.Now().Before(deadline) {
		word = e2eVerdict(t, ctx, script)
		if word == "pressed" {
			return
		}
		time.Sleep(e2eStep)
	}
	if word == "missing" {
		t.Fatalf("no element for %s", find)
	}
	t.Fatalf("never became pressable: %s", find)
}

// e2eTeamsDiff is what one file gained and lost, counted by the text of the
// line: a save that writes one line in the middle of a list shifts every line
// under it, so a line-by-line reading would call the shift a change. The lines
// the fixture holds where the tests save are all different, so the text of a
// line is enough to say which one came and went.
func e2eTeamsDiff(before, after string) (added, out []string) {
	count := func(s string) map[string]int {
		m := map[string]int{}
		for _, l := range strings.Split(s, "\n") {
			m[l]++
		}
		return m
	}
	bm, am := count(before), count(after)
	for l, n := range am {
		for i := 0; i < n-bm[l]; i++ {
			added = append(added, l)
		}
	}
	for l, n := range bm {
		for i := 0; i < n-am[l]; i++ {
			out = append(out, l)
		}
	}
	sort.Strings(added)
	sort.Strings(out)
	return added, out
}

// e2eTeamsStructure is S1〜S3・S7・S8 and T1・T5: the band's チーム定義 is pressable
// and current, the two tabs, the six columns and six rows in the order the catalog
// reads them, the place pills, the counts, and the YAML tab's two groups of files.
func e2eTeamsStructure(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"), chromedp.WaitReady("#view .tdef"))
	e2eTeamsWait(t, ctx, `document.querySelectorAll('#view .tdef').length===6`)

	// S1: the band's チーム定義 is an anchor that lands here, and reads as current.
	if got := e2eTeamsValue(t, ctx, `[document.querySelector('[data-screen=teams]').tagName,document.querySelector('[data-screen=teams]').classList.contains('on'),document.querySelector('[data-screen=teams]').textContent.trim()].join('|')`); got != "SPAN|true|チーム定義6" {
		t.Errorf("the band's チーム定義 reads %q, want a current チーム定義6", got)
	}

	// S2: the two tabs, with no counts of their own.
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('#tabs .tab').length`); got != 2 {
		t.Errorf("the page has %d tabs, want 2", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('#tabs .tab')].map(function(b){return b.dataset.t+'='+b.textContent.trim()}).join(',')`); got != "list=一覧,yaml=YAML" {
		t.Errorf("the tabs read %q", got)
	}

	// S3: the six columns, the six rows, the order the catalog reads.
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.list .head > span')].map(function(e){return e.textContent.trim()}).join(',')`); got != ",名前,使命,置き場所,メンバー,ワーカー,約束" {
		t.Errorf("the head reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('#view .tdef')].map(function(r){return r.dataset.name}).join(',')`); got != "core,docs,draft,migrate,research,review" {
		t.Errorf("the rows read %q, want the catalog's order", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelectorAll('.c-place .pill').length+','+`+e2eTeamsRowJS("docs")+`.querySelectorAll('.c-place .pill.on').length+','+`+e2eTeamsRowJS("docs")+`.querySelectorAll('.c-place .pill.shadow').length`); got != "2,1,1" {
		t.Errorf("the docs row's places read %q, want the project's and the shadowed shared one", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.c-members').textContent+','+`+e2eTeamsRowJS("core")+`.querySelector('.c-turns').textContent`); got != "5,2" {
		t.Errorf("the core row's counts read %q, want 5 members and 2 workers", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.c-agree').textContent.trim()`); got != "53含めない 1" {
		t.Errorf("the core row's agreement reads %q, want 53 with one kept out", got)
	}
	// 1.2: the row counts what validate found — the figure, the agreement and the
	// role documents behind it.
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.c-name .vcount').textContent.trim()`); got != "問題 2警告 11" {
		t.Errorf("the core row's problems read %q", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.c-name .vcount').textContent.trim()`); got != "問題 1" {
		t.Errorf("the docs row's problems read %q, want the one problem of its broken worker", got)
	}
	// 1.9: a team.yaml that does not read says 使命 — and leaves its counts blank.
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.c-mission').textContent.trim()+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-mission').classList.contains('none')+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-members').textContent+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-turns').textContent`); got != "—,true,—,—" {
		t.Errorf("the docs row reads %q, want 使命 — and no counts", got)
	}

	// T1・T5: the words the screen uses on the list, and none of the words it must not.
	for _, want := range []string{"名前", "使命", "置き場所", "メンバー", "ワーカー", "約束"} {
		if got := e2eTeamsValue(t, ctx, `(document.querySelector('#view').textContent.indexOf(`+strconv.Quote(want)+`)>=0)+''`); got != "true" {
			t.Errorf("the screen does not read %q", want)
		}
	}
	for _, banned := range []string{"層", "layer", "担当", "役割", "版"} {
		if got := e2eTeamsValue(t, ctx, `(document.querySelector('#view').textContent.indexOf(`+strconv.Quote(banned)+`)>=0)+''`); got != "false" {
			t.Errorf("the screen reads the word %q", banned)
		}
	}

	// S8: the page's classes are the mock's.
	for _, sel := range []string{".list", ".head", ".tdef", ".sum", ".c-name", ".c-mission", ".c-place", ".c-members", ".c-turns", ".c-agree", ".tabs", ".tab"} {
		if got := e2eTeamsInt(t, ctx, `document.querySelectorAll(`+strconv.Quote(sel)+`).length`); got == 0 {
			t.Errorf("the page has nothing at %s", sel)
		}
	}

	// S7: the YAML tab's files — the project's eight and the shared six.
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams/yaml"), chromedp.WaitReady("#view .yed .ytext"))
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('.fgroup').length`); got != 2 {
		t.Errorf("the YAML tab has %d groups, want 2", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.fgroup .gh b')].map(function(b){return b.textContent}).join(',')`); got != "プロジェクト,共通" {
		t.Errorf("the groups read %q", got)
	}
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('.files .file').length`); got != 14 {
		t.Errorf("the YAML tab has %d files, want 14", got)
	}
	if got := e2eTeamsInt(t, ctx, `[...document.querySelectorAll('.files .file')].filter(function(b){return b.closest('.fgroup').querySelector('b').textContent==='プロジェクト'}).length`); got != 8 {
		t.Errorf("the project group has %d files, want 8", got)
	}
	if got := e2eTeamsInt(t, ctx, `[...document.querySelectorAll('.files .file')].filter(function(b){return b.closest('.fgroup').querySelector('b').textContent==='共通'}).length`); got != 6 {
		t.Errorf("the shared group has %d files, want 6", got)
	}
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('.file.hid').length`); got != 2 {
		t.Errorf("the YAML tab marks %d hidden files, want the shared docs' two", got)
	}
	if got := e2eTeamsValue(t, ctx, `document.querySelector('.file.hid .ft').textContent`); !strings.Contains(got, "隠れている") {
		t.Errorf("the hidden file reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, `document.querySelector('.yed .ytext').value`); !strings.HasPrefix(got, "version: 1") {
		t.Errorf("the YAML editor does not show the document:\n%s", got)
	}
}

// e2eTeamsOpen is S4〜S6, P1〜P3・P9, V1: a row opens under itself with the shape
// panel first, the figure's own check column carries validate's sentences, the
// 約束 tab draws the shared part with no name or copy of its own, and a definition
// that stands in both places says 効いている and 隠れている.
func e2eTeamsOpen(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"), chromedp.WaitReady("#view .tdef"))

	// P1: pressing the row opens it right under itself, and the address keeps it.
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("core")+`.classList.contains('open')`)
	if got := e2eTeamsValue(t, ctx, `window.location.search`); !strings.Contains(got, "open=core") {
		t.Errorf("opening the core row left %q in the address", got)
	}

	// S4: the mission's check list, the two tabs, and the shape panel below them.
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.detail > ul.mvm').tagName`); got != "UL" {
		t.Errorf("the open row's first child reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.dtabs').nextElementSibling.className`); !strings.Contains(got, "team-p") {
		t.Errorf("the shape panel does not stand under the tabs: %q", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .dtabs .dtab`)+`)].map(function(b){return b.textContent.trim()}).join(',')`); got != "チームの姿,チームの約束53" {
		t.Errorf("the tabs read %q", got)
	}

	// S5: the shape panel is 頭 → 図 → 検査の列 → 足.
	panel := e2eTeamsPanelJS("core")
	for _, want := range []string{".ph", ".gbody .gwrap svg", ".gbody .vlist", "footer.pf [data-act=t-save]"} {
		if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll(`+strconv.Quote(want)+`).length`); got == 0 {
			t.Errorf("the shape panel has nothing at %s", want)
		}
	}
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.ph .path').textContent`); !strings.HasSuffix(got, "teams/core/team.yaml") {
		t.Errorf("the shape panel's path reads %q", got)
	}
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.gn.member').length`); got != 5 {
		t.Errorf("the figure draws %d members, want 5", got)
	}
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.gn.worker').length`); got != 2 {
		t.Errorf("the figure draws %d workers, want 2", got)
	}
	// 1.4.1: the members whose role document is broken wear 問題, and no one else does.
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gn.member.err`)+`)].map(function(g){return g.dataset.id}).join(',')`); got != "impl-a,impl-b" {
		t.Errorf("the members in problem read %q", got)
	}

	// V1: the check column carries validate's own sentences, with the role
	// documents behind them.
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.vlist .vhead .vt').textContent`); got != "validate" {
		t.Errorf("the check column's head reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.vlist .vcount').textContent.trim()`); got != "問題 2警告 1" {
		t.Errorf("the figure's own check reads %q, want 2 problems and 1 warning", got)
	}
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.vlist .vitem').length`); got != 3 {
		t.Errorf("the check column shows %d items, want 3", got)
	}
	if got := e2eTeamsValue(t, ctx, `(function(){var m=`+panel+`.querySelector('.vlist .vsub');return m?m.textContent:''})()`); got != "ロールの文書" {
		t.Errorf("the role documents' head reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.vlist .vitem .msg').textContent`); !strings.Contains(got, "BUILD-4") {
		t.Errorf("the first problem reads %q", got)
	}

	// P9: pressing one item of the check column picks its node and shows its card.
	e2eTeamsClick(t, ctx, panel+`.querySelector('.vlist .vitem[data-node]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard[data-sel=node]').dataset.id`); got != "impl-a" {
		t.Errorf("pressing the first problem picked %q, want impl-a", got)
	}
	// 1.4.4: a member's card carries the assignment chips and the ターンを渡す相手 head.
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .links .uchip').textContent`); got != "ロール builder" {
		t.Errorf("the card's role chip reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .tl .gsub').textContent`); got != "core-impl-a · ターンを渡す相手" {
		t.Errorf("the card's head reads %q", got)
	}
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard .x')`)
	e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.gcard')`)

	// P3: the 約束 tab draws the shared part, with the team's own rules and none
	// of the name, copy or delete a role's own panel carries (S6).
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.dtabs .dtab[data-t=agree]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsAgreeJS("core"))
	agree := e2eTeamsAgreeJS("core")
	if got := e2eTeamsInt(t, ctx, agree+`.querySelectorAll('.rule').length`); got == 0 {
		t.Errorf("the agreement panel draws no rules")
	}
	if got := e2eTeamsValue(t, ctx, agree+`.querySelector('.ph .path').textContent`); !strings.HasSuffix(got, "teams/core/agreement.yaml") {
		t.Errorf("the agreement panel's path reads %q", got)
	}
	if got := e2eTeamsInt(t, ctx, agree+`.querySelectorAll('[data-act=copy],[data-act=del],[data-act=dup],[data-f=name]').length`); got != 0 {
		t.Errorf("the team's agreement panel carries a name, copy or delete of its own")
	}
	if got := e2eTeamsInt(t, ctx, `getComputedStyle(`+agree+`.querySelector('.pmain')).gridTemplateColumns.split(' ').filter(Boolean).length`); got != 2 {
		t.Errorf("2000: the agreement panel stands in %d columns, want the editor and the preview", got)
	}
	// T1・T4: the words of the 約束 part stand on the screen.
	if got := e2eTeamsValue(t, ctx, `(document.querySelector('#view').textContent.indexOf('チームの約束')>=0)+''`); got != "true" {
		t.Errorf("the 約束 tab does not read チームの約束")
	}

	// 1.3・1.4.5・1.9: a definition that does not read draws no figure and no
	// save — the check column alone — while the same name's shared file is shown
	// as hidden; a definition in both places says which one is in effect.
	docsPanel := e2eTeamsPanelJS("docs")
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, `!!`+docsPanel+`.querySelector('.vlist .vitem .msg')`)
	if got := e2eTeamsInt(t, ctx, docsPanel+`.querySelectorAll('.gwrap, footer.pf [data-act=t-save]').length`); got != 0 {
		t.Errorf("the broken definition's panel draws %d figures or saves, want the check column alone", got)
	}
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('.ph .state').textContent`); got != "効いている" {
		t.Errorf("the shared definition's state reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.panel.shadowed .state').textContent`); got != "隠れている" {
		t.Errorf("the hidden panel's state reads %q", got)
	}
	// V1: the broken worker's sentence stands in the figure's own check column.
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('.vlist .vitem .msg').textContent`); !strings.Contains(got, "gemini-flash") {
		t.Errorf("the docs figure's problem reads %q", got)
	}
}

// e2eTeamsYAML is P11・V1: the YAML tab is the shared part — the file's lines, the
// check the typing rides on, the line a problem names, the jump, 元に戻す and the
// refusal a text with a new error gets.
func e2eTeamsYAML(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams/yaml"), chromedp.WaitReady(".yed .ytext"))
	e2eTeamsClick(t, ctx, `[...document.querySelectorAll('.files .file')].find(function(b){return b.querySelector('code').textContent.trim()==='core/team.yaml'})`)
	e2eTeamsWait(t, ctx, `document.querySelector('.yed .path').textContent.indexOf('teams/core/team.yaml')>=0`)

	orig := e2eTeamsValue(t, ctx, `document.querySelector('.yed .ytext').value`)
	n0 := e2eTeamsInt(t, ctx, `document.querySelectorAll('.yed .vitem').length`)
	// V1: a worker's profile that is not there is an error on the worker's line.
	broken := strings.Replace(orig, "profile: codex-mini", "profile: nosuch-profile", 1)
	if broken == orig {
		t.Fatalf("the fixture's core/team.yaml no longer holds the worker profile the test breaks")
	}
	e2eSet(t, ctx, `document.querySelector('.yed .ytext')`, broken)
	e2eTeamsWait(t, ctx, `[...document.querySelectorAll('.yed .vitem .msg')].some(function(m){return m.textContent.indexOf('nosuch-profile')>=0})`)
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.yed .vitem .msg')].map(function(m){return m.textContent}).join('\n')`); !strings.Contains(got, "nosuch-profile") {
		t.Errorf("the broken line says %q", got)
	}
	if got := e2eTeamsInt(t, ctx, `Number((document.querySelector('.yed .vitem .lno').textContent||'').replace(/[^0-9]/g,''))`); got == 0 {
		t.Errorf("the problem names no line")
	}
	if got := e2eTeamsValue(t, ctx, `(document.querySelectorAll('.yed .gutter span.err').length>0)+''`); got != "true" {
		t.Errorf("the gutter marks no line as broken")
	}
	if got := e2eTeamsValue(t, ctx, `document.querySelector('[data-act=yaml-save]').disabled+''`); got != "true" {
		t.Errorf("a text with a new problem can be saved")
	}
	// P11: pressing the problem puts the cursor on its line.
	e2eTeamsClick(t, ctx, `document.querySelector('.yed .vitem')`)
	e2eTeamsWait(t, ctx, `document.querySelector('.yed .ytext').selectionStart>0`)
	// 元に戻す throws away both the text and the check it drew, and leaves the
	// problems the saved text already held.
	e2eTeamsClick(t, ctx, `document.querySelector('[data-act=yaml-revert]')`)
	e2eTeamsWait(t, ctx, `document.querySelector('.yed .ytext').value.indexOf('nosuch-profile')<0`)
	e2eTeamsWait(t, ctx, `document.querySelectorAll('.yed .vitem').length===`+strconv.Itoa(n0))
}

// e2eTeamsSaveShape is F1・F2・F7 and P2・P6・P7: the mission's one line, one turn's
// one line with its 確かめ, and a broken definition whose mission can still be
// written because the save only stops when a new problem appears.
func e2eTeamsSaveShape(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml")
	before := readFile(t, path)

	// F2: the mission alone moves, and the check rides on the typing (P2).
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"), chromedp.WaitReady(`.tdef[data-name="core"] .panel.team-p footer.pf [data-act=t-save]`))
	const mission = "E2E が書き換えた使命。"
	e2eSet(t, ctx, `document.querySelector('.in-mission')`, mission)
	e2eTeamsWait(t, ctx, `document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .dtabs .dtab[data-t=shape] .dot`)+`).length>0`)
	e2eTeamsWait(t, ctx, e2eTeamsPanelJS("core")+`.querySelector('.dirty').textContent.trim()==='1 か所'`)
	e2eTeamsClick(t, ctx, e2eTeamsPanelJS("core")+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsPanelJS("core")+`.querySelector('.ok')`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsPanelJS("core")+`.querySelector('.ok').textContent`); got != "保存しました" {
		t.Errorf("the shape save line reads %q", got)
	}
	after := readFile(t, path)
	added, removed := e2eTeamsDiff(before, after)
	if len(added) != 1 || len(removed) != 1 || added[0] != "mission: \""+mission+"\"" || !strings.HasPrefix(removed[0], "mission: ") {
		t.Errorf("the mission save wrote %v and lost %v, want the mission line alone", added, removed)
	}
	before = after

	// F1・P6・P7: one turn goes, its card shows the line that would leave and the
	// sentence that would appear (M2), and the save writes the one line.
	panel := e2eTeamsPanelJS("core")
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=impl-b]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard .row button[data-act=pick-edge][data-b=reviewer]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=edge]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard .gpl s')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .gpl s').textContent`); !strings.Contains(got, "core-reviewer") {
		t.Errorf("the line that would leave reads %q", got)
	}
	e2eTeamsWait(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)+`)].some(function(li){return li.textContent.indexOf('CORE-12')>=0})`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard .gdl .tag')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .gdl .tag').textContent`); got != "消した" {
		t.Errorf("the removed turn's tag reads %q", got)
	}
	e2eTeamsWait(t, ctx, panel+`.querySelector('.dirty').textContent.trim()==='1 か所'`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.ok')`)
	after = readFile(t, path)
	added, removed = e2eTeamsDiff(before, after)
	if len(added) != 0 || len(removed) != 1 || strings.TrimSpace(removed[0]) != "- impl-b -> reviewer" {
		t.Errorf("the turn save wrote %v and lost %v, want the turn's own line alone", added, removed)
	}
	if !strings.Contains(after, mission) {
		t.Errorf("the turn save lost the mission the test wrote:\n%s", after)
	}

	// F7: docs' team.yaml does not read — its worker names a profile that is not
	// there — and the shape panel offers no save (1.9). The YAML tab is where it
	// is put right: a text edit that only moves the mission adds no problem the
	// saved text did not already hold, so the save goes (X3).
	docsPath := filepath.Join(f.proj, ".hergy", "teams", "docs", "team.yaml")
	docsBefore := readFile(t, docsPath)
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams/yaml?file=docs.project.team"), chromedp.WaitReady(".yed .ytext"))
	e2eTeamsWait(t, ctx, `document.querySelector('.yed .path').textContent.indexOf('teams/docs/team.yaml')>=0`)
	e2eTeamsWait(t, ctx, `document.querySelectorAll('.yed .vitem').length>0`)
	docsText := e2eTeamsValue(t, ctx, `document.querySelector('.yed .ytext').value`)
	const docsMission = "E2E が直した文書の使命。"
	edited := strings.Replace(docsText, "mission: \"利用者向けの文書を整える\"", "mission: \""+docsMission+"\"", 1)
	if edited == docsText {
		t.Fatalf("the fixture's docs/team.yaml no longer holds the mission the test moves")
	}
	e2eSet(t, ctx, `document.querySelector('.yed .ytext')`, edited)
	// The problem the file already held is not a new one, so the save is open.
	e2eTeamsWait(t, ctx, `!document.querySelector('[data-act=yaml-save]').disabled`)
	e2eTeamsClick(t, ctx, `document.querySelector('[data-act=yaml-save]')`)
	e2eTeamsWait(t, ctx, `!!document.querySelector('.yed .ok')`)
	docsAfter := readFile(t, docsPath)
	added, removed = e2eTeamsDiff(docsBefore, docsAfter)
	if len(added) != 1 || len(removed) != 1 || added[0] != "mission: \""+docsMission+"\"" {
		t.Errorf("the docs mission save wrote %v and lost %v, want the mission line alone", added, removed)
	}
}

// e2eTeamsSaveAgreement is F4・F6 and P10: a rule's 含めない is one line of the
// team's own agreement (X2), the save leaves the rest byte for byte, and a file
// changed outside the screen is refused with nothing written.
func e2eTeamsSaveAgreement(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "teams", "research", "agreement.yaml")
	before := readFile(t, path)

	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=research"), chromedp.WaitReady(`.tdef[data-name="research"] .panel.team-p`))
	e2eTeamsClick(t, ctx, `document.querySelector(`+strconv.Quote(`.tdef[data-name="research"] .dtabs .dtab[data-t=agree]`)+`)`)
	agree := e2eTeamsAgreeJS("research")
	e2eTeamsWait(t, ctx, `!!`+agree+`.querySelector('.rule .rsum')`)
	// The rule whose line the test will move: the first rule of the first section.
	e2eTeamsClick(t, ctx, agree+`.querySelector('.sec .rule .rsum')`)
	e2eTeamsWait(t, ctx, `!!`+agree+`.querySelector('.rule.open [data-act=in-prompt][data-v=false]')`)
	e2eTeamsClick(t, ctx, agree+`.querySelector('.rule.open [data-act=in-prompt][data-v=false]')`)
	e2eTeamsWait(t, ctx, `!!document.querySelector(`+strconv.Quote(`.tdef[data-name="research"] .dtabs .dtab[data-t=agree] .dot`)+`)`)
	e2eTeamsWait(t, ctx, `!`+agree+`.querySelector('.pf [data-act=save]').disabled`)
	e2eTeamsClick(t, ctx, agree+`.querySelector('.pf [data-act=save]')`)
	e2eTeamsWait(t, ctx, `!!`+agree+`.querySelector('.pf .ok')`)
	if got := e2eTeamsValue(t, ctx, agree+`.querySelector('.pf .ok').textContent`); got != "保存しました" {
		t.Errorf("the agreement save line reads %q", got)
	}
	after := readFile(t, path)
	added, removed := e2eTeamsDiff(before, after)
	if len(added) != 1 || len(removed) != 0 || !strings.Contains(added[0], "in_prompt: false") {
		t.Errorf("keeping one rule out wrote %v and lost %v, want the in_prompt line alone", added, removed)
	}

	// F6: the rule goes back in (one more change), the file changes under the
	// screen, and the save is refused with nothing written.
	e2eTeamsClick(t, ctx, agree+`.querySelector('.sec .rule .rsum')`)
	e2eTeamsWait(t, ctx, `!!`+agree+`.querySelector('.rule.open [data-act=in-prompt][data-v=true]')`)
	e2eTeamsClick(t, ctx, agree+`.querySelector('.rule.open [data-act=in-prompt][data-v=true]')`)
	e2eTeamsWait(t, ctx, `!`+agree+`.querySelector('.pf [data-act=save]').disabled`)
	writeFile(t, path, strings.Replace(after, "title: ", "title: 外から ", 1))
	outside := readFile(t, path)
	e2eTeamsClick(t, ctx, agree+`.querySelector('.pf [data-act=save]')`)
	e2eTeamsWait(t, ctx, `!!`+agree+`.querySelector('.pf .errline.doc:not([hidden]), .pf .refusal')`)
	if got := readFile(t, path); got != outside {
		t.Errorf("a refused save wrote the file:\n%s", got)
	}
}

// e2eTeamsWidths is W1〜W2 and D1: no page flows sideways at 2000, 1440 or 500, in
// both lights, the figure fits at 500, and the ink reads against its ground.
func e2eTeamsWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	ready := map[string]string{
		"/teams":               `document.querySelectorAll('#view .tdef').length===6`,
		"/teams?open=core":     `!!document.querySelector(` + strconv.Quote(`.tdef[data-name="core"] .panel.team-p .vlist .vhead`) + `)`,
		"/teams?open=docs":     `!!document.querySelector(` + strconv.Quote(`.tdef[data-name="docs"] .panel.team-p .vlist .vhead`) + `)`,
		"/teams?open=research": `!!document.querySelector(` + strconv.Quote(`.tdef[data-name="research"] .panel.team-p .vlist .vhead`) + `)`,
		"/teams/yaml":          `!!document.querySelector('.yed .ytext')`,
	}
	paths := []string{"/teams?open=core", "/teams?open=docs", "/teams?open=research", "/teams/yaml", "/teams"}

	for _, size := range []int64{2000, 1440, 500} {
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
			for _, path := range paths {
				e2eRun(t, ctx, chromedp.Navigate(base+path))
				e2eTeamsWait(t, ctx, ready[path])
				if theme == "" {
					e2eRun(t, ctx, chromedp.Evaluate(`delete document.documentElement.dataset.theme`, nil))
				} else {
					e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.dataset.theme=`+strconv.Quote(theme), nil))
				}
				if over := e2eTeamsInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
					t.Errorf("%s at %d/%s flows %dpx sideways", path, size, theme, over)
				}
			}
		}
	}

	// W2: at 500 the figure fits the width and no node is cut.
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 900))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"))
	e2eTeamsWait(t, ctx, ready["/teams?open=core"])
	if got := e2eTeamsValue(t, ctx, `getComputedStyle(document.querySelector('.head')).display`); got != "none" {
		t.Errorf("500: the list's head row is still shown")
	}
	if got := e2eTeamsInt(t, ctx, `(function(){var g=document.querySelector(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gscroll`)+`);return g?g.scrollWidth-g.clientWidth:0})()`); got > 1 {
		t.Errorf("500: the figure flows %dpx inside its box", got)
	}

	// D1: the ink reads against its ground in both lights, and the OS's request
	// for dark is honoured when the page is told nothing.
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"))
	e2eTeamsWait(t, ctx, ready["/teams?open=core"])
	for _, theme := range []string{"light", "dark"} {
		e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.dataset.theme=`+strconv.Quote(theme), nil))
		if got := e2eRead[float64](t, ctx, e2eRolesContrast); got < 4.5 {
			t.Errorf("the body's ink against its ground is %.2f in %s, want at least 4.5", got, theme)
		}
	}
	e2eRun(t, ctx, chromedp.Evaluate(`delete document.documentElement.dataset.theme`, nil))
	e2eRun(t, ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}}).Do(ctx)
	}))
	if got := e2eTeamsValue(t, ctx, `getComputedStyle(document.body).backgroundColor`); got != "rgb(15, 18, 24)" {
		t.Errorf("the OS dark ground is %q", got)
	}
}
