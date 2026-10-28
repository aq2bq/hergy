package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/deliver"
)

// fakeApply is an activity reading that also knows the drift answer, so the
// boundary can be tested without a herdr.
type fakeApply struct {
	fakeActivity
	answer ApplyList
	sent   []string
}

func (f *fakeApply) Apply() ApplyList    { return f.answer }
func (f *fakeApply) ApplyCount() int     { return f.answer.Count }
func (f *fakeApply) InvalidateDelivery() {}
func (f *fakeApply) ApplyMembers(members []string) (HQLaunch, error) {
	f.sent = append([]string(nil), members...)
	return HQLaunch{Action: ActionApply, Phase: "starting", Steps: []HQLaunchStep{}, Members: []string{"lead"}}, nil
}

// TestApplyAPIAnswersTheCountAndTheDrift: the band reads the count alone, and
// the sheet reads each member with the way hergy puts it right.
func TestApplyAPIAnswersTheCountAndTheDrift(t *testing.T) {
	f := newFixture(t)
	reading := &fakeApply{answer: ApplyList{
		Count: 2,
		Teams: []ApplyTeam{{
			Team: "demo", Definition: "demo", Project: "/p/demo", ProjectName: "demo",
			Notes: []string{},
			Members: []ApplyMember{
				{
					Name: "demo-lead", Short: "lead", Role: "lead", State: "idle",
					How: ApplyResend, CanApply: true,
					Changes: []deliver.Change{{Kind: "rule", Document: "demo/agreement.yaml", Rule: "TEAM-1", Text: "TEAM-1 を直した"}},
				},
				{
					Name: "demo-builder", Short: "builder", Role: "builder", State: "absent",
					How: ApplyStart, CanApply: true, Changes: []deliver.Change{},
				},
			},
		}},
		Herdr: ActivityNote{OK: true},
	}}
	server, err := New(f.catalog(), reading)
	if err != nil {
		t.Fatal(err)
	}
	resp, body := f.api(t, server, http.MethodGet, "/api/apply/count", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET apply count answers %d: %s", resp.StatusCode, body)
	}
	if body != `{"count":2}` {
		t.Errorf("count = %s, want {\"count\":2}", body)
	}
	resp, body = f.api(t, server, http.MethodGet, "/api/apply", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET apply answers %d: %s", resp.StatusCode, body)
	}
	for _, want := range []string{`"how":"resend"`, `"how":"start"`, `"TEAM-1"`, `"demo-builder"`, `"can_apply":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("apply = %s, want %s", body, want)
		}
	}
}
