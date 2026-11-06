package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// TestPROFILES_DangerLossNamesWhatDeleteTakes: the words of the confirmation and
// the entry Delete removes come from the same target (ac-ddb6).
func TestPROFILES_DangerLossNamesWhatDeleteTakes(t *testing.T) {
	f, stop := fixtureProject(t)
	defer stop()
	server := f.server(t)
	lines, refusal := server.dangerLossProfile(p2Project, "sonnet-fast")
	if refusal != nil {
		t.Fatalf("the loss: %+v", refusal)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "sonnet-fast") || !strings.Contains(joined, "ファイルは残ります。") {
		t.Errorf("the loss does not name the entry and the file: %v", lines)
	}
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	text := readFile(t, path)
	resp, body := f.api(t, server, http.MethodDelete, "/api/profile/sonnet-fast?place=project&hash="+source.Hash(text), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE answers %d: %s", resp.StatusCode, body)
	}
	after := readFile(t, path)
	if strings.Contains(after, "sonnet-fast") {
		t.Errorf("the entry the loss named is still there:\n%s", after)
	}
	if _, err := profile.ReadAll(after, path); err != nil {
		_ = err
	}
	_ = fmt.Sprint()
}
