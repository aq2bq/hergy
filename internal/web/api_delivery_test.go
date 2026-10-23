package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/deliver"
)

// fakeDelivery is an activity reading that also knows the changed-definition
// answer, so the boundary can be tested without a herdr.
type fakeDelivery struct {
	fakeActivity
	answer deliver.Answer
	sent   []string
}

func (f *fakeDelivery) Delivery() deliver.Answer { return f.answer }
func (f *fakeDelivery) DeliveryCount() int       { return f.answer.Count }
func (f *fakeDelivery) Deliver(names []string) (deliver.Answer, error) {
	f.sent = append([]string(nil), names...)
	answer := f.answer
	answer.Sent = f.sent
	return answer, nil
}

func TestDeliveryAPIAnswersTheCountTheSheetAndOneSend(t *testing.T) {
	f := newFixture(t)
	reading := &fakeDelivery{answer: deliver.Answer{
		Count: 1,
		Teams: []deliver.Team{{
			Name: "demo", Definition: "demo",
			Members: []deliver.Member{{
				Name: "demo-lead", Short: "lead", Role: "lead", State: "idle", CanSend: true,
				Changes: []deliver.Change{{Kind: "rule", Document: "demo/agreement.yaml", Rule: "TEAM-1", Text: "TEAM-1 を直した"}},
			}},
		}},
		Herdr: deliver.Note{OK: true},
	}}
	server, err := New(f.catalog(), reading)
	if err != nil {
		t.Fatal(err)
	}
	resp, body := f.api(t, server, http.MethodGet, "/api/delivery/count", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET count answers %d: %s", resp.StatusCode, body)
	}
	if body != `{"count":1}` {
		t.Errorf("count = %s, want {\"count\":1}", body)
	}
	resp, body = f.api(t, server, http.MethodGet, "/api/delivery", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET delivery answers %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"demo-lead"`) || !strings.Contains(body, `"TEAM-1"`) {
		t.Errorf("delivery = %s, want the stale member and its rule", body)
	}
	resp, body = f.api(t, server, http.MethodPost, "/api/delivery", map[string]any{"members": []string{"demo-lead"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST delivery answers %d: %s", resp.StatusCode, body)
	}
	if len(reading.sent) != 1 || reading.sent[0] != "demo-lead" {
		t.Errorf("sent = %+v, want demo-lead", reading.sent)
	}
}
