package app

import (
	"encoding/json"
	"path/filepath"
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
