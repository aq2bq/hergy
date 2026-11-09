package turns

import (
	"path/filepath"
	"time"
)

// Signal is a harness's own word that a member's turn ended (r-378e): hergy
// turn-done writes one when Claude Code's Stop hook, Codex's notify or
// opencode's plugin calls it, with the turn's last reply. The watcher reads
// signals before herdr: a signal ends the turn whatever herdr's status says,
// and its text is what is handed on (d-ef1d).
type Signal struct {
	// Member is the short name of the member or worker whose turn ended.
	Member string    `json:"member"`
	At     time.Time `json:"at"`
	// Text is the last reply of the turn, as the harness gave it.
	Text string `json:"text"`
	// Source is which harness spoke: claude, codex or opencode.
	Source string `json:"source"`
}

// SignalGrace is how long a wired member's stop, as herdr sees it, waits for
// the harness's signal before the pane is read instead (d-5f1a): the signal is
// written before the harness goes idle, so it is normally there first, and one
// that is not within this time is not coming.
const SignalGrace = 5 * time.Second

// SignalDir is where one team's signals wait: <stateDir>/turns/<team>.signals/,
// one file each, so a hook's write and the watcher's read never meet halfway.
func SignalDir(stateDir, team string) string {
	return filepath.Join(stateDir, "turns", team+".signals")
}

// WriteSignal puts one signal in the team's directory, through a temporary
// file and a rename. The name orders them by time.
func WriteSignal(dir string, sig Signal) error {
	return writeNote(dir, sig, sig.At, sig.Member)
}

// TakeSignals reads every signal waiting in the directory, oldest first, and
// removes them. A directory that is not there holds none.
func TakeSignals(dir string) ([]Signal, error) {
	return takeNotes[Signal](dir)
}
