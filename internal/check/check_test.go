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
	return filepath.Join(f.proj, ".hergy", "roles", "lead.md")
}

// teamHead is the part of a team.yaml before its members: everything the
// fixture's definitions share.
const teamHead = "version: 1\ndescription: a team for the test\nmembers:\n"

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
	f.role("lead", "      - id: PAAF\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
}

// role writes one role document with the rules a test wants compared.
func (f *fixture) role(name, rules string) {
	f.t.Helper()
	f.roleText(name, rules)
}

// roleText writes one role document and returns its text, so a test that calls
// check.Role, which reads the document the screen hands it rather than the disk,
// can hand it what it wrote.
func (f *fixture) roleText(name, rules string) string {
	f.t.Helper()
	text := "version: 1\ntitle: " + name + "\nsections:\n  - heading: 仕事\n    rules:\n" + rules
	f.write(filepath.Join(f.proj, ".hergy", "roles", name+".md"), text)
	return text
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

// TestDefinitionWarnsAboutTheStarterDescription: a definition whose description is still
// the starter's example reads, and warns once. The finding names the description
// field, so a screen can put the sentence beside what the person writes.
func TestDefinitionWarnsAboutTheStarterDescription(t *testing.T) {
	f := newFixture(t)
	f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\ndescription: "+scaffold.DescriptionPlaceholder[i18n.Fallback]+"\nmembers:\n"+
			leadMember+specMember+"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: PAAF\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := f.findings()
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want the one description warning", got)
	}
	want := Finding{Field: "description", Text: i18n.T("validate.description_placeholder"), Kind: KindDescription}
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
	f.role("lead", "      - id: PAAF\n        text: 何かをする。\n")
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
	if strings.Contains(got[0].Text, "description") {
		t.Errorf("the break names the description: %q", got[0].Text)
	}
}

// TestDefinitionSaysWhenTheGlobalOneIsHidden: a name in both layers is read from
// the project's own; the global one is reported where a person reading it finds
// it (I11). The one way a name is looked up reports nothing.
func TestDefinitionSaysWhenTheGlobalOneIsHidden(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: PAAF\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	global := filepath.Join(f.global, "teams", "demo", "team.yaml")
	f.write(global, "version: 1\ndescription: a team for the test\nmembers:\n"+leadMember+specMember+
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

// TestDefinitionReportsEveryMemberThatCannotBeBuilt: a definition whose two
// members have no role document is reported whole, with a stop for each member
// and the definition's warnings beside them, so a screen shows the list at once
// rather than one more sentence per fixing (N1).
func TestDefinitionReportsEveryMemberThatCannotBeBuilt(t *testing.T) {
	f := newFixture(t)
	// The description is still the starter's example: a warning that must be there
	// even though a prompt cannot be built.
	f.write(f.teamPath(), "version: 1\ndescription: "+scaffold.DescriptionPlaceholder[i18n.Fallback]+
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
	if len(warnings) != 1 || warnings[0].Kind != KindDescription {
		t.Errorf("warnings = %+v, want the description warning with the two stops", warnings)
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
			name: "description, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"description"} },
		},
		{
			name: "team, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\nteam: another\ndescription: a team for the test\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"team"} },
		},
		{
			name: "prompt, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nprompt: \"\"\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"prompt"} },
		},
		{
			name: "docs, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\ndocs:\n  - old\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"docs"} },
		},
		{
			name: "roles, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nroles:\n  - old\nmembers:\n"+leadMember+specMember+teamTurns)
				f.roles()
			},
			want: func(*fixture) []string { return []string{"roles"} },
		},
		{
			name: "members, a top-level key",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nturns:\n  - human -> lead\n")
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
			name: "turns[i], the i-th line",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+leadMember+specMember+
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
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+leadMember+specMember+"turns:\n")
				f.roles()
			},
			want: func(*fixture) []string { return []string{"node:human", "node:lead", "node:spec"} },
		},
		{
			name: "the file as a whole",
			write: func(f *fixture) {
				f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers: 3\n")
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

// TestUnknownProfileIsAProblemNotABreak: a profile profiles.yaml does not hold
// no longer stops the file reading (spec-teams 1.9). The definition reads — a
// screen draws its figure, its description and its numbers — and the check names the
// profile at the member and at the worker that write it, as a problem that stops
// hergy from starting them (監査 R10).
func TestUnknownProfileIsAProblemNotABreak(t *testing.T) {
	f := newFixture(t)
	f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+
		"  - name: lead\n    role: lead\n    profile: absent\n"+
		specMember+
		"workers:\n  - from: spec\n    purpose: 調査\n    profile: gone\n"+
		"turns:\n  - human -> lead\n  - lead -> spec\n  - spec -> human\n  - spec -> lead\n")
	f.role("lead", "      - id: PAAF\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := f.findings()
	var named []Finding
	for _, finding := range got {
		if finding.Field == "members[0].profile" || finding.Field == "workers[0]" {
			named = append(named, finding)
		}
	}
	if len(named) != 2 {
		t.Fatalf("the check named %d unknown profiles, want the member's and the worker's:\n%+v", len(named), got)
	}
	for _, finding := range named {
		if !finding.Stops || finding.Kind != KindRead {
			t.Errorf("the finding at %s does not stop the definition: %+v", finding.Field, finding)
		}
	}
	if text := named[0].Text + named[1].Text; !strings.Contains(text, "absent") || !strings.Contains(text, "gone") {
		t.Errorf("the findings read %q, want the profiles' own names", text)
	}
	// The rest of the definition still checks: no break about the figure itself,
	// so a screen has a figure to draw and a person a name to fix.
	for _, finding := range got {
		if finding.Stops && finding.Field != "members[0].profile" && finding.Field != "workers[0]" {
			t.Errorf("the unknown profiles stopped something else too: %+v", finding)
		}
	}
}
