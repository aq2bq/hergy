package activity

import (
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
)

// The words the screen says about time (6 A4). They live here rather than in the
// screen because the server makes every one of them: the screen writes no time
// itself (spec-activity 3, V1).

// Length is how long a state has lasted, as a person reads it ("25 分", "1 時間
// 5 分", "30 秒"), with "以上" saying it is a lower bound.
func Length(d time.Duration, lower bool) string {
	return durationText(d, lower)
}

// Clock is a moment as the screen writes it: today reads "15:04", another day
// reads "1/2 15:04", so a time from yesterday is not read as today's. The moment
// is read in the reader's own time zone, which is the clock the reader is
// looking at. A time nobody knows has no words.
func Clock(now, at time.Time) string {
	if at.IsZero() {
		return ""
	}
	local := at.Local()
	if local.Year() == now.Local().Year() && local.YearDay() == now.Local().YearDay() {
		return local.Format("15:04")
	}
	return local.Format("1/2 15:04")
}

// SinceAt is when a state began, as the row reads it: "13:34 から", or "13:34
// より前" when the state was already there when the watching began, so the real
// moment is somewhere before it (6 A2, A4).
func SinceAt(now, at time.Time, lower bool) string {
	clock := Clock(now, at)
	if clock == "" {
		return ""
	}
	if lower {
		return i18n.Tf("activity.since_before", clock)
	}
	return i18n.Tf("activity.since_at", clock)
}

// Slug is the state as the API writes it (spec-activity 3): answer, turn,
// stalled or working.
func (s State) Slug() string {
	switch s {
	case WaitingAnswer:
		return "answer"
	case WaitingTurn:
		return "turn"
	case Stalled:
		return "stalled"
	default:
		return "working"
	}
}

// Slug is one node's state as the API writes it (spec-activity 3): blocked,
// handed, active, idle or absent.
func (s NodeState) Slug() string {
	switch s {
	case Blocked:
		return "blocked"
	case HandedOver:
		return "handed"
	case Active:
		return "active"
	case Idle:
		return "idle"
	default:
		return "absent"
	}
}
