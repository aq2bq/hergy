package ops

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/launcher"
)

// Plan is the resolved input shown before an operation starts, or as a dry plan.
type Plan struct {
	Resolved *Resolved
	Members  []definition.Start
	Mode     string
	Next     string
}

// Progress describes a launcher step without exposing the launcher to a UI.
type Progress struct {
	Stage, Member, State string
	Pane                 string
	TabID, Workspace     string
	Waiting              *Waiting
	CreatedTab           string
	CreatedPanes         int
}

// Waiting is the startup question that the person answers in herdr.
type Waiting struct{ Member, Herdr, Workspace, Tab, Pane string }

// Event is the one stream for command output and screen operation state.
// Only adapters choose where to display its text or warning.
type Event struct {
	Applied       *Applied
	Plan          *Plan
	Start         *StartPlan
	Progress      *Progress
	Text, Warning string
	Failure       error
	Diagnostics   string
	Cancelled     bool
}

// OperationError marks a failure already emitted as an operation event.
type OperationError struct {
	Err       error
	Cancelled bool
}

func (e *OperationError) Error() string { return e.Err.Error() }
func (e *OperationError) Unwrap() error { return e.Err }

func emit(observe func(Event), event Event) {
	if observe != nil {
		observe(event)
	}
}

type eventWriter struct{ observe func(Event) }

func (w eventWriter) Write(p []byte) (int, error) {
	emit(w.observe, Event{Text: string(p)})
	return len(p), nil
}

func operationLauncher(ctx context.Context, r *Resolved, timeout int, port Port, observe func(Event)) *launcher.Launcher {
	emit(observe, Event{Text: PluginNote()})
	l := NewLauncherWithPort(r, timeout, eventWriter{observe}, stopPort{Port: portOrDefault(port), ctx: ctx}).WithContext(ctx)
	l.Observe(func(e launcher.Event) { emit(observe, Event{Progress: progressOf(e)}) })
	return l
}

// progressOf adapts the launcher while the remaining callers migrate.
func progressOf(e launcher.Event) *Progress {
	p := &Progress{Stage: e.Stage, Member: e.Member, State: e.State, Pane: e.Pane.String(), TabID: e.TabID, Workspace: e.Workspace, CreatedTab: e.CreatedTab, CreatedPanes: e.CreatedPanes}
	if e.Waiting != nil {
		p.Waiting = &Waiting{Member: e.Waiting.Member, Herdr: e.Waiting.Herdr, Workspace: e.Waiting.Workspace, Tab: e.Waiting.Tab, Pane: e.Waiting.Pane}
	}
	return p
}

func execute(ctx context.Context, l *launcher.Launcher, observe func(Event), transform func(error) error, run func() error) error {
	err := run()
	if ctx.Err() != nil || errors.Is(err, launcher.ErrCancelled) {
		var detail bytes.Buffer
		l.ReportInterrupted(&detail)
		emit(observe, Event{Cancelled: true, Diagnostics: detail.String()})
		return &OperationError{Err: context.Canceled, Cancelled: true}
	}
	if err == nil {
		return nil
	}
	if transform != nil {
		err = transform(err)
	}
	var detail bytes.Buffer
	l.ReportFailure(&detail)
	emit(observe, Event{Failure: err, Diagnostics: detail.String()})
	return &OperationError{Err: err}
}

func warning(observe func(Event), text string) {
	if text != "" {
		emit(observe, Event{Warning: text})
	}
}

var _ io.Writer = eventWriter{}
