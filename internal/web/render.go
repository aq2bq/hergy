package web

import (
	"html/template"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
)

// t is one message of the locale in use.
func t(key string) string { return i18n.T(key) }

// tf is one message with its values.
func tf(key string, args ...any) string { return i18n.Tf(key, args...) }

// Funcs are the template functions the two screens render with: the four kinds
// the profile screen offers as buttons, and the rest in a select (r-c683).
func Funcs() template.FuncMap {
	return template.FuncMap{
		"splitKinds": splitKinds,
		"otherKinds": otherKinds,
		"kindClass":  p2KindClass,
		"hasPrefix":  strings.HasPrefix,
	}
}

// splitKinds is the four kinds the profile screen offers as buttons.
func splitKinds() []string {
	return []string{"claude", "codex", "opencode", "gemini"}
}

// otherKinds is every kind the select offers: the kinds outside the four, and
// the profile's own kind when it is outside them too, so a file that names a
// kind herdr has added since still shows what it runs.
func otherKinds(current string) []string {
	base := map[string]bool{"claude": true, "codex": true, "opencode": true, "gemini": true}
	out := make([]string, 0, len(profile.Kinds()))
	for _, kind := range profile.Kinds() {
		if !base[kind] {
			out = append(out, kind)
		}
	}
	if current != "" && !base[current] && !slices.Contains(out, current) {
		out = append(out, current)
	}
	return out
}
