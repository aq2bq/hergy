package app

import (
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/deliver"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is what hergy hands a member when it sends the bootstrap prompt
// again: the prompt the current definition builds, headed with what changed since
// the prompt the member last read (d-36ef). The preamble is what tells the agent
// that the definition moved and that the text replaces the earlier bootstrap
// prompt without restarting its work.

// promptsToResend is the prompts a resend hands to the members: each member's
// current prompt, with the preamble when the launch record shows a change. A
// member the record does not know, or one whose prompt is the prompt it already
// read, is sent as it is.
func promptsToResend(cat *catalog.Catalog, project string, r *resolved, members []definition.Start) []definition.Start {
	out := append([]definition.Start(nil), members...)
	log, err := launchlog.Open()
	if err != nil {
		return out
	}
	recorded := teamsRecorded(latestMemberRecords(), project)[r.team.Name]
	for i := range out {
		record, ok := recorded[out[i].Name]
		if !ok {
			continue
		}
		recordedMember, ok := recordedMemberOf(record, out[i])
		if !ok {
			continue
		}
		old, err := log.Text(recordedMember.Prompt)
		if err != nil {
			continue
		}
		changes := deliver.Compare(string(old), oldDocs(log, record), out[i], docsOfStart(cat, out[i]))
		if len(changes) == 0 {
			continue
		}
		out[i].Prompt = deliver.Preamble(changes) + out[i].Prompt
	}
	return out
}

// docsOfStart is the rule documents one built member read, as the comparison
// reads them.
func docsOfStart(cat *catalog.Catalog, start definition.Start) []deliver.Doc {
	var out []deliver.Doc
	add := func(doc source.Doc) {
		text, err := source.Disk{}.Read(doc)
		if err != nil {
			return
		}
		out = append(out, deliver.Doc{Path: doc.Path, Display: cat.Display(doc.Path), Text: string(text)})
	}
	if start.HasRole {
		add(start.RoleDoc)
	}
	for _, doc := range start.Read {
		add(doc)
	}
	return out
}
