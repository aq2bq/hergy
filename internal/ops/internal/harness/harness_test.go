package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
)

func here(t *testing.T) Here {
	t.Helper()
	return Here{Bin: "/opt/hergy bin/hergy", StateDir: filepath.Join(t.TempDir(), "state"), CodexHome: t.TempDir()}
}

// TestWireClaudePassesAStopHookAndLeavesAProfileWithSettingsAlone
func TestWireClaudePassesAStopHook(t *testing.T) {
	s := &definition.Start{Short: fixtureShortName("lead"), Kind: "claude", Args: []string{"--model", "opus"}, Env: map[string]string{"GY_ACTOR": "demo-lead"}}
	Wire(s, "demo", here(t))
	if !s.Signal.Wired || len(s.Signal.Args) != 2 || s.Signal.Args[0] != "--settings" {
		t.Fatalf("Signal = %+v, want --settings <json>", s.Signal)
	}
	want := `"command":"'/opt/hergy bin/hergy' 'turn-done' 'demo' 'lead'"`
	if !strings.Contains(s.Signal.Args[1], `"Stop"`) || !strings.Contains(s.Signal.Args[1], want) {
		t.Errorf("settings = %s, want a Stop hook running %s", s.Signal.Args[1], want)
	}
	if got := s.StartArgs(); len(got) != 4 || got[0] != "--model" || got[2] != "--settings" {
		t.Errorf("StartArgs = %v", got)
	}
	if got := s.PaneEnv(); len(got) != 1 || got["GY_ACTOR"] != "demo-lead" {
		t.Errorf("PaneEnv = %v, want the profile's alone", got)
	}
	own := &definition.Start{Short: fixtureShortName("lead"), Kind: "claude", Args: []string{"--settings", "{}"}}
	Wire(own, "demo", here(t))
	if own.Signal.Wired || own.Signal.Skipped != SkippedArgs {
		t.Errorf("a profile with its own --settings: %+v, want skipped for args", own.Signal)
	}
}

// TestWireCodexPassesNotifyAndKeepsThePersons: the person's notify in
// config.toml is handed to turn-done to call afterwards, not replaced (d-a70c).
func TestWireCodexPassesNotifyAndKeepsThePersons(t *testing.T) {
	h := here(t)
	if err := os.WriteFile(filepath.Join(h.CodexHome, "config.toml"), []byte("model = \"x\"\nnotify = [\"/Apps/Notify.app/notify\", \"turn-ended\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &definition.Start{Short: fixtureShortName("dev"), Kind: "codex", Args: []string{"-m", "gpt"}}
	Wire(s, "demo", h)
	if !s.Signal.Wired || len(s.Signal.Args) != 2 || s.Signal.Args[0] != "-c" {
		t.Fatalf("Signal = %+v", s.Signal)
	}
	if s.Signal.Args[1] != `notify=["/opt/hergy bin/hergy","turn-done","demo","dev"]` {
		t.Errorf("notify = %s", s.Signal.Args[1])
	}
	if s.Signal.Env[EnvNotifyThen] != `["/Apps/Notify.app/notify","turn-ended"]` {
		t.Errorf("then = %q, want the person's notify", s.Signal.Env[EnvNotifyThen])
	}
	// No config.toml, or no notify in it: nothing to chain.
	bare := &definition.Start{Short: fixtureShortName("dev"), Kind: "codex"}
	Wire(bare, "demo", Here{Bin: "hergy", CodexHome: t.TempDir()})
	if !bare.Signal.Wired || len(bare.Signal.Env) != 0 {
		t.Errorf("without a notify: %+v, want wired with no env", bare.Signal)
	}
	own := &definition.Start{Short: fixtureShortName("dev"), Kind: "codex", Args: []string{"-c", "notify=[\"x\"]"}}
	Wire(own, "demo", h)
	if own.Signal.Wired || own.Signal.Skipped != SkippedArgs {
		t.Errorf("a profile with its own notify: %+v", own.Signal)
	}
}

// TestWireOpencodePointsAtThePluginOnlyWhenStandalone (d-e511)
func TestWireOpencodePointsAtThePluginOnlyWhenStandalone(t *testing.T) {
	h := here(t)
	// The profile's own inline config (its model) is kept, with hergy's plugin
	// directory added to its plugins.
	s := &definition.Start{Short: fixtureShortName("spec"), Kind: "opencode", Args: []string{"--standalone"}, Env: map[string]string{"GY_ACTOR": "demo-spec", EnvConfigContent: `{"model":"opencode-go/x","plugins":["other"]}`}}
	Wire(s, "demo", h)
	if !s.Signal.Wired || len(s.Signal.Args) != 0 {
		t.Fatalf("Signal = %+v, want env only", s.Signal)
	}
	env := s.PaneEnv()
	if env[EnvBin] != h.Bin || env[EnvTeam] != "demo" || env[EnvMember] != "spec" || env["GY_ACTOR"] != "demo-spec" {
		t.Errorf("PaneEnv = %v", env)
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(env[EnvConfigContent]), &content); err != nil {
		t.Fatalf("OPENCODE_CONFIG_CONTENT = %q: %v", env[EnvConfigContent], err)
	}
	if content["model"] != "opencode-go/x" || fmt.Sprint(content["plugins"]) != fmt.Sprint([]any{"other", PluginDir(h.StateDir)}) {
		t.Errorf("OPENCODE_CONFIG_CONTENT = %v, want the model kept and the plugin added", content)
	}
	if _, has := env["OPENCODE_CONFIG_DIR"]; has {
		t.Error("OPENCODE_CONFIG_DIR is set, which would replace the person's global directory")
	}
	// No inline config in the profile: hergy's plugin alone.
	bare := &definition.Start{Short: fixtureShortName("spec"), Kind: "opencode", Args: []string{"--standalone"}}
	Wire(bare, "demo", h)
	if bare.Signal.Env[EnvConfigContent] != `{"plugins":["`+PluginDir(h.StateDir)+`"]}` {
		t.Errorf("bare OPENCODE_CONFIG_CONTENT = %q", bare.Signal.Env[EnvConfigContent])
	}
	shared := &definition.Start{Short: fixtureShortName("spec"), Kind: "opencode"}
	Wire(shared, "demo", h)
	if shared.Signal.Wired || shared.Signal.Skipped != SkippedStandalone {
		t.Errorf("without --standalone: %+v", shared.Signal)
	}
	own := &definition.Start{Short: fixtureShortName("spec"), Kind: "opencode", Args: []string{"--standalone"}, Env: map[string]string{EnvConfigContent: "not json"}}
	Wire(own, "demo", h)
	if own.Signal.Wired || own.Signal.Skipped != SkippedEnv {
		t.Errorf("with an inline config hergy cannot add to: %+v", own.Signal)
	}
	other := &definition.Start{Short: fixtureShortName("g"), Kind: "gemini"}
	Wire(other, "demo", h)
	if other.Signal.Wired || other.Signal.Skipped != SkippedKind {
		t.Errorf("a harness without a signal: %+v", other.Signal)
	}
}

func TestWritePluginWritesOnceAndAgainWhenItDiffers(t *testing.T) {
	state := t.TempDir()
	if err := WritePlugin(state); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(PluginDir(state), "index.js")
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), `"turn-done"`) || !strings.Contains(string(raw), "session.execution.") {
		t.Fatalf("plugin = %q, %v", raw, err)
	}
	if pkg, err := os.ReadFile(filepath.Join(PluginDir(state), "package.json")); err != nil || !strings.Contains(string(pkg), `"main":"index.js"`) {
		t.Fatalf("package.json = %q, %v", pkg, err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WritePlugin(state); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); string(again) == "old" {
		t.Error("an older plugin was left in place")
	}
}

func TestPayloadReadsEachHarness(t *testing.T) {
	cases := []struct {
		arg, stdin         string
		wantText, wantFrom string
	}{
		{`{"type":"agent-turn-complete","thread-id":"t","last-assistant-message":"pong"}`, "", "pong", "codex"},
		{"", `{"session_id":"s","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"done\n→ spec"}`, "done\n→ spec", "claude"},
		{"", `{"source":"opencode","last_assistant_message":"ok"}`, "ok", "opencode"},
	}
	for _, c := range cases {
		text, from, err := Payload(c.arg, []byte(c.stdin))
		if err != nil || text != c.wantText || from != c.wantFrom {
			t.Errorf("Payload(%q, %q) = %q, %q, %v", c.arg, c.stdin, text, from, err)
		}
	}
	// Codex's title turn is not the member's turn (ErrNotATurn), told by its
	// input or by its reply.
	for _, title := range []string{
		`{"type":"agent-turn-complete","thread-id":"t2","input-messages":["Generate a concise, single-line task title of at most 36 characters.\n\nUser prompt:\nreply pong"],"last-assistant-message":"{\"title\":\"Reply pong\"}"}`,
		`{"type":"agent-turn-complete","thread-id":"t2","input-messages":["x"],"last-assistant-message":"{\"title\":\"開発タスクを実装する\"}"}`,
	} {
		if _, from, err := Payload(title, nil); !errors.Is(err, ErrNotATurn) || from != "codex" {
			t.Errorf("the title turn %s = %q, %v; want ErrNotATurn", title, from, err)
		}
	}
	if text, _, err := Payload(`{"type":"agent-turn-complete","input-messages":["x"],"last-assistant-message":"{\"title\":\"a\"}\nmore"}`, nil); err != nil || text == "" {
		t.Errorf("a reply that only starts like a title object was dropped: %q, %v", text, err)
	}
	if _, _, err := Payload("", []byte("  \n")); !errors.Is(err, ErrNoPayload) {
		t.Errorf("empty: %v, want ErrNoPayload", err)
	}
	if _, _, err := Payload("not json", nil); err == nil {
		t.Error("bad JSON accepted")
	}
}
