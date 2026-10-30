// profiles/actions.ts — 一覧のタブの押し込み（spec 4 章、r-c683）。
//
// 押し込みはここに集める: 部品（view.tsx / assign.tsx）は描き、自分の onClick /
// onInput からここの関数を直接呼ぶ。だから同じ押し込みが 2 回走ることも、部品が
// 自分の DOM を書き換えることもない（webui-preact-design 5 章 1）。
import {
  rows, filterKind, query, placeGroups, pathFor, labelOf, classOf, BASE_KINDS, assignKind,
  addRow, duplicateRow, copyPanel, discardPanel, deletePanel, savePanel, checkPanel,
  patchPanel, panelByKey, toggleRow, closeRow, closeAllConfirm, askFocus, askScroll, syncURL,
  syncAssignURL, saveTeam, tellDirty,
} from './state.js';
import type { Place } from '../lib/api.js';
import type { Row } from './types.js';

// ---- the toolbar --------------------------------------------------------
// pressNew adds a row under the place's head (r-410c): the head says the place,
// so no row carries a switch to move it afterwards.
export function pressNew(group?: string): Row {
  clearFilter();
  const key = group === undefined ? placeGroups()[0].key : group;
  const place: Place = key === 'global' ? 'global' : 'project';
  const row = addRow(place, key);
  if (row && row.panels[0]) askFocus(row.panels[0].key, 'name', 0, false);
  tellDirty();
  return row;
}

export function pressFilter(kind: string): void {
  filterKind.value = kind || '';
  syncURL();
  tellDirty();
}

export function pressQuery(value: string): void {
  query.value = value || '';
  syncURL();
  tellDirty();
}

export function clearFilter(): void {
  filterKind.value = '';
  query.value = '';
  syncURL();
}

// ---- the rows -----------------------------------------------------------
export function pressToggle(key: string): void {
  toggleRow(key);
  const row = rows.value.find((r) => r.key === key);
  if (row && row.open) askScroll(key);
  syncURL();
  tellDirty();
}

// ---- the panel ----------------------------------------------------------
export function pressSetKind(key: string, kind: string): void {
  patchPanel(key, (panel) => ({ draft: { ...panel.draft, kind } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressKindOther(key: string, kind: string): void {
  if (!kind) return;
  pressSetKind(key, kind);
}

export function pressArgAdd(key: string): void {
  const panel = panelByKey(key);
  if (!panel) return;
  const args = panel.draft.args.concat(['']);
  patchPanel(key, (p) => ({ draft: { ...p.draft, args } }));
  askFocus(key, 'arg', args.length - 1, false);
  scheduleCheck(key);
  tellDirty();
}

export function pressArgDel(key: string, index: number): void {
  const panel = panelByKey(key);
  if (!panel) return;
  patchPanel(key, (p) => ({ draft: { ...p.draft, args: p.draft.args.filter((_, i) => i !== index) } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressEnvAdd(key: string): void {
  const panel = panelByKey(key);
  if (!panel) return;
  const env = panel.draft.env.concat([{ key: '', value: '' }]);
  patchPanel(key, (p) => ({ draft: { ...p.draft, env } }));
  askFocus(key, 'ek', env.length - 1, false);
  scheduleCheck(key);
  tellDirty();
}

export function pressEnvDel(key: string, index: number): void {
  patchPanel(key, (p) => ({ draft: { ...p.draft, env: p.draft.env.filter((_, i) => i !== index) } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressSave(key: string): Promise<void> {
  return savePanel(key);
}

export function pressDup(key: string): void {
  const panel = panelByKey(key);
  if (!panel) return;
  const row = duplicateRow(panel);
  if (row && row.panels[0]) {
    initPanel(row.panels[0].key);
    askFocus(row.panels[0].key, 'name', 0, true);
  }
  tellDirty();
}

export function pressCopy(key: string): void {
  const panel = panelByKey(key);
  if (!panel) return;
  // The row the person pressed closes, and the copy under the other place's
  // head is the one open row they read (d-022a).
  closeRow(panel.rowKey);
  const fresh = copyPanel(panel);
  if (fresh) {
    initPanel(fresh.key);
    // The copy stands off the screen when the other place's head is below it:
    // its name comes to the band, all of it chosen (d-022a).
    askFocus(fresh.key, 'name', 0, true, true);
  }
  tellDirty();
}

export function pressDiscard(key: string): void {
  discardPanel(key);
  syncURL();
  tellDirty();
}

// reorderArgs is a drag that ended: the chips' order is the order the save
// writes (spec 4 章).
export function reorderArgs(key: string, order: string[]): void {
  const panel = panelByKey(key);
  if (!panel) return;
  patchPanel(key, (p) => ({ draft: { ...p.draft, args: order.slice() } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressDel(key: string): void {
  patchPanel(key, { confirming: true });
  askFocus(key, 'del-yes', 0, false);
}

export function pressDelNo(key: string): void {
  patchPanel(key, { confirming: false });
}

export function pressDelYes(key: string): Promise<void> {
  return deletePanel(key);
}

export function pressRename(key: string, on: boolean): void {
  patchPanel(key, { rename: !!on });
}

// ---- the layer, the assign tab's kind and its save ----------------------
// pressLayer moves a row being added between the project's file and the
// global one: the path, the segment and the pending pill follow.
export function pressLayer(key: string, file: Place): void {
  const panel = panelByKey(key);
  if (!panel || panel.file === file) return;
  patchPanel(key, { file, path: pathFor(file, panel.group) });
  scheduleCheck(key);
  tellDirty();
}

// pressKind is the assign tab's kind: the other kinds and people go thin, the
// chosen one stays plain, and the address keeps it (R6).
export function pressKind(kind: string): void {
  assignKind.value = assignKind.value === kind ? null : kind;
  syncAssignURL();
  tellDirty();
}

export function pressTeamSave(name: string): Promise<boolean> {
  return saveTeam(name).then((ok) => { tellDirty(); return ok; });
}

// ---- the fields ---------------------------------------------------------
export function inputField(key: string, field: string, index: number, value: string): void {
  const panel = panelByKey(key);
  if (!panel) return;
  if (field === 'name') {
    patchPanel(key, (p) => ({ draft: { ...p.draft, name: value } }));
  } else if (field === 'arg') {
    patchPanel(key, (p) => ({ draft: { ...p.draft, args: p.draft.args.map((arg, i) => (i === index ? value : arg)) } }));
  } else if (field === 'ek') {
    patchPanel(key, (p) => ({ draft: { ...p.draft, env: p.draft.env.map((pair, i) => (i === index ? { ...pair, key: value } : pair)) } }));
  } else if (field === 'ev') {
    patchPanel(key, (p) => ({ draft: { ...p.draft, env: p.draft.env.map((pair, i) => (i === index ? { ...pair, value } : pair)) } }));
  } else {
    return;
  }
  scheduleCheck(key);
  tellDirty();
}

// ---- the check the typing rides on (300ms after it stops) ---------------
const timers = new Map<string, ReturnType<typeof setTimeout>>();
export function scheduleCheck(key: string): void {
  const timer = timers.get(key);
  if (timer !== undefined) clearTimeout(timer);
  timers.set(key, setTimeout(() => { checkPanel(key); }, 300));
}

// initPanel is a panel that stands open: it asks what is wrong with its draft
// once, as the screen always did.
export function initPanel(key: string): void {
  checkPanel(key);
}

export { closeAllConfirm, pathFor, labelOf, classOf, BASE_KINDS };
