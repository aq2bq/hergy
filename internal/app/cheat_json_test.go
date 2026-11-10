package app

import (
	"bytes"
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/launcher"
)

// TestCheatDryJSONKeys compares the cheat's lists with actual command output,
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
				sameJSONKeys(t, plan, append(slices.Clone(always), optional...))
				if hasMode {
					// The shared writer omits turns when there is no definition.
					// This also checks the cheat did not call turns an always key.
					var out bytes.Buffer
					if err := printPlanJSON(&out, mode, &resolved{}, nil, 30000, launcher.NewPanes{}); err != nil {
						t.Fatal(err)
					}
					var withoutTurns map[string]any
					if err := json.Unmarshal(out.Bytes(), &withoutTurns); err != nil {
						t.Fatal(err)
					}
					sameJSONKeys(t, withoutTurns, always)
				}
			})
			t.Run("members", func(t *testing.T) {
				checkCheatJSONMembers(t, doc, command, plan)
			})
		})
	}
}

// TestCheatLaunchMemberOptionalKeys checks both ends of ToPlan's conditional
// fields. CLI plans always carry GY_ACTOR, even with no profile env (r-1c42).
func TestCheatLaunchMemberOptionalKeys(t *testing.T) {
	newFixture(t)
	doc := mustRun(t, "cheat", "--ai")
	always, optional := cheatJSONKeys(t, doc, "up/refresh/reprompt/hire members[]")
	member := definition.Start{Env: map[string]string{"GY_ACTOR": "demo-lead"}}
	sameJSONKeys(t, member.ToPlan(), always)
	member.Profile = "demo-profile"
	member.Turns = []string{"human"}
	member.Workers = []definition.WorkerNode{{From: "lead", Purpose: "inspect", Profile: "demo-profile"}}
	member.HasRole = true
	sameJSONKeys(t, member.ToPlan(), append(slices.Clone(always), optional...))
}

func dryJSONForCheat(t *testing.T, command string) map[string]any {
	t.Helper()
	f := applyDemo(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	writeFile(t, path, readFile(t, path)+"workers:\n  - from: lead\n    purpose: inspect\n    profile: claude-default\n")
	recordAgain(t)
	f.herdr(t, liveHerdr)
	args := map[string][]string{
		"up":       {"up", "demo"},
		"refresh":  {"refresh", "lead", "--team", "demo"},
		"reprompt": {"reprompt", "--all", "--team", "demo"},
		"hire":     {"hire", "scout", "--purpose", "inspect", "--task", "read", "--team", "demo"},
		"apply":    {"apply", "--team", "demo"},
		"down":     {"down", "demo"},
	}[command]
	wantCode := 0
	if command == "apply" {
		for _, member := range []string{"lead", "builder"} {
			role := filepath.Join(f.proj, ".hergy", "roles", member+".md")
			writeFile(t, role, readFile(t, role)+"\nName the files to read.\n")
		}
		applyLeftHerdr(t, f)
		wantCode = 1
	}
	args = append(args, "--dry", "--json")
	stdout, stderr, code := run(args...)
	if code != wantCode {
		t.Fatalf("%v: exit %d, want %d\n%s\n%s", args, code, wantCode, stdout, stderr)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, stdout)
	}
	return plan
}

func checkCheatJSONMembers(t *testing.T, doc, command string, plan map[string]any) {
	t.Helper()
	if command == "down" {
		if !strings.Contains(doc, "\n    down members[]: herdr name strings\n") {
			t.Fatal("cheat --ai does not describe down members as herdr name strings")
		}
		members, ok := plan["members"].([]any)
		if !ok || len(members) == 0 {
			t.Fatalf("down members = %#v, want a nonempty list", plan["members"])
		}
		for _, member := range members {
			if _, ok := member.(string); !ok {
				t.Errorf("down member = %#v, want a herdr name string", member)
			}
		}
		return
	}
	label := "up/refresh/reprompt/hire members[]"
	members := plan["members"]
	if command == "apply" {
		label = "apply teams[].members[]"
		teams := jsonObjectList(t, plan["teams"])
		members = teams[0]["members"]
	}
	always, optional := cheatJSONKeys(t, doc, label)
	applyStates := map[bool]bool{}
	for _, member := range jsonObjectList(t, members) {
		if command == "apply" {
			canApply, ok := member["can_apply"].(bool)
			if !ok {
				t.Fatalf("apply can_apply = %#v, want a boolean", member["can_apply"])
			}
			applyStates[canApply] = true
		}
		want := slices.Clone(always)
		for _, key := range optional {
			_, present := member[key]
			if command == "apply" {
				present = member["can_apply"] == false
			}
			if present {
				want = append(want, key)
			}
		}
		sameJSONKeys(t, member, want)
	}
	if command == "apply" && (!applyStates[true] || !applyStates[false]) {
		t.Fatal("apply fixture must exercise members with and without why_not")
	}
}

// The cheat uses one explicit row per shape, with always and optional lists.
// Read those lists instead of keeping another copy of the keys in the tests.
func cheatJSONKeys(t *testing.T, doc, label string) ([]string, []string) {
	t.Helper()
	prefix := "    " + label + ": always: "
	for line := range strings.SplitSeq(doc, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		required, conditional, hasConditional := strings.Cut(strings.TrimPrefix(line, prefix), "; optional: ")
		var optional []string
		if hasConditional {
			optional = strings.Split(conditional, ", ")
		}
		return strings.Split(required, ", "), optional
	}
	t.Fatalf("cheat --ai has no JSON key description for %q", label)
	return nil, nil
}

func sameJSONKeys(t *testing.T, object map[string]any, want []string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(object))
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("JSON keys = %v, cheat --ai says %v", got, want)
	}
}

func jsonObjectList(t *testing.T, raw any) []map[string]any {
	t.Helper()
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("JSON list = %#v, want a nonempty list", raw)
	}
	objects := make([]map[string]any, len(items))
	for i, item := range items {
		objects[i], ok = item.(map[string]any)
		if !ok {
			t.Fatalf("JSON member = %#v, want an object", item)
		}
	}
	return objects
}
