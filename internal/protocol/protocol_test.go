package protocol

import (
	"strings"
	"testing"
)

func TestResult(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		stderr  string
		status  int
		code    string // what ErrorCode must return
		wantErr bool
	}{
		{
			name:   "result object",
			stdout: `{"result":{"pane":{"pane_id":"p1"}}}`,
		},
		{
			name:    "failure with the code on stderr",
			stderr:  `{"error":{"code":"agent_pane_busy"}}`,
			status:  1,
			code:    "agent_pane_busy",
			wantErr: true,
		},
		{
			name:    "failure with the code on stdout instead",
			stdout:  `{"error":{"code":"agent_pane_busy"}}`,
			status:  2,
			code:    "agent_pane_busy",
			wantErr: true,
		},
		{
			name:    "failure with no code at all",
			stderr:  "herdr: boom",
			status:  1,
			wantErr: true,
		},
		{
			name:    "error object with a zero exit status",
			stdout:  `{"error":{"code":"pane_gone"}}`,
			code:    "pane_gone",
			wantErr: true,
		},
		{
			name:    "reply with no result object",
			stdout:  `{"ok":true}`,
			wantErr: true,
		},
		{
			name:    "reply that is not JSON",
			stdout:  "not json",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Result([]string{"agent", "start", "worker"}, tt.stdout, tt.stderr, tt.status)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Result returned no error; result = %v", result)
				}
				if got := ErrorCode(err); got != tt.code {
					t.Fatalf("ErrorCode = %q, want %q (err: %v)", got, tt.code, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Result: %v", err)
			}
			got, err := DigString(result, "pane", "pane_id")
			if err != nil || got != "p1" {
				t.Fatalf("result pane_id = %q, %v; want p1", got, err)
			}
		})
	}
}

func TestCheckHelp(t *testing.T) {
	const subs = "  herdr agent get\n  herdr agent prompt\n  herdr agent start\n  herdr agent list\n"
	const full = "Usage: herdr agent <command>\n" + subs
	tests := []struct {
		name    string
		help    string
		status  int
		wantErr bool
	}{
		{
			name:   "every needed command is advertised",
			help:   full,
			status: 0,
		},
		{
			name:   "a missing subcommand may exit 2",
			help:   full,
			status: 2,
		},
		{
			name:    "an unexpected exit status is an error",
			help:    full,
			status:  1,
			wantErr: true,
		},
		{
			name:    "one subcommand is not advertised",
			help:    strings.Replace(full, "  herdr agent list\n", "", 1),
			status:  0,
			wantErr: true,
		},
		{
			name:    "the group itself is not advertised",
			help:    "unknown command\n",
			status:  0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckHelp("agent", []string{"get", "prompt", "start", "list"}, tt.help, tt.status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckHelp = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
