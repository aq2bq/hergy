package ops

import (
	"errors"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/harness"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/turns"
)

// TurnDoneInput carries hergy turn-done <team> <member> [payload]: the harness says a turn ended
// (r-378e). The payload is Codex's one JSON argument or, for Claude Code's Stop
// hook and the opencode plugin, stdin. The signal goes to the team's signal
// directory for hergy turns to take on its next tick. A Codex member's own
// notify, which hergy's replaced for that process, is called afterwards with
// the same argument (d-a70c).
// TurnDoneInput carries the harness payload and its observed turn identity.
type TurnDoneInput struct {
	Team, Member, Payload string
	Raw                   []byte
	Now                   func() time.Time
}

// TurnDone records a harness signal and preserves the user's notify hook.
func TurnDone(in TurnDoneInput) error {
	text, source, err := harness.Payload(in.Payload, in.Raw)
	if errors.Is(err, harness.ErrNotATurn) {
		return nil // the harness's own side turn (a title): nothing to hand on
	}
	if err != nil {
		return i18n.Errorf("error.turn_done_no_payload", err.Error())
	}
	now := in.Now
	if now == nil {
		now = time.Now
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	sig := turns.Signal{Member: in.Member, At: now(), Text: text, Source: source}
	if err := turns.WriteSignal(turns.SignalDir(dir, in.Team), sig); err != nil {
		return err
	}
	if source == "codex" {
		harness.NotifyThen(os.Getenv(harness.EnvNotifyThen), in.Payload)
	}
	return nil
}
