package check

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

func TestRawProseReachesPromptAndRosterUsesTitle(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> human\n")
	f.roles()
	raw := "# 実装担当\r\n\r\n  lead → unknown\t%{role_doc}  \r\n"
	f.write(f.leadRolePath(), raw)
	agree := "\n\t合意 %{roster}  \n"
	f.write(f.agreementPath(), agree)
	def, err := f.cat.Definition("demo", source.LayerLocal)
	if err != nil {
		t.Fatal(err)
	}
	run := prompt.Run{Team: fixtureNaming("demo"), Reader: f.cat.Reader(), Sources: f.cat, Lang: "ja"}
	built, err := prompt.Member(def, &def.Graph.Members[0], run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.Text, raw) || !strings.Contains(built.Text, agree) {
		t.Fatal("prompt rewrote prose")
	}
	if !strings.Contains(built.Text, "lead: 実装担当（lead）— demo-lead（あなた）") {
		t.Fatalf("roster: %s", built.Text)
	}
	f.write(f.leadRolePath(), strings.Repeat("あ", 1201))
	if _, err := prompt.Member(def, &def.Graph.Members[0], run); err != nil {
		t.Fatalf("budget warning stopped prompt: %v", err)
	}
}
