package check

import (
	"fmt"
	"os"
	"path/filepath"
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

// team writes the definition: members lead and spec, both on the claude profile.
// turns is written as it stands (a test writes turns: itself, or none).
func (f *fixture) team(members, turns string) {
	f.t.Helper()
	f.write(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a team for the test\nmembers:\n"+members+"turns:\n"+turns)
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
	want := Finding{Field: "mission", Text: i18n.T("validate.mission_placeholder")}
	if got[0] != want {
		t.Fatalf("finding = %+v, want %+v", got[0], want)
	}
}

// TestDefinitionStopsOnAGraphProblem: a definition whose graph cannot hold stops,
// with one finding per problem, and the finding names the field the problem is
// about. A definition that stops is not a warning list.
func TestDefinitionStopsOnAGraphProblem(t *testing.T) {
	f := newFixture(t)
	f.team(leadMember+specMember, "")
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
		if finding.Field != "turns:" {
			t.Errorf("finding %+v is not about the turns", finding)
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
	f.team(leadMember+specMember, "  - human -> lead\n  - lead -> human\n  - spec -> human\n")
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
