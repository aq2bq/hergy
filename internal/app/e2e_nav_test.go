//go:build e2e

package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestE2ENavItems walks checks/nav.md's items that the band of screens owns and
// TestE2ENav does not name one by one: the three widths in both lights and the
// explicit theme (W2〜W6, D1), the first tab and the words that are gone (S3,
// S4), the band's links and the place each screen was left (N2, N3, N4, N5,
// N10), and the changes one screen carries to the other (N7, N8, N9). It runs
// over the same real-scale fixture as TestE2ENav — testdata/profiles-v2 with
// testdata/roles-v2 and testdata/nav over it, no team running — and the serve
// the way a person runs it.
func TestE2ENavItems(t *testing.T) {
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

	// The run's one browser, and this test's tab in it (r-a107).
	browser := e2eChrome(t, chrome)
	ctx, stopTab := e2eTab(t, browser, 8*time.Minute)
	defer stopTab()
	e2eRun(t, ctx, chromedp.EmulateViewport(2000, 1100))
	e2eThrottle(t, ctx)

	// The three widths in both lights (W2〜W6), and the explicit theme (D1).
	t.Run("NAV-W2 the band at 2000 in the dark", func(t *testing.T) { e2eNavBandAt(t, ctx, base, 2000, true) })
	t.Run("NAV-W3 the band at 1440 in the light", func(t *testing.T) {
		e2eNavBandAt(t, ctx, base, 1440, false)
		e2eNavTabs(t, ctx, base)
	})
	t.Run("NAV-W4 the band at 1440 in the dark", func(t *testing.T) {
		e2eNavBandAt(t, ctx, base, 1440, true)
		e2eNavTabs(t, ctx, base)
	})
	t.Run("NAV-W5 the band at 500 in the light", func(t *testing.T) { e2eNavBandAt(t, ctx, base, 500, false) })
	t.Run("NAV-W6 the band at 500 in the dark", func(t *testing.T) { e2eNavBandAt(t, ctx, base, 500, true) })
	t.Run("NAV-D1 the explicit theme", func(t *testing.T) { e2eNavExplicitTheme(t, ctx, base) })

	// The words: the first tab (S3), and the ones that are gone (S4).
	t.Run("NAV-S3 the first tab", func(t *testing.T) { e2eNavFirstTab(t, ctx, base) })
	t.Run("NAV-S4 the words that are gone", func(t *testing.T) { e2eNavNoOldWords(t, ctx, base) })

	// The band's links, and the place each screen was left.
	t.Run("NAV-N2 activity and teams", func(t *testing.T) { e2eNavN2(t, ctx, base) })
	t.Run("NAV-N3 the screen a person is on", func(t *testing.T) { e2eNavN3(t, ctx, base) })
	t.Run("NAV-N4 hergy", func(t *testing.T) { e2eNavN4(t, ctx, base) })
	t.Run("NAV-N5 the place of each screen", func(t *testing.T) { e2eNavN5(t, ctx, base) })
	t.Run("NAV-N10 the keyboard", func(t *testing.T) { e2eNavN10(t, ctx, base, browser) })

	// The changes one screen carries to the other. Each runs over a project of
	// its own, because each moves the files the others read.
	t.Run("NAV-N7 the carried pick comes back", func(t *testing.T) { e2eNavN7(t, ctx, cwd) })
	t.Run("NAV-N8 the file moved outside", func(t *testing.T) { e2eNavN8(t, ctx, cwd) })
	t.Run("NAV-N9 a reload", func(t *testing.T) { e2eNavN9(t, ctx, cwd) })
}

// navPick is the assign row N6〜N9 work on: the core team's impl-a, whose
// profile the fixture has as codex-high.
const navPick = `.team[data-g="core"] .arow[data-k="m-2"]`

// e2eNavServe starts a serve over a fresh navigation project, for the items that
// change the files a person's screens read, and returns it with the address and
// a stop function.
func e2eNavServe(t *testing.T, cwd string) (*fixture, string, func()) {
	t.Helper()
	f := newFixture(t)
	copyNavFixture(t, f, cwd)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	f.runningHerdr(t, "", "")
	base, stop := startServe(t, f)
	return f, base, stop
}

// e2eNavScheme asks the page to read prefers-color-scheme as light or dark, which
// is what a machine set to that theme reports to the screens.
func e2eNavScheme(t *testing.T, ctx context.Context, dark bool) {
	t.Helper()
	scheme := "light"
	if dark {
		scheme = "dark"
	}
	e2eCall(t, ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: scheme}},
	})
}

// e2eNavThree is the three URLs every width and light item opens.
var e2eNavThree = []string{"/profiles", "/roles?open=builder", "/nope"}

// e2eNavBandAt opens the three URLs at one width and light (W1〜W6): the band
// stands on each, the page never flows sideways, the band is one row to the right
// of hergy / <the folder> at the wide sizes and a two-row grid of four parts that
// fill it at 500, and the screen a person is on takes the ink's ground — a dark
// one in the light and a light one with dark letters in the dark.
func e2eNavBandAt(t *testing.T, ctx context.Context, base string, size int64, dark bool) {
	t.Helper()
	e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))
	e2eNavScheme(t, ctx, dark)
	for _, path := range e2eNavThree {
		e2eRun(t, ctx, chromedp.Navigate(base+path))
		e2eNavWait(t, ctx, `!!document.querySelector('.screens')`)
		if over := e2eNavInt(t, ctx, `document.documentElement.scrollWidth - window.innerWidth`); over > 1 {
			t.Errorf("%s at %d flows %dpx sideways", path, size, over)
		}
		display := e2eNavText(t, ctx, `getComputedStyle(document.querySelector('.screens')).display`)
		if size > 760 {
			if display != "flex" {
				t.Errorf("%s at %d: the band reads %q, want one row of flex", path, size, display)
			}
			placed := e2eNavText(t, ctx, `JSON.stringify((function(){var c=document.querySelector('.crumb').getBoundingClientRect(),s=document.querySelector('.screens').getBoundingClientRect();return [s.left>=c.right-1,s.top<c.bottom];})())`)
			if placed != "[true,true]" {
				t.Errorf("%s at %d: the band is not to the right of hergy / the folder on the same line: %s", path, size, placed)
			}
		} else {
			if display != "grid" {
				t.Errorf("%s at %d: the band reads %q, want a two-row grid", path, size, display)
			}
			if cols := e2eNavInt(t, ctx, `getComputedStyle(document.querySelector('.screens')).gridTemplateColumns.split(' ').filter(Boolean).length`); cols != 4 {
				t.Errorf("%s at %d: the band's second row holds %d parts, want 4", path, size, cols)
			}
			fill := e2eNavText(t, ctx, `JSON.stringify((function(){var s=document.querySelector('.screens'),c=document.querySelector('.crumb').getBoundingClientRect();var parts=[...document.querySelectorAll('.screens .scr')];var sum=parts.reduce(function(a,e){return a+e.getBoundingClientRect().width;},0);return [Math.abs(sum-s.clientWidth)<1.5,Math.abs(s.getBoundingClientRect().width-c.width)<1.5];})())`)
			if fill != "[true,true]" {
				t.Errorf("%s at %d: the four parts do not fill the band, or it is not as wide as the crumb: %s", path, size, fill)
			}
		}
		// The screen a person is on takes the ink's ground: a dark one in the
		// light (W1), a light one with dark letters in the dark (W2). The 404's
		// page marks no screen, so there is nothing to read there.
		if e2eNavText(t, ctx, `String(document.querySelectorAll('.scr.on').length>0)`) != "true" {
			continue
		}
		groundDarker := e2eNavText(t, ctx, `String((function(){function lum(c){var m=c.match(/[0-9.]+/g).map(Number).slice(0,3).map(function(v){v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);});return 0.2126*m[0]+0.7152*m[1]+0.0722*m[2];}var s=getComputedStyle(document.querySelector('.scr.on'));return lum(s.backgroundColor)<lum(s.color);})())`)
		want := "true"
		if dark {
			want = "false"
		}
		if groundDarker != want {
			t.Errorf("%s at %d: the screen a person is on is not drawn with the %s ground", path, size, map[bool]string{true: "dark", false: "light"}[!dark])
		}
	}
}

// e2eNavTabs is W3・W4's own line: the profile screen's tabs read 一覧 /
// 割り当て 22 / YAML and the role screen's read 一覧 / 割り当て 19 / YAML.
func e2eNavTabs(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	read := func(path, ready, want string) {
		t.Helper()
		e2eRun(t, ctx, chromedp.Navigate(base+path))
		e2eNavWait(t, ctx, ready)
		got := e2eNavText(t, ctx, `JSON.stringify([...document.querySelectorAll('#tabs .tab')].map(function(e){return e.textContent.replace(/\s+/g,'');}))`)
		if got != want {
			t.Errorf("%s: the tabs read %s, want %s", path, got, want)
		}
	}
	read("/profiles", `!!document.querySelector('#tabs .tab')`, `["一覧","割り当て22","YAML"]`)
	read("/roles?open=builder", `!!document.querySelector('#rows .role.open .panel')`, `["一覧","割り当て19","YAML"]`)
}

// e2eNavExplicitTheme is D1: data-theme="light" and data-theme="dark" look the
// same as the OS's own light and dark at every one of W1〜W6's widths.
func e2eNavExplicitTheme(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, size := range []int64{2000, 1440, 500} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 1000))

		e2eNavScheme(t, ctx, false)
		e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
		e2eNavWait(t, ctx, `!!document.querySelector('.screens .scr.on')`)
		osLight := e2eNavSnapshot(t, ctx)
		e2eTheme(t, ctx, "light")
		if got := e2eNavSnapshot(t, ctx); got != osLight {
			t.Errorf("%d: data-theme=light looks different from the light:\n%s\nvs\n%s", size, got, osLight)
		}

		e2eNavScheme(t, ctx, true)
		e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
		e2eNavWait(t, ctx, `!!document.querySelector('.screens .scr.on')`)
		osDark := e2eNavSnapshot(t, ctx)
		e2eNavScheme(t, ctx, false)
		e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
		e2eNavWait(t, ctx, `!!document.querySelector('.screens .scr.on')`)
		e2eTheme(t, ctx, "dark")
		if got := e2eNavSnapshot(t, ctx); got != osDark {
			t.Errorf("%d: data-theme=dark looks different from the dark:\n%s\nvs\n%s", size, got, osDark)
		}
		e2eTheme(t, ctx, "")
	}
}

// e2eNavSnapshot is what the band looks like: the page's ground and ink, the
// band's own ground and layout, and the ink and ground of each part.
func e2eNavSnapshot(t *testing.T, ctx context.Context) string {
	t.Helper()
	return e2eNavText(t, ctx, `JSON.stringify((function(){var cs=getComputedStyle;var b=cs(document.body);var band=document.querySelector('.screens');var out={bg:b.backgroundColor,ink:b.color,display:cs(band).display,cols:cs(band).gridTemplateColumns,band:cs(band).backgroundColor,scrs:[...band.querySelectorAll('.scr')].map(function(e){var c=cs(e);return c.color+'|'+c.backgroundColor;})};var on=document.querySelector('.scr.on');if(on){var c=cs(on);out.on=c.backgroundColor+'|'+c.color;}return out;})())`)
}

// e2eNavFirstTab is S3: each screen's first tab reads 一覧 and carries no number,
// and the screen's own word and number stand in the band only — never a second
// time among the tabs.
func e2eNavFirstTab(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, path := range []string{"/profiles", "/roles", "/teams", "/activity"} {
		e2eRun(t, ctx, chromedp.Navigate(base+path))
		e2eNavWait(t, ctx, `!!document.querySelector('.tabs .tab')`)
		first := e2eNavText(t, ctx, `JSON.stringify((function(){var t=document.querySelector('.tabs .tab');return [t.textContent.replace(/\s+/g,''),!!t.querySelector('.n')];})())`)
		if first != `["一覧",false]` {
			t.Errorf("%s: the first tab reads %s, want 一覧 with no number", path, first)
		}
		// The band's own word and number for the screen a person is on, split
		// where the number starts.
		on := e2eNavText(t, ctx, `(document.querySelector('.screens .scr.on')||{}).textContent.replace(/\s+/g,'')`)
		if on == "" {
			t.Errorf("%s: the band marks no screen as the one a person is on", path)
			continue
		}
		word := strings.TrimRight(on, "0123456789")
		num := on[len(word):]
		tabs := e2eNavText(t, ctx, `[...document.querySelectorAll('.tabs .tab')].map(function(e){return e.textContent.replace(/\s+/g,'');}).join(',')`)
		if strings.Contains(tabs, word) {
			t.Errorf("%s: the band's word %q stands among the tabs too: %s", path, word, tabs)
		}
		if num != "" && strings.Contains(tabs, num) {
			t.Errorf("%s: the band's number %s stands among the tabs too: %s", path, num, tabs)
		}
	}
}

// e2eNavNoOldWords is S4: 「層」「layer」「担当」「役割」「版」stand nowhere on a
// screen that carries the band.
func e2eNavNoOldWords(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	ready := map[string]string{
		"/profiles":        `!!document.querySelector('#rows .row')`,
		"/profiles/assign": `!!document.querySelector('.team')`,
		"/profiles/yaml":   `!!document.querySelector('.yed')`,
		"/roles":           `!!document.querySelector('#rows .role')`,
		"/roles/assign":    `!!document.querySelector('.teams .team')`,
		"/roles/yaml":      `!!document.querySelector('.yed')`,
		"/teams":           `!!document.querySelector('#view .tdef')`,
		"/activity":        `!!document.querySelector('#list')`,
		"/nope":            `!!document.querySelector('.gone')`,
	}
	for _, path := range []string{"/profiles", "/profiles/assign", "/profiles/yaml", "/roles", "/roles/assign", "/roles/yaml", "/teams", "/activity", "/nope"} {
		e2eRun(t, ctx, chromedp.Navigate(base+path))
		e2eNavWait(t, ctx, ready[path])
		for _, word := range []string{"層", "layer", "担当", "役割", "版"} {
			if n := e2eNavInt(t, ctx, `document.body.innerText.split(`+strconv.Quote(word)+`).length-1`); n != 0 {
				t.Errorf("%s holds %q %d times", path, word, n)
			}
		}
	}
}

// e2eNavN2 is N2: the band's 活動 and チーム定義 walk to their screens.
func e2eNavN2(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, ctx, `!!document.querySelector('a.scr[data-screen="activity"]')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="activity"]`)
	e2eNavWaitPath(t, ctx, "/activity")
	e2eNavWait(t, ctx, `!!document.querySelector('#list')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="teams"]`)
	e2eNavWaitPath(t, ctx, "/teams")
	e2eNavWait(t, ctx, `!!document.querySelector('#view .tdef')`)
}

// e2eNavN3 is N3: the screen a person is on is a span with no href, so pressing
// it changes nothing.
func e2eNavN3(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, ctx, `!!document.querySelector('.scr.on')`)
	on := e2eNavText(t, ctx, `JSON.stringify((function(){var e=document.querySelector('.scr.on');return [e.tagName,e.dataset.screen,e.hasAttribute('href'),e.getAttribute('aria-current')];})())`)
	if on != `["SPAN","profiles",false,"page"]` {
		t.Errorf("the screen a person is on reads %s, want a span with no href and aria-current", on)
	}
	e2eNavClick(t, ctx, `.scr.on`)
	e2eNavWaitPath(t, ctx, "/profiles")
	if screen := e2eNavText(t, ctx, `(document.querySelector('.scr.on')||{}).dataset.screen`); screen != "profiles" {
		t.Errorf("pressing the screen a person is on moved to %q, want nothing", screen)
	}
}

// e2eNavN4 is N4: hergy goes to the root, which carries a person to O1's
// destination — /teams with no team of this project running — with nothing open.
func e2eNavN4(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open .panel')`)
	e2eNavClick(t, ctx, `.crumb a.mark`)
	e2eNavWaitPath(t, ctx, "/teams")
	e2eNavWait(t, ctx, `!!document.querySelector('#view .tdef')`)
	if open := e2eNavInt(t, ctx, `document.querySelectorAll('#view .tdef.open').length`); open != 0 {
		t.Errorf("hergy left %d team definitions open, want none", open)
	}
	if search := e2eNavText(t, ctx, `location.search`); search != "" {
		t.Errorf("hergy landed with %q in the address, want the plain list", search)
	}
	if tab := e2eNavText(t, ctx, `(document.querySelector('.tabs .tab.on')||document.querySelector('.tabs .tab')||{}).textContent.replace(/\s+/g,'')`); tab != "一覧" {
		t.Errorf("hergy lands on the %q tab, want 一覧", tab)
	}
}

// e2eNavN5 is N5: a row opened on the role screen comes back opened, and a
// screen left on its 割り当て tab comes back on that tab.
func e2eNavN5(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles?open=builder"))
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open .panel')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles")
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles?open=builder")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows .role.open .panel')`)

	e2eRun(t, ctx, chromedp.Navigate(base+"/roles/assign"))
	e2eNavWait(t, ctx, `!!document.querySelector('.teams .team')`)
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles")
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles/assign")
	e2eNavWait(t, ctx, `!!document.querySelector('.teams .team')`)
}

// e2eNavN10 is N10: the keyboard reaches a band link that is not the screen a
// person is on, the focus ring shows, and Enter opens it like N1. It works in a
// tab of its own, so the band points its links at the plain screens (a tab that
// was not there when a place was remembered).
func e2eNavN10(t *testing.T, ctx context.Context, base string, browser context.Context) {
	t.Helper()
	c10, stop := e2eTab(t, browser, 60*time.Second)
	defer stop()
	e2eRun(t, c10, chromedp.Navigate(base+"/profiles"))
	e2eNavWait(t, c10, `!!document.querySelector('a.scr[data-screen="roles"]')`)
	e2eNavFocus(t, c10, `a.scr[data-screen="roles"]`)
	if outline := e2eNavText(t, c10, `getComputedStyle(document.querySelector('a.scr[data-screen="roles"]')).outlineWidth`); outline != "2px" {
		t.Errorf("the focused band link has an outline of %s, want 2px", outline)
	}
	e2eKey(t, c10, kb.Enter)
	e2eNavWait(t, c10, `!!document.querySelector('.scr.on') && document.querySelector('.scr.on').dataset.screen === 'roles'`)
	if path := e2eNavText(t, c10, `location.pathname`); path != "/roles" {
		t.Errorf("after Enter the page is at %q, want /roles", path)
	}
}

// e2eNavCarryPick is N6's own walk: the core team's impl-a is picked to
// opus-deep on the profile screen's 割り当て tab, and the person walks to the
// role screen with the band — the profile screen keeps a mark, the role screen
// shows nothing changed.
func e2eNavCarryPick(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/profiles/assign"))
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	e2eNavSelect(t, ctx, navPick+" select", "opus-deep")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(navPick)+`).classList.contains('changed')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .scr[data-screen="profiles"] .dot'))`); dot != "true" {
		t.Errorf("a pick made and not saved left no mark on the profile screen")
	}
	e2eNavClick(t, ctx, `a.scr[data-screen="roles"]`)
	e2eNavWaitPath(t, ctx, "/roles")
	e2eNavWait(t, ctx, `!!document.querySelector('#rows')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .scr[data-screen="profiles"] .dot'))`); dot != "true" {
		t.Errorf("the carried pick left no mark on the profile screen")
	}
	if changed := e2eNavInt(t, ctx, `document.querySelectorAll('.teams .arow.changed').length`); changed != 0 {
		t.Errorf("the role screen shows %d changed picks, want none", changed)
	}
}

// e2eNavN7 is N7: walking back with the band puts the pick where it was, drawn
// as the changed screen's blue dashed edge with the core card's head reading
// 1 人, and the mark stands on the screen a person is on in that screen's own
// colour — and goes when the pick is saved.
func e2eNavN7(t *testing.T, ctx context.Context, cwd string) {
	t.Helper()
	_, base, stop := e2eNavServe(t, cwd)
	defer stop()
	e2eNavCarryPick(t, ctx, base)

	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(navPick)+`).classList.contains('changed')`)
	if v := e2eNavText(t, ctx, `document.querySelector(`+strconv.Quote(navPick+" select")+`).value`); v != "opus-deep" {
		t.Errorf("the carried pick reads %q, want opus-deep", v)
	}
	dashed := e2eNavText(t, ctx, `JSON.stringify((function(){var e=document.querySelector(`+strconv.Quote(navPick+" select")+`);var s=getComputedStyle(e);var p=document.createElement('i');p.style.backgroundColor='var(--changed)';document.body.appendChild(p);var tok=getComputedStyle(p).backgroundColor;p.remove();return [s.borderTopStyle,s.borderTopColor===tok];})())`)
	if dashed != `["dashed",true]` {
		t.Errorf("the carried pick is not drawn with the changed screen's edge: %s", dashed)
	}
	if n := e2eNavText(t, ctx, `((document.querySelector('.team[data-g="core"] .tside .dirty')||{}).textContent||'')`); n != "1 人" {
		t.Errorf("the core card's head reads %q, want 1 人", n)
	}
	ownColour := e2eNavText(t, ctx, `JSON.stringify((function(){var d=document.querySelector('.scr.on .dot');if(!d)return false;var p=document.createElement('i');p.style.backgroundColor='var(--accent-soft)';document.body.appendChild(p);var tok=getComputedStyle(p).backgroundColor;p.remove();return getComputedStyle(d).backgroundColor===tok;})())`)
	if ownColour != "true" {
		t.Errorf("the mark on the screen a person is on is not that screen's own colour: %s", ownColour)
	}

	e2eNavClick(t, ctx, `.team[data-g="core"] [data-act="g-save"]`)
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"] .ok')`)
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("the band still marks a screen after the pick was saved")
	}
}

// e2eNavN8 is N8: the draft is not put back when the file moved outside while
// the profile screen was not open, and the band's mark goes with it.
func e2eNavN8(t *testing.T, ctx context.Context, cwd string) {
	t.Helper()
	f, base, stop := e2eNavServe(t, cwd)
	defer stop()
	e2eNavCarryPick(t, ctx, base)

	team := filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml")
	writeFile(t, team, strings.Replace(readFile(t, team), "注文 API を作り直す", "外で書き換えた使命", 1))

	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if n := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); n != 0 {
		t.Errorf("a draft made before the file moved outside was put back (%d picks), want none", n)
	}
	if dot := e2eNavText(t, ctx, `String(!!document.querySelector('.screens .dot'))`); dot != "false" {
		t.Errorf("the band still marks a screen after the draft was dropped")
	}
}

// e2eNavN9 is N9: a reload of the profile screen puts nothing back — only the
// band's own walk carries a draft, and the walk that did carry it spent it.
func e2eNavN9(t *testing.T, ctx context.Context, cwd string) {
	t.Helper()
	_, base, stop := e2eNavServe(t, cwd)
	defer stop()
	e2eNavCarryPick(t, ctx, base)

	// The band's walk back puts the carried draft where it was.
	e2eNavClick(t, ctx, `a.scr[data-screen="profiles"]`)
	e2eNavWaitPath(t, ctx, "/profiles/assign")
	e2eNavWait(t, ctx, `document.querySelector(`+strconv.Quote(navPick)+`).classList.contains('changed')`)

	// A reload does not carry it a second time.
	e2eRun(t, ctx, chromedp.Reload())
	e2eNavWait(t, ctx, `!!document.querySelector('.team[data-g="core"]')`)
	if n := e2eNavInt(t, ctx, `document.querySelectorAll('.arow.changed').length`); n != 0 {
		t.Errorf("a reload put %d changed picks back, want none", n)
	}
}
