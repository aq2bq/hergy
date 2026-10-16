package web

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestTheTeamListRowsCarryTheMission: every row says what the team is for
// (r-341e): the mission team.yaml writes, or, when it writes none, the note that
// none is written. A row whose definition cannot be read carries no mission
// line: the file was not read, so the row shows why in its place.
func TestTheTeamListRowsCarryTheMission(t *testing.T) {
	port := fakePort{teams: map[string][]Team{
		LayerLocal: {
			{Name: "demo", Members: 1, Rules: 2, Mission: "hergy をよくする"},
			{Name: "quiet", Members: 1, Rules: 2},
			{Name: "broken", Broken: ".hergy/teams/broken/team.yaml: version must be 1"},
		},
	}}
	s := newTestServer(t, port)
	body := get(t, s, "/teams").Body.String()

	mustContain(t, "the team list", body, `<span class="ts">hergy をよくする</span>`)
	if n := strings.Count(body, i18n.T("web.mission.empty")); n != 1 {
		t.Errorf("the team list says no mission is written %d times, want once, on the one row without one:\n%s", n, body)
	}
	// The broken row keeps the reason for its file where the mission would be.
	if broken := row(body, "broken"); !strings.Contains(broken, "version must be 1") ||
		strings.Contains(broken, i18n.T("web.mission.empty")) {
		t.Errorf("the broken row carries a mission line rather than the reason:\n%s", broken)
	}
}

// row is the markup of one team list row: the row whose name cell carries the
// name, up to the row's own closing tag.
func row(body, name string) string {
	for _, part := range strings.Split(body, `<a class="tline"`)[1:] {
		line, _, _ := strings.Cut(part, "</a>")
		if strings.Contains(line, `class="tn">`+name+`<`) {
			return line
		}
	}
	return ""
}
