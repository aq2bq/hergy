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

// TestDangerFindingsFlagsTheBrowsersConfirmOutsideItsOwner: the browser's own
// confirmation is the same concept and stands in the owner alone (d-021e).
func TestDangerFindingsFlagsTheBrowsersConfirmOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { leaveWithDraft: 'confirm' };\nexport function ask(): void { window.confirm('move?'); }\n")
	writeTree(t, root, "internal/web/src/lib/nav.ts", "export function leave(): boolean { return window.confirm('move?'); }\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "nav.ts:1") || !strings.Contains(findings[0], "window.confirm") {
		t.Fatalf("findings = %v, want one for the browser's confirm in nav.ts", findings)
	}
}

// TestDangerFindingsFlagsTheClosingWarning: the browser's beforeunload stands in
// a string, which the code's mask hides, so the raw line is read; only the owner
// or an allowlist entry holds it (d-3a42).
func TestDangerFindingsFlagsTheClosingWarning(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, dangerOwnerForTest, "export const DANGER = { leaveWithDraft: 'confirm' };\n")
	writeTree(t, root, "internal/web/src/lib/prose-transfer-view.tsx",
		"export function hold(): void { window.addEventListener('beforeunload', warn); }\n"+
			"// a comment that names beforeunload is not a listener\n")
	r := &runner{root: root}
	findings, err := r.dangerFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "prose-transfer-view.tsx:1") {
		t.Fatalf("findings = %v, want one for the closing warning", findings)
	}
	conf := ownersFile{Allow: []ownerAllow{{
		Path: "internal/web/src/lib/prose-transfer-view.tsx", Name: "beforeunload",
		Why: "the browser's own words only", Decision: "d-3a42",
	}}}
	findings, err = r.dangerFindings(conf)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("an allowed closing warning failed:\n%s", strings.Join(findings, "\n"))
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
	writeTree(t, root, "internal/web/src/teams/canvas/prompt.tsx", "const workerPrompt = 1;\n")
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

// TestSelectionFindingsFlagsTheOldWriterAndReader: a screen that writes the pick
// or the column itself, or brings back the names the owner replaced, is the copy
// ⑥ r-3d54 removed.
func TestSelectionFindingsFlagsTheOldWriterAndReader(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/canvas/act.ts",
		"export function clear(r) { r.sel.value = null; }\n"+
			"const back = () => selectPick(r, null);\n"+
			"const old = () => selOf(r);\n")
	r := &runner{root: root}
	findings, err := r.selectionFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("read %d findings, want 3:\n%s", len(findings), strings.Join(findings, "\n"))
	}
	joined := strings.Join(findings, "\n")
	for _, want := range []string{"r.sel", "selectPick", "selOf"} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings = %v, want %s named", findings, want)
		}
	}
	if !strings.Contains(joined, "internal/web/src/teams/canvas/act.ts:1:") {
		t.Errorf("findings = %v, want the write's line named", findings)
	}
}

// TestSelectionFindingsFlagsASecondSideOf: sideOf's definition stands in the
// owner alone; a screen may call it, and reading the column through it passes.
func TestSelectionFindingsFlagsASecondSideOf(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/canvas/shared.ts",
		"export const sideOf = (r) => r.side.value || 'attr';\n")
	r := &runner{root: root}
	findings, err := r.selectionFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "sideOf の定義") {
		t.Fatalf("findings = %v, want one for the second definition", findings)
	}
}

// TestSelectionFindingsLetsTheOwnerThrough: the owner writes the pick and the
// column and defines sideOf, the table passes, and a screen that calls sideOf
// passes.
func TestSelectionFindingsLetsTheOwnerThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, selectionOwner,
		"export function sideOf(r) { return r.side.value; }\n"+
			"export function select(r, e) { r.sel.value = null; r.side.value = 'attr'; }\n")
	writeTree(t, root, selectionTable, "export const SIDE_RULES = [];\n")
	writeTree(t, root, "internal/web/src/teams/canvas.tsx",
		"import { sideOf } from '../selection.js';\nconst s = () => sideOf(r);\n")
	r := &runner{root: root}
	findings, err := r.selectionFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestOwnersSelectionSectionIsHard: the selection section reports while its flag
// is false and fails once selection_enforce is true.
func TestOwnersSelectionSectionIsHard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/x.ts", "export const f = (r) => { r.sel.value = null; };\n")
	writeTree(t, root, ownersPath, "selection_enforce: false\nallow: []\n")
	r := &runner{root: root}
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a report run failed: %v", err)
	}
	writeTree(t, root, ownersPath, "selection_enforce: true\nallow: []\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced selection section passed with a writer outside its owner")
	}
}

// TestAddressFindingsFlagsTheOpenValueWrittenByAScreen: a screen that spells the
// open value itself is the copy ⑥-2 removed, in either quote and on any receiver.
func TestAddressFindingsFlagsTheOpenValueWrittenByAScreen(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/roles/state.ts",
		"const q = new URLSearchParams();\n"+
			"q.set('open', 'builder|global');\n"+
			"const one = new URLSearchParams(location.search).get(\"open\");\n")
	r := &runner{root: root}
	findings, err := r.addressFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || !strings.Contains(strings.Join(findings, "\n"), "address.ts") {
		t.Fatalf("findings = %v, want the set and the get", findings)
	}
}

// TestAddressFindingsLetsTheOwnerThrough: the owner spells it, and a screen that
// asks the owner passes.
func TestAddressFindingsLetsTheOwnerThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, addressOwner,
		"export function readOpen(p) { return parseOpen(p.get('open')); }\n"+
			"export function writeOpen(p, refs) { if (refs.length) p.set('open', ''); }\n")
	writeTree(t, root, "internal/web/src/roles/state.ts",
		"import { readOpen, writeOpen } from '../lib/address.js';\nconst one = readOpen(qs);\nwriteOpen(q, one);\n")
	r := &runner{root: root}
	findings, err := r.addressFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
}

// TestOwnersAddressSectionIsHard: the address section reports while its flag is
// false and fails once address_enforce is true.
func TestOwnersAddressSectionIsHard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/profiles/state.ts", "const one = q.get('open');\n")
	writeTree(t, root, ownersPath, "address_enforce: false\nallow: []\n")
	r := &runner{root: root}
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a report run failed: %v", err)
	}
	writeTree(t, root, ownersPath, "address_enforce: true\nallow: []\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced address section passed with a screen spelling the open value")
	}
}

// TestPagePositionFindingsFlagsThePageMovedOutsideItsOwner: the calls that move
// the page, a panel field written outside an allowed panel, a focus without
// preventScroll, and the old name all fail (⑦ r-6ff8, d-c5ce).
func TestPagePositionFindingsFlagsThePageMovedOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/roles/view.tsx",
		"const a = () => window.scrollBy(0, 10);\n"+
			"const b = () => el.scrollIntoView({ block: 'center' });\n"+
			"const c = () => { el.scrollTop = 3; };\n"+
			"const d = () => { el.focus(); };\n"+
			"const e = () => bringUnderBand(el);\n"+
			"const ok = () => { if (el.scrollTop === 0) el.focus({ preventScroll: true }); };\n")
	r := &runner{root: root}
	findings, err := r.pagePositionFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 5 {
		t.Fatalf("read %d findings, want 5:\n%s", len(findings), strings.Join(findings, "\n"))
	}
	joined := strings.Join(findings, "\n")
	for _, want := range []string{".scrollBy(", ".scrollIntoView(", "scrollTop", "focus()", "bringUnderBand"} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings = %v, want %s named", findings, want)
		}
	}
}

// TestPagePositionFindingsLetsTheOwnerAndTheAllowedPanelThrough: the owner moves
// the page, an allowlisted panel scrolls inside itself, and a comparison is not
// a write. The old name stands nowhere, the owner included.
func TestPagePositionFindingsLetsTheOwnerAndTheAllowedPanelThrough(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/lib/page-position.ts",
		"export function movePage(): void { window.scrollBy({ top: 1 }); el.scrollIntoView({ block: 'nearest' }); host.scrollTop += 2; }\n")
	writeTree(t, root, "internal/web/src/lib/yamltab.tsx",
		"const jump = () => { el.focus({ preventScroll: true }); el.scrollTop = 4; el.scrollLeft = 0; };\n"+
			"const same = () => el.scrollTop === 0;\n")
	conf := ownersFile{Allow: []ownerAllow{
		{Path: "internal/web/src/lib/yamltab.tsx", Name: "scrollTop", Why: "the panel", Decision: "d-a504"},
		{Path: "internal/web/src/lib/yamltab.tsx", Name: "scrollLeft", Why: "the panel", Decision: "d-a504"},
	}}
	r := &runner{root: root}
	findings, err := r.pagePositionFindings(conf)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("read %d findings, want none:\n%s", len(findings), strings.Join(findings, "\n"))
	}
	writeTree(t, root, "internal/web/src/lib/page-position.ts",
		"export function bringUnderBand(): void { window.scrollBy({ top: 1 }); }\n")
	findings, err = r.pagePositionFindings(conf)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(findings[0], "bringUnderBand") {
		t.Fatalf("findings = %v, want one for the old name", findings)
	}
}

// TestOwnersPagePositionSectionIsHard: the section reports while its flag is
// false and fails once page_position_enforce is true.
func TestOwnersPagePositionSectionIsHard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/x.ts", "export function f(): void { window.scrollBy(0, 1); }\n")
	writeTree(t, root, ownersPath, "page_position_enforce: false\nallow: []\n")
	r := &runner{root: root}
	if err := r.ownersGate(); err != nil {
		t.Fatalf("a report run failed: %v", err)
	}
	writeTree(t, root, ownersPath, "page_position_enforce: true\nallow: []\n")
	if err := r.ownersGate(); err == nil {
		t.Fatal("an enforced page-position section passed with a page moved outside its owner")
	}
}

// TestPagePositionFindingsSeesThePanelScrollAfterADot: a field written through
// another field (body.current.scrollTop) and a += move are the panel's own
// scroll, and the page-position section must see them (⑦, d-b608).
func TestPagePositionFindingsSeesThePanelScrollAfterADot(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/canvas/prompt-comparison.tsx",
		"useEffect(() => { if (body.current) body.current.scrollTop = 0; }, [view]);\n"+
			"const go = () => { if (host && first) host.scrollTop += first.top - 8; };\n")
	r := &runner{root: root}
	findings, err := r.pagePositionFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || !strings.Contains(strings.Join(findings, "\n"), ".scrollTop の代入") {
		t.Fatalf("findings = %v, want the two panel writes", findings)
	}
}

// TestAddressFindingsFlagsARowMatchOutsideItsOwner: the rule that says which rows
// an address names stands in lib/address.ts; a screen that matches refs itself is
// the copy ⑥-2 removed.
func TestAddressFindingsFlagsARowMatchOutsideItsOwner(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/src/teams/state.ts",
		"const rowsOfRef = (ref) => rows.filter((r) => sameRow(ref, r));\n")
	r := &runner{root: root}
	findings, err := r.addressFindings(ownersFile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 || !strings.Contains(strings.Join(findings, "\n"), "rowsFor") {
		t.Fatalf("findings = %v, want the two names", findings)
	}
}
