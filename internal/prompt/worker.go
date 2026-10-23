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
	simple := map[string]func() string{
		"team":      func() string { return in.Team },
		"lead":      func() string { return in.Lead },
		"lead_role": func() string { return in.LeadRole },
		"purpose":   func() string { return in.Purpose },
		"name":      func() string { return in.Name },
	}
	parts := map[string]func() []definition.PromptSegment{
		"actor": func() []definition.PromptSegment {
			return []definition.PromptSegment{{
				Kind: "actor", Text: i18n.Tf("prompt.actor", in.Name),
				Origin: definition.PromptOrigin{Kind: "hergy"},
			}}
		},
		"task": func() []definition.PromptSegment {
			return []definition.PromptSegment{{
				Kind: "task", Text: in.Task,
				Origin: definition.PromptOrigin{Kind: "hire", Name: in.Name},
			}}
		},
		"herdr": func() []definition.PromptSegment {
			return []definition.PromptSegment{{
				Kind: "herdr", Text: HerdrWorker(i18n.Tf("prompt.self_intro_worker", in.Team, in.Purpose, in.Name)),
				Origin: definition.PromptOrigin{Kind: "hergy"},
			}}
		},
	}
	text, segments, err := ExpandSegments(WorkerTemplate(), definition.PromptOrigin{Kind: "hergy"}, simple, parts)
	if err != nil {
		return nil, err
	}
	return &Built{Text: text, Segments: segments}, nil
}
