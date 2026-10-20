//go:build e2e

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// The e2e run drives the screens through chromedp's generic API (v0.14 以降,
// r-36b4): an action is Action[T], an action that returns nothing is
// Action[chromedp.Void], several such actions go through Do, and reading the
// page is chromedp.Run with one chromedp.Evaluate[T]. The older, non-generic
// API — Run with any number of actions, Evaluate with a result pointer — is
// gone, and this file is where the difference is kept: the tests call its parts
// the way they did before, and nothing a test asserts changed with the API.
//
// It drives one Chrome, not one per test (r-a107). Four browsers used
// to be started — one for each e2e test, and one more for the tab the "another
// tab" case opens — and each start was a chance for the whole run to die while
// the browser came up: on a loaded CI machine Chrome missed the 20 second wait
// for its WebSocket URL, and a test failed with "websocket url timeout reached"
// without a page having been driven. One browser is now started for the whole
// run, and each test opens and closes its own tab in it (a tab has a page and a
// sessionStorage of its own, so one test does not read what another left). One
// start also means the wait for it can be a long one.
const (
	// e2eStartDeadline is how long the run waits for its one Chrome to come up.
	// It is one wait for the whole run, so it can be far longer than the 20
	// seconds chromedp waits by default — the number reached on CI (r-a107).
	e2eStartDeadline = 60 * time.Second
	// e2eCloseDeadline is how long the run waits for that browser to close,
	// after its tests have passed. The run is over either way; this only says
	// how long it holds the terminal.
	e2eCloseDeadline = 15 * time.Second
)

// The run's one browser: started the first time a test asks for it, closed once
// by TestMain after the last test (r-a107).
var (
	e2eChromeOnce sync.Once
	e2eChromeCtx  context.Context
	e2eChromeDone func()
	e2eChromeErr  error
)

// TestMain closes the one browser the run drives, after the tests that drove it
// (r-a107). A run that never started a browser — no Chrome on this machine, so
// every e2e test skipped — has nothing to close.
func TestMain(m *testing.M) {
	code := m.Run()
	if e2eChromeDone != nil {
		e2eChromeDone()
	}
	os.Exit(code)
}

// e2eChrome returns the one browser the whole run drives, starting it the first
// time it is asked for; every later test is handed the same one (r-a107). chrome
// is where Chrome is, from chromeBinary. A browser that did not come up is told
// to every test that asks, with the reason Chrome or its allocator gave, so a
// run that fails says why instead of leaving the reader to guess.
func e2eChrome(t *testing.T, chrome string) context.Context {
	t.Helper()
	e2eChromeOnce.Do(func() {
		e2eChromeCtx, e2eChromeDone, e2eChromeErr = e2eStartChrome(chrome)
	})
	if e2eChromeErr != nil {
		t.Fatalf("the e2e run could not start its browser: %v (r-a107)", e2eChromeErr)
	}
	return e2eChromeCtx
}

// e2eStartChrome starts the one browser: an allocator with Chrome's path and a
// long wait for its WebSocket URL, and a browser on it. The Run is what starts
// Chrome — chromedp starts it on the first Run of a context, not when the
// context is made — so it happens here, once, where a failure is one reason to
// tell every test. What it returns closes the browser (and the allocator) once.
func e2eStartChrome(chrome string) (context.Context, func(), error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.Headless,
			chromedp.WSURLReadTimeout(e2eStartDeadline))...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	// done closes the browser and its allocator, and the end of the run calls it
	// whether or not the start worked: a Run that was still waiting when the
	// deadline passed is unblocked by this, not left behind.
	done := func() {
		closed := make(chan struct{})
		go func() {
			cancelBrowser()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(e2eCloseDeadline):
			fmt.Fprintf(os.Stderr, "the e2e browser did not close within %s\n", e2eCloseDeadline)
		}
		cancelAlloc()
	}
	if err := e2eAwaitChrome(browserCtx); err != nil {
		return nil, done, err
	}
	return browserCtx, done, nil
}

// e2eAwaitChrome starts Chrome and waits for it to answer: the first Run of a
// context is what starts the browser and attaches to its first tab, and a Chrome
// that comes up but never answers a call would otherwise hold the whole run
// until go test's own timeout, ten minutes later and with nothing said about
// why. The Run is handed the browser's own context — a deadline on that context
// would end the browser itself, as chromedp says — so the wait is a watch
// beside it, and a Run that goes on after the watch has given up is unblocked
// when the run closes the browser at the end.
func e2eAwaitChrome(browserCtx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- chromedp.Do(browserCtx) }()
	timer := time.NewTimer(e2eStartDeadline)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("Chrome did not come up within %s", e2eStartDeadline)
	}
}

// e2eTab opens one tab in the run's browser and returns a context that drives
// it, with the function that closes the tab — not the browser, which stays up
// for the next test (r-a107). A test works in its own tab, so the page one test
// left behind is not the page the next one finds. life bounds the tab: a page
// that will not answer fails that test instead of the run.
func e2eTab(t *testing.T, browser context.Context, life time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	tabCtx, cancelTab := chromedp.NewContext(browser)
	ctx, cancelLife := context.WithTimeout(tabCtx, life)
	return ctx, func() {
		cancelLife()
		cancelTab()
	}
}

// The e2e run drives the real screens over a real Chrome, so the driver is
// always a little ahead of the person: it can read an element that is not drawn
// yet, press one before the page has loaded, and press a button the screen has
// shut. A person cannot do any of those — reading a missing element throws in
// the page, and pressing a shut button is no press at all (HTML) — so every act
// here waits for the page to offer it, and then acts once (r-3c41).
const (
	// e2eDeadline is how long one act waits for the page to offer it.
	e2eDeadline = 8 * time.Second
	// e2eStep is how long it looks away before looking again.
	e2eStep = 50 * time.Millisecond
)

// e2eWait waits until one expression is true. An expression that throws is not
// an answer — the element it reads may not be drawn yet — so it waits like any
// other "not yet", and says what it kept hearing when it never became true.
func e2eWait(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	e2eReady(t, ctx)
	deadline := time.Now().Add(e2eDeadline)
	why := ""
	for time.Now().Before(deadline) {
		ok, reason, answered := e2eLook[bool](t, ctx, script)
		if answered && ok {
			return
		}
		why = reason
		time.Sleep(e2eStep)
	}
	if why == "" {
		t.Fatalf("never became true: %s", script)
	}
	t.Fatalf("never became true: %s (%s)", script, why)
}

// e2eRead waits until the page can give one expression, and returns it as the
// kind the caller asked for: the text, the number, the truth. The expression is
// tried again while the page cannot answer it yet, and what the page kept saying
// — the throw of an element that is not drawn, the word of a page that is not
// loaded — is the failure's last word.
func e2eRead[T any](t *testing.T, ctx context.Context, script string) T {
	t.Helper()
	e2eReady(t, ctx)
	deadline := time.Now().Add(e2eDeadline)
	why := ""
	for time.Now().Before(deadline) {
		value, reason, answered := e2eLook[T](t, ctx, script)
		if answered {
			return value
		}
		why = reason
		time.Sleep(e2eStep)
	}
	var zero T
	if why == "" {
		why = "the page gave nothing"
	}
	t.Fatalf("the page never gave %s (%s)", script, why)
	return zero
}

// e2ePressWhenEnabled presses the element one expression stands for, and waits
// for the screen to offer it: the element may not be drawn yet, the page may not
// have loaded, and the screen may have shut the button. A press on a shut button
// is no press at all — the click() of a disabled form control returns without
// dispatching (HTML) — and the screen writes a save's shut state from the
// check's answer, which lands after the fields were written (roles 1.2.9), so a
// reading taken one round trip before the press can be the answer for the text
// before the typing. Reading and pressing are one act here, and are tried again
// until the screen takes it (r-3c41).
func e2ePressWhenEnabled(t *testing.T, ctx context.Context, find string) {
	t.Helper()
	e2eReady(t, ctx)
	script := `(function(){var el=` + find + `;if(!el){return 'missing';}` +
		`if(el.disabled){return 'shut';}el.click();return 'pressed';})()`
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

// e2eSet writes one field's value and fires the input event a person's typing
// fires, once the field is there (r-3c41).
func e2eSet(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eReady(t, ctx)
	script := `(function(){var el=` + find + `;if(!el){return 'missing';}` +
		`el.value=` + strconv.Quote(value) + `;el.dispatchEvent(new Event('input',{bubbles:true}));return 'set';})()`
	e2eSetField(t, ctx, script, find)
}

// e2eSelect picks one option of a select and fires the change event a person's
// picking fires, once the select is there (r-3c41).
func e2eSelect(t *testing.T, ctx context.Context, find, value string) {
	t.Helper()
	e2eReady(t, ctx)
	script := `(function(){var el=` + find + `;if(!el){return 'missing';}` +
		`el.value=` + strconv.Quote(value) + `;el.dispatchEvent(new Event('change',{bubbles:true}));return 'picked';})()`
	e2eSetField(t, ctx, script, find)
}

// e2eSetField tries one writing script until the element it writes is there.
func e2eSetField(t *testing.T, ctx context.Context, script, find string) {
	t.Helper()
	deadline := time.Now().Add(e2eDeadline)
	word := ""
	for time.Now().Before(deadline) {
		word = e2eVerdict(t, ctx, script)
		if word != "missing" {
			return
		}
		time.Sleep(e2eStep)
	}
	t.Fatalf("no element for %s", find)
}

// e2eVerdict runs one script that answers a word about what it could do
// ('missing', 'shut', 'pressed', 'set', 'picked'), and returns that word. The
// scripts stand in the page once, so nothing is read and then acted on a round
// trip later. A script that fails — rather than one that has nothing to work on
// — still fails the test.
func e2eVerdict(t *testing.T, ctx context.Context, script string) string {
	t.Helper()
	return e2eValue[string](t, ctx, script)
}

// e2eValue runs one script and returns what it answered, once. It is the
// generic API's reading: chromedp.Run with one chromedp.Evaluate[T]. Nothing is
// waited for here, so a caller that retries (e2eVerdict, e2eSetField) or that
// reads again (e2eLook) does its own waiting.
func e2eValue[T any](t *testing.T, ctx context.Context, script string) T {
	t.Helper()
	value, err := chromedp.Run(ctx, chromedp.Evaluate[T](script))
	if err != nil {
		t.Fatalf("chromedp: %v", err)
	}
	return value
}

// e2eScript runs one script in the page for its effect, and fails the test when
// the page cannot run it. The script's answer is thrown away (chromedp.Void is
// the generic API's "no value").
func e2eScript(t *testing.T, ctx context.Context, script string) {
	t.Helper()
	e2eReady(t, ctx)
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[chromedp.Void](script)); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
}

// e2eTheme sets the theme the screens read, the way a person's choice lands in
// the page: the dataset attribute, or none at all, where the screens fall back
// to the browser's own preference (r-3c41 の 明暗の確かめ).
func e2eTheme(t *testing.T, ctx context.Context, theme string) {
	t.Helper()
	if theme == "" {
		e2eScript(t, ctx, `delete document.documentElement.dataset.theme`)
		return
	}
	e2eScript(t, ctx, `document.documentElement.dataset.theme=`+strconv.Quote(theme))
}

// e2eDark asks the page to read prefers-color-scheme as dark, which is what a
// machine set to a dark theme reports to the screens.
func e2eDark(t *testing.T, ctx context.Context) {
	t.Helper()
	e2eCall(t, ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}},
	})
}

// e2eCall runs one protocol command on the tab (chromedp.Call is the generic
// API's way to the protocol from a part that holds the context, not an action).
func e2eCall[P, R any](t *testing.T, ctx context.Context, cmd cdp.Command[P, R], params P) {
	t.Helper()
	if _, err := chromedp.Call(ctx, cmd, params); err != nil {
		t.Fatalf("chromedp: %v", err)
	}
}

// e2eConsole holds what the page said in red while a test drove it, so the test
// can check at the end that a screen logged nothing. It is the run's one way to
// listen: chromedp.Events is the generic API's replacement for ListenTarget
// (chromedp v0.14 以降), and each event is a cdp.Event value (runtime.
// ConsoleAPICalled, runtime.ExceptionThrown).
type e2eConsole struct {
	mu    sync.Mutex
	lines []string
}

// e2eListen starts listening on one tab's context and returns what it hears;
// exceptions says whether an uncaught exception is counted as well as a console
// error (an uncaught exception is not a console call, but it is the same promise
// to the person: nothing red in the drawer). It must be called before the first
// page loads. Each listener ends with the context, so closing the tab ends it.
func e2eListen(ctx context.Context, exceptions bool) *e2eConsole {
	heard := &e2eConsole{}
	go func() {
		for called, err := range chromedp.Events(ctx, runtime.ConsoleAPICalled) {
			if err != nil {
				return
			}
			if called.Type != "error" {
				continue
			}
			var parts []string
			for _, arg := range called.Args {
				parts = append(parts, fmt.Sprintf("%v %v %v", arg.Value, arg.Description, arg.Preview))
			}
			heard.add(strings.Join(parts, " | "))
		}
	}()
	if exceptions {
		go func() {
			for thrown, err := range chromedp.Events(ctx, runtime.ExceptionThrown) {
				if err != nil {
					return
				}
				heard.add("uncaught: " + e2eException(thrown.ExceptionDetails))
			}
		}()
	}
	return heard
}

// e2eException renders an uncaught exception: its text, and the error itself
// when the browser sent one. The generic API's ExceptionDetails carries the
// details but no longer renders them (chromedp v0.14 以降).
func e2eException(d *runtime.ExceptionDetails) string {
	text := d.Text
	if d.Exception != nil {
		text += ": " + d.Exception.Description
	}
	return text
}

// Lines is what the page said in red, in the order it said it.
func (c *e2eConsole) Lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.lines...)
}

func (c *e2eConsole) add(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

// e2eReady waits until the document has loaded. A person cannot read or press
// before the page is there; a driver can, so every act waits for it. The run
// also waits for it after a move, where the address of the old page is still the
// one the driver reads (e2eNavWaitPath).
func e2eReady(t *testing.T, ctx context.Context) {
	t.Helper()
	deadline := time.Now().Add(e2eDeadline)
	for time.Now().Before(deadline) {
		loaded, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.readyState === 'complete'`))
		if err == nil && loaded {
			return
		}
		time.Sleep(e2eStep)
	}
	t.Fatalf("the page never finished loading")
}

// e2eKey sends one key to whatever has the keyboard, once the page is there
// (r-3c41).
func e2eKey(t *testing.T, ctx context.Context, key string) {
	t.Helper()
	e2eReady(t, ctx)
	e2eRun(t, ctx, chromedp.KeyEvent(key))
}

// e2eLook asks the page one expression, once, and returns what it made of it:
// the value and true, or the reason it has none yet and false. It is the one
// place an expression becomes a Go value, so every reading waits the same way.
func e2eLook[T any](t *testing.T, ctx context.Context, script string) (T, string, bool) {
	t.Helper()
	var zero T
	value, why := e2eAsk(t, ctx, script)
	if value == nil {
		return zero, why, false
	}
	var got T
	if err := json.Unmarshal(value, &got); err != nil {
		t.Fatalf("the page's answer to %s is not a %T: %v", script, got, err)
	}
	return got, "", true
}

// e2eAsk asks the page one question and returns what it answered: the value of
// the expression, or why it has none yet. An expression that throws is one of
// those answers — an element that is not drawn yet throws when it is read — so
// the throw is what the caller waits past, not what ends the test. The driver's
// own failure is the same kind of "not yet": a page that is being replaced has
// no execution context to read (r-3c41).
func e2eAsk(t *testing.T, ctx context.Context, script string) (json.RawMessage, string) {
	t.Helper()
	ask := `(function(){try{return JSON.stringify({v:(` + script + `)});}catch(e){return JSON.stringify({e:String((e&&e.message)||e)});}})()`
	text, err := chromedp.Run(ctx, chromedp.Evaluate[string](ask))
	if err != nil {
		return nil, err.Error()
	}
	var answer struct {
		Value json.RawMessage `json:"v"`
		Why   string          `json:"e"`
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		return nil, "the page's answer is not readable: " + err.Error()
	}
	if answer.Value == nil && answer.Why == "" {
		return nil, "the expression gave nothing"
	}
	return answer.Value, answer.Why
}

// e2eThrottle slows the page down the way a slower machine would, when
// HERGY_E2E_CPU_THROTTLE names a rate (6 is six times slower than this machine).
// It is how the run is held under the windows CI opens: a race the fast machine
// hides shows up here, and the waits above have to hold it shut. Unset, the run
// is as fast as the machine (r-3c41).
func e2eThrottle(t *testing.T, ctx context.Context) {
	t.Helper()
	raw := os.Getenv("HERGY_E2E_CPU_THROTTLE")
	if raw == "" {
		return
	}
	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil || rate <= 0 {
		t.Fatalf("HERGY_E2E_CPU_THROTTLE=%q is not a rate above zero", raw)
	}
	e2eCall(t, ctx, emulation.SetCPUThrottlingRate, emulation.SetCPUThrottlingRateParams{Rate: rate})
}
