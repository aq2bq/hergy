package ops

import "time"

type deliveryPlan = DeliveryPlan
type plannedMember = PlannedMember

// Apply reads the drift and available operations.
func (a *Reader) Apply() ApplyList { return a.plan(a.clock()).Apply }

// ApplyCount reads the number of members with drift.
func (a *Reader) ApplyCount() int { return a.Apply().Count }
func (a *Reader) plan(now time.Time) deliveryPlan {
	return ComputeDelivery(DeliveryInput{Views: a.projectViews(), Session: sessionOf(a.readHerdr(now)), Now: now})
}
