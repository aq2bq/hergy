package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing" // receiptHerdr augments the existing fake binary with ordered prompt answers.
	// Only the middle recipient fails; later recipients must still be contacted.
)

func receiptHerdr(t *testing.T, f *fixture, target, code string, retry bool) string {
	t.Helper()
	path := os.Getenv("HERGY_HERDR")
	script := readFile(t, path)
	count := filepath.Join(f.cfg, "receipt-count")
	_ = os.Remove(count)
	handler := `  "agent prompt")
    case "$3" in ` +
		target + `|p-` + target + `)
      n=0; [ -f '` +
		count + `' ] && n=$(cat '` + count + `')
      n=$((n+1)); printf '%s' "$n" > '` +
		count + `'
      if [ "$n" -le ` +
		map[bool]string{true: "1", false: "2"}[retry] + ` ] && [ -n '` + code + `' ]; then
        echo '{"error":{"code":"` +
		code + `","message":"receipt test"}}'; exit 1
      fi ;;
    esac
    echo '{"result":{}}' ;;
`
	script = strings.Replace(script, "case \"$g $s\" in\n", "case \"$g $s\" in\n"+handler, 1)
	script = strings.Replace(script, `  "pane process-info") echo`, `  "pane process-info")
    name="${4#p-}"; awk -v n="$name" '$2 != n' "$state" > "$state.tmp"; mv "$state.tmp" "$state"
    echo`, 1)
	f.herdr(t, script)
	log := filepath.Join(f.cfg, "herdr.log")
	writeFile(t, log, "")
	return log
}
func receiptTeam(t *testing.T, f *fixture) {
	t.Helper()
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
description: receipt test
members:
  - {name: a, role: lead, profile: claude-default}
  - {name: b, role: lead, profile: claude-default}
  - {name: c, role: lead, profile: claude-default}
workers:
  - {from: a, purpose: research, profile: claude-default}
turns:
  - human -> a
  - a -> b
  - b -> c
  - c -> human
`)
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
}
