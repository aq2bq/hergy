package web

import (
	"html/template"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// t is one message of the locale in use.
func t(key string) string { return i18n.T(key) }

// tf is one message with its values.
func tf(key string, args ...any) string { return i18n.Tf(key, args...) }

// Funcs are the template functions the frame and the screens render with: t and
// tf (the words), inline (a rule's text or a prompt, as HTML), lang and html.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"t":      i18n.T,
		"tf":     i18n.Tf,
		"inline": Inline,
		"lang":   i18n.Language,
		"html":   HTML,
	}
}

// HTML marks a string this package already rendered as HTML. Only this package's
// own rendering goes through it.
func HTML(rendered string) template.HTML {
	//nolint:gosec // G203: the caller passes this package's own rendering
	return template.HTML(rendered)
}

// Inline turns a rule's text or a prompt into HTML: the whole text is escaped
// first; then only two marks are replaced.
//
//	**bold**  ->  <strong>bold</strong>
//	`code`    ->  <code>code</code>
//
// Everything else stays the characters it is, and line breaks are kept (the
// stylesheet shows them with white-space: pre-wrap).
func Inline(text string) template.HTML {
	escaped := template.HTMLEscapeString(text)
	escaped = replacePairs(escaped, "**", "<strong>", "</strong>")
	escaped = replacePairs(escaped, "`", "<code>", "</code>")
	return HTML(escaped)
}

// replacePairs turns every pair of one mark into an open and a close tag. An
// unpaired mark is left as the character it is.
func replacePairs(text, mark, open, end string) string {
	if mark == "" {
		return text
	}
	var b strings.Builder
	for {
		i := strings.Index(text, mark)
		if i < 0 {
			b.WriteString(text)
			return b.String()
		}
		rest := text[i+len(mark):]
		before, after, ok := strings.Cut(rest, mark)
		if !ok {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:i])
		b.WriteString(open)
		b.WriteString(before)
		b.WriteString(end)
		text = after
	}
}

// firstClause is the first sentence of a rule's text, shown on its card beside
// its id so a card never stands for its id alone (bar.md E1). It cuts at the
// first sentence end, and falls back to the whole text when there is none.
func firstClause(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	for _, mark := range []string{"。", ". "} {
		if i := strings.Index(line, mark); i >= 0 {
			if mark == ". " {
				return line[:i+1]
			}
			return line[:i+len(mark)]
		}
	}
	return line
}

// problemBody is the one paragraph a failed screen shows, with the reason on it.
func problemBody(reason string) template.HTML {
	return HTML(`<p class="problem">` + template.HTMLEscapeString(reason) + `</p>`)
}
