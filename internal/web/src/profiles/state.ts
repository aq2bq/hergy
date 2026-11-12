// profiles/state.ts — プロファイルの画面の state（webui-preact-design 4 章・5 章）。
//
// サーバーの答えはここだけに入る（5 章 2）。部品（view.tsx）は signal を読んで
// 描くだけで、捕まえておいた要素に書き込まない。画面は 3 つのタブを持つ:
// 一覧（プロファイルの行と、行の中のプロジェクト/グローバルの面）、
// 割り当て（チーム定義のメンバーのプロファイル）、YAML（profiles.yaml の面）。
import { signal, computed } from '../lib/preact.js';
import { editProfile, editProfileAssignments, editYaml, profileFiles, type ProfileAssignments, type YamlEditing } from '../lib/editing.js';
import { resourceIdentity } from '../lib/resource-identity.js';
import { profileModel, list as apiList, type Answer, type Place, type ProfilePanel, type ProfileRow } from '../lib/api.js';
import { readOpen, rowsFor, writeOpen } from '../lib/address.js';
import { copyGroupOf } from '../lib/copy';
import { assignDirty } from '../lib/changed.js';
import { setDirty } from '../lib/nav.js';
import type {
  Assign, AssignRow, Carried, Draft, EnvPair, FocusAsk, Island,
  IslandHQ, HQProfileGroup, IslandRow, OpenTarget, Panel,
  ProfileList, Row, Team, YamlFile,
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

// envList is a JSON array of key/value pairs, read the way stringList reads
// names: a value that is not one reads as no pair.
function envList(raw: string | undefined): EnvPair[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((one): one is EnvPair => typeof one === 'object' && one !== null && 'key' in one && 'value' in one && typeof one.key === 'string' && typeof one.value === 'string')
      : [];
  } catch (e) { return []; }
}

// The page carries the kind answers in the boundary's shape (spec-boundary 1,
// 2, 5): the profile list and, on the 割り当て tab, the assignment cards. The
// rows, the panels and the YAML sections are drawn from them here, and the Go
// templates draw none of them.
export const ISLAND: Island = (() => {
  const el = document.getElementById('profiles-data');
  if (!el) return {};
  // Boundary: the page's embedded JSON is read here and nowhere else.
  try { return JSON.parse(el.textContent || '{}') as Island; } catch (e) { return {}; }
})();

export const TAB = view.dataset.tab || '';
export const BASE_KINDS = ['claude', 'codex', 'opencode', 'gemini'];
export const otherKinds = (): string[] => stringList(view.dataset.otherKinds);
export const classOf = (kind: string): string => (BASE_KINDS.includes(kind) ? 'k-' + kind : '');
// PLACES are the two layers a document can stand in, in the order the screen
// draws them.
export const PLACES: readonly Place[] = ['project', 'global'];
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
export const CURRENT = HQ ? ((HQ.projects && HQ.projects.current && HQ.projects.current.path) || '') : '';
export const groupKeyOf = (g: HQProfileGroup): string => (g.global ? 'global' : (g.project ? g.project.path : ''));
const groupNameOf = (g: HQProfileGroup): string => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const GROUPS: HQProfileGroup[] | null = HQ ? (HQ.profiles || []) : null;

// PlaceGroup is one place the list draws a head for (r-410c, d-e77c): the
// project's own place (its key is "" without projects: listed), the global
// layer's, or another listed project's. The one-project headquarters' groups
// are read with the same shape.
export type PlaceGroup = { key: string; global: boolean; name: string; display: string };
export const placeGroups = (): PlaceGroup[] => {
  if (HQ && HQ.profiles && HQ.profiles.length) {
    return HQ.profiles.map((g) => ({
      key: GROUPED ? groupKeyOf(g) : (g.global ? 'global' : ''),
      global: !!g.global,
      name: groupNameOf(g),
      display: g.display || '',
    }));
  }
  return [
    { key: '', global: false, name: currentProjectName(), display: pathOf('project') },
    { key: 'global', global: true, name: 'グローバル', display: pathOf('global') },
  ];
};
const groupOrder = (): string[] => placeGroups().map((g) => g.key);
const groupRank = (group: string): number => {
  const at = groupOrder().indexOf(group);
  return at < 0 ? groupOrder().length : at;
};
export const groupName = (key: string): string => {
  if (key === 'global') return 'グローバル';
  if (!key || key === CURRENT) {
    if (!HQ) return 'プロジェクト';
    const p = HQ.projects;
    if (p.mode === 'current') return (p.current && p.current.name) || 'プロジェクト';
    const found = (p.projects || []).find((x) => x.path === CURRENT);
    return found ? found.name : ((p.current && p.current.name) || 'プロジェクト');
  }
  const g = (GROUPS || []).find((x) => groupKeyOf(x) === key);
  return g ? groupNameOf(g) : String(key).split('/').filter(Boolean).pop() || '';
};
// currentProjectName is the name the 置き場所 column gives this serve's
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
  if (!group || group === CURRENT) return pathOf(file);
  if (group === 'global') return '~/.config/hergy/profiles.yaml';
  const g = (GROUPS || []).find((x) => groupKeyOf(x) === group);
  return `${(g && g.project && g.project.display) || ''}/.hergy/profiles.yaml`;
};
// profiles.yaml bases are owned by editing, keyed by project and place.
const fileBases = profileFiles();
const identityOf = (file: Place, group = '', name?: string) => resourceIdentity('profile', { place: file, project: group || CURRENT, name });
fileBases.learn(identityOf('project'), view.dataset.hashProject || '');
fileBases.learn(identityOf('global'), view.dataset.hashShared || '');
export const fileHashOf = (group: string | undefined, file: Place): string => fileBases.hash(identityOf(file, group));
function learnFiles(group: string, list: ProfileList | null): void {
  if (!list) return;
  for (const place of PLACES) fileBases.learn(identityOf(place, group), list.hashes?.[place] || '');
  for (const one of list.files || []) fileBases.learn(identityOf(placeOf(one.place), group), one.hash);
}

// ---- the address --------------------------------------------------------
// The list carries no screen state (spec-boundary 5): the open rows, the kind
// and the search stand in the address, and the address is where the screen
// starts from.
const params = (): URLSearchParams => new URLSearchParams(location.search);
const OPEN_ROWS: OpenTarget[] = readOpen(params());
// rowRef reads a row the way the address writes it (⑥-2, d-a143).
const rowRef = (row: { name: string; group?: string }): { name: string; group: string } => ({
  name: row.name || '', group: GROUPED ? row.group || '' : row.group === 'global' ? 'global' : 'project',
});
const here = GROUPED ? CURRENT : '';
const isOpenRow = (row: { name: string; group?: string }): boolean =>
  OPEN_ROWS.some((t) => rowsFor({ name: t.name, group: t.group || '' }, [row], rowRef, here).length > 0);


export const filterKind = signal(TAB === 'profiles' ? (params().get('kind') || '') : '');
export const query = signal(TAB === 'profiles' ? (params().get('q') || '') : '');
// The key a row is opened by: its name and the group the rows draw in (the
// project's own key is '', the global layer's 'global' on the one-project screen).
const keyGroup = (group: string): string => (GROUPED ? group : group === 'global' ? 'global' : '');
export const openKeys = signal(new Set<string>(OPEN_ROWS.map((t) => t.name + '|' + keyGroup(t.group || ''))));

// ---- the list's rows ----------------------------------------------------
let nextId = 1;
const newKey = (): string => 'new-' + nextId++;

// panelOf reads one server panel as the panel's state: what the file holds
// (saved), and the draft the fields start from.
function panelOf(p: ProfilePanel, group: string | undefined, rowKey: string): Panel {
  const identity = identityOf(p.file, group, p.saved ? p.savedName : undefined);
  const editing = editProfile(identity, {
    saved: p.saved && identity.name ? { name: identity.name, kind: p.savedKind, args: stringList(p.savedArgs), env: envList(p.savedEnv) } : null,
    draft: { name: p.name || '', kind: p.kind || 'claude', args: (p.args || []).slice(), env: (p.env || []).map((pair) => ({ ...pair })) },
    hash: p.hash || fileBases.hash(identity), readable: !p.problem, savedLine: p.savedLine,
  });
  return {
    key: rowKey + ':' + p.file, rowKey, file: p.file, group: group || '',
    path: p.path || pathFor(p.file, group), label: p.label, otherLabel: p.otherLabel,
    users: p.users || 0, rename_targets: p.rename_targets || 0,
    problem: p.problem || '', mark: p.mark || '', layerSwitch: false, editing,
    get saved() { return editing.read.value.saved; },
    get draft() { return editing.read.value.draft; },
    get born() { return editing.read.value.born; },
    get hash() { return editing.read.value.hash; },
    get problems() { return editing.read.value.problems; },
    get baseProblems() { return editing.read.value.baseProblems; },
    get errors() { return editing.read.value.problems.filter((p) => p.sev === 'err').length; },
    get refusal() { return editing.read.value.refusal; },
    get conflict() { return editing.read.value.conflict; },
    get rename() { return editing.read.value.rename; },
    get savedLine() { return editing.read.value.savedLine; },
  };
}

// rowOf reads one row of the model: its summary columns, and the panels it
// opens into.
function rowOf(row: ProfileRow & { group?: string }, index: number): Row {
  const key = (row.name || '') + '|' + (row.group || '') + (row.name ? '' : '|' + index);
  return {
    key,
    name: row.name || '',
    mark: row.mark || '',
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
    open: openKeys.value.has(key) || isOpenRow(row),
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
  // The answer already holds one place per row (r-ea17): a name both files hold
  // is two items, one under each head, so this copies each item as a row.
  // Each row is one place: a global row stands under the global head, and a
  // project row under the group the answer was asked for (a listed project's
  // key, or this project's ""). A save in the global row does not move the
  // project rows to the global head.
  const base = group && group !== 'global' ? group : '';
  const rows: ModelRow[] = model.rows.map((row) => {
    const own = (row.panels || [])[0]?.file === 'global' ? 'global' : base;
    return { ...row, group: own, open: isOpenRow({ ...row, group: own }) };
  });
  return { rows, files: model.files };
}

export const rows = signal<Row[]>([]);
export const files = signal<YamlFile[]>([]);

function readInitial(): { rows: Row[]; files: YamlFile[] } {
  if (GROUPED && HQ && HQ.profiles && HQ.profiles.length) {
    const all: ModelRow[] = [];
    (HQ.profiles || []).forEach((g) => {
      const key = GROUPED ? groupKeyOf(g) : (g.global ? 'global' : '');
      learnFiles(key, g.list || null);
      all.push(...modelOf(g.list || null, key).rows);
    });
    // The island's own list is this serve's project's: a write to its group
    // carries the hashes that list read (r-57d8).
    learnFiles(CURRENT, ISLAND.profile || null);
    return { rows: all.map(rowOf), files: modelOf(ISLAND.profile || null).files || [] };
  }
  learnFiles('', ISLAND.profile || null);
  const model = modelOf(ISLAND.profile || null);
  return { rows: model.rows.map(rowOf), files: model.files || [] };
}
{
  const first = readInitial();
  rows.value = first.rows;
  files.value = first.files;
}

// ---- what a panel holds, and how far it is from the file ----------------
// panelFlags is which fields changed, for the screens to draw: the owner's
// answer, not a second comparison (r-1e9a).
export const panelFlags = (panel: Panel) => panel.editing.flags.value;
export const profileChanges = (panel: Panel) => panel.editing.changes.value;
export const profileDirty = (panel: Panel): boolean => panel.editing.dirty.value;
export const rowUnsaved = (row: Row): boolean => row.panels.some(profileDirty);
export const saveBlocked = (panel: Panel): boolean =>
  panel.errors > 0 || (panel.saved ? !profileDirty(panel) : false) || panel.draft.name === '';

// ---- what the screen asks the page to do after a press -------------------
// Focus is put only where a person just pressed (webui-preact-design 5 章 5):
// ＋引数 puts the caret in the new argument, ＋環境変数 in the new key, 複製 in
// the name of the copy. The ask is state, and the panel that owns the field
// reads it after the draw.
export const focusAsk = signal<FocusAsk | null>(null);
export function askFocus(key: string, field: string, index: number, select: boolean, scroll?: boolean): void {
  focusAsk.value = { key, field, index: index || 0, select: !!select, scroll: !!scroll, seq: ((focusAsk.value && focusAsk.value.seq) || 0) + 1 };
}

// ---- the list's own operations -----------------------------------------
export function updateRow(fn: (row: Row) => Row): void {
  rows.value = rows.value.map(fn);
}

export function changeDraft(key: string, change: Draft | ((draft: Draft) => Draft)): void {
  panelByKey(key)?.editing.change(change);
}
export function renamePanel(key: string, on: boolean): void { panelByKey(key)?.editing.renameUsers(on); }

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

// closeRow shuts one row: a copy closes the row it was pressed in, so the copy
// under the other place's head is the one open row a person reads (d-022a).
export function closeRow(key: string): void {
  const next = new Set(openKeys.value);
  next.delete(key);
  openKeys.value = next;
  updateRow((r) => (r.key === key ? { ...r, open: false } : r));
}

export function closeAllConfirm(): void {
  updateRow((row) => ({ ...row, panels: row.panels.map((p) => p) }));
}

// newRow is a row a person is adding under one place's head: it opens at once,
// and its panel starts from the kind the head would give it (r-410c).

export function addRow(file: Place, group: string | undefined): Row {
  const key = newKey();
  const g = group || (file === 'global' ? 'global' : '');
  const panel = panelOf({
    file, label: labelOf(file), otherLabel: labelOf(otherOf(file)), path: pathFor(file, g),
    saved: false, name: '', kind: 'claude', args: ['--model', ''], env: [], argsJSON: '[]', envJSON: '[]',
    savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]', hash: '', users: 0, rename_targets: 0, err: 0, warn: 0,
    other: otherOf(file), savedLine: false, problem: '',
  }, g, key);
  const row: Row = {
    key, name: '', group: g, kind: 'claude', kindClass: classOf('claude'), args: '', env: [], more: 0,
    mark: '',
    users: 0, roles: '', err: 0, warn: 0, chips: [], places: [], isNew: true, open: true, panels: [panel],
  };
  const next = new Set(openKeys.value);
  next.add(key);
  openKeys.value = next;
  // The row stands at the top of the head it was added under.
  const rank = groupRank(g);
  let at = rows.value.findIndex((r) => (r.group || '') === g);
  if (at < 0) {
    at = rows.value.findIndex((r) => groupRank(r.group || '') > rank);
    if (at < 0) at = rows.value.length;
  }
  rows.value = [...rows.value.slice(0, at), row, ...rows.value.slice(at)];
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
    key: newKey(), name: `${saved.name}-${suffix}`, group: panel.group, kind: saved.kind, kindClass: classOf(saved.kind), mark: '',
    args: saved.args.join(' '), env: saved.env.map((pair) => pair.value), more: 0, users: 0, roles: '', err: 0, warn: 0,
    chips: [], places: [], isNew: true, open: true,
    panels: [panelOf({
      file: panel.file, label: labelOf(panel.file), otherLabel: labelOf(otherOf(panel.file)), path: pathFor(panel.file, panel.group),
      saved: false, name: `${saved.name}-${suffix}`, kind: saved.kind, args: saved.args.slice(),
      env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })),
      argsJSON: '[]', envJSON: '[]', savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]',
      hash: '', users: 0, rename_targets: 0, err: 0, warn: 0,
      other: otherOf(panel.file), savedLine: false, problem: '',
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

// copyPanel adds the other place's panel as a new row under its head: グローバル
// にコピー and プロジェクトにコピー save as a new file on that side, and the copy
// stands in the place it was copied to (r-f781, r-410c).
export function copyPanel(panel: Panel): Panel | null {
  const saved = panel.saved;
  if (!saved) return null;
  const file: Place = otherOf(panel.file);
  const group = copyGroupOf(file, panel.group, CURRENT);
  const fresh = panelOf({
    file, label: labelOf(file), otherLabel: labelOf(panel.file), path: pathFor(file, group),
    saved: false, name: saved.name, kind: saved.kind, args: saved.args.slice(),
    env: saved.env.map((pair) => ({ key: pair.key, value: pair.value })),
    argsJSON: '[]', envJSON: '[]', savedName: '', savedKind: '', savedArgs: '[]', savedEnv: '[]',
    hash: '', users: 0, rename_targets: 0, err: 0, warn: 0,
    other: otherOf(file), savedLine: false, problem: '',
  }, group, panel.rowKey);
  const row: Row = {
    key: newKey(), name: '', group, kind: saved.kind, kindClass: classOf(saved.kind), mark: '',
    args: saved.args.join(' '), env: saved.env.map((pair) => pair.value), more: 0, users: 0, roles: '', err: 0, warn: 0,
    chips: [], places: [], isNew: true, open: true, panels: [fresh],
  };
  fresh.rowKey = row.key;
  fresh.key = row.key + ':' + file;
  const next = new Set(openKeys.value);
  next.add(row.key);
  openKeys.value = next;
  // The copy stands at the top of the head it was copied to.
  const rank = groupRank(group);
  let at = rows.value.findIndex((r) => (r.group || '') === group);
  if (at < 0) {
    at = rows.value.findIndex((r) => groupRank(r.group || '') > rank);
    if (at < 0) at = rows.value.length;
  }
  rows.value = [...rows.value.slice(0, at), row, ...rows.value.slice(at)];
  return fresh;
}

// canCopy is whether a panel can be copied into the other place: the other
// place must not already hold that name, in this row or in the row the copy
// would stand beside (r-f781).
export function canCopy(panel: Panel): boolean {
  if (!panel.saved) return false;
  const file = otherOf(panel.file);
  const group = copyGroupOf(file, panel.group, CURRENT);
  return !rows.value.some((row) => row.group === group && row.panels.some((p) => p.file === file && p.draft.name === panel.saved!.name));
}

export function discardPanel(key: string): void {
  const row = rows.value.find((r) => r.panels.some((p) => p.key === key));
  if (!row) return;
  panelByKey(key)?.editing.dispose();
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
export async function checkPanel(key: string): Promise<void> { await panelByKey(key)?.editing.check(); }
export const projectOf = (panel: Panel): string | undefined => identityOf(panel.file, panel.group).project;
export async function savePanel(key: string): Promise<void> {
  const panel = panelByKey(key);
  if (!panel) return;
  const answer = await panel.editing.save();
  if (answer) applySaved(panel, answer.json);
}
export function pressOverwrite(key: string): void {
  const panel = panelByKey(key);
  if (panel) panel.editing.overwrite().then((answer) => { if (answer) applySaved(panel, answer.json); });
}
export function pressRevertConflict(key: string): void { panelByKey(key)?.editing.readConflict(); }
export function pressUndoConflict(key: string): void { panelByKey(key)?.editing.undoConflict(); }

// deletePanel takes one document away, after the person pressed 削除する.
export async function deletePanel(key: string): Promise<void> {
  const panel = panelByKey(key);
  if (!panel || !panel.saved) return;
  const answer = await panel.editing.remove();
  if (answer) applySaved(panel, answer.json);
}

// SavedAnswer is what a save and a delete answer with: the document they wrote
// and the list under it (a delete writes no document).
type SavedAnswer = Answer<'profile', 'save'> | Answer<'profile', 'del'>;

// applySaved puts the list a save (or a delete) answered with in place of the
// rows, keeping open the rows a person had open and opening the one just
// written by its new name.
function applySaved(panel: Panel, answer: SavedAnswer | null): void {
  const carry = panel.editing.dirty.value ? panel.editing.take() : null;
  const previous = allPanels().map(({ panel }) => panel);
  const list: ProfileList = {
    items: (answer && answer.items) || null,
    hashes: (answer && answer.hashes) || {},
    files: (answer && answer.files) || [],
  };
  const group = panel.group || '';
  // The answer is the list of the group this write named: its files' hashes are
  // that group's, and no other group's panels stand on them (r-57d8).
  learnFiles(group, list);
  const model = modelOf(list, panel.group);
  const order = groupOrder();
  const byGroup = new Map<string, Row[]>();
  rows.value.forEach((row) => {
    const key = row.group || '';
    if (!byGroup.has(key)) byGroup.set(key, []);
    const held = byGroup.get(key);
    if (held) held.push(row);
  });
  // The answer's rows go into the frame of their own place (the item's
  // Effective): a save under one head carries the global rows too, and they
  // must stay under the global head (r-ea17).
  const answered = model.rows.map(rowOf);
  const own = new Map<string, Row[]>();
  answered.forEach((row) => {
    const key = row.group || '';
    if (!own.has(key)) own.set(key, []);
    own.get(key)?.push(row);
  });
  own.forEach((list, key) => byGroup.set(key, list));
  if (!own.has(group)) byGroup.set(group, []);
  const next = order.flatMap((key) => byGroup.get(key) || []);
  const open = new Set(openKeys.value);
  const written = panel.saved?.name || panel.draft.name;
  const found = next.find((row) => row.name === written && (row.group || '') === group);
  if (found) {
    open.add(found.key);
    const target = found.panels.find((p) => p.file === panel.file);
    if (carry && target) target.editing.restore(carry);
  }
  else if (panel.saved) open.add((panel.draft.name || '') + '|' + group);
  openKeys.value = open;
  // The answer read every file again, and a save gives the rules their ids
  // after it (d-4aab): each panel of a file the answer names stands on the hash
  // of the text that is there now, so the next save is not refused as a change
  // another place made (r-b41c). The answer names one project's files, so only
  // that group's panels take them (r-57d8).
  rows.value = next.map((row) => ({ ...row, open: open.has(row.key) }));
  const retained = new Set(allPanels().map(({ panel }) => panel.editing));
  previous.forEach((p) => { if (!retained.has(p.editing)) p.editing.dispose(); });
  // A rename can split one row into two; the counts and the address follow.
  if (TAB === 'profiles') syncURL();
  tellDirty();
}

// ---- the address the screen writes --------------------------------------
export function syncURL(): void {
  const params = new URLSearchParams();
  if (filterKind.value) params.set('kind', filterKind.value);
  if (query.value) params.set('q', query.value);
  // The row's name and the place it stands in. A screen with projects listed
  // writes the project's path (or `global`); the one-project screen writes
  // `project`/`global`, never a bare name (⑥-2, d-a143).
  const open = rows.value
    .filter((row) => row.open && !row.isNew && row.name)
    .map((row) => ({ name: row.name, group: GROUPED && row.group ? row.group : (row.group === 'global' ? 'global' : 'project') }));
  writeOpen(params, open);
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
  const panels = allPanels().some(({ panel }) => profileDirty(panel));
  const ytexts = yamlTexts().some((one) => one.editing.dirty.value);
  const picks = assignTeams().some((team) => (team.rows || []).some((row) => assignDirty({ name: row.key, saved: row.saved || '', draft: row.profile })));
  return panels || ytexts || picks;
}
export function tellDirty(): void {
  setDirty(dirty());
}

// lost names what leaving the page now would throw away: changed's words for
// each panel, pick and YAML text that differs (r-1e9a, d-021e).
export function lost(): string[] {
  const out: string[] = [];
  for (const { panel } of allPanels()) {
    if (!profileDirty(panel)) continue;
    out.push(...panel.editing.lost.value);
  }
  for (const one of yamlTexts()) {
    if (!one.editing.dirty.value) continue;
    out.push(...one.editing.lost.value);
  }
  for (const team of assignTeams()) for (const row of team.rows || []) {
    if (!assignDirty({ name: row.key, saved: row.saved || '', draft: row.profile })) continue;
    out.push(...(assignmentEdits.get(team.name)?.lost.value || []));
  }
  return [...new Set(out)];
}

// take is the draft the band puts aside: every panel that is not what the file
// holds, every YAML text that was typed, every pick that was made — each with
// the hash of the file it was read from (spec-nav 1.5).
export function take(): Carried {
  const out: Required<Carried> = { rows: [], ytexts: [], picks: [] };
  allPanels().forEach(({ panel }) => {
    if (panel.saved && !profileDirty(panel)) return;
    out.rows.push({
      file: panel.file,
      group: panel.group,
      saved: panel.saved ? panel.saved.name : null,
      // The hash is the one of the file this panel's group writes: a listed
      // project's file is not this serve's (r-57d8).
      hash: fileHashOf(panel.group, panel.file),
      values: panel.editing.take().draft,
    });
  });
  yamlTexts().forEach((one) => {
    if (one.text === one.base) return;
    out.ytexts.push({ file: one.file, hash: one.hash, text: one.text });
  });
  assignTeams().forEach((team) => {
    (team.rows || []).forEach((row) => {
      if (!assignDirty({ name: row.key, saved: row.saved || '', draft: row.profile })) return;
      out.picks.push({ team: team.name, hash: team.hash, key: row.key, profile: row.profile });
    });
  });
  return out;
}

// ---- the assign tab -----------------------------------------------------
// The cards come from the island's own reading of the team definitions; the
// picks a person makes are this screen's.
const assignmentEdits = new Map<string, ProfileAssignments>();
export const assign = signal<Assign>({ empty: false, teams: [] });
export function assignTeams(): Team[] { return assign.value.teams || []; }
export const assignKind = signal<string | null>(params().get('kind'));

export function initAssign(): void {
  const card = ISLAND.assign;
  assign.value = {
    empty: !!card && !!card.empty,
    teams: ((card && card.teams) || []).map((team): Team => {
      const editing = editProfileAssignments(resourceIdentity('team', { name: team.name, place: team.place, project: CURRENT }), (team.rows || []).map((r) => ({ key: r.id, name: r.name, role: r.role, profile: r.profile })), team.hash);
      assignmentEdits.set(team.name, editing);
      return ({
      name: team.name,
      // The place the card's save writes: a global definition is saved in the
      // global file, and every project reading it changes (d-0f32, r-f8b9).
      place: placeOf(team.place || 'project'),
      get hash() { return editing.read.value.hash; },
      path: team.path,
      members: team.members || 0,
      workers: team.workers || 0,
      err: team.err || 0,
      get saved() { return editing.read.value.savedLine || !!team.saved; },
      get refusal() { return editing.read.value.refusal; },
      rows: (team.rows || []).map((row: IslandRow): AssignRow => ({
        key: row.id,
        kind: row.kind,
        name: row.name,
        role: row.role,
        get profile() { return editing.read.value.draft.find((r) => r.key === row.id)?.profile || ''; },
        get saved() { return editing.read.value.saved?.find((r) => r.key === row.id)?.profile || ''; },
        options: row.options || [],
        get missing() { return !!row.missing && editing.read.value.draft.find((r) => r.key === row.id)?.profile === row.profile; },
        get error() { return this.missing ? row.error || '' : ''; },
        kindCls: row.kindCls || '',
      })),
    }); }),
  };
}

export function setPick(teamName: string, key: string, profile: string): void {
  assignmentEdits.get(teamName)?.change((rows) => rows.map((row) => row.key === key ? { ...row, profile } : row));
}

export function teamDirty(team: Team): number {
  return (team.rows || []).filter((row) => assignDirty({ name: row.key, saved: row.saved || '', draft: row.profile })).length;
}
export function teamMissing(team: Team): number {
  return (team.rows || []).filter((row) => row.missing).length;
}

export async function saveTeam(name: string): Promise<boolean> {
  const team = assignTeams().find((t) => t.name === name);
  if (!team) return false;
  return !!(await assignmentEdits.get(name)?.save());
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
const yamlEdits = new Map<string, YamlEditing>();
export const yamlEditing = (file: string): YamlEditing | undefined => yamlEdits.get(file);
export function yamlTexts() {
  return yamlFiles.value.flatMap((file) => {
    const editing = yamlEdits.get(file.file);
    if (!editing) return [];
    const one = editing.read.value;
    return [{ file: file.file, hash: one.hash, base: one.saved || '', text: one.draft, editing }];
  });
}
export function initYaml(): void {
  yamlEdits.forEach((one) => one.dispose());
  yamlEdits.clear();
  yamlFiles.value = (files.value || []).map((file) => {
    yamlEdits.set(file.file, editYaml('profile', { ...file, place: file.file, name: '_yaml', project: CURRENT }));
    return { ...file };
  });
}

// applyCarried puts the band's draft back, skipping every part whose file moved
// outside since the draft was made (spec-nav 1.5).
export function restore(carried: Carried): void {
  (carried.rows || []).forEach((c) => {
    // The draft's file is the one of the group it was taken from: another
    // project's file moving is not this draft's file moving (r-57d8).
    const group = c.group || '';
    if (c.hash && c.hash !== fileHashOf(group, placeOf(c.file))) return;
    const found = allPanels().find(({ panel }) => panel.file === placeOf(c.file) && (panel.group || '') === group && (panel.saved ? panel.saved.name : null) === c.saved);
    if (found) {
      found.panel.editing.restore({ hash: c.hash, draft: c.values });
      return;
    }
    if (c.saved) return;
    const row = addRow(placeOf(c.file), group || CURRENT);
    row.panels[0].editing.restore({ hash: c.hash, draft: c.values });
  });
  (carried.ytexts || []).forEach((c) => {
    const file = yamlFiles.value.find((f) => f.file === c.file);
    if (!file || (c.hash && c.hash !== file.hash)) return;
    yamlEdits.get(c.file)?.restore({ hash: c.hash, draft: c.text });
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
