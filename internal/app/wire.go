package app

import (
	"io"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/harness"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/screenlog"
)

// wireStart gives one start its harness's turn-end signal (r-378e): the one
// place a start is wired, so every way a member is built (up, hire, refresh,
// apply, the screens) starts it the same.
func wireStart(start *definition.Start, teamName string) {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		dir = ""
	}
	harness.Wire(start, teamName, harness.Here{Bin: executable(), StateDir: dir, CodexHome: harness.CodexHome()})
}

// ensurePlugin writes hergy's opencode plugin into the state directory, so a
// wired opencode member finds it. A failure is said once and the member's
// stops are seen through herdr instead.
func ensurePlugin(out io.Writer) {
	dir, err := screenlog.DefaultDir()
	if err == nil {
		err = harness.WritePlugin(dir)
	}
	if err != nil {
		_, _ = io.WriteString(out, i18n.Tf("launcher.plugin_failed", err.Error()))
	}
}

// printSignalNotes says which members got no signal although their harness
// has one, and why: the profile passes the same argument or environment, or
// opencode runs without --standalone (d-e511).
func printSignalNotes(out io.Writer, starts []definition.Start) {
	for _, s := range starts {
		switch s.Signal.Skipped {
		case harness.SkippedArgs:
			_, _ = io.WriteString(out, i18n.Tf("launcher.signal_skipped_args", s.Short, s.Kind))
		case harness.SkippedEnv:
			_, _ = io.WriteString(out, i18n.Tf("launcher.signal_skipped_env", s.Short))
		case harness.SkippedStandalone:
			_, _ = io.WriteString(out, i18n.Tf("launcher.signal_skipped_standalone", s.Short))
		}
	}
}
