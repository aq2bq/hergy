// Package gycli runs the gy binary. It is the only place that starts a gy
// process, so a fake gy can be substituted with HERGY_GY, and the activity's
// questions come from gy's own output rather than from hergy holding a ledger
// (I17, 設計 5.3). It sits beside herdrcli: both are the top layer, reached from
// the app only.
package gycli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/activity"
)

// configName is the file whose presence at a project root says gy is used here
// (設計 5.3): without it, hergy makes the activity from herdr alone and says
// nothing.
const configName = "gy.toml"

// CLI calls one gy binary against one project's ledger.
type CLI struct {
	// Dir is the project root: gy reads the gy.toml there.
	Dir string
	// Bin is the executable, gy, or the HERGY_GY override.
	Bin string
}

// OpenQuestions reads the open questions of one project, as the activity shows
// them: the title, the options, the decider and when it was asked, and the
// writer that asked it (設計 5.3).
//
// A project with no gy.toml, or a machine with no gy on PATH, is no gy here:
// the answer is no questions and no error, so the activity is made from herdr
// alone (I17). A gy that is there and cannot be read is an error, told apart
// from the absent case.
func OpenQuestions(dir string) ([]activity.Question, error) {
	bin := os.Getenv("HERGY_GY")
	if bin == "" {
		found, err := exec.LookPath("gy")
		if err != nil {
			return nil, nil
		}
		bin = found
	}
	if !isFile(filepath.Join(dir, configName)) {
		return nil, nil
	}
	return CLI{Dir: dir, Bin: bin}.questions()
}

// questions runs the two reads one question needs: the list of the open ones,
// and each one's details. The writers come from the ledger's writes, which is
// where gy records who asked; a project with no open question reads none of
// them.
func (c CLI) questions() ([]activity.Question, error) {
	listed, err := c.list()
	if err != nil {
		return nil, err
	}
	if len(listed) == 0 {
		return nil, nil
	}
	writers, err := c.writers()
	if err != nil {
		return nil, err
	}
	out := make([]activity.Question, 0, len(listed))
	for _, row := range listed {
		detail, err := c.show(row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, activity.Question{
			ID:      row.ID,
			Title:   row.Title,
			Options: detail.Options,
			Decider: detail.Decider,
			At:      parseTime(row.Created),
			Writer:  writers[row.ID],
		})
	}
	return out, nil
}

// listRow is one open question as `gy list --json` writes it.
type listRow struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Created string `json:"created"`
}

// list reads the open questions.
func (c CLI) list() ([]listRow, error) {
	var rows []listRow
	out, err := c.capture("list", "--type", "question", "--status", "open", "--json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// showRow is one question as `gy show <id> --json` writes it: the options and
// the decider live under data.Question.
type showRow struct {
	Data struct {
		Question struct {
			Decider string   `json:"decider"`
			Options []string `json:"options"`
		} `json:"Question"`
	} `json:"data"`
}

// shown is what one question's own read carries.
type shown struct {
	Decider string
	Options []string
}

// show reads one question's details.
func (c CLI) show(id string) (shown, error) {
	var rows []showRow
	out, err := c.capture("show", id, "--json")
	if err != nil {
		return shown{}, err
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return shown{}, err
	}
	if len(rows) == 0 {
		return shown{}, nil
	}
	return shown{Decider: rows[0].Data.Question.Decider, Options: rows[0].Data.Question.Options}, nil
}

// writeRow is one write as `gy list --since` writes it: who wrote, about which
// node, and what they did.
type writeRow struct {
	Actor string `json:"actor"`
	Node  string `json:"node"`
	What  string `json:"what"`
}

// writers is who asked each open question: the actor of the write that created
// it. gy keeps the writer on the write unit, not on the question's own fields,
// so it is read from there (設計 10.1).
func (c CLI) writers() (map[string]string, error) {
	var rows []writeRow
	out, err := c.capture("list", "--since", "0", "--type", "question", "--json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	out2 := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.What == "created" && row.Node != "" && row.Actor != "" {
			out2[row.Node] = row.Actor
		}
	}
	return out2, nil
}

// capture runs one gy command in the project's directory and returns its stdout.
func (c CLI) capture(args ...string) (string, error) {
	cmd := exec.Command(c.Bin, args...) //nolint:gosec // G204: c.Bin is gy and args are the fixed subcommands this package builds
	cmd.Dir = c.Dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(errBuf.String())
		if reason == "" {
			reason = err.Error()
		}
		return "", gyError{args: args, reason: reason}
	}
	return out.String(), nil
}

// gyError is one failed gy call: the subcommand and what gy said.
type gyError struct {
	args   []string
	reason string
}

func (e gyError) Error() string {
	return "gy " + strings.Join(e.args, " ") + ": " + e.reason
}

// isFile reports whether a path is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// parseTime is one of gy's timestamps, or the zero time when it does not read.
func parseTime(text string) time.Time {
	at, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}
	return at
}
