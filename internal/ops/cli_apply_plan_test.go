package ops

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing" // TestApplyDryShowsATeamsOwnNote: a worker's changed profile is the team's note,
	// not a member's change (d-36ef), and the command line shows it where the sheet
	// does: in the plan and in the plan's JSON (r-7aad).
	// TestApplyDryShowsATeamsNoteWhileItsMemberIsLeft: a member that is working is
	// left, and the team's own note stands beside it: the sheet shows it there too
	// (r-7aad).
)

func TestApplyDryShowsATeamsOwnNote(t *testing.T) {
	f := japaneseDemo(t)
	note := changedWorkerProfile(t, f)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`, `{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	out := mustRun(t, "apply", "--dry")
	if !strings.Contains(out, note) {
		t.Errorf("apply --dry does not show the team's note %q:\n%s", note, out)
	}
	body := mustRun(t, "apply", "--dry", "--json")
	var list wireApplyList
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("apply --dry --json: %v\n%s", err, body)
	}
	found := false
	for _, team := range list.Teams {
		for _, got := range team.Notes {
			if got == note {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("apply --dry --json notes = %+v, want %q", list.Teams, note)
	}
}

func TestApplyDryShowsATeamsNoteWhileItsMemberIsLeft(t *testing.T) {
	f := japaneseDemo(t)
	note := changedWorkerProfile(t, f)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`, `{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	out, stderr, code := run("apply", "--dry")
	if code != 1 {
		t.Fatalf("apply --dry: exit %d, want 1\n%s", code, stderr)
	}
	if !strings.Contains(out, note) {
		t.Errorf("apply --dry does not show the team's note %q:\n%s", note, out)
	}
}

// TestApplyDryNamesAnAddedRuleThatHasNoIDYet: a rule a person has just written
// has no id until a write gives it one, and the plan is read before that write.
// The plan names the rule by its own text, so a person reads which rule was
// added (r-7aad).
func TestApplyDryNamesAnAddedRuleThatHasNoIDYet(t *testing.T) {
	f := japaneseDemo(t)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
	const added = "依頼を渡す前に、読むファイルを名指す。"
	writeFile(t, rolePath, readFile(t, rolePath)+"      - text: "+added+"\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	out := mustRun(t, "apply", "--dry")
	if !strings.Contains(out, "lead.md") {
		t.Errorf("apply --dry does not name the rule it would add (%q):\n%s", added, out)
	}
	if strings.Contains(out, "規則  を足した") {
		t.Errorf("apply --dry names the added rule with an empty name:\n%s", out)
	}
}

// TestApplyDrySaysTheWayAsAPlan: --dry is a plan, so each member's way is the
// words the sheet's mark uses (d-3d30, d-7b45), not the past tense of something
// that has not happened (r-7aad).
func TestApplyDrySaysTheWayAsAPlan(t *testing.T) {
	t.Run("resend", func(t *testing.T) {
		f := japaneseDemo(t)
		rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
		role := readFile(t, rolePath)
		writeFile(t, rolePath, strings.Replace(role,
			"受け手が会話を読まずに着手できる依頼を渡す。",
			"受け手が会話を読まずに着手できる依頼を、読むファイルの名前とともに渡す。", 1))
		f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
			`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

		out := mustRun(t, "apply", "--dry")
		if !strings.Contains(out, "送り直し（会話は残る）") {
			t.Errorf("apply --dry does not say the way as a plan:\n%s", out)
		}
		if strings.Contains(out, "送り直しました") {
			t.Errorf("apply --dry says the way in the past tense:\n%s", out)
		}
	})
	t.Run("restart", func(t *testing.T) {
		f := japaneseDemo(t)
		writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
			"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, opus]\n")
		f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
			`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

		out := mustRun(t, "apply", "--dry")
		if !strings.Contains(out, "起動し直し（会話が消える）") {
			t.Errorf("apply --dry does not say the way as a plan:\n%s", out)
		}
		if strings.Contains(out, "起動し直しました") {
			t.Errorf("apply --dry says the way in the past tense:\n%s", out)
		}
	})
}
