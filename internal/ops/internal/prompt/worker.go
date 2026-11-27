package prompt

import (
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// WorkerInput is what one worker's prompt is built from: the team, the member
// that hires it, its purpose and herdr name, and the task the hire gives it.
type WorkerInput struct {
	Team     string
	Lead     string // the hiring member's herdr name
	LeadRole string
	Purpose  string
	Name     string // the worker's herdr name, "<team>-<worker>"
	Task     string
}

// Worker builds a hired worker's bootstrap prompt and its segments. The worker
// receives no rules (d-0e08): its prompt is hergy's own template, the task the
// hire gave it, and the contact guide. Joining the segments' texts gives Text
// exactly (r-4cf9).
func Worker(in WorkerInput) (*Built, error) {
	simple := map[string]i18n.AgentText{
		"team":      i18n.AgentText(in.Team),
		"lead":      i18n.AgentText(in.Lead),
		"lead_role": i18n.AgentText(in.LeadRole),
		"purpose":   i18n.AgentText(in.Purpose),
		"name":      i18n.AgentText(in.Name),
	}
	parts := map[string]func() []agentPart{
		"actor": func() []agentPart {
			return oneSegment("actor", i18n.Handedf("prompt.actor", in.Name), definition.PromptOrigin{Kind: "hergy"})
		},
		"task": func() []agentPart {
			return []agentPart{{
				kind: "task", text: i18n.AgentText(in.Task),
				origin: definition.PromptOrigin{Kind: "hire", Name: in.Name},
			}}
		},
		"herdr": func() []agentPart {
			return oneSegment("herdr", HerdrWorker(), definition.PromptOrigin{Kind: "hergy"})
		},
	}
	text, segments, err := expandSegments(WorkerTemplate(), definition.PromptOrigin{Kind: "hergy"}, simple, parts)
	if err != nil {
		return nil, err
	}
	return &Built{Text: text, Segments: segments}, nil
}
