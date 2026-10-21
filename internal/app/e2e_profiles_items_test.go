//go:build e2e

package app

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/chromedp/chromedp"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestE2EProfilesItems walks checks/profiles.md's items that TestE2EProfiles
// does not name one by one, over the same real-scale fixture (testdata/
// profiles-v2: 11 profiles, 4 definitions, 15 members, 3 workers) and the serve
// a person runs. Each subtest's name starts with the item's ID, so the gate
// reads which item a walk holds (r-f9ef).
//
// The read-only walks run first, and the three that write profiles.yaml — S7
// (a save so the footer says 保存しました), T1 and P7 — run last and take their
// profile back out, so a walk behind them opens the list the fixture wrote.
func TestE2EProfilesItems(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	initLocales(t)
	f := newFixture(t)
	copyProfilesFixture(t, f, cwd)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")
	pItemsRestore(t, f)

	base, stop := startServe(t, f)
	defer stop()

	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 8*time.Minute)
	defer stopTab()

	console := e2eListen(ctx, false)
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	t.Run("PROFILES-S2 the list's columns and order", func(t *testing.T) { e2ePItemsList(t, ctx, base) })
	t.Run("PROFILES-S3 the detail's order", func(t *testing.T) { e2ePItemsDetail(t, ctx, base) })
	t.Run("PROFILES-S4 a panel's order", func(t *testing.T) { e2ePItemsPanel(t, ctx, base) })
	t.Run("PROFILES-S5 the assign cards and rows", func(t *testing.T) { e2ePItemsAssign(t, ctx, base) })
	t.Run("PROFILES-S6 the two YAML sections", func(t *testing.T) { e2ePItemsYAML(t, ctx, base) })

	t.Run("PROFILES-T2 the assign tab's words", func(t *testing.T) { e2ePItemsAssignWords(t, ctx, base) })
	t.Run("PROFILES-T4 the words that are gone", func(t *testing.T) { e2ePItemsNoOldWords(t, ctx, base) })
	t.Run("PROFILES-T5 the env is not hidden", func(t *testing.T) { e2ePItemsEnvShown(t, ctx, base) })

	t.Run("PROFILES-P4 renaming a profile", func(t *testing.T) { e2ePItemsRename(t, ctx, base) })
	t.Run("PROFILES-P5 the kind buttons", func(t *testing.T) { e2ePItemsKind(t, ctx, base) })
	t.Run("PROFILES-P6 the args' and env's x and +", func(t *testing.T) { e2ePItemsArgsEnv(t, ctx, base) })
	t.Run("PROFILES-P9 copying to the other panel", func(t *testing.T) { e2ePItemsCopy(t, ctx, base) })
	t.Run("PROFILES-P10 the delete's two questions", func(t *testing.T) { e2ePItemsDeleteConfirm(t, ctx, base) })
	t.Run("PROFILES-P13 the kind bar thins the cards", func(t *testing.T) { e2ePItemsKindBar(t, ctx, base) })
	t.Run("PROFILES-P14 the check jumps into its line", func(t *testing.T) { e2ePItemsJump(t, ctx, base) })

	t.Run("PROFILES-W2 no width flows sideways", func(t *testing.T) { e2ePItemsWidths(t, ctx, base) })
	t.Run("PROFILES-W3 the long JSON at 500", func(t *testing.T) { e2ePItemsLongEnv(t, ctx, base) })
	t.Run("PROFILES-W4 the eight args at 500", func(t *testing.T) { e2ePItemsEightArgs(t, ctx, base) })

	t.Run("PROFILES-D1 the four lights read", func(t *testing.T) { e2ePItemsLights(t, ctx, base) })
	t.Run("PROFILES-D2 the colours are told apart", func(t *testing.T) { e2ePItemsColours(t, ctx, base) })

	t.Run("PROFILES-V1 the shared codex warning", func(t *testing.T) { e2ePItemsWarn(t, ctx, base) })
	t.Run("PROFILES-V2 the docs worker's missing profile", func(t *testing.T) { e2ePItemsMissing(t, ctx, base) })
	t.Run("PROFILES-V3 a name the pattern refuses", func(t *testing.T) { e2ePItemsBadName(t, ctx, base) })
	t.Run("PROFILES-V4 a broken line is marked", func(t *testing.T) { e2ePItemsBrokenLine(t, ctx, base) })

	t.Run("PROFILES-S7 the mock's classes", func(t *testing.T) { e2ePItemsClasses(t, ctx, base) })
	t.Run("PROFILES-T1 the list's words", func(t *testing.T) { e2ePItemsListWords(t, ctx, base) })
	t.Run("PROFILES-P7 a save says so", func(t *testing.T) { e2ePItemsSave(t, ctx, base) })

	if lines := console.Lines(); len(lines) > 0 {
		t.Errorf("the profile screen logged %d console errors: %v", len(lines), lines)
	}
}

// TestE2EProfilesYAMLHome is V7: the shared file stands under the home directory
// (~/.config/hergy) rather than in a made-up XDG directory, and every path the
// screen shows — the panel's head, the check's sentence, the YAML tab — is the
// .hergy/… or ~/.config/hergy/… form, with the working directory's absolute path
// nowhere. It runs its own serve because the settings directory is the whole
// point.
func TestE2EProfilesYAMLHome(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	initLocales(t)
	f := newFixture(t)
	root := filepath.Dir(f.proj)
	home := filepath.Join(root, "home")
	// The shared layer is under the home directory: XDG names none, so hergy
	// falls back to ~/.config/hergy (team.globalDir).
	t.Setenv("XDG_CONFIG_HOME", "")
	src := filepath.Join(cwd, "..", "..", "testdata", "profiles-v2")
	copyDir(t, filepath.Join(src, "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "config", "hergy"), filepath.Join(home, ".config", "hergy"))
	writeFile(t, filepath.Join(home, ".config", "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")

	base, stop := startServe(t, f)
	defer stop()

	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 3*time.Minute)
	defer stopTab()
	e2eListen(ctx, false)

	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	t.Run("PROFILES-V7 the shared file under the home", func(t *testing.T) { e2ePItemsHome(t, ctx, base, f, root) })
}

// pItemsRestore puts the project's profiles.yaml back after the walks that write
// it, so a walk behind them opens what the fixture wrote.
func pItemsRestore(t *testing.T, f *fixture) {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	before := readFile(t, path)
	t.Cleanup(func() { writeFile(t, path, before) })
}

// pItemsNames is the list's order over the fixture: the project's own profiles
// in the file's order, then the ones only the shared file holds, in its order
// (S2).
var pItemsNames = []string{
	"sonnet-fast", "opus-deep", "fable-review", "haiku-chore", "codex-high",
	"codex-mini", "opencode-deepseek", "gemini-pro", "opus-plan", "codex", "opencode-local",
}

// pItemsOpen opens the list and waits for its rows to be drawn from the island.
func pItemsOpen(t *testing.T, ctx context.Context, base, query string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"+query), chromedp.WaitReady("#rows .row"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row').length===11`)
}

// pItemsPanel returns a JS expression for one panel of the list.
func pItemsPanel(name, file string) string {
	return `document.querySelector('#rows .row[data-name=' + JSON.stringify(` + strconv.Quote(name) + `) + ` +
		`'] .panel[data-file=' + JSON.stringify(` + strconv.Quote(file) + `) + ']')`
}

// e2ePItemsList is S2: the head's columns and the eleven rows in the two files'
// order.
func e2ePItemsList(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "")
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('.list > .head > span')].map(function(e){return e.textContent.trim()}).filter(Boolean).join(',')`); got != "名前,kind,置き場所,引数,環境変数,使っている人" {
		t.Errorf("the head reads %q, want the six columns", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row').length`); got != 11 {
		t.Errorf("the list has %d rows, want 11", got)
	}
	got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('#rows .row')].map(function(e){return e.dataset.name}).join(',')`)
	if want := strings.Join(pItemsNames, ","); got != want {
		t.Errorf("the rows read %q, want %q", got, want)
	}
}

// e2ePItemsDetail is S3: an opened row holds the people first, then the two
// panels, the project's on the left and the shared one on the right.
func e2ePItemsDetail(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=sonnet-fast")
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=sonnet-fast] .detail:not([hidden]) .panel').length===2`)
	got := e2eProfilesValue(t, ctx, `[...document.querySelector('#rows .row[data-name=sonnet-fast] .detail').children].map(function(e){return e.className}).join('|')`)
	if !strings.HasPrefix(got, "usage|panels") {
		t.Errorf("the detail's children read %q, want the usage then the panels", got)
	}
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('#rows .row[data-name=sonnet-fast] .panel')].map(function(e){return e.dataset.file}).join(',')`); got != "project,global" {
		t.Errorf("the panels read %q, want the project's then the shared one", got)
	}
}

// e2ePItemsPanel is S4: one panel is a head, then 名前, kind, 引数, 環境変数, and
// a foot.
func e2ePItemsPanel(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=sonnet-fast")
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=sonnet-fast] .panel').length===2`)
	p := pItemsPanel("sonnet-fast", "project")
	if got := e2eProfilesValue(t, ctx, `[...`+p+`.querySelectorAll(':scope > .field > label')].map(function(e){return e.textContent}).join(',')`); got != "名前,kind,引数,環境変数" {
		t.Errorf("the panel's fields read %q, want 名前,kind,引数,環境変数", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project]').firstElementChild.className`); got != "ph" {
		t.Errorf("the panel's first child is %q, want the head", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project]').lastElementChild.className`); got != "pf" {
		t.Errorf("the panel's last child is %q, want the foot", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelector('#rows .row[data-name=sonnet-fast] .panel[data-file=project]').children.length`); got != 6 {
		t.Errorf("the panel has %d children, want the head, four fields and the foot", got)
	}
}

// e2ePItemsAssign is S5: four cards in the definitions' order, 15 members and 3
// workers.
func e2ePItemsAssign(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('.team')].map(function(e){return e.dataset.g}).join(',')`); got != "core,docs,migrate,research" {
		t.Errorf("the cards read %q, want core,docs,migrate,research", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.team').length`); got != 4 {
		t.Errorf("the assign tab has %d cards, want 4", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow[data-k^="m-"]').length`); got != 15 {
		t.Errorf("the cards hold %d members, want 15", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow[data-k^="w-"]').length`); got != 3 {
		t.Errorf("the cards hold %d workers, want 3", got)
	}
}

// e2ePItemsYAML is S6: two sections, the project's then the shared one.
func e2ePItemsYAML(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed').length===2`)
	if got := e2eProfilesValue(t, ctx, `[...document.querySelectorAll('.yed')].map(function(e){return e.dataset.file}).join(',')`); got != "project,global" {
		t.Errorf("the sections read %q, want project,global", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.yed').length`); got != 2 {
		t.Errorf("the YAML tab has %d sections, want 2", got)
	}
}

// e2ePItemsAssignWords is T2: the assign tab says チーム定義, ワーカー, from <名>,
// role <ロール>, and N 人.
func e2ePItemsAssignWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	text := e2eProfilesValue(t, ctx, `document.querySelector('#view').textContent`)
	for _, word := range []string{"チーム定義", "ワーカー"} {
		if !strings.Contains(text, word) {
			t.Errorf("the assign tab never says %q", word)
		}
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.arow[data-k^="w-"] .role code').textContent`); !strings.HasPrefix(got, "from ") {
		t.Errorf("a worker's line reads %q, want from <メンバー名>", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.arow[data-k^="m-"] .role code').textContent`); !strings.HasPrefix(got, "role ") {
		t.Errorf("a member's line reads %q, want role <ロール>", got)
	}
	// N 人 stands on a card once a pick makes it dirty.
	e2eProfilesSelect(t, ctx, `document.querySelector('.team[data-g=core] .arow[data-k="m-2"] select')`, "opus-deep")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=core] .tside .dirty')`)
	got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=core] .tside .dirty').textContent`)
	if !strings.HasSuffix(got, " 人") || got == " 人" {
		t.Errorf("the card's own count reads %q, want N 人", got)
	}
}

// e2ePItemsNoOldWords is T4: 層, layer, 担当, 役割 and 版 are gone from all three
// tabs.
func e2ePItemsNoOldWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, one := range []struct{ path, ready string }{
		{"/profiles", "#rows .row"},
		{"/profiles/assign", ".team"},
		{"/profiles/yaml", ".yed"},
	} {
		e2eRun(t, ctx, chromedp.Navigate(base+one.path), chromedp.WaitReady(one.ready))
		text := e2eProfilesValue(t, ctx, `document.body.textContent`)
		for _, word := range []string{"層", "layer", "担当", "役割", "版"} {
			if strings.Contains(text, word) {
				t.Errorf("%s still says %q", one.path, word)
			}
		}
	}
}

// e2ePItemsEnvShown is T5: the env's value stands as it is, not hidden behind
// dots.
func e2ePItemsEnvShown(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=opencode-deepseek")
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]').tagName`); got != "TEXTAREA" {
		t.Errorf("the value stands in a %s, want the text it is", got)
	}
	val := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]').value`)
	if !strings.Contains(val, `"$schema"`) || utf8.RuneCountInString(val) != 2022 {
		t.Errorf("the value reads %d characters and %v, want the whole JSON", utf8.RuneCountInString(val), strings.Contains(val, `"$schema"`))
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('input[type=password]').length`); got != 0 {
		t.Errorf("the screen draws %d password fields, want none", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.row[data-name=opencode-deepseek] .c-env').textContent`); !strings.Contains(got, `"$schema"`) {
		t.Errorf("the row's own column hides the value: %q", got)
	}
}

// e2ePItemsRename is P4: a rename changes the name's field, the rewrite box under
// it and the check's sentence, and nothing else.
func e2ePItemsRename(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=opus-deep")
	p := pItemsPanel("opus-deep", "project")
	e2eProfilesSet(t, ctx, p+` .querySelector('[data-f=name]')`+"", "opus-deep-2")
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .rename').hidden`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .rename span').textContent`); got != "使っている 2 人の指定も opus-deep-2 に書き換える" {
		t.Errorf("the rewrite box reads %q", got)
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name=opus-deep] .panel[data-file=project] .changed').length`); got != 1 {
		t.Errorf("the rename changed %d fields, want the name alone", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .changed').dataset.f`); got != "name" {
		t.Errorf("the changed field is %q, want the name", got)
	}
}

// e2ePItemsKind is P5: a kind's button changes itself and the args' head beside
// it, and nothing else.
func e2ePItemsKind(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=opus-deep")
	e2eProfilesClick(t, ctx, `#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act="set-kind"][data-k="codex"]`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .bin').textContent`); got != "codex" {
		t.Errorf("the args' head reads %q after the kind change", got)
	}
	if !e2eProfilesBool(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act="set-kind"][data-k="codex"]').classList.contains('on')`) {
		t.Errorf("the pressed kind is not the one marked on")
	}
	if e2eProfilesBool(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act="set-kind"][data-k="claude"]').classList.contains('on')`) {
		t.Errorf("the old kind is still marked on")
	}
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name=opus-deep] .panel[data-file=project] .changed').length`); got != 1 {
		t.Errorf("the kind change changed %d fields, want the kind alone", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .changed').dataset.f`); got != "kind" {
		t.Errorf("the changed field is %q, want the kind", got)
	}
}

// e2ePItemsArgsEnv is P6: the × and the ＋ each change their own chip or row.
func e2ePItemsArgsEnv(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=codex-high")
	args := `document.querySelectorAll('#rows .row[data-name=codex-high] .panel[data-file=project] .args > .arg').length`
	envs := `document.querySelectorAll('#rows .row[data-name=codex-high] .panel[data-file=project] .envrow').length`
	if got := e2eProfilesInt(t, ctx, args); got != 8 {
		t.Errorf("codex-high shows %d args, want 8", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=project] .args .arg .x`)
	if got := e2eProfilesInt(t, ctx, args); got != 7 {
		t.Errorf("removing an arg left %d, want 7", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=project] [data-act="arg-add"]`)
	if got := e2eProfilesInt(t, ctx, args); got != 8 {
		t.Errorf("adding an arg left %d, want 8", got)
	}
	if got := e2eProfilesInt(t, ctx, envs); got != 1 {
		t.Errorf("codex-high shows %d env rows, want 1", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=project] .envrow .x`)
	if got := e2eProfilesInt(t, ctx, envs); got != 0 {
		t.Errorf("removing the env left %d, want 0", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=project] [data-act="env-add"]`)
	if got := e2eProfilesInt(t, ctx, envs); got != 1 {
		t.Errorf("adding an env left %d, want 1", got)
	}
}

// e2ePItemsCopy is P9: copying into the other panel puts that panel in the same
// row, marked as not yet written.
func e2ePItemsCopy(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=codex-high")
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=project] [data-act="copy"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=codex-high] .panel').length===2`)
	if !e2eProfilesBool(t, ctx, `!!document.querySelector('#rows .row[data-name=codex-high] .panel[data-file=global]')`) {
		t.Errorf("the copy did not put the shared panel in the row")
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=codex-high] .pill.pending').textContent`); got != "共通" {
		t.Errorf("the copied place reads %q, want 共通", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=codex-high] .panel[data-file=global] [data-act="discard"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=codex-high] .panel').length===1`)
}

// e2ePItemsDeleteConfirm is P10: the delete's first press shows two questions,
// 削除する（N 人が使っている） and やめる, and やめる puts 削除 back.
func e2ePItemsDeleteConfirm(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=opus-deep")
	e2eProfilesClick(t, ctx, `#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-yes"]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-yes"]').textContent`); got != "削除する（2 人が使っている）" {
		t.Errorf("the confirm reads %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-no"]').textContent`); got != "やめる" {
		t.Errorf("the way out reads %q, want やめる", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=opus-deep] [data-act="del-no"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-yes"]')`)
	if !e2eProfilesBool(t, ctx, `!!document.querySelector('#rows .row[data-name=opus-deep] [data-act="del"]')`) {
		t.Errorf("やめる did not put 削除 back")
	}
}

// e2ePItemsKindBar is P13: pressing a kind's bar thins the bar and the cards'
// rows, and the address keeps the choice; pressing it again puts them back.
func e2ePItemsKindBar(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	e2eProfilesClick(t, ctx, `.kseg[data-k="codex"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.kseg.dim').length>0`)
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow.dim').length`); got == 0 {
		t.Errorf("picking a kind thins no row")
	}
	if got := e2eProfilesValue(t, ctx, `window.location.search`); got != "?kind=codex" {
		t.Errorf("the address reads %q after the pick", got)
	}
	e2eProfilesClick(t, ctx, `.kseg[data-k="codex"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.kseg.dim').length===0`)
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('.arow.dim').length`); got != 0 {
		t.Errorf("pressing the kind again left %d rows thin", got)
	}
}

// e2ePItemsJump is P14: pressing a check's line selects that line of the same
// section's text and puts the cursor in it.
func e2ePItemsJump(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `!!document.querySelector('.yed[data-file=global] .vitem[data-line="9"]')`)
	e2eProfilesClick(t, ctx, `.yed[data-file=global] .vitem[data-line="9"]`)
	e2eProfilesWait(t, ctx, `document.activeElement === document.querySelector('.yed[data-file=global] .ytext')`)
	selected := `(function(){var ta=document.querySelector('.yed[data-file=global] .ytext');var lines=ta.value.split('\n');var start=0;for(var i=0;i<8;i++){start+=lines[i].length+1;}return ta.selectionStart===start && ta.selectionEnd===start+lines[8].length;})()`
	if !e2eProfilesBool(t, ctx, selected) {
		t.Errorf("the jump did not select line 9 of the section's own text")
	}
	if got := e2eProfilesValue(t, ctx, `(function(){var ta=document.querySelector('.yed[data-file=global] .ytext');return ta.value.substring(ta.selectionStart,ta.selectionEnd);})()`); got != "  codex:" {
		t.Errorf("the selected line reads %q, want the codex key", got)
	}
}

// e2ePItemsWidths is W2: no tab flows sideways at any of the three widths.
func e2ePItemsWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pages := []struct{ path, ready string }{
		{"/profiles?open=sonnet-fast", `document.querySelectorAll('#rows .row[data-name=sonnet-fast] .detail:not([hidden]) .panel').length===2`},
		{"/profiles/assign", `document.querySelectorAll('.team').length===4`},
		{"/profiles/yaml", `document.querySelectorAll('.yed').length===2`},
	}
	for _, size := range []int64{2000, 1440, 500} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
		for _, page := range pages {
			e2eRun(t, ctx, chromedp.Navigate(base+page.path), chromedp.WaitReady("#view"))
			e2eProfilesWait(t, ctx, page.ready)
			if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
				t.Errorf("%s at %d flows %dpx sideways", page.path, size, over)
			}
		}
	}
}

// e2ePItemsLongEnv is W3: at 500 the 2022-character JSON wraps and shows whole.
func e2ePItemsLongEnv(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 900))
	pItemsOpen(t, ctx, base, "?open=opencode-deepseek")
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]')`)
	if got := e2eProfilesInt(t, ctx, `document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]').value.length`); got != 2022 {
		t.Errorf("the long JSON is %d characters, want 2022", got)
	}
	box := `document.querySelector('#rows .row[data-name=opencode-deepseek] .panel[data-file=project] [data-f=ev]')`
	if over := e2eProfilesInt(t, ctx, `Math.max(`+box+`.scrollWidth-`+box+`.clientWidth, `+box+`.scrollHeight-`+box+`.clientHeight)`); over > 2 {
		t.Errorf("the JSON is cut off inside its box by %dpx, want it all shown", over)
	}
	if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
		t.Errorf("500: the long JSON flows the page %dpx sideways", over)
	}
}

// e2ePItemsEightArgs is W4: at 500 the eight args wrap and show whole.
func e2ePItemsEightArgs(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(500, 900))
	pItemsOpen(t, ctx, base, "?open=codex-high")
	if got := e2eProfilesInt(t, ctx, `document.querySelectorAll('#rows .row[data-name=codex-high] .panel[data-file=project] input[data-f=arg]').length`); got != 8 {
		t.Errorf("codex-high shows %d args, want 8", got)
	}
	if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.querySelector('#rows .row[data-name=codex-high] .panel[data-file=project] .args')).flexWrap`); got != "wrap" {
		t.Errorf("the args read %q, want them wrapped", got)
	}
	if over := e2eProfilesInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
		t.Errorf("500: the eight args flow the page %dpx sideways", over)
	}
}

// e2ePItemsLights is D1: the OS's light and dark and the page's own light and
// dark all read, the body's ink and the name, the place and the field's label
// staying legible against their ground.
func e2ePItemsLights(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	pItemsOpen(t, ctx, base, "?open=sonnet-fast")
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
			{"the row's name", `#rows .row[data-name=sonnet-fast] .c-name .nm`},
			{"the place's pill", `#rows .row[data-name=sonnet-fast] .pill`},
			{"the field's label", `#rows .row[data-name=sonnet-fast] .panel[data-file=project] .field > label`},
		} {
			if got := e2eRead[float64](t, ctx, e2eActivityItemInk(one.find)); got < 4.5 {
				t.Errorf("in %s %s reads at %.2f against its ground, want at least 4.5", light.name, one.what, got)
			}
		}
	}
	// The OS's dark is the design's own ground.
	e2eDark(t, ctx)
	e2eTheme(t, ctx, "")
	if got := e2eProfilesValue(t, ctx, `getComputedStyle(document.body).backgroundColor`); got != "rgb(15, 18, 24)" {
		t.Errorf("the OS dark ground is %q", got)
	}
	e2eTheme(t, ctx, "")
	e2eActivityLight(t, ctx)
}

// e2ePItemsColours is D2: in both lights the change is blue, the problem red and
// the warning yellow, and the four kinds are told apart from one another.
func e2ePItemsColours(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(1440, 1000))
	pItemsOpen(t, ctx, base, "?open=sonnet-fast")
	palette := `(function(){` +
		`var pick=function(t){var d=document.createElement('i');d.style.background='var('+t+')';document.body.appendChild(d);var c=getComputedStyle(d).backgroundColor;d.remove();return c;};` +
		`var sw=function(k){return getComputedStyle(document.querySelector('.seg[data-f=kind] .sw.'+k)).backgroundColor;};` +
		`return [pick('--changed'),pick('--danger'),pick('--warn'),sw('k-claude'),sw('k-codex'),sw('k-opencode'),sw('k-gemini')].join('|');})()`
	for _, light := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		if light.dark {
			e2eDark(t, ctx)
		} else {
			e2eActivityLight(t, ctx)
		}
		e2eTheme(t, ctx, "")
		parts := strings.Split(e2eProfilesValue(t, ctx, palette), "|")
		if len(parts) != 7 {
			t.Fatalf("in %s the palette read %d colours, want 7: %v", light.name, len(parts), parts)
		}
		changed, danger, warn := pItemsRGB(parts[0]), pItemsRGB(parts[1]), pItemsRGB(parts[2])
		kinds := []struct {
			name string
			rgb  [3]int
		}{{"claude", pItemsRGB(parts[3])}, {"codex", pItemsRGB(parts[4])}, {"opencode", pItemsRGB(parts[5])}, {"gemini", pItemsRGB(parts[6])}}
		if h := pItemsHue(changed); h < 190 || h > 265 {
			t.Errorf("in %s the change's hue is %.0f, want the design's blue", light.name, h)
		}
		if h := pItemsHue(danger); h > 30 && h < 340 {
			t.Errorf("in %s the problem's hue is %.0f, want the design's red", light.name, h)
		}
		if h := pItemsHue(warn); h < 30 || h > 75 {
			t.Errorf("in %s the warning's hue is %.0f, want the design's yellow", light.name, h)
		}
		// The three states are told apart by their bands — blue, red, yellow —
		// and the kinds from one another by a clear gap; a state is never the
		// same swatch as a kind. The design's own red and orange sit close (the
		// prototype's palette), so the state-versus-kind promise is that they are
		// not the same colour, not that they stand a fixed distance apart.
		for _, a := range []struct {
			what string
			rgb  [3]int
		}{{"the change", changed}, {"the problem", danger}, {"the warning", warn}} {
			for _, k := range kinds {
				if a.rgb == k.rgb {
					t.Errorf("in %s %s and the %s kind read the same colour %v", light.name, a.what, k.name, a.rgb)
				}
			}
		}
		for i := 0; i < len(kinds); i++ {
			for j := i + 1; j < len(kinds); j++ {
				if d := pItemsDist(kinds[i].rgb, kinds[j].rgb); d < 40 {
					t.Errorf("in %s the %s and %s kinds read %.0f apart, want them told apart", light.name, kinds[i].name, kinds[j].name, d)
				}
			}
		}
	}
	e2eActivityLight(t, ctx)
}

// e2ePItemsWarn is V1: the shared codex carries 警告 1 in the list and the same
// warning on line 9 of the shared YAML.
func e2ePItemsWarn(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "")
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=codex] .c-name .sev').textContent`); got != "警告 1" {
		t.Errorf("the shared codex says %q, want 警告 1", got)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed[data-file=global] .vitem').length>0`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=global] .gutter .warn').textContent`); got != "9" {
		t.Errorf("the warning is on line %q, want 9", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=global] .vitem .msg').textContent`); !strings.Contains(got, "codex") {
		t.Errorf("the warning's sentence reads %q, want it about codex", got)
	}
}

// e2ePItemsMissing is V2: the docs worker names a profile that is not there, the
// card counts 問題 1, and the card's save is shut.
func e2ePItemsMissing(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"), chromedp.WaitReady(".team"))
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=docs] .arow.missing .vm')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=docs] .arow.missing .vm').textContent`); !strings.Contains(got, "workers[0].profile: gemini-flash というプロファイルはありません") {
		t.Errorf("the worker's line reads %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.team[data-g=docs] .tside').textContent`); !strings.Contains(got, "問題 1") {
		t.Errorf("the card's head does not say 問題 1: %q", got)
	}
	// A pick makes the card dirty, so its save appears — and it is shut while a
	// person still names nothing.
	e2eProfilesSelect(t, ctx, `document.querySelector('.team[data-g=docs] .arow[data-k="m-1"] select')`, "sonnet-fast")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=docs] [data-act="g-save"]')`)
	if !e2eProfilesBool(t, ctx, `document.querySelector('.team[data-g=docs] [data-act="g-save"]').disabled`) {
		t.Errorf("a card with a missing profile can be saved")
	}
}

// e2ePItemsBadName is V3: a name the pattern refuses is written under the field
// and stops the save.
func e2ePItemsBadName(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=opus-deep")
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] [data-f=name]')`, "sonnet-5.5")
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .vm[data-for=name] .sev.err')`)
	want := "profiles.sonnet-5.5: プロファイル名は [a-z][a-z0-9_-]{0,31} の形にしてください"
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .vm[data-for=name]').textContent`); !strings.Contains(got, want) {
		t.Errorf("the name's check reads %q, want %q", got, want)
	}
	if !e2eProfilesBool(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act=save]').disabled`) {
		t.Errorf("a name the pattern refuses can be saved")
	}
}

// e2ePItemsBrokenLine is V4: a broken line is red in the gutter and named with
// its number in the check's column.
func e2ePItemsBrokenLine(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed').length===2`)
	e2eProfilesSet(t, ctx, `document.querySelector('.yed[data-file=project] .ytext')`, "version: 1\nprofiles:\n  bad:\n    kind: \"\"\n")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.yed[data-file=project] .gutter .err')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=project] .gutter .err').textContent`); got != "4" {
		t.Errorf("the red line is %q, want 4", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=project] .vitem .lno').textContent`); got != "4 行" {
		t.Errorf("the check's line reads %q, want 4 行", got)
	}
	e2eProfilesClick(t, ctx, `.yed[data-file=project] [data-act="yaml-revert"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed[data-file=project] .gutter .err').length===0`)
}

// e2ePItemsListWords is T1: every word the list and an opened row say, which
// takes a rename, a delete's question, a new row and a save to draw them all.
func e2ePItemsListWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=sonnet-fast,codex-high,opus-deep,opus-plan")
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row.open').length===4`)
	text := e2eProfilesValue(t, ctx, `document.querySelector('#view').textContent`)
	for _, word := range []string{
		"＋ プロファイルを追加", "すべて", "置き場所", "使っている人", "効いている", "隠れている",
		"複製", "共通にコピー", "プロジェクトにコピー", "ほか",
	} {
		if !strings.Contains(text, word) {
			t.Errorf("the list never says %q", word)
		}
	}
	// N か所: a saved panel with a change.
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] [data-f=name]')`, "opus-deep-2")
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .dirty') && document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .dirty').textContent!==''`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] .panel[data-file=project] .dirty').textContent`); got != "1 か所" {
		t.Errorf("the changed count reads %q, want 1 か所", got)
	}
	// 削除する（N 人が使っている） and やめる.
	e2eProfilesClick(t, ctx, `#rows .row[data-name=opus-deep] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-yes"]')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-yes"]').textContent`); got != "削除する（2 人が使っている）" {
		t.Errorf("the confirm reads %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=opus-deep] [data-act="del-no"]').textContent`); got != "やめる" {
		t.Errorf("the way out reads %q, want やめる", got)
	}
	pItemsOpen(t, ctx, base, "")
	// 新規 and 取り消し on a row a person added.
	e2eProfilesClick(t, ctx, `[data-act="new"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row.unsaved .tag')`)
	row := `document.querySelector('#rows .row.unsaved')`
	if got := e2eProfilesValue(t, ctx, row+`.querySelector('.tag').textContent`); got != "新規" {
		t.Errorf("the new row's tag reads %q, want 新規", got)
	}
	if got := e2eProfilesValue(t, ctx, row+`.textContent`); !strings.Contains(got, "取り消し") {
		t.Errorf("the new row does not offer 取り消し")
	}
	// 保存しました: the save writes the row and the footer says so.
	e2eProfilesSet(t, ctx, row+`.querySelector('[data-f=name]')`, "t1-fresh")
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name=t1-fresh] [data-act=save]')`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=t1-fresh] .ok')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=t1-fresh] .ok').textContent`); got != "保存しました" {
		t.Errorf("the saved line reads %q", got)
	}
	// Take the row out again so the walks behind this one open the fixture.
	e2eProfilesClick(t, ctx, `#rows .row[data-name=t1-fresh] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=t1-fresh] [data-act="del-yes"]')`)
	e2eProfilesClick(t, ctx, `#rows .row[data-name=t1-fresh] [data-act="del-yes"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name=t1-fresh]')`)
}

// e2ePItemsSave is P7: saving a new row writes 保存しました in its foot and the
// row's own summary takes the args up.
func e2ePItemsSave(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "")
	e2eProfilesClick(t, ctx, `[data-act="new"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row.unsaved [data-f=name]')`)
	row := `document.querySelector('#rows .row.unsaved')`
	if got := e2eProfilesValue(t, ctx, `[...`+row+`.querySelectorAll('.args > .arg input')].map(function(e){return e.value}).join('|')`); got != "--model|" {
		t.Errorf("the new row's args read %q, want --model and an empty one", got)
	}
	e2eProfilesSet(t, ctx, row+`.querySelector('[data-f=name]')`, "p7-fresh")
	e2eProfilesSet(t, ctx, `[...`+row+`.querySelectorAll('.args > .arg input')][1]`, "sonnet")
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name=p7-fresh] [data-act=save]')`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=p7-fresh] .ok')`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=p7-fresh] .ok').textContent`); got != "保存しました" {
		t.Errorf("the saved line reads %q", got)
	}
	if got := e2eProfilesValue(t, ctx, `document.querySelector('#rows .row[data-name=p7-fresh] .c-args').textContent`); !strings.Contains(got, "--model sonnet") {
		t.Errorf("the row's own summary reads %q, want the saved args", got)
	}
	e2eProfilesClick(t, ctx, `#rows .row[data-name=p7-fresh] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name=p7-fresh] [data-act="del-yes"]')`)
	e2eProfilesClick(t, ctx, `#rows .row[data-name=p7-fresh] [data-act="del-yes"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name=p7-fresh]')`)
}

// e2ePItemsHome is V7: with the shared file under the home directory, every path
// stands as .hergy/… or ~/.config/hergy/…, and the working directory's absolute
// path is nowhere.
func e2ePItemsHome(t *testing.T, ctx context.Context, base string, f *fixture, root string) {
	t.Helper()
	pItemsOpen(t, ctx, base, "?open=sonnet-fast")
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row[data-name=sonnet-fast] .panel').length===2`)
	pItemsReadPath(t, ctx, `#rows .row[data-name=sonnet-fast] .panel[data-file=project] .path`, ".hergy/profiles.yaml")
	pItemsReadPath(t, ctx, `#rows .row[data-name=sonnet-fast] .panel[data-file=global] .path`, "~/.config/hergy/profiles.yaml")

	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed').length===2`)
	pItemsReadPath(t, ctx, `.yed[data-file=project] .path`, ".hergy/profiles.yaml")
	pItemsReadPath(t, ctx, `.yed[data-file=global] .path`, "~/.config/hergy/profiles.yaml")

	// A broken shared text names the file in the ~/ form.
	e2eProfilesSet(t, ctx, `document.querySelector('.yed[data-file=global] .ytext')`, "version: 1\nprofiles: {}\nbogus: 1\n")
	e2eProfilesWait(t, ctx, `[...document.querySelectorAll('.yed[data-file=global] .vitem .msg')].some(function(e){return e.textContent.indexOf('field bogus not found')>=0})`)
	if got := e2eProfilesValue(t, ctx, `document.querySelector('.yed[data-file=global] .vlist').textContent`); !strings.Contains(got, "~/.config/hergy/profiles.yaml: field bogus not found") {
		t.Errorf("the broken shared file reads %q, want the ~/ form of its path", got)
	}

	// No absolute path of the working directory stands anywhere on the page.
	text := e2eProfilesValue(t, ctx, `document.body.textContent`)
	for _, abs := range []string{f.proj, root} {
		if strings.Contains(text, abs) {
			t.Errorf("the page writes the working directory's absolute path %q", abs)
		}
	}
}

// pItemsReadPath reads one path element and holds it to what a person should
// read.
func pItemsReadPath(t *testing.T, ctx context.Context, selector, want string) {
	t.Helper()
	if got := e2eProfilesValue(t, ctx, `document.querySelector(`+strconv.Quote(selector)+`).textContent.trim()`); got != want {
		t.Errorf("%s reads %q, want %q", selector, got, want)
	}
}

// pItemsRGB reads one "rgb(r, g, b)" as three numbers.
func pItemsRGB(s string) [3]int {
	s = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "rgb("), ")")
	parts := strings.Split(s, ",")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(strings.TrimSpace(parts[i]))
	}
	return out
}

// pItemsHue is one colour's hue in degrees, 0 for a grey.
func pItemsHue(c [3]int) float64 {
	r, g, b := float64(c[0])/255, float64(c[1])/255, float64(c[2])/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	d := mx - mn
	if d == 0 {
		return 0
	}
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}

// pItemsDist is how far apart two colours stand, the plain RGB distance a
// person's eye reads as two colours (D2).
func pItemsDist(a, b [3]int) float64 {
	dr, dg, db := float64(a[0]-b[0]), float64(a[1]-b[1]), float64(a[2]-b[2])
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// pItemsMockClasses is the class names the design's prototype (mock-profiles 版
// 1) writes, which spec says the screen copies as they stand (S7): the inline
// style's own selectors, the whole vocabulary the screen may write.
var pItemsMockClasses = []string{
	"acts", "add", "arg", "args", "arow", "bad", "bin", "btn", "c-args", "c-env",
	"c-kind", "c-name", "c-place", "c-users", "changed", "chev", "code", "crumb",
	"danger", "detail", "dim", "dirty", "env", "envrow", "eq", "err", "fchip",
	"field", "filters", "flag", "flash", "g", "grip", "gutter", "head", "in-name",
	"k-claude", "k-codex", "k-gemini", "k-opencode", "kindbar", "kseg", "lbl",
	"list", "ln", "lno", "mark", "missing", "more", "msg", "n", "nm", "none",
	"off", "ok", "on", "open", "panel", "panels", "path", "pending", "pf", "ph",
	"pick", "pill", "primary", "purpose", "r", "rename", "role", "row", "search",
	"seg", "sel", "sep", "sev", "shadow", "shadowed", "small", "solid", "state",
	"sub", "sum", "sw", "tab", "tabs", "tag", "team", "teams", "th", "tid", "tl",
	"tn", "toolbar", "top", "tside", "two", "uchip", "unsaved", "usage", "vclean",
	"vcount", "vhead", "vitem", "vlist", "vm", "vt", "warn", "was", "who", "wk",
	"wrap", "x", "ybody", "yed", "yeds", "ytext",
}

// pItemsMockCarve is the part of that vocabulary the screen does not draw inside
// #tabs/#view over this fixture: the shell and the band around it (wrap, top,
// crumb, mark, sep), which are the prototype's own copy of the page's frame and
// stand outside the screen's two parts, and more, the +N of a row that holds
// more than two env values — this fixture's widest profile holds two, so no
// honest row draws it. The reverse check below still holds the screen to not
// inventing a class in their place.
var pItemsMockCarve = map[string]bool{"wrap": true, "top": true, "crumb": true, "mark": true, "sep": true, "more": true}

// pItemsScreenOnlyClasses is what the screen writes that the prototype names not
// at rest: the kind bar's grown segments and the YAML code's height classes,
// which the script's own code makes (gN, hN), and the argument a drag holds
// (dragging).
func pItemsScreenOnlyClasses() []string {
	out := []string{"dragging"}
	for i := 1; i <= 20; i++ {
		out = append(out, fmt.Sprintf("g%d", i))
	}
	for i := 6; i <= 34; i++ {
		out = append(out, fmt.Sprintf("h%d", i))
	}
	return out
}

// e2ePItemsClasses is S7: standing beside the design's prototype, the screen's
// class names are the same. Every class the prototype names that this fixture
// draws stands inside #tabs or #view somewhere over a walk that opens every
// state, the screen writes no class the prototype does not (with the three
// carve-outs above), and the CSS the prototype agreed on is the one in effect —
// the two panels lay out as a grid.
func e2ePItemsClasses(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	seen := map[string]bool{}
	collect := func() {
		got := e2eRead[[]string](t, ctx, `(function(){var o={};`+
			`document.querySelectorAll('#tabs, #view, #tabs *, #view *').forEach(function(e){if(e.classList)e.classList.forEach(function(c){o[c]=1})});`+
			`return Object.keys(o)})()`)
		for _, c := range got {
			seen[c] = true
		}
	}
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))

	// The list with every row open, and the CSS in effect on the two panels.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles?open="+strings.Join(pItemsNames, ",")), chromedp.WaitReady("#rows .row"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('#rows .row.open').length===11`)
	collect()
	if got := e2eProfilesInt(t, ctx, `getComputedStyle(document.querySelector('#rows .row[data-name=sonnet-fast] .panels.two')).gridTemplateColumns.split(' ').filter(Boolean).length`); got != 2 {
		t.Errorf("the two panels read %d columns, want the prototype's grid of 2", got)
	}

	// A new row: the pending pill, the tag, the layer switch, the bad field, and
	// a save so the footer says 保存しました, then the delete's solid confirm.
	e2eProfilesClick(t, ctx, `[data-act="new"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row.unsaved [data-f=name]')`)
	collect()
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row.unsaved [data-f=name]')`, "Bad Name")
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="Bad Name"] .vm[data-for=name] .sev.err')`)
	collect()
	e2eProfilesSet(t, ctx, `document.querySelector('#rows .row[data-name="Bad Name"] [data-f=name]')`, "s7-fresh")
	e2ePressWhenEnabled(t, ctx, `document.querySelector('#rows .row[data-name="s7-fresh"] [data-act=save]')`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="s7-fresh"] .ok')`)
	collect()
	e2eProfilesClick(t, ctx, `#rows .row[data-name="s7-fresh"] .panel[data-file=project] [data-act="del"]`)
	e2eProfilesWait(t, ctx, `!!document.querySelector('#rows .row[data-name="s7-fresh"] [data-act="del-yes"].solid')`)
	collect()
	e2eProfilesClick(t, ctx, `#rows .row[data-name="s7-fresh"] [data-act="del-yes"]`)
	e2eProfilesWait(t, ctx, `!document.querySelector('#rows .row[data-name="s7-fresh"]')`)

	// The YAML tab, clean and with a broken text.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/yaml"), chromedp.WaitReady(".yed"))
	e2eProfilesWait(t, ctx, `!!document.querySelector('.yed[data-file=global] .vitem')`)
	collect()
	e2eProfilesSet(t, ctx, `document.querySelector('.yed[data-file=project] .ytext')`, "version: 1\nprofiles:\n  bad:\n    kind: \"\"\n")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.yed[data-file=project] .gutter .err')`)
	collect()
	e2eProfilesClick(t, ctx, `.yed[data-file=project] [data-act="yaml-revert"]`)
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.yed[data-file=project] .gutter .err').length===0`)

	// The assign tab: the dim kind, a jump's flash, and a changed pick.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign?kind=codex"), chromedp.WaitReady(".team"))
	e2eProfilesWait(t, ctx, `document.querySelectorAll('.kseg.dim').length>0`)
	collect()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign#core.m-0"), chromedp.WaitReady(".team"))
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=core] .arow[data-k="m-0"].flash')`)
	collect()
	e2eProfilesSelect(t, ctx, `document.querySelector('.team[data-g=core] .arow[data-k="m-2"] select')`, "opus-deep")
	e2eProfilesWait(t, ctx, `!!document.querySelector('.team[data-g=core] .arow[data-k="m-2"].changed s.was')`)
	collect()

	var missing []string
	for _, class := range pItemsMockClasses {
		if pItemsMockCarve[class] {
			continue
		}
		if !seen[class] {
			missing = append(missing, class)
		}
	}
	if len(missing) != 0 {
		t.Errorf("the screen draws none of %v, which the prototype names", missing)
	}
	allowed := map[string]bool{}
	for _, class := range pItemsMockClasses {
		allowed[class] = true
	}
	for _, class := range pItemsScreenOnlyClasses() {
		allowed[class] = true
	}
	var extra []string
	for class := range seen {
		if !allowed[class] {
			extra = append(extra, class)
		}
	}
	sort.Strings(extra)
	if len(extra) != 0 {
		t.Errorf("the screen writes %v, which the prototype does not name", extra)
	}
}
