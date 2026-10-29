package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// TestAWriteGivesRuleIDsAndSaysSo: a command that writes hands every rule hergy
// reads an id, writes it into the file, and says how many it gave; the next
// write finds nothing to give and leaves the file alone (d-4aab).
func TestAWriteGivesRuleIDsAndSaysSo(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	writeFile(t, rolePath, readFile(t, rolePath)+"      - text: a rule with no id yet\n")
	logPath := filepath.Join(f.cfg, "herdr.log")
	f.herdr(t, `#!/bin/sh
log='`+logPath+`'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"demo"}]}}' ;;
  "agent list") echo '{"result":{"agents":[
    {"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}
  ]}}' ;;
  "agent get") echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}}}' ;;
  "agent prompt") echo '{"result":{}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	out := mustRun(t, "reprompt", "lead", "--team", "demo")
	if !strings.Contains(out, i18n.Tf("rules.ids_given", 1)) {
		t.Fatalf("reprompt did not say it gave an id:\n%s", out)
	}
	written := readFile(t, rolePath)
	doc, problems := rules.ReadAll(written, rolePath)
	if len(problems) != 0 {
		t.Fatalf("the written role does not read: %+v", problems)
	}
	last := doc.Rules()[len(doc.Rules())-1]
	if !rules.RuleIDPattern.MatchString(last.ID) {
		t.Errorf("the new rule's id = %q, want hergy's shape", last.ID)
	}
	again := mustRun(t, "reprompt", "lead", "--team", "demo")
	if strings.Contains(again, "ID を付けました") {
		t.Errorf("the second write gave an id again:\n%s", again)
	}
	if readFile(t, rolePath) != written {
		t.Errorf("the second write changed the file")
	}
}

// TestADryRunWritesNoID: --dry is read-only, so a rule without an id stays
// without one and the command does not say it gave any.
func TestADryRunWritesNoID(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	before := readFile(t, rolePath) + "      - text: a rule with no id yet\n"
	writeFile(t, rolePath, before)

	out := mustRun(t, "reprompt", "lead", "--team", "demo", "--dry")
	if strings.Contains(out, i18n.T("rules.ids_given")) {
		t.Errorf("reprompt --dry says it gave an id:\n%s", out)
	}
	if readFile(t, rolePath) != before {
		t.Errorf("reprompt --dry wrote the file")
	}
}

// TestValidateCountsTheRulesWithoutAnIDAndWritesNothing: validate says how many
// rules the next write will give an id, and is not an error (d-4aab).
func TestValidateCountsTheRulesWithoutAnIDAndWritesNothing(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	before := readFile(t, rolePath) + "      - text: a rule with no id yet\n"
	writeFile(t, rolePath, before)

	out, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate answered %d for a rule without an id:\n%s", code, out)
	}
	if !strings.Contains(out, i18n.Tf("rules.ids_missing", 1)) {
		t.Errorf("validate does not count the rule without an id:\n%s", out)
	}
	if readFile(t, rolePath) != before {
		t.Errorf("validate wrote the file")
	}
}
