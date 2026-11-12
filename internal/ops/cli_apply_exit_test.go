package ops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n" // TestApplyDryAndApplyExitOneWhileAMemberIsLeft covers both a plan that can
	// apply nobody and one that can still apply a peer, including JSON (r-980a).
)

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
