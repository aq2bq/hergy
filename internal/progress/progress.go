// Package progress records how far a boot, refresh or hire got, so a failure
// can say where it stopped.
package progress

import "fmt"

// Progress is the running state of one operation.
type Progress struct {
	Stage  string
	TabID  string
	Panes  map[string]string
	States map[string]string
	order  []string
}

// Start begins a progress record for the given member names.
func Start(names []string) *Progress {
	states := make(map[string]string, len(names))
	for _, name := range names {
		states[name] = "not started"
	}
	return &Progress{Stage: "preflight", Panes: map[string]string{}, States: states, order: append([]string(nil), names...)}
}

// At moves to a stage.
func (p *Progress) At(stage string) { p.Stage = stage }

// Place records the pane a member got.
func (p *Progress) Place(name, pane string) { p.Panes[name] = pane }

// Mark records a member's state.
func (p *Progress) Mark(name, state string) { p.States[name] = state }

// Report is the failure report, in the order it is printed.
func (p *Progress) Report() []string {
	lines := []string{fmt.Sprintf("Stopped at: %s", p.Stage)}
	if p.TabID != "" {
		lines = append(lines, fmt.Sprintf("Created tab: %s", p.TabID))
	}
	for _, name := range p.order {
		state := p.States[name]
		pane := p.Panes[name]
		if pane == "" {
			pane = "-"
		}
		lines = append(lines, fmt.Sprintf("  %s: pane=%s; %s", name, pane, state))
	}
	lines = append(lines,
		"Existing panes/processes are kept. A failed request may already have taken effect.",
		"Inspect live state before retrying; bootstrap prompts are never retried automatically.",
	)
	return lines
}
