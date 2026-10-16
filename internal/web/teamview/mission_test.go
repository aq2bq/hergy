package teamview

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/web"
)

// missionPort is the demo team's definition with the given mission written on
// it (empty for none).
func missionPort(mission string) *fakePort {
	def := demoDefinition()
	def.Mission = mission
	return &fakePort{
		teams: map[string][]web.Team{web.LayerLocal: {{Name: "demo"}}},
		roots: map[string]string{web.LayerLocal: ".hergy"},
		defs:  map[string]Definition{"demo": def},
	}
}

// TestDefinitionScreenShowsTheMissionUnderTheName: the definition screen carries
// what the team is for (r-341e) as the line under the definition's name, the
// first line of the screen; a definition that writes none says so there, in the
// same place, so the line is never simply absent.
func TestDefinitionScreenShowsTheMissionUnderTheName(t *testing.T) {
	base := start(t, missionPort("hergy をよくする"))
	code, body := get(t, base, "/team?layer=local")
	if code != 200 {
		t.Fatalf("GET /team = %d, want 200", code)
	}
	want := `<h1 class="mono">demo</h1>` + "\n  " + `<p class="mission">hergy をよくする</p>`
	if !strings.Contains(body, want) {
		t.Errorf("the definition screen does not carry the mission under the name:\n%s", body)
	}
	mustNotContain(t, "the definition screen", body, shown("web.mission.empty"))

	// No mission: the same line says so rather than standing empty.
	base = start(t, missionPort(""))
	code, body = get(t, base, "/team?layer=local")
	if code != 200 {
		t.Fatalf("GET /team = %d, want 200", code)
	}
	mustContain(t, "the definition screen", body,
		`<p class="mission">`+shown("web.mission.empty")+`</p>`)
}

// TestDefinitionScreenKeepsTheMissionOutOfABrokenFile: a definition that cannot
// be read has no mission to show, so the screen says why the file is broken and
// no line about a mission: it cannot know whether one is written.
func TestDefinitionScreenKeepsTheMissionOutOfABrokenFile(t *testing.T) {
	def := demoDefinition()
	def.Broken = "version must be 1"
	def.Docs = nil
	def.Members = nil
	port := missionPort("")
	port.defs["demo"] = def
	base := start(t, port)

	code, body := get(t, base, "/team?layer=local")
	if code != 200 {
		t.Fatalf("GET /team = %d, want 200", code)
	}
	mustContain(t, "the broken definition screen", body, "version must be 1")
	mustNotContain(t, "the broken definition screen", body, shown("web.mission.empty"))
}
