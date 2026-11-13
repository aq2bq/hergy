package launchlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadIgnoresLegacyHiresWithLaunchIdentity(t *testing.T) {
	log := testLog(t)
	raw := `{"command":"up","team":"demo","definition":"demo","project":"/project","team_yaml":"/project/team.yaml","workspace":"workspace","workspace_id":"w1"}
{"command":"hire","team":"previous-alias","definition":"demo","project":"/previous","team_yaml":"/project/team.yaml","workspace":"workspace","workspace_id":"w1","hire":{"worker":"previous-alias-worker","from":"lead","purpose":"research","tab":"worker:previous-alias/worker"}}
{"command":"reprompt","team":"demo","definition":"demo","project":"/project","team_yaml":"/project/team.yaml"}
`
	if err := os.MkdirAll(log.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(log.Dir(), "log.jsonl")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	records, err := log.Read()
	if err != nil || len(records) != 2 {
		t.Fatalf("Read returned %+v err=%v; want only up and reprompt", records, err)
	}
	if records[0].Command != CommandUp || records[1].Command != CommandReprompt {
		t.Fatalf("history order=%+v", records)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != raw {
		t.Fatalf("reading rewrote history: %q %v", after, err)
	}
}
