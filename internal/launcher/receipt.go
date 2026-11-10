package launcher

import (
	"errors"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// Undelivered names an initial prompt for which herdr did not observe receipt
// after the one retry. Only this error permits continuing with other members;
// startup and other sending failures keep their existing stop behavior.
type Undelivered struct {
	Name     string
	Pane     string
	Recovery string
	Cause    error
}

func (e *Undelivered) Error() string {
	return i18n.Tf("launcher.undelivered", e.Name, e.Pane, protocol.Message(e.Cause), e.Recovery)
}

// IsUndelivered recognizes receipt failures, including the joined failures an
// operation returns after processing its remaining members.
func IsUndelivered(err error) bool {
	_, ok := errors.AsType[*Undelivered](err)
	return ok
}

// Received reads the existing progress record; it is also how apply avoids
// recording or claiming success for a member whose prompt did not arrive.
func (l *Launcher) Received(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.progress.States[name] == i18n.T("progress.mark.bootstrap_submitted")
}

// hireRetry reconstructs the original task and effective hiring member, rather
// than the generated bootstrap prompt. Quotes keep tasks as data in the shell.
func hireRetry(project, team string, worker *definition.Start, task string) string {
	return "hergy down " + worker.Name + " -C " + shellWord(project) + " && hergy hire " + worker.Short +
		" --team " + team + " -C " + shellWord(project) + " --purpose " + shellWord(worker.Purpose) +
		" --task " + shellWord(task) + " --from " + worker.From
}

func shellWord(s string) string {
	s = strings.ReplaceAll(s, "'", "'\"'\"'")
	s = strings.ReplaceAll(s, "\n", "'$'\\n''")
	s = strings.ReplaceAll(s, "\r", "'$'\\r''")
	return "'" + s + "'"
}
