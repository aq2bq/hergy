// Package i18n keeps hergy's own words in one file per language and picks the
// one to use. The files are embedded in the binary; nothing is read at runtime.
//
// The language is chosen as --lang, then the global settings, then en. Init
// loads every language and stops if their key sets differ, so a missing or
// extra key is caught before the first message is printed.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed locales/*.json
var files embed.FS

// Available is the languages hergy ships, in preference order. The first one is
// the key-set reference and the fallback.
var Available = []string{"en", "ja"}

// Fallback is the language used when neither --lang nor the settings say one.
const Fallback = "en"

var (
	mu       sync.RWMutex
	catalogs = map[string]map[string]string{}
	current  map[string]string
	language = Fallback
)

// Init loads every locale and checks that their keys agree, then uses Fallback.
func Init() error {
	for _, lang := range Available {
		messages, err := load(lang)
		if err != nil {
			return err
		}
		catalogs[lang] = messages
	}
	if err := checkKeys(); err != nil {
		return err
	}
	use(Fallback)
	return nil
}

func load(lang string) (map[string]string, error) {
	raw, err := files.ReadFile("locales/" + lang + ".json")
	if err != nil {
		return nil, fmt.Errorf("cannot read locale %s: %w", lang, err)
	}
	messages := map[string]string{}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("locale %s is not valid JSON: %w", lang, err)
	}
	return messages, nil
}

// checkKeys fails when two languages disagree on their key set, naming each
// missing or extra key so the fix is obvious.
func checkKeys() error {
	base := Available[0]
	for _, lang := range Available[1:] {
		missing, extra := diffKeys(catalogs[base], catalogs[lang])
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, fmt.Sprintf("missing in %s: %s", lang, strings.Join(missing, ", ")))
		}
		if len(extra) > 0 {
			parts = append(parts, fmt.Sprintf("extra in %s: %s", lang, strings.Join(extra, ", ")))
		}
		if len(parts) > 0 {
			return fmt.Errorf("locales %s and %s disagree: %s", base, lang, strings.Join(parts, "; "))
		}
	}
	return nil
}

func diffKeys(a, b map[string]string) (missing, extra []string) {
	for key := range a {
		if _, ok := b[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range b {
		if _, ok := a[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

func use(lang string) {
	mu.Lock()
	defer mu.Unlock()
	current = catalogs[lang]
	language = lang
}

// Resolve picks a language from the --lang value and the configured language
// (from the global settings): the explicit value wins, then the configured one,
// then Fallback. An unknown language is an error that names the ones that exist.
func Resolve(explicit, configured string) (string, error) {
	wanted := explicit
	if wanted == "" {
		wanted = configured
	}
	if wanted == "" {
		return Fallback, nil
	}
	if has(Available, wanted) {
		return wanted, nil
	}
	return "", fmt.Errorf(T("error.unknown_language"), wanted, strings.Join(Available, ", "))
}

// Use switches to a language Resolve returned.
func Use(lang string) error {
	if _, ok := catalogs[lang]; !ok {
		return fmt.Errorf("locale %s is not loaded", lang)
	}
	use(lang)
	return nil
}

// Language is the language in use.
func Language() string {
	mu.RLock()
	defer mu.RUnlock()
	return language
}

// T is the message for a key in the language in use. A key with no message
// shows as [[key]] rather than an empty string.
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	text, ok := current[key]
	if !ok {
		return "[[" + key + "]]"
	}
	return text
}

// Tf is T with fmt.Sprintf.
func Tf(key string, args ...any) string { return fmt.Sprintf(T(key), args...) }

// Errorf is fmt.Errorf with the format taken from a key. It exists so callers
// never pass a non-constant format to fmt.Errorf, which go vet rejects.
func Errorf(key string, args ...any) error {
	return errors.New(Tf(key, args...))
}

func has(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
