package ops

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/ops/internal/team"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/status"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
)

// StatusRow is one member or worker observed by the status operation.
type StatusRow status.Row

// Text returns the shared status wording for this read row.
func (r StatusRow) Text() string { return status.Row(r).Text() }

// StatusField is an already-resolved turn label and value.
type StatusField struct{ Label, Value string }

// TeamStatus contains every value a CLI or DTO needs to display one team.
type TeamStatus struct {
	Team             RunningTeam
	Rows             []StatusRow
	Members, Workers int
	Judgement        string
	Turn             []StatusField
	NoWatcher        string
}

// StatusInput gives status the same unresolved team selection and an injectable clock.
type StatusInput struct {
	MemberInput
	Now func() time.Time
}

// Status reads live teams, rows and turn state and updates screen observations.
func Status(ctx context.Context, in StatusInput) ([]TeamStatus, error) {
	if in.Team.String() != "" {
		r, err := resolveMember(in.MemberInput, false)
		if err != nil {
			read := readSession(in.Port)
			if isRunningTeam(read, in.Team.String()) {
				return nil, errors.New(err.Error() + "\n" + i18n.Tf("status.can_down", in.Team.String()))
			}
			return nil, err
		}
		one := RunningTeam{Name: r.Team.Name.Observation(), Definition: r.Team.Definition.Name.String(), Project: in.Env.ProjectRoot, Built: r}
		result, err := statusTeam(ctx, in, one, readSession(in.Port))
		if err != nil {
			return nil, err
		}
		return []TeamStatus{result}, nil
	}
	read := readSession(in.Port)
	if read.Err != nil {
		return nil, read.Err
	}
	running := readRunningTeams(in.Env, read)
	out := make([]TeamStatus, 0, len(running))
	for _, one := range running {
		result, err := statusTeam(ctx, in, one, read)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, nil
}
func isRunningTeam(read session, name string) bool {
	_, ok := team.FindRunning(team.RunningTeams(read.Tabs, currentTeamStates()), name)
	return ok
}
func statusTeam(ctx context.Context, in StatusInput, one RunningTeam, read session) (TeamStatus, error) {
	out := TeamStatus{Team: one}
	if one.Built == nil {
		return out, nil
	}
	rows, err := operationLauncher(ctx, one.Built, in.Timeout, in.Port, in.Observe).Status(one.Built.Starts)
	if err != nil {
		return out, err
	}
	markStatusScreens(rows, in.Now)
	for _, row := range rows {
		out.Rows = append(out.Rows, StatusRow(row))
	}
	out.Members, out.Workers = status.Counts(rows)
	turn, watching := readTeamTurn(one.Name, read.Tabs)
	input := activity.Input{Team: one.Name, Turn: turnInput(turn), Watching: watching, Now: operationTime(in.Now)}
	for _, row := range rows {
		if row.Live {
			input.Agents = append(input.Agents, activity.Agent{Herdr: observedname.Observe(row.Name), Status: row.State})
		}
	}
	out.Judgement = activity.Of(input).State.Text()
	if in.Team.String() != "" {
		out.Turn, out.NoWatcher = turnStatus(one.Name, turn, watching)
	}
	return out, nil
}
func markStatusScreens(rows []status.Row, now func() time.Time) {
	log, err := screenlog.Open()
	if err != nil {
		return
	}
	prev, err := log.Read()
	if err != nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	at := now()
	next := make(map[string]screenlog.Entry, len(prev))
	maps.Copy(next, prev)
	for i := range rows {
		row := &rows[i]
		if row.Screen == "" {
			continue
		}
		entry, seen := prev[row.Name]
		if seen && entry.Hash == row.Screen {
			if row.Live && row.State == "working" {
				minutes := int(at.Sub(entry.At).Minutes())
				row.UnchangedMinutes = &minutes
			}
			continue
		}
		next[row.Name] = screenlog.Entry{Hash: row.Screen, At: at}
	}
	_ = log.Write(next)
}
func turnStatus(name domainname.Seen, st *turns.State, watching bool) ([]StatusField, string) {
	var fields []StatusField
	if st != nil && !st.Since.IsZero() {
		text := ""
		switch {
		case st.Holder == turns.Human:
			why := ""
			if st.Why != "" {
				why = i18n.T("status.turn_why_" + st.Why)
			}
			text = i18n.Tf("status.turn_human", st.From, why)
		case st.Asked[st.Holder].Seq != 0:
			text = i18n.Tf("status.turn_holder_asked", st.Holder, st.From)
		default:
			text = i18n.Tf("status.turn_holder", st.Holder, st.From)
		}
		fields = append(fields, StatusField{Label: i18n.T("status.turn"), Value: text})
		if waits := st.Waits(); len(waits) > 0 {
			fields = append(fields, StatusField{Label: i18n.T("status.turn_waits"), Value: strings.Join(waits, i18n.T("list.separator"))})
		}
	}
	if watching {
		return fields, ""
	}
	return fields, i18n.Tf("status.no_watcher", name)
}
