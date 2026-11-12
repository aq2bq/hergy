package ops

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
	"go.uber.org/goleak"
)

var fixtureCLI string

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "hergy-ops-test-")
	if err != nil {
		panic(err)
	}
	fixtureCLI = filepath.Join(dir, "hergy")
	cmd := exec.Command("go", "build", "-o", fixtureCLI, "../..")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out))
	}
	goleak.VerifyTestMain(m, goleak.Cleanup(func(int) { _ = os.RemoveAll(dir) }))
}
func Run(args []string, stdout, stderr io.Writer) int {
	configured, _ := team.Lang()
	lang, _ := i18n.Resolve(options.ScanLang(args), configured)
	_ = i18n.Use(lang)
	cmd := exec.Command(fixtureCLI, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if e, ok := errors.AsType[*exec.ExitError](err); ok {
			return e.ExitCode()
		}
		return 1
	}
	return 0
}
func run(argv ...string) (string, string, int) {
	var out, err bytes.Buffer
	code := Run(argv, &out, &err)
	return out.String(), err.String(), code
}

type activityRead = Reader

type statusJSONList struct {
	Teams []struct {
		Error      string                          `json:"error"`
		Team       string                          `json:"team"`
		Definition string                          `json:"definition"`
		Members    []struct{ Name, Status string } `json:"members"`
	} `json:"teams"`
}
