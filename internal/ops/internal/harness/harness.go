// Package harness owns what hergy adds to a member's start so its harness says
// when a turn ends (r-378e, n-9cee): the arguments and environment that wire
// Claude Code's Stop hook, Codex's notify and opencode's plugin to `hergy
// turn-done <team> <member>`. Nothing is written into the person's own
// configuration (~/.claude, ~/.codex, ~/.config/opencode) or the project: the
// hook is a start argument, the notify a start argument, and the plugin a file
// in hergy's own state directory that OPENCODE_CONFIG_DIR names.
//
// The rules, one per harness, measured on 2026-11-07:
//   - claude: `--settings '{"hooks":{"Stop":[…]}}'` runs the command once per
//     turn with the turn's last reply on stdin (last_assistant_message); the
//     person's own hooks keep running. A profile that passes --settings itself
//     is left alone.
//   - codex: `-c notify=[…]` runs the command with the turn as one JSON
//     argument (last-assistant-message); it replaces the person's notify for
//     that process, so the person's own notify is handed to hergy turn-done to
//     call after it (HERGY_NOTIFY_THEN; d-a70c). A profile that sets notify
//     itself is left alone.
//   - opencode: OPENCODE_CONFIG_CONTENT, the inline config opencode merges
//     over the person's own, names hergy's plugin directory in "plugins"; a
//     profile's own OPENCODE_CONFIG_CONTENT (the way a model is picked) is
//     kept and the entry added to it. OPENCODE_CONFIG_DIR is not used: it
//     replaces the person's global directory (measured 2026-11-07: with it,
//     neither the global plugins nor cli.json are read). The plugin reads
//     HERGY_BIN, HERGY_TEAM and HERGY_MEMBER from the pane's environment,
//     which only a --standalone opencode has (d-e511). A profile without
//     --standalone, or whose OPENCODE_CONFIG_CONTENT is not JSON, is left
//     alone.
package harness

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
)

// Here is what the wiring needs of this machine: the hergy to call back, the
// state directory the plugin lives in, and the person's Codex home (CODEX_HOME,
// else ~/.codex) whose notify is kept.
type Here struct {
	Bin       string
	StateDir  string
	CodexHome string
}

// Why a member got no signal (definition.Signal.Skipped).
const (
	// SkippedKind: the harness is one hergy has no signal for.
	SkippedKind = "kind"
	// SkippedArgs: the profile passes the same argument itself (--settings,
	// -c notify=).
	SkippedArgs = "args"
	// SkippedEnv: the profile's OPENCODE_CONFIG_CONTENT is not JSON hergy can
	// add to.
	SkippedEnv = "env"
	// SkippedStandalone: an opencode profile without --standalone.
	SkippedStandalone = "standalone"
)

// Environment the opencode plugin and hergy turn-done read.
const (
	EnvBin        = "HERGY_BIN"
	EnvTeam       = "HERGY_TEAM"
	EnvMember     = "HERGY_MEMBER"
	EnvNotifyThen = "HERGY_NOTIFY_THEN"
	// EnvConfigContent is opencode's inline config, merged over the person's.
	EnvConfigContent = "OPENCODE_CONFIG_CONTENT"
)

// Wire fills start.Signal for the member's harness: what to add to its start
// so the harness calls hergy turn-done, or why nothing is added.
func Wire(start *definition.Start, team string, here Here) {
	switch start.Kind {
	case "claude":
		start.Signal = claude(start, team, here)
	case "codex":
		start.Signal = codex(start, team, here)
	case "opencode":
		start.Signal = opencode(start, team, here)
	default:
		start.Signal = definition.Signal{Skipped: SkippedKind}
	}
}

func claude(start *definition.Start, team string, here Here) definition.Signal {
	if slices.ContainsFunc(start.Args, func(a string) bool { return a == "--settings" || strings.HasPrefix(a, "--settings=") }) {
		return definition.Signal{Skipped: SkippedArgs}
	}
	settings := map[string]any{"hooks": map[string]any{"Stop": []any{
		map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellWords(here.Bin, "turn-done", team, start.Short.String())}}},
	}}}
	return definition.Signal{Wired: true, Args: []string{"--settings", jsonText(settings)}}
}

func codex(start *definition.Start, team string, here Here) definition.Signal {
	for i, a := range start.Args {
		if strings.HasPrefix(a, "notify=") || (a == "-c" && i+1 < len(start.Args) && strings.HasPrefix(start.Args[i+1], "notify=")) {
			return definition.Signal{Skipped: SkippedArgs}
		}
	}
	sig := definition.Signal{Wired: true, Args: []string{"-c", "notify=" + jsonText([]string{here.Bin, "turn-done", team, start.Short.String()})}}
	if then, ok := CodexNotify(here.CodexHome); ok {
		sig.Env = map[string]string{EnvNotifyThen: jsonText(then)}
	}
	return sig
}

func opencode(start *definition.Start, team string, here Here) definition.Signal {
	if !slices.Contains(start.Args, "--standalone") {
		return definition.Signal{Skipped: SkippedStandalone}
	}
	content, ok := withPlugin(start.Env[EnvConfigContent], PluginDir(here.StateDir))
	if !ok {
		return definition.Signal{Skipped: SkippedEnv}
	}
	return definition.Signal{Wired: true, Env: map[string]string{
		EnvConfigContent: content,
		EnvBin:           here.Bin,
		EnvTeam:          team,
		EnvMember:        start.Short.String(),
	}}
}

// withPlugin is the profile's OPENCODE_CONFIG_CONTENT (empty: none) with
// hergy's plugin directory added to its "plugins"; false when the profile's is
// not a JSON object hergy can add to.
func withPlugin(content, dir string) (string, bool) {
	config := map[string]any{}
	if strings.TrimSpace(content) != "" {
		if err := json.Unmarshal([]byte(content), &config); err != nil || config == nil {
			return "", false
		}
	}
	var plugins []any
	switch have := config["plugins"].(type) {
	case nil:
	case []any:
		plugins = have
	default:
		return "", false
	}
	if !slices.Contains(plugins, any(dir)) {
		plugins = append(plugins, dir)
	}
	config["plugins"] = plugins
	return jsonText(config), true
}

//go:embed plugin.js
var plugin []byte

// pluginPackage names the plugin directory as a package, which is how opencode
// loads a local directory named in "plugins".
const pluginPackage = `{"name":"hergy-turns","version":"1.0.0","type":"module","main":"index.js"}` + "\n"

// PluginDir is the directory hergy's opencode plugin lives in, named in a
// wired member's OPENCODE_CONFIG_CONTENT: <stateDir>/opencode/hergy-turns.
func PluginDir(stateDir string) string {
	return filepath.Join(stateDir, "opencode", "hergy-turns")
}

// WritePlugin puts hergy's opencode plugin in PluginDir, as this hergy has it:
// index.js and its package.json. A file is written again whenever its text
// differs, so a newer hergy's plugin replaces an older one's and an unchanged
// one is left alone.
func WritePlugin(stateDir string) error {
	dir := PluginDir(stateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, text := range map[string][]byte{"index.js": plugin, "package.json": []byte(pluginPackage)} {
		if err := writeIfChanged(filepath.Join(dir, name), text); err != nil {
			return err
		}
	}
	return nil
}

func writeIfChanged(path string, text []byte) error {
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, text) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hergy-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(text); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// notifyLine is Codex's notify setting as config.toml writes it: an array of
// basic strings, which is also JSON.
var notifyLine = regexp.MustCompile(`(?m)^\s*notify\s*=\s*(\[[^\]]*\])`)

// CodexNotify is the notify command the person's Codex config.toml sets, if any
// that hergy can read (an array of double-quoted strings). It reads the file
// and writes nothing.
func CodexNotify(codexHome string) ([]string, bool) {
	if codexHome == "" {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		return nil, false
	}
	m := notifyLine.FindSubmatch(raw)
	if m == nil {
		return nil, false
	}
	var then []string
	if err := json.Unmarshal(m[1], &then); err != nil || len(then) == 0 {
		return nil, false
	}
	return then, true
}

// CodexHome is the person's Codex directory: CODEX_HOME, else ~/.codex.
func CodexHome() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex")
}

// jsonText is v as JSON with no HTML escaping, so a path or name reads as it is.
func jsonText(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimRight(b.String(), "\n")
}

// shellWords is the words as one shell command line, each quoted for sh.
func shellWords(words ...string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

// isCodexTitleTurn tells Codex's title turn by its input, or by a reply that is
// nothing but a title object.
func isCodexTitleTurn(inputs []string, reply string) bool {
	if len(inputs) == 1 && strings.HasPrefix(inputs[0], codexTitleTask) {
		return true
	}
	var title struct {
		Title *string `json:"title"`
	}
	trimmed := strings.TrimSpace(reply)
	return strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &title) == nil && title.Title != nil && !strings.Contains(trimmed, "\n")
}

// NotifyThen runs the person's own Codex notify (EnvNotifyThen, as Wire set
// it) with the same JSON argument, and does not wait for it: hergy replaced it
// for that process only (d-a70c). An empty or unreadable value runs nothing.
func NotifyThen(then, payload string) {
	var words []string
	if then == "" || json.Unmarshal([]byte(then), &words) != nil || len(words) == 0 {
		return
	}
	cmd := exec.Command(words[0], append(words[1:], payload)...) //nolint:gosec // G204: the person's own notify, from their config.toml
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

// ErrNoPayload is turn-done's answer when the harness handed it nothing.
var ErrNoPayload = errors.New("no payload")

// ErrNotATurn is Payload's answer for a harness's own side turn, which is not
// the member's: Codex's TUI asks the model for a session title after the first
// prompt, in a thread of its own, and notify reports that too (measured
// 2026-11-07: input-messages is the title instruction and the reply is
// {"title": …}). Nothing is handed on for it.
var ErrNotATurn = errors.New("not the member's turn")

// codexTitleTask opens the input of Codex's title turn.
const codexTitleTask = "Generate a concise, single-line task title"

// Payload is what a harness handed hergy turn-done, read as the turn's text
// and which harness spoke. Codex passes its JSON as one argument; Claude Code
// and the opencode plugin write theirs to stdin.
func Payload(arg string, stdin []byte) (text, source string, err error) {
	raw := []byte(arg)
	if arg == "" {
		raw = bytes.TrimSpace(stdin)
	}
	if len(raw) == 0 {
		return "", "", ErrNoPayload
	}
	var p struct {
		Type      string   `json:"type"`
		HookEvent string   `json:"hook_event_name"`
		Source    string   `json:"source"`
		Claude    string   `json:"last_assistant_message"`
		Codex     string   `json:"last-assistant-message"`
		Inputs    []string `json:"input-messages"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", err
	}
	switch {
	case p.Type == "agent-turn-complete":
		if isCodexTitleTurn(p.Inputs, p.Codex) {
			return "", "codex", ErrNotATurn
		}
		return p.Codex, "codex", nil
	case p.Source != "":
		return p.Claude, p.Source, nil
	case p.HookEvent == "Stop":
		return p.Claude, "claude", nil
	}
	return p.Claude + p.Codex, "unknown", nil
}
