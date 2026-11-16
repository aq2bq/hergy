package ops

import (
	"slices"

	"github.com/aq2bq/hergy/internal/activity"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
)

// judgedTeam keeps the badge and turn line on the same reading of the file.
type judgedTeam struct {
	activity.Team
	turn *turns.State
}

func readTeamTurn(name domainname.Seen, tabs []team.Tab) (*turns.State, bool) {
	watching := slices.ContainsFunc(tabs, func(tab team.Tab) bool {
		return slices.ContainsFunc(tab.Panes, func(pane domainname.Seen) bool { return pane.String() == name.Watcher().String() })
	})
	if !watching {
		return nil, false
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return nil, watching
	}
	st, _ := turns.Lookup(turns.Path(dir, name.String()))
	return st, watching
}

func turnInput(st *turns.State) *activity.Turn {
	if st == nil {
		return nil
	}
	return &activity.Turn{Holder: st.Holder, From: st.From, Since: st.Since}
}
