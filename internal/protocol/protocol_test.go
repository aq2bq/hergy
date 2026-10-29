package protocol

import (
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
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

// TestRequiredCommandsStatus: a status run reads the live agents, their screens
// and the live tabs, and nothing else.
func TestRequiredCommandsStatus(t *testing.T) {
	var got []string
	for _, group := range RequiredCommands(ModeStatus) {
		for _, sub := range group.Subs {
			got = append(got, group.Name+" "+sub)
		}
	}
	want := []string{"agent list", "agent read", "tab list"}
	if !slices.Equal(got, want) {
		t.Fatalf("RequiredCommands(%s) needs %v, want %v", ModeStatus, got, want)
	}
}

// TestAgentFromTitle: the terminal title is the stripped one when Herdr reports
// it, and the raw one otherwise.
func TestAgentFromTitle(t *testing.T) {
	tests := []struct {
		name   string
		object map[string]any
		want   string
	}{
		{
			name: "the stripped title",
			object: map[string]any{
				"name": "dev-lead", "agent": "claude", "agent_status": "working",
				"pane_id": "p1", "tab_id": "t1",
				"terminal_title": "✳ 報告を待っている", "terminal_title_stripped": "報告を待っている",
			},
			want: "報告を待っている",
		},
		{
			name: "only the raw title",
			object: map[string]any{
				"name": "dev-lead", "agent_status": "idle", "terminal_title": "OpenCode",
			},
			want: "OpenCode",
		},
		{
			name: "no title",
			object: map[string]any{
				"name": "dev-lead", "agent_status": "idle",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := AgentFrom(tt.object)
			if agent.Title != tt.want {
				t.Fatalf("AgentFrom title = %q, want %q", agent.Title, tt.want)
			}
			if agent.PaneID != tt.object["pane_id"] && tt.object["pane_id"] != nil {
				t.Fatalf("AgentFrom pane = %q, want %v", agent.PaneID, tt.object["pane_id"])
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

// TestMessageIsHerdrsReasonInWords: a Herdr failure gives its own message as the
// reason a person reads, and a reply with no message does not hand the JSON on
// (d-5f02).
func TestMessageIsHerdrsReasonInWords(t *testing.T) {
	if err := i18n.Init(); err != nil {
		t.Fatal(err)
	}
	withReason := Failure([]string{"agent", "start"}, "",
		`{"error":{"code":"profile_unknown","message":"the profile coder is not installed"}}`, 1)
	if got := Message(withReason); got != "the profile coder is not installed" {
		t.Errorf("Message = %q, want herdr's own reason", got)
	}
	if got := ErrorCode(withReason); got != "profile_unknown" {
		t.Errorf("ErrorCode = %q, want profile_unknown", got)
	}
	withoutReason := Failure([]string{"agent", "start"}, "", `{"error":{"code":"nope"}}`, 1)
	if got := Message(withoutReason); got != i18n.T("error.herdr_no_message") {
		t.Errorf("Message = %q, want the sentence for a reply with no reason", got)
	}
	plain := Failure([]string{"agent", "start"}, "", "herdr: command not found", 127)
	if got := Message(plain); got != "herdr: command not found" {
		t.Errorf("Message = %q, want the command's own words", got)
	}
}
