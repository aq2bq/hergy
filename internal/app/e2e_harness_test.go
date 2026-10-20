//go:build e2e

package app

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

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
	var word string
	e2eRun(t, ctx, chromedp.Evaluate(script, &word))
	return word
}

// e2eReady waits until the document has loaded. A person cannot read or press
// before the page is there; a driver can, so every act waits for it. The run
// also waits for it after a move, where the address of the old page is still the
// one the driver reads (e2eNavWaitPath).
func e2eReady(t *testing.T, ctx context.Context) {
	t.Helper()
	deadline := time.Now().Add(e2eDeadline)
	for time.Now().Before(deadline) {
		var loaded bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.readyState === 'complete'`, &loaded)); err == nil && loaded {
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
	var text string
	if err := chromedp.Run(ctx, chromedp.Evaluate(ask, &text)); err != nil {
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
	e2eRun(t, ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return emulation.SetCPUThrottlingRate(rate).Do(ctx)
	}))
}
