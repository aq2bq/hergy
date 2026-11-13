package ops

import (
	"github.com/aq2bq/hergy/internal/definition"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

// This file is what hergy hands a member when it sends the bootstrap prompt
// again: the prompt the current definition builds, headed with what changed since
// the prompt the member last read (d-36ef). The preamble is what tells the agent
// that the definition moved and that the text replaces the earlier bootstrap
// prompt without restarting its work.

// promptsToResend is the prompts a resend hands to the members: each member's
// current prompt, with the preamble when the running state shows a change. A
// member the record does not know, or one whose prompt is the prompt it already
// read, is sent as it is.
func promptsToResend(cat *catalog.Catalog, project domainplace.Project, r *Resolved, members []definition.Start) []definition.Start {
	out := append([]definition.Start(nil), members...)
	store, err := runningstate.Open()
	if err != nil {
		return out
	}
	state, exists, err := store.Read(r.ModelTeamName())
	if err != nil || !exists || state.Launch().Project.Compare(project) != domainplace.Same {
		return out
	}
	recorded := memberReceipts(state)
	for i := range out {
		record, ok := recorded[out[i].Name.String()]
		if !ok {
			continue
		}
		changes := deliver.Compare(r.ModelTeamName(), record.Lang, record.Prompt, oldDocs(record), out[i], deliveryDocs(cat, out[i].Documents))
		if len(changes) == 0 {
			continue
		}
		out[i].Prompt = deliver.Preamble(changes) + out[i].Prompt
	}
	return out
}
