package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
)

// promptPlan reads the --dry --json plan's members: the prompt and the pieces
// hergy built it from.
func promptPlan(t *testing.T, out string) []struct {
	Name     string                     `json:"name"`
	Prompt   string                     `json:"prompt"`
	Segments []definition.PromptSegment `json:"segments"`
} {
	t.Helper()
	var plan struct {
		Members []struct {
			Name     string                     `json:"name"`
			Prompt   string                     `json:"prompt"`
			Segments []definition.PromptSegment `json:"segments"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("the plan is not JSON: %v\n%s", err, out)
	}
	return plan.Members
}

// origin finds one segment of a kind, and fails when it is not there.
func segmentOf(t *testing.T, member string, segments []definition.PromptSegment, kind string) definition.PromptSegment {
	t.Helper()
	for _, segment := range segments {
		if segment.Kind == kind {
			return segment
		}
	}
	t.Fatalf("%s has no %s segment: %+v", member, kind, segments)
	return definition.PromptSegment{}
}

// TestUpDrySegmentsJoinThePrompt: a member's prompt is the texts of its segments
// joined in order, character for character, and the segments say where the
// template, the role document and the team's rules came from (r-4cf9). The team
// prompt and the member prompt are both used.
func TestUpDrySegmentsJoinThePrompt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
mission: a mission for the test
prompt: |
  Team template for %{team} / %{name}.
  %{role_doc}
  %{docs}
  ## Roster
  %{roster}
  ## Contact
  %{herdr}
  %{turns}%{workers}
members:
  - name: lead
    role: lead
    profile: claude-default
    prompt: |
      Member template for %{name}.
      %{role_doc}
      %{docs}
      ## Roster
      %{roster}
      ## Contact
      %{herdr}
      %{turns}%{workers}
  - name: builder
    role: builder
    profile: claude-default
turns:
  - human -> lead
  - lead -> human
  - lead -> builder
  - builder -> lead
`)
	out := mustRun(t, "up", "demo", "--dry", "--json")
	members := promptPlan(t, out)
	if len(members) != 2 {
		t.Fatalf("the plan has %d members, want 2", len(members))
	}
	for _, member := range members {
		joined := ""
		for _, segment := range member.Segments {
			joined += segment.Text
		}
		if joined != member.Prompt {
			t.Errorf("%s's segments do not join to its prompt:\n%s", member.Name, joined)
		}
		role := segmentOf(t, member.Name, member.Segments, "role_doc")
		if role.Origin.Kind != "role" || role.Origin.Place != "project" || !strings.HasSuffix(role.Origin.Path, "roles/"+strings.TrimPrefix(member.Name, "demo-")+".yaml") {
			t.Errorf("%s's role segment reads %+v", member.Name, role)
		}
		rules := segmentOf(t, member.Name, member.Segments, "team_rules")
		if rules.Origin.Kind != "team_rules" || rules.Origin.Path != ".hergy/teams/demo/agreement.yaml" {
			t.Errorf("%s's team rules segment reads %+v", member.Name, rules)
		}
	}
	lead := members[0]
	if !strings.Contains(lead.Prompt, "Member template for demo-lead") {
		t.Errorf("the member prompt is not lead's own:\n%s", lead.Prompt)
	}
	template := segmentOf(t, lead.Name, lead.Segments, "template")
	if template.Origin.Kind != "member" || template.Origin.Name != "lead" || template.Origin.Path != ".hergy/teams/demo/team.yaml" {
		t.Errorf("lead's template segment reads %+v", template)
	}
	builder := members[1]
	if !strings.Contains(builder.Prompt, "Team template for demo / demo-builder") {
		t.Errorf("the team prompt is not builder's:\n%s", builder.Prompt)
	}
	template = segmentOf(t, builder.Name, builder.Segments, "template")
	if template.Origin.Kind != "team" || template.Origin.Name != "demo" {
		t.Errorf("builder's template segment reads %+v", template)
	}
}

// TestHireDrySegmentsJoinThePrompt: a worker's prompt is its segments joined,
// and the task it was hired for is its own segment (r-4cf9).
func TestHireDrySegmentsJoinThePrompt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), `version: 1
mission: a mission for the test
members:
  - name: lead
    role: lead
    profile: claude-default
  - name: builder
    role: builder
    profile: claude-default
workers:
  - from: lead
    purpose: 調査
    profile: claude-default
turns:
  - human -> lead
  - lead -> human
  - lead -> builder
  - builder -> lead
`)
	out := mustRun(t, "hire", "helper", "--purpose", "調査", "--task", "read the thing", "--team", "demo", "--dry", "--json")
	members := promptPlan(t, out)
	if len(members) != 1 {
		t.Fatalf("the plan has %d members, want the worker", len(members))
	}
	worker := members[0]
	joined := ""
	for _, segment := range worker.Segments {
		joined += segment.Text
	}
	if joined != worker.Prompt {
		t.Errorf("the worker's segments do not join to its prompt:\n%s", joined)
	}
	task := segmentOf(t, worker.Name, worker.Segments, "task")
	if task.Text != "read the thing" || task.Origin.Kind != "hire" {
		t.Errorf("the task segment reads %+v", task)
	}
	herdr := segmentOf(t, worker.Name, worker.Segments, "herdr")
	if herdr.Origin.Kind != "hergy" {
		t.Errorf("the contact segment reads %+v", herdr)
	}
}

// The values the screen asks a worker's prompt with: they are chosen at hire,
// and the column shows slots where they stand (teams/state.js workerPrompt).
const (
	canvasWorkerName = "zz-worker-name-0"
	canvasWorkerTask = "zzTaskPlaceholder0"
)

// canvasCheck is one /api/team/check answer as this test reads it.
type canvasCheck struct {
	Members []struct {
		Name     string                     `json:"name"`
		Segments []definition.PromptSegment `json:"segments"`
	} `json:"members"`
	Prompt   []definition.PromptSegment `json:"prompt"`
	Problems []struct {
		Text string `json:"text"`
	} `json:"problems"`
}

// canvasJoin puts segments back together in order.
func canvasJoin(segments []definition.PromptSegment) string {
	var out strings.Builder
	for _, segment := range segments {
		out.WriteString(segment.Text)
	}
	return out.String()
}

// canvasToken reads a page's own token, the one its writes carry.
func canvasToken(t *testing.T, base, path string) string {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s answers %d: %s", path, resp.StatusCode, body)
	}
	found := regexp.MustCompile(`data-token="([^"]+)"`).FindSubmatch(body)
	if found == nil {
		t.Fatalf("%s carries no token:\n%s", path, body)
	}
	return string(found[1])
}

// canvasAsk sends one check to the running serve, the way the attribute column
// sends it.
func canvasAsk(t *testing.T, base, token string, payload map[string]any) canvasCheck {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/team/check", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hergy-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ask the check: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the check answers %d: %s", resp.StatusCode, body)
	}
	var answer canvasCheck
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("the check does not read: %v\n%s", err, body)
	}
	return answer
}

// TestCANVAS_PROMPT_TheCheckSegmentsAreThePlans: the lines the attribute column
// draws are the check's segments (r-4cf9), and put back together they are
// `up --dry`'s prompt line for line (ac-5bfa). The sample's three teams are the
// three places a template can come from: hergy's own, the team's, and one
// member's own. A worker's name and task are chosen at hire, so the screen asks
// its prompt with two values of its own; hire --dry with the same values gives
// the prompt the column's placeholders stand in.
func TestCANVAS_PROMPT_TheCheckSegmentsAreThePlans(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	src := filepath.Join(cwd, "..", "..", "testdata")
	copyDir(t, filepath.Join(src, "teams-v2", "project", ".hergy"), filepath.Join(f.proj, ".hergy"))
	copyDir(t, filepath.Join(src, "teams-v2", "config", "hergy"), filepath.Join(f.cfg, "hergy"))
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")

	base, stop := startServe(t, f)
	defer stop()
	base = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]+`).FindString(base)
	if base == "" {
		t.Fatal("the serve printed no address")
	}
	token := canvasToken(t, base, "/teams")

	for _, team := range []string{"core", "docs", "research"} {
		plan := promptPlan(t, mustRun(t, "up", team, "--dry", "--json"))
		if len(plan) == 0 {
			t.Fatalf("%s has no members", team)
		}
		answer := canvasAsk(t, base, token, map[string]any{
			"place": "project", "name": team, "part": "team",
			"doc": map[string]any{"ops": []map[string]any{}},
		})
		// A warning (the sample's own size) does not keep hergy from building
		// the prompt; only a problem that stops it leaves the members out
		// (d-20ac), and then there is nothing to compare.
		if len(answer.Members) != len(plan) {
			t.Fatalf("%s's check answers %d members, the plan has %d", team, len(answer.Members), len(plan))
		}
		for _, member := range plan {
			var segments []definition.PromptSegment
			// The plan names a member by its herdr name; the column names it
			// by its own (teams/canvas.js), so the team's prefix is dropped.
			short := strings.TrimPrefix(member.Name, team+"-")
			for _, got := range answer.Members {
				if got.Name == short {
					segments = got.Segments
				}
			}
			if segments == nil {
				t.Fatalf("%s's check does not answer %s's prompt", team, member.Name)
			}
			// The column draws the segments' own texts, so what it puts back
			// together is the plan's prompt.
			if joined := canvasJoin(segments); joined != member.Prompt {
				t.Errorf("%s: the check's segments do not join to up --dry's prompt:\n got:\n%s\nwant:\n%s", member.Name, joined, member.Prompt)
			}
			segmentOf(t, member.Name, segments, "role_doc")
			segmentOf(t, member.Name, segments, "team_rules")
		}
	}

	// The worker: the screen asks with the two values it shows as slots, so the
	// same values through hire --dry give the same prompt.
	answer := canvasAsk(t, base, token, map[string]any{
		"place": "project", "name": "core", "part": "worker",
		"doc": map[string]any{
			"name": canvasWorkerName, "purpose": "依存の更新", "task": canvasWorkerTask, "from": "lead",
		},
	})
	hire := promptPlan(t, mustRun(t, "hire", canvasWorkerName, "--from", "lead", "--purpose", "依存の更新", "--task", canvasWorkerTask, "--team", "core", "--dry", "--json"))
	if len(hire) != 1 {
		t.Fatalf("the hire plan has %d members, want the worker", len(hire))
	}
	joined := canvasJoin(answer.Prompt)
	if joined != hire[0].Prompt {
		t.Errorf("the check's worker segments do not join to hire --dry's prompt:\n got:\n%s\nwant:\n%s", joined, hire[0].Prompt)
	}
	// The two values are what the column turns into slots (the unit test holds
	// the slots' own text and identity).
	full := "core-" + canvasWorkerName
	if !strings.Contains(joined, full) || !strings.Contains(joined, canvasWorkerTask) {
		t.Errorf("the worker's prompt does not hold the values the column shows as slots:\n%s", joined)
	}
	task := segmentOf(t, hire[0].Name, answer.Prompt, "task")
	if task.Text != canvasWorkerTask {
		t.Errorf("the task segment reads %+v", task)
	}
}
