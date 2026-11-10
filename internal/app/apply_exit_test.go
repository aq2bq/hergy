package app

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// TestApplyDryAndApplyExitOneWhileAMemberIsLeft covers both a plan that can
// apply nobody and one that can still apply a peer, including JSON (r-980a).
func TestApplyDryAndApplyExitOneWhileAMemberIsLeft(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		dryError := map[string]string{"en": "1 member(s) cannot be applied", "ja": "反映できずに残るメンバーが 1 人います"}[lang]
		for _, scenario := range []string{"left-only", "mixed"} {
			t.Run(lang+"/"+scenario, func(t *testing.T) {
				f := applyDemo(t)
				if lang == "ja" {
					writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
					mustRun(t, "version")
					recordAgain(t)
				}
				members := []string{"lead"}
				if scenario == "mixed" {
					members = append(members, "builder")
				}
				for _, member := range members {
					path := filepath.Join(f.proj, ".hergy", "roles", member+".md")
					writeFile(t, path, readFile(t, path)+"\nName the files to read.\n")
				}
				applyLeftHerdr(t, f)
				t.Setenv("HERDR_ENV", "1")
				t.Setenv("HERDR_PANE_ID", "w0:p1")
				for _, flags := range [][]string{{"--dry"}, {"--dry", "--json"}, {}} {
					args := append([]string{"apply"}, flags...)
					stdout, stderr, code := run(args...)
					if code != 1 {
						t.Errorf("%v: exit %d, want 1\n%s\n%s", args, code, stdout, stderr)
					}
					wantError := i18n.Tf("apply.left_count", 1)
					if len(flags) > 0 {
						wantError = dryError
					}
					if !strings.Contains(stderr, wantError) {
						t.Errorf("%v stderr = %q, want %q", args, stderr, wantError)
					}
					why := i18n.Tf("apply.refused_state", "lead")
					if len(flags) == 2 {
						checkApplyLeftJSON(t, stdout, why)
					} else if want := i18n.Tf("apply.left", "lead", why); !strings.Contains(stdout, want) {
						t.Errorf("%v stdout = %q, want %q", args, stdout, want)
					}
					if len(flags) == 0 && scenario == "mixed" && !strings.Contains(stdout, i18n.Tf("apply.applied", "builder", i18n.T("apply.how_resend"))) {
						t.Errorf("apply did not apply the idle peer:\n%s", stdout)
					}
				}
			})
		}
	}
}

func applyLeftHerdr(t *testing.T, f *fixture) {
	t.Helper()
	const builder = `{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":1}`
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1},`+builder)
	path := filepath.Join(f.cfg, "herdr")
	script := strings.Replace(readFile(t, path), `case "$g $s" in`,
		`case "$g $s" in
  "agent get") echo '{"result":{"agent":`+builder+`}}' ;;`, 1)
	f.herdr(t, script)
}

func checkApplyLeftJSON(t *testing.T, stdout, why string) {
	t.Helper()
	var list web.ApplyList
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("apply --dry --json: %v\n%s", err, stdout)
	}
	for _, team := range list.Teams {
		for _, member := range team.Members {
			if member.Name == "demo-lead" && member.Short == "lead" && !member.CanApply && member.WhyNot == why {
				return
			}
		}
	}
	t.Errorf("JSON does not name the member left with reason %q:\n%s", why, stdout)
}
