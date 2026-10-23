package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// This file holds the small checks of the team screen that are about the model
// rather than a browser: the effect one change has on a member's own prompt
// (TEAMS-M2) and the rule Markdown a screen previews against the prompt a member
// is handed (TEAMS-M4). The fixture is the team screen's real-scale one
// (testdata/teams-v2), the same one the screen's own tests and the e2e walks
// read.
//
// Every helper here is this file's own.

// teamsV2Catalog copies the real-scale teams fixture into a working project and
// a settings directory, and answers the catalog both checks read it through. The
// tests here only read, but the copy keeps the machine's own settings out of the
// run all the same.
func teamsV2Catalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "teams-v2")
	root := t.TempDir()
	proj := filepath.Join(root, "project")
	global := filepath.Join(root, "config", "hergy")
	teamsV2Copy(t, filepath.Join(src, "project"), proj)
	teamsV2Copy(t, filepath.Join(src, "config", "hergy"), global)
	return catalog.New(global, filepath.Join(proj, ".hergy"), proj, source.Disk{},
		func(path string) string { return path })
}

// teamsV2Copy copies a directory as it stands.
func teamsV2Copy(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTEAMS_M2_TheEffectIsThePromptsOwnLine: the effect of taking the turn
// impl-b -> reviewer away from core is one line of impl-b's own prompt
// ("ターンを渡す相手"), written the way that prompt writes it, with the problem
// the change would add — the rule CORE-12 still writing the turn. Taking away
// the last turn a member hands out shows both the line that goes and the
// sentence that stops the definition (TEAMS-M2).
func TestTEAMS_M2_TheEffectIsThePromptsOwnLine(t *testing.T) {
	cat := teamsV2Catalog(t)
	in := TeamInput{Catalog: cat, Name: "core", Layer: source.LayerLocal}

	// One turn away: impl-b's own line changes, and CORE-12, which writes the
	// turn, warns.
	got := TeamEffect(in, definition.RemoveTurn(memberNode("impl-b"), memberNode("reviewer")))
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %+v, want the one line of impl-b", got.Lines)
	}
	line := got.Lines[0]
	if line.Member != "core-impl-b" || line.Part != "turns" {
		t.Errorf("line = %+v, want core-impl-b's turns", line)
	}
	wantRemoved := []string{i18n.Tf("turns.member_item", "core-reviewer", "reviewer")}
	if !slices.Equal(line.Removed, wantRemoved) {
		t.Errorf("removed = %q, want %q", line.Removed, wantRemoved)
	}
	if len(line.Added) != 0 {
		t.Errorf("added = %q, want nothing", line.Added)
	}
	warn := i18n.Tf("validate.turn_not_in_turns", "CORE-12", "impl-b → reviewer")
	found := false
	for _, problem := range got.Problems {
		if problem.Text == warn {
			found = true
		}
	}
	if !found {
		t.Errorf("problems = %+v, want the turn warning %q", got.Problems, warn)
	}

	// The removed line is the very line the member's own prompt writes: the
	// short "ターンを渡す相手" note the prompt carries, built from the same graph,
	// names reviewer with the same words.
	def, err := cat.Definition("core", source.LayerLocal)
	if err != nil {
		t.Fatalf("core does not read: %v", err)
	}
	note := prompt.TurnsNote(i18n.Language(), prompt.TurnsOf(def, "impl-b", "core"))
	if !strings.Contains(note, wantRemoved[0]) {
		t.Errorf("impl-b's turn note does not hold %q:\n%s", wantRemoved[0], note)
	}

	// The last turn a member hands out: analyst keeps no turn, so the line that
	// goes and the sentence that stops the definition both stand in the answer
	// (the earlier two turns are the ops already applied).
	ops := []definition.Op{
		definition.RemoveTurn(memberNode("analyst"), memberNode("lead")),
		definition.RemoveTurn(memberNode("analyst"), memberNode("impl-a")),
	}
	last := TeamEffect(TeamInput{Catalog: cat, Name: "core", Layer: source.LayerLocal, Ops: ops},
		definition.RemoveTurn(memberNode("analyst"), memberNode("impl-b")))
	if len(last.Lines) != 1 {
		t.Fatalf("lines = %+v, want the one line of analyst", last.Lines)
	}
	gone := last.Lines[0]
	if gone.Member != "core-analyst" || gone.Part != "turns" {
		t.Errorf("line = %+v, want core-analyst's turns", gone)
	}
	if want := i18n.Tf("turns.member_item", "core-impl-b", "builder"); !slices.Contains(gone.Removed, want) {
		t.Errorf("removed = %q, want %q among them", gone.Removed, want)
	}
	if len(gone.Added) != 0 {
		t.Errorf("added = %q, want nothing", gone.Added)
	}
	stop := i18n.Tf("error.graph_member_no_turn", "analyst", "analyst", "analyst", "lead")
	stopped := false
	for _, problem := range last.Problems {
		if strings.Contains(problem.Text, stop) {
			stopped = true
		}
	}
	if !stopped {
		t.Errorf("problems = %+v, want the sentence %q", last.Problems, stop)
	}
}

// TestTEAMS_M4_ThePreviewIsThePromptsAgreementPart: the Markdown a screen
// previews for core's own agreement is the very part the member's initial prompt
// carries, one character not differing, and CORE-8 — the one rule kept out of
// the prompt — is a struck line with no line number, inside neither the numbered
// preview nor the prompt (TEAMS-M4).
func TestTEAMS_M4_ThePreviewIsThePromptsAgreementPart(t *testing.T) {
	cat := teamsV2Catalog(t)
	def, err := cat.Definition("core", source.LayerLocal)
	if err != nil {
		t.Fatalf("core does not read: %v", err)
	}

	// The team's own agreement, parsed the way a screen's preview parses it.
	path := cat.TeamYAML("core", source.LayerLocal)
	agreePath := filepath.Join(filepath.Dir(path), "agreement.yaml")
	text, err := cat.Reader().Read(source.Doc{Path: agreePath})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := rules.Parse(text, cat.Display(agreePath))
	if err != nil {
		t.Fatal(err)
	}

	// The numbered preview lines, joined, are exactly the Markdown a member
	// reads: an excluded rule carries no line here.
	preview := doc.PreviewLines()
	var numbered []string
	excluded := 0
	for _, line := range preview {
		if line.Part == rules.PartExcluded {
			excluded++
			if !strings.HasPrefix(line.Text, "CORE-8 ") {
				t.Errorf("the excluded line reads %q, want CORE-8's own", line.Text)
			}
			continue
		}
		numbered = append(numbered, line.Text)
	}
	if excluded != 1 {
		t.Errorf("the preview draws %d excluded lines, want CORE-8's one", excluded)
	}
	region := strings.Join(numbered, "\n")
	if region != strings.TrimRight(doc.Markdown(), "\n") {
		t.Errorf("the numbered preview is not the Markdown a member reads:\n--- preview ---\n%s\n--- markdown ---\n%s", region, doc.Markdown())
	}
	if strings.Contains(region, "CORE-8") || strings.Contains(doc.Markdown(), "CORE-8") {
		t.Error("CORE-8 stands in the text a member reads, and it is kept out of the prompt")
	}

	// The prompt of a member of core carries the preview's lines verbatim. core's
	// lead reads the shared agreement and the team's own; the team's own is the
	// part compared here.
	var lead *definition.MemberNode
	for i := range def.Graph.Members {
		if def.Graph.Members[i].Name == "lead" {
			lead = &def.Graph.Members[i]
		}
	}
	if lead == nil {
		t.Fatal("core has no lead to build a prompt for")
	}
	built, err := prompt.Member(def, lead, prompt.Run{
		Team: "core", Base: cat.Base(), Reader: cat.Reader(), Sources: cat, Display: cat.Display,
	})
	if err != nil {
		t.Fatalf("lead's prompt did not build: %v", err)
	}
	if !strings.Contains(built.Text, "\n\n"+region+"\n") {
		t.Errorf("the prompt does not carry the agreement's preview verbatim:\n--- preview ---\n%s\n--- prompt ---\n%s", region, built.Text)
	}
	if strings.Contains(built.Text, "CORE-8") {
		t.Error("the prompt carries the rule kept out of it")
	}
}
