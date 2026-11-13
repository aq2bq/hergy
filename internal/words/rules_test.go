package words

import (
	"fmt"
	"strings"
	"testing"
)

func TestRuleCatalogIsCompleteAndRenderable(t *testing.T) {
	ids, keys := map[RuleID]bool{}, map[string]bool{}
	for _, rule := range Rules() {
		if rule.ID == "" || ids[rule.ID] {
			t.Fatalf("duplicate or empty ID: %s", rule.ID)
		}
		ids[rule.ID] = true
		if rule.ErrorKey != "" {
			if keys[rule.ErrorKey] {
				t.Fatalf("two owners of %s", rule.ErrorKey)
			}
			keys[rule.ErrorKey] = true
		}
		for _, lang := range []string{"en", "ja"} {
			text := rule.Explain(lang)
			if text == "" || strings.Contains(text, "{{") || strings.Contains(text, "[[") || strings.Contains(text, "%!") {
				t.Errorf("%s/%s: incomplete projection: %s", rule.ID, lang, text)
			}
		}
	}
}

func TestRefusalAndExplanationUseSameCauseAndRecovery(t *testing.T) {
	for _, id := range []RuleID{"refresh.no-pane", "refresh.own-pane", "apply.env-changed", "up.name-taken"} {
		rule := Find(id)
		for _, lang := range []string{"en", "ja"} {
			err := Errorf(id, lang, "demo-builder")
			got := err.Error()
			want := fmt.Sprintf(rule.Pattern(lang), "demo-builder")
			if got != want {
				t.Errorf("%s/%s: %q != %q", id, lang, got, want)
			}
			if !strings.Contains(got, "demo-builder") {
				t.Errorf("%s: lost subject", id)
			}
			if recovery := rule.Recovery.In(lang); recovery != "" && !strings.Contains(rule.Explain(lang), recovery) {
				t.Errorf("%s: explanation lost recovery", id)
			}
		}
	}
}
