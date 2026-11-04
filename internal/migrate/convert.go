// Package migrate prepares the explicit conversion of legacy YAML definitions.
// It is not connected to CLI commands or normal definition loading.
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
