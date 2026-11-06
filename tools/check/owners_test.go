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

// TestChangedFindingsFlagsAComparatorOutsideItsOwner: the names the changed
// owner replaced fail outside lib/changed.ts (r-1e9a).
func TestChangedFindingsFlagsAComparatorOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/roles/state.ts", "export const c = (r) => changeCount(r) > 0;\n")
	writeTree(t, root, behaviorOwnerPath, "export const changeCount = (r) => 0;\n")
	r := &runner{root: root}
	findings, err := r.changedFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "changeCount") {
		t.Fatalf("findings = %v, want one for changeCount", findings)
	}
}

const behaviorOwnerPath = "internal/web/src/lib/changed.ts"

// TestDangerFindingsFlagsTheOldConfirmationState: the confirmation state the
// screens held (setConfirm and its callers) fails outside the owner (r-ccf8).
func TestDangerFindingsFlagsTheOldConfirmationState(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/state.ts",
		"const setConfirm = (key: string, on: boolean) => {};\n"+
			"export function pressDel(r: object) { setConfirm('del', true); }\n")
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { removeMember: 'undoable' };\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("read %d findings, want 2 (setConfirm and pressDel):\n%s", len(findings), strings.Join(findings, "\n"))
	}
	if !strings.Contains(strings.Join(findings, "\n"), "setConfirm") || !strings.Contains(strings.Join(findings, "\n"), "pressDel") {
		t.Errorf("findings = %v, want setConfirm and pressDel named", findings)
	}
}

// TestDangerFindingsLetsTheOwnerAndTheDefinitionsThrough: the owner calls
// askConfirm and offerUndo; their own files define them; the dialog is mounted
// once, in lib/dialogs.tsx.
func TestDangerFindingsLetsTheOwnerAndTheDefinitionsThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { removeMember: 'undoable' };\nexport function runDanger(a: string, o: object): void { askConfirm(o); offerUndo(o); }\n")
	writeTree(t, root, dangerDialogForTest, "export const askConfirm = (ask: object): void => {};\n")
	writeTree(t, root, dangerNoticeForTest, "export function offerUndo(one: object): void {}\n")
	writeTree(t, root, dangerMountedForTest, "export function Dialogs() { return <ConfirmDialog />; }\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestDangerFindingsFlagsASecondDialogMount: <ConfirmDialog stands in one place
// (d-2cce); a screen that mounts it again fails.
func TestDangerFindingsFlagsASecondDialogMount(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, dangerMountedForTest, "export function Dialogs() { return <ConfirmDialog />; }\n")
	writeTree(t, root, "internal/web/src/roles/view.tsx", "export function Screen() { return <div><ConfirmDialog /></div>; }\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "roles/view.tsx:1") {
		t.Fatalf("findings = %v, want one for the second <ConfirmDialog", findings)
	}
}

// TestDangerFindingsChecksTheDangerButtons: a button with data-danger says which
// action it runs, and the name is one the table knows (d-2cce).
func TestDangerFindingsChecksTheDangerButtons(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { removeMember: 'undoable', stopTeam: 'confirm' };\n")
	writeTree(t, root, "internal/web/src/teams/canvas/attr.tsx",
		"export const A = () => <button data-danger data-danger-act=\"removeMember\">外す</button>;\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("a marked button failed:\n%s", strings.Join(findings, "\n"))
	}

	writeTree(t, root, "internal/web/src/teams/canvas/attr.tsx",
		"export const A = () => <button data-danger>外す</button>;\n"+
			"export const B = () => <button data-danger data-danger-act=\"removeEverything\">外す</button>;\n")
	findings, err = r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("read %d findings, want 2 (no act, unknown act):\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestDangerGateIsAReportUntilEnforced: the danger section prints its findings
// while danger_enforce is false and fails once it is true.
func TestDangerGateIsAReportUntilEnforced(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/state.ts", "export function f(): void { setConfirm('del', true); }\n")
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { removeMember: 'undoable' };\n")
	r := &runner{root: root}
	writeTree(t, root, ownersPath, "enforce: true\ndanger_enforce: false\n")
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a reporting danger section failed the gate: %v", err)
	}
	writeTree(t, root, ownersPath, "enforce: true\ndanger_enforce: true\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced danger section passed with a screen holding its own confirmation state")
	}
}

const (
	dangerOwnerForTest   = "internal/web/src/lib/danger.ts"
	dangerDialogForTest  = "internal/web/src/lib/confirm.tsx"
	dangerNoticeForTest  = "internal/web/src/lib/undo.tsx"
	dangerMountedForTest = "internal/web/src/lib/dialogs.tsx"
)
