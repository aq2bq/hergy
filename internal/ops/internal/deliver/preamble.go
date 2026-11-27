package deliver

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Preamble is what hergy puts before a bootstrap prompt it hands again: that the
// team's definition changed, what changed, and that the prompt is the one to
// follow from now on while the work and conversation continue (d-36ef). A send
// with nothing changed gets no preamble, so a plain reprompt reads as before.
func Preamble(changes []Change) i18n.AgentText {
	if len(changes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(string(i18n.Handed("apply.preamble_lead")))
	b.WriteString("\n")
	for _, change := range changes {
		b.WriteString("\n- ")
		b.WriteString(string(change.Handed))
	}
	b.WriteString("\n\n")
	b.WriteString(string(i18n.Handed("apply.preamble_close")))
	b.WriteString("\n\n")
	return i18n.AgentText(b.String())
}
