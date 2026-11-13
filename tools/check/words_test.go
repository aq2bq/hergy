package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/words"
)

func TestLocaleProjectionPreservesUnownedBytes(t *testing.T) {
	raw := []byte("{\n  \"keep\": \"a & b\",\n  \"owned\": \"old\"\n}\n")
	made, err := projectLocale(raw, map[string]string{"owned": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if string(made) != strings.Replace(string(raw), "old", "new", 1) {
		t.Fatalf("unowned text changed: %s", made)
	}
	if _, err := projectLocale(raw, map[string]string{"missing": "value"}); err == nil {
		t.Fatal("missing destination accepted")
	}
}

func TestWordsGateRejectsStaleOutput(t *testing.T) {
	r := &runner{root: t.TempDir()}
	messages := words.LocaleMessages("en")
	messages["usage"], messages["cheat.people"] = "", ""
	raw, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	tree(t, r.root, map[string]string{
		"internal/i18n/locales/en.json": string(raw), "internal/i18n/locales/ja.json": string(raw),
		"internal/words/terms_generated.go": "stale", "internal/web/src/lib/terms.generated.ts": "stale",
	})
	if err := r.wordsGate(); err == nil {
		t.Fatal("stale output passed comparison")
	}
	r.write = true
	if err := r.wordsGate(); err != nil {
		t.Fatal(err)
	}
	r.write = false
	if err := r.wordsGate(); err != nil {
		t.Fatal(err)
	}
}
