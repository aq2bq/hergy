package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

type renameActivity struct {
	fakeActivity
	refusal error
}

func (a renameActivity) CheckDefinitionRename(string) error { return a.refusal }

func TestTeamRenameFilesAndRefusals(t *testing.T) {
	for _, problem := range []string{"", "taken", "format", "team changed", "agreement changed", "agreement appeared", "running", "unavailable", "locked"} {
		t.Run(problem, func(t *testing.T) {
			f := newFixture(t)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.root, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(f.root, "state"))
			old := filepath.Join(f.proj, ".hergy", "teams", "demo")
			teamFile := filepath.Join(old, "team.yaml")
			agreeFile := filepath.Join(old, "agreement.md")
			before, _ := os.ReadFile(teamFile)
			before = append([]byte("# preserved\nteam: 'demo' # name\n"), before...)
			if err := os.WriteFile(teamFile, before, 0o644); err != nil {
				t.Fatal(err)
			}
			if problem == "agreement appeared" {
				if err := os.Remove(agreeFile); err != nil {
					t.Fatal(err)
				}
			}
			agree, agreeErr := os.ReadFile(agreeFile)
			in := map[string]any{"place": "project", "rename_to": "next", "hash": source.Hash(string(before)), "agreement_hash": source.Hash(string(agree)), "agreement_exists": agreeErr == nil}
			act := renameActivity{}
			want := http.StatusConflict
			switch problem {
			case "taken":
				f.writeTeam(t, "next", "taken")
			case "format":
				in["rename_to"] = "../bad"
				want = http.StatusUnprocessableEntity
			case "team changed":
				before = append(before, []byte("# changed\n")...)
				if err := os.WriteFile(teamFile, before, 0o644); err != nil {
					t.Fatal(err)
				}
			case "agreement changed", "agreement appeared":
				agree = []byte("# changed\n")
				if err := os.WriteFile(agreeFile, agree, 0o644); err != nil {
					t.Fatal(err)
				}
			case "running":
				act.refusal = &HQError{Status: http.StatusConflict, Text: "チームを止めてから名前を変更してください"}
			case "unavailable":
				act.refusal = &HQError{Status: http.StatusServiceUnavailable, Text: "herdrを確認してください"}
				want = http.StatusServiceUnavailable
			case "locked":
				release, err := source.LockTeams(teamFile)
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			default:
				want = http.StatusOK
			}
			server, err := New(f.catalog(), act)
			if err != nil {
				t.Fatal(err)
			}
			resp, body := f.api(t, server, http.MethodPost, "/api/team/demo/rename", in)
			if resp.StatusCode != want {
				t.Fatalf("status %d want %d: %s", resp.StatusCode, want, body)
			}
			if problem != "" {
				got, _ := os.ReadFile(teamFile)
				if string(got) != string(before) {
					t.Fatal("refusal changed team.yaml")
				}
				got, _ = os.ReadFile(agreeFile)
				if string(got) != string(agree) {
					t.Fatal("refusal changed agreement.md")
				}
				return
			}
			if _, err := os.Stat(old); !os.IsNotExist(err) {
				t.Fatal("old directory remains")
			}
			got, err := os.ReadFile(filepath.Join(filepath.Dir(old), "next", "team.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != strings.Replace(string(before), "team: 'demo'", "team: 'next'", 1) {
				t.Fatalf("unexpected bytes: %s", got)
			}
			got, _ = os.ReadFile(filepath.Join(filepath.Dir(old), "next", "agreement.md"))
			if string(got) != string(agree) {
				t.Fatal("agreement changed")
			}
		})
	}
}
