package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"os"
)

// TestStatusJSONCarriesStateAndUnchangedMinutes: status --json prints the rows
// as an object, with the unchanged count as a number after a second look.
func TestStatusJSONCarriesStateAndUnchangedMinutes(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead working p1 screen-lead\ndemo-builder idle p2 screen-builder\n")

	setClock(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	mustRun(t, "status", "demo")
	setClock(t, time.Date(2026, 1, 1, 0, 12, 0, 0, time.UTC))

	raw := mustRun(t, "status", "demo", "--json")
	var got struct {
		Team    string `json:"team"`
		Members []struct {
			Name             string `json:"name"`
			State            string `json:"state"`
			UnchangedMinutes *int   `json:"unchanged_minutes"`
			Live             bool   `json:"live"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("status --json is not JSON: %v\n%s", err, raw)
	}
	if got.Team != "demo" {
		t.Fatalf("status --json team = %q, want demo", got.Team)
	}
	byName := map[string]int{}
	for _, m := range got.Members {
		if m.UnchangedMinutes != nil {
			byName[m.Name] = *m.UnchangedMinutes
		}
	}
	if byName["demo-lead"] != 12 {
		t.Fatalf("unchanged_minutes for demo-lead = %d, want 12 (%s)", byName["demo-lead"], raw)
	}
	if _, ok := byName["demo-builder"]; ok {
		t.Fatalf("an idle member carries unchanged_minutes: %s", raw)
	}
}

// TestStatusMarksAWorkingMemberWhoseScreenHasNotChanged: a second status, twelve
// minutes on with the same screen, says so; a screen that changed starts over.
func TestStatusMarksAWorkingMemberWhoseScreenHasNotChanged(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.controlHerdr(t, "demo-lead working p1 screen-lead\n")

	setClock(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	first := mustRun(t, "status", "demo")
	if !strings.Contains(first, "demo-lead: working\n") {
		t.Fatalf("the first status already marks the member:\n%s", first)
	}

	setClock(t, time.Date(2026, 1, 1, 0, 12, 0, 0, time.UTC))
	second := mustRun(t, "status", "demo")
	if !strings.Contains(second, "demo-lead: working (unchanged for 12 min)\n") {
		t.Fatalf("the second status does not mark the unchanged screen:\n%s", second)
	}

	f.controlHerdr(t, "demo-lead working p1 a different screen\n")
	setClock(t, time.Date(2026, 1, 1, 0, 20, 0, 0, time.UTC))
	third := mustRun(t, "status", "demo")
	if !strings.Contains(third, "demo-lead: working\n") {
		t.Fatalf("a changed screen is still marked:\n%s", third)
	}
}

// setClock fixes the clock status stamps a screen with, for the length of one
// test.
func setClock(t *testing.T, at time.Time) {
	t.Helper()
	previous := screenClock
	screenClock = func() time.Time { return at }
	t.Cleanup(func() { screenClock = previous })
}

// controlHerdr writes a fake herdr for the member commands (focus, nudge, stop,
// tell) over one running team, demo. <cfg>/status.txt holds one live agent per
// line, "<name> <status> <pane> <screen>": agent list, agent get and agent read
// answer from it. Every call is recorded in <cfg>/herdr.log, and every prompt in
// <cfg>/prompts.txt as "<target>\t<text>" with newlines turned to spaces, so a
// test can read the body a member was handed.
//
// agent send-keys counts the escs sent to each member in <cfg>/esc.txt and, when
// a test wrote a stop plan, moves the member to the state that plan gives after
// that count (<cfg>/stop-plan.txt, one "<herdr name> <esc count> <status>" line
// per step). That is how the fake herdr stands for a harness that stops on the
// first esc, one that stops on the second, and one that never stops.
func (f *fixture) controlHerdr(t *testing.T, agents string) {
	seedRunningTeam(t, f.proj, "demo", "demo")
	t.Helper()
	status := filepath.Join(f.cfg, "status.txt")
	logPath := filepath.Join(f.cfg, "herdr.log")
	promptsPath := filepath.Join(f.cfg, "prompts.txt")
	esc := filepath.Join(f.cfg, "esc.txt")
	plan := filepath.Join(f.cfg, "stop-plan.txt")
	if err := os.WriteFile(status, []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
status='` + status + `'
log='` + logPath + `'
prompts='` + promptsPath + `'
esc='` + esc + `'
plan='` + plan + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"demo"}]}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r name st pane screen; do
      [ -z "$name" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}' "$name" "$st" "$pane"
    done < "$status"
    printf ']}}'
    ;;
  "agent get")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$name" ] || [ "$pane" = "$3" ]; then
        printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}}}\n' "$n" "$st" "$pane"
        exit 0
      fi
    done < "$status"
    echo '{"error":{"code":"agent_not_found"}}'
    ;;
  "agent read")
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$3" ]; then printf '%s' "$screen"; exit 0; fi
    done < "$status"
    ;;
  "agent prompt")
    printf '%s\t' "$3" >> "$prompts"
    printf '%s' "$4" | tr '\n' ' ' >> "$prompts"
    printf '\n' >> "$prompts"
    echo '{"result":{}}'
    ;;
  "agent focus")     echo '{"result":{}}' ;;
  "agent send-keys")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    n=0
    if [ -f "$esc" ]; then n=$(cat "$esc"); fi
    n=$((n+1))
    printf '%s' "$n" > "$esc"
    if [ -f "$plan" ]; then
      new=$(awk -v n="$n" -v nm="$name" '$1==nm && $2==n {print $3}' "$plan")
      if [ -n "$new" ]; then
        awk -v nm="$name" -v st="$new" '{ if ($1==nm) $2=st; print }' "$status" > "$status.new"
        mv "$status.new" "$status"
      fi
    fi
    echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
}
