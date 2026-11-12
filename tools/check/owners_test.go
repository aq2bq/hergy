package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOwnerFindingsFlagsTheBandWrittenOutsideItsOwner: a screen that writes the
// band's number itself is the copy the gate exists for (r-ea17, d-4d88). A
// comment that names the writer is not a call.
func TestOwnerFindingsFlagsTheBandWrittenOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/roles/state.ts",
		"// a screen must not call setCount itself\n"+
			"export function savedCount(): number { return 2; }\n"+
			"export function band(): void { setCount('roles', savedCount()); }\n")
	r := &runner{root: root}
	findings, err := r.ownerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("read %d findings, want 1:\n%s", len(findings), strings.Join(findings, "\n"))
	}
	if !strings.Contains(findings[0], "internal/web/src/roles/state.ts:3:") || !strings.Contains(findings[0], "setCount") {
		t.Errorf("finding = %q, want the call at state.ts:3 named", findings[0])
	}
}

// TestOwnerFindingsLetsTheOwnerThrough: the owner and the place that asks it
// pass, and an allowlisted name passes only with its own file.
func TestOwnerFindingsLetsTheOwnerThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/lib/nav.ts", "export function setCount(screen: string, n: number): void {}\n")
	writeTree(t, root, "internal/web/src/lib/band-counts.ts", "import { setCount } from './nav.js';\nexport function applyCounts(n: number): void { setCount('teams', n); }\n")
	writeTree(t, root, "internal/web/src/lib/hq/state.ts", "import { applyCounts } from '../band-counts.js';\nexport function bandCounts(): void { applyCounts(3); }\n")
	writeTree(t, root, "internal/web/src/teams/state.ts", "export function band(): void { applyCounts(4); }\n")
	r := &runner{root: root}
	conf := ownersFile{Allow: []ownerAllow{{
		Path: "internal/web/src/teams/state.ts", Name: "applyCounts",
		Why: "a made-up exception", Decision: "d-0000",
	}}}
	findings, err := r.ownerFindings(conf)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestOwnersGateRefusesAnEntryWithoutADecision: an exception needs the path, the
// name, why, and the decision that allowed it (owners.yml's own rule).
func TestOwnersGateRefusesAnEntryWithoutADecision(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, ownersPath, "enforce: true\nallow:\n  - path: internal/web/src/x.ts\n    name: setCount\n    why: something\n")
	r := &runner{root: root}
	err := r.ownersGate()
	if err == nil || !strings.Contains(err.Error(), "決定") {
		t.Fatalf("ownersGate = %v, want a refusal naming the missing decision", err)
	}
}

// TestOwnersGateIsAReportUntilEnforced: the same finding passes while the file
// says enforce: false, and fails once it says true.
func TestOwnersGateIsAReportUntilEnforced(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/x.ts", "export function f(): void { setCount('teams', 1); }\n")
	r := &runner{root: root}
	writeTree(t, root, ownersPath, "enforce: false\n")
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a report run failed: %v", err)
	}
	writeTree(t, root, ownersPath, "enforce: true\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced run passed with a writer outside its owner")
	}
}

// TestOwnersConfReadsTheRepositoryFile keeps the shipped owners.yml valid: the
// gate it turns on is the one this change ships.
func TestOwnersConfReadsTheRepositoryFile(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ownersPath)); err != nil {
		t.Fatal(err)
	}
	r := &runner{root: root}
	if _, err := r.ownersConf(); err != nil {
		t.Fatal(err)
	}
}

// TestPromptFindingsFlagsTheBuilderOutsideItsOwner: a WebUI server file that
// builds a prompt itself is the copy ⑤ r-ba9b removed.
func TestPromptFindingsFlagsTheBuilderOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/api_team.go", "func build(def *Definition) { prompt.Member(def, node, run) }\n")
	r := &runner{root: root}
	findings, err := r.promptFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "prompt.Member") || !strings.Contains(findings[0], "api_prompt.go") {
		t.Fatalf("findings = %v, want one for prompt.Member", findings)
	}
}

// TestPromptFindingsFlagsTheReadersOutsideTheirOwner: a screen that calls the
// comparison itself, or brings back an old cache name, fails.
func TestPromptFindingsFlagsTheReadersOutsideTheirOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/canvas/prompt.tsx",
		"const one = () => call('POST', '/api/team/prompt-comparison');\n"+
			"const two = () => workerPrompt(r, i);\n")
	r := &runner{root: root}
	findings, err := r.promptFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %v, want the address and the old name", findings)
	}
	joined := strings.Join(findings, "\n")
	if !strings.Contains(joined, "prompt-comparison") || !strings.Contains(joined, "workerPrompt") {
		t.Errorf("findings = %v, want both named", findings)
	}
}

// TestPromptFindingsLetsTheOwnersThrough: the builder and the reader pass, and
// the command's own senders are outside the section's scope.
func TestPromptFindingsLetsTheOwnersThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/api_prompt.go", "func build(def *Definition) { prompt.Member(def, node, run); prompt.Worker(prompt.WorkerInput{}) }\n")
	writeTree(t, root, "internal/web/src/teams/prompt-source.ts",
		"const p = signal({});\nconst runOf = (k) => '';\nconst setRun = (k, t) => {};\n"+
			"const one = () => call('POST', '/api/team/prompt-comparison');\n")
	writeTree(t, root, "internal/web/src/lib/editing.ts", "const checkWorker = () => check('team', { part: 'worker' });\n")
	writeTree(t, root, "internal/cli/hire.go", "func plan() { prompt.Worker(prompt.WorkerInput{}) }\n")
	r := &runner{root: root}
	findings, err := r.promptFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestOwnersPromptSectionIsHard: the prompt section reports while its flag is
// false and fails once prompt_enforce is true.
func TestOwnersPromptSectionIsHard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/api_team.go", "func build() { prompt.Member(nil, nil, run) }\n")
	writeTree(t, root, ownersPath, "prompt_enforce: false\nallow: []\n")
	r := &runner{root: root}
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a report run failed: %v", err)
	}
	writeTree(t, root, ownersPath, "prompt_enforce: true\nallow: []\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced prompt section passed with a builder outside its owner")
	}
}
