package check

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/scaffold"
	"github.com/aq2bq/hergy/internal/source"
)

// TestMain loads the messages, so the sentences a test reads are the ones a
// person sees.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// fixture is a small project a check reads: one definition named demo, the
// profiles it runs, and the role documents it names. It is written to a
// temporary directory, so a test never reads the person's own hergy settings.
type fixture struct {
	t      *testing.T
	proj   string
	global string
	cat    *catalog.Catalog
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	local := filepath.Join(proj, ".hergy")
	f := &fixture{t: t, proj: proj, global: filepath.Join(root, "global")}
	f.write(filepath.Join(local, "profiles.yaml"),
		"version: 1\nprofiles:\n  claude:\n    kind: claude\n")
	f.cat = catalog.New(f.global, local, proj, source.Disk{},
		func(path string) string { return path })
	return f
}

// write writes one file, making the directories it needs.
func (f *fixture) write(path, text string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// teamPath is where the fixture's definition sits.
func (f *fixture) teamPath() string {
	return filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
}

// leadRolePath is where the fixture's lead role document sits.
func (f *fixture) leadRolePath() string {
	return filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
}

// teamHead is the part of a team.yaml before its members: everything the
// fixture's definitions share.
const teamHead = "version: 1\nmission: a team for the test\nmembers:\n"

// team writes the definition: members lead and spec, both on the claude profile.
// turns is written as it stands (a test writes turns: itself, or none).
func (f *fixture) team(turns string) {
	f.t.Helper()
	f.write(f.teamPath(), teamHead+leadMember+specMember+"turns:\n"+turns)
}

// roles writes a harmless role document for every member of the fixture, so a
// test that is about something else does not stop on a prompt.
func (f *fixture) roles() {
	f.t.Helper()
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
}

// role writes one role document with the rules a test wants compared.
func (f *fixture) role(name, rules string) {
	f.t.Helper()
	f.write(filepath.Join(f.proj, ".hergy", "roles", name+".yaml"),
		"version: 1\ntitle: "+name+"\nsections:\n  - heading: 仕事\n    style: ids\n    rules:\n"+rules)
}

// findings is the definition's findings, by the one way a name is looked up.
func (f *fixture) findings() []Finding {
	f.t.Helper()
	return Definition(Input{Catalog: f.cat, Name: "demo"})
}

const (
	leadMember = "  - name: lead\n    role: lead\n    profile: claude\n"
	specMember = "  - name: spec\n    role: spec\n    profile: claude\n"
)

// TestDefinitionWarnsAboutTheStarterMission: a definition whose mission is still
// the starter's example reads, and warns once. The finding names the mission
// field, so a screen can put the sentence beside what the person writes.
func TestDefinitionWarnsAboutTheStarterMission(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: "+scaffold.MissionPlaceholder[i18n.Fallback]+"\nmembers:\n"+
			leadMember+specMember+"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := f.findings()
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want the one mission warning", got)
	}
	want := Finding{Field: "mission", Text: i18n.T("validate.mission_placeholder"), Kind: KindMission}
	if got[0] != want {
		t.Fatalf("finding = %+v, want %+v", got[0], want)
	}
}

// TestDefinitionStopsOnAGraphProblem: a definition whose graph cannot hold stops,
// with one finding per problem, and the finding names the field the problem is
// about. A definition that stops is not a warning list.
func TestDefinitionStopsOnAGraphProblem(t *testing.T) {
	f := newFixture(t)
	f.team("")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := f.findings()
	if len(got) == 0 {
		t.Fatal("findings = none, want the graph problems")
	}
	for _, finding := range got {
		if !finding.Stops {
			t.Errorf("finding %+v warns; a graph problem stops the definition", finding)
		}
		if finding.Kind != KindGraph {
			t.Errorf("finding %+v is not a graph problem", finding)
		}
		if !strings.HasPrefix(finding.Field, "node:") {
			t.Errorf("finding %+v is not about a node", finding)
		}
		if finding.Text == "" {
			t.Errorf("finding %+v has no sentence", finding)
		}
	}
	if strings.Contains(got[0].Text, "mission") {
		t.Errorf("the break names the mission: %q", got[0].Text)
	}
}

// TestTurnFindings: a rule text that writes a turn the graph does not have warns
// (段 5); a name the graph has no node for warns too, and says which name (spec/33
// H2); a worker's purpose is not such a name, because workers: ties the worker to
// its member (spec/33 の 3).
func TestTurnFindings(t *testing.T) {
	f := newFixture(t)
	// lead may hire a worker whose purpose is 実装: workers: ties that worker to
	// lead, so the arrow LEAD-3 writes is not a name without a node.
	f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a team for the test\nmembers:\n"+leadMember+specMember+
			"workers:\n  - from: lead\n    purpose: 実装\n    profile: claude\nturns:\n"+
			"  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", strings.Join([]string{
		"      - id: LEAD-1\n        text: 設計は spec に渡す（lead → spec）。\n",
		"      - id: LEAD-2\n        text: 実装は impl に渡す（lead → impl）。\n",
		"      - id: LEAD-3\n        text: 実装はワーカーに払い出す（lead → 実装）。\n",
	}, ""))
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := f.findings()
	var texts []string
	for _, finding := range got {
		if finding.Stops {
			t.Fatalf("the definition stopped: %+v", finding)
		}
		if finding.Kind != KindTurn {
			t.Errorf("finding %+v is not about a turn", finding)
		}
		if want := "rule:" + f.leadRolePath() + "#"; !strings.HasPrefix(finding.Field, want) {
			t.Errorf("finding %+v does not name the rule it is about (want %s...)", finding, want)
		}
		texts = append(texts, finding.Text)
	}
	joined := strings.Join(texts, "\n")
	if want := i18n.Tf("validate.turn_not_in_turns", "LEAD-1", "lead → spec"); !strings.Contains(joined, want) {
		t.Errorf("no warning that turns: lacks lead → spec:\n%s", joined)
	}
	if want := i18n.Tf("validate.turn_unknown_name", "LEAD-2", "lead → impl", "impl"); !strings.Contains(joined, want) {
		t.Errorf("no warning for the name impl:\n%s", joined)
	}
	if strings.Contains(joined, "実装") {
		t.Errorf("a worker's purpose warned:\n%s", joined)
	}
}

// TestDefinitionSaysWhenTheGlobalOneIsHidden: a name in both layers is read from
// the project's own; the global one is reported where a person reading it finds
// it (I11). The one way a name is looked up reports nothing.
func TestDefinitionSaysWhenTheGlobalOneIsHidden(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	global := filepath.Join(f.global, "teams", "demo", "team.yaml")
	f.write(global, "version: 1\nmission: a team for the test\nmembers:\n"+leadMember+specMember+
		"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")

	got := Definition(Input{Catalog: f.cat, Name: "demo", Layer: source.LayerGlobal})
	var joined strings.Builder
	for _, finding := range got {
		joined.WriteString(finding.Text + "\n")
	}
	if want := i18n.Tf("validate.global_hidden", "demo"); !strings.Contains(joined.String(), want) {
		t.Errorf("the global layer does not say it is hidden:\n%s", joined.String())
	}
	if got := f.findings(); len(got) != 0 {
		t.Errorf("the one way reports the hidden warning: %+v", got)
	}
}

// TestTheHumanIsReadInBothWords: a rule's text writes the human the way its own
// language does, and hergy reads either word in either language, because a rule
// is written in one language while turns: names the human the same way in both
// (S1). A Japanese rule that writes "human" is a turn to the human, and a rule
// that writes it where turns: has no such edge is warned about in the same
// words.
func TestTheHumanIsReadInBothWords(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"a Japanese text writes the English word", "設計は human に聞く（spec → human）。"},
		{"an English text writes the Japanese word", "the design asks 人間 (spec → 人間)."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
			f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
			f.role("spec", "      - id: SPEC-1\n        text: "+c.text+"\n")

			got := f.findings()
			for _, finding := range got {
				if finding.Stops {
					t.Fatalf("the definition stopped: %+v", finding)
				}
				if strings.Contains(finding.Text, "human") || strings.Contains(finding.Text, "人間") {
					t.Errorf("the human was read as an unknown name: %s", finding.Text)
				}
			}
		})

		t.Run(c.name+", and turns: does not have the edge", func(t *testing.T) {
			f := newFixture(t)
			// spec hands the turn to lead rather than to the human, so the graph
			// holds while the rule's turn is one turns: does not have.
			f.team("  - human -> lead\n  - lead -> human\n  - spec -> lead\n")
			f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
			f.role("spec", "      - id: SPEC-1\n        text: "+c.text+"\n")

			got := f.findings()
			var joined strings.Builder
			for _, finding := range got {
				if finding.Stops {
					t.Fatalf("the definition stopped: %+v", finding)
				}
				joined.WriteString(finding.Text + "\n")
			}
			if want := i18n.Tf("validate.turn_not_in_turns", "SPEC-1", "spec → human"); !strings.Contains(joined.String(), want) {
				t.Errorf("no warning that turns: lacks spec → human:\n%s", joined.String())
			}
		})
	}
}

// TestDefinitionReportsEveryMemberThatCannotBeBuilt: a definition whose two
// members have no role document is reported whole, with a stop for each member
// and the definition's warnings beside them, so a screen shows the list at once
// rather than one more sentence per fixing (N1).
func TestDefinitionReportsEveryMemberThatCannotBeBuilt(t *testing.T) {
	f := newFixture(t)
	// The mission is still the starter's example: a warning that must be there
	// even though a prompt cannot be built.
	f.write(f.teamPath(), "version: 1\nmission: "+scaffold.MissionPlaceholder[i18n.Fallback]+
		"\nmembers:\n"+leadMember+specMember+"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")

	got := f.findings()
	var stops, warnings []Finding
	for _, finding := range got {
		if finding.Stops {
			stops = append(stops, finding)
			continue
		}
		warnings = append(warnings, finding)
	}
	if len(stops) != 2 {
		t.Fatalf("stops = %+v, want one for each member", stops)
	}
	if stops[0].Field != "members[0].role" || stops[1].Field != "members[1].role" {
		t.Errorf("fields = %q, %q, want members[0].role and members[1].role", stops[0].Field, stops[1].Field)
	}
	if stops[0].Kind != KindPrompt {
		t.Errorf("kind = %v, want a prompt that cannot be built", stops[0].Kind)
	}
	if len(warnings) != 1 || warnings[0].Kind != KindMission {
		t.Errorf("warnings = %+v, want the mission warning with the two stops", warnings)
	}
}

// TestFieldKeysReadTheSameInBothLanguages: the field a sentence belongs beside
// is one key per case, written the same way whatever language the sentence is in
// (N2). The cases are the ones check.Finding.Field writes.
func TestFieldKeysReadTheSameInBothLanguages(t *testing.T) {
	teamTurns := "turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n"
	reads := teamHead + leadMember + specMember + teamTurns

	cases := []struct {
		name  string
		write func(f *fixture)
		want  func(f *fixture) []string
	}{
		{
			name: "mission, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"mission"} },
		},
		{
			name: "team, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nteam: another\nmission: a team for the test\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"team"} },
		},
		{
			name: "prompt, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nprompt: \"\"\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"prompt"} },
		},
		{
			name: "docs, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\ndocs:\n  - old\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"docs"} },
		},
		{
			name: "roles, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nroles:\n  - old\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"roles"} },
		},
		{
			name: "members, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nturns:\n  - human -> lead\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members"} },
		},
		{
			name: "members[i].name",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead%{team}\n    role: lead\n    profile: claude\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0].name"} },
		},
		{
			name: "members[i].role",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead\n    role: \"\"\n    profile: claude\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0].role"} },
		},
		{
			name: "members[i].profile",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead\n    role: lead\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0].profile"} },
		},
		{
			name: "members[i].prompt",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead\n    role: lead\n    profile: claude\n    prompt: \"\"\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0].prompt"} },
		},
		{
			name: "members[i].kind, the engine the member writes",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead\n    role: lead\n    profile: claude\n    kind: claude\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0].kind"} },
		},
		{
			name: "members[i], the member as a whole (may_spawn)",
			write: func(f *fixture) {
				f.write(f.teamPath(), teamHead+
					"  - name: lead\n    role: lead\n    profile: claude\n    may_spawn: []\n"+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"members[0]"} },
		},
		{
			name: "members[i], the member's documents together (one rule id twice)",
			write: func(f *fixture) {
				f.write(f.teamPath(), reads)
				f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
				f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
				f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"),
					"version: 1\ntitle: agreement\nsections:\n  - heading: 約束\n    style: ids\n    rules:\n"+
						"      - id: SPEC-1\n        text: 何かをする。\n")
			},
			want: func(*fixture) []string { return []string{"members[1]"} },
		},
		{
			name: "turns[i], the i-th line",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nmembers:\n"+leadMember+specMember+
					"turns:\n  - human -> lead\n  - nobody talks\n  - spec -> human\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{"turns[1]"} },
		},
		{
			name: "workers[i].from",
			write: func(f *fixture) {
				f.write(f.teamPath(), reads+"workers:\n  - from: \"\"\n    purpose: 実装\n    profile: claude\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{"workers[0].from"} },
		},
		{
			name: "node:human and node:<member>",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nmembers:\n"+leadMember+specMember+"turns:\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{"node:human", "node:lead", "node:spec"} },
		},
		{
			name: "rule:<document path>#<rule id>",
			write: func(f *fixture) {
				f.write(f.teamPath(), reads)
				f.role("lead", "      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n")
				f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
			},
			want: func(f *fixture) []string { return []string{"rule:" + f.leadRolePath() + "#LEAD-1"} },
		},
		{
			name: "the file as a whole",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmission: a team for the test\nmembers: 3\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{""} },
		},
	}

	for _, c := range cases {
		for _, lang := range []string{"en", "ja"} {
			t.Run(c.name+" ("+lang+")", func(t *testing.T) {
				f := newFixture(t)
				c.write(f)
				if err := i18n.Use(lang); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
				got := fields(f.findings())
				want := slices.Clone(c.want(f))
				slices.Sort(want)
				want = slices.Compact(want)
				if !slices.Equal(got, want) {
					t.Errorf("fields = %v, want %v", got, want)
				}
			})
		}
	}
}

// fields is the fields the findings name, each once, in order, for a table that
// compares what a case gives in two languages.
func fields(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Field)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// stopFields is the fields of the findings that stop the definition.
func stopFields(findings []Finding) []string {
	var out []string
	for _, finding := range findings {
		if finding.Stops {
			out = append(out, finding.Field)
		}
	}
	return out
}

// TestADraftRuleIdIsCheckedAgainstTheAgreement: the check builds every prompt and
// reads every rule document through the catalog, so a role document's draft (C3b,
// With) that carries an id the team's agreement already has stops the definition
// before it is saved, while the same files on disk are accepted (N1). The draft
// is not written: the file keeps the text it had.
func TestADraftRuleIdIsCheckedAgainstTheAgreement(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	// The agreement holds an id the draft of the lead's role document takes.
	f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"),
		"version: 1\ntitle: agreement\nsections:\n  - heading: 約束\n    style: ids\n    rules:\n"+
			"      - id: DUP-1\n        text: 何かをする。\n")

	if got := stopFields(f.findings()); len(got) != 0 {
		t.Fatalf("the definition stops on the files as they are: %v", got)
	}

	draft := "version: 1\ntitle: lead\nsections:\n  - heading: 仕事\n    style: ids\n    rules:\n" +
		"      - id: LEAD-1\n        text: 何かをする。\n" +
		"      - id: DUP-1\n        text: 書きかけの規則。\n"
	got := Definition(Input{Catalog: f.cat.With(f.leadRolePath(), draft), Name: "demo"})
	var stops []Finding
	for _, finding := range got {
		if finding.Stops {
			stops = append(stops, finding)
		}
	}
	if len(stops) != 1 {
		t.Fatalf("stops = %+v, want the one member whose draft holds the id twice", stops)
	}
	if stops[0].Field != "members[0]" || stops[0].Kind != KindPrompt {
		t.Errorf("stop = %+v, want the member whose documents hold the id twice", stops[0])
	}
	if !strings.Contains(stops[0].Text, "DUP-1") {
		t.Errorf("the stop does not name the id that stands twice: %s", stops[0].Text)
	}

	onDisk, err := os.ReadFile(f.leadRolePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "DUP-1") {
		t.Errorf("the check wrote the draft to disk:\n%s", onDisk)
	}
}

// TestATurnWarningReadsTheDraft: the comparison of a rule text's arrows reads the
// draft too (N1), so a rule the screen is editing, which writes a turn the graph
// does not have, warns before it is saved. The same file on disk writes a turn
// turns: has, so it warns about nothing, and it is left as it was.
func TestATurnWarningReadsTheDraft(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 設計は spec の話を聞く（spec → human）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	for _, finding := range f.findings() {
		if finding.Kind == KindTurn {
			t.Fatalf("the file on disk warns about a turn: %+v", finding)
		}
	}

	draft := "version: 1\ntitle: lead\nsections:\n  - heading: 仕事\n    style: ids\n    rules:\n" +
		"      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n"
	got := Definition(Input{Catalog: f.cat.With(f.leadRolePath(), draft), Name: "demo"})
	var warned int
	for _, finding := range got {
		if finding.Kind != KindTurn {
			continue
		}
		warned++
		if want := "rule:" + f.leadRolePath() + "#LEAD-1"; finding.Field != want {
			t.Errorf("the warning names %q, want %q", finding.Field, want)
		}
		if want := i18n.Tf("validate.turn_unknown_name", "LEAD-1", "lead → impl", "impl"); finding.Text != want {
			t.Errorf("the warning reads %q, want %q", finding.Text, want)
		}
	}
	if warned != 1 {
		t.Errorf("the draft warned %d times about a turn, want the one arrow it writes", warned)
	}

	onDisk, err := os.ReadFile(f.leadRolePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "spec → human") {
		t.Errorf("the check wrote the draft to disk:\n%s", onDisk)
	}
}

// TestTurnWarningNamesItsRule: a warning about a rule's text names the document
// and the rule, so a screen puts the sentence on that rule's slip (N3). A rule
// of the role document and one of the team's agreement each name their own
// document.
func TestTurnWarningNamesItsRule(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	agreement := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	f.write(agreement, "version: 1\ntitle: agreement\nsections:\n  - heading: 約束\n    style: ids\n    rules:\n"+
		"      - id: AGREE-1\n        text: 設計は spec に渡す（lead → spec）。\n")

	got := f.findings()
	want := map[string]string{
		"LEAD-1":  "rule:" + f.leadRolePath() + "#LEAD-1",
		"AGREE-1": "rule:" + agreement + "#AGREE-1",
	}
	seen := map[string]bool{}
	for _, finding := range got {
		if finding.Stops {
			t.Fatalf("the definition stopped: %+v", finding)
		}
		if finding.Kind != KindTurn {
			continue
		}
		seen[finding.Field] = true
	}
	for id, key := range want {
		if !seen[key] {
			t.Errorf("no warning names the rule %s of its own document (%s): %v", id, key, seen)
		}
	}
}
