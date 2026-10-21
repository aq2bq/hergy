//go:build e2e

package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/scaffold"
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

	console := e2eListen(ctx, false)

	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	t.Run("structure", func(t *testing.T) { e2eTeamsStructure(t, ctx, base) })
	t.Run("open and checks", func(t *testing.T) { e2eTeamsOpen(t, ctx, base) })
	t.Run("yaml", func(t *testing.T) { e2eTeamsYAML(t, ctx, base) })
	t.Run("widths and lights", func(t *testing.T) { e2eTeamsWidths(t, ctx, base) })
	t.Run("graph problems", func(t *testing.T) { e2eTeamsGraphProblems(t, ctx, base) })
	t.Run("worker purpose", func(t *testing.T) { e2eTeamsWorkerPurpose(t, ctx, base) })

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

	// The version-7 walks stand in projects of their own: one adds, copies and
	// takes away definitions, so the next one reads a fixture nothing has
	// written over (spec-teams B1〜B8).
	f3 := newFixture(t)
	copyTeamsFixture(t, f3, cwd)
	writeFile(t, filepath.Join(f3.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f3.runningHerdr(t, "", "")
	base3, stop3 := startServe(t, f3)
	defer stop3()
	t.Run("add, copy and delete", func(t *testing.T) { e2eTeamsAddCopyDelete(t, ctx, base3, f3) })

	f4 := newFixture(t)
	copyTeamsFixture(t, f4, cwd)
	writeFile(t, filepath.Join(f4.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f4.runningHerdr(t, "", "")
	base4, stop4 := startServe(t, f4)
	defer stop4()
	t.Run("topology preview", func(t *testing.T) { e2eTeamsTopology(t, ctx, base4, f4) })

	// The audit's own walk reads the page a URL opens, the band the YAML box stops
	// above, and a new row's name, so it stands in a project nothing has written
	// over (監査 R1〜R8).
	f5 := newFixture(t)
	copyTeamsFixture(t, f5, cwd)
	writeFile(t, filepath.Join(f5.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f5.runningHerdr(t, "", "")
	base5, stop5 := startServe(t, f5)
	defer stop5()
	t.Run("audit 8", func(t *testing.T) { e2eTeamsAudit(t, ctx, base5, f5) })

	if lines := console.Lines(); len(lines) > 0 {
		t.Errorf("the team screen logged %d console errors: %v", len(lines), lines)
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
	// R2: pressing a tab moves the screen and the address (spec-teams 2 章).
	e2eTeamsClick(t, ctx, `document.querySelector('#tabs .tab[data-t=yaml]')`)
	e2eTeamsWait(t, ctx, `!!document.querySelector('.yed .ytext')`)
	if got := e2eTeamsValue(t, ctx, `window.location.pathname`); got != "/teams/yaml" {
		t.Errorf("pressing YAML left %q in the address", got)
	}
	e2eTeamsClick(t, ctx, `document.querySelector('#tabs .tab[data-t=list]')`)
	e2eTeamsWait(t, ctx, `document.querySelectorAll('#view .tdef').length===6`)
	if got := e2eTeamsValue(t, ctx, `window.location.pathname`); got != "/teams" {
		t.Errorf("pressing 一覧 left %q in the address", got)
	}

	// S3: the six columns, the six rows, the order the catalog reads.
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.list .head > span')].map(function(e){return e.textContent.trim()}).join(',')`); got != ",名前,使命,置き場所,メンバー,ワーカー,ルール" {
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
	// S3・R10: the docs definition reads — only a name it writes is not in
	// profiles.yaml — so its row counts its problem and the role documents
	// behind it, like every other row (spec-teams 1.9).
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.c-name .vcount').textContent.trim()`); got != "問題 1警告 6" {
		t.Errorf("the docs row's problems read %q, want the problem of its unresolved worker profile", got)
	}
	// 1.9・R10: the mission, the members and the workers stand for a file that
	// reads, and only a file that is not YAML at all draws — and counts — nothing.
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.c-mission').textContent.trim()+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-mission').classList.contains('none')+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-members').textContent+','+`+e2eTeamsRowJS("docs")+`.querySelector('.c-turns').textContent`); got != "利用者向けの文書を整える,false,3,6" {
		t.Errorf("the docs row reads %q, want the mission and its counts", got)
	}

	// T1・T5: the words the screen uses on the list, and none of the words it must not.
	for _, want := range []string{"名前", "使命", "置き場所", "メンバー", "ワーカー", "ルール"} {
		if got := e2eTeamsValue(t, ctx, `(document.querySelector('#view').textContent.indexOf(`+strconv.Quote(want)+`)>=0)+''`); got != "true" {
			t.Errorf("the screen does not read %q", want)
		}
	}
	for _, banned := range []string{"層", "layer", "担当", "役割", "版", "約束"} {
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
// ルール tab draws the shared part with no name or copy of its own, and a definition
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
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .dtabs .dtab`)+`)].map(function(b){return b.textContent.trim()}).join(',')`); got != "トポロジー,ルール53" {
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

	// P3: the ルール tab draws the shared part, with the team's own rules and none
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
	// T1・T4: the words of the ルール part stand on the screen — the tab reads
	// ルール with its own count, and the word the tab used to read is gone from
	// the list screen (spec-teams B9; the list's own words are checked in the
	// structure walk, and the team's own agreement carries its title from its
	// file).
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .dtabs .dtab`)+`)].map(function(b){return b.dataset.t+'='+b.textContent.trim()}).join(',')`); got != "shape=トポロジー,agree=ルール53" {
		t.Errorf("the open row's tabs read %q, want トポロジー and ルール53", got)
	}
	// R4: a sentence about the document as a whole — the members' reading volume
	// — marks the first line of the preview alone, not every heading and blank
	// line (spec-roles 1.2.6); the roles screen's own part reads the same way.
	if got := e2eTeamsInt(t, ctx, agree+`.querySelectorAll('.yprev .yl.warn').length`); got != 1 {
		t.Errorf("the agreement's preview marks %d lines as a warning, want the first line alone", got)
	}
	if got := e2eTeamsInt(t, ctx, agree+`.querySelectorAll('.yprev .yl.err').length`); got != 0 {
		t.Errorf("the agreement's preview marks %d lines as a problem, want none", got)
	}
	// R3: the head is the path alone — the place stands in the row's pills, not
	// here (spec-teams 1.5).
	if got := e2eTeamsValue(t, ctx, `(`+agree+`.querySelector('.ph').textContent.indexOf('プロジェクト')>=0)+''`); got != "false" {
		t.Errorf("the agreement panel's head names a place")
	}
	// R5: nothing has changed, so 元に戻す is not offered — the same as the roles
	// screen's own part.
	if got := e2eTeamsValue(t, ctx, agree+`.querySelector('.pf [data-act=revert]').hidden+''`); got != "true" {
		t.Errorf("the agreement's foot offers 元に戻す with nothing changed")
	}

	// 1.3・1.4.5・1.9・R10: a definition whose names do not resolve is drawn like
	// one that reads — the figure, the mission, the counts and the save — and its
	// one problem stands on the worker it names; the same name's shared file is
	// shown as hidden, and a definition in both places says which one is in
	// effect.
	docsPanel := e2eTeamsPanelJS("docs")
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, `!!`+docsPanel+`.querySelector('.vlist .vitem .msg')`)
	if got := e2eTeamsInt(t, ctx, docsPanel+`.querySelectorAll('.gwrap svg, footer.pf [data-act=t-save]').length`); got != 2 {
		t.Errorf("the definition whose profile does not resolve draws %d figures and saves, want the figure and the save", got)
	}
	// V1・R10: its own sentence stands in the check column, which counts the one
	// problem (the role documents behind the row are not this column's).
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('.vlist .vcount').textContent.trim()`); got != "問題 1" {
		t.Errorf("the docs panel's check reads %q, want its one problem", got)
	}
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('.vlist .vitem .msg').textContent`); !strings.Contains(got, "gemini-flash") {
		t.Errorf("the docs figure's problem reads %q", got)
	}
	// 1.4.1・R10: the problem stands on the worker 翻訳's own point, with 問題.
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="docs"] .panel.team-p .gn.worker.err`)+`)].map(function(g){return g.dataset.i}).join(',')`); got != "0" {
		t.Errorf("the workers in problem read %q, want the 翻訳 worker alone", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="docs"] .panel.team-p .gn.worker.err`)+`)].map(function(g){return g.querySelector('.bdg text').textContent}).join(',')`); got != "問題" {
		t.Errorf("the worker's badge reads %q, want 問題", got)
	}
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('.ph .state').textContent`); got != "効いている" {
		t.Errorf("the shared definition's state reads %q", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("docs")+`.querySelector('.panel.shadowed .state').textContent`); got != "隠れている" {
		t.Errorf("the hidden panel's state reads %q", got)
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
	e2eTeamsWait(t, ctx, e2eTeamsPanelJS("core")+`.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`)
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
	// R6: the line the change would take away stands once, in the server's own
	// words (effect.lines[].removed), and the check column under it holds the
	// server's problems alone — the screen builds no sentence of its own (X3).
	if got := e2eTeamsInt(t, ctx, `(function(){var s=`+panel+`.querySelector('.gcard .gpl s');return s?[...s.parentElement.children].filter(function(n){return n.tagName==='S'&&n.textContent.indexOf('core-reviewer')>=0}).length:0})()`); got != 1 {
		t.Errorf("the card strikes the line that would leave %d times, want once", got)
	}
	if got := e2eTeamsInt(t, ctx, `[...`+panel+`.querySelectorAll('.gcard .vm li')].filter(function(li){return li.textContent.indexOf('core-reviewer')>=0}).length`); got != 0 {
		t.Errorf("the check column holds %d lines of the change as a problem, want none", got)
	}
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard .gdl .tag')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .gdl .tag').textContent`); got != "消した" {
		t.Errorf("the removed turn's tag reads %q", got)
	}
	e2eTeamsWait(t, ctx, panel+`.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`)
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

	// F7・R10: docs' team.yaml reads — only a profile its worker names is not in
	// profiles.yaml — so the YAML tab holds its text and its own problem: a text
	// edit that only moves the mission adds no problem the saved text did not
	// already hold, so the save goes (X3).
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

	// F7・R10: the same definition's shape panel offers its save now (1.9). The
	// problem it holds is the file's own, so a mission edit adds nothing new and
	// the save goes; a change that adds a problem of its own shuts it again (X3).
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=docs"), chromedp.WaitReady(`.tdef[data-name="docs"] .panel.team-p footer.pf [data-act=t-save]`))
	docsPanel := e2eTeamsPanelJS("docs")
	const docsMission2 = "E2E が姿から直した使命。"
	e2eSet(t, ctx, `document.querySelector('.in-mission')`, docsMission2)
	e2eTeamsWait(t, ctx, docsPanel+`.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`)
	if got := e2eTeamsValue(t, ctx, docsPanel+`.querySelector('[data-act=t-save]').disabled+''`); got != "false" {
		t.Errorf("the docs shape save is shut although the only problem is the file's own")
	}
	e2eTeamsClick(t, ctx, docsPanel+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!!`+docsPanel+`.querySelector('.ok')`)
	docsAfter2 := readFile(t, docsPath)
	added, removed = e2eTeamsDiff(docsAfter, docsAfter2)
	if len(added) != 1 || len(removed) != 1 || added[0] != "mission: \""+docsMission2+"\"" {
		t.Errorf("the docs shape save wrote %v and lost %v, want the mission line alone", added, removed)
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
	// R5: a change is here, so 元に戻す stands.
	if got := e2eTeamsValue(t, ctx, agree+`.querySelector('.pf [data-act=revert]').hidden+''`); got != "false" {
		t.Errorf("the agreement's foot hides 元に戻す although the draft moved")
	}
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

	// R5 again: 元に戻す puts the draft back where the save is held to, and with
	// nothing left to take back the button goes (the roles screen's part too).
	e2eTeamsClick(t, ctx, agree+`.querySelector('.pf [data-act=revert]')`)
	e2eTeamsWait(t, ctx, agree+`.querySelector('.pf [data-act=revert]').hidden+''==='true'`)
	if got := e2eTeamsValue(t, ctx, agree+`.querySelector('.pf [data-act=save]').disabled+''`); got != "true" {
		t.Errorf("the agreement's save stands with nothing changed")
	}
}

// e2eTeamsWorkerPurpose is R7: ＋ ワーカー opens the card with the cursor in the
// purpose, the purpose takes the whole word while the cursor stays in the field,
// the card writes the check's own sentence rather than 問題 undefined, and the
// added worker counts as one place — one that a save may write, until the
// purpose it lacks adds a problem the file did not hold (1.4.4・1.4.6, X3).
func e2eTeamsWorkerPurpose(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"), chromedp.WaitReady(`.tdef[data-name="core"] .panel.team-p .gwrap svg`))
	panel := e2eTeamsPanelJS("core")
	field := panel + `.querySelector('.gcard[data-sel=worker] .in-purpose')`

	e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=lead]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=worker-add]')`)
	e2eTeamsWait(t, ctx, `!!`+field)
	if got := e2eTeamsValue(t, ctx, `(document.activeElement===`+field+`)+''`); got != "true" {
		t.Errorf("＋ ワーカー left the cursor outside the purpose")
	}
	// A worker with no purpose is a problem the saved file did not hold, so the
	// save is shut until the purpose is there (X3).
	e2eTeamsWait(t, ctx, panel+`.querySelector('[data-act=t-save]').disabled+''==='true'`)
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`)+`)].map(function(li){return li.textContent}).join('|')`); !strings.Contains(got, "purpose") || strings.Contains(got, "undefined") {
		t.Errorf("the new worker's card reads %q, want the check's own sentence", got)
	}
	// One letter at a time, the way a person types: the field is not drawn again,
	// so the cursor stays and the word collects.
	const purpose = "ログの整理"
	letters := []rune(purpose)
	for i := 1; i <= len(letters); i++ {
		e2eSet(t, ctx, field, string(letters[:i]))
		e2eTeamsWait(t, ctx, `(document.activeElement===`+field+`)+''==='true'`)
		if got := e2eTeamsValue(t, ctx, field+`.value`); got != string(letters[:i]) {
			t.Errorf("after %d letters the purpose reads %q, want %q", i, got, string(letters[:i]))
		}
	}
	// One worker, however many letters wrote its purpose (1.4.6), and the problem
	// its empty purpose added is gone, so the save is open again.
	e2eTeamsWait(t, ctx, panel+`.querySelector('footer.pf .dirty').textContent.trim()==='1 か所'`)
	e2eTeamsWait(t, ctx, panel+`.querySelector('[data-act=t-save]').disabled+''==='false'`)
	if got := e2eTeamsValue(t, ctx, `(function(){var u=`+panel+`.querySelector('.gcard[data-sel=worker] .vm');return u?u.textContent:''})()`); strings.Contains(got, "purpose") {
		t.Errorf("the worker's card still holds its purpose's problem: %q", got)
	}
	// 元に戻す puts the definition back, and the count with it.
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('[data-act=t-revert]:not([hidden])')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('[data-act=t-revert]')`)
	e2eTeamsWait(t, ctx, panel+`.querySelector('footer.pf .dirty').textContent.trim()===''`)
}

// e2eTeamsGraphProblems is R8・R9, with the two leftovers R11・R12: the card of a
// line that would take a member's last turn says the problem the change would add
// before it is made (5.4 M3) — and keeps the line that would leave beside it even
// though the change is what stops the definition (1.4.4, R11) — the problem of
// the draft then stands on that member's own point and names it in the check
// column (1.4.1・1.7), and a warning about the team's own agreement stands under
// チームのルール and opens that rule in the ルール tab, brought into the window with
// the cursor in the rule's body, from the press and from the address alike
// (1.4.5, 2, 監査 R12).
func e2eTeamsGraphProblems(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"), chromedp.WaitReady(`.tdef[data-name="core"] .panel.team-p .gwrap svg`))
	panel := e2eTeamsPanelJS("core")
	stop := `[...document.querySelectorAll(` + strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gcard .vm li`) + `)].some(function(li){return li.textContent.indexOf('analyst から出るターンがありません')>=0})`

	// R8: analyst hands out three turns and hires no worker, so taking the last
	// one away leaves it handing the turn to nobody. The card says so before the
	// press; the two cards before it do not.
	for _, to := range []string{"impl-a", "impl-b", "lead"} {
		e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=analyst]')`)
		e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
		e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard .row button[data-act=pick-edge][data-b=`+to+`]')`)
		e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=edge]')`)
		e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-ask][data-a=analyst][data-b=`+to+`]')`)
		e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard [data-act=turn-del]')`)
		if to == "lead" {
			e2eTeamsWait(t, ctx, stop)
			// R11: the line the change would take away stands on the card even
			// though the change itself is what stops the definition (1.4.4).
			e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard .gpl s')`)
			if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard .gpl s').textContent`); got != "- core-lead（ロール lead）" {
				t.Errorf("the card's line that would leave reads %q, want - core-lead（ロール lead）", got)
			}
		} else if got := e2eTeamsValue(t, ctx, `(`+stop+`)+''`); got != "false" {
			t.Errorf("the card of the %s turn already names the problem that would come", to)
		}
		e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-del]')`)
		e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard .gdl .tag')`)
	}

	// R9: the problem of the draft stands on analyst's own point, wearing 問題,
	// and the check column says where it stands (1.4.1・1.7).
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gn.member[data-id=analyst].err')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gn.member[data-id=analyst] .bdg text').textContent`); got != "問題" {
		t.Errorf("analyst's badge reads %q, want 問題", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .vlist .vitem`)+`)].filter(function(b){return b.textContent.indexOf('analyst から出るターンがありません')>=0}).map(function(b){return b.querySelector('.lno').textContent}).join(',')`); got != "analyst" {
		t.Errorf("the check column puts analyst's problem at %q, want analyst", got)
	}

	// R9: scout's turn to the human goes, so the team's own agreement's rule
	// RES-6 warns. It stands under チームのルール — the role documents' head is not
	// its place — and its press opens that rule in the ルール tab (1.4.5). The
	// window is the one a person reads on (1440×1000), where the rule a press
	// opens falls below the save's band (監査 R12).
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=research"), chromedp.WaitReady(`.tdef[data-name="research"] .panel.team-p .gwrap svg`))
	rp := e2eTeamsPanelJS("research")
	e2eTeamsClick(t, ctx, rp+`.querySelector('.gn.member[data-id=scout]')`)
	e2eTeamsWait(t, ctx, `!!`+rp+`.querySelector('.gcard[data-sel=node]')`)
	e2eTeamsClick(t, ctx, rp+`.querySelector('.gcard .row button[data-act=pick-edge][data-b=human]')`)
	e2eTeamsWait(t, ctx, `!!`+rp+`.querySelector('.gcard[data-sel=edge]')`)
	e2eTeamsClick(t, ctx, rp+`.querySelector('.gcard [data-act=turn-ask][data-a=scout][data-b=human]')`)
	e2eTeamsWait(t, ctx, `!!`+rp+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsClick(t, ctx, rp+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, `!!`+rp+`.querySelector('.gcard .gdl .tag')`)
	item := `[...document.querySelectorAll(` + strconv.Quote(`.tdef[data-name="research"] .panel.team-p .vlist .vitem`) + `)].find(function(b){return b.textContent.indexOf('RES-6')>=0})`
	e2eTeamsWait(t, ctx, `!!`+item)
	if got := e2eTeamsValue(t, ctx, rp+`.querySelector('.vlist .vsub').textContent`); got != "チームのルール" {
		t.Errorf("the check column's first heading reads %q, want チームのルール", got)
	}
	if got := e2eTeamsValue(t, ctx, item+`.querySelector('.lno').textContent`); got != "agreement.yaml" {
		t.Errorf("the team agreement's warning stands at %q, want agreement.yaml", got)
	}
	e2eTeamsClick(t, ctx, item)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsAgreeJS("research")+`.querySelector('.rule.open')`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsAgreeJS("research")+`.querySelector('.rule.open .rid b').textContent`); got != "RES-6" {
		t.Errorf("the press opened the rule %q, want RES-6", got)
	}
	if got := e2eTeamsValue(t, ctx, `window.location.search`); !strings.Contains(got, "agree=research") {
		t.Errorf("opening the rule left %q in the address", got)
	}

	// R12: the rule the press opened stands in the window — its head below the
	// top, its body above the save's band — and its body has the cursor (1.7).
	// Without the press's own move the rule falls below the band, where the
	// person who pressed the check cannot see it.
	above := `(function(){var p=` + e2eTeamsAgreeJS("research") + `;var r=p.querySelector('.rule.open'),t=p.querySelector('.rule.open [data-f=text]'),f=p.querySelector('footer.pf');if(!r||!t||!f)return false;var rr=r.getBoundingClientRect(),tr=t.getBoundingClientRect(),fr=f.getBoundingClientRect();return rr.top>=0&&tr.top>rr.top&&tr.bottom<=fr.top})()`
	cursor := `(document.activeElement&&document.activeElement.dataset.f)==='text'`
	e2eTeamsWait(t, ctx, above)
	e2eTeamsWait(t, ctx, cursor)

	// R12: the address opens the same rule the same way.
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?agree=research&rule=research.RES-6"), chromedp.WaitReady(`.tdef[data-name="research"] .panel.agree-p .rule.open`))
	e2eTeamsWait(t, ctx, above)
	e2eTeamsWait(t, ctx, cursor)
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
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
					e2eTheme(t, ctx, "")
				} else {
					e2eTheme(t, ctx, theme)
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

	// W3: a definition that is not there yet, and the topology tab's YAML, flow
	// no further than the page at the three widths and in both lights; at 500
	// the definition's own acts stand on the tab's own row (spec-teams B1〜B8,
	// W3).
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"))
	e2eTeamsWait(t, ctx, ready["/teams"])
	e2eTeamsClick(t, ctx, `document.querySelector('.toolbar [data-act=new]')`)
	e2eTeamsWait(t, ctx, `!!document.querySelector('.tdef[data-name^="~new"] .in-name')`)
	for _, size := range []int64{2000, 1440, 500} {
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
			if theme == "" {
				e2eTheme(t, ctx, "")
			} else {
				e2eTheme(t, ctx, theme)
			}
			if over := e2eTeamsInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
				t.Errorf("a new row at %d/%s flows %dpx sideways", size, theme, over)
			}
			if got := e2eTeamsValue(t, ctx, `getComputedStyle(document.querySelector('.tdef[data-name^="~new"] .panel.team-p .tprev')).display`); (size <= 1100 && got != "none") || (size > 1100 && got == "none") {
				t.Errorf("a new row's YAML preview at %d/%s reads display %q", size, theme, got)
			}
		}
	}
	// 500: 定義の操作 stands on the tab's own row, under the tabs.
	if got := e2eTeamsValue(t, ctx, `(function(){var d=document.querySelector('.tdef[data-name^="~new"] .dacts'),row=d.closest('.tdef');if(!d||!row)return 'no';var dr=d.getBoundingClientRect(),rr=row.getBoundingClientRect();return (Math.round(dr.width/rr.width*100)>=80?'wide':'narrow')+'|'+getComputedStyle(d).marginLeft;})()`); got != "wide|0px" {
		t.Errorf("500: the definition's acts read %q, want the tab's own row", got)
	}

	// R1: the figure is drawn at the width of its own viewBox — the sheet takes the
	// width the SVG carries — and the card of a chosen line covers no point
	// (1.4.2・1.4.4). Checked at the two widths a person works at, on a team with
	// workers and on one without.
	for _, size := range []int64{2000, 1440} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
		for _, name := range []string{"core", "research"} {
			e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open="+name))
			panel := e2eTeamsPanelJS(name)
			e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gwrap svg')`)
			if got := e2eTeamsInt(t, ctx, `(function(){var s=`+panel+`.querySelector('.gwrap svg'),vb=s.dataset.vb.split(' ');return Math.round(s.getBoundingClientRect().width)-Number(vb[2]);})()`); got != 0 {
				t.Errorf("%s at %d: the figure stands %dpx from the width of its viewBox", name, size, got)
			}
			e2eTeamsClick(t, ctx, panel+`.querySelector('.hit')`)
			e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard')`)
			if got := e2eTeamsInt(t, ctx, `(function(){var p=`+panel+`,c=p.querySelector('.gcard');if(!c)return -1;var cr=c.getBoundingClientRect(),n=0;p.querySelectorAll('.gn .box').forEach(function(b){var r=b.getBoundingClientRect();if(Math.min(cr.right,r.right)-Math.max(cr.left,r.left)>0&&Math.min(cr.bottom,r.bottom)-Math.max(cr.top,r.top)>0)n++;});return n;})()`); got != 0 {
				t.Errorf("%s at %d: the card of the chosen line covers %d points", name, size, got)
			}
		}
	}

	// D1: the ink reads against its ground in both lights, and the OS's request
	// for dark is honoured when the page is told nothing.
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"))
	e2eTeamsWait(t, ctx, ready["/teams?open=core"])
	for _, theme := range []string{"light", "dark"} {
		e2eTheme(t, ctx, theme)
		if got := e2eRead[float64](t, ctx, e2eRolesContrast); got < 4.5 {
			t.Errorf("the body's ink against its ground is %.2f in %s, want at least 4.5", got, theme)
		}
	}
	e2eTheme(t, ctx, "")
	e2eDark(t, ctx)
	if got := e2eTeamsValue(t, ctx, `getComputedStyle(document.body).backgroundColor`); got != "rgb(15, 18, 24)" {
		t.Errorf("the OS dark ground is %q", got)
	}
}

// e2eTeamsAddCopyDelete walks the version-7 additions to the list: the toolbar's
// ＋ チーム定義を追加 (B1), a new row's name problems and the files a save would
// write (B2, N1〜N4), a duplicate (B3, N5), a copy into the other place (B4, N6),
// taking one place's folder away (B6, N7), and 取り消し (B5, N8). Every save is
// read back from the filesystem, so the walk is about what was written rather
// than about what the screen says.
func e2eTeamsAddCopyDelete(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"), chromedp.WaitReady("#view .tdef"))
	e2eTeamsWait(t, ctx, `document.querySelectorAll('#view .tdef').length===6`)

	draft := `document.querySelector('.tdef[data-name^="~new"]')`
	rows := func() int { return e2eTeamsInt(t, ctx, `document.querySelectorAll('#view .tdef').length`) }

	// B1: the toolbar's button stands above the list.
	if got := e2eTeamsValue(t, ctx, `document.querySelector('.toolbar [data-act=new]').textContent.trim()`); got != "＋ チーム定義を追加" {
		t.Errorf("the toolbar's add reads %q, want ＋ チーム定義を追加", got)
	}

	// N1: pressing it puts a new row first, open, with the cursor in its name,
	// プロジェクト picked, the starter's mission, a name problem, and the
	// mission's warning beside it (B1, B2).
	e2eTeamsClick(t, ctx, `document.querySelector('.toolbar [data-act=new]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.in-name')`)
	if got := e2eTeamsValue(t, ctx, `document.querySelector('#view .tdef').dataset.name.indexOf('~new')===0?'first':'no'`); got != "first" {
		t.Errorf("the new row does not stand first")
	}
	if got := e2eTeamsValue(t, ctx, `(document.activeElement===`+draft+`.querySelector('.in-name'))+''`); got != "true" {
		t.Errorf("the new row left the cursor outside its name field")
	}
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.c-place .seg button.on').textContent.trim()`); got != "プロジェクト" {
		t.Errorf("the new row's place reads %q, want プロジェクト", got)
	}
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.in-mission').value`); got != "このチームの使命を 1 行で" {
		t.Errorf("the new row's mission reads %q, want the starter's placeholder", got)
	}
	// N1: the name's problem and the figure's own sentences both stand in the
	// count — the name is the problem, and the rest warnings (B2).
	e2eTeamsWait(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.indexOf('問題 1')>=0`)
	e2eTeamsWait(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.indexOf('警告')>=0`)
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.trim()`); strings.Contains(got, "問題 2") {
		t.Errorf("the new row counts %q, want the name alone as a problem", got)
	}
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.in-name').classList.contains('bad')+''`); got != "true" {
		t.Errorf("the empty name's field is not marked")
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.tdef[data-name^="~new"] .vlist .vitem')].map(function(v){return v.querySelector('.lno').textContent+':'+v.querySelector('.msg').textContent}).join('|')`); !strings.Contains(got, "名前:チーム定義名を書いてください") {
		t.Errorf("the check column reads %q, want the name's own problem", got)
	}
	// N2: the name is a problem, so the save is shut; the foot says the row is
	// new and names the files a save would write (B2).
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('[data-act=t-save]').disabled+''`); got != "true" {
		t.Errorf("the save stands although the name is a problem")
	}
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('footer.pf .dirty').textContent.trim()`); got != "新規" {
		t.Errorf("the new row's foot reads %q, want 新規", got)
	}
	if got := e2eTeamsInt(t, ctx, draft+`.querySelectorAll('footer.pf .also code').length`); got != 3 {
		t.Errorf("the new row names %d files a save would write, want the project's three", got)
	}
	// N2: a name hergy new takes clears the problem and opens the save.
	e2eSet(t, ctx, draft+`.querySelector('.in-name')`, "ops")
	e2eTeamsWait(t, ctx, draft+`.querySelector('.in-name').classList.contains('bad')+''==='false'`)
	e2eTeamsWait(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.indexOf('問題')<0`)
	e2eTeamsWait(t, ctx, draft+`.querySelector('[data-act=t-save]').disabled+''==='false'`)
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('#view .tdef').length`); got != 7 {
		t.Errorf("the list holds %d rows with the draft open, want 7", got)
	}

	// N3: the save writes hergy new's own files, one letter apart from none. A
	// row that is not there yet is saved from either tab, and the ルール tab's
	// foot is the shape tab's own — 新規, the files its save would write, and a
	// 保存 that the name alone lets be pressed (spec-teams B2, B6, 監査 R9).
	files, err := scaffold.Team("ja")
	if err != nil {
		t.Fatal(err)
	}
	// R3: the starter profile is added to the project's profiles.yaml with one
	// edit, so every other line a person wrote — the order of the profiles, a
	// comment, the quoting of a value — comes back as it was (spec-boundary 5,
	// 監査 R3).
	profilePath := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	beforeProfile := readFile(t, profilePath)
	e2eTeamsClick(t, ctx, draft+`.querySelector('.dtabs .dtab[data-t=agree]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.panel.agree-p footer.pf')`)
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.panel.agree-p footer.pf .dirty').textContent.trim()`); got != "新規" {
		t.Errorf("the new row's rule foot reads %q, want 新規", got)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.tdef[data-name^="~new"] .panel.agree-p footer.pf .also code')].map(function(c){return c.textContent}).join(',')`); got != "team.yaml,agreement.yaml,profiles.yaml" {
		t.Errorf("the new row's rule foot names %q, want the project's three files", got)
	}
	e2eTeamsWait(t, ctx, draft+`.querySelector('.panel.agree-p [data-act=save]').disabled+''==='false'`)
	e2eTeamsClick(t, ctx, draft+`.querySelector('.panel.agree-p [data-act=save]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsRowJS("ops"))
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
	if got := e2eTeamsInt(t, ctx, `document.querySelectorAll('#view .tdef').length`); got != 7 {
		t.Errorf("the list holds %d rows after the save, want 7", got)
	}
	if got := e2eTeamsValue(t, ctx, `document.querySelector('#view .tdef').dataset.name`); got != "ops" {
		t.Errorf("the saved row stands first as %q, want ops", got)
	}
	for _, file := range files {
		got := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "ops", file.Path))
		if got != string(file.Data) {
			t.Errorf("the saved ops/%s reads\n%s\nwant hergy new's own\n%s", file.Path, got, file.Data)
		}
	}
	if got, want := readFile(t, profilePath), beforeProfile+"  claude-default:\n    kind: claude\n"; got != want {
		t.Errorf("the save rewrote the project's profiles.yaml:\n%s\nwant only the starter profile added:\n%s", got, want)
	}
	// R7: the band's number follows the save, and the row's own foot says 保存しました
	// (spec-nav 1.1, 1.4.6, 監査 R7).
	e2eTeamsWait(t, ctx, `document.querySelector('.screens .scr[data-screen=teams] .n').textContent==='7'`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("ops")+`.querySelector('footer.pf .ok')!==null`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("ops")+`.querySelector('footer.pf .ok').textContent.trim()`); got != "保存しました" {
		t.Errorf("the saved row's foot reads %q, want 保存しました", got)
	}

	// N4: the same name saved into 共通 writes the shared place, with the role
	// documents and the profile that place does not hold (B2, Y3).
	e2eTeamsClick(t, ctx, `document.querySelector('.toolbar [data-act=new]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.in-name')`)
	// The place comes first: the same name in プロジェクト is taken by N3's save
	// (B2's name rules are one place's own).
	e2eTeamsClick(t, ctx, draft+`.querySelector('.c-place .seg button[data-x=global]')`)
	e2eTeamsWait(t, ctx, draft+`.querySelector('.c-place .seg button[data-x=global]').classList.contains('on')+''==='true'`)
	e2eSet(t, ctx, draft+`.querySelector('.in-name')`, "ops")
	e2eTeamsWait(t, ctx, draft+`.querySelector('[data-act=t-save]').disabled+''==='false'`)
	// The files it would write are the shared place's: the definition's two and the
	// two role documents it lacks. The starter profile is not among them: the
	// catalog reads claude-default from the project's profiles.yaml, which N3's
	// save just put it in, and a layer's profile is resolved for both places
	// (B2, Y3).
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.tdef[data-name^="~new"] footer.pf .also code')].map(function(c){return c.textContent}).join(',')`); got != "team.yaml,agreement.yaml,lead.yaml,builder.yaml" {
		t.Errorf("the new row in 共通 names %q, want the definition's two and the two role documents", got)
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
	global := filepath.Join(f.cfg, "hergy")
	for _, file := range files {
		got := readFile(t, filepath.Join(global, "teams", "ops", file.Path))
		if got != string(file.Data) {
			t.Errorf("the shared ops/%s reads\n%s\nwant hergy new's own\n%s", file.Path, got, file.Data)
		}
	}
	roles, err := scaffold.Roles("ja")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range roles {
		got := readFile(t, filepath.Join(global, "roles", file.Path))
		if got != string(file.Data) {
			t.Errorf("the shared roles/%s reads\n%s\nwant the scaffold's own\n%s", file.Path, got, file.Data)
		}
	}
	if got := readFile(t, filepath.Join(global, "profiles.yaml")); strings.Contains(got, "claude-default") {
		t.Errorf("the shared profiles.yaml was written although the project's own resolves the starter profile:\n%s", got)
	}
	// Both places hold ops now, so its row says プロジェクト is in effect and 共通
	// stands beside it — one row, not two (B4).
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("ops")+`.querySelectorAll('.c-place .pill').length===2`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("ops")+`.querySelector('.c-place .pill.on').textContent.trim()`); got != "プロジェクト" {
		t.Errorf("the row in both places holds %q in effect, want プロジェクト", got)
	}

	// N5: core's 複製 opens core-2 right under it, its save writes core's own two
	// files one letter apart from none, and it keeps core's place in the list.
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("core")+`.classList.contains('open')+''==='true'`)
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.dacts [data-act=dup]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.in-name')`)
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.in-name').value`); got != "core-2" {
		t.Errorf("the duplicate's name reads %q, want core-2", got)
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`); got != "below" {
		t.Errorf("the duplicate does not stand right under core")
	}
	if got := e2eTeamsValue(t, ctx, `(document.activeElement===`+draft+`.querySelector('.in-name'))+''`); got != "true" {
		t.Errorf("the duplicate left the cursor outside its name field")
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsRowJS("core-2"))
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
	for _, name := range []string{"team.yaml", "agreement.yaml"} {
		got := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "core-2", name))
		want := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "core", name))
		if got != want {
			t.Errorf("the duplicate's %s reads\n%s\nwant core's own\n%s", name, got, want)
		}
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core")+`.nextElementSibling.dataset.name`); got != "core-2" {
		t.Errorf("the saved duplicate stands at %q, want core-2 right under core", got)
	}

	// N6: review (共通) copied プロジェクトに leaves one row, プロジェクト in effect
	// and 共通 struck out, and the project's folder holds review's own two files.
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("review")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("review")+`.classList.contains('open')+''==='true'`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("review")+`.querySelector('.dacts [data-act=copy]').textContent.trim()`); got != "プロジェクトにコピー" {
		t.Errorf("review's copy reads %q, want プロジェクトにコピー", got)
	}
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("review")+`.querySelector('.dacts [data-act=copy]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.c-place .pill.pending')`)
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.c-place .pill.pending').textContent.trim()`); got != "プロジェクト" {
		t.Errorf("the copy's place reads %q, want プロジェクト", got)
	}
	if got := e2eTeamsInt(t, ctx, draft+`.querySelectorAll('.in-name').length`); got != 0 {
		t.Errorf("the copy offers a name field, but its name is review's own")
	}
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("review")+`.nextElementSibling.dataset.name.indexOf('~new')===0?'below':'no'`); got != "below" {
		t.Errorf("the copy does not stand right under review")
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=t-save]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
	for _, name := range []string{"team.yaml", "agreement.yaml"} {
		got := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "review", name))
		want := readFile(t, filepath.Join(global, "teams", "review", name))
		if got != want {
			t.Errorf("the copy's %s reads\n%s\nwant review's own\n%s", name, got, want)
		}
	}
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("review")+`.querySelectorAll('.c-place .pill').length===2`)
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="review"] .c-place .pill`)+`)].map(function(p){return p.textContent.trim()+'='+p.className}).join(',')`); got != "プロジェクト=pill on,共通=pill shadow" {
		t.Errorf("review's places read %q, want プロジェクト in effect and 共通 hidden", got)
	}

	// N7: 削除 takes the project's folder away, and the row stands on 共通 alone.
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("review")+`.querySelector('.dacts [data-act=del]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsRowJS("review")+`.querySelector('[data-act=del-yes]')`)
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("review")+`.querySelector('[data-act=del-yes]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector(`+strconv.Quote(`.tdef[data-name="review"] [data-act=del-yes]`)+`)`)
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "review", "team.yaml")); !os.IsNotExist(err) {
		t.Errorf("the project's review/team.yaml still stands")
	}
	if _, err := os.Stat(filepath.Join(global, "teams", "review", "team.yaml")); err != nil {
		t.Errorf("taking the project's folder away took the shared one with it: %v", err)
	}
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("review")+`.querySelectorAll('.c-place .pill').length===1`)
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("review")+`.querySelector('.c-place .pill.on').textContent.trim()`); got != "共通" {
		t.Errorf("the row stands on %q, want 共通", got)
	}

	// N8: 取り消し takes a row that is not there yet out and writes nothing.
	before := rows()
	e2eTeamsClick(t, ctx, `document.querySelector('.toolbar [data-act=new]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.in-name')`)
	if got := rows(); got != before+1 {
		t.Errorf("the list holds %d rows with the new row open, want %d", got, before+1)
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=discard]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
	if got := rows(); got != before {
		t.Errorf("取り消し left %d rows, want %d", got, before)
	}
	entries, err := os.ReadDir(filepath.Join(f.proj, ".hergy", "teams"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if want := []string{"core", "core-2", "docs", "migrate", "ops", "research"}; !slices.Equal(names, want) {
		t.Errorf("取り消し left the project's teams as %v, want %v", names, want)
	}

	// R7: 削除 draws the band's number again: taking core-2's folder away leaves
	// one definition fewer, and the band follows the list (spec-nav 1.1, 監査
	// R7).
	before = rows()
	if got := e2eTeamsValue(t, ctx, e2eTeamsRowJS("core-2")+`.classList.contains('open')+''`); got != "true" {
		e2eTeamsClick(t, ctx, e2eTeamsRowJS("core-2")+`.querySelector('.sum')`)
		e2eTeamsWait(t, ctx, e2eTeamsRowJS("core-2")+`.classList.contains('open')+''==='true'`)
	}
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core-2")+`.querySelector('.dacts [data-act=del]')`)
	e2eTeamsWait(t, ctx, `!!`+e2eTeamsRowJS("core-2")+`.querySelector('[data-act=del-yes]')`)
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core-2")+`.querySelector('[data-act=del-yes]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector(`+strconv.Quote(`.tdef[data-name="core-2"]`)+`)`)
	if got := rows(); got != before-1 {
		t.Errorf("the delete left %d rows, want %d", got, before-1)
	}
	e2eTeamsWait(t, ctx, `document.querySelector('.screens .scr[data-screen=teams] .n').textContent===`+strconv.Quote(strconv.Itoa(before-1)))
}

// e2eTeamsTopology walks the トポロジー tab's team.yaml preview (spec-teams B7,
// B8, C10): the lines a picked point owns (Y1), a press on a turn's line (Y4),
// a turn added and one taken away with the head's own counts (Y2, Y3), the
// mission typed while the preview moves under it (Y5), the preview against the
// file a save would write (Y6), the narrow places' 編集 / YAML switch (Y7), and
// the card that leaves every point uncovered (Y8).
func e2eTeamsTopology(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	panel := e2eTeamsPanelJS("core")
	line := func(find string) string { return panel + `.querySelector(` + strconv.Quote(`.tprev .yl`+find) + `)` }

	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"), chromedp.WaitReady("#view .tdef"))
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.tprev .ybox .yl')`)
	// The preview is the text a save would write, and the head counts its lines
	// (B7).
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.tprev .yph .vt').textContent`); got != "42 行" {
		t.Errorf("the preview's head reads %q, want 42 行", got)
	}
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.tprev .yph .dirty')+''`); got != "null" {
		t.Errorf("the preview of an unchanged definition shows a change count")
	}

	// Y1: picking analyst lines up its own member lines and every turn that comes
	// in or leaves it (B7).
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=analyst]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
	e2eTeamsWait(t, ctx, panel+`.querySelectorAll('.tprev .yl.cur').length>0`)
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.tprev .yl.cur').length`); got != 7 {
		t.Errorf("analyst lines up %d preview lines, want its 3 member lines and its 4 turns", got)
	}

	// Y4: pressing a turn's line picks that line of the figure, and its card
	// names the two points (B7).
	e2eTeamsClick(t, ctx, line(`[data-a=impl-b][data-b=reviewer]`))
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=edge]')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard[data-sel=edge]').dataset.k`); got != "impl-b|reviewer" {
		t.Errorf("pressing the impl-b → reviewer line picked %q", got)
	}

	// Y3: taking that turn away marks the gap and counts one line less (B7).
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-ask][data-a=impl-b][data-b=reviewer]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, panel+`.querySelector('.tprev .yph .dirty') && `+panel+`.querySelector('.tprev .yph .dirty').textContent.trim()==='+0 −1'`)
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.tprev .yl.del,.tprev .yl.dele').length`); got != 1 {
		t.Errorf("the removed turn marks %d lines, want the gap alone", got)
	}
	// 元に戻す puts the turn back, so the preview matches the file again.
	e2eTeamsClick(t, ctx, panel+`.querySelector('footer.pf [data-act=t-revert]')`)
	e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.tprev .yph .dirty')`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("core")+`.classList.contains('unsaved')+''==='false'`)

	// Y6: the preview is the saved team.yaml line for line, comments and all.
	text := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml"))
	want := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	got := strings.Split(e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .tprev .yl .yt`)+`)].map(function(t){return t.textContent}).join('\u0001')`), "\u0001")
	if len(got) != len(want) {
		t.Fatalf("the preview holds %d lines, want the file's %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] && !(got[i] == " " && strings.TrimSpace(want[i]) == "") {
			t.Errorf("the preview's line %d reads %q, want %q", i+1, got[i], want[i])
		}
	}

	// Y2: a turn added from analyst to reviewer stands at the end of the preview
	// as an added line, the head counts it, and the box was sent to it (B7).
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=analyst]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`)
	e2eSelect(t, ctx, panel+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`, "reviewer")
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=edge]')`)
	e2eTeamsWait(t, ctx, line(`.ins`)+` && `+line(`.ins`)+`.textContent.indexOf('analyst -> reviewer')>=0`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.tprev .yph .dirty').textContent.trim()`); got != "+1 −0" {
		t.Errorf("the head reads %q after the turn was added, want +1 −0", got)
	}
	if got := e2eTeamsValue(t, ctx, `(function(){var b=`+panel+`.querySelector('.tprev .ybox'),l=`+line(`.ins`)+`;if(!b||!l)return false;var br=b.getBoundingClientRect(),lr=l.getBoundingClientRect();return lr.top>=br.top-1&&lr.bottom<=br.bottom+1;})()+''`); got != "true" {
		t.Errorf("the box was not sent to the line the turn added")
	}
	// And picking that line picks the new turn of the figure (B7).
	e2eTeamsClick(t, ctx, line(`[data-a=analyst][data-b=reviewer]`))
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=edge]')`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard[data-sel=edge]').dataset.k`); got != "analyst|reviewer" {
		t.Errorf("pressing the added line picked %q", got)
	}
	if got := e2eTeamsInt(t, ctx, panel+`.querySelectorAll('.tprev .yl.cur').length`); got != 1 {
		t.Errorf("the added turn's line takes %d preview lines, want the one line it wrote", got)
	}

	// Y5: the mission's line follows the typing, and the cursor stays in the
	// field (B7, Y5).
	const mission = "E2E がプレビューで打った使命"
	field := `document.querySelector(` + strconv.Quote(`.tdef[data-name="core"] .in-mission`) + `)`
	e2eRun(t, ctx, chromedp.Focus(`.tdef[data-name="core"] .in-mission`))
	e2eTeamsWait(t, ctx, `(document.activeElement===`+field+`)+''==='true'`)
	e2eSet(t, ctx, field, mission)
	e2eTeamsWait(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .tprev .yl`)+`)].some(function(l){return l.textContent.indexOf(`+strconv.Quote(mission)+`)>=0})`)
	if got := e2eTeamsValue(t, ctx, `(document.activeElement===`+field+`)+''`); got != "true" {
		t.Errorf("typing the mission moved the cursor out of the field")
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .tprev .yl.ins`)+`)].some(function(l){return l.textContent.indexOf(`+strconv.Quote(mission)+`)>=0})+''`); got != "true" {
		t.Errorf("the mission's typed line does not stand in the preview as an added line")
	}
	e2eTeamsClick(t, ctx, panel+`.querySelector('footer.pf [data-act=t-revert]')`)
	e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.tprev .yph .dirty')`)

	// Y7: at 1100 or less the 編集 / YAML switch picks which column shows, the
	// box goes to the last changed line, and pressing a line comes back to 編集
	// with that point picked (B7).
	e2eRun(t, ctx, chromedp.EmulateViewport(1100, 900))
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('[data-act=tmode][data-v=yaml]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gn.member[data-id=impl-a]')`)
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard[data-sel=node]')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('[data-act=tmode][data-v=yaml]')`)
	e2eTeamsWait(t, ctx, panel+`.closest('.panel').dataset.mode==='yaml'`)
	if got := e2eTeamsValue(t, ctx, `getComputedStyle(`+panel+`.querySelector('.pedit')).display`); got != "none" {
		t.Errorf("1100: the YAML switch left the figure showing")
	}
	e2eTeamsWait(t, ctx, panel+`.querySelector('.tprev .yl.cur')!==null`)
	e2eTeamsClick(t, ctx, line(`[data-node=reviewer]`))
	e2eTeamsWait(t, ctx, panel+`.closest('.panel').dataset.mode==='edit'`)
	if got := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard[data-sel=node]').dataset.id`); got != "reviewer" {
		t.Errorf("pressing the reviewer line in the YAML column picked %q", got)
	}
	e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard .x')`)
	e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.gcard')`)
	e2eTeamsClick(t, ctx, panel+`.querySelector('footer.pf [data-act=t-revert]')`)
	e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.tprev .yph .dirty')`)

	// Y8: at 1440 every point's card leaves every point uncovered, or stands
	// below the figure (B8).
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gwrap svg')`)
	cover := `(function(){var p=` + panel + `,c=p.querySelector('.gcard');if(!c)return -1;var cr=c.getBoundingClientRect(),n=0;p.querySelectorAll('.gn .box').forEach(function(b){var r=b.getBoundingClientRect();if(Math.min(cr.right,r.right)-Math.max(cr.left,r.left)>0&&Math.min(cr.bottom,r.bottom)-Math.max(cr.top,r.top)>0)n++;});return n;})()`
	points := e2eTeamsValue(t, ctx, `[...document.querySelectorAll(`+strconv.Quote(`.tdef[data-name="core"] .panel.team-p .gn`)+`)].map(function(g){return g.classList.contains('worker')?'w'+g.dataset.i:'n'+g.dataset.id}).join(' ')`)
	for _, p := range strings.Fields(points) {
		pick := panel + `.querySelector('.gn[data-id="` + p[1:] + `"]')`
		if p[0] == 'w' {
			pick = panel + `.querySelector('.gn.worker[data-i="` + p[1:] + `"]')`
		}
		e2eTeamsClick(t, ctx, pick)
		e2eTeamsWait(t, ctx, `!!`+panel+`.querySelector('.gcard')`)
		if got := e2eTeamsInt(t, ctx, cover); got != 0 {
			if below := e2eTeamsValue(t, ctx, panel+`.querySelector('.gcard').classList.contains('below')+''`); below != "true" {
				t.Errorf("1440: the card of %s covers %d points and does not stand below the figure", p, got)
			}
		}
		e2eTeamsClick(t, ctx, panel+`.querySelector('.gcard .x')`)
		e2eTeamsWait(t, ctx, `!`+panel+`.querySelector('.gcard')`)
	}
}

// e2eTeamsAudit walks what the version-7 audit sent back that the other walks do
// not read (監査 R1〜R8): the page a URL opens stays where it stands and lights no
// line (R1), the YAML box stops above the save's band in both layouts and the
// line a change moved is brought above that band (R2), a new row's name problems
// stand under its name field (R4), both panels' heads follow the name being
// typed (R5), a name a saved definition already holds reads the draft's own
// agreement (R6), and a duplicate's name field holds the whole name chosen (R8).
// R3 and R7 stand in e2eTeamsAddCopyDelete, where a save happens.
func e2eTeamsAudit(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	// tline is true when one line stands inside its box and above the save's band.
	// The band's own top edge is what the audit measured, so that is what the
	// line is held to.
	tline := func(p, find string) string {
		return `(function(){var p=` + p + `,l=p.querySelector(` + strconv.Quote(find) + `),b=p.querySelector('.tprev .ybox'),f=p.querySelector('footer.pf');` +
			`if(!l||!b||!f)return false;var lr=l.getBoundingClientRect(),br=b.getBoundingClientRect(),fr=f.getBoundingClientRect();` +
			`return lr.top>=br.top-1&&lr.bottom<=br.bottom+1&&lr.bottom<=fr.top+1;})()+''==='true'`
	}
	boxAbove := func(p string) string {
		return `(function(){var p=` + p + `,b=p.querySelector('.tprev'),f=p.querySelector('footer.pf');` +
			`if(!b||!f)return false;return b.getBoundingClientRect().bottom<=f.getBoundingClientRect().top+1;})()+''==='true'`
	}

	// R1: a URL that opens a row brings the page nowhere and lights no line. The
	// first answer for a row is not a move, and the box moves its own scroll and
	// nothing else (追補 B7, 監査 R1).
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	for _, name := range []string{"core", "research"} {
		e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open="+name))
		p := e2eTeamsPanelJS(name)
		e2eTeamsWait(t, ctx, p+`!==null&&`+p+`.querySelectorAll('.tprev .ybox .yl').length>0`)
		if got := e2eTeamsInt(t, ctx, `window.scrollY`); got != 0 {
			t.Errorf("opening %s by URL sent the page %dpx down", name, got)
		}
		if got := e2eTeamsInt(t, ctx, p+`.querySelector('.tprev .ybox').scrollTop`); got != 0 {
			t.Errorf("opening %s by URL sent its YAML box %dpx down", name, got)
		}
		if got := e2eTeamsInt(t, ctx, p+`.querySelectorAll('.tprev .yl.step').length`); got != 0 {
			t.Errorf("opening %s by URL lit %d lines as the move", name, got)
		}
	}

	// R2: the box stops above the save's band, and the line a turn added, and the
	// line a turn taken away left, are brought above that band (追補 B7, 監査 R2).
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"))
	p := e2eTeamsPanelJS("core")
	e2eTeamsWait(t, ctx, p+`!==null&&!!`+p+`.querySelector('.tprev .ybox .yl')`)
	e2eTeamsWait(t, ctx, boxAbove(p))
	e2eTeamsClick(t, ctx, p+`.querySelector('.gn.member[data-id=analyst]')`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`)
	e2eSelect(t, ctx, p+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`, "reviewer")
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.tprev .yl.ins')`)
	e2eTeamsWait(t, ctx, tline(p, ".tprev .yl.ins"))
	e2eTeamsClick(t, ctx, p+`.querySelector('.tprev .yl[data-a=analyst][data-b=impl-a]')`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.gcard [data-act=turn-ask]')`)
	e2eTeamsClick(t, ctx, p+`.querySelector('.gcard [data-act=turn-ask]')`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsClick(t, ctx, p+`.querySelector('.gcard [data-act=turn-del]')`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.tprev .yl.del')`)
	e2eTeamsWait(t, ctx, tline(p, ".tprev .yl.del"))

	// R2 at one column: the row's YAML mode shows the box over the whole width,
	// and the line the added turn is above the band there too (Y7, 監査 R2).
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams?open=core"))
	e2eRun(t, ctx, chromedp.EmulateViewport(1100, 900))
	e2eTeamsWait(t, ctx, p+`!==null&&!!`+p+`.querySelector('.gn.member[data-id=analyst]')`)
	e2eTeamsClick(t, ctx, p+`.querySelector('.gn.member[data-id=analyst]')`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`)
	e2eSelect(t, ctx, p+`.querySelector('.gcard select[data-act-change=turn-add-sel]')`, "reviewer")
	e2eTeamsWait(t, ctx, p+`.querySelector('.tprev .yl.ins')!==null`)
	e2eTeamsClick(t, ctx, p+`.querySelector('.pview [data-act=tmode][data-v=yaml]')`)
	e2eTeamsWait(t, ctx, p+`.dataset.mode==='yaml'`)
	e2eTeamsWait(t, ctx, `!!`+p+`.querySelector('.tprev .ybox .yl')`)
	e2eTeamsWait(t, ctx, boxAbove(p))
	e2eTeamsWait(t, ctx, tline(p, ".tprev .yl.ins"))

	// R4〜R6, R8: the list's own new row.
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eRun(t, ctx, chromedp.Navigate(base+"/teams"), chromedp.WaitReady("#view .tdef"))
	e2eTeamsWait(t, ctx, `document.querySelectorAll('#view .tdef').length===6`)
	draft := `document.querySelector('.tdef[data-name^="~new"]')`
	mvm := `[...document.querySelectorAll('.tdef[data-name^="~new"] .mvm li')].map(function(l){return l.lastElementChild.textContent}).join('|')`
	e2eTeamsClick(t, ctx, `document.querySelector('.toolbar [data-act=new]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.in-name')`)

	// R4: the name's own problem stands under the name field, first in the same
	// list as the mission's warning (追補 B2, 監査 R4).
	e2eTeamsWait(t, ctx, `document.querySelectorAll('.tdef[data-name^="~new"] .mvm li').length===2`)
	if got := e2eTeamsValue(t, ctx, mvm); !strings.HasPrefix(got, "チーム定義名を書いてください") {
		t.Errorf("the name field's own list reads %q, want the name's problem first", got)
	}
	fresh := e2eTeamsValue(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.trim()`)

	// R5: both panels' heads follow the name a person is typing (追補 B2, 監査 R5).
	e2eSet(t, ctx, draft+`.querySelector('.in-name')`, "ops2")
	e2eTeamsWait(t, ctx, draft+`.querySelector('.panel.team-p .ph .path').textContent.indexOf('teams/ops2/team.yaml')>=0`)
	e2eTeamsClick(t, ctx, draft+`.querySelector('.dtabs .dtab[data-t=agree]')`)
	e2eTeamsWait(t, ctx, `!!`+draft+`.querySelector('.panel.agree-p .ph .path')`)
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.panel.agree-p .ph .path').textContent`); !strings.HasSuffix(got, "teams/ops2/agreement.yaml") {
		t.Errorf("the rules panel's head reads %q, want the typed name's agreement.yaml", got)
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('.dtabs .dtab[data-t=shape]')`)

	// R4, R6: a name a place already holds is the problem it says, and that
	// definition's own rules do not come with the name (追補 B2, 監査 R4, R6).
	e2eSet(t, ctx, draft+`.querySelector('.in-name')`, "core")
	e2eTeamsWait(t, ctx, mvm+`.indexOf('すでにあります')>=0`)
	if got := e2eTeamsValue(t, ctx, mvm); !strings.HasPrefix(got, "チーム定義") {
		t.Errorf("a taken name's list reads %q, want the name's problem first", got)
	}
	if got := e2eTeamsValue(t, ctx, draft+`.querySelector('.c-name .vcount').textContent.trim()`); got != fresh {
		t.Errorf("a row named core counts %q, want the %q a fresh draft counts: its own rules are not read", got, fresh)
	}
	if got := e2eTeamsValue(t, ctx, `[...document.querySelectorAll('.tdef[data-name^="~new"] .vlist .vitem .msg')].map(function(m){return m.textContent}).join('|')`); strings.Contains(got, "CORE-") {
		t.Errorf("a row named after core reads core's own rules: %q", got)
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=discard]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)

	// R8: a duplicate's name field holds the whole name chosen, so the first
	// letter a person types replaces it (追補 B3, 監査 R8).
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.sum')`)
	e2eTeamsWait(t, ctx, e2eTeamsRowJS("core")+`.classList.contains('open')+''==='true'`)
	e2eTeamsClick(t, ctx, e2eTeamsRowJS("core")+`.querySelector('.dacts [data-act=dup]')`)
	e2eTeamsWait(t, ctx, draft+`.querySelector('.in-name')&&`+draft+`.querySelector('.in-name').value==='core-2'`)
	if got := e2eTeamsValue(t, ctx, `(function(){var el=`+draft+`.querySelector('.in-name');return el.selectionStart+'|'+el.selectionEnd+'|'+el.value.length;})()`); got != "0|6|6" {
		t.Errorf("the duplicate's name field is chosen as %q, want the whole core-2 (0|6|6)", got)
	}
	if got := e2eTeamsValue(t, ctx, `(document.activeElement===`+draft+`.querySelector('.in-name'))+''`); got != "true" {
		t.Errorf("the duplicate left the cursor outside its name field")
	}
	e2eTeamsClick(t, ctx, draft+`.querySelector('[data-act=discard]')`)
	e2eTeamsWait(t, ctx, `!document.querySelector('.tdef[data-name^="~new"]')`)
}
