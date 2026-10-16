package options

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		kind    Kind
		dry     bool
		ai      bool
		noOpen  bool
		wantErr bool
	}{
		{name: "no arguments prints usage", kind: Help},
		{name: "unknown command", argv: []string{"frobnicate"}, wantErr: true},
		{name: "unknown flag", argv: []string{"up", "--frobnicate"}, wantErr: true},
		{name: "unknown flag with a value", argv: []string{"up", "--frobnicate=1"}, wantErr: true},
		{name: "a flag that belongs to another command", argv: []string{"down", "--profile", "p"}, wantErr: true},
		{name: "version word", argv: []string{"version"}, kind: Version},
		{name: "version flag", argv: []string{"--version"}, kind: Version},
		{name: "up with dry", argv: []string{"up", "--dry"}, kind: Up, dry: true},
		{name: "ls", argv: []string{"ls"}, kind: Ls},
		{name: "up with --as", argv: []string{"up", "demo", "--as", "alpha"}, kind: Up},
		{name: "--as is not allowed on down", argv: []string{"down", "alpha", "--as", "a"}, wantErr: true},
		{name: "--as is not allowed on status", argv: []string{"status", "alpha", "--as", "a"}, wantErr: true},
		{name: "--as is not allowed on refresh", argv: []string{"refresh", "lead", "--as", "a"}, wantErr: true},
		{name: "--as is not allowed on reprompt", argv: []string{"reprompt", "lead", "--as", "a"}, wantErr: true},
		{name: "--as is not allowed on hire", argv: []string{"hire", "w", "--as", "a"}, wantErr: true},
		{name: "status", argv: []string{"status"}, kind: Status},
		{name: "status with a team", argv: []string{"status", "demo"}, kind: Status},
		{name: "status rejects --dry", argv: []string{"status", "--dry"}, wantErr: true},
		{name: "status rejects --json", argv: []string{"status", "--json"}, wantErr: true},
		{name: "status takes at most one team", argv: []string{"status", "a", "b"}, wantErr: true},
		{name: "--as must match the name pattern", argv: []string{"up", "--as", "Alpha"}, wantErr: true},
		{name: "--as is not allowed on ls", argv: []string{"ls", "--as", "alpha"}, wantErr: true},
		{name: "cheat", argv: []string{"cheat"}, kind: Cheat},
		{name: "cheat --ai", argv: []string{"cheat", "--ai"}, kind: Cheat, ai: true},
		{name: "cheat --lang", argv: []string{"cheat", "--lang", "ja"}, kind: Cheat},
		{name: "cheat rejects --dry", argv: []string{"cheat", "--dry"}, wantErr: true},
		{name: "cheat rejects --json", argv: []string{"cheat", "--json"}, wantErr: true},
		{name: "cheat takes no positional arguments", argv: []string{"cheat", "extra"}, wantErr: true},
		{name: "hire with a purpose, a task and no profile", argv: []string{"hire", "w", "--purpose", "x", "--task", "t"}, kind: Hire},
		{name: "hire without a purpose", argv: []string{"hire", "w", "--task", "t"}, wantErr: true},
		{name: "hire without a task", argv: []string{"hire", "w", "--purpose", "x"}, wantErr: true},
		{name: "serve", argv: []string{"serve"}, kind: Serve},
		{name: "serve takes no positional arguments", argv: []string{"serve", "demo"}, wantErr: true},
		{name: "serve rejects --dry", argv: []string{"serve", "--dry"}, wantErr: true},
		{name: "serve rejects --global", argv: []string{"serve", "--global"}, wantErr: true},
		{name: "serve --port", argv: []string{"serve", "--port", "9000"}, kind: Serve},
		{name: "--port is not allowed on up", argv: []string{"up", "--port", "9000"}, wantErr: true},
		{name: "serve --port must be a number", argv: []string{"serve", "--port", "nine"}, wantErr: true},
		{name: "serve --port is at most 65535", argv: []string{"serve", "--port", "65536"}, wantErr: true},
		{name: "serve --port is at least 0", argv: []string{"serve", "--port", "-1"}, wantErr: true},
		{name: "serve --no-open", argv: []string{"serve", "--no-open"}, kind: Serve, noOpen: true},
		{name: "serve opens by default", argv: []string{"serve"}, kind: Serve},
		{name: "--no-open is not allowed on up", argv: []string{"up", "--no-open"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.argv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%v) = %+v, want an error", tt.argv, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v): %v", tt.argv, err)
			}
			if got.Kind != tt.kind {
				t.Fatalf("Parse(%v).Kind = %d, want %d", tt.argv, got.Kind, tt.kind)
			}
			if got.Dry != tt.dry {
				t.Fatalf("Parse(%v).Dry = %v, want %v", tt.argv, got.Dry, tt.dry)
			}
			if got.AI != tt.ai {
				t.Fatalf("Parse(%v).AI = %v, want %v", tt.argv, got.AI, tt.ai)
			}
			if got.NoOpen != tt.noOpen {
				t.Fatalf("Parse(%v).NoOpen = %v, want %v", tt.argv, got.NoOpen, tt.noOpen)
			}
		})
	}
}

// TestServePortDefaults: without --port hergy serve listens on DefaultPort, and
// 0 is the way to ask the operating system for a free one.
func TestServePortDefaults(t *testing.T) {
	got, err := Parse([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != DefaultPort {
		t.Errorf("hergy serve defaults to port %d, want %d", got.Port, DefaultPort)
	}
	got, err = Parse([]string{"serve", "--port", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 0 {
		t.Errorf("hergy serve --port 0 gives port %d, want 0", got.Port)
	}
}

// TestKindsCoversEveryCommand: Kinds is what the cheatsheet tests walk, so a
// command in the parser table, or Version, that is missing from it would leave
// that command unchecked.
func TestKindsCoversEveryCommand(t *testing.T) {
	inKinds := map[Kind]bool{}
	for _, kind := range Kinds {
		inKinds[kind] = true
	}
	for kind := range allowed {
		if !inKinds[kind] {
			t.Errorf("Kinds misses the command %q from the allowed table", commandName(kind))
		}
	}
	if !inKinds[Version] {
		t.Errorf("Kinds misses the command %q", commandName(Version))
	}
}
