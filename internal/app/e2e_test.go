//go:build e2e

package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/options"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestE2E walks the screens in a real headless Chrome (設計 8 節の E2E の行). It
// runs the serve the way a person does: the command with --no-open, a working
// XDG, and a fake herdr, and then drives the pages with chromedp. It is behind
// the e2e build tag, which the e2e gate of go run ./tools/check passes.
//
// Chrome missing fails the run in CI and skips it on a laptop with one line
// (d-a8b1 の W3).
func TestE2E(t *testing.T) {
	chrome := chromeBinary(t)
	f := newFixture(t)
	initLocales(t)
	// The global config says the run is in Japanese, so the scaffold and the
	// screens both print the words the test walks.
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	mustRun(t, "new", "demo")
	// A second profile, so the kind beside the profile pick has something to
	// follow (I7).
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n  e2e-kind:\n    kind: codex\n")
	f.runningHerdr(t, "", "")

	base, stop := startServe(t, f)
	defer stop()

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.Headless)...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var mu sync.Mutex
	var console []string
	chromedp.ListenTarget(browserCtx, func(event any) {
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

	ctx, cancel := context.WithTimeout(browserCtx, 3*time.Minute)
	defer cancel()
	e2eRun(t, ctx, chromedp.Navigate(base+"/"))

	t.Run("bands", func(t *testing.T) { e2eBands(t, ctx, base) })
	t.Run("main line", func(t *testing.T) { e2eMainLine(t, ctx, base) })
	t.Run("close again escape back", func(t *testing.T) { e2eClose(t, ctx, base) })
	t.Run("mission", func(t *testing.T) { e2eMission(t, ctx, base) })
	t.Run("members and workers", func(t *testing.T) { e2eMembers(t, ctx, base) })
	t.Run("turns", func(t *testing.T) { e2eTurns(t, ctx, base) })
	t.Run("draft", func(t *testing.T) { e2eDraft(t, ctx, base) })
	t.Run("pressable", func(t *testing.T) { e2ePressable(t, ctx, base) })
	t.Run("widths", func(t *testing.T) { e2eWidths(t, ctx, base) })
	t.Run("keyboard", func(t *testing.T) { e2eKeyboard(t, ctx, base) })

	mu.Lock()
	defer mu.Unlock()
	if len(console) > 0 {
		t.Errorf("the pages logged %d console errors: %v", len(console), console)
	}
}

// e2eBands is B1・B2: each band's page opens with what it is, and the band the
// person is in is marked.
func e2eBands(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	cases := []struct{ path, lead, band string }{
		{"/", "チーム定義", "チーム定義"},
		{"/activity", "活動", "活動"},
		{"/roles", "ロール", "ロール"},
		{"/profiles", "プロファイル", "プロファイル"},
	}
	for _, c := range cases {
		e2eRun(t, ctx, chromedp.Navigate(base+c.path), chromedp.WaitReady("main h1"))
		var lead, band string
		e2eRun(t, ctx,
			chromedp.Evaluate(`document.querySelector("main h1").textContent.trim()`, &lead),
			chromedp.Evaluate(`(document.querySelector('nav.band a[aria-current="page"]')||{}).textContent.trim() || ""`, &band),
		)
		if !strings.HasPrefix(lead, c.lead) {
			t.Errorf("%s opens with %q, want %q", c.path, lead, c.lead)
		}
		if band != c.band {
			t.Errorf("%s marks the band %q, want %q", c.path, band, c.band)
		}
	}

	// L1: pressing a band moves the mark and the page's own title, even though
	// only the region the server swaps is replaced.
	e2eRun(t, ctx, chromedp.Navigate(base+"/"), chromedp.WaitReady("main h1"))
	e2eRun(t, ctx, chromedp.Click(`nav.band a[href="/activity"]`))
	e2eWaitText(t, ctx, "main h1", "活動")
	var band, title string
	e2eRun(t, ctx,
		chromedp.Evaluate(`(document.querySelector('nav.band a[aria-current="page"]')||{}).textContent.trim() || ""`, &band),
		chromedp.Evaluate(`document.title`, &title),
	)
	if band != "活動" {
		t.Errorf("after pressing the activity band the mark is %q", band)
	}
	if !strings.Contains(title, "活動") {
		t.Errorf("after pressing the activity band the title is %q", title)
	}
}

// e2eMainLine is A3: the band leads to the list, a row to the field, in two
// presses, without the figure.
func e2eMainLine(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx,
		chromedp.Navigate(base+"/definitions"),
		chromedp.WaitReady("table.list"),
		chromedp.Click(`table.list a[href="/definitions/demo"]`),
		chromedp.WaitReady(".mission"),
	)
	if got := e2eURL(t, ctx); got != base+"/definitions/demo" {
		t.Fatalf("the row opened %q, want the definition", got)
	}
	e2eRun(t, ctx, chromedp.Click(`.detail a[href="/definitions/demo/members/lead"]`))
	e2eWaitField(t, ctx, "メンバー lead")
}

// e2eClose is B3: the field closes with its own button, with the same node
// pressed again, with Escape, and with the browser's back.
func e2eClose(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	open := func() {
		e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo/members/lead"))
		e2eWaitField(t, ctx, "メンバー lead")
	}

	open()
	e2eRun(t, ctx, chromedp.Click("#panel .close"))
	e2eWaitClosed(t, ctx, "the close button")
	if got := e2eURL(t, ctx); got != base+"/definitions/demo" {
		t.Errorf("closing landed on %q, want the definition", got)
	}

	// The same figure node pressed again closes the field it opened.
	e2eRun(t, ctx, chromedp.Click(`.fig a[href="/definitions/demo/members/lead"]`))
	e2eWaitField(t, ctx, "メンバー lead")
	e2eRun(t, ctx, chromedp.Click(`.fig a[href="/definitions/demo"]`))
	e2eWaitClosed(t, ctx, "the same node again")

	// Escape closes.
	open()
	e2eEscape(t, ctx)
	e2eWaitClosed(t, ctx, "Escape")

	// The browser's back returns to the page the field was opened from.
	e2eRun(t, ctx, chromedp.Navigate(base+"/roles"), chromedp.WaitReady("main h1"))
	open()
	e2eRun(t, ctx, chromedp.Evaluate(`window.history.back()`, nil))
	e2eWaitURL(t, ctx, base+"/roles")
}

// e2eMission is C1〜C3: the field is writable from the start, a change shows the
// buttons, revert undoes it, and a save writes the mission and shows the saved
// line, keeping the place.
func e2eMission(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo"), chromedp.WaitReady(".mission"))
	before := e2eMissionValue(t, ctx)
	if before == "" {
		t.Fatalf("the definition opens with no mission")
	}
	if e2eChangeButtons(t, ctx) {
		t.Errorf("the save buttons show before anything changed")
	}
	e2eSet(t, ctx, "E2E の使命")
	if !e2eChangeButtons(t, ctx) {
		t.Fatalf("typing the mission shows no save button")
	}
	// Revert puts the file's own line back.
	e2eRun(t, ctx, chromedp.Click(".mission [data-revert]"))
	if value := e2eMissionValue(t, ctx); value != before {
		t.Errorf("revert left %q, want the file's own line %q", value, before)
	}
	// Write and save.
	e2eSet(t, ctx, "E2E の使命")
	e2eRun(t, ctx, chromedp.Click(".mission button[type=submit]"), chromedp.WaitVisible(".saved"))
	if e2eChangeButtons(t, ctx) {
		t.Errorf("the save buttons still show after the save")
	}
	var saved string
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelector('.saved').textContent`, &saved))
	if !strings.Contains(saved, "書きました") {
		t.Errorf("the save line is %q", saved)
	}
	if got := e2eURL(t, ctx); got != base+"/definitions/demo" {
		t.Errorf("the save moved to %q, want to stay", got)
	}
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo"), chromedp.WaitReady(".mission"))
	if value := e2eMissionValue(t, ctx); value != "E2E の使命" {
		t.Errorf("the mission on disk is %q, want the saved one", value)
	}
}

// e2eMembers is the writable field of this round (C1〜C3, I6・I7): a member's
// role is picked from the start, a change shows the buttons, revert puts the
// file's own value back, and a save writes it and shows the saved line without
// moving the place. The kind follows the profile pick, and the press that takes
// a worker away says what the hire loses before anything is written.
func e2eMembers(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	role := `form.edit[data-key="role"]`
	profile := `form.edit[data-key="profile"]`
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo/members/lead"), chromedp.WaitReady(role))
	if e2eActions(t, ctx, role) {
		t.Errorf("the save buttons show before anything changed")
	}
	if got := e2eKind(t, ctx, role); got != "" {
		t.Errorf("a role field shows a kind: %q", got)
	}

	// T1: a pick says what it would do before it is written.
	e2ePick(t, ctx, `select[name="role"]`, "builder")
	e2eWaitText(t, ctx, role+" .save-to", "lead のプロンプトの『ロールの文書』が変わります")
	if !e2eActions(t, ctx, role) {
		t.Fatalf("picking another role shows no save button")
	}
	if got := e2eValue(t, ctx, `select[name="role"]`); got != "builder" {
		t.Errorf("the field reads %q after the pick, want the pick", got)
	}

	// 取り消す leaves the pick: the field is read again without it, so the line
	// about it does not stand over a value that is no longer chosen.
	e2eRun(t, ctx, chromedp.Click(role+` [data-revert]`), chromedp.WaitReady(role))
	e2eWaitURL(t, ctx, base+"/definitions/demo/members/lead")
	if got := e2eValue(t, ctx, `select[name="role"]`); got != "lead" {
		t.Errorf("revert left %q, want the file's own role", got)
	}
	var pending bool
	e2eRun(t, ctx, chromedp.Evaluate(`!!document.querySelector('`+role+` .save-to')`, &pending))
	if pending {
		t.Errorf("the line about the pick is still there after 取り消す")
	}

	// The kind of the profile follows the pick, as its attribute (I7).
	if got := e2eKind(t, ctx, profile); got != "claude" {
		t.Errorf("the kind beside the profile is %q, want claude", got)
	}
	e2ePick(t, ctx, `select[name="profile"]`, "e2e-kind")
	e2eWaitText(t, ctx, profile+" .save-to", "lead の起動のしかたが変わります")
	if got := e2eKind(t, ctx, profile); got != "codex" {
		t.Errorf("the kind beside the picked profile is %q, want codex", got)
	}
	e2eRun(t, ctx, chromedp.Click(profile+` [data-revert]`), chromedp.WaitReady(profile))

	e2ePick(t, ctx, `select[name="role"]`, "builder")
	e2eRun(t, ctx, chromedp.Click(role+` button[type=submit]`), chromedp.WaitVisible(".panel .saved"))
	if e2eActions(t, ctx, role) {
		t.Errorf("the save buttons still show after the save")
	}
	var saved string
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelector('.panel .saved').textContent`, &saved))
	if !strings.Contains(saved, "書きました") || !strings.Contains(saved, "プロンプト") {
		t.Errorf("the saved line is %q", saved)
	}
	if got := e2eURL(t, ctx); got != base+"/definitions/demo/members/lead" {
		t.Errorf("the save moved to %q, want to stay", got)
	}
	if got := e2eValue(t, ctx, `select[name="role"]`); got != "builder" {
		t.Errorf("the field reads %q after the save, want the written role", got)
	}

	// Hiring a worker, and the press that takes it away: the line comes first.
	e2eType(t, ctx, `form.edit[data-key="worker-add"] input[name="purpose"]`, "e2e")
	e2eRun(t, ctx, chromedp.Click(`form.edit[data-key="worker-add"] button[type=submit]`), chromedp.WaitVisible(".panel .saved"))
	e2eRun(t, ctx, chromedp.Click(`a[href="/definitions/demo/members/lead?remove=e2e"]`), chromedp.WaitVisible(".panel .confirm"))
	e2eWaitText(t, ctx, ".panel .save-to", "lead のプロンプトの『呼べるワーカー』から e2e が消えます")
	if got := e2eURL(t, ctx); !strings.Contains(got, "remove=e2e") {
		t.Errorf("the press moved to %q, want the field with the removal pending", got)
	}
	e2eRun(t, ctx, chromedp.Click(".panel .confirm a.btn"))
	e2eWaitURL(t, ctx, base+"/definitions/demo/members/lead")
	e2eRun(t, ctx, chromedp.WaitReady(`form.edit[data-key="worker-add"]`))

	// A half-written pick survives opening another field (C5).
	e2ePick(t, ctx, `select[name="role"]`, "lead")
	e2eRun(t, ctx, chromedp.Click(`.detail a[href="/definitions/demo/members/builder"]`), chromedp.WaitReady(`form.edit[data-key="role"]`))
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo/members/lead"), chromedp.WaitReady(`form.edit[data-key="role"]`))
	if got := e2eValue(t, ctx, `select[name="role"]`); got != "lead" {
		t.Errorf("the half-written pick was lost: the field reads %q", got)
	}
}

// e2eTurns is the turn round (設計 4 節): opening a party's field offers the "＋"
// handle on the parties it does not hand the turn to yet; pressing it shows the
// line and the dashed edge; saving adds the arrow; the new arrow's own field
// chooses the removal, which shows what leaves and takes the arrow away again.
func e2eTurns(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	arrows := func(t *testing.T) int {
		t.Helper()
		var n int
		e2eRun(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.fig path.edge[data-from]:not([data-from=""])').length`, &n))
		return n
	}

	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo/members/builder"), chromedp.WaitReady(`form.edit[data-key="role"]`))
	before := arrows(t)
	if before != 4 {
		t.Fatalf("the demo definition draws %d turn arrows, want 4", before)
	}
	// The handle stands on the human (builder does not hand the turn to it yet)
	// and not on lead (it already does).
	var handle string
	e2eRun(t, ctx, chromedp.Evaluate(`(document.querySelector('.fig a.handle[href*="to=human"]')||{}).getAttribute('href')||""`, &handle))
	if handle == "" {
		t.Fatalf("the open party's field offers no handle to the human")
	}
	var toLead int
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.fig a.handle[href*="to=lead"]').length`, &toLead))
	if toLead != 0 {
		t.Errorf("the figure offers a handle to a party the node already hands the turn to")
	}
	// D3: the handle can be reached with the keyboard.
	var focusable bool
	e2eRun(t, ctx, chromedp.Evaluate(
		`(function(){var h=document.querySelector('.fig a.handle');h.focus();return document.activeElement===h;})()`, &focusable))
	if !focusable {
		t.Errorf("the handle cannot be reached with the keyboard")
	}

	e2eRun(t, ctx, chromedp.Click(`.fig a.handle[href*="to=human"]`))
	e2eWaitText(t, ctx, `form.edit[data-key="turn-add"] .save-to`, "builder のプロンプトの『ターンを渡す相手』に人間が入ります")
	var pending int
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.fig path.pending-add').length`, &pending))
	if pending != 1 {
		t.Fatalf("the chosen edge is not drawn dashed before the write")
	}
	if !e2eActions(t, ctx, `form.edit[data-key="turn-add"]`) {
		t.Fatalf("the chosen turn shows no save button")
	}

	e2eRun(t, ctx, chromedp.Click(`form.edit[data-key="turn-add"] button[type=submit]`), chromedp.WaitVisible(".panel .saved"))
	if got := arrows(t); got != before+1 {
		t.Errorf("after the save the figure draws %d arrows, want %d", got, before+1)
	}
	var saved string
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelector('.panel .saved').textContent`, &saved))
	if !strings.Contains(saved, "ターン builder → 人間") {
		t.Errorf("the saved line is %q", saved)
	}

	// The new arrow's own field chooses the removal, which lands on the party's
	// field once it is written. The arrow is pressed the way a person presses
	// it (the click goes to the path's own link).
	e2eRun(t, ctx, chromedp.Evaluate(
		`document.querySelector('.fig path[data-from="builder"][data-to="human"]').parentElement.dispatchEvent(new MouseEvent("click",{bubbles:true,cancelable:true}))`, nil))
	e2eWaitField(t, ctx, "builder → 人間")
	e2eRun(t, ctx, chromedp.Click(`a[href="/definitions/demo/turns/builder/human?turn=human"]`), chromedp.WaitVisible(".panel .confirm"))
	e2eWaitText(t, ctx, ".panel .confirm .save-to", "builder のプロンプトの『ターンを渡す相手』から人間が消えます")
	var faded int
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.fig path.pending-remove').length`, &faded))
	if faded != 1 {
		t.Fatalf("the edge chosen to be taken away is not drawn faint")
	}
	e2eRun(t, ctx, chromedp.Click(".panel .confirm button[type=submit]"), chromedp.WaitVisible(".panel .saved"))
	e2eWaitField(t, ctx, "メンバー builder")
	if got := arrows(t); got != before {
		t.Errorf("after the removal the figure draws %d arrows, want %d", got, before)
	}
}

// e2eDraft is C5: a change not saved survives opening another field and coming
// back.
func e2eDraft(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo"), chromedp.WaitReady(".mission"))
	e2eSet(t, ctx, "書きかけの使命")
	e2eRun(t, ctx, chromedp.Click(`.detail a[href="/definitions/demo/members/lead"]`))
	e2eWaitField(t, ctx, "メンバー lead")
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo"), chromedp.WaitReady(".mission"))
	if value := e2eMissionValue(t, ctx); value != "書きかけの使命" {
		t.Errorf("the draft was lost: the field reads %q", value)
	}
}

// e2ePressable is B4: nothing shows the pointer cursor without something to
// press.
func e2ePressable(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, path := range []string{"/", "/definitions/demo", "/definitions/demo/members/lead", "/activity", "/roles", "/profiles"} {
		e2eRun(t, ctx, chromedp.Navigate(base+path), chromedp.WaitReady("main"))
		var dead []string
		script := `(function(){
			var out=[];
			document.querySelectorAll('main *').forEach(function(el){
				if(getComputedStyle(el).cursor!=='pointer') return;
				if(el.closest('a,button,summary,input,select,textarea,label')) return;
				out.push(el.tagName+'.'+(el.className||''));
			});
			return out;
		})()`
		e2eRun(t, ctx, chromedp.Evaluate(script, &dead))
		if len(dead) > 0 {
			t.Errorf("%s: %d elements show the pointer cursor with nothing to press: %v", path, len(dead), dead)
		}
	}
}

// e2eWidths is D2: at 1440, 1000 and 500 the page does not scroll sideways.
func e2eWidths(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	for _, size := range []int64{1440, 1000, 500} {
		e2eRun(t, ctx, chromedp.EmulateViewport(size, 900))
		for _, path := range []string{"/", "/definitions/demo", "/definitions/demo/members/lead", "/activity"} {
			e2eRun(t, ctx, chromedp.Navigate(base+path), chromedp.WaitReady("main"))
			var over bool
			e2eRun(t, ctx, chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth + 1`, &over))
			if over {
				t.Errorf("%s at %d: the page scrolls sideways", path, size)
			}
		}
	}
}

// e2eKeyboard is D3: Tab reaches the main line, and Enter opens what it reached.
func e2eKeyboard(t *testing.T, ctx context.Context, base string) {
	t.Helper()
	e2eRun(t, ctx, chromedp.Navigate(base+"/definitions/demo"), chromedp.WaitReady(".mission"))
	found := false
	for i := 0; i < 80; i++ {
		e2eRun(t, ctx, chromedp.KeyEvent("\t"))
		var href string
		e2eRun(t, ctx, chromedp.Evaluate(`document.activeElement.getAttribute('href') || ""`, &href))
		if strings.HasSuffix(href, "/definitions/demo/members/lead") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Tab never reached the member's link")
	}
	e2eRun(t, ctx, chromedp.KeyEvent("\r"))
	e2eWaitField(t, ctx, "メンバー lead")
}

// e2eEscape presses Escape in the page, the key the close script listens for.
func e2eEscape(t *testing.T, ctx context.Context) {
	t.Helper()
	e2eRun(t, ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		down := input.DispatchKeyEventParams{Type: input.KeyRawDown, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27}
		if err := down.Do(ctx); err != nil {
			return err
		}
		up := input.DispatchKeyEventParams{Type: input.KeyUp, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27}
		return up.Do(ctx)
	}))
}

// e2eChangeButtons is whether the save and revert buttons are drawn (C1: their
// being hidden is a rendering, not an attribute).
func e2eChangeButtons(t *testing.T, ctx context.Context) bool {
	t.Helper()
	var shown bool
	e2eRun(t, ctx, chromedp.Evaluate(
		`getComputedStyle(document.querySelector('.mission [data-change]')).display !== 'none'`, &shown))
	return shown
}

// e2eActions is whether one form's save group is drawn (C1).
func e2eActions(t *testing.T, ctx context.Context, form string) bool {
	t.Helper()
	var shown bool
	e2eRun(t, ctx, chromedp.Evaluate(
		`getComputedStyle(document.querySelector('`+form+` [data-change]')).display !== 'none'`, &shown))
	return shown
}

// e2ePick picks one option of a select and fires the event a person's picking
// fires.
func e2ePick(t *testing.T, ctx context.Context, selector, value string) {
	t.Helper()
	script := `(function(){var el=document.querySelector('` + selector + `');el.value="` + value + `";el.dispatchEvent(new Event("change",{bubbles:true}));})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eType writes one line into a text field and fires the event a person's
// typing fires.
func e2eType(t *testing.T, ctx context.Context, selector, text string) {
	t.Helper()
	script := `(function(){var el=document.querySelector('` + selector + `');el.value="` + text + `";el.dispatchEvent(new Event("input",{bubbles:true}));})()`
	e2eRun(t, ctx, chromedp.Evaluate(script, nil))
}

// e2eValue is what one control reads.
func e2eValue(t *testing.T, ctx context.Context, selector string) string {
	t.Helper()
	var value string
	e2eRun(t, ctx, chromedp.Evaluate(`document.querySelector('`+selector+`').value`, &value))
	return value
}

// e2eKind is the kind shown beside one form's profile pick (I7).
func e2eKind(t *testing.T, ctx context.Context, form string) string {
	t.Helper()
	var kind string
	e2eRun(t, ctx, chromedp.Evaluate(
		`((document.querySelector('`+form+` [data-kind-show]')||{}).textContent||"").trim()`, &kind))
	return kind
}

// e2eWaitText waits until one element's text starts with one line.
func e2eWaitText(t *testing.T, ctx context.Context, selector, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var got string
		e2eRun(t, ctx, chromedp.Evaluate(`(document.querySelector('`+selector+`')||{}).textContent || ""`, &got))
		if strings.HasPrefix(strings.TrimSpace(got), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never read %q", selector, want)
}

// e2eSet clears the mission field and types one line into it, firing the input
// events a person's typing fires.
func e2eSet(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	clear := `(function(){var el=document.getElementById('mission');el.value='';el.dispatchEvent(new Event('input',{bubbles:true}));})()`
	e2eRun(t, ctx, chromedp.Evaluate(clear, nil))
	if text != "" {
		e2eRun(t, ctx, chromedp.SendKeys("#mission", text))
	}
}

// e2eMissionValue is what the mission field shows.
func e2eMissionValue(t *testing.T, ctx context.Context) string {
	t.Helper()
	var value string
	e2eRun(t, ctx, chromedp.Evaluate(`document.getElementById('mission').value`, &value))
	return value
}

// e2eRun runs the actions, failing the test on the first error.
func e2eRun(t *testing.T, ctx context.Context, actions ...chromedp.Action) {
	t.Helper()
	if err := chromedp.Run(ctx, actions...); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
}

// e2eURL is the page's address, without a trailing slash.
func e2eURL(t *testing.T, ctx context.Context) string {
	t.Helper()
	var loc string
	e2eRun(t, ctx, chromedp.Evaluate(`window.location.href`, &loc))
	if len(loc) > 1 && strings.HasSuffix(loc, "/") {
		loc = loc[:len(loc)-1]
	}
	return loc
}

// e2eWaitURL waits until the page reaches one address.
func e2eWaitURL(t *testing.T, ctx context.Context, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e2eURL(t, ctx) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the page never reached %q (it is at %q)", want, e2eURL(t, ctx))
}

// e2eWaitField waits until the right panel holds one field.
func e2eWaitField(t *testing.T, ctx context.Context, title string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var got string
		e2eRun(t, ctx, chromedp.Evaluate(`(document.querySelector("#panel .panel-title")||{}).textContent || ""`, &got))
		if strings.TrimSpace(got) == title {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the field %q never opened", title)
}

// e2eWaitClosed waits until the right panel holds no field.
func e2eWaitClosed(t *testing.T, ctx context.Context, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var open bool
		e2eRun(t, ctx, chromedp.Evaluate(`!!document.querySelector("#panel .panel-title")`, &open))
		if !open {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s left the field open", what)
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
