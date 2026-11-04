// Package migrate prepares the explicit conversion of legacy YAML definitions.
// Normal definition loading does not use the legacy parser.
package migrate

import (
	"github.com/aq2bq/hergy/internal/prose"
	"github.com/aq2bq/hergy/internal/rules"
)

// Convert preserves the old prompt's ordering and layout, without rule IDs.
// Excluded rules and notes remain only in the original YAML backup.
func Convert(text, path string) (prose.Document, error) {
	doc, err := rules.Parse(text, path)
	if err != nil {
		return prose.Document{}, err
	}
	for s := range doc.Sections {
		for r := range doc.Sections[s].Rules {
			doc.Sections[s].Rules[r].ID = ""
		}
	}
	return prose.Document{Text: doc.Markdown()}, nil
}

// LegacyMarkdown is the text an unmigrated snapshot put in its old prompt.
// It is read-only and retains the historical rule IDs.
func LegacyMarkdown(text, path string) (string, error) {
	doc, err := rules.Parse(text, path)
	if err != nil {
		return "", err
	}
	return doc.Markdown(), nil
}
