package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestApplyNamesAStartWithAnEmptyChangesList: a member hergy starts has no
// change of its own, and the promise says the member's changes are an array, so
// the answer carries [] and never null.
func TestApplyNamesAStartWithAnEmptyChangesList(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	got := newActivityRead(serveEnv(t)).Apply()
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, `"changes":null`) {
		t.Errorf("a member's changes are null: %s", text)
	}
	if !strings.Contains(text, `"changes":[]`) {
		t.Errorf("a start's changes are not an empty array: %s", text)
	}
}
