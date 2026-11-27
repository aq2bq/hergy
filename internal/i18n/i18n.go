// Package i18n keeps hergy's own words. The files are embedded in the binary;
// nothing is read at runtime.
//
// What a person reads — help, the cheatsheet, errors, a command's output, the
// sentences the screens are answered with — is English, whatever the settings
// say (d-8e53): T, Tf and Errorf read it. What hergy hands to an agent — the
// bootstrap prompt and its parts, the lines the watcher and hergy tell put in a
// pane — is written in the handed language: --lang, then the global
// config.yaml's lang, then en. Handed, Handedf and Text read that.
//
// Init loads every language and checks that each translation has an English
// message. Only agent text needs translations.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed locales/*.json
var files embed.FS

// Available is the languages hergy ships, in preference order. The first one is
// the key-set reference and the fallback.
var Available = []string{"en", "ja"}

// Fallback is the handed language when neither --lang nor the settings say one.
const Fallback = "en"

// English is the language of everything a person reads.
const English = "en"

var (
	mu       sync.RWMutex
	catalogs = map[string]map[string]string{}
	// current is the English catalog: what T reads.
	current map[string]string
	// handed is the language of the text hergy hands to an agent.
	handed = Fallback
)

// Init loads every locale and selects the fallback language for agent text.
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

// checkKeys rejects translations of unknown messages.
func checkKeys() error {
	for _, lang := range Available[1:] {
		var extra []string
		for key := range catalogs[lang] {
			if _, ok := catalogs[English][key]; !ok {
				extra = append(extra, key)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return fmt.Errorf("locale %s has unknown keys: %s", lang, strings.Join(extra, ", "))
		}
	}
	return nil
}

func use(lang string) {
	mu.Lock()
	defer mu.Unlock()
	current = catalogs[English]
	handed = lang
}

// Resolve picks a language from the --lang value and the configured language
// (from the global config.yaml): the explicit value wins, then the configured one,
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

// Use makes a language Resolve returned the handed language. What a person
// reads stays English.
func Use(lang string) error {
	if _, ok := catalogs[lang]; !ok {
		return fmt.Errorf("locale %s is not loaded", lang)
	}
	use(lang)
	return nil
}

// HandedLanguage is the language of the text hergy hands to an agent, and of
// the starter files hergy new writes.
func HandedLanguage() string {
	mu.RLock()
	defer mu.RUnlock()
	return handed
}

// AgentText is text composed for an agent. Ordinary person-facing messages do
// not satisfy an assembly parameter of this type.
type AgentText string

// Handed is the message for a key in the handed language: a sentence an agent
// reads in its pane.
func Handed(key string) AgentText { return Text(HandedLanguage(), key) }

// Handedf is Handed with fmt.Sprintf.
func Handedf(key string, args ...any) AgentText { return Textf(HandedLanguage(), key, args...) }

// T is the message a person reads for a key, in English. A key with no message
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

// Text is the message for a key in one language: the language a prompt is
// rebuilt in, which is the launch record's, not the handed one now (C9). An
// empty language is the handed one; an unknown language or a key that language
// lacks reads English, so a caller still reads hergy's own words.
func Text(lang, key string) AgentText {
	if lang == "" {
		lang = HandedLanguage()
	}
	mu.RLock()
	messages, known := catalogs[lang]
	if known {
		if text, ok := messages[key]; ok {
			mu.RUnlock()
			return AgentText(text)
		}
	}
	mu.RUnlock()
	return AgentText(T(key))
}

// Textf is Text with fmt.Sprintf.
func Textf(lang, key string, args ...any) AgentText {
	return AgentText(fmt.Sprintf(string(Text(lang, key)), args...))
}

// Errorf is fmt.Errorf with the format taken from a key. It exists so callers
// never pass a non-constant format to fmt.Errorf, which go vet rejects.
func Errorf(key string, args ...any) error {
	return errors.New(Tf(key, args...))
}

func has(list []string, value string) bool {
	return slices.Contains(list, value)
}

// Number groups character counts consistently in both supported languages.
func Number(n int) string {
	text := strconv.Itoa(n)
	for i := len(text) - 3; i > 0 && text[i-1] != '-'; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}
