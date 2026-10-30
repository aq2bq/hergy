package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// descriptionTeam writes one local team definition whose team.yaml writes the given
// description line (empty for a definition that writes none), over the starter
// team's own files, so show, up --dry and validate read a definition. The
// starter files are written the first time only; every later call rewrites
// team.yaml alone.
func descriptionTeam(t *testing.T, f *fixture, description string) {
	t.Helper()
	starter := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	if _, err := os.Stat(starter); err != nil {
		mustRun(t, "new", "demo")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	body := "version: 1\n" + description +
		"members:\n  - name: lead\n    role: lead\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n"
	writeFile(t, path, body)
}

// TestShowPrintsTheDescriptionAndATeamCommandDoesNot: show is where a person
// reads one definition, so it prints the description; a team command's header
// shows a running team, and the description is not there (d-5f55, ac-29f3).
func TestShowPrintsTheDescriptionAndATeamCommandDoesNot(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	descriptionTeam(t, f, "description: hergy をよくする\n")

	show := mustRun(t, "show", "demo")
	if want := "description: hergy をよくする\n"; !strings.HasPrefix(show, want) {
		t.Errorf("show does not open with the description:\n%s", show)
	}
	if !strings.Contains(show, "team definition: demo\n") {
		t.Errorf("show does not carry the header after the description:\n%s", show)
	}

	plan := mustRun(t, "up", "demo", "--dry")
	if strings.Contains(plan, "hergy をよくする") {
		t.Errorf("up --dry prints the description:\n%s", plan)
	}
	// The plan names what it is before the team's own lines, and a team
	// command names the team; the definition is named only when the two
	// differ.
	if !strings.HasPrefix(plan, "mode: boot\nteam: demo\n") {
		t.Errorf("up --dry does not open with the plan and the team:\n%s", plan)
	}
}

// TestValidateStopsOnABadDescription: the description is required (I15), so a
// definition that writes none, writes an empty one, or writes one over more
// than one line does not read, and hergy validate reports the file broken. One
// line reads, and validate says nothing about the description.
func TestValidateStopsOnABadDescription(t *testing.T) {
	f := newFixture(t)
	initLocales(t)

	cases := []struct{ name, description, key string }{
		{"no description", "", "error.description_missing"},
		{"written empty", "description: \"\"\n", "error.description_missing"},
		{"written blank", "description: \"  \"\n", "error.description_missing"},
		{"over two lines", "description: |-\n  first\n  second\n", "error.description_multiline"},
		{"the old mission key", "mission: hergy をよくする\n", "error.mission_gone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			descriptionTeam(t, f, tc.description)
			stdout, _, code := run("validate")
			if code == 0 {
				t.Fatalf("validate accepted a definition with %s:\n%s", tc.name, stdout)
			}
			// The detail carries the file, the way hergy validate names a
			// broken file (D2).
			want := i18n.Tf(tc.key, ".hergy/teams/demo/team.yaml")
			if !strings.Contains(stdout, want) {
				t.Errorf("validate does not say %q:\n%s", want, stdout)
			}
			if tc.key == "error.mission_gone" && !strings.Contains(want, "description:") {
				t.Errorf("the refusal does not tell the person to write description:: %q", want)
			}
			if _, _, code := run("show", "demo"); code == 0 {
				t.Errorf("show read a definition with %s", tc.name)
			}
		})
	}

	// One line: the definition reads, validate is quiet, and show prints it.
	descriptionTeam(t, f, "description: one line is enough\n")
	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a written description is a break?\n%s", stdout)
	}
	for _, unwanted := range []string{
		i18n.Tf("error.description_missing", "team.yaml"),
		i18n.Tf("error.description_multiline", "team.yaml"),
		i18n.T("validate.description_placeholder"),
	} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("validate says %q about a description written in one line:\n%s", unwanted, stdout)
		}
	}
	if show := mustRun(t, "show", "demo"); !strings.HasPrefix(show, "description: one line is enough\n") {
		t.Errorf("show does not print the description:\n%s", show)
	}
}

// TestStatusNamesWhyARunningTeamsDefinitionDoesNotRead: a running team whose
// definition is here but does not read is not "in another project": status says
// why the file does not read, with its path (D1). A definition that is not here
// at all keeps the sentence that says so.
func TestStatusNamesWhyARunningTeamsDefinitionDoesNotRead(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// The definition stops reading: the description line is gone.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\n"+
			"members:\n  - name: lead\n    role: lead\n    profile: claude-default\n"+
			"  - name: builder\n    role: builder\n    profile: claude-default\n")

	got := mustRun(t, "status")
	if strings.Contains(got, i18n.T("status.definition_elsewhere")) {
		t.Errorf("status says a definition that is here is in another project:\n%s", got)
	}
	want := i18n.Tf("error.description_missing", ".hergy/teams/demo/team.yaml")
	if !strings.Contains(got, want) {
		t.Errorf("status does not say why the file does not read (%q):\n%s", want, got)
	}
}

// TestStatusSaysWhenTheDefinitionIsNotHere: a running team whose definition is
// gone from this project keeps the sentence that says so (D1's other half).
func TestStatusSaysWhenTheDefinitionIsNotHere(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	if err := os.RemoveAll(filepath.Join(f.proj, ".hergy", "teams", "demo")); err != nil {
		t.Fatal(err)
	}
	got := mustRun(t, "status")
	if !strings.Contains(got, i18n.T("status.definition_elsewhere")) {
		t.Errorf("status does not say the definition is not in this project:\n%s", got)
	}
}

// TestValidateWarnsAboutTheStarterDescription: the starter team's description is an
// example, so a definition that still writes it reads and boots, and hergy
// validate warns that its team has not said what it is for (d-3cae). The
// warning moves no exit code, and rewriting the line clears it.
func TestValidateWarnsAboutTheStarterDescription(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")

	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("the starter team is a break?\n%s", stdout)
	}
	if want := "  warning: " + i18n.T("validate.description_placeholder") + "\n"; !strings.Contains(stdout, want) {
		t.Errorf("validate does not warn about the starter description:\n%s", stdout)
	}

	// A description of the definition's own: the warning is gone.
	descriptionTeam(t, f, "description: hergy をよくする\n")
	stdout, _, code = run("validate")
	if code != 0 {
		t.Fatalf("a written description is a break?\n%s", stdout)
	}
	if strings.Contains(stdout, i18n.T("validate.description_placeholder")) {
		t.Errorf("validate warns about the starter description after it was rewritten:\n%s", stdout)
	}
}

// TestADescriptionChangeLeavesThePromptAndTheDriftAlone: the description is a
// note for the person choosing a definition, not part of what a team does
// (d-5f55, ac-0c4b). Two definitions whose descriptions differ build the same
// prompts byte for byte, and a definition whose description alone changed has
// nothing for hergy apply to put right.
func TestADescriptionChangeLeavesThePromptAndTheDriftAlone(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	before, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}

	// What apply answers before the description moves is the line the after
	// must not cross: the plan names every member and how hergy would put it
	// right, so a description that made a drift would change this text.
	planned := mustRun(t, "apply", "--dry")

	// Only the description changes, in the running team's own definition.
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	text := readFile(t, path)
	writeFile(t, path, strings.Replace(text, "description: ", "description: 書き直した ", 1))
	after, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if after.team.Definition.Description == before.team.Definition.Description {
		t.Fatalf("the description did not change: %q", after.team.Definition.Description)
	}
	for i := range before.starts {
		if before.starts[i].Prompt != after.starts[i].Prompt {
			t.Errorf("%s's prompt changed with the description:\n--- before ---\n%s\n--- after ---\n%s",
				before.starts[i].Name, before.starts[i].Prompt, after.starts[i].Prompt)
		}
	}

	if out := mustRun(t, "apply", "--dry"); out != planned {
		t.Errorf("apply --dry changed with only the description:\n--- before ---\n%s\n--- after ---\n%s", planned, out)
	}
}

// TestTwoDescriptionsBuildTheSamePrompts: the description is not in a member's
// or a worker's prompt (ac-0c4b). The prompts are the whole of what a member is
// handed, so they are compared byte for byte.
func TestTwoDescriptionsBuildTheSamePrompts(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	starter := readFile(t, path)
	writeFile(t, path, strings.Replace(starter, "description: このチームが想定するケースを 1 行で",
		"description: 注文の流れを作り直す", 1))
	first := mustRun(t, "up", "demo", "--dry")
	writeFile(t, path, strings.Replace(starter, "description: このチームが想定するケースを 1 行で",
		"description: 見積もりの計算をそろえる", 1))
	second := mustRun(t, "up", "demo", "--dry")
	if first != second {
		t.Errorf("two descriptions built different plans:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if strings.Contains(first, "注文の流れを作り直す") || strings.Contains(second, "見積もりの計算をそろえる") {
		t.Errorf("the description is in the plan:\n%s\n%s", first, second)
	}
}
