package ops

import (
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
)

// PendingLaunch is a locked view of a serve's launch. The adapter owns storage
// and DTO conversion; the stop operation owns when a launch is cancelled.
type PendingLaunch struct {
	Team   string
	Phase  string
	Cancel func()
	Finish func(phase string, at time.Time)
}

// finishLaunchFor ends starts belonging to the stopped run. Multiple-team
// apply operations still have no team name; their selection moves in C4.
func finishLaunchFor(team domainname.Seen, visit func(func(PendingLaunch)), now func() time.Time) {
	if visit == nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	visit(func(launch PendingLaunch) {
		if launch.Team != team.String() || (launch.Phase != "starting" && launch.Phase != "awaiting") {
			return
		}
		if launch.Cancel != nil {
			launch.Cancel()
		}
		launch.Finish("cancelled", now())
	})
}
