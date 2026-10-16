package app

import (
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/web"
)

// TestBumpRaisesTheVersionAndKeepsTheDocument: raising a version writes the
// document with its version one higher and nothing else lost, and the next launch
// reads the raised version as the change being handed over.
func TestBumpRaisesTheVersionAndKeepsTheDocument(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	// The document moves after the launch, which is what a version is raised for.
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one changed\n")

	port := recordPort(t)
	answer, err := port.Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
	if !answer.Written {
		t.Fatalf("the bump wrote nothing: %+v", answer)
	}
	bumped := answer.Raised
	if bumped.From != 1 || bumped.To != 2 {
		t.Errorf("the bump went %d → %d, want 1 → 2", bumped.From, bumped.To)
	}
	if bumped.Layer != web.LayerLocal {
		t.Errorf("the bump says the document is in the %q layer, want the local one", bumped.Layer)
	}
	if bumped.Definition != "demo" {
		t.Errorf("the bump names %q to run next, want the definition whose launch read it", bumped.Definition)
	}
	if bumped.File != protocol {
		t.Errorf("the bump wrote %q, want %q", bumped.File, protocol)
	}

	raised, err := rules.Parse(readRule(t, protocol), protocol)
	if err != nil {
		t.Fatalf("the file the bump wrote does not read back: %v", err)
	}
	if raised.Revision == nil || *raised.Revision != 2 {
		t.Errorf("the file writes revision %v, want 2", raised.Revision)
	}
	if ids := idsOfRules(raised); !equalIDs(ids, "P1") {
		t.Errorf("the file the bump wrote holds %v, want the rule P1", ids)
	}

	// The launch read version 1 and the file is at 2 now, so the home says the
	// next launch hands the change over.
	if doc := docAt(t, port.Home(web.LayerLocal), protocol); !doc.Bumped {
		t.Error("the home does not see the raised version as handed over by the next launch")
	}
}

// TestBumpWritesTheFirstVersion: a document that writes no version is at 0, so
// raising it writes 1.
func TestBumpWritesTheFirstVersion(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nsections:\n  - rules:\n      - id: P1\n        text: one\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	writeFile(t, protocol, "version: 1\nsections:\n  - rules:\n      - id: P1\n        text: two\n")

	answer, err := recordPort(t).Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
	if !answer.Written {
		t.Fatalf("the bump wrote nothing: %+v", answer)
	}
	if answer.Raised.From != 0 || answer.Raised.To != 1 {
		t.Errorf("the bump went %d → %d, want 0 → 1", answer.Raised.From, answer.Raised.To)
	}
	raised, err := rules.Parse(readRule(t, protocol), protocol)
	if err != nil {
		t.Fatalf("the file the bump wrote does not read back: %v", err)
	}
	if raised.Revision == nil || *raised.Revision != 1 {
		t.Errorf("the file writes revision %v, want 1", raised.Revision)
	}
}

// TestBumpRefusesADocumentThatHasNotMoved: the screen lists the documents whose
// content moved since the last launch, and a POST cannot raise one that did not:
// the two texts are the same, so there is no change to hand over.
func TestBumpRefusesADocumentThatHasNotMoved(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: one\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	before := readRule(t, protocol)

	answer, err := recordPort(t).Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
	if answer.Written || answer.Why == "" {
		t.Fatalf("the bump wrote %+v, want a refusal with a reason", answer)
	}
	if after := readRule(t, protocol); after != before {
		t.Errorf("the refused bump changed the file:\n%s", after)
	}
}

// TestBumpRefusesASecondRaiseFromAStalePage: the page the person still has open
// carries the version the document held before the first write. Posting it again
// (a resend, a second press, the browser's back button) raises nothing: the
// version is ahead of the one the launch that read the document read.
func TestBumpRefusesASecondRaiseFromAStalePage(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: one\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: one changed\n")

	port := recordPort(t)
	if answer, err := port.Bump(web.LayerLocal, protocol); err != nil || !answer.Written {
		t.Fatalf("the first bump wrote %+v (%v), want the version raised", answer, err)
	}
	afterFirst := readRule(t, protocol)

	answer, err := port.Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version again: %v", err)
	}
	if answer.Written || answer.Why == "" {
		t.Fatalf("the second bump wrote %+v, want a refusal with a reason", answer)
	}
	if after := readRule(t, protocol); after != afterFirst {
		t.Errorf("the refused second bump changed the file:\n%s", after)
	}
	// The screen says the document's version is already raised, and offers no
	// button for it.
	bumps := port.Home(web.LayerLocal).Bumps()
	if len(bumps) != 1 || !bumps[0].Already {
		t.Errorf("the screen offers %+v, want the document marked as already raised", bumps)
	}
}

// TestBumpRefusesADocumentTheRecordDoesNotHold: a document a member reads now but
// no launch of the layer ever read cannot be compared, so no version is raised in
// it: the record is the only thing that says a document moved.
func TestBumpRefusesADocumentTheRecordDoesNotHold(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	dir := filepath.Join(f.proj, ".hergy", "teams", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// The definition gains a document after the launch: the launch read nothing
	// of it.
	added := filepath.Join(dir, "added.yaml")
	writeFile(t, added, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: A1\n        text: one\n")
	writeFile(t, filepath.Join(dir, "team.yaml"), `version: 1
docs: [protocol.yaml, added.yaml]
members:
  - name: lead
    role: lead
    profile: claude-default
  - name: builder
    role: builder
    profile: claude-default
`)
	before := readRule(t, added)

	answer, err := recordPort(t).Bump(web.LayerLocal, added)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
	if answer.Written || answer.Why == "" {
		t.Fatalf("the bump wrote %+v, want a refusal with a reason", answer)
	}
	if after := readRule(t, added); after != before {
		t.Errorf("the refused bump changed the file:\n%s", after)
	}
}

// TestBumpRefusesAPathNoMemberReads: the write is the same one the rules screen
// uses, so a version cannot be raised in a file the team definitions of the layer
// do not read, and the file is left as it was.
func TestBumpRefusesAPathNoMemberReads(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	stray := filepath.Join(f.proj, ".hergy", "teams", "demo", "stray.yaml")
	writeFile(t, stray, "version: 1\nrevision: 1\n")
	before := readRule(t, stray)

	if _, err := recordPort(t).Bump(web.LayerLocal, stray); err == nil {
		t.Fatal("a file no member of the layer reads was written")
	}
	if after := readRule(t, stray); after != before {
		t.Errorf("the refused bump changed the file:\n%s", after)
	}
}

// TestBumpOfAFileThatIsNotThereIsAnError: raising a version of a document that
// went away says so rather than writing anything.
func TestBumpOfAFileThatIsNotThereIsAnError(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	gone := filepath.Join(f.proj, ".hergy", "teams", "demo", "gone.yaml")

	if _, err := recordPort(t).Bump(web.LayerLocal, gone); err == nil {
		t.Fatal("a document that is not there was written")
	}
}

// idsOfRules is a rule document's ids, for comparing.
func idsOfRules(doc *rules.Document) []string {
	out := make([]string, 0, len(doc.Rules()))
	for _, rule := range doc.Rules() {
		out = append(out, rule.ID)
	}
	return out
}
