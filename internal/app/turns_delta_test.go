package app

import (
	"strings"
	"testing"
)

// TestPaneDeltaIsWhatCameAfterTheTurnStarted: the pane as it stood when the turn
// started is found again in the pane now, and what follows it is the member's
// text; the harness's own prompt box and status line under it are not.
func TestPaneDeltaIsWhatCameAfterTheTurnStarted(t *testing.T) {
	before := strings.Join([]string{
		"⏺ 前のターンの最後の文",
		"",
		"❯ hergy より: dev のターンが終わり、あなたのターンです。",
		"────────────",
		"  ➜ …/hergy main Opus 5.5",
	}, "\n")
	after := strings.Join([]string{
		"古い行はスクロールで消えた",
		"⏺ 前のターンの最後の文",
		"",
		"❯ hergy より: dev のターンが終わり、あなたのターンです。",
		"",
		"⏺ 照合した。2 件通った。",
		"",
		"  残り 1 件は次の段で。",
		"→ dev",
		"",
		"✻ Worked for 14s · done 9:35 PM",
		"────────────",
		"❯ ",
		"  ➜ …/hergy main Opus 5.5",
		"  ⏵⏵ auto mode on (shift+tab to cycle)",
	}, "\n")
	got := paneDelta(before, after)
	want := "⏺ 照合した。2 件通った。\n\n  残り 1 件は次の段で。\n→ dev"
	if got != want {
		t.Errorf("paneDelta =\n%q\nwant\n%q", got, want)
	}
}

// TestPaneDeltaWithNoAnchorTakesThePane: a pane whose earlier end is nowhere to
// be found (the first turn, or a scrollback that moved past it) is taken whole,
// less the chrome.
func TestPaneDeltaWithNoAnchorTakesThePane(t *testing.T) {
	after := "  ┃ Build · DeepSeek V4.1 Flash\n計測done\n\n     Build · DeepSeek V4.1 Flash · 24.3s · 11.7 tok/s\n  ╹▀▀▀▀\n  ~/go/src/hergy:main   465.2K (47%)  ctrl+p commands"
	if got := paneDelta("", after); got != "計測done" {
		t.Errorf("paneDelta = %q, want the one line the agent wrote", got)
	}
	if got := paneDelta("something else entirely\nthat is not there", after); got != "計測done" {
		t.Errorf("paneDelta with a lost anchor = %q, want the pane less its chrome", got)
	}
}

// TestPaneDeltaDropsASidePanel: opencode draws its context panel to the right
// of a wide pane, on the same lines as the agent's words, after a run of blank
// space; the words change between readings, so the panel is neither the text
// handed on nor part of the anchor.
func TestPaneDeltaDropsASidePanel(t *testing.T) {
	before := strings.Join([]string{
		"     依頼はまだ受け取っていません。                               smoke-lead の小さな役",
		"     → 人間                                                        Context",
		"                                                                   11,219 tokens",
		"  ┃                                                                1% used",
	}, "\n")
	after := strings.Join([]string{
		"     依頼はまだ受け取っていません。                               smoke-lead の小さな役",
		"     → 人間                                                        Context",
		"                                                                   11,735 tokens",
		"     dev さん、「計測done」と 1 行だけ書いて止まってください。    1% used",
		"     → dev",
		"     Build · DeepSeek V4.1 Flash · 1.5s · 40.4 tok/s",
		"  ┃",
	}, "\n")
	got := paneDelta(before, after)
	want := "     dev さん、「計測done」と 1 行だけ書いて止まってください。\n     → dev"
	if got != want {
		t.Errorf("paneDelta =\n%q\nwant\n%q", got, want)
	}
}
