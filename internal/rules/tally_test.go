package rules

import "testing"

// TestTallyCountsWhatTheRulesCarry: the origin (by kind, once per rule however
// many origins it writes), the reason (note is not a reason), and the check
// (a report field or a test, with the breakdown).
func TestTallyCountsWhatTheRulesCarry(t *testing.T) {
	doc := parse(t, `version: 1
sections:
  - rules:
      - id: A1
        text: x
        why: because
        origin:
          - kind: incident
          - kind: incident
      - id: A2
        text: y
        note: a note is not a reason
        report: audit.scope
      - id: A3
        text: z
        why: sent or not, it is a reason
        send_why: false
        tests: [T-1, T-2]
        origin:
          - kind: derived
            refs: [A1]
      - id: A4
        text: w
        why: because
        report: audit.who
        tests: [T-3]
        origin:
          - kind: instruction
      - id: A5
        text: v
reports:
  audit:
    where: the ledger
    fields:
      scope: the changed files
      who: who wrote it
`)
	tally := &Tally{}
	tally.Add(doc)

	if tally.Files != 1 || tally.Total != 5 {
		t.Fatalf("files/total = %d/%d, want 1/5", tally.Files, tally.Total)
	}
	if tally.WithOrigin != 3 || tally.Incident != 1 || tally.Instruction != 1 || tally.Derived != 1 {
		t.Errorf("origin = %d (incident %d / instruction %d / derived %d), want 3 (1/1/1)",
			tally.WithOrigin, tally.Incident, tally.Instruction, tally.Derived)
	}
	if tally.WithWhy != 3 {
		t.Errorf("with a reason = %d, want 3 (A2 writes only a note)", tally.WithWhy)
	}
	if tally.WithCheck != 3 || tally.Report != 2 || tally.Tests != 2 {
		t.Errorf("check = %d (report %d / test %d), want 3 (2/2)", tally.WithCheck, tally.Report, tally.Tests)
	}
	if len(tally.Missing) != 1 {
		t.Fatalf("missing origin = %+v, want one file", tally.Missing)
	}
	missing := tally.Missing[0]
	if missing.Path != "rules.yaml" || len(missing.IDs) != 2 || missing.IDs[0] != "A2" || missing.IDs[1] != "A5" {
		t.Errorf("missing = %+v, want rules.yaml with A2, A5", missing)
	}
}

// TestTallySkipsAFileWithEveryOrigin: a document whose rules all say where they
// came from adds no line to the missing list, and adding documents sums them.
func TestTallySkipsAFileWithEveryOrigin(t *testing.T) {
	withOrigin := parseAt(t, "one.yaml", "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n        origin:\n          - kind: incident\n")
	without := parseAt(t, "two.yaml", "version: 1\nsections:\n  - rules:\n      - id: B1\n        text: y\n")
	tally := &Tally{}
	tally.Add(withOrigin)
	tally.Add(without)
	if tally.Files != 2 || tally.Total != 2 || tally.WithOrigin != 1 {
		t.Errorf("tally = %+v, want 2 files, 2 rules, 1 with an origin", tally)
	}
	if len(tally.Missing) != 1 || tally.Missing[0].Path != "two.yaml" {
		t.Errorf("missing = %+v, want only two.yaml", tally.Missing)
	}
}
