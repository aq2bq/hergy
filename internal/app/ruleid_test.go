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
	// One rule with no id at all, and one whose id is in an older shape: hergy
	// did not give either, and the next write gives both its own (d-4aab).
	before := readFile(t, rolePath) + "      - text: a rule with no id yet\n      - id: LEAD-1\n        text: a rule with an older id\n"
	writeFile(t, rolePath, before)

	out, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate answered %d for a rule without an id:\n%s", code, out)
	}
	if !strings.Contains(out, i18n.Tf("rules.ids_missing", 2)) {
		t.Errorf("validate does not count the rule without an id and the older id:\n%s", out)
	}
	if readFile(t, rolePath) != before {
		t.Errorf("validate wrote the file")
	}
}

// TestNewGivesEachTeamsRulesTheirOwnIDs: `hergy new` writes rule files, so it
// gives their rules ids; two teams made in one project do not share an id, and
// the template carries none to begin with (d-4aab).
func TestNewGivesEachTeamsRulesTheirOwnIDs(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	mustRun(t, "new", "demo2")
	seen := map[string]string{}
	for _, rel := range []string{
		filepath.Join("teams", "demo", "agreement.yaml"),
		filepath.Join("teams", "demo2", "agreement.yaml"),
		filepath.Join("roles", "lead.yaml"),
		filepath.Join("roles", "builder.yaml"),
	} {
		path := filepath.Join(f.proj, ".hergy", rel)
		doc, problems := rules.ReadAll(readFile(t, path), path)
		if len(problems) != 0 {
			t.Fatalf("%s does not read: %+v", rel, problems)
		}
		for _, rule := range doc.Rules() {
			if !rules.RuleIDPattern.MatchString(rule.ID) {
				t.Errorf("%s: rule %q has id %q, want hergy's shape", rel, rule.Text, rule.ID)
			}
			if prev, ok := seen[rule.ID]; ok {
				t.Errorf("%s and %s share the id %s", rel, prev, rule.ID)
			}
			seen[rule.ID] = rel
		}
	}
	for _, name := range []string{"demo", "demo2"} {
		path := filepath.Join(f.proj, ".hergy", "teams", name, "agreement.yaml")
		doc, _ := rules.ReadAll(readFile(t, path), path)
		for _, rule := range doc.Rules() {
			if rule.ID == "" {
				t.Errorf("%s still holds a rule with no id", name)
			}
		}
	}
}
