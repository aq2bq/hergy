package ops

import (
	"testing"
) // TestCheatDryJSONKeys compares the cheat's lists with actual command output,
// so changing either side without the other fails (r-fded, r-60ce).
func TestCheatDryJSONKeys(t *testing.T) {
	for _, command := range []string{"up", "refresh", "reprompt", "hire", "apply", "down"} {
		t.Run(command, func(t *testing.T) {
			plan := dryJSONForCheat(t, command)
			doc := mustRun(t, "cheat", "--ai")
			t.Run("top-level", func(t *testing.T) {
				label := command
				mode, hasMode := plan["mode"].(string)
				if hasMode {
					label += " (mode=" + mode + ")"
				}
				always, optional := cheatJSONKeys(t, doc, label)
				sameJSONKeys(t, plan, always)
				if len(optional) != 0 {
					t.Errorf("CLI top-level keys are always present, cheat --ai calls %v optional", optional)
				}
			})
			t.Run("members", func(t *testing.T) { checkCheatJSONMembers(t, doc, command, plan) })
		})
	}
} // TestCheatLaunchMemberOptionalKeys classifies fields by CLI output (d-e038):
// always keys appear in every member and worker; optional keys appear in some.
func TestCheatLaunchMemberOptionalKeys(t *testing.T) {
	counts := map[string]int{}
	total := 0
	doc := ""
	for _, command := range []string{"up", "refresh", "reprompt", "hire"} {
		t.Run(command, func(t *testing.T) {
			plan := dryJSONForCheat(t, command)
			doc = mustRun(t, "cheat", "--ai")
			for _, member := range jsonObjectList(t, plan["members"]) {
				total++
				for key := range member {
					counts[key]++
				}
			}
		})
	}
	always, optional := cheatJSONKeys(t, doc, "up/refresh/reprompt/hire members[]")
	observedAlways, observedOptional := map[string]any{}, map[string]any{}
	for key, count := range counts {
		if count == total {
			observedAlways[key] = true
		} else {
			observedOptional[key] = true
		}
	}
	sameJSONKeys(t, observedAlways, always)
	sameJSONKeys(t, observedOptional, optional)
}
