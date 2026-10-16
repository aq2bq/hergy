package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/status"
	"github.com/aq2bq/hergy/internal/web"
)

// teamviewHerdr is a fake herdr reporting the demo team the way herdr does when
// a team is up: its tab carries the team definition it was booted from, one
// member is working with a terminal title, and one is waiting on a human. It
// records the target of every agent focus call in the file HERGY_TEST_FOCUS_LOG
// names, so a test can read what the one write of these screens asked for.
const teamviewHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","terminal_title_stripped":"設計を書いている"},{"name":"demo-builder","agent":"claude","agent_status":"blocked","pane_id":"p2","tab_id":"t1"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  "agent focus") printf '%s\n' "$3" >> "$HERGY_TEST_FOCUS_LOG"; echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// foreignHerdr is a fake herdr reporting a team this layer has no definition
// for: it runs in another project, and these screens list it without its rows.
const foreignHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"other-lead","agent":"claude","agent_status":"idle","pane_id":"p9","tab_id":"t9"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t9","label":"team:other@other"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// TestServePortDefinition: the definition screen reads one definition in full
// the way hergy show reads it: the documents every member reads, each member
// with its role file and the prompt hergy hands it.
func TestServePortDefinition(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	port := servePort{env: serveEnv(t)}

	def, err := port.Definition(web.LayerLocal, "demo")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if def.Name != "demo" || def.Layer != web.LayerLocal {
		t.Errorf("Definition = %q of %q, want demo of the local layer", def.Name, def.Layer)
	}
	if want := filepath.Join(".hergy", "teams", "demo", "team.yaml"); def.Path != want {
		t.Errorf("the definition path is %q, want %q", def.Path, want)
	}
	if def.Broken != "" {
		t.Fatalf("a definition that reads came back broken: %s", def.Broken)
	}

	if len(def.Docs) != 1 {
		t.Fatalf("the definition reads %d documents, want the one agreement:", len(def.Docs))
	}
	doc := def.Docs[0]
	// Key is the file itself, which is what a link carries; Path is the file
	// as a person writes it, which is what the screen shows.
	if want := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"); doc.Key != want {
		t.Errorf("the document key is %q, want %q", doc.Key, want)
	}
	if want := filepath.Join(".hergy", "teams", "demo", "agreement.yaml"); doc.Path != want {
		t.Errorf("the document path is %q, want %q", doc.Path, want)
	}
	if doc.Layer != web.LayerLocal || doc.Label != "agreement.yaml" || doc.Role != "" {
		t.Errorf("the document is %+v, want a local document every member reads", doc)
	}
	if len(doc.Rules) != 2 || doc.Rules[0].ID != "C1" || doc.Rules[0].Text != "one" || doc.Rules[1].ID != "C2" {
		t.Errorf("the document carries %+v, want the rules C1 and C2", doc.Rules)
	}
	if _, ok := def.Doc(doc.Key); !ok {
		t.Error("the definition does not find its own document by key")
	}

	lead, ok := def.Member("lead")
	if !ok {
		t.Fatal("the definition has no member lead")
	}
	if lead.Herdr != "demo-lead" || lead.Role != "lead" || lead.Kind != "claude" {
		t.Errorf("lead is %+v, want demo-lead reading the lead role through claude", lead)
	}
	if lead.RoleFile == nil {
		t.Fatal("lead reads no role file")
	}
	if want := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"); lead.RoleFile.Key != want {
		t.Errorf("lead's role file is %q, want %q", lead.RoleFile.Key, want)
	}
	if want := filepath.Join(".hergy", "roles", "lead.yaml"); lead.RoleFile.Path != want {
		t.Errorf("lead's role file reads as %q, want %q", lead.RoleFile.Path, want)
	}
	if lead.RoleFile.Layer != web.LayerLocal || lead.RoleFile.Role != "lead" {
		t.Errorf("lead's role file is %+v, want a local file of the lead role", lead.RoleFile)
	}
	if len(lead.RoleFile.Rules) != 1 || lead.RoleFile.Rules[0].ID != "L1" {
		t.Errorf("lead's role file carries %+v, want the rule L1", lead.RoleFile.Rules)
	}
	for _, want := range []string{
		"You are demo-lead (lead) on team demo.", // who the member is
		"three",                                  // the rule of its role file
		"one",                                    // a rule of the document every member reads
		"demo-builder",                           // the rest of the team, to talk to
	} {
		if !strings.Contains(lead.Prompt, want) {
			t.Errorf("lead's prompt does not contain %q:\n%s", want, lead.Prompt)
		}
	}
	if _, ok := def.Doc(lead.RoleFile.Key); !ok {
		t.Error("the definition does not find a member's role file by key")
	}

	builder, ok := def.Member("builder")
	if !ok || builder.RoleFile == nil || builder.RoleFile.Rules[0].Text != "four" {
		t.Errorf("builder is %+v, want the builder role file with its own rule", builder)
	}
	if strings.Contains(builder.Prompt, "three") {
		t.Error("builder's prompt carries the lead's role file")
	}
}

// TestServePortDefinitionSaysWhyItIsBroken: a definition that cannot be read
// comes back with the one line hergy validate writes for it, and nothing else,
// so the screen shows the same line the command shows.
func TestServePortDefinitionSaysWhyItIsBroken(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "broken", "team.yaml"), "version: 2\n")
	port := servePort{env: serveEnv(t)}

	def, err := port.Definition(web.LayerLocal, "broken")
	if err != nil {
		t.Fatalf("Definition of a broken file returned an error: %v", err)
	}
	if def.Broken == "" {
		t.Fatal("a definition that cannot be read came back whole")
	}
	if !strings.Contains(def.Broken, def.Path) {
		t.Errorf("the reason %q does not name the file %q", def.Broken, def.Path)
	}
	if len(def.Docs) != 0 || len(def.Members) != 0 {
		t.Errorf("a broken definition carries %d documents and %d members, want none", len(def.Docs), len(def.Members))
	}

	if _, err := port.Definition(web.LayerLocal, "nobody"); err == nil {
		t.Error("Definition of a name the layer does not have returned no error")
	}
}

// TestServePortRunningReadsHerdr: the running screen reads herdr the way hergy
// status reads it, and carries what hergy status does not print but the screen
// shows: the member's role, the terminal title, and the pane.
func TestServePortRunningReadsHerdr(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	f.herdr(t, teamviewHerdr)
	port := servePort{env: serveEnv(t)}

	live, err := port.Running(web.LayerLocal)
	if err != nil {
		t.Fatalf("Running: %v", err)
	}
	if live.At.IsZero() || !live.At.Before(time.Now().Add(time.Minute)) {
		t.Errorf("Running stamped %v, want the time it read herdr", live.At)
	}
	if len(live.Teams) != 1 {
		t.Fatalf("herdr reports %d teams, want the one demo team", len(live.Teams))
	}
	demo := live.Teams[0]
	if demo.Team != "demo" || demo.Definition != "demo" || !demo.Known {
		t.Errorf("the running team is %+v, want demo built from the demo definition", demo)
	}
	if demo.TabID != "t1" {
		t.Errorf("the running team sits in tab %q, want t1", demo.TabID)
	}
	if demo.Broken != "" || len(demo.Rows) != 2 {
		t.Fatalf("the running team is %+v, want two rows and no reason", demo)
	}

	lead, ok := demo.Row("demo-lead")
	if !ok {
		t.Fatalf("the running team has no row for demo-lead: %+v", demo.Rows)
	}
	if lead.State != "working" || !lead.Live || lead.PaneID != "p1" {
		t.Errorf("demo-lead is %+v, want working in pane p1", lead)
	}
	if lead.Title != "設計を書いている" {
		t.Errorf("demo-lead's terminal says %q, want what herdr reports", lead.Title)
	}
	if lead.Role != "lead" || lead.Kind != "claude" || lead.Worker {
		t.Errorf("demo-lead is %+v, want a member reading the lead role through claude", lead)
	}
	builder, ok := demo.Row("demo-builder")
	if !ok || builder.State != "blocked" || builder.PaneID != "p2" || !builder.Live {
		t.Errorf("demo-builder is %+v, want blocked in pane p2", builder)
	}
	if verdict := status.Judge(demo.Rows).Text(); !strings.Contains(verdict, "demo-builder") {
		t.Errorf("the judgement is %q, want it to name the member waiting on the human", verdict)
	}
}

// TestServePortRunningListsATeamItCannotBuild: a team booted from another
// project's definition is listed, without its rows, as hergy status lists it.
func TestServePortRunningListsATeamItCannotBuild(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	f.herdr(t, foreignHerdr)
	port := servePort{env: serveEnv(t)}

	live, err := port.Running(web.LayerLocal)
	if err != nil {
		t.Fatalf("Running: %v", err)
	}
	if len(live.Teams) != 1 {
		t.Fatalf("herdr reports %d teams, want the one other team", len(live.Teams))
	}
	other := live.Teams[0]
	if other.Team != "other" || other.Definition != "other" || other.Known {
		t.Errorf("the running team is %+v, want other, unknown to this layer", other)
	}
	if len(other.Rows) != 0 {
		t.Errorf("a team this layer cannot build carries %d rows, want none", len(other.Rows))
	}
	if other.TabID != "t9" {
		t.Errorf("the running team sits in tab %q, want t9", other.TabID)
	}
}

// TestServePortRunningWithoutHerdr: a herdr that cannot be read is one reason
// the screen shows, and the team list is marked as before: none running.
func TestServePortRunningWithoutHerdr(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	port := servePort{env: serveEnv(t)}

	if _, err := port.Running(web.LayerLocal); err == nil {
		t.Error("Running with no herdr returned no error")
	}
	rows := port.Teams(web.LayerLocal)
	if len(rows) != 1 {
		t.Fatalf("the team list has %d rows, want the one definition", len(rows))
	}
	if rows[0].Running {
		t.Error("a herdr that cannot be read marked a definition as running")
	}
}

// TestServePortTeamsMarksTheRunningDefinition: the team list says which
// definitions herdr reports a running team for, matched through the definition
// the tab carries.
func TestServePortTeamsMarksTheRunningDefinition(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeDemoTeam(t, f, "demo")
	writeDemoTeam(t, f, "idle")
	f.herdr(t, teamviewHerdr)
	port := servePort{env: serveEnv(t)}

	rows := port.Teams(web.LayerLocal)
	if len(rows) != 2 {
		t.Fatalf("the team list has %d rows, want two definitions", len(rows))
	}
	for _, row := range rows {
		want := row.Name == "demo"
		if row.Running != want {
			t.Errorf("%s is marked running=%v, want %v", row.Name, row.Running, want)
		}
	}
}

// TestServePortFocusMovesHerdr: the one write these screens hold asks herdr to
// move to a pane, and a request with no pane is refused with the line the screen
// shows instead of calling herdr.
func TestServePortFocusMovesHerdr(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	log := filepath.Join(t.TempDir(), "focus.log")
	t.Setenv("HERGY_TEST_FOCUS_LOG", log)
	f.herdr(t, teamviewHerdr)
	port := servePort{env: serveEnv(t)}

	if err := port.Focus("p2"); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake herdr recorded nothing: %v", err)
	}
	if string(got) != "p2\n" {
		t.Errorf("focus asked herdr for %q, want the pane p2", got)
	}

	err = port.Focus("")
	if err == nil {
		t.Fatal("focus with no pane returned no error")
	}
	if !strings.Contains(err.Error(), i18n.T("error.focus_no_pane")) {
		t.Errorf("focus with no pane said %q, want %q", err, i18n.T("error.focus_no_pane"))
	}
}
