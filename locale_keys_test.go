package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This file holds the words the web screens name to the locales: a key a screen
// writes must be one the locale holds, or the page shows the key itself
// ([[label.human]]), which is what a person must never read (E1, 歩き walk-1b の
// L2).

// keyInSource is one locale key a screen names, in Go ("web.band.activity") or
// in a template ('{{t "label.human"}}').
var keyInSource = regexp.MustCompile(`"(web|label)\.[a-z0-9_.]+"`)

// TestWebScreensNameKeysTheLocalesHold: every web.* and label.* key the web
// screens name is in both locales.
func TestWebScreensNameKeysTheLocalesHold(t *testing.T) {
	held := map[string]map[string]bool{}
	for _, lang := range []string{"en", "ja"} {
		raw, err := os.ReadFile(filepath.Join("internal", "i18n", "locales", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		keys := make(map[string]bool, len(messages))
		for key := range messages {
			keys[key] = true
		}
		held[lang] = keys
	}

	walk := func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		readable := strings.HasSuffix(name, ".html") || (strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"))
		if !readable {
			return nil
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, quoted := range keyInSource.FindAllString(string(b), -1) {
			key := strings.Trim(quoted, `"`)
			for lang, keys := range held {
				if !keys[key] {
					t.Errorf("%s names %s, which the %s locale does not hold", file, key, lang)
				}
			}
		}
		return nil
	}
	if err := filepath.WalkDir(filepath.Join("internal", "web"), walk); err != nil {
		t.Fatal(err)
	}
}
