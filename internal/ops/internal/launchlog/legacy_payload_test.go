package launchlog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyPayloadAndBlobsDoNotAffectHistory(t *testing.T) {
	log := testLog(t)
	blob := filepath.Join(log.Dir(), "blobs", "retained")
	if err := os.MkdirAll(filepath.Dir(blob), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blob, []byte("old prompt body"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := `{"time":"2026-11-01T01:00:00Z","command":"up","project":"/project","team":"alias","team_yaml":"/project/team.yaml","workspace":"workspace","workspace_id":"w1","hergy":"hergy old","definition":"demo","layer":"local","hash":"old hash","executable":"/old/hergy","lang":"ja","members":[{"name":"alias-lead","short":"lead","prompt":"retained","docs":["/old/role.md"],"start":"old start","env":"old env"}],"docs":[{"path":"/old/role.md","layer":"local","revision":3,"sha256":"retained"}],"workers":[{"from":"lead","purpose":"research","profile":"old","start":"old start","env":"old env"}]}
`
	path := filepath.Join(log.Dir(), "log.jsonl")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	records, err := log.Read()
	want := Record{Time: "2026-11-01T01:00:00Z", Command: CommandUp, Project: projectPath("/project").Seen(), Team: fixtureSeen("alias"), TeamYAML: projectPath("/project/team.yaml").Seen(), Workspace: "workspace", WorkspaceID: "w1"}
	if err != nil || !reflect.DeepEqual(records, []Record{want}) {
		t.Fatalf("legacy metadata=%+v err=%v", records, err)
	}
	if got, found, err := log.Latest(projectPath("/project/team.yaml")); err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy latest=%+v found=%t err=%v", got, found, err)
	}
	if err := log.Append(Record{Command: CommandReprompt, Team: fixtureSeen("alias")}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || len(after) <= len(raw) || string(after[:len(raw)]) != raw {
		t.Fatalf("append changed old history: %q %v", after, err)
	}
	old, err := os.ReadFile(blob)
	if err != nil || string(old) != "old prompt body" {
		t.Fatalf("legacy blob changed: %q %v", old, err)
	}
}
