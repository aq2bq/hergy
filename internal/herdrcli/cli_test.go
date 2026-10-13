package herdrcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/protocol"
)

// advertised is every subcommand the launcher may need, as a help text shows it.
var advertised = []string{
	"herdr agent get", "herdr agent prompt", "herdr agent start", "herdr agent list",
	"herdr pane current", "herdr pane split", "herdr pane rename", "herdr pane close",
	"herdr tab create", "herdr tab list",
}

// fakeHerdr writes a stand-in herdr executable and returns a CLI that runs it
// through the documented HERGY_HERDR override.
func fakeHerdr(t *testing.T, script string) CLI {
	t.Helper()
	path := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_HERDR", path)
	return New(os.Getenv("HERGY_HERDR"))
}

// helpScript prints the given help text for every invocation and exits like
// `herdr <group>` does when the command is missing.
func helpScript(commands []string) string {
	return "#!/bin/sh\ncat <<'HELP'\n" + strings.Join(commands, "\n") + "\nHELP\nexit 2\n"
}

func TestJSONReadsTheReply(t *testing.T) {
	cli := fakeHerdr(t, "#!/bin/sh\nprintf '%s' '{\"result\":{\"pane\":{\"workspace_id\":\"w1\"}}}'\n")

	result, err := cli.JSON("pane", "current", "--current")
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	got, err := protocol.DigString(result, "pane", "workspace_id")
	if err != nil || got != "w1" {
		t.Fatalf("workspace_id = %q, %v; want w1", got, err)
	}
}

func TestJSONKeepsTheErrorCode(t *testing.T) {
	cli := fakeHerdr(t, "#!/bin/sh\nprintf '%s' '{\"error\":{\"code\":\"agent_pane_busy\"}}' >&2\nexit 1\n")

	if _, err := cli.JSON("agent", "start", "worker"); err == nil {
		t.Fatal("JSON returned no error for a failed herdr command")
	} else if got := protocol.ErrorCode(err); got != "agent_pane_busy" {
		t.Fatalf("ErrorCode = %q, want agent_pane_busy", got)
	}
}

func TestVerifyAcceptsAdvertisedCommands(t *testing.T) {
	for _, mode := range []string{protocol.ModeBoot, protocol.ModeRefresh, protocol.ModeDown} {
		t.Run(mode, func(t *testing.T) {
			if err := fakeHerdr(t, helpScript(advertised)).Verify(mode); err != nil {
				t.Fatalf("Verify(%s): %v", mode, err)
			}
		})
	}
}

func TestVerifyRejectsAMissingCommand(t *testing.T) {
	var without []string
	for _, command := range advertised {
		if command != "herdr agent list" {
			without = append(without, command)
		}
	}
	if err := fakeHerdr(t, helpScript(without)).Verify(protocol.ModeBoot); err == nil {
		t.Fatal("Verify accepted help that does not advertise `herdr agent list`")
	}
}

func TestVerifyFailsWhenHerdrIsMissing(t *testing.T) {
	cli := New(filepath.Join(t.TempDir(), "no-such-herdr"))
	if err := cli.Verify(protocol.ModeBoot); err == nil {
		t.Fatal("Verify accepted a herdr that cannot be executed")
	}
}
