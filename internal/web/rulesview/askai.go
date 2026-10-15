package rulesview

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// askAI is the prompt the panel offers to copy: what to ask an AI to find out
// where one rule came from, with the rule's own words filled in (試作 askAi).
// root is where the layer's files live, as a person writes it. What goes in is
// the open rule, that layer's root and the file's display path, and nothing
// else; the reason line is left out of the prompt when the rule has no reason.
func askAI(root string, doc Doc, section int, edit EditRule) string {
	why := ""
	if strings.TrimSpace(edit.Why) != "" {
		why = "\n" + i18n.Tf("web.rules.ask_ai.prompt.why", edit.Why)
	}
	return i18n.Tf(
		"web.rules.ask_ai.prompt",
		strings.TrimSpace(edit.ID),
		doc.Display,
		sectionLabel(doc.Document, section),
		edit.Text,
		why,
		root,
	)
}
