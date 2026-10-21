//go:build e2e

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestE2EProfiles walks the profile screen over the real-scale fixture (r-c683):
// the three tabs, the row that opens in place, the checks while a field is
// typed, a save of each kind, and the three widths in both lights. It is the
// same fixture the design was drawn on (testdata/profiles-v2), so the numbers
// (11 profiles, 4 definitions, 15 members, 3 workers) are the ones the mock
// shows.
func TestE2EProfiles(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	initLocales(t)
	copyProfilesFixture(t, f, cwd)
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

	console := e2eListen(ctx, false)

	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	t.Run("PROFILES-S1 structure", func(t *testing.T) { e2eProfilesStructure(t, ctx, base) })
	t.Run("PROFILES-P1 rows open in place", func(t *testing.T) { e2eProfilesOpen(t, ctx, base) })
	t.Run("PROFILES-P2 filter and search", func(t *testing.T) { e2eProfilesFilter(t, ctx, base) })
	t.Run("PROFILES-P3 edit and save", func(t *testing.T) { e2eProfilesSave(t, ctx, base, f) })
	t.Run("PROFILES-P8 duplicate copy delete", func(t *testing.T) { e2eProfilesCopy(t, ctx, base, f) })
	t.Run("PROFILES-P12 assign", func(t *testing.T) { e2eProfilesAssign(t, ctx, base, f) })
	t.Run("PROFILES-P11 usage jumps", func(t *testing.T) { e2eProfilesUsage(t, ctx, base) })
	t.Run("PROFILES-T3 yaml", func(t *testing.T) { e2eProfilesYAML(t, ctx, base, f) })
	t.Run("PROFILES-W1 widths and lights", func(t *testing.T) { e2eProfilesWidths(t, ctx, base) })
	t.Run("delete a used profile", func(t *testing.T) { e2eProfilesDeleteUsed(t, ctx, base) })

	if lines := console.Lines(); len(lines) > 0 {
		t.Errorf("the profile screen logged %d console errors: %v", len(lines), lines)
	}
}

// copyProfilesFixture puts the real-scale fixture under the test's project and
// outside settings directory, the way a person's project stands.
func copyProfilesFixture(t *testing.T, f *fixture, cwd string) {
	t.Helper()
	src := filepath.Join(cwd, "..", "..", "testdata", "profiles-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(f.cfg, "hergy"))
}

// copyDir copies a directory tree.
func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}

// e2eProfilesValue reads one expression as the text the page draws. The reading
// waits until the page can give it, the way every act does (e2eRead, r-3c41).
func e2eProfilesValue(t *testing.T, ctx context.Context, script string) string {
	t.Helper()
	return e2eRead[string](t, ctx, script)
}

// e2eProfilesInt reads one expression as the number the page draws.
func e2eProfilesInt(t *testing.T, ctx context.Context, script string) int {
	t.Helper()
	return e2eRead[int](t, ctx, script)
}

// e2eProfilesBool reads one expression as the truth the page draws.
func e2eProfilesBool(t *testing.T, ctx context.Context, script string) bool {
	t.Helper()
	return e2eRead[bool](t, ctx, script)
}

// e2eProfilesWait waits until one expression is true.
func e2eProfilesWait(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	e2eWait(t, ctx, script)
}

// e2eProfilesClick presses one element the way a person presses it: by the
// selector the test names it with. It waits for the screen to offer it, so a
// button the screen has shut takes no press — the assign tab writes its save
// into the card from the check's answer, which lands after the pick was made
// (r-3c41).
func e2eProfilesClick(t *testing.T, ctx context.Context, selector string) {
	t.Helper()
	e2ePressWhenEnabled(t, ctx, `document.querySelector(`+strconv.Quote(selector)+`)`)
}

// e2eProfilesSet writes one field's value and fires the input event a person's
// typing fires.
func e2eProfilesSet(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eSet(t, ctx, find, value)
}

// e2eProfilesSelect picks one option of a select and fires the change event a
// person's picking fires.
func e2eProfilesSelect(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eSelect(t, ctx, find, value)
}

// e2eProfilesFile waits until one file holds one line.
func e2eProfilesFile(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			got = string(raw)
			if strings.Contains(got, want) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never held %q:\n%s", path, want, got)
}

// e2eProfilesStructure is S1・S2・S5・S6: the three tabs, the list's columns and
// eleven rows, the four cards with their people, and the two YAML sections.
func e2eProfilesStructure(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	e2eProfilesWait(t, ctx, `document.fonts.check('14px "Zen Kaku Gothic New"') && document.fonts.check('13px "IBM Plex Mono"')`)
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#tabs .tab').length`); got != 3 {
		t.Errorf("the page has %d tabs, want 3", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row').length`); got != 11 {
		t.Errorf("the list has %d rows, want 11", got)
	}
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('.head > span')].map(e=>e.textContent.trim()).filter(Boolean).join(',')`); got != "名前,kind,置き場所,引数,環境変数,使っている人" {
		t.Errorf("the head reads %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row').dataset.name`); got != "sonnet-fast" {
		t.Errorf("the first row is %q, want the project's first profile", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="codex"]').querySelector('.c-name .sev').textContent.trim()`); got != "警告 1" {
		t.Errorf("the shared codex row says %q, want 警告 1", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .pill').length`); got != 2 {
		t.Errorf("sonnet-fast has %d place pills, want 2", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .pill.shadow').length`); got != 1 {
		t.Errorf("the shared sonnet-fast is not drawn as hidden")
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.team').length`); got != 4 {
		t.Errorf("the assign tab has %d cards, want 4", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow').length`); got != 18 {
		t.Errorf("the assign tab has %d people, want 18", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.team[data-g=docs] .arow.missing').length`); got != 1 {
		t.Errorf("the docs worker with a missing profile is not marked")
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=docs] .arow.missing .vm').textContent`); !strings.Contains(got, "workers[0].profile: gemini-flash というプロファイルはありません") {
		t.Errorf("the docs worker says %q", got)
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.yed').length`); got != 2 {
		t.Errorf("the YAML tab has %d sections, want 2", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=project] .ln').textContent`); got != "プロジェクト" {
		t.Errorf("the first YAML section is %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=global] .vhead').textContent`); !strings.Contains(got, "警告 1") {
		t.Errorf("the shared YAML does not carry the one warning: %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=global] .gutter .warn').textContent`); got != "9" {
		t.Errorf("the warning is on line %q, want 9", got)
	}
}

// e2eProfilesOpen is P1・S3・S4: the row opens under itself, the detail holds the
// people then the two panels, and every panel is a head, name, kind, args, env
// and foot.
func e2eProfilesOpen(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="sonnet-fast"] .sum`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="sonnet-fast"] .detail').hidden`)
	if got := e2eProfilesValue(t, ctx, `window.location.search`); !strings.Contains(got, "open=sonnet-fast") {
		t.Errorf("opening a row did not write the URL: %q", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .detail .panel').length`); got != 2 {
		t.Errorf("sonnet-fast shows %d panels, want the project and the shared one", got)
	}
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .detail .panel .ln')].map(e=>e.textContent).join(',')`); got != "プロジェクト,共通" {
		t.Errorf("the panels read %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .panel .state')].map(e=>e.textContent).join(',')`); got != "効いている,隠れている" {
		t.Errorf("the states read %q", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name="sonnet-fast"] .panel .field').length`); got != 8 {
		t.Errorf("the two panels hold %d fields, want 4 each", got)
	}
	for _, name := range []string{"project", "global"} {
		script := fmt.Sprintf(`document.querySelector('#rows .row[data-name="sonnet-fast"] .panel[data-file=%s] .path').textContent`, name)
		if got := e2eProfilesValue(t, ctx, script); !strings.HasSuffix(got, "profiles.yaml") {
			t.Errorf("the %s panel's head reads %q, want the file's path", name, got)
		}
	}
	if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.querySelector('.uchip')).color`); got != e2eProfilesValue(t, ctx, `getComputedStyle(document.body).color`) {
		t.Errorf("the name on a person's chip is %q, want the body's ink", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.uchip .g').length`); got == 0 {
		t.Errorf("a member's role on a chip is not the faint mono word")
	}
	visible := `(function(){var d=document.querySelector('#rows .row[data-name="sonnet-fast"] .detail');if(!d){return;}var r=d.getBoundingClientRect();return r.top < window.innerHeight && r.bottom > 0;})()`
	if !e2eProfilesBool(t, ctx, visible) {
		t.Errorf("the opened detail is not in the view")
	}

	// P1 again: the same row closes.
	e2eProfilesClick(t, ctx, `#rows .row[data-name="sonnet-fast"] .sum`)
	e2eProfilesWait(t, ctx, `document.querySelector('#rows .row[data-name="sonnet-fast"] .detail').hidden`)
	if got := e2eProfilesValue(t, ctx, `window.location.search`); strings.Contains(got, "open=") {
		t.Errorf("closing the row left %q in the URL", got)
	}
	// R1: a name both files hold still counts the readers of the panel's own file.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=sonnet-fast"), chromedp.WaitReady("#rows .row[data-name=sonnet-fast]"))
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project] [data-f=name]')`, "sonnet-5.5")
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project] .rename').hidden`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project] .rename span').textContent`); !strings.Contains(got, "使っている 3 人の指定も sonnet-5.5 に書き換える") {
		t.Errorf("the rename hint reads %q", got)
	}
	if !e2eProfilesBool(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project] .rename input').checked`) {
		t.Errorf("the rename box is not ticked at first")
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))

}

// e2eProfilesFilter is P2: the chips and the search change the list right under
// them.
func e2eProfilesFilter(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	e2eProfilesClick(t, ctx, `.fchip[data-k="codex"]`)
	visible := `[...document.querySelectorAll('#rows .row')].filter(r=>!r.hidden).length`
	if got := e2eProfilesInt(t, ctx, visible); got != 3 {
		t.Errorf("the codex filter shows %d rows, want 3", got)
	}
	e2eProfilesClick(t, ctx, `.fchip[data-k="codex"]`)
	if got := e2eProfilesInt(t, ctx, visible); got != 11 {
		t.Errorf("pressing the chip again shows %d rows, want 11", got)
	}
	e2eProfilesSet(t, ctx, `document.querySelector('#q')`, "opus")
	e2eProfilesWait(t, ctx, `[...document.querySelectorAll('#rows .row')].filter(r=>!r.hidden).length===2`)
	e2eProfilesSet(t, ctx, `document.querySelector('#q')`, "")
	if got := e2eProfilesInt(t, ctx, visible); got != 11 {
		t.Errorf("clearing the search shows %d rows, want 11", got)
	}
}

// e2eProfilesSave is P3・P5・P7: a new profile is added, a kind is changed in
// place, and the save says so beside the button.
func e2eProfilesSave(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	e2eProfilesClick(t, ctx, `[data-act="new"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=""] [data-f=name]')`)
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name=""] [data-f=name]')`, "fresh-one")
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] [data-act=save]')`)
	e2eProfilesWait(t, ctx, `[...document.querySelectorAll('#rows .row')].some(r=>r.dataset.name==='fresh-one' && !r.querySelector('.tag'))`)
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml")); !strings.Contains(got, "  fresh-one:\n    kind: claude\n") {
		t.Errorf("the new profile is not in the file:\n%s", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] .ok').textContent`); got != "保存しました" {
		t.Errorf("the saved line reads %q", got)
	}

	// P5: the kind button changes the args' head beside it.
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-one"] .panel[data-file=project] [data-act="set-kind"][data-k="gemini"]`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] .panel[data-file=project] .bin').textContent`); got != "gemini" {
		t.Errorf("the args' head reads %q after the kind change", got)
	}
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] .panel[data-file=project] [data-act=save]')`)
	e2eProfilesFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "  fresh-one:\n    kind: gemini\n")

	// R5: a rename moves the name in the URL's open= too.
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] .panel[data-file=project] [data-f=name]')`, "fresh-two")
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name="fresh-one"] .panel[data-file=project] [data-act=save]')`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="fresh-two"]')`)
	if got := e2eProfilesValue(t, ctx, `window.location.search`); !strings.Contains(got, "open=fresh-two") || strings.Contains(got, "fresh-one") {
		t.Errorf("the URL after the rename reads %q", got)
	}
}

// e2eProfilesCopy is P8・P9・P10: a profile is duplicated, copied to the other
// file as a panel not yet written, and taken away with a second press.
func e2eProfilesCopy(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=fresh-two"), chromedp.WaitReady("#rows .row[data-name=fresh-two]"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two"] .panel[data-file=project] [data-act="dup"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="fresh-two-2"]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="fresh-two-2"] [data-f=name]').value`); got != "fresh-two-2" {
		t.Errorf("the duplicate is named %q", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two-2"] [data-act="discard"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="fresh-two-2"]')`)

	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=codex-high"), chromedp.WaitReady("#rows .row[data-name=codex-high]"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="codex-high"] .panel[data-file=project] [data-act="copy"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name="codex-high"] .panel').length===2`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="codex-high"] .pill.pending').textContent`); got != "共通" {
		t.Errorf("the copied place is %q, want 共通", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name="codex-high"] .panel[data-file=global] [data-act="discard"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name="codex-high"] .panel').length===1`)

	// P10: the first press asks, the second writes; やめる keeps the row.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=fresh-two"), chromedp.WaitReady("#rows .row[data-name=fresh-two]"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two"] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="fresh-two"] [data-act="del-yes"]')`)
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two"] [data-act="del-no"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="fresh-two"] [data-act="del-yes"]')`)
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two"] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="fresh-two"] [data-act="del-yes"]')`)
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fresh-two"] [data-act="del-yes"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="fresh-two"]')`)
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml")); strings.Contains(got, "fresh-two") {
		t.Errorf("the deleted profile is still in the file:\n%s", got)
	}

	// R4: a copy into the project puts the project's panel first.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=opus-plan"), chromedp.WaitReady("#rows .row[data-name=opus-plan]"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="opus-plan"] .panel[data-file=global] [data-act="copy"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name="opus-plan"] .panel').length===2`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="opus-plan"] .panel').dataset.file`); got != "project" {
		t.Errorf("after a copy the left panel is %q, want the project's", got)
	}

}

// e2eProfilesAssign is P12・P13・V2: one row is picked, the card's head and the
// kind bar follow, and the save writes the one line.
func e2eProfilesAssign(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=docs] .tside').textContent`); !strings.Contains(got, "問題 1") {
		t.Errorf("the docs card does not carry 問題 1: %q", got)
	}
	if e2eProfilesBool(t, ctx, `!!document.querySelector('.team[data-g=docs] [data-act=g-save]')`) {
		t.Errorf("a card with a problem shows a save button")
	}

	path := filepath.Join(f.proj, ".hergy", "teams", "migrate", "team.yaml")
	before := readFile(t, path)
	e2eProfilesSelect(t, ctx, `document.querySelector('.team[data-g=migrate] .arow[data-k="m-1"] select')`, "sonnet-fast")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=migrate] [data-act="g-save"]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=migrate] .tside .dirty').textContent`); got != "1 人" {
		t.Errorf("the card's head says %q, want 1 人", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=migrate] .arow[data-k="m-1"] s.was').textContent`); got != "codex-high" {
		t.Errorf("the old name is %q", got)
	}
	e2eProfilesClick(t, ctx, `.team[data-g=migrate] [data-act="g-save"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=migrate] .ok')`)
	after := readFile(t, path)
	if !strings.Contains(after, "  - name: impl\n    role: builder\n    profile: sonnet-fast\n") {
		t.Errorf("the pick was not written:\n%s", after)
	}
	// The pick travels as the team kind's own op, so it writes that line and no
	// other (spec-teams 1.3, r-8e85).
	if changed := e2eRolesChangedLines(before, after); len(changed) != 1 {
		t.Errorf("the assign save changed %d lines, want the one it picked: %v", len(changed), changed)
	}

	// P13・R6: a kind dims the rest, and the address keeps the choice.
	e2eProfilesClick(t, ctx, `.kseg[data-k="codex"]`)
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow.dim').length`); got == 0 {
		t.Errorf("picking a kind dims no row")
	}
	if got := e2eProfilesValue(t, ctx, `window.location.search`); got != "?kind=codex" {
		t.Errorf("picking a kind left the address as %q", got)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign?kind=codex"), chromedp.WaitReady(".team"))
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow.dim').length`); got == 0 {
		t.Errorf("the reloaded address does not keep the kind thinned")
	}
	e2eProfilesClick(t, ctx, `.kseg[data-k="codex"]`)
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow.dim').length`); got != 0 {
		t.Errorf("pressing the kind again left %d rows dim", got)
	}
}

// e2eProfilesDeleteUsed is J1: a profile a team names goes after the second
// press, and the assign tab then shows the people left naming nothing.
func e2eProfilesDeleteUsed(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=fable-review"), chromedp.WaitReady("#rows .row[data-name=fable-review]"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fable-review"] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="fable-review"] [data-act="del-yes"]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name="fable-review"] [data-act="del-yes"]').textContent`); got != "削除する（1 人が使っている）" {
		t.Errorf("the confirm reads %q", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name="fable-review"] [data-act="del-yes"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="fable-review"]')`)

	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=core] .arow.missing .vm').textContent`); !strings.Contains(got, "members[4].profile: fable-review というプロファイルはありません") {
		t.Errorf("the assign tab does not show the person left without a profile: %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=core] .tside').textContent`); !strings.Contains(got, "問題 1") {
		t.Errorf("the core card does not say 問題 1: %q", got)
	}
}

// e2eProfilesUsage is P11: a person's chip moves to the assign tab, centers the
// person's row and lights it.
func e2eProfilesUsage(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=opus-plan"), chromedp.WaitReady(".uchip"))
	e2eProfilesClick(t, ctx, `#rows .row[data-name="opus-plan"] .uchip`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=core] .arow[data-k="m-0"].flash')`)
	if got := e2eProfilesValue(t, ctx, `document.activeElement.tagName`); got != "SELECT" {
		t.Errorf("the person's row does not have the focus (%q)", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.activeElement.closest('.arow').dataset.k`); got != "m-0" {
		t.Errorf("the focus is on %q, want m-0", got)
	}
}

// e2eProfilesYAML is T3・V4: the editor shows the text and its problems on their
// lines, and a broken text stops the save.
func e2eProfilesYAML(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=project] .ytext').value`); !strings.Contains(got, "opencode-deepseek:") {
		t.Errorf("the project's YAML does not show its profiles")
	}
	e2eProfilesSet(t, ctx, `document.querySelector('.yed[data-file=project] .ytext')`, "version: 1\nprofiles:\n  broken:\n    kind: \"\"\n")
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed[data-file=project] .vitem').length > 0`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=project] .vitem .msg').textContent`); !strings.Contains(got, "エージェントの種類") {
		t.Errorf("the broken kind says %q", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.yed[data-file=project] .gutter .err').length`); got == 0 {
		t.Errorf("the broken line is not marked in the gutter")
	}
	if !e2eProfilesBool(t, ctx, `document.querySelector('.yed[data-file=project] [data-act=yaml-save]').disabled`) {
		t.Errorf("a broken text can be saved")
	}
	e2eProfilesClick(t, ctx, `.yed[data-file=project] [data-act="yaml-revert"]`)
	e2eProfilesWait(t, ctx, `document.querySelector('.yed[data-file=project] .ytext').value.indexOf('version: 1')===0 && document.querySelectorAll('.yed[data-file=project] .vitem').length===0`)

	// T2・V4: a good change is written as typed, through the shared YAML part.
	// The save carries the name of the file the section drew as well as the
	// text, so the kind writes the file the screen handed it (r-8e85).
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	good := strings.Replace(before, "version: 1", "version: 1 # e2e", 1)
	if good == before {
		t.Fatalf("the fixture's profiles.yaml no longer starts with the line the test marks")
	}
	e2eProfilesSet(t, ctx, `document.querySelector('.yed[data-file=project] .ytext')`, good)
	e2ePressWhenEnabled(t, ctx, `document.querySelector('.yed[data-file=project] [data-act=yaml-save]')`)
	e2eProfilesFile(t, path, "# e2e")
	if got := readFile(t, path); got != good {
		t.Errorf("the YAML save wrote %q, want the typed text", got)
	}
	// Put the fixture back: the tests behind this one open the profiles this
	// file names.
	writeFile(t, path, before)
}

// e2eProfilesWidths is W1〜W4 and D1: the three widths lay out as the spec says,
// the page never flows sideways, and both lights are readable.
func e2eProfilesWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, size := range []int64{2000, 1440, 500} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=sonnet-fast"), chromedp.WaitReady("#rows .row"))
			e2eTheme(t, ctx, theme)
			e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=sonnet-fast] .detail:not([hidden]) .panel').length===2`)
			if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
				t.Errorf("%d/%s: the page flows %dpx sideways", size, theme, over)
			}
			if size <= 1180 {
				if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.querySelector('.c-env')).display`); got != "none" {
					t.Errorf("%d: the env column is still shown", size)
				}
			}
			if size <= 760 {
				if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.querySelector('.head')).display`); got != "none" {
					t.Errorf("%d: the head row is still shown", size)
				}
			}
			cols := e2eProfilesValue(t, ctx, `getComputedStyle(document.querySelector('#rows .row[data-name=sonnet-fast] .panels')).gridTemplateColumns.split(' ').length+''`)
			if size == 500 && cols != "1" {
				t.Errorf("%d: the panels stand in %s columns, want 1", size, cols)
			}
		}
	}

	// D1: the OS asks for dark, and the text still reads against its ground.
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	e2eDark(t, ctx)
	if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.body).backgroundColor`); got != "rgb(15, 18, 24)" {
		t.Errorf("the OS dark ground is %q", got)
	}

	// W3・W4 at 500: the long JSON and the eight args all show, and the page
	// itself does not flow.
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 900))
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=opencode-DeepSeek-Flash-V4.1"), chromedp.WaitReady("#rows .row"))
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=opencode-deepseek"), chromedp.WaitReady("#rows .row"))
	if got := e2eProfilesInt(t, ctx, `document.querySelector('.row[data-name=opencode-deepseek] .panel[data-file=project] textarea[data-f=ev]').value.length`); got != 2022 {
		t.Errorf("the long JSON is %d characters, want 2022", got)
	}
	if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
		t.Errorf("500: the long JSON flows the page %dpx sideways", over)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open=codex-high"), chromedp.WaitReady("#rows .row"))
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.row[data-name=codex-high] .panel[data-file=project] input[data-f=arg]').length`); got != 8 {
		t.Errorf("codex-high shows %d args, want 8", got)
	}
	if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
		t.Errorf("500: the eight args flow the page %dpx sideways", over)
	}
}
