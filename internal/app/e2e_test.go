//go:build e2e

package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
)

// TestE2ENav walks the navigation band over the real-scale fixture
// (testdata/profiles-v2 + testdata/roles-v2 + testdata/nav, spec-nav.md 5.1):
// the four names and the three numbers, the two screens, where a person last
// stood, the changes one screen carries to the other, the URLs the old WebUI
// answered, and the six widths and lights of 5.2. It runs the serve the way a
// person does — the command with --no-open, a working XDG, a made-up herdr —
// and drives the pages with chromedp.
func TestE2ENav(t *testing.T) {
	chrome := chromeBinary(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	initLocales(t)
	copyNavFixture(t, f, cwd)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, "", "")

	base, stop := startServe(t, f)
	defer stop()

	// The URLs a person hands the address bar, over the real serve (O1〜O4).
	t.Run("the address", func(t *testing.T) { e2eNavAddress(t, base) })

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.Headless)...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var mu sync.Mutex
	var console []string
	chromedp.ListenTarget(browserCtx, func(event any) {
		switch e := event.(type) {
		case *runtime.EventConsoleAPICalled:
			if e.Type != "error" {
				return
			}
			var parts []string
			for _, arg := range e.Args {
				parts = append(parts, fmt.Sprintf("%v %v %v", arg.Value, arg.Description, arg.Preview))
			}
			mu.Lock()
			console = append(console, strings.Join(parts, " | "))
			mu.Unlock()
		case *runtime.EventExceptionThrown:
			// An uncaught exception is not a console call, but it is the same
			// promise to the person: nothing red in the drawer.
			mu.Lock()
			console = append(console, "uncaught: "+e.ExceptionDetails.Error())
			mu.Unlock()
		}
	})

	ctx, cancel := context.WithTimeout(browserCtx, 5*time.Minute)
	defer cancel()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))

	t.Run("band", func(t *testing.T) { e2eNavBand(t, ctx, base) })
	t.Run("another tab", func(t *testing.T) { e2eNavAnotherTab(t, ctx, allocCtx, base) })
	t.Run("the place a person stood", func(t *testing.T) { e2eNavPlace(t, ctx, base) })
	t.Run("widths and lights", func(t *testing.T) { e2eNavWidths(t, ctx, base) })

	// The number a save moves, and the carried changes, both need a project whose
	// files the test may change under the page, so they run over projects of
	// their own (spec-nav.md 5.1).
	f3 := newFixture(t)
	copyNavFixture(t, f3, cwd)
	writeFile(t, filepath.Join(f3.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f3.runningHerdr(t, "", "")
	base3, stop3 := startServe(t, f3)
	defer stop3()
	t.Run("the number follows a save", func(t *testing.T) { e2eNavNumbers(t, ctx, base3) })

	f2 := newFixture(t)
	copyNavFixture(t, f2, cwd)
	writeFile(t, filepath.Join(f2.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f2.runningHerdr(t, "", "")
	base2, stop2 := startServe(t, f2)
	defer stop2()
	t.Run("the changes one screen carries", func(t *testing.T) { e2eNavCarry(t, ctx, base2, f2) })

	mu.Lock()
	defer mu.Unlock()
	if len(console) > 0 {
		t.Errorf("the band logged %d console errors: %v", len(console), console)
	}
}

// copyNavFixture puts the navigation's fixture under the test's project: the
// role screen's fixture — the profile screen's with the role fixture over it
// (spec-roles.md 4 章) — and the two shared team definitions (spec-nav.md 5.1).
func copyNavFixture(t *testing.T, f *fixture, cwd string) {
	t.Helper()
	src := filepath.Join(cwd, "..", "..", "testdata")
	for _, one := range []string{"roles-v2", "nav"} {
		for _, half := range []struct{ from, to string }{
			{filepath.Join("project", ".hergy"), filepath.Join(f.proj, ".hergy")},
			{filepath.Join("config", "hergy"), filepath.Join(f.cfg, "hergy")},
		} {
			from := filepath.Join(src, one, half.from)
			if _, err := os.Stat(from); err != nil {
				continue
			}
			copyDir(t, from, half.to)
		}
	}
}

// navGet asks the real serve for one URL without following a move, so the
// answer itself — its status and its page — is what the test reads.
func navGet(t *testing.T, base, path string) (*http.Response, string) {
	t.Helper()
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(body)
}

// e2eNavAddress is O1〜O4: the root moves to the profile screen, every URL of
// the old WebUI is not found with the band on it, the old files are gone, and a
// URL still carrying ?layer= answers like the same URL without it.
func e2eNavAddress(t *testing.T, base string) {
	t.Helper()
	resp, _ := navGet(t, base, "/")
	if resp.StatusCode != http.StatusFound {
		t.Errorf("GET / answers %d, want 302", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/profiles" {
		t.Errorf("GET / moves to %q, want /profiles", got)
	}

	gone := []string{
		"/definitions", "/definitions/core", "/definitions/core/members/lead",
		"/definitions/core/workers/lead/依存の更新", "/definitions/core/human",
		"/definitions/core/turns/lead/analyst", "/definitions/core/agreement",
		"/definitions/core/agreement/new", "/definitions/core/agreement/TEAM-1",
		"/activity", "/activity/core", "/agreement", "/agreement/new", "/agreement/SHARED-1",
		"/teams", "/running", "/team", "/team?name=core",
		"/rules", "/rules/", "/diff", "/bump", "/bump/", "/figure", "/panel/", "/nope",
	}
	for _, path := range gone {
		resp, body := navGet(t, base, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s answers %d, want 404 (and no move)", path, resp.StatusCode)
			continue
		}
		for _, want := range []string{`class="screens"`, "その画面はありません。", "<title>hergy</title>"} {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s does not hold %q:\n%s", path, want, body)
			}
		}
		if !strings.Contains(body, `<span class="scr soon"`) {
			t.Errorf("GET %s shows no screen as not built yet:\n%s", path, body)
		}
		if strings.Contains(body, `aria-current="page"`) {
			t.Errorf("GET %s marks a screen as the one the person is on:\n%s", path, body)
		}
	}

	for _, path := range []string{"/static/hergy.css", "/static/htmx-2.0.4.min.js", "/static/ui.js", "/static/copy.js"} {
		if resp, _ := navGet(t, base, path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s answers %d, want 404", path, resp.StatusCode)
		}
	}
	for _, path := range []string{"/static/nav.css", "/static/nav.js", "/static/profiles-v2.css", "/static/roles-v2.css"} {
		if resp, _ := navGet(t, base, path); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s answers %d, want 200", path, resp.StatusCode)
		}
	}

	// O4: a URL that still carries ?layer= is answered, not moved.
	resp, with := navGet(t, base, "/profiles?layer=project")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /profiles?layer=project answers %d, want 200", resp.StatusCode)
	}
	if _, without := navGet(t, base, "/profiles"); without != with {
		t.Errorf("?layer= answers a different page than the plain URL")
	}
}

// e2eNavBand is S1〜S4 and W1's row: the four names in order with their numbers,
// the two not built yet struck through, the one the person is on marked, the
// project's folder name, and the first tab reading 一覧.
func e2eNavBand(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, ctx, `!!document.querySelector('.screens .scr.on')`)

	got := e2eNavText(t, ctx, `JSON.stringify([...document.querySelectorAll('.screens .scr')].map(function(e){
		return [e.textContent.replace(/\s+/g,''), e.tagName, e.getAttribute('aria-current')||'', e.getAttribute('aria-disabled')||''];}))`)
	want := `[["活動","SPAN","","true"],["チーム定義6","SPAN","","true"],["プロファイル11","SPAN","page",""],["ロール6","A","",""]]`
	if got != want {
		t.Errorf("the band on /profiles reads\n%s\nwant\n%s", got, want)
	}

	crumbs := e2eNavText(t, ctx, `JSON.stringify((function(){var c=document.querySelector('.crumb');
		return [c.textContent.replace(/\s+/g,''), c.querySelector('a.mark').getAttribute('href')];})())`)
	if crumbs != `["hergy/proj","/"]` {
		t.Errorf("the band's crumb reads %s, want the project's folder name", crumbs)
	}

	if key := e2eNavText(t, ctx, `document.querySelector('.screens').dataset.key`); len(key) != 12 {
		t.Errorf("the band's key is %q, want 12 hex digits", key)
	}
	if pos := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('.bar')).position`); pos != "sticky" {
		t.Errorf("the band stands %q, want sticky", pos)
	}
	if rows := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('.screens')).display`); rows != "flex" {
		t.Errorf("the band reads %q at 2000, want one row of flex", rows)
	}

	// S3: the screen's name and its number stand in the band only; the tabs read
	// 一覧 / 割り当て 22 / YAML.
	tabs := e2eNavText(t, ctx, `JSON.stringify([...document.querySelectorAll('#tabs .tab')].map(function(e){return e.textContent.replace(/\s+/g,'');}))`)
	if tabs != `["一覧","割り当て22","YAML"]` {
		t.Errorf("the profile screen's tabs read %s, want 一覧 / 割り当て 22 / YAML", tabs)
	}

	// S4: no word of the layers, the duties or the version stands on a screen.
	for _, word := range []string{"層", "layer", "担当", "役割", "版"} {
		if n := e2eNavInt(t, ctx, `document.body.innerText.split(`+strconv.Quote(word)+`).length-1`); n > 0 {
			t.Errorf("the word %q stands %d times on /profiles", word, n)
		}
	}

	// The role screen's own band and tabs.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open .panel')`)
	roles := e2eNavText(t, ctx, `JSON.stringify([...document.querySelectorAll('.screens .scr')].map(function(e){
		return [e.textContent.replace(/\s+/g,''), e.tagName];}))`)
	wantRoles := `[["活動","SPAN"],["チーム定義6","SPAN"],["プロファイル11","A"],["ロール6","SPAN"]]`
	if roles != wantRoles {
		t.Errorf("the band on /roles reads\n%s\nwant\n%s", roles, wantRoles)
	}
	rtabs := e2eNavText(t, ctx, `JSON.stringify([...document.querySelectorAll('#tabs .tab')].map(function(e){return e.textContent.replace(/\s+/g,'');}))`)
	if rtabs != `["一覧","割り当て19","YAML"]` {
		t.Errorf("the role screen's tabs read %s, want 一覧 / 割り当て 19 / YAML", rtabs)
	}
}

// e2eNavPlace is N1, N3, N4, N5 and N10: the band's links walk the two screens,
// each screen comes back to the place it was left, the screen the person is on
// is not a link, and hergy goes to the profile screen's list.
func e2eNavPlace(t *testing.T, ctx context.Context, base string) {
	t.Helper()

	// N10: reach the band's role link with the keyboard, see its focus ring, and
	// open it with Enter — the same as pressing it (N1).
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, ctx, `!!document.querySelector('a.scr[data-screen="roles"]')`)
	e2eNavFocus(t, ctx, `a.scr[data-screen="roles"]`)
	outline := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('a.scr[data-screen="roles"]')).outlineWidth`)
	if outline != "2px" {
		t.Errorf("the focused band link has an outline of %s, want 2px", outline)
	}
	e2eRun(t, ctx, chromedp.KeyEvent(kb.Enter))
	e2eNavWait(t, ctx, `!!document.querySelector('.scr.on') && document.querySelector('.scr.on').dataset.screen === 'roles'`)
	if path := e2eNavText(t, ctx, `location.pathname`); path != "/roles" {
		t.Errorf("after Enter the page is at %q, want /roles (or the place the screen was left)", path)
	}

	// N3: the screen the person is on is a span, so pressing it does nothing.
	if tag := e2eNavText(t, ctx, `document.querySelector('.scr.on').tagName`); tag != "SPAN" {
		t.Errorf("the screen the person is on is a %s, want a span", tag)
	}

	// N2: the two screens that are not built yet are not links, they do not
	// offer a finger, and pressing one changes nothing.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, ctx, `!!document.querySelector('.scr.soon')`)
	if n := e2eNavInt(t, ctx, `document.querySelectorAll('.scr.soon').length`); n != 2 {
		t.Errorf("the band shows %d screens struck through, want 2", n)
	}
	for _, name := range []string{"活動", "チーム定義"} {
		script := `(function(){var el=[...document.querySelectorAll('.scr.soon')].find(function(e){return e.textContent.indexOf(` + strconv.Quote(name) + `)===0;});return [el.tagName, getComputedStyle(el).cursor, el.hasAttribute('href')];})()`
		got := e2eNavText(t, ctx, `JSON.stringify(`+script+`)`)
		if got != `["SPAN","default",false]` {
			t.Errorf("the band's %s reads %s, want a span with no finger and no link", name, got)
		}
	}
	e2eNavClick(t, ctx, `.scr.soon`)
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .row')`)
	if path := e2eNavText(t, ctx, `location.pathname`); path != "/profiles" {
		t.Errorf("pressing a screen that is not built moved the page to %q", path)
	}

	// N5: a row opened on the role screen comes back opened.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open .panel')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles")
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles?open=builder")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open')`)

	// The same for a screen left on its assign tab.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"))
	e2eNavWait(t, ctx, `!!document.querySelector('.teams .team')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles")
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles/assign")
	e2eNavWait(t, ctx, `!!document.querySelector('.teams .team')`)

	// N4: hergy goes to the profile screen's list, with nothing open.
	e2eNavClick(t, ctx, `.crumb a.mark`)
	e2eNavWaitPath(t, ctx, "/profiles")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows')`)
	if open := e2eNavInt(t, ctx, `document.querySelectorAll('#rows .row.open').length`); open != 0 {
		t.Errorf("hergy left %d rows open on the profile screen, want none", open)
	}
	if tab := e2eNavText(t, ctx, `document.querySelector('#tabs .tab.on').textContent.replace(/\s+/g,'')`); tab != "一覧" {
		t.Errorf("hergy lands on the %q tab, want 一覧", tab)
	}

	// The band stands on the third tab of both screens too, and each tab is a
	// page of its own.
	for _, one := range []struct{ path, screen string }{
		{"/profiles/yaml", "profiles"}, {"/roles/yaml", "roles"},
	} {
		e2eRun(t, ctx, chromedp.Navigate(base+one.path))
		e2eNavWait(t, ctx, `!!document.querySelector('.scr.on')`)
		if on := e2eNavText(t, ctx, `document.querySelector('.scr.on').dataset.screen`); on != one.screen {
			t.Errorf("%s shows %q as the screen the person is on, want %s", one.path, on, one.screen)
		}
		if tab := e2eNavText(t, ctx, `document.querySelector('#tabs .tab.on').textContent.replace(/\s+/g,'')`); tab != "YAML" {
			t.Errorf("%s lands on the %q tab, want YAML", one.path, tab)
		}
	}
}

// e2eNavAnotherTab is O5: a browser tab that was not there when the place was
// remembered walks the band's plain links.
func e2eNavAnotherTab(t *testing.T, ctx context.Context, allocCtx context.Context, base string) {
	t.Helper()
	otherCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	runCtx, cancelRun := context.WithTimeout(otherCtx, 30*time.Second)
	defer cancelRun()
	e2eRun(t, runCtx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, runCtx, `!!document.querySelector('a.scr[data-screen="roles"]')`)
	if href := e2eNavText(t, runCtx, `document.querySelector('a.scr[data-screen="roles"]').getAttribute('href')`); href != "/roles" {
		t.Errorf("a fresh tab's band points the role screen at %q, want /roles", href)
	}
	if dot := e2eNavText(t, runCtx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("a fresh tab's band shows a change mark, want none")
	}
}

// e2eNavCarry is N6〜N9 and R2/R3: a pick made and not saved is carried to the
// other screen and back, is dropped when the file moved outside, is not put
// back by a reload, and a draft the band did not carry is spent for good.
func e2eNavCarry(t *testing.T, ctx context.Context, base string, f *fixture) {
	t.Helper()
	const pick = `.team[data-g="core"] .arow[data-k="m-2"]`

	// N6: make a pick in the assign tab and walk to the role screen with the
	// band. The band shows a mark on the profile screen.
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"))
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	e2eNavSelect(t, ctx, pick+" select", "opus-deep")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(pick)+`).classList.contains('changed')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.scr[data-screen="profiles"] .dot'))`); dot != "true" {
		t.Errorf("N6: the band marks the profile screen, want a dot")
	}
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('a.scr[data-screen="profiles"] .dot'))`); dot != "true" {
		t.Errorf("N6: the changed pick left no dot on the profile screen")
	}
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.teams .arow.changed').length`); changed != 0 {
		t.Errorf("N6: the role screen shows %d changed picks, want none", changed)
	}

	// N7: walking back puts the pick where it was, marked as not saved yet.
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(pick)+`).classList.contains('changed')`)
	if was := e2eNavText(t, ctx, `((document.querySelector(`+strconv.Quote(pick)+`+' s.was')||{}).textContent||'')`); was != "codex-high" {
		t.Errorf("N7: the carried pick does not show what it was (%q)", was)
	}
	if n := e2eNavText(t, ctx, `((document.querySelector('.team[data-g="core"] .tside .dirty')||{}).textContent||'')`); n != "1 人" {
		t.Errorf("N7: the team's head reads %q, want 1 人", n)
	}
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.scr.on .dot'))`); dot != "true" {
		t.Errorf("N7: the screen the person is on shows no change mark")
	}

	// N9: a reload puts nothing back — the band carried it, and it is spent.
	e2eRun(t, ctx, chromedp.Reload())
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); changed != 0 {
		t.Errorf("N9: a reload put %d changed picks back, want none", changed)
	}

	// R2: a draft the band did not carry is spent, not put back later. Make a
	// pick, walk to the role screen with the band, then open the profile screen
	// by typing its URL — the draft is dropped there. Walking the band back and
	// forth afterwards must not bring it back.
	e2eNavSelect(t, ctx, pick+" select", "opus-deep")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(pick)+`).classList.contains('changed')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows')`)
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"))
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); changed != 0 {
		t.Errorf("R2: opening the profile screen by URL put %d changed picks back, want none", changed)
	}
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("R2: the band marks a screen after a typed URL dropped the draft")
	}
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('a.scr[data-screen="profiles"] .dot'))`); dot != "false" {
		t.Errorf("R2: the profile screen still shows a dot after its draft was dropped")
	}
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if v := e2eNavText(t, ctx, `document.querySelector(`+strconv.Quote(pick+" select")+`).value`); v != "codex-high" {
		t.Errorf("R2: the pick reads %q after the dropped draft, want codex-high", v)
	}
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); changed != 0 {
		t.Errorf("R2: a draft the band did not carry came back (%d changed picks), want none", changed)
	}

	// N6 again, for N8: carry the pick over, change team.yaml outside, and walk
	// back — the draft is not put back, and the band's mark goes.
	e2eNavSelect(t, ctx, pick+" select", "opus-deep")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(pick)+`).classList.contains('changed')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles")
	team := filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml")
	writeFile(t, team, strings.Replace(readFile(t, team), "注文 API を作り直す", "外で書き換えた使命", 1))
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); changed != 0 {
		t.Errorf("N8: a draft made before the file moved outside was put back (%d picks)", changed)
	}
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("N8: the band still marks a screen after the draft was dropped")
	}

	// R3: a field inside a role's row does not redraw the screen, but it is a
	// change all the same — the band's mark comes with it and goes when the
	// field is put back. (Last, so the place it leaves on the role screen does
	// not stand in for the band's walk of N6.)
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eNavWait(t, ctx, `!!document.querySelector('.role.open .panel[data-l="project"] [data-f="title"]')`)
	const title = `document.querySelector('.role.open .panel[data-l="project"] [data-f="title"]')`
	e2eNavSet(t, ctx, title, "設計と実装")
	e2eNavWait(t, ctx, `!!document.querySelector('.scr.on[data-screen="roles"] .dot')`)
	e2eNavSet(t, ctx, title, "設計実装")
	e2eNavWait(t, ctx, `!document.querySelector('.scr.on[data-screen="roles"] .dot')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("R3: the band still marks a screen after the field was put back")
	}
}

// e2eNavWidths is W1〜W6 and D1: the three widths in both lights, with the band
// in one row at the wide ones and in two at 500, the page never flowing
// sideways.
func e2eNavWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	paths := []string{"/profiles", "/roles?open=builder", "/nope"}
	for _, size := range []int64{2000, 1440, 500} {
		for _, theme := range []string{"", "light", "dark"} {
			e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
			for _, path := range paths {
				e2eRun(t, ctx, chromedp.Navigate(base+path))
				e2eNavWait(t, ctx, `!!document.querySelector('.screens')`)
				if theme == "" {
					e2eRun(t, ctx, chromedp.Evaluate(`delete document.documentElement.dataset.theme`, nil))
				} else {
					e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.dataset.theme=`+strconv.Quote(theme), nil))
				}
				if over := e2eNavInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
					t.Errorf("%s at %d/%s flows %dpx sideways", path, size, theme, over)
				}
				// R1: the band's own links take the browser's underline out;
				// only hergy's crumb keeps one, on hover.
				deco := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('.screens a.scr')).textDecorationLine`)
				if deco != "none" {
					t.Errorf("%s at %d/%s: the band's links read %q, want no underline", path, size, theme, deco)
				}
				rows := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('.screens')).display`)
				if size > 760 && rows != "flex" {
					t.Errorf("%s at %d: the band reads %q, want one row of flex", path, size, rows)
				}
				if size <= 760 {
					if rows != "grid" {
						t.Errorf("%s at %d: the band reads %q, want a grid of two rows", path, size, rows)
					}
					if cols := e2eNavInt(t, ctx, `getComputedStyle(document.querySelector('.screens')).gridTemplateColumns.split(' ').length`); cols != 4 {
						t.Errorf("%s at %d: the band's second row holds %d parts, want 4", path, size, cols)
					}
					// The four parts fill the band: it is as wide as the row above
					// it (the crumb), which stands on its own line.
					span := e2eNavInt(t, ctx, `Math.round(document.querySelector('.screens').getBoundingClientRect().width - document.querySelector('.crumb').getBoundingClientRect().width)`)
					if span < -1 || span > 1 {
						t.Errorf("%s at %d: the band is %dpx different from the crumb above it, want the same width", path, size, span)
					}
				}
			}
		}
	}
}

// e2eNavNumbers is 1.1's last line: a save moves the band's number the same
// time it moves the screen's own, and the mark goes with the save.
func e2eNavNumbers(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	band := `document.querySelector('.scr[data-screen="roles"] .n').textContent`
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("#rows .role"))
	if n := e2eNavText(t, ctx, band); n != "6" {
		t.Errorf("the band counts %s roles, want 6", n)
	}

	e2eNavClick(t, ctx, `[data-act=new]`)
	e2eNavWait(t, ctx, `!!document.querySelector('.panel[data-l=project] .rule.open [data-f=id]')`)
	panel := `document.querySelector('.panel[data-l=project]')`
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.scr.on .dot'))`); dot != "true" {
		t.Errorf("a row a person added left no mark in the band")
	}
	e2eNavSet(t, ctx, panel+`.querySelector('[data-f=name]')`, "nav-new")
	e2eNavSet(t, ctx, panel+`.querySelector('.rule.open [data-f=id]')`, "NAV-1")
	e2eNavSet(t, ctx, panel+`.querySelector('.rule.open [data-f=text]')`, "帯の数のために足した規則。")
	e2eNavWait(t, ctx, `!`+panel+`.querySelector('.pf [data-act=save]').disabled`)
	e2eNavPress(t, ctx, panel+`.querySelector('.pf [data-act=save]')`)
	e2eNavWait(t, ctx, `!!`+panel+`.querySelector('.ok')`)
	if n := e2eNavText(t, ctx, band); n != "7" {
		t.Errorf("after a save the band counts %s roles, want 7", n)
	}
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("the band still marks a screen after everything was saved")
	}

	// The profile screen keeps its own number the same way, from the answer's
	// count (1.1).
	bandP := `document.querySelector('.scr[data-screen="profiles"] .n').textContent`
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"), chromedp.WaitReady("#rows .row"))
	if n := e2eNavText(t, ctx, bandP); n != "11" {
		t.Errorf("the band counts %s profiles, want 11", n)
	}
	e2eNavClick(t, ctx, `[data-act=new]`)
	e2eNavSet(t, ctx, `document.querySelector('#rows .row[data-name=""] [data-f=name]')`, "nav-count")
	e2eNavWait(t, ctx, `!document.querySelector('#rows .row[data-name="nav-count"] [data-act=save]').disabled`)
	e2eNavClick(t, ctx, `#rows .row[data-name="nav-count"] [data-act=save]`)
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .row[data-name="nav-count"] .ok')`)
	if n := e2eNavText(t, ctx, bandP); n != "12" {
		t.Errorf("after a save the band counts %s profiles, want 12", n)
	}
}

// e2eNavText evaluates one expression and returns its string value.
func e2eNavText(t *testing.T, ctx context.Context, script string) string {
	t.Helper()
	var value string
	e2eRun(t, ctx, chromedp.Evaluate(script, &value))
	return value
}

// e2eNavInt evaluates one expression and returns its number.
func e2eNavInt(t *testing.T, ctx context.Context, script string) int {
	t.Helper()
	var value int
	e2eRun(t, ctx, chromedp.Evaluate(script, &value))
	return value
}

// e2eNavWait waits until one expression is true.
func e2eNavWait(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		e2eRun(t, ctx, chromedp.Evaluate(script, &ok))
		if ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("never became true: %s", script)
}

// e2eNavWaitPath waits until the page's address ends with one path, and the
// document has loaded. The address alone is not enough: the band remembers
// where a person stood and points each screen's link at those places when the
// document loads (nav.js 1.4), so reading or pressing before that acts on the
// addresses the server wrote, not on the band's (r-3c41).
func e2eNavWaitPath(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var loc string
		e2eRun(t, ctx, chromedp.Evaluate(`location.pathname + location.search`, &loc))
		if loc == path {
			e2eNavLoaded(t, ctx)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	var loc string
	e2eRun(t, ctx, chromedp.Evaluate(`location.pathname + location.search`, &loc))
	t.Fatalf("the page never reached %q (it is at %q)", path, loc)
}

// e2eNavLoaded waits until the document has loaded. A person cannot press
// before the page is there; a driver can, so the test waits for it.
func e2eNavLoaded(t *testing.T, ctx context.Context) {
	t.Helper()
	e2eNavWait(t, ctx, `document.readyState === 'complete'`)
}

// e2eNavClick presses one element the way a person presses it.
func e2eNavClick(t *testing.T, ctx context.Context, selector string) {
	t.Helper()
	e2eNavLoaded(t, ctx)
	script := `(function(){var el=document.querySelector(` + strconv.Quote(selector) + `);if(!el){throw new Error('no element for '+` + strconv.Quote(selector) + `);}el.click();})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eNavPress presses the element one expression stands for.
func e2eNavPress(t *testing.T, ctx context.Context, find string) {
	t.Helper()
	script := `(function(){var el=` + find + `;if(!el){throw new Error('no element for '+` + strconv.Quote(find) + `);}el.click();})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eNavSet writes one field's value and fires the input event a person's
// typing fires.
func e2eNavSet(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	script := `(function(){var el=` + find + `;if(!el){throw new Error('no element for '+` + strconv.Quote(find) + `);}el.value=` + strconv.Quote(value) + `;el.dispatchEvent(new Event('input',{bubbles:true}));})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eNavSelect picks one option of a select and fires the change event a
// person's picking fires.
func e2eNavSelect(t *testing.T, ctx context.Context, selector, value string) {
	t.Helper()
	script := `(function(){var el=document.querySelector(` + strconv.Quote(selector) + `);if(!el){throw new Error('no select for '+` + strconv.Quote(selector) + `);}el.value=` + strconv.Quote(value) + `;el.dispatchEvent(new Event('change',{bubbles:true}));})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eNavFocus presses Tab until one element has the keyboard, the way a person
// reaches it.
func e2eNavFocus(t *testing.T, ctx context.Context, selector string) {
	t.Helper()
	focused := `document.activeElement === document.querySelector(` + strconv.Quote(selector) + `)`
	for i := 0; i < 12; i++ {
		var ok bool
		e2eRun(t, ctx, chromedp.Evaluate(focused, &ok))
		if ok {
			return
		}
		e2eRun(t, ctx, chromedp.KeyEvent(kb.Tab))
	}
	t.Fatalf("Tab never reached %s", selector)
}

// chromeBinary is where Chrome is, or a skip on a machine that has none. In CI a
// missing Chrome is a failure (d-a8b1 の W3).
func chromeBinary(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("CHROME_PATH"),
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if os.Getenv("CI") != "" {
		t.Fatal("the e2e run needs Chrome and this machine has none")
	}
	t.Skip("no Chrome found; the e2e run is skipped on this machine (design W3)")
	return ""
}

// serveEnv is the environment the screens read, built the way Run builds it.
func serveEnv(t *testing.T) team.Env {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// startServe runs hergy serve with --no-open on a working XDG and a made-up
// herdr, and returns the URL it prints and a stop function.
func startServe(t *testing.T, f *fixture) (string, func()) {
	t.Helper()
	env := serveEnv(t)
	opts := options.Options{Port: 0, NoOpen: true}
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, env, &opts, urlWriter{urls}, io.Discard)
	}()
	select {
	case url := <-urls:
		return url, func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("serve did not stop")
			}
		}
	case err := <-done:
		cancel()
		t.Fatalf("serve stopped before it answered: %v", err)
	case <-time.After(20 * time.Second):
		cancel()
		t.Fatal("serve printed no URL")
	}
	return "", func() {}
}

// urlWriter picks the URL out of what runServe prints.
type urlWriter struct{ urls chan string }

func (w urlWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		if i := strings.Index(line, "http://"); i >= 0 {
			select {
			case w.urls <- strings.TrimSpace(line[i:]):
			default:
			}
		}
	}
	return len(p), nil
}

// e2eRun runs the actions, failing the test on the first error.
func e2eRun(t *testing.T, ctx context.Context, actions ...chromedp.Action) {
	t.Helper()
	if err := chromedp.Run(ctx, actions...); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
}
