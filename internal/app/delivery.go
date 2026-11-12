package app

import (
	"time"

	"github.com/aq2bq/hergy/internal/deliver"
	"github.com/aq2bq/hergy/internal/ops"
	"github.com/aq2bq/hergy/internal/web"
)

type deliveryPlan = ops.DeliveryPlan
type plannedMember = ops.PlannedMember

func (a *activityRead) Apply() web.ApplyList            { return applyDTO(a.plan(a.clock()).Apply) }
func (a *activityRead) ApplyCount() int                 { return a.Apply().Count }
func (a *activityRead) plan(now time.Time) deliveryPlan { return a.computeDelivery(now) }
func (a *activityRead) computeDelivery(now time.Time) deliveryPlan {
	return ops.ComputeDelivery(ops.DeliveryInput{Views: a.projectViews(), Session: sessionOf(a.readHerdr(now)), Now: now})
}
func applyDTO(in ops.ApplyList) web.ApplyList {
	out := web.ApplyList{Herdr: web.ActivityNote{OK: in.Herdr.OK, Text: in.Herdr.Text}, Count: in.Count, Teams: []web.ApplyTeam{}}
	for _, team := range in.Teams {
		row := web.ApplyTeam{Team: team.Team, Definition: team.Definition, Project: team.Project, ProjectName: team.ProjectName, Error: team.Error, Notes: team.Notes, Members: []web.ApplyMember{}}
		for _, member := range team.Members {
			changes := member.Changes
			if changes == nil {
				changes = []deliver.Change{}
			}
			row.Members = append(row.Members, web.ApplyMember{Name: member.Name, Short: member.Short, Role: member.Role, State: member.State, How: string(member.How), Changes: changes, CanApply: member.CanApply, WhyNot: member.WhyNot, ConversationLost: member.ConversationLost})
		}
		out.Teams = append(out.Teams, row)
	}
	return out
}
