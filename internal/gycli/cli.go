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
	"sync"
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
	return OpenQuestionsBy(dir, nil)
}

// OpenQuestionsBy reads the open questions of one project that one of writers
// asked, newest read first in gy's own order (6 A5): a screen names the herdr
// names of one team's members and shows only what they asked, and the questions
// of everyone else are never even asked about. An empty writers means every
// writer.
//
// The writer is on the write that created the question, so the writes are read
// first and only the questions that are left are read one by one — a project
// with 55 open questions asked by 20 people reads 3 of them for one team rather
// than 55.
func OpenQuestionsBy(dir string, writers []string) ([]activity.Question, error) {
	cli, ok := open(dir)
	if !ok {
		return nil, nil
	}
	return cli.questions(writers)
}

// open is the gy binary of one project, and whether this project uses gy at all:
// the HERGY_GY override or gy on PATH, and a gy.toml at dir. ok is false when
// either is missing, which is no gy here rather than an error.
//
// A HERGY_GY that is not a file is no gy either: a binary that is not there
// cannot have been read, so it is the same as a machine with no gy, which is how
// a screen reads quietly rather than reporting a failure that did not happen
// (spec-activity 2.4, E2).
func open(dir string) (CLI, bool) {
	bin := os.Getenv("HERGY_GY")
	if bin == "" {
		found, err := exec.LookPath("gy")
		if err != nil {
			return CLI{}, false
		}
		bin = found
	} else if !isFile(bin) {
		return CLI{}, false
	}
	if !isFile(filepath.Join(dir, configName)) {
		return CLI{}, false
	}
	return CLI{Dir: dir, Bin: bin}, true
}

// Write is one write the ledger holds, as `gy list --since` writes it: who
// wrote, about which node, and why. The first two words of why name the write
// ("req approve r-1c42"), which is what a team definition's ledger: rules speak
// of (spec-ledger).
type Write struct {
	Actor string
	Node  string
	Why   string
}

// Ledger reads one project's ledger, the whole of it. It is the one place the
// ledger check goes through the gy binary, so the tests can substitute a fake gy
// with HERGY_GY (spec-ledger: テストでは gycli を差し替える).
type Ledger struct{ cli CLI }

// OpenLedger is the ledger of one project, and whether this project has one: a
// project with no gy.toml, a machine with no gy on PATH, or a HERGY_GY that is
// not a file, is no ledger here, told apart from a gy that is there and cannot be
// read (whose reads return the error).
func OpenLedger(dir string) (Ledger, bool) {
	cli, ok := open(dir)
	if !ok {
		return Ledger{}, false
	}
	return Ledger{cli: cli}, true
}

// Writes reads every write of the ledger, oldest first (`gy list --since 0`). A
// write with no writer or no why is not one the rules can judge, so it is
// skipped.
func (l Ledger) Writes() ([]Write, error) {
	var rows []writeRow
	out, err := l.cli.capture("list", "--since", "0", "--json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	writes := make([]Write, 0, len(rows))
	for _, row := range rows {
		if row.Actor == "" || row.Why == "" {
			continue
		}
		writes = append(writes, Write{Actor: row.Actor, Node: row.Node, Why: row.Why})
	}
	return writes, nil
}

// Decider reads one question's decider (`gy show <id>`).
func (l Ledger) Decider(node string) (string, error) {
	detail, err := l.cli.show(node)
	if err != nil {
		return "", err
	}
	return detail.Decider, nil
}

// questions runs the two reads one question needs: the list of the open ones,
// and each one's details. The writers come from the ledger's writes, which is
// where gy records who asked; a project with no open question reads none of
// them, and one where writers is not empty reads the details of the questions
// those writers asked and no others (6 A5).
func (c CLI) questions(writers []string) ([]activity.Question, error) {
	listed, err := c.list()
	if err != nil {
		return nil, err
	}
	if len(listed) == 0 {
		return nil, nil
	}
	who, err := c.writers()
	if err != nil {
		return nil, err
	}
	if len(writers) > 0 {
		wanted := make(map[string]bool, len(writers))
		for _, writer := range writers {
			wanted[writer] = true
		}
		kept := listed[:0]
		for _, row := range listed {
			if wanted[who[row.ID]] {
				kept = append(kept, row)
			}
		}
		listed = kept
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
			Writer:  who[row.ID],
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
	out, err := c.captureList("list", "--type", "question", "--status", "open", "--json")
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
// node, what they did, and why.
type writeRow struct {
	Actor string `json:"actor"`
	Node  string `json:"node"`
	What  string `json:"what"`
	Why   string `json:"why"`
}

// writers is who asked each open question: the actor of the write that created
// it. gy keeps the writer on the write unit, not on the question's own fields,
// so it is read from there (設計 10.1).
func (c CLI) writers() (map[string]string, error) {
	var rows []writeRow
	out, err := c.captureList("list", "--since", "0", "--type", "question", "--json")
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

// listMemory is how long a gy list's answer is remembered (6 A5).
const listMemory = 10 * time.Second

// listCache is the remembered answers of gy list, one per binary and project and
// argv. The activity's own read runs the same two lists once per row it opens,
// and the ledger does not change under a screen; the memory is small and old
// entries are dropped as they are read, so it needs no goroutine.
var listCache struct {
	sync.Mutex
	entries map[string]listAnswer
}

// listAnswer is one list's answer and when it was read.
type listAnswer struct {
	out string
	at  time.Time
}

// captureList is capture for a gy list: the answer is remembered for ten seconds
// (6 A5). A list that failed is not remembered, so a gy that was not there is
// asked again rather than answered with a stale failure.
func (c CLI) captureList(args ...string) (string, error) {
	key := c.Bin + "\x00" + c.Dir + "\x00" + strings.Join(args, " ")
	listCache.Lock()
	if answer, ok := listCache.entries[key]; ok && time.Since(answer.at) < listMemory {
		listCache.Unlock()
		return answer.out, nil
	}
	listCache.Unlock()

	out, err := c.capture(args...)
	if err != nil {
		return "", err
	}
	listCache.Lock()
	if listCache.entries == nil {
		listCache.entries = map[string]listAnswer{}
	}
	for stale, answer := range listCache.entries {
		if time.Since(answer.at) >= listMemory {
			delete(listCache.entries, stale)
		}
	}
	listCache.entries[key] = listAnswer{out: out, at: time.Now()}
	listCache.Unlock()
	return out, nil
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
	info, err := os.Stat(path) //nolint:gosec // G703: path is HERGY_GY or the gy hergy found, not a path the request chose
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
