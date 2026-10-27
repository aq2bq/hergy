// profiles/actions.js — 一覧のタブの押し込み（spec 4 章、r-c683）。
//
// 押し込みはここに集める: 部品（view.js）は描くだけ、main.js は押し込みを
// ここへ渡す。だから同じ押し込みが 2 回走ることも、部品が自分の DOM を書き換える
// こともない（webui-preact-design 5 章 1）。
import {
  rows, filterKind, query, GROUPED, CURRENT, pathFor, labelOf, classOf, BASE_KINDS,
  addRow, duplicateRow, copyPanel, discardPanel, deletePanel, savePanel, checkPanel,
  patchPanel, panelByKey, toggleRow, closeAllConfirm, askFocus, askScroll, syncURL, tellDirty,
} from './state.js';

// ---- the toolbar --------------------------------------------------------
export function pressNew() {
  clearFilter();
  const row = addRow('project', CURRENT);
  if (row && row.panels[0]) askFocus(row.panels[0].key, 'name', 0, false);
  return row;
}

export function pressFilter(kind) {
  filterKind.value = kind || '';
  syncURL();
  tellDirty();
}

export function pressQuery(value) {
  query.value = value || '';
  syncURL();
  tellDirty();
}

export function clearFilter() {
  filterKind.value = '';
  query.value = '';
  syncURL();
}

// ---- the rows -----------------------------------------------------------
export function pressToggle(key) {
  toggleRow(key);
  const row = rows.value.find((r) => r.key === key);
  if (row && row.open) askScroll(key);
  syncURL();
  tellDirty();
}

// ---- the panel ----------------------------------------------------------
export function pressSetKind(key, kind) {
  patchPanel(key, (panel) => ({ draft: { ...panel.draft, kind } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressKindOther(key, kind) {
  if (!kind) return;
  pressSetKind(key, kind);
}

export function pressArgAdd(key) {
  const panel = panelByKey(key);
  if (!panel) return;
  const args = panel.draft.args.concat(['']);
  patchPanel(key, (p) => ({ draft: { ...p.draft, args } }));
  askFocus(key, 'arg', args.length - 1, false);
  scheduleCheck(key);
  tellDirty();
}

export function pressArgDel(key, index) {
  const panel = panelByKey(key);
  if (!panel) return;
  patchPanel(key, (p) => ({ draft: { ...p.draft, args: p.draft.args.filter((_, i) => i !== index) } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressEnvAdd(key) {
  const panel = panelByKey(key);
  if (!panel) return;
  const env = panel.draft.env.concat([{ key: '', value: '' }]);
  patchPanel(key, (p) => ({ draft: { ...p.draft, env } }));
  askFocus(key, 'ek', env.length - 1, false);
  scheduleCheck(key);
  tellDirty();
}

export function pressEnvDel(key, index) {
  patchPanel(key, (p) => ({ draft: { ...p.draft, env: p.draft.env.filter((_, i) => i !== index) } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressSave(key) {
  return savePanel(key);
}

export function pressDup(key) {
  const panel = panelByKey(key);
  if (!panel) return;
  const row = duplicateRow(panel);
  if (row && row.panels[0]) {
    initPanel(row.panels[0].key);
    askFocus(row.panels[0].key, 'name', 0, true);
  }
}

export function pressCopy(key) {
  const panel = panelByKey(key);
  if (!panel) return;
  const fresh = copyPanel(panel);
  if (fresh) {
    initPanel(fresh.key);
    askFocus(fresh.key, 'name', 0, true);
  }
}

export function pressDiscard(key) {
  discardPanel(key);
  syncURL();
  tellDirty();
}

// reorderArgs is a drag that ended: the chips' order is the order the save
// writes (spec 4 章).
export function reorderArgs(key, order) {
  const panel = panelByKey(key);
  if (!panel) return;
  patchPanel(key, (p) => ({ draft: { ...p.draft, args: order.slice() } }));
  scheduleCheck(key);
  tellDirty();
}

export function pressDel(key) {
  patchPanel(key, { confirming: true });
  askFocus(key, 'del-yes', 0, false);
}

export function pressDelNo(key) {
  patchPanel(key, { confirming: false });
}

export function pressDelYes(key) {
  return deletePanel(key);
}

export function pressRename(key, on) {
  patchPanel(key, { rename: !!on });
}

// ---- the fields ---------------------------------------------------------
export function inputField(key, field, index, value) {
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
const timers = new Map();
export function scheduleCheck(key) {
  clearTimeout(timers.get(key));
  timers.set(key, setTimeout(() => { checkPanel(key); }, 300));
}

// initPanel is a panel that stands open: it asks what is wrong with its draft
// once, as the screen always did.
export function initPanel(key) {
  checkPanel(key);
}

export { closeAllConfirm, pathFor, labelOf, classOf, BASE_KINDS };
