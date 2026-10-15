package web

import (
	"html/template"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Funcs are the template functions a screen renders with: t (one message of the
// locale in use), tf (a message with its values), inline (a rule's text or a
// prompt, as HTML) and lang (the language code). A screen outside this package
// parses its templates with these, so the words come from the locales and text
// is rendered by the one rule below, wherever it is written.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"t":      i18n.T,
		"tf":     i18n.Tf,
		"inline": Inline,
		"lang":   i18n.Language,
	}
}

// HTML marks a string this package already rendered as HTML for a View's Body.
// A screen builds its body with a template, or with this package's rendering, so
// the string passed here is HTML hergy made. Nothing else goes through it.
func HTML(rendered string) template.HTML {
	//nolint:gosec // G203: the caller passes this package's own rendering
	return template.HTML(rendered)
}

// Inline turns the text of a rule or of a prompt into HTML for a page. The whole
// text is escaped first; only then are two marks replaced, and nothing else of
// Markdown is drawn (spec/10 の 4 節):
//
//	**bold**  ->  <strong>bold</strong>
//	`code`    ->  <code>code</code>
//
// Everything else stays the characters it is: a link, an image, a raw tag and an
// unpaired mark all come out escaped and visible. Line breaks are kept as they
// are, and the stylesheet lets them show (white-space: pre-wrap), so a prompt
// reads the way it is sent.
func Inline(text string) template.HTML {
	escaped := template.HTMLEscapeString(text)
	escaped = replacePairs(escaped, "**", "<strong>", "</strong>")
	escaped = replacePairs(escaped, "`", "<code>", "</code>")
	return HTML(escaped)
}

// problemBody is the one paragraph a failed screen shows, with the reason on it.
func problemBody(reason string) template.HTML {
	return HTML(`<p class="problem">` + template.HTMLEscapeString(reason) + `</p>`)
}

// replacePairs turns every pair of one mark into an open and a close tag. An
// unpaired mark is left as the character it is: text that opens ** and never
// closes it shows the marks rather than swallowing what follows.
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
