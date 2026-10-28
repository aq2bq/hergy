// profiles/state.ts — プロファイルの画面の state（webui-preact-design 4 章・5 章）。
//
// サーバーの答えはここだけに入る（5 章 2）。部品（view.tsx）は signal を読んで
// 描くだけで、捕まえておいた要素に書き込まない。画面は 3 つのタブを持つ:
// 一覧（プロファイルの行と、行の中のプロジェクト/グローバルの面）、
// 割り当て（チーム定義のメンバーのプロファイル）、YAML（profiles.yaml の面）。
import { signal, computed } from '../lib/preact.js';
import { resource, type Reading } from '../lib/resource';
import { profileModel, check as apiCheck, save as apiSave, del as apiDel, list as apiList, problemText, type Place, type ProfilePanel, type ProfileRow, type Schemas } from '../lib/api.js';
import { parseOpen, token as hqToken } from '../lib/hq.js';
import { setDirty } from '../lib/nav.js';
import type {
  Assign, AssignRow, Carried, ChangeSet, EnvPair, FocusAsk, Island,
  IslandHQ, IslandGroup, IslandRow, IslandTeam, OpenTarget, Panel, ProfileCheck,
  ProfileList, Problem, Row, ScrollAsk, Team, YamlFile, YamlOne,
} from './types.js';

// mustElement is a node the page always carries: the screen reads it once, and
// a page without it is a template that stopped naming the screen's own view.
function mustElement(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (!el) throw new Error(`profiles: the page carries no #${id}`);
  return el;
}

export const view = mustElement('view');

// stringList is a JSON array of strings the page or the address carries. A
// value that is not one reads as no names, as it did when the screen parsed it
// without looking.
function stringList(raw: string | undefined): string[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((one): one is string => typeof one === 'string') : [];
  } catch (e) { return []; }
}

// The page carries the kind answers in the boundary's shape (spec-boundary 1,
// 2, 5): the profile list and, on the 割り当て tab, the assignment cards. The
// rows, the panels and the YAML sections are drawn from them here, and the Go
// templates draw none of them.
export const ISLAND: Island = (() => {
  const el = document.getElementById('profiles-data');
  if (!el) return {};
  try { return JSON.parse(el.textContent || '{}') as Island; } catch (e) { return {}; }
})();

export const TAB = view.dataset.tab || '';
export const BASE_KINDS = ['claude', 'codex', 'opencode', 'gemini'];
export const otherKinds = (): string[] => stringList(view.dataset.otherKinds);
export const classOf = (kind: string): string => (BASE_KINDS.includes(kind) ? 'k-' + kind : '');
// placeOf is a place a string names: the address and the band carry words, and
// anything that is not the global one is this project's (the two the screen has).
export const placeOf = (file: string): Place => (file === 'global' ? 'global' : 'project');
export const labelOf = (file: string): string => (file === 'global' ? 'グローバル' : 'プロジェクト');
export const otherOf = (file: Place): Place => (file === 'global' ? 'project' : 'global');
const pathOf = (file: Place): string => (file === 'global' ? view.dataset.pathShared : view.dataset.pathProject) || '';

// HQ is the headquarters lists (d-73fc): every listed project's profiles, then
// the global layer's. Without it the screen is the one project's, as it was.
export const HQ: IslandHQ | null = ISLAND.hq || null;
// GROUPED is a headquarters with projects: listed: the rows stand in groups and
// carry their project. Without projects: the screen is the one project's.
export const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
export const CURRENT = GROUPED ? ((HQ && HQ.projects && HQ.projects.current && HQ.projects.current.path) || '') : '';
export const groupKeyOf = (g: IslandGroup): string => (g.global ? 'global' : (g.project ? g.project.path : ''));
const groupNameOf = (g: IslandGroup): string => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const GROUPS: IslandGroup[] | null = GROUPED ? (HQ && HQ.profiles) || [] : null;
const groupOrder = (): string[] => (GROUPS || []).map(groupKeyOf);
export const groupName = (key: string): string => {
  if (key === 'global') return 'グローバル';
  if (!key || key === CURRENT) {
    if (!GROUPED || !HQ) return 'プロジェクト';
    const p = HQ.projects;
    if (p.mode === 'current') return (p.current && p.current.name) || 'プロジェクト';
    const found = (p.projects || []).find((x) => x.path === CURRENT);
    return found ? found.name : ((p.current && p.current.name) || 'プロジェクト');
  }
  const g = (GROUPS || []).find((x) => groupKeyOf(x) === key);
  return g ? groupNameOf(g) : String(key).split('/').filter(Boolean).pop() || '';
};
// currentProjectName is the name the プロジェクト column gives this serve's
// project, whether or not it is listed (d-8a45, r-5b96 audit 5).
export const currentProjectName = (): string => {
  if (!HQ) return 'プロジェクト';
  const p = HQ.projects;
  return (p.current && p.current.name) || '';
};
// pathFor is the file one panel of a group would write: the group's own
// profiles.yaml. The saved panels carry the server's path already; this is for
// a row a person is adding.
export const pathFor = (file: Place, group: string | undefined): string => {
  if (!GROUPED || !group || group === CURRENT) return pathOf(file);
  if (group === 'global') return '~/.config/hergy/profiles.yaml';
  const g = (GROUPS || []).find((x) => groupKeyOf(x) === group);
  return `${(g && g.project && g.project.display) || ''}/.hergy/profiles.yaml`;
};
// hashes are the two files' hashes as the newest answer left them: a save
// answers with the list, and the hashes are the list's, so the next write
// carries the file the page just wrote (spec-boundary 5).
export const hashes = signal({ project: view.dataset.hashProject || '', global: view.dataset.hashShared || '' });
export const hashOf = (file: Place): string => (file === 'global' ? hashes.value.global : hashes.value.project);

// ---- the address --------------------------------------------------------
// The list carries no screen state (spec-boundary 5): the open rows, the kind
// and the search stand in the address, and the address is where the screen
// starts from.
const params = (): URLSearchParams => new URLSearchParams(location.search);
const OPEN = (params().get('open') || '').split(',').filter(Boolean);
const OPEN_ROWS: OpenTarget[] = parseOpen(OPEN.join(',')) || [];
const isOpenRow = (row: { name: string; group?: string }): boolean =>
  OPEN_ROWS.some((t) => t.name === row.name && (!t.group || t.group === row.group));

export const filterKind = signal(TAB === 'profiles' ? (params().get('kind') || '') : '');
export const query = signal(TAB === 'profiles' ? (params().get('q') || '') : '');
export const openKeys = signal(new Set<string>(OPEN_ROWS.map((t) => t.name + '|' + (t.group || ''))));

// ---- the list's rows ----------------------------------------------------
let nextId = 1;
const newKey = (): string => 'new-' + nextId++;

// panelOf reads one server panel as the panel's state: what the file holds
// (saved), and the draft the fields start from.
function panelOf(p: ProfilePanel, group: string | undefined, rowKey: string): Panel {
  return {
    key: rowKey + ':' + p.file,
    rowKey,
    file: p.file,
    group: group || '',
    path: p.path || pathFor(p.file, group),
    label: p.label,
    otherLabel: p.otherLabel,
    both: !!p.both,
    shadowed: !!p.shadowed,
    saved: p.saved ? { name: p.savedName, kind: p.savedKind, args: JSON.parse(p.savedArgs || '[]') as string[], env: JSON.parse(p.savedEnv || '[]') as EnvPair[] } : null,
    hash: p.hash || '',
    users: p.users || 0,
    renameUsers: p.renameUsers || 0,
    draft: { name: p.name || '', kind: p.kind || 'claude', args: (p.args || []).slice(), env: (p.env || []).map((pair) => ({ key: pair.key, value: pair.value })) },
    problems: [],
    errors: 0,
    savedLine: !!p.savedLine,
    refusal: '',
    confirming: false,
    rename: true,
    layerSwitch: false,
  };
}

// rowOf reads one row of the model: its summary columns, and the panels it
// opens into.
function rowOf(row: ProfileRow & { group?: string }, index: number): Row {
  const key = (row.name || '') + '|' + (row.group || '') + (row.name ? '' : '|' + index);
  return {
    key,
    name: row.name || '',
    group: row.group || '',
    kind: row.kind || '',
    kindClass: row.kindClass || '',
    args: row.args || '',
    env: row.env || [],
    more: row.more || 0,
    users: row.users || 0,
    roles: row.roles || '',
    err: row.err || 0,
    warn: row.warn || 0,
    chips: row.chips || [],
    places: row.places || [],
    isNew: !!row.new,
    open: openKeys.value.has(key),
    panels: (row.panels || []).map((p) => panelOf(p, row.group || '', key)),
  };
}

// modelOf reads one list answer as the screen's model. A grouped screen says
// which project each row stands in, so the rows keep that beside the model's own
// columns and the address's open rows.
type ModelRow = ProfileRow & { group?: string };
type Model = { rows: ModelRow[]; files: YamlFile[] };

function modelOf(list: ProfileList | null, group?: string): Model {
  const model = profileModel(list, OPEN_ROWS.map((t) => t.name));
  const rows: ModelRow[] = model.rows.map((row) => {
    const placed: ModelRow = { ...row, group: GROUPED ? (group || '') : '' };
    return GROUPED ? { ...placed, open: isOpenRow(placed) } : placed;
  });
  return { rows, files: model.files };
}

export const rows = signal<Row[]>([]);
export const files = signal<YamlFile[]>([]);

function readInitial(): { rows: Row[]; files: YamlFile[] } {
  if (!GROUPED) {
    const model = modelOf(ISLAND.profile || null);
    return { rows: model.rows.map(rowOf), files: model.files || [] };
  }
  const all: ModelRow[] = [];
  (GROUPS || []).forEach((g) => { all.push(...modelOf(g.list || null, groupKeyOf(g)).rows); });
  return { rows: all.map(rowOf), files: [] };
}
{
  const first = readInitial();
  rows.value = first.rows;
  files.value = first.files;
}

// ---- what a panel holds, and how far it is from the file ----------------
export function changedOf(panel: Panel): ChangeSet {
  const saved = panel.saved;
  const draft = panel.draft;
  const beforeArgs = (saved && saved.args) || [];
  const beforeEnv = (saved && saved.env) || [];
  const out: ChangeSet = { name: false, kind: false, args: [], env: [], count: 0 };
  out.name = !saved || draft.name !== saved.name;
  out.kind = !saved || draft.kind !== saved.kind;
  draft.args.forEach((arg, i) => { out.args[i] = !saved || beforeArgs[i] !== arg; });
  out.env = draft.env.map((pair, i) => {
    const before = beforeEnv[i];
    return { key: !before || before.key !== pair.key, value: !before || before.value !== pair.value };
  });
  let n = 0;
  if (out.name) n++;
  if (out.kind) n++;
  out.args.forEach((one) => { if (one) n++; });
  if (beforeArgs.length > draft.args.length) n += beforeArgs.length - draft.args.length;
  out.env.forEach((one) => { if (one.key) n++; if (one.value) n++; });
  if (beforeEnv.length > draft.env.length) n += beforeEnv.length - draft.env.length;
  out.count = n;
  return out;
}

export const isDirtyPanel = (panel: Panel): boolean => panel.saved === null || changedOf(panel).count > 0;
export const rowUnsaved = (row: Row): boolean => row.panels.some(isDirtyPanel);
export const saveBlocked = (panel: Panel): boolean =>
  panel.errors > 0 || (panel.saved ? changedOf(panel).count === 0 : false) || panel.draft.name === '';

// ---- what the screen asks the page to do after a press -------------------
// Focus is put only where a person just pressed (webui-preact-design 5 章 5):
// ＋引数 puts the caret in the new argument, ＋環境変数 in the new key, 複製 in
// the name of the copy. The ask is state, and the panel that owns the field
// reads it after the draw.
export const focusAsk = signal<FocusAsk | null>(null);
export function askFocus(key: string, field: string, index: number, select: boolean): void {
  focusAsk.value = { key, field, index: index || 0, select: !!select, seq: ((focusAsk.value && focusAsk.value.seq) || 0) + 1 };
}
export const scrollAsk = signal<ScrollAsk | null>(null);
export function askScroll(rowKey: string): void {
  scrollAsk.value = { key: rowKey, seq: ((scrollAsk.value && scrollAsk.value.seq) || 0) + 1 };
}

// ---- the list's own operations -----------------------------------------
export function updateRow(fn: (row: Row) => Row): void {
  rows.value = rows.value.map(fn);
}

export function patchPanel(key: string, patch: Partial<Panel> | ((panel: Panel) => Partial<Panel>)): void {
  updateRow((row) => (row.panels.some((p) => p.key === key)
    ? { ...row, panels: row.panels.map((p) => (p.key === key ? { ...p, ...(typeof patch === 'function' ? patch(p) : patch) } : p)) }
    : row));
}

export const panelsOf = (row: Row): Panel[] => row.panels;
export function allPanels(): { row: Row; panel: Panel }[] {
  return rows.value.flatMap((row) => row.panels.map((panel) => ({ row, panel })));
}
export function panelByKey(key: string): Panel | undefined {
  return rows.value.flatMap((row) => row.panels).find((p) => p.key === key);
}

// toggleRow opens or shuts one row. Which rows are open is in the address, and
// the rows a person opens are the ones a re-reading keeps.
export function toggleRow(key: string): void {
  const next = new Set(openKeys.value);
  const row = rows.value.find((r) => r.key === key);
  if (!row) return;
  if (next.has(key)) next.delete(key); else next.add(key);
  openKeys.value = next;
  updateRow((r) => (r.key === key ? { ...r, open: next.has(key) } : r));
}

export function closeAllConfirm(): void {
  updateRow((row) => ({ ...row, panels: row.panels.map((p) => (p.confirming ? { ...p, confirming: false } : p)) }));
}

// newRow is a row a person is adding: it opens at once, and its panel starts
// from the kind the toolbar would give it.
export function addRow(file: Place, group: string | undefined): Row {
  const key = newKey();
  const panel = panelOf({
    file, label: labelOf(file), otherLabel: labelOf(otherOf(file)), path: pathFor(file, group),
    saved: false, name: '', kind: 'claude', args: ['--model', ''], env: [], argsJSON: '[]', envJSON: '[]',
    savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]', hash: '', users: 0, renameUsers: 0, err: 0, warn: 0,
    shadowed: false, both: false, other: otherOf(file), savedLine: false,
  }, group, key);
  panel.layerSwitch = true;
  const row: Row = {
    key, name: '', group: group || '', kind: 'claude', kindClass: classOf('claude'), args: '', env: [], more: 0,
    users: 0, roles: '', err: 0, warn: 0, chips: [], places: [], isNew: true, open: true, panels: [panel],
  };
  const next = new Set(openKeys.value);
  next.add(key);
  openKeys.value = next;
  const list = rows.value.slice();
  const same = list.findIndex((r) => (r.group || '') === (group || ''));
  const at = same >= 0 ? same + list.filter((r) => (r.group || '') === (group || '')).length : list.length;
  list.splice(at, 0, row);
  rows.value = list;
  return row;
}

// duplicateRow adds a copy of one panel's saved profile under a name that is
// not in the list yet.
export function duplicateRow(panel: Panel): Row | null {
  const saved = panel.saved;
  if (!saved) return null;
  const names = rows.value.map((r) => r.name);
  let suffix = 2;
  while (names.includes(`${saved.name}-${suffix}`)) suffix++;
  const list = rows.value.slice();
  const at = list.findIndex((r) => r.panels.some((p) => p.key === panel.key));
  const row: Row = {
    key: newKey(), name: `${saved.name}-${suffix}`, group: panel.group, kind: saved.kind, kindClass: classOf(saved.kind),
    args: saved.args.join(' '), env: saved.env.map((pair) => pair.value), more: 0, users: 0, roles: '', err: 0, warn: 0,
    chips: [], places: [], isNew: true, open: true,
    panels: [panelOf({
      file: panel.file, label: labelOf(panel.file), otherLabel: labelOf(otherOf(panel.file)), path: pathFor(panel.file, panel.group),
      saved: false, name: `${saved.name}-${suffix}`, kind: saved.kind, args: saved.args.slice(),
      env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })),
      argsJSON: '[]', envJSON: '[]', savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]',
      hash: '', users: 0, renameUsers: 0, err: 0, warn: 0,
      shadowed: false, both: false, other: otherOf(panel.file), savedLine: false,
    }, panel.group, `${saved.name}-${suffix}`)],
  };
  row.panels[0].key = row.key + ':' + panel.file;
  row.panels[0].rowKey = row.key;
  const next = new Set(openKeys.value);
  next.add(row.key);
  openKeys.value = next;
  list.splice(at + 1, 0, row);
  rows.value = list;
  return row;
}

// copyPanel adds the other layer's panel to a row: グローバルにコピー and
// プロジェクトにコピー, which save as a new file on that side.
export function copyPanel(panel: Panel): Panel | null {
  const saved = panel.saved;
  if (!saved) return null;
  const file: Place = otherOf(panel.file);
  const fresh = panelOf({
    file, label: labelOf(file), otherLabel: labelOf(panel.file), path: pathFor(file, panel.group),
    saved: false, name: saved.name, kind: saved.kind, args: saved.args.slice(),
    env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })),
    argsJSON: '[]', envJSON: '[]', savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]',
    hash: '', users: 0, renameUsers: 0, err: 0, warn: 0,
    shadowed: false, both: false, other: otherOf(file), savedLine: false,
  }, panel.group, panel.rowKey);
  fresh.key = panel.rowKey + ':' + file;
  updateRow((row) => (row.key === panel.rowKey ? { ...row, panels: [...row.panels, fresh].sort((a, b) => (a.file === 'project' ? 0 : 1) - (b.file === 'project' ? 0 : 1)) } : row));
  return fresh;
}

export function discardPanel(key: string): void {
  const row = rows.value.find((r) => r.panels.some((p) => p.key === key));
  if (!row) return;
  if (row.panels.length > 1) {
    updateRow((r) => (r.key === row.key ? { ...r, panels: r.panels.filter((p) => p.key !== key) } : r));
    return;
  }
  const next = new Set(openKeys.value);
  next.delete(row.key);
  openKeys.value = next;
  rows.value = rows.value.filter((r) => r.key !== row.key);
}

// ---- asking the server --------------------------------------------------
// checkers are one resource per panel: a check is a reading like any other, so
// the newest request is the one whose answer is kept and the one before it is
// dropped (5 章 3). The panel keeps no numbering of its own.
type CheckRequest = Schemas['ProfileCheckRequest'];
type CheckAnswer = { json: ProfileCheck | null };

const checkers = new Map<string, Reading<CheckAnswer, [CheckRequest]>>();
function checkerFor(key: string): Reading<CheckAnswer, [CheckRequest]> {
  let one = checkers.get(key);
  if (one) return one;
  one = resource<CheckAnswer, [CheckRequest]>(async (signal, payload) => {
    try {
      const answer = await apiCheck('profile', payload, { signal });
      return { json: answer.json as ProfileCheck | null };
    } catch (e) {
      // A refusal about the draft still carries what is wrong with it.
      const refusal = e as { json?: ProfileCheck | null } | null;
      if (refusal && refusal.json) return { json: refusal.json };
      throw e;
    }
  });
  checkers.set(key, one);
  return one;
}

// check asks what is wrong with one panel's draft. A panel that is shut keeps
// its last answer, and a check a newer one replaced answers nothing.
export async function checkPanel(key: string): Promise<void> {
  const panel = panelByKey(key);
  if (!panel) return;
  const answer = await checkerFor(key).run({
    place: panel.file, original: panel.saved ? panel.saved.name : '', project: projectOf(panel), doc: panel.draft,
  });
  const problems: Problem[] = (answer && answer.json && answer.json.problems) || [];
  if (!panelByKey(key)) return;
  patchPanel(key, { problems, errors: problems.filter((p) => p.sev === 'err').length });
}

// projectOf is the listed project a panel writes to: the group the row stands
// in, and nothing for the global layer or this serve's own project (whose files
// the API writes without being told).
export function projectOf(panel: Panel): string | undefined {
  const group = panel.group || '';
  if (!group || group === 'global' || group === CURRENT) return undefined;
  return group;
}

// savePanel writes one panel. The answer carries the project's list, which
// stands in place of the rows: the counts move with it (spec-boundary 5).
export async function savePanel(key: string): Promise<void> {
  const panel = panelByKey(key);
  if (!panel) return;
  const draft = panel.draft;
  patchPanel(key, { refusal: '' });
  try {
    const answer = await apiSave('profile', panel.saved ? panel.saved.name : (draft.name || '_'), {
      place: panel.file,
      original: panel.saved ? panel.saved.name : '',
      project: projectOf(panel),
      doc: draft,
      rename_users: !!panel.rename,
      hash: panel.hash || hashOf(panel.file),
    });
    applySaved(panel, answer.json);
  } catch (e) {
    patchPanel(key, { refusal: problemText(e, '保存できませんでした') });
  }
}

// deletePanel takes one document away, after the person pressed 削除する.
export async function deletePanel(key: string): Promise<void> {
  const panel = panelByKey(key);
  if (!panel || !panel.saved) return;
  patchPanel(key, { refusal: '', confirming: false });
  try {
    const answer = await apiDel('profile', panel.saved.name, panel.file, panel.hash || hashOf(panel.file), undefined, projectOf(panel));
    applySaved(panel, answer.json);
  } catch (e) {
    patchPanel(key, { refusal: problemText(e, '削除できませんでした') });
  }
}

// SavedList is what a save or a delete answered with, read for the one thing
// the screen takes from it: the rows that stand again and the files' hashes. A
// save answers the document and the list under it; a delete answers the list.
type SavedList = {
  items?: Schemas['ProfileItem'][] | null;
  hashes?: Record<string, string> | null;
  files?: Schemas['File'][] | null;
};

// applySaved puts the list a save (or a delete) answered with in place of the
// rows, keeping open the rows a person had open and opening the one just
// written by its new name.
function applySaved(panel: Panel, answer: SavedList | null): void {
  const list: ProfileList = {
    items: (answer && answer.items) || null,
    hashes: (answer && answer.hashes) || {},
    files: (answer && answer.files) || [],
  };
  const model = modelOf(list, panel.group);
  if (!GROUPED && answer && answer.hashes) {
    hashes.value = { project: answer.hashes.project || hashes.value.project, global: answer.hashes.global || hashes.value.global };
  }
  const order = GROUPED ? groupOrder() : [''];
  const byGroup = new Map<string, Row[]>();
  rows.value.forEach((row) => {
    const key = row.group || '';
    if (!byGroup.has(key)) byGroup.set(key, []);
    const group = byGroup.get(key);
    if (group) group.push(row);
  });
  byGroup.set(panel.group || '', model.rows.map(rowOf));
  const next = order.flatMap((key) => byGroup.get(key) || []);
  const open = new Set(openKeys.value);
  const written = panel.saved ? panel.draft.name : panel.draft.name;
  const found = next.find((row) => row.name === written && (row.group || '') === (panel.group || ''));
  if (found) open.add(found.key);
  else if (panel.saved) open.add((panel.draft.name || '') + '|' + (panel.group || ''));
  openKeys.value = open;
  rows.value = next.map((row) => ({ ...row, open: open.has(row.key) }));
  // A rename can split one row into two; the counts and the address follow.
  if (TAB === 'profiles') syncURL();
  tellDirty();
}

// ---- the address the screen writes --------------------------------------
export function syncURL(): void {
  const params = new URLSearchParams();
  if (filterKind.value) params.set('kind', filterKind.value);
  if (query.value) params.set('q', query.value);
  const open = rows.value
    .filter((row) => row.open && !row.isNew)
    .map((row) => (GROUPED && row.group ? hqToken(row.name, row.group) : row.name))
    .filter(Boolean);
  if (open.length) params.set('open', open.join(','));
  const search = params.toString();
  history.replaceState(null, '', location.pathname + (search ? '?' + search : ''));
}

// ---- the filter ---------------------------------------------------------
export const shownRows = computed(() => rows.value.filter((row) => {
  const kind = filterKind.value;
  const q = query.value;
  return (!kind || row.kind === kind) && (!q || (row.name || '').includes(q));
}));

// chips are the toolbar's filters: すべて and every kind the list holds, in the
// toolbar's order (render.go's splitKinds, then the other kinds).
export const chips = computed(() => {
  const counts: Record<string, number> = {};
  rows.value.forEach((row) => { counts[row.kind] = (counts[row.kind] || 0) + 1; });
  const out: { name: string; count: number; kindClass: string }[] = [{ name: '', count: rows.value.length, kindClass: '' }];
  [...BASE_KINDS, ...otherKinds().filter((k) => !BASE_KINDS.includes(k))].forEach((kind) => {
    if (counts[kind]) out.push({ name: kind, count: counts[kind], kindClass: classOf(kind) });
  });
  Object.keys(counts).filter((k) => !out.some((c) => c.name === k)).sort().forEach((kind) => {
    out.push({ name: kind, count: counts[kind], kindClass: classOf(kind) });
  });
  return out;
});

// ---- the band's report and the draft it carries -------------------------
// dirty is whether the screen holds something no save wrote yet (spec-nav 1.5).
export function dirty(): boolean {
  const panels = allPanels().some(({ panel }) => isDirtyPanel(panel));
  const ytexts = yamlTexts().some((one) => one.text !== one.base);
  const picks = assignTeams().some((team) => (team.rows || []).some((row) => pickChanged(row)));
  return panels || ytexts || picks;
}
export function tellDirty(): void {
  setDirty(dirty());
}

// take is the draft the band puts aside: every panel that is not what the file
// holds, every YAML text that was typed, every pick that was made — each with
// the hash of the file it was read from (spec-nav 1.5).
export function take(): Carried {
  const out: Required<Carried> = { rows: [], ytexts: [], picks: [] };
  allPanels().forEach(({ panel }) => {
    if (panel.saved && changedOf(panel).count === 0) return;
    out.rows.push({
      file: panel.file,
      saved: panel.saved ? panel.saved.name : null,
      hash: hashOf(panel.file),
      values: { name: panel.draft.name, kind: panel.draft.kind, args: panel.draft.args.slice(), env: panel.draft.env.map((pair) => ({ ...pair })) },
    });
  });
  yamlTexts().forEach((one) => {
    if (one.text === one.base) return;
    out.ytexts.push({ file: one.file, hash: one.hash, text: one.text });
  });
  assignTeams().forEach((team) => {
    (team.rows || []).forEach((row) => {
      if (!pickChanged(row)) return;
      out.picks.push({ team: team.name, hash: team.hash, key: row.key, profile: row.profile });
    });
  });
  return out;
}

// ---- the assign tab -----------------------------------------------------
// The cards come from the island's own reading of the team definitions; the
// picks a person makes are this screen's.
export const assign = signal<Assign>({ empty: false, teams: [] });
export function assignTeams(): Team[] { return assign.value.teams || []; }
export const pickChanged = (row: AssignRow): boolean => row.profile !== (row.saved || '');
export const assignKind = signal<string | null>(params().get('kind'));

export function initAssign(): void {
  const card = (ISLAND.assign || {}) as Partial<{ empty: boolean; teams: IslandTeam[] }>;
  assign.value = {
    empty: !!card.empty,
    teams: (card.teams || []).map((team): Team => ({
      name: team.name,
      // The place the card's save writes: a global definition is saved in the
      // global file, and every project reading it changes (d-0f32, r-f8b9).
      place: placeOf(team.place || 'project'),
      hash: team.hash,
      path: team.path,
      members: team.members || 0,
      workers: team.workers || 0,
      err: team.err || 0,
      saved: !!team.saved,
      refusal: '',
      rows: (team.rows || []).map((row: IslandRow): AssignRow => ({
        key: row.id,
        kind: row.kind,
        name: row.name,
        role: row.role,
        profile: row.profile,
        saved: row.profile,
        options: row.options || [],
        missing: !!row.missing,
        error: row.error || '',
        kindCls: row.kindCls || '',
      })),
    })),
  };
}

export function setPick(teamName: string, key: string, profile: string): void {
  assign.value = {
    ...assign.value,
    teams: assign.value.teams.map((team) => (team.name === teamName
      ? { ...team, saved: false, rows: team.rows.map((row) => (row.key === key ? { ...row, profile, missing: false, error: '' } : row)) }
      : team)),
  };
}

export function teamDirty(team: Team): number {
  return (team.rows || []).filter(pickChanged).length;
}
export function teamMissing(team: Team): number {
  return (team.rows || []).filter((row) => row.missing).length;
}

export async function saveTeam(name: string): Promise<boolean> {
  const team = assignTeams().find((t) => t.name === name);
  if (!team) return false;
  const ops = (team.rows || []).filter(pickChanged).map((row) => (row.key.startsWith('m')
    ? { kind: 'memberProfile', member: row.name, profile: row.profile }
    : { kind: 'workerProfile', from: row.role, purpose: row.name, profile: row.profile }));
  try {
    const answer = await apiSave('team', team.name, { place: team.place || 'project', doc: { ops }, hash: team.hash });
    const json = answer.json;
    assign.value = {
      ...assign.value,
      teams: assign.value.teams.map((t) => (t.name === name
        ? { ...t, hash: (json && json.hash) || t.hash, saved: true, rows: t.rows.map((row) => ({ ...row, saved: row.profile })) }
        : t)),
    };
    return true;
  } catch (e) {
    const text = problemText(e, '保存できませんでした');
    assign.value = { ...assign.value, teams: assign.value.teams.map((t) => (t.name === name ? { ...t, refusal: text } : t)) };
    return false;
  }
}

export function syncAssignURL(): void {
  const params = new URLSearchParams(location.search);
  if (assignKind.value) params.set('kind', assignKind.value); else params.delete('kind');
  const search = params.toString();
  // The hash names the person a chip sent the cursor to (P11): it stays.
  history.replaceState(null, '', location.pathname + (search ? '?' + search : '') + location.hash);
}

// ---- the YAML tab -------------------------------------------------------
// Each file's section is the shared YAML part (lib/yamltab.js); what this
// screen keeps is the text each file was read with, so the band knows what was
// typed and the carry can put it back (spec-nav 1.5).
export const yamlFiles = signal<YamlFile[]>([]);
export const yamlState = signal(new Map<string, YamlOne>());
export function yamlTexts(): { file: string; hash: string; base: string; text: string }[] {
  return yamlFiles.value.map((file) => {
    const one = yamlState.value.get(file.file) || {};
    return { file: file.file, hash: file.hash, base: one.base || file.text || '', text: one.text === undefined ? (file.text || '') : one.text };
  });
}
export function initYaml(): void {
  yamlFiles.value = (files.value || []).map((file) => ({ ...file }));
}
export function tellYaml(file: string, one: YamlOne): void {
  const next = new Map(yamlState.value);
  next.set(file, one);
  yamlState.value = next;
}

// applyCarried puts the band's draft back, skipping every part whose file moved
// outside since the draft was made (spec-nav 1.5).
export function restore(carried: Carried): void {
  (carried.rows || []).forEach((c) => {
    if (c.hash && c.hash !== hashOf(placeOf(c.file))) return;
    const found = allPanels().find(({ panel }) => panel.file === placeOf(c.file) && (panel.saved ? panel.saved.name : null) === c.saved);
    if (found) {
      patchPanel(found.panel.key, { draft: { name: c.values.name, kind: c.values.kind, args: c.values.args.slice(), env: c.values.env.map((pair) => ({ ...pair })) } });
      return;
    }
    if (c.saved) return;
    const row = addRow(placeOf(c.file), CURRENT);
    patchPanel(row.panels[0].key, { draft: { name: c.values.name, kind: c.values.kind, args: c.values.args.slice(), env: c.values.env.map((pair) => ({ ...pair })) } });
  });
  (carried.ytexts || []).forEach((c) => {
    const file = yamlFiles.value.find((f) => f.file === c.file);
    if (!file || (c.hash && c.hash !== file.hash)) return;
    tellYaml(c.file, { ...(yamlState.value.get(c.file) || {}), text: c.text, carried: c.text });
  });
  (carried.picks || []).forEach((c) => {
    const team = assignTeams().find((t) => t.name === c.team);
    if (!team || (c.hash && c.hash !== team.hash)) return;
    const row = (team.rows || []).find((r) => r.key === c.key);
    if (!row) return;
    if (!(row.options || []).some((g) => (g.names || []).includes(c.profile))) return;
    setPick(c.team, c.key, c.profile);
  });
  if (TAB === 'profiles') { syncURL(); tellDirty(); } else tellDirty();
}

export { modelOf, rowOf, panelOf, apiList };
