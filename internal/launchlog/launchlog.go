// Package launchlog writes and reads the record of hergy's launches: what hergy
// handed each member, and which rule files it read to build it. The WebUI reads
// it to show what changed since the last launch and how the prompts differ; an
// evaluation reads it to line the launches up with an agent's own logs. The
// commands write it after a launch succeeds, and a launch that cannot write it
// still runs (spec/11).
//
// The files live under $XDG_STATE_HOME/hergy/launches: log.jsonl, one line per
// launch, and blobs/<sha256>, one file per text. A line holds no prompt text —
// only the hashes of the texts it points at — so the same document read by five
// members is stored once, and a launch that changes one document adds one blob.
package launchlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	// logName is the one line per launch, appended and never rewritten.
	logName = "log.jsonl"
	// blobsDir is where the texts live, one file per text, named by its hash.
	blobsDir = "blobs"
)

// Command is what a record was written for. A launcher's commands write their
// launches; hire writes one line about the worker it added, so the activity
// screen can say which member hired it (d-b4b9).
const (
	CommandUp       = "up"
	CommandRefresh  = "refresh"
	CommandReprompt = "reprompt"
	CommandHire     = "hire"
)

// Hire is one worker a member hired, as its hire line records it: the name herdr
// sees, the member that added it (the short name team.yaml writes), the purpose
// it was hired for, and the tab it was put in. A worker hired before this line
// was written has no hire line and no From anywhere.
type Hire struct {
	Worker  string `json:"worker"` // the name herdr sees ("<team>-<worker>")
	From    string `json:"from"`   // the short name of the member that added it
	Purpose string `json:"purpose"`
	Tab     string `json:"tab"`
}

// Member is one member as it was handed a prompt. Prompt is the hash of the
// prompt text, which is in blobs/. Start and Env are the fingerprints of how the
// member was started: what the comparison needs to tell a changed profile from a
// changed prompt (d-36ef). A record written before they were kept has neither.
type Member struct {
	Name   string `json:"name"`            // the name herdr sees ("<team>-<member>")
	Short  string `json:"short"`           // the short name team.yaml writes
	Prompt string `json:"prompt"`          // sha256 of the prompt text
	Start  string `json:"start,omitempty"` // sha256 of the command the member is started by
	Env    string `json:"env,omitempty"`   // sha256 of the env keys and values; the values are written nowhere
}

// Doc is one rule file a member read. Revision is the document's own revision,
// or 0 when it has none.
type Doc struct {
	Path     string `json:"path"`
	Layer    string `json:"layer"`
	Revision int    `json:"revision,omitempty"`
	SHA256   string `json:"sha256"`
}

// Record is one launch: one line of log.jsonl. The texts it points at are in
// blobs/, under Member.Prompt and Doc.SHA256. No environment value is written
// here or in a blob — a value can be a key — and env counts toward Hash only.
//
// The facts the prompts were built with are written here too (Executable, the
// project root, the language), because a prompt names this hergy's own path and
// the language it is written in: a screen that rebuilds a prompt to see what has
// changed since the launch builds it from these, not from how hergy is run now.
// A record written before these were kept has neither, and nothing can be
// rebuilt from it.
type Record struct {
	Time       string `json:"time"` // RFC 3339, where hergy ran
	Command    string `json:"command"`
	Hergy      string `json:"hergy"` // which hergy wrote this: version.Line()
	Project    string `json:"project"`
	Layer      string `json:"layer"`
	Definition string `json:"definition"`
	Team       string `json:"team"`
	TeamYAML   string `json:"team_yaml"`
	// Workspace is the herdr workspace the team was started in: the label a
	// person reads, and the id it had then. The headquarters offers the last one
	// as "前に動いた所" (d-73fc). A record written before this was kept has
	// neither, and so has refresh, reprompt and hire: they start no tab.
	Workspace   string   `json:"workspace,omitempty"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
	Hash        string   `json:"hash"`
	Executable  string   `json:"executable,omitempty"`
	Lang        string   `json:"lang,omitempty"`
	Members     []Member `json:"members"`
	Docs        []Doc    `json:"docs"`
	// Hire is the worker this line added, for CommandHire; nil for a launch.
	// A hire line carries no members, no docs and no team_yaml: nothing was
	// handed a prompt by it.
	Hire *Hire `json:"hire,omitempty"`
}

// IsHire says this line records a worker's hire, not a launch.
func (r Record) IsHire() bool { return r.Command == CommandHire }

// Workspace is one herdr workspace as a launch names it: the label a person
// reads, and the id it had when the team was started there.
type Workspace struct {
	Label string
	ID    string
}

// Material is what the hash of a launch is made of: the team definition's
// settings file, and for every member of the definition, how it is started and
// the prompt it is handed. hergy compares two launches of one team definition by
// this hash; revision, which a person bumps, is a name they use and never a
// substitute for it.
type Material struct {
	TeamYAML []byte
	Members  []MaterialMember
}

// MaterialMember is one member's part of the hash: what a member is started
// with, after its profile is resolved, and the prompt it is handed.
type MaterialMember struct {
	Kind   string
	Args   []string
	Env    map[string]string
	Prompt string
}

// Hash is the hash of one team definition as it is launched, in lowercase hex
// over the whole definition: every member, in the order the definition lists
// them, whatever it is that reached them. A change to a profile no member reads
// does not reach a member, so it does not change the hash; a change to a profile
// a member reads does.
func Hash(m Material) string {
	var b strings.Builder
	field(&b, "team.yaml", string(m.TeamYAML))
	field(&b, "members", strconv.Itoa(len(m.Members)))
	for _, member := range m.Members {
		field(&b, "kind", member.Kind)
		field(&b, "args", strconv.Itoa(len(member.Args)))
		for _, arg := range member.Args {
			field(&b, "arg", arg)
		}
		// An env map has no order of its own, so the keys are sorted: two runs
		// that hand a member the same environment hash the same. The values are
		// folded in here and written nowhere else.
		keys := make([]string, 0, len(member.Env))
		for key := range member.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		field(&b, "env", strconv.Itoa(len(keys)))
		for _, key := range keys {
			field(&b, "env key", key)
			field(&b, "env value", member.Env[key])
		}
		field(&b, "prompt", member.Prompt)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// StartFingerprint is the hash of how a member is started: the kind and the
// profile's args, which are the command hergy starts the agent with. The env is
// not part of it: env is fixed when the pane is made, so a change to it is a
// different question (EnvFingerprint). A person reads this fingerprint only as
// "changed", never as a command.
func StartFingerprint(kind string, args []string) string {
	var b strings.Builder
	field(&b, "kind", kind)
	field(&b, "args", strconv.Itoa(len(args)))
	for _, arg := range args {
		field(&b, "arg", arg)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// EnvFingerprint is the hash of one member's environment: the keys, in order, and
// their values. The values are folded in here and written nowhere else, so a
// change to one can be seen without the record holding a secret.
func EnvFingerprint(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	field(&b, "env", strconv.Itoa(len(keys)))
	for _, key := range keys {
		field(&b, "env key", key)
		field(&b, "env value", env[key])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// field writes one labelled field: its name, its length, then its bytes. The
// length is what keeps two fields from running into each other, so a member
// whose prompt is "ab" and whose kind is "c" cannot hash like one whose prompt
// is "a" and whose kind is "bc".
func field(b *strings.Builder, name, value string) {
	b.WriteString(name)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(len(value)))
	b.WriteByte(':')
	b.WriteString(value)
}

// Log is one machine's record of launches.
type Log struct{ dir string }

// DefaultDir is where this machine's record lives: $XDG_STATE_HOME/hergy/launches,
// or ~/.local/state/hergy/launches when XDG_STATE_HOME is unset or not absolute.
// State is not settings, so it is not put under the config directory, and it is
// not put in the project: a launch is a fact about this machine, and the global
// layer's team definitions have no project of their own.
func DefaultDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "hergy", "launches"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hergy", "launches"), nil
}

// Open is this machine's record.
func Open() (Log, error) {
	dir, err := DefaultDir()
	if err != nil {
		return Log{}, err
	}
	return At(dir), nil
}

// At is the record in one directory: the tests use it, and so does a caller that
// knows where the record belongs better than this package does.
func At(dir string) Log { return Log{dir: dir} }

// Dir is where this record's files go.
func (l Log) Dir() string { return l.dir }

// Put stores one text and returns the hash it is stored under: sha256 of the
// text, in lowercase hex. The same text is stored once — putting it again writes
// nothing — so a launch that changes one document adds one file.
func (l Log) Put(text []byte) (string, error) {
	sum := sha256.Sum256(text)
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(l.dir, blobsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hash)
	if _, err := os.Stat(path); err == nil {
		return hash, nil
	}
	// A temp file and a rename, so a reader never sees half a text under a name
	// that claims to be its whole hash.
	tmp, err := os.CreateTemp(dir, hash+".*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(text); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return hash, nil
}

// Text is the text stored under one hash.
func (l Log) Text(hash string) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.dir, blobsDir, hash))
}

// Append adds one launch: one line at the end of log.jsonl. The texts the record
// points at are put first (Put), and the line is written with one write, so a
// reader sees whole lines. Nothing is ever rewritten: the record is history.
func (l Log) Append(rec Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(l.dir, logName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Read returns every launch, oldest first. A record that is not there yet is an
// empty list, not an error: nothing has been launched on this machine.
func (l Log) Read() ([]Record, error) {
	text, err := os.ReadFile(filepath.Join(l.dir, logName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	for i, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", logName, i+1, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// Latest is the newest launch of one team definition, and whether there is one. A
// launch of a team running under another name (--as) belongs to the definition it
// was launched from.
//
// The definition is named by the team.yaml its launch read: this machine's record
// holds every project's launches, and two projects can hold a definition of the
// same name, so the layer and the name alone would hand back another project's
// launch (M1).
func (l Log) Latest(teamYAML string) (Record, bool, error) {
	records, err := l.Read()
	if err != nil {
		return Record{}, false, err
	}
	for _, record := range slices.Backward(records) {
		if record.TeamYAML == teamYAML {
			return record, true, nil
		}
	}
	return Record{}, false, nil
}
