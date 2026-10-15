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

	port := recordPort(t)
	bumped, err := port.Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
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

	bumped, err := recordPort(t).Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("raising the version: %v", err)
	}
	if bumped.From != 0 || bumped.To != 1 {
		t.Errorf("the bump went %d → %d, want 0 → 1", bumped.From, bumped.To)
	}
	raised, err := rules.Parse(readRule(t, protocol), protocol)
	if err != nil {
		t.Fatalf("the file the bump wrote does not read back: %v", err)
	}
	if raised.Revision == nil || *raised.Revision != 1 {
		t.Errorf("the file writes revision %v, want 1", raised.Revision)
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
