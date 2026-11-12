package launcher

import (
	"errors"
	"strings"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
)

// Undelivered names an initial prompt for which herdr did not observe receipt
// after the one retry. Only this error permits continuing with other members;
// startup and other sending failures keep their existing stop behavior.
type Undelivered struct {
	Name     string
	Pane     domainname.PaneID
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
	projectSetup, projectWord := recoveryWord(project, "project")
	purposeSetup, purposeWord := recoveryWord(worker.Purpose, "purpose")
	taskSetup, taskWord := recoveryWord(task, "task")
	return projectSetup + purposeSetup + taskSetup +
		"hergy down " + worker.Name.String() + " -C " + projectWord + " && hergy hire " + worker.Short.String() +
		" --team " + team + " -C " + projectWord + " --purpose " + purposeWord +
		" --task " + taskWord + " --from " + worker.From.String()
}

func repromptRetry(project, team, member string) string {
	setup, word := recoveryWord(project, "project")
	return setup + "hergy reprompt " + member + " --team " + team + " -C " + word
}

// recoveryWord keeps a recovery command on one physical line in POSIX shells.
// For multiline values, the sentinel protects trailing newlines from command
// substitution; parameter expansion removes only that final sentinel.
func recoveryWord(value, name string) (setup, word string) {
	if !strings.ContainsAny(value, "\r\n") {
		return "", shellWord(value)
	}
	encoded := strings.ReplaceAll(value, "\\", "\\\\")
	encoded = strings.ReplaceAll(encoded, "\n", "\\n")
	encoded = strings.ReplaceAll(encoded, "\r", "\\r")
	variable := "_hergy_retry_" + name
	return variable + "=$(printf '%b_' " + shellWord(encoded) + ") && ", "\"${" + variable + "%_}\""
}

func shellWord(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
