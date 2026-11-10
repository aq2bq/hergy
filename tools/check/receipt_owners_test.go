package main

import (
	"strings"
	"testing"
)

const receiptRulesFixture = `enforce: true
confirmed_send:
  owner: internal/herdr/receipt.go#Herdr.ConfirmPrompt
  wait_builder: internal/herdr/herdr.go#Herdr.PromptWait
  single_attempt:
    - internal/app/turns.go#turnWatcher.ask
`

func TestConfirmedSendOwners(t *testing.T) {
	for _, tc := range []struct {
		name, path, code string
		bad              bool
	}{
		{"wait-builder", "internal/herdr/herdr.go", `func (h *Herdr) PromptWait() { h.port.JSON("agent", "prompt", target, text, "--wait") }`, false},
		{"owner", "internal/herdr/receipt.go", `func (h *Herdr) ConfirmPrompt() { for range 2 { h.PromptWait(target, text, 5000) } }`, false},
		{"ask-exception", "internal/app/turns.go", `func (w *turnWatcher) ask() { w.herdr.PromptWait(target, text, 5000) }`, false},
		{"deliver-is-no-exception", "internal/app/turns.go", `func (w *turnWatcher) deliver() { w.herdr.PromptWait(target, text, 5000) }`, true},
		{"other-function-in-owner-file", "internal/herdr/receipt.go", `func (h *Herdr) another() { h.PromptWait(target, text, 5000) }`, true},
		{"wait-method-value", "internal/app/copy.go", `func send() { confirmed := h.PromptWait; confirmed(target,text,5000) }`, true},
		{"raw-send", "internal/app/copy.go", `func send() { p.JSON("agent", "prompt", target, text, "--wait") }`, true},
		{"assembled-send", "internal/app/copy.go", `func send() { args := []string{"agent", "prompt", target, text}; args = append(args,"--wait"); p.JSON(args...) }`, true},
		{"builder-other-function", "internal/herdr/herdr.go", `func another() { p.JSON("agent", "prompt", target, text, "--wait") }`, true},
		{"package-arguments", "internal/app/copy.go", `var args = []string{"agent", "prompt", "--wait"}`, true},
		{"duplicate-owner", "internal/app/copy.go", `func (h *Herdr) ConfirmPrompt() {}`, true},
		{"unconfirmed-tell", "internal/app/copy.go", `func tell() { h.Prompt(target,text); p.JSON("agent","prompt",target,text) }`, false},
		{"comment-and-text", "internal/app/copy.go", `// h.PromptWait(target,text,5000)
var description = "agent prompt --wait is confirmed"`, false},
		{"test-stand-in", "internal/app/copy_test.go", `func fake() { p.JSON("agent","prompt",target,text,"--wait") }`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, ownersPath, receiptRulesFixture)
			writeTree(t, root, tc.path, "package standin\n"+tc.code+"\n")
			err := (&runner{root: root}).ownersGate()
			if (err != nil) != tc.bad {
				t.Fatalf("ownersGate=%v, forbidden=%t", err, tc.bad)
			}
			if tc.bad && !strings.Contains(err.Error(), "持ち主") {
				t.Fatalf("wrong failure: %v", err)
			}
		})
	}
}
