package ops

import (
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/harness"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/screenlog"
)

// WireStart gives one start its harness's turn-end signal (r-378e): the one
// place a start is wired, so every way a member is built (up, hire, refresh,
// apply, the screens) starts it the same.
func WireStart(start *definition.Start, teamName string) {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		dir = ""
	}
	harness.Wire(start, teamName, harness.Here{Bin: Executable(), StateDir: dir, CodexHome: harness.CodexHome()})
}

// PluginNote writes hergy's opencode plugin into the state directory, so a
// wired opencode member finds it. A failure is said once and the member's
// stops are seen through herdr instead.
func PluginNote() string {
	dir, err := screenlog.DefaultDir()
	if err == nil {
		err = harness.WritePlugin(dir)
	}
	if err != nil {
		return i18n.Tf("launcher.plugin_failed", err.Error())
	}
	return ""
}

// SignalNotes says which members got no signal although their harness
// has one, and why: the profile passes the same argument or environment, or
// opencode runs without --standalone (d-e511).
func SignalNotes(starts []definition.Start) []string {
	var notes []string
	for _, s := range starts {
		switch s.Signal.Skipped {
		case harness.SkippedArgs:
			notes = append(notes, i18n.Tf("launcher.signal_skipped_args", s.Short.String(), s.Kind))
		case harness.SkippedEnv:
			notes = append(notes, i18n.Tf("launcher.signal_skipped_env", s.Short.String()))
		case harness.SkippedStandalone:
			notes = append(notes, i18n.Tf("launcher.signal_skipped_standalone", s.Short.String()))
		}
	}
	return notes
}
