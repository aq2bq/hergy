// These are the tests of reading team.yaml, moved from internal/config when the
// reading became definition.Read and the prompt became prompt.Member. The
// package is the external test package, because building a member's prompt is
// prompt.Member's and this package must not import it.
//
// 段 4 moved where the documents stand (K3, K5): a team's own agreement is
// teams/<name>/agreement.yaml, the agreement every team of the layer shares is
// agreement.yaml in the layer's root, and a member's role document is
// roles/<role>.yaml in the layer. A test team therefore stands in a layer (the
// layer type below), and the catalog over that layer is what a prompt reads the
// two documents through (prompt.Sources).
package definition_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testProfiles is the one profile a test team names: kind is the profile's, so a
// team.yaml body names it rather than writing the kind itself (I7).
func testProfiles(dir string) map[string]profile.Profile {
	return map[string]profile.Profile{
		"claude-default": {Name: "claude-default", Kind: "claude", Path: filepath.Join(dir, "profiles.yaml")},
	}
}

// layer is one test's layer: the root a team definition stands in, and that
// definition's own directory. The layer holds the documents that are not the
// team's own (段 4): roles/<role>.yaml, and agreement.yaml when the layer writes
// one.
type layer struct {
	root string // the layer's root: roles/ and agreement.yaml stand here
	team string // the team definition's own directory (teams/t)
}

// newLayer builds a layer with the definition's directory, a role document for
// the role every test team names (r), and the shared agreement when one is given
// (empty for none).
func newLayer(t *testing.T, shared string) layer {
	t.Helper()
	root := t.TempDir()
	l := layer{root: root, team: filepath.Join(root, "teams", "t")}
	if err := os.MkdirAll(l.team, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "roles", "r.yaml"), oneRule("R0", "role doc"))
	if shared != "" {
		write(t, filepath.Join(root, "agreement.yaml"), shared)
	}
	return l
}

// sources is the catalog a prompt looks a role document and the shared agreement
// up in, over this layer. The global layer is a directory that is not there, so
// every lookup is answered by this layer.
func (l layer) sources() prompt.Sources {
	return catalog.New(filepath.Join(l.root, "global"), l.root, l.root, source.Disk{},
		func(path string) string { return path })
}

// rel is where one document stands in the layer, for the assertions below.
func (l layer) rel(t *testing.T, path string) string {
	t.Helper()
	out, err := filepath.Rel(l.root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(out)
}

// built is one team definition read: the definition itself and its members as
// the launcher would take them (the same projection app builds).
type built struct {
	def    *definition.Definition
	starts []definition.Start
}

// parseTeam reads a team.yaml body written into the layer's definition, with the
// one test profile.
func parseTeam(t *testing.T, l layer, text string) (*built, error) {
	t.Helper()
	return parse(text, "t", "", l, testProfiles(l.team))
}

// TestReadHandsBackADefinitionWhoseTurnsCannotHold: a member that hands its turn
// to nobody stops the definition (I12a, I12b), and every command that reads it
// stops on the sentence. The definition itself comes back beside the error,
// because the parts of a prompt a change moves are readable from a graph that
// cannot hold: a screen's card shows the line a change takes away even when the
// change is what stops the definition (spec-teams 1.4.4, 監査 R11).
func TestReadHandsBackADefinitionWhoseTurnsCannotHold(t *testing.T) {
	l := newLayer(t, "")
	text := "version: 1\nmission: 何かをする。\nmembers:\n" +
		"  - name: a\n    role: r\n    profile: claude-default\n" +
		"  - name: b\n    role: r\n    profile: claude-default\n" +
		"turns:\n  - human -> a\n  - a -> human\n  - a -> b\n"

	def, err := definition.Read(definition.Input{
		YAML:         text,
		Name:         "t",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(l.team, "team.yaml"),
		TeamDir:      l.team,
		Base:         l.team,
		Reader:       source.Disk{},
	})
	var gerr *definition.GraphError
	if !errors.As(err, &gerr) {
		t.Fatalf("read = %v, want the one that stops on b handing its turn to nobody", err)
	}
	if len(gerr.Problems) != 1 || gerr.Problems[0].Key != "error.graph_member_no_turn" {
		t.Errorf("the problems read %+v, want b handing its turn to nobody", gerr.Problems)
	}
	if def == nil || def.Graph == nil {
		t.Fatalf("the definition came back as %v, want it with its graph, for the parts a change moves", def)
	}
	if def.Name != "t" {
		t.Errorf("the definition came back named %q, want t", def.Name)
	}
	b := definition.NodeID{Kind: definition.KindMember, Name: "b"}
	if _, ok := def.Graph.Member("b"); !ok {
		t.Error("the definition came back without b, want its members")
	}
	if got := def.Graph.Out(b); len(got) != 0 {
		t.Errorf("b hands its turn to %+v, want nobody", got)
	}
}

func parse(text, name, runName string, l layer, profiles map[string]profile.Profile) (*built, error) {
	def, err := definition.Read(definition.Input{
		YAML:         text,
		Name:         name,
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(l.team, "team.yaml"),
		TeamDir:      l.team,
		Base:         l.team,
		Reader:       source.Disk{},
	})
	if err != nil {
		return nil, err
	}
	if runName == "" {
		runName = name
	}
	starts, err := startsOf(def, runName, l, profiles)
	if err != nil {
		return nil, err
	}
	return &built{def: def, starts: starts}, nil
}

// startsOf is the member projection app builds: the herdr name, the profile's
// engine, and prompt.Member's text.
func startsOf(def *definition.Definition, runName string, l layer, profiles map[string]profile.Profile) ([]definition.Start, error) {
	run := prompt.Run{Team: runName, Executable: "hergy", Base: l.team, Reader: source.Disk{}, Sources: l.sources()}
	out := make([]definition.Start, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		node := &def.Graph.Members[i]
		herdr := runName + "-" + node.Name
		if !profile.NamePattern.MatchString(herdr) {
			return nil, i18n.Errorf("error.herdr_name_pattern", herdr)
		}
		text, err := prompt.Member(def, node, run)
		if err != nil {
			return nil, err
		}
		value := profiles[node.Profile]
		out = append(out, definition.Start{
			Name: herdr, Short: node.Name, Role: node.Role, Profile: node.Profile,
			Kind: value.Kind, Args: value.Args, Env: value.Env, Cwd: node.Cwd,
			Prompt: text.Text, RoleDoc: text.Role, HasRole: text.HasRole, Read: text.Read,
		})
	}
	return out, nil
}

func (b *built) usesProfiles() bool {
	return len(b.def.Graph.Members) > 0 || len(b.def.Graph.Workers) > 0
}

// workerProfile is what one member's workers pair with a purpose.
func workerProfile(def *definition.Definition, member, purpose string) (string, bool) {
	for _, worker := range def.Graph.Workers {
		if worker.From == member && worker.Purpose == purpose {
			return worker.Profile, true
		}
	}
	return "", false
}

// teamText is a team.yaml body whose prompt is only the two doors, so the tests
// read what a member is handed without any other prompt part in the way. A
// template must keep both doors (d-0ccd), so every test team writes them and
// every test layer holds the role document r. Its turns are the shortest graph
// a team can have (段 5): the human hands the turn to a, and a hands it back.
func teamText() string {
	return "version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}\"\n" +
		"members:\n  - name: a\n    role: r\n    profile: claude-default\n" +
		"turns:\n  - human -> a\n  - a -> human\n"
}

// oneRule is a rule file with one section (no heading) and one rule, so a test
// can put a known sentence into a member's prompt.
func oneRule(id, text string) string {
	return "version: 1\nsections:\n  - rules:\n      - id: " + id + "\n        text: |-\n          " + text + "\n"
}

// TestMemberReadsTheRoleThenTheSharedThenTheTeamAgreement: the documents reach a
// member's prompt in the one order (段 4): the member's role document, the
// agreement every team of the layer shares, then the team's own agreement.
// Built.Read carries the two agreements, in that order, and the role document is
// the separate one that the member's own line shows.
func TestMemberReadsTheRoleThenTheSharedThenTheTeamAgreement(t *testing.T) {
	l := newLayer(t, oneRule("S1", "shared doc"))
	write(t, filepath.Join(l.team, "agreement.yaml"), oneRule("T1", "team doc"))

	got, err := parseTeam(t, l, teamText())
	if err != nil {
		t.Fatal(err)
	}
	want := "- **R0** role doc\n- **S1** shared doc\n\n- **T1** team doc\n"
	if got.starts[0].Prompt != want {
		t.Errorf("prompt = %q, want %q", got.starts[0].Prompt, want)
	}
	var read []string
	for _, doc := range got.starts[0].Read {
		read = append(read, l.rel(t, doc.Path))
	}
	if wantRead := "agreement.yaml,teams/t/agreement.yaml"; strings.Join(read, ",") != wantRead {
		t.Errorf("Read = %v, want %v", read, wantRead)
	}
	if !got.starts[0].HasRole || l.rel(t, got.starts[0].RoleDoc.Path) != "roles/r.yaml" {
		t.Errorf("the role document is %q, %v; want roles/r.yaml, true",
			got.starts[0].RoleDoc.Path, got.starts[0].HasRole)
	}
	if !got.def.HasAgreement || l.rel(t, got.def.Agreement.Path) != "teams/t/agreement.yaml" {
		t.Errorf("the team's agreement is %q, %v; want teams/t/agreement.yaml, true",
			got.def.Agreement.Path, got.def.HasAgreement)
	}
}

// TestMemberReadsNoAgreement: both agreements are optional (段 4): a team that
// writes none, in a layer that writes none, hands the member its role document
// and nothing else, and the prompt is what is left.
func TestMemberReadsNoAgreement(t *testing.T) {
	l := newLayer(t, "")
	got, err := parseTeam(t, l, teamText())
	if err != nil {
		t.Fatal(err)
	}
	if got.starts[0].Prompt != "- **R0** role doc" {
		t.Errorf("prompt = %q, want %q", got.starts[0].Prompt, "- **R0** role doc")
	}
	if len(got.starts[0].Read) != 0 {
		t.Errorf("Read = %v, want none", got.starts[0].Read)
	}
	if got.def.HasAgreement {
		t.Error("HasAgreement = true, want false")
	}
}

// TestParseRendersWhatIsSent: a rule file is rendered to Markdown, and only what
// a member reads reaches the prompt: the title, the headings, the intro, the
// text, the reason and the report shapes. The revision, the note, the origin,
// the report pointer and the tests stay in the file.
func TestParseRendersWhatIsSent(t *testing.T) {
	l := newLayer(t, "")
	write(t, filepath.Join(l.team, "agreement.yaml"), `version: 1
title: TEAM PROTOCOL
revision: 3
sections:
  - heading: 基本
    intro: 前置きである。
    style: ids
    rules:
      - id: R1
        text: 本文である。
        why: 理由である。
        note: 保守する人のメモ
        report: 監査.scope
        tests: [T-1]
        origin:
          - kind: incident
            note: 覚え書き
reports:
  監査:
    where: 担当する Issue
    fields:
      scope: 変更の範囲
`)

	got, err := parseTeam(t, l, teamText())
	if err != nil {
		t.Fatal(err)
	}
	prompt := got.starts[0].Prompt
	for _, want := range []string{
		"# TEAM PROTOCOL", "## 基本", "前置きである。", "- **R1** 本文である。 理由である。",
		"## " + i18n.T("rules.reports_heading"), "**監査** (担当する Issue)", "- `scope`: 変更の範囲",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	for _, unwanted := range []string{"revision", "保守する人のメモ", "覚え書き", "T-1", "監査.scope"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt still contains %q:\n%s", unwanted, prompt)
		}
	}
}

// TestMemberStopsOnARoleThatIsStillMarkdown: a role left as roles/<role>.md
// stops with the way to move over (段 4): the place is now one, so the sentence
// names it.
func TestMemberStopsOnARoleThatIsStillMarkdown(t *testing.T) {
	l := newLayer(t, "")
	write(t, filepath.Join(l.root, "roles", "r.md"), "- R1: old\n")
	if err := os.Remove(filepath.Join(l.root, "roles", "r.yaml")); err != nil {
		t.Fatal(err)
	}
	_, err := parseTeam(t, l, teamText())
	if err == nil {
		t.Fatal("a role still left as Markdown did not stop the prompt")
	}
	if !strings.Contains(err.Error(), "r.md") {
		t.Errorf("error %q does not name the Markdown file", err)
	}
}

// TestMemberStopsOnAMissingRole: a member whose role has no document at all is
// stopped, and the sentence names the member, the role and every place it looks
// (段 4, spec/32 D2).
func TestMemberStopsOnAMissingRole(t *testing.T) {
	l := newLayer(t, "")
	_, err := parseTeam(t, l,
		"version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: lead\n    role: absent\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n")
	if err == nil {
		t.Fatal("a missing role document did not stop the prompt")
	}
	for _, want := range []string{"lead", "absent", "roles/absent.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestMemberTemplateMustKeepTheTwoDoors: a template that drops the role
// document or the agreements stops (d-0ccd, I4), whichever file wrote it, and
// the sentence names the team.yaml it read (spec/32 D2).
func TestMemberTemplateMustKeepTheTwoDoors(t *testing.T) {
	cases := []struct{ name, prompt, want string }{
		{"a prompt with no role document", `"%{docs}"`, "role_doc"},
		{"a prompt with no agreements", `"%{role_doc}"`, "docs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayer(t, "")
			_, err := parseTeam(t, l,
				"version: 1\nmission: a mission for the test\nteam: t\nprompt: "+tc.prompt+"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\nturns:\n  - human -> a\n  - a -> human\n")
			if err == nil {
				t.Fatal("a template with a door missing did not stop the prompt")
			}
			for _, want := range []string{tc.want, filepath.Join(l.team, "team.yaml")} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestMemberStopsOnBreaksAcrossTheDocumentsOneMemberReads: a rule id and a report
// name are unique among the documents that go into one member's prompt, and a
// report and a derived rule point at something those documents hold. The two
// documents are the layer's shared agreement and the team's own.
func TestMemberStopsOnBreaksAcrossTheDocumentsOneMemberReads(t *testing.T) {
	audit := "reports:\n  監査:\n    where: 記録\n    fields:\n      scope: 範囲\n"
	cases := []struct {
		name   string
		shared string
		team   string
		want   string
	}{
		{
			name:   "a rule id that stands twice",
			shared: oneRule("R1", "one"),
			team:   oneRule("R1", "two"),
			want:   "R1",
		},
		{
			name:   "a report name that stands twice",
			shared: "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n" + audit,
			team:   "version: 1\nsections:\n  - rules:\n      - id: B1\n        text: y\n" + audit,
			want:   "監査",
		},
		{
			name: "a report that is not there",
			team: "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n        report: 監査.absent\n" + audit,
			want: "監査.absent",
		},
		{
			name: "a derived rule that is not there",
			team: "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n        origin:\n          - kind: derived\n            refs: [B9]\n",
			want: "B9",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayer(t, tc.shared)
			if tc.team != "" {
				write(t, filepath.Join(l.team, "agreement.yaml"), tc.team)
			}
			_, err := parseTeam(t, l, teamText())
			if err == nil {
				t.Fatal("a break across the documents did not stop the prompt")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestTeamYAMLDocsAndRolesAreGone: docs: and roles: are gone from team.yaml
// (段 4); a file that still writes one is stopped with the sentence that says
// where the documents went, rather than with an unknown-key complaint.
func TestTeamYAMLDocsAndRolesAreGone(t *testing.T) {
	cases := []struct{ name, key, want string }{
		{"docs", "docs: [old.yaml]\n", "agreement.yaml"},
		{"roles", "roles: [../roles]\n", "roles/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayer(t, "")
			_, err := parseTeam(t, l,
				"version: 1\nmission: a mission for the test\nteam: t\n"+tc.key+"members:\n  - name: a\n    role: r\n    profile: claude-default\n")
			if err == nil {
				t.Fatalf("%s: in team.yaml did not stop the read", tc.name)
			}
			for _, want := range []string{tc.name, tc.want} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestParseStopsOnBrokenYAML(t *testing.T) {
	cases := []struct {
		name  string
		parse func() error
	}{
		{
			name: "team.yaml without a version",
			parse: func() error {
				_, err := parseTeam(t, newLayer(t, ""), "team: t\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
				return err
			},
		},
		{
			name: "team.yaml with an unknown key",
			parse: func() error {
				_, err := parseTeam(t, newLayer(t, ""), "version: 1\nmission: a mission for the test\nteam: t\nunknown_key: 1\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(); err == nil {
				t.Error("broken YAML did not stop the parse")
			}
		})
	}
}

func TestParseReadsProfilesIntoMembers(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{
		"lead": {Name: "lead", Kind: "claude", Args: []string{"--model", "opus"}, Path: "profiles.yaml"},
	}
	got, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{name}\"\nmembers:\n  - name: a\n    role: r\n    profile: lead\nturns:\n  - human -> a\n  - a -> human\n", "t", "", l, profiles)
	if err != nil {
		t.Fatal(err)
	}
	member := got.starts[0]
	if member.Kind != "claude" || member.Profile != "lead" {
		t.Errorf("member kind/profile = %q/%q, want claude/lead", member.Kind, member.Profile)
	}
	if strings.Join(member.Args, " ") != "--model opus" {
		t.Errorf("member args = %v", member.Args)
	}
	if !got.usesProfiles() {
		t.Error("usesProfiles() = false, want true")
	}
}

// parseWorkersTeam builds a one-member team whose top-level workers: is the
// given block (already indented under workers:), with two profiles defined.
func parseWorkersTeam(t *testing.T, workers string) (*built, error) {
	t.Helper()
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{
		"seeker":    {Name: "seeker", Kind: "claude"},
		"developer": {Name: "developer", Kind: "codex"},
	}
	return parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{workers}\"\nmembers:\n"+
		"  - name: lead\n    role: r\n    profile: seeker\n"+
		"workers:\n"+workers+
		"turns:\n  - human -> lead\n  - lead -> human\n", "t", "", l, profiles)
}

// TestWorkerNoteNamesTheHerdrName: the member's "%{workers}" note says who a hired
// worker reports back to. That name is the member's herdr name, the same name
// the worker's own prompt uses, so it follows --as rather than the short name
// written in team.yaml (only the embedded hire command takes the short name).
func TestWorkerNoteNamesTheHerdrName(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{
		"seeker": {Name: "seeker", Kind: "claude"},
	}
	got, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{workers}\"\nmembers:\n"+
		"  - name: lead\n    role: r\n    profile: seeker\n"+
		"workers:\n  - from: lead\n    purpose: build\n    profile: seeker\n"+
		"turns:\n  - human -> lead\n  - lead -> human\n",
		"t", "alt", l, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.starts[0].Prompt, "reports back to you (alt-lead)") {
		t.Errorf("the worker note does not name the herdr name (alt-lead):\n%s", got.starts[0].Prompt)
	}
	if strings.Contains(got.starts[0].Prompt, "reports back to you (lead)") {
		t.Errorf("the worker note names the short member name:\n%s", got.starts[0].Prompt)
	}
}

// TestWorkersReadPurposeProfilePairs: workers: is a list of member, purpose and
// profile, and each pair reaches that member's prompt.
func TestWorkersReadPurposeProfilePairs(t *testing.T) {
	got, err := parseWorkersTeam(t, "  - from: lead\n    purpose: research\n    profile: seeker\n"+
		"  - from: lead\n    purpose: build\n    profile: developer\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.def.Graph.Workers) == 0 {
		t.Fatal("workers: was not read")
	}
	if profile, ok := workerProfile(got.def, "lead", "build"); !ok || profile != "developer" {
		t.Errorf("workerProfile(build) = %q, %v, want developer, true", profile, ok)
	}
	if !got.usesProfiles() {
		t.Error("usesProfiles() = false, want true")
	}
	for _, want := range []string{"research → seeker", "build → developer"} {
		if !strings.Contains(got.starts[0].Prompt, want) {
			t.Errorf("the prompt does not pair %q:\n%s", want, got.starts[0].Prompt)
		}
	}
}

// TestWorkersStopsOnBrokenEntries: every way a workers: entry breaks stops the
// parse, naming the entry or the key it did not like.
func TestWorkersStopsOnBrokenEntries(t *testing.T) {
	cases := []struct{ name, block, want string }{
		{"an entry without a member", "  - purpose: research\n    profile: seeker\n", "from"},
		{"an entry without a purpose", "  - from: lead\n    profile: seeker\n", "purpose"},
		{"an entry without a profile", "  - from: lead\n    purpose: research\n", "profile"},
		{"two workers of one member and purpose", "  - from: lead\n    purpose: research\n    profile: seeker\n  - from: lead\n    purpose: research\n    profile: developer\n", "research"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseWorkersTeam(t, tc.block)
			if err == nil {
				t.Fatal("a broken workers: entry did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestMaySpawnIsGone: the workers moved to the top level (段 5). A member that
// still writes may_spawn is stopped with the sentence that says where they went,
// rather than being read as an unknown key.
func TestMaySpawnIsGone(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
	_, err := parse("version: 1\nmission: a mission for the test\nteam: t\nmembers:\n"+
		"  - name: lead\n    role: r\n    profile: seeker\n"+
		"    may_spawn:\n      workers:\n        - purpose: research\n          profile: seeker\n",
		"t", "", l, profiles)
	if err == nil {
		t.Fatal("a member that still writes may_spawn did not stop the parse")
	}
	for _, want := range []string{"may_spawn", "workers:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestSpawnPlaceholderIsGone: the part of a prompt template that lists the
// workers is %{workers} now. A template that still writes %{spawn} is stopped
// with the name of the part that replaced it (段 5).
func TestSpawnPlaceholderIsGone(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
	cases := []struct{ name, text string }{
		{"the team's own template", "version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{spawn}\"\nmembers:\n  - name: lead\n    role: r\n    profile: seeker\n"},
		{"one member's template", "version: 1\nmission: a mission for the test\nteam: t\nmembers:\n  - name: lead\n    role: r\n    profile: seeker\n    prompt: \"%{spawn}\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(tc.text, "t", "", l, profiles)
			if err == nil {
				t.Fatal("a template that still writes %{spawn} did not stop the parse")
			}
			if !strings.Contains(err.Error(), "%{workers}") {
				t.Errorf("error %q does not name the new placeholder", err)
			}
		})
	}
}

// TestTurnsReadsTheGraph: turns: lines are read into the graph's turns, in the
// order they are written, with the human and the member names as the ends.
func TestTurnsReadsTheGraph(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
	got, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{turns}\"\nmembers:\n"+
		"  - name: lead\n    role: r\n    profile: seeker\n"+
		"  - name: impl\n    role: r\n    profile: seeker\n"+
		"turns:\n  - human -> lead\n  - lead -> impl\n  - impl -> human\n", "t", "", l, profiles)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"human -> lead", "lead -> impl", "impl -> human"}
	if len(got.def.Graph.Turns) != len(want) {
		t.Fatalf("turns = %v, want %v", got.def.Graph.Turns, want)
	}
	for i, edge := range got.def.Graph.Turns {
		if got := edge.String(); got != want[i] {
			t.Errorf("turns[%d] = %q, want %q", i, got, want[i])
		}
	}
}

// TestTurnsStopsOnBrokenLines: every way a turns: line breaks stops the parse.
func TestTurnsStopsOnBrokenLines(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"no arrow", "  - lead impl\n", "lead impl"},
		{"two arrows", "  - lead -> impl -> human\n", "->"},
		{"an empty left end", "  - -> lead\n", "->"},
		{"an empty right end", "  - lead ->\n", "->"},
		{"a kind prefix", "  - lead -> team:review\n", "team:review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayer(t, "")
			profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
			_, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{turns}\"\nmembers:\n"+
				"  - name: lead\n    role: r\n    profile: seeker\n"+
				"turns:\n"+tc.line, "t", "", l, profiles)
			if err == nil {
				t.Fatal("a broken turns: line did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestTurnsBrokenLineNamesTheLine: a broken turns: line is reported by the line
// a person counts, not by the index hergy walks, so "turns:" does not appear
// twice (spec/33, 段 7).
func TestTurnsBrokenLineNamesTheLine(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
	// The broken line is the second one, so a person calls it line 2.
	_, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{turns}\"\nmembers:\n"+
		"  - name: lead\n    role: r\n    profile: seeker\n"+
		"turns:\n  - human -> lead\n  - lead impl\n", "t", "", l, profiles)
	if err == nil {
		t.Fatal("a broken turns: line did not stop the parse")
	}
	want := i18n.Tf("label.turns_line", 2)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the error %q does not name %q", err, want)
	}
}

// TestTurnLeavesThePromptWithItsEdge is the golden of I13: the turns a member's
// prompt names come from the graph, so taking one turn out of turns: takes
// exactly that party out of the prompt and leaves every other line as it was.
func TestTurnLeavesThePromptWithItsEdge(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"seeker": {Name: "seeker", Kind: "claude"}}
	head := "version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{name}\\n%{turns}\"\nmembers:\n" +
		"  - name: lead\n    role: r\n    profile: seeker\n" +
		"  - name: spec\n    role: r\n    profile: seeker\n"
	full, err := parse(head+"turns:\n  - human -> lead\n  - lead -> spec\n  - lead -> human\n  - spec -> lead\n", "t", "", l, profiles)
	if err != nil {
		t.Fatal(err)
	}
	less, err := parse(head+"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> lead\n", "t", "", l, profiles)
	if err != nil {
		t.Fatal(err)
	}

	// The lead hands the turn to spec and to the human; taking the turn to spec
	// away takes that one line out.
	specLine := fmt.Sprintf(i18n.T("turns.member_item"), "t-spec", "r") + "\n"
	if !strings.Contains(full.starts[0].Prompt, specLine) {
		t.Fatalf("the prompt does not name spec although lead -> spec is written:\n%s", full.starts[0].Prompt)
	}
	humanLine := i18n.T("turns.human_item") + "\n"
	if got := strings.Replace(full.starts[0].Prompt, specLine, "", 1); got != less.starts[0].Prompt {
		t.Errorf("taking lead -> spec away changed more than its line:\ngot:\n%s\nwant:\n%s", got, less.starts[0].Prompt)
	}
	if !strings.Contains(less.starts[0].Prompt, humanLine) {
		t.Errorf("taking one turn away took the turn to the human with it:\n%s", less.starts[0].Prompt)
	}
}

// TestParseKeepsAnUnknownProfile: a profile profiles.yaml does not hold is not
// the reading's business (spec-teams 1.9): the file reads with the name as it is
// written, so a screen draws the definition and a person fixes the name, and
// the check reports the name the definition cannot resolve.
func TestParseKeepsAnUnknownProfile(t *testing.T) {
	got, err := parse("version: 1\nmission: a mission for the test\nteam: t\nmembers:\n  - name: a\n    role: r\n    profile: absent\nturns:\n  - human -> a\n  - a -> human\n", "t", "", newLayer(t, ""), testProfiles(""))
	if err != nil {
		t.Fatalf("a profile that does not exist stopped the parse: %v", err)
	}
	if got.def.Graph.Members[0].Profile != "absent" {
		t.Errorf("the member's profile reads %q, want the name as it is written", got.def.Graph.Members[0].Profile)
	}
}

// TestMemberCannotWriteItsOwnEngine: kind, args and env are the profile's, so a
// member that writes one of them is stopped with the sentence that says where it
// goes (I7). The three are refused the same way, so a file that reads a member
// with one of them is fixed before the team boots.
func TestMemberCannotWriteItsOwnEngine(t *testing.T) {
	cases := []struct{ name, member, want string }{
		{"kind", "    kind: claude\n", "kind"},
		{"args", "    args: [--model, opus]\n", "args"},
		{"env", "    env: {ANTHROPIC_MODEL: opus}\n", "env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTeam(t, newLayer(t, ""),
				"version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\n"+tc.member)
			if err == nil {
				t.Fatalf("a member writing %s did not stop the parse", tc.name)
			}
			for _, want := range []string{"lead", tc.want, "profiles.yaml"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestMemberNeedsAProfile: a member with no profile has no engine at all, and is
// stopped with the way to give it one.
func TestMemberNeedsAProfile(t *testing.T) {
	_, err := parseTeam(t, newLayer(t, ""), "version: 1\nmission: a mission for the test\nteam: t\nmembers:\n  - name: lead\n    role: r\n")
	if err == nil {
		t.Fatal("a member with no profile did not stop the parse")
	}
	for _, want := range []string{"lead", "profile", "profiles.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestMemberNameGetsTheTeamPrefix: the name written in team.yaml is the short member name;
// the member's herdr name is "<team>-<member>".
func TestMemberNameGetsTheTeamPrefix(t *testing.T) {
	l := newLayer(t, "")
	got, err := parseTeam(t, l, "version: 1\nmission: a mission for the test\nteam: t\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n")
	if err != nil {
		t.Fatal(err)
	}
	member := got.starts[0]
	if member.Short != "lead" {
		t.Errorf("Short = %q, want %q", member.Short, "lead")
	}
	if member.Name != "t-lead" {
		t.Errorf("Name = %q, want %q", member.Name, "t-lead")
	}
}

// TestMemberNameTakesTheRunName: --as replaces the prefix, not the member name.
func TestMemberNameTakesTheRunName(t *testing.T) {
	l := newLayer(t, "")
	got, err := parse("version: 1\nmission: a mission for the test\nteam: dev\nprompt: \"%{role_doc}%{docs}%{name}\"\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n", "dev", "dev2", l, testProfiles(l.team))
	if err != nil {
		t.Fatal(err)
	}
	if got.starts[0].Name != "dev2-lead" {
		t.Errorf("Name = %q, want %q", got.starts[0].Name, "dev2-lead")
	}
	if got.starts[0].Short != "lead" {
		t.Errorf("Short = %q, want %q", got.starts[0].Short, "lead")
	}
}

// TestMemberNameWithTeamPlaceholderStops: the team name is added for the writer,
// so a literal %{team} left in a member name is refused.
func TestMemberNameWithTeamPlaceholderStops(t *testing.T) {
	_, err := parseTeam(t, newLayer(t, ""), "version: 1\nmission: a mission for the test\nteam: t\nmembers:\n  - name: \"%{team}-lead\"\n    role: r\n    profile: claude-default\n")
	if err == nil {
		t.Fatal("a member name with %{team} did not stop the parse")
	}
	if !strings.Contains(err.Error(), "%{team}") {
		t.Errorf("error %q does not name the leftover placeholder", err)
	}
}

// TestMemberNameTooLongForHerdrStops: the team name and the member name are each within
// their own pattern, but the combined herdr name is checked too.
func TestMemberNameTooLongForHerdrStops(t *testing.T) {
	l := newLayer(t, "")
	_, err := parse("version: 1\nmission: a mission for the test\nmembers:\n  - name: worker\n    role: r\n    profile: claude-default\nturns:\n  - human -> worker\n  - worker -> human\n",
		"abcdefghijklmnopqrstuvwxyz", "", l, testProfiles(l.team)) // 26 chars, over the team-name limit
	if err == nil {
		t.Fatal("an over-long herdr name did not stop the parse")
	}
	if !strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz-worker") {
		t.Errorf("error %q does not name the combined name", err)
	}
}

// TestParseReadsTheMission: team.yaml's mission: is required (I15) and is the
// one line a team says it is for. The ends of what it wrote are trimmed, so a
// block scalar's trailing newline is not a second line.
func TestParseReadsTheMission(t *testing.T) {
	l := newLayer(t, "")
	// teamWith is a team.yaml body with the given text in place of the mission
	// line, so each case writes only what it is about.
	teamWith := func(mission string) string {
		return "version: 1\n" + mission +
			"team: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n" +
			"turns:\n  - human -> a\n  - a -> human\n"
	}
	for _, tc := range []struct {
		name    string
		yaml    string
		mission string
	}{
		{
			name:    "one line",
			yaml:    teamWith("mission:  hergy をよくする  \n"),
			mission: "hergy をよくする",
		},
		{
			name:    "a block scalar",
			yaml:    teamWith("mission: |-\n  one line\n"),
			mission: "one line",
		},
		{
			name:    "a block scalar with a trailing newline",
			yaml:    teamWith("mission: |\n  one line\n"),
			mission: "one line",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTeam(t, l, tc.yaml)
			if err != nil {
				t.Fatal(err)
			}
			if got.def.Mission != tc.mission {
				t.Errorf("Mission = %q, want %q", got.def.Mission, tc.mission)
			}
		})
	}
}

// TestParseStopsOnAMissionThatIsNotOneLine: a definition that writes none,
// writes an empty one, or writes one over more than one line does not read
// (I15).
func TestParseStopsOnAMissionThatIsNotOneLine(t *testing.T) {
	l := newLayer(t, "")
	teamWith := func(mission string) string {
		return "version: 1\n" + mission +
			"team: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n"
	}
	cases := []struct{ name, mission, key string }{
		{"no mission", "", "error.mission_missing"},
		{"written empty", "mission: \"\"\n", "error.mission_missing"},
		{"written blank", "mission: \"  \"\n", "error.mission_missing"},
		{"over two lines", "mission: |-\n  first\n  second\n", "error.mission_multiline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTeam(t, l, teamWith(tc.mission))
			if err == nil {
				t.Fatalf("a definition with %s did not stop the parse", tc.name)
			}
			// The error carries the file's path, the way the other reading
			// errors do (D2).
			want := i18n.Tf(tc.key, filepath.Join(l.team, "team.yaml"))
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not say %q", err, want)
			}
		})
	}
}

// The YAML reader must never panic, whatever the file holds. The seeds run with
// go test; the nightly workflow (go run ./tools/check nightly) runs them with
// -fuzz.
func FuzzRead(f *testing.F) {
	f.Add("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{name}\"\nmembers:\n  - name: a\n    role: r\n    profile: claude-default\n")
	f.Add("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}%{spawn}\"\nmembers:\n  - name: lead\n    role: r\n    profile: claude-default\n    may_spawn:\n      workers:\n        - purpose: 調査\n          profile: seeker\n")
	f.Add("version: 2\n")
	f.Add("members: [")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		l := newLayer(t, "")
		profiles := map[string]profile.Profile{
			"claude-default": {Name: "claude-default", Kind: "claude"},
			"seeker":         {Name: "seeker", Kind: "claude"},
		}
		got, err := parse(text, "t", "", l, profiles)
		if (got == nil) == (err == nil) {
			t.Fatalf("parse = %v, %v: exactly one of the two must be set", got, err)
		}
	})
}
