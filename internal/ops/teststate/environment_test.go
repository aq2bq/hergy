package teststate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

func TestHelpersRequireExplicitAbsoluteStateHome(t *testing.T) {
	for _, root := range []string{"", "relative-state"} {
		for _, action := range []string{"write", "hire"} {
			t.Run(action+"/"+root, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("XDG_CONFIG_HOME", t.TempDir())
				t.Setenv("XDG_STATE_HOME", root)
				var err error
				if action == "write" {
					err = Write(Team{Name: "demo", Definition: "demo", Project: t.TempDir()})
				} else {
					err = Hire("demo", Worker{Name: "demo-worker"})
				}
				if err == nil || !strings.Contains(err.Error(), "XDG_STATE_HOME") {
					t.Fatalf("%s without absolute state home: %v", action, err)
				}
				if _, err := os.Stat(filepath.Join(home, ".local", "state", "hergy")); !os.IsNotExist(err) {
					t.Fatalf("helper accessed fallback state: %v", err)
				}
			})
		}
	}
}

func TestExplicitStateHomeKeepsTeamAndWorkerAwayFromHome(t *testing.T) {
	home, root, project := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", root)
	if err := Write(Team{Name: "demo", Definition: "demo", Project: project}); err != nil {
		t.Fatal(err)
	}
	if err := Hire("demo", Worker{Name: "demo-worker", From: "lead"}); err != nil {
		t.Fatal(err)
	}
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	name, _ := domainname.ParseTeamName("demo")
	state, exists, err := store.Read(name)
	if err != nil || !exists || len(state.Workers()) != 1 || state.Workers()[0].Name != "demo-worker" {
		t.Fatalf("explicit state: exists=%t workers=%+v err=%v", exists, state.Workers(), err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "hergy")); !os.IsNotExist(err) {
		t.Fatalf("helper used home: %v", err)
	}
}
