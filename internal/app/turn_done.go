package app

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/harness"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/turns"
)

// hergy turn-done <team> <member> [payload]: the harness says a turn ended
// (r-378e). The payload is Codex's one JSON argument or, for Claude Code's Stop
// hook and the opencode plugin, stdin. The signal goes to the team's signal
// directory for hergy turns to take on its next tick. A Codex member's own
// notify, which hergy's replaced for that process, is called afterwards with
// the same argument (d-a70c).
func runTurnDone(opts *options.Options, stdin io.Reader) error {
	var raw []byte
	if opts.Body == "" {
		var err error
		if raw, err = io.ReadAll(stdin); err != nil {
			return err
		}
	}
	text, source, err := harness.Payload(opts.Body, raw)
	if errors.Is(err, harness.ErrNotATurn) {
		return nil // the harness's own side turn (a title): nothing to hand on
	}
	if err != nil {
		return i18n.Errorf("error.turn_done_no_payload", err.Error())
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	sig := turns.Signal{Member: opts.Member, At: time.Now(), Text: text, Source: source}
	if err := turns.WriteSignal(turns.SignalDir(dir, opts.Team), sig); err != nil {
		return err
	}
	if source == "codex" {
		harness.NotifyThen(os.Getenv(harness.EnvNotifyThen), opts.Body)
	}
	return nil
}
