// Package prose reads and writes Markdown definitions without rewriting their text.
package prose

import (
	"os"
	"strings"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/source"
)

// Document keeps the exact text read from a definition.
type Document struct{ Text string }

// Read reads a definition without normalizing whitespace or line endings.
func Read(path string) (Document, error) {
	data, err := os.ReadFile(path)
	return Document{Text: string(data)}, err
}

// Save uses the hash from the caller's read to refuse stale edits.
func (d Document) Save(path, previousHash string) error {
	return source.WriteIfUnchanged(path, previousHash, d.Text)
}

// Title returns the first line without its Markdown heading marker, or fallback
// when that line is empty. It never searches later lines for a title.
func (d Document) Title(fallback string) string {
	line, _, _ := strings.Cut(d.Text, "\n")
	line = strings.TrimSpace(line)
	marker := len(line) - len(strings.TrimLeft(line, "#"))
	if marker > 0 && marker <= 6 && (marker == len(line) || line[marker] == ' ' || line[marker] == '\t') {
		line = strings.TrimSpace(line[marker:])
	}
	if line == "" {
		return fallback
	}
	return line
}

// Chars counts Unicode code points, including whitespace and Markdown syntax.
// CRLF counts as LF, without changing Text.
func (d Document) Chars() int {
	return utf8.RuneCountInString(strings.ReplaceAll(d.Text, "\r\n", "\n"))
}
