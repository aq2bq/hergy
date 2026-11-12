package web

import (
	"github.com/aq2bq/hergy/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops"
)

// ApplyDTO converts an operation plan to the HTTP response shape.
func ApplyDTO(in ops.ApplyList) ApplyList {
	out := ApplyList{Herdr: ActivityNote{OK: in.Herdr.OK, Text: in.Herdr.Text}, Count: in.Count, Teams: []ApplyTeam{}}
	for _, team := range in.Teams {
		row := ApplyTeam{Team: team.Team, Definition: team.Definition, Project: team.Project, ProjectName: team.ProjectName, Error: team.Error, Notes: team.Notes, Members: []ApplyMember{}}
		for _, member := range team.Members {
			changes := member.Changes
			if changes == nil {
				changes = []deliver.Change{}
			}
			row.Members = append(row.Members, ApplyMember{Name: member.Name, Short: member.Short, Role: member.Role, State: member.State, How: string(member.How), Changes: changes, CanApply: member.CanApply, WhyNot: member.WhyNot, ConversationLost: member.ConversationLost})
		}
		out.Teams = append(out.Teams, row)
	}
	return out
}
