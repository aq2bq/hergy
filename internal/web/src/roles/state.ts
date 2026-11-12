// roles/state.ts — role rows, navigation and read-through projections of editing.
// Resource drafts, checks, conflicts and save requests belong to lib/editing.
import { editRole, restoreRole, editRoleAssignments, type RoleCarry, type CompleteCarry, type RoleEditing, type RoleAssignments, type ReadonlyDeep } from '../lib/editing.js';
import { resourceIdentity, type ResourceIdentity } from '../lib/resource-identity.js';
import { signal } from '../lib/preact.js';
import { roleProblems } from '../lib/role-check.js';
import { takeDraft, setDirty } from '../lib/nav.js';
import { readOpen, rowsFor, writeOpen, rowAddress } from '../lib/address.js';
import { copyGroupOf, projectOfGroup } from '../lib/copy';
import {
  saved as apiSaved,
  roleModel, teamModel, bases,
} from '../lib/api.js';
import {
  fromJSON,
} from '../lib/ruledoc.js';
import type { EditableDocument, RuleIssue, RulePanel } from '../lib/ruledoc.js';
import type { RoleRow as ApiRoleRow, TeamCard as ApiTeamCard } from '../lib/api.js';
import type { components, operations } from '../../../apicontract/api-types';

export type Place = operations['getTeam']['parameters']['query']['place'];
type HQIsland = {
  projects: components['schemas']['HQProjects'];
  roles?: components['schemas']['HQRoleGroup'][];
  teams?: components['schemas']['HQTeamGroup'][];
  // Why the headquarters' lists could not be read; a screen says it (r-1db7 L1).
  error?: string;
};
type RoleIsland = {
  role?: components['schemas']['RoleList'];
  team?: components['schemas']['TeamList'];
  texts?: Record<string, string>;
  places?: Partial<Record<Place, string>>;
  hq?: HQIsland | null;
};
type RoleModel = { roles: ApiRoleRow[] };
type RoleEntry = {
  readonly editing: RoleEditing;
  readonly saved: ReadonlyDeep<EditableDocument> | null;
  readonly draft: ReadonlyDeep<EditableDocument>;
  readonly hash: string; readonly text: string; readonly problem?: string;
};
export type RoleRow = {
  uid: number;
  group: string;
  project: RoleEntry | null;
  global: RoleEntry | null;
  // mark is the owner's word for a name both layers hold (r-ea17).
  mark?: string;
  paths?: Partial<Record<Place, string>>;
  markdown?: string;
  name?: string;
  noDoc?: boolean;
  isNew?: boolean;
};
type RoleMember = { name: string; readonly saved: string; readonly draft: string; readonly changed: boolean };
type TeamCard = Omit<ApiTeamCard, 'group' | 'members' | 'hash'> & { group: string; readonly hash: string; readonly members: RoleMember[]; readonly editing: RoleAssignments };
type HqTeam = { id: string; group: string; members: Pick<RoleMember, 'name' | 'saved' | 'draft'>[] };
type RoleState = {
  tab: 'roles' | 'assign' | 'yaml';
  open: Set<number>;
  filter: 'issues' | null;
  q: string;
  roleHi: string | null;
  yfile: string | null;
  pmode: Record<string, string>;
};
type CheckState = {
  issues: RuleIssue[];
  markdown: ReadonlyDeep<components['schemas']['RolePreview']>;
  diff: ReadonlyDeep<components['schemas']['RoleDiff']>;
  readers: ReadonlyDeep<components['schemas']['RoleReader'][]> | null;
  size?: number;
  limit?: number;
};
type FileEntry = { r: RoleRow; l: Place; e: RoleEntry };
export type SavedFileEntry = { r: RoleRow; l: Place; e: RoleEntry & { saved: EditableDocument } };
// group is the project the role's file stands in (a headquarters screen's
// group): a draft belongs to that project's file, not to another one's of the
// same name (r-57d8).
type CarriedRole = { place: Place; group?: string; saved: string | null; name: string; hash: string; draft: EditableDocument; isNew?: boolean; noDoc?: boolean };
type CarriedYaml = { place: Place; group?: string; name: string; hash: string; text: string };
type CarriedMember = { team: string; hash: string; name: string; saved: string; draft: string };
type CarriedRoles = { entries?: RoleCarry[]; assignments?: CompleteCarry<{ name: string; role: string }[]>[]; rows?: CarriedRole[]; ytexts?: CarriedYaml[]; members?: CarriedMember[] };

export const view = document.getElementById('view');
export const tabsEl = document.getElementById('tabs');

const ISLAND: RoleIsland = (() => {
  const el = document.getElementById('roles-data');
  if (!el) return {};
  try {
    // This is the browser boundary for the server-rendered roles JSON island.
    return JSON.parse(el.textContent || '{}') as RoleIsland;
  } catch { return {}; }
})();

export const PLACES: Place[] = ['project', 'global'];
export const DIR: Record<Place, string> = Object.assign({ project: '.hergy/roles/', global: '~/.config/hergy/roles/' }, ISLAND.places || {});
export const TEXT: Record<string, string> = ISLAND.texts || {};
// fill is the server's sentence with the screen's numbers in it: the words stay
// the server's (the same ones hergy validate prints).
export const fill = (s: string | undefined, ...a: string[]): string => String(s || '').replace(/%[sd]/g, () => (a.length ? a.shift() || '' : ''));
export const LNAME: Record<Place, string> = { project: 'プロジェクト', global: 'グローバル' };
export const clone = <T,>(o: T): T => structuredClone(o);
const other = (l: Place): Place => (l === 'project' ? 'global' : 'project');
let uid = 0;

// HQ is the headquarters lists (d-73fc): every listed project's roles and team
// definitions, then the global layer's. Without it the screen is the one
// project's, as it always was.
export const HQ = ISLAND.hq || null;
// GROUPED is a headquarters with projects: listed: every row stands in a group
// and carries its project. Without projects: the screen is the one project's,
// the way it always was (d-73fc).
export const GROUPED = !!(HQ && HQ.projects && HQ.projects.mode === 'listed');
export const CURRENT = GROUPED && HQ ? ((HQ.projects.current && HQ.projects.current.path) || '') : '';
export const groupKeyOf = (g: components['schemas']['HQRoleGroup'] | components['schemas']['HQTeamGroup']): string => (g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupNameOf = (g: components['schemas']['HQRoleGroup'] | components['schemas']['HQTeamGroup']): string => (g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const currentGroup = () => (GROUPED ? CURRENT : '');
// currentProjectName is the name the プロジェクト column gives this serve's
// project, whether or not it is listed (d-8a45, r-5b96 audit 5).
export const currentProjectName = (): string => {
  if (!HQ) return LNAME.project;
  const p = HQ.projects || {};
  return (p.current && p.current.name) || '';
};
export const currentGroupName = (): string => {
  if (!HQ) return '';
  const p = HQ.projects || {};
  if (p.mode === 'current') return (p.current && p.current.name) || '';
  const found = (p.projects || []).find((x) => x.path === CURRENT);
  return found ? found.name : ((p.current && p.current.name) || '');
};

function entryOf(l: Place, group: string, data: { saved: EditableDocument | null; draft: EditableDocument; hash: string; text: string; path?: string; problem?: string; base?: ResourceIdentity<'role'> }): RoleEntry {
  const identity = resourceIdentity('role', { place: l, project: projectOfGroup(group), name: data.saved?.name });
  const editing = editRole(identity, { ...data, saved: data.saved && identity.name ? { ...data.saved, name: identity.name } : null, readable: !data.problem });
  return entryView(editing, data.problem);
}
function entryView(editing: RoleEditing, problem?: string): RoleEntry {
  return {
    editing, problem,
    get saved() { return editing.read.value.saved; }, get draft() { return editing.read.value.draft; },
    get hash() { return editing.read.value.hash; }, get text() { return editing.read.value.text || ''; },
  };
}

// ---- the model -------------------------------------------------------
// rowsOf is every row the list draws: the answer already carries one place per
// row (r-ea17), so this copies each item and gives it its group. A name both
// layers hold is two items, one row each.
function rowsOf(model: RoleModel, group: string): RoleRow[] {
  const out: RoleRow[] = [];
  for (const r of model.roles) {
    const row: RoleRow = { uid: r.uid, group: r.global && !r.project ? 'global' : group, project: null, global: null };
    if (r.markdown) {
      row.markdown = r.markdown;
      row.name = r.name || '';
      out.push(row);
      continue;
    }
    row.paths = {};
    for (const l of PLACES) {
      if (r[l]) {
        const draft = fromJSON(r[l]);
        // A role no layer holds a document for opens as a new one (spec-roles
        // 1.2.10), and a file that cannot be read keeps its row with the reason
        // (r-ea17, d-17ab): neither stands on a saved document.
        const saved = r.noDoc || r[l].problem ? null : draft;
        row[l] = entryOf(l, row.group, { saved, draft, hash: r[l].hash || '', text: r[l].text || '', path: r[l].path, problem: r[l].problem });
        if (r[l].path) row.paths[l] = r[l].path;
      } else { row[l] = null; }
    }
    if (r.mark) row.mark = r.mark;
    if (!row.project && !row.global) continue;
    if (r.noDoc) row.noDoc = true;
    if (r.isNew) row.isNew = true;
    out.push(row);
  }
  return out;
}

// RoleGroup is one place the list draws a head for (r-410c, d-e77c): the
// project's own place (its list key, which is "" without projects: listed), the
// global layer's, or another listed project's.
export type RoleGroup = { key: string; global: boolean; name: string; display: string; count: number };

// groupList is the places the list's heads stand for. A headquarters with
// projects: listed has its own groups; the one-project headquarters' groups
// are read with the same shape (the project's key is ""); a page with no
// headquarters reading still draws the project and the global layer.
function groupList(): Omit<RoleGroup, 'count'>[] {
  if (HQ && HQ.roles && HQ.roles.length) {
    return HQ.roles.map((g) => ({
      key: GROUPED ? groupKeyOf(g) : (g.global ? 'global' : ''),
      global: !!g.global,
      name: groupNameOf(g),
      display: g.display || '',
    }));
  }
  return [
    { key: '', global: false, name: currentProjectName(), display: DIR.project },
    { key: 'global', global: true, name: LNAME.global, display: DIR.global },
  ];
}

// roleGroups is groupList with the rows each head holds now.
export const roleGroups = (): RoleGroup[] => groupList().map((g) => ({
  ...g, count: roles.value.filter((r) => r.group === g.key && !r.markdown).length,
}));

// groupOfRow is the key a row's head stands under. A row that came from a list
// before r-410c (the one-project page read both places in one row) is put on
// its document's side.
export const groupOfRow = (r: RoleRow): string => (r.group || (r.global && !r.project ? 'global' : ''));

function initialRoles(): RoleRow[] {
  const out: RoleRow[] = [];
  if (GROUPED && HQ && HQ.roles && HQ.roles.length) {
    for (const g of HQ.roles) {
      const key = GROUPED ? groupKeyOf(g) : (g.global ? 'global' : '');
      out.push(...rowsOf(roleModel(g.list), key));
    }
  } else {
    out.push(...rowsOf(roleModel(ISLAND.role || null), ''));
  }
  out.forEach((r) => { r.uid = ++uid; });
  return out;
}

// ---- the screen's state ---------------------------------------------
// Rows and navigation are the screen's; entries read their private editors.
const FIRST = initialRoles();
export const roles = signal<RoleRow[]>(FIRST);
export const state = signal<RoleState>({
  tab: 'roles', open: new Set(), filter: null, q: '', roleHi: null,
  yfile: null, pmode: {},
});
// focusAsk and gotoAsk are where the caret goes after a press: which row's which
// field (5 章 5), and which member of the assign tab a chip sent it to (P11).
export const focusAsk = signal<{ uid: number; sel: string; select?: boolean; scroll?: boolean; seq: number } | null>(null);
export const gotoAsk = signal<{ team: string; member: string; i: number; seq: number } | null>(null);

const patch = (part: Partial<RoleState>): void => { state.value = { ...state.value, ...part }; };
export const askFocus = (id: number | string, sel: string, opts?: { select?: boolean; scroll?: boolean }): void => {
  focusAsk.value = {
    uid: Number(id), sel, select: !!(opts && opts.select), scroll: !!(opts && opts.scroll),
    seq: ((focusAsk.value && focusAsk.value.seq) || 0) + 1,
  };
};
export const askGoto = (team: string, member: string, i: number): void => {
  gotoAsk.value = { team, member, i, seq: ((gotoAsk.value && gotoAsk.value.seq) || 0) + 1 };
};

const TEAMS: TeamCard[] = teamModel(ISLAND.team || null).teams.map((t) => {
  const editing = editRoleAssignments(resourceIdentity('team', { place: t.place, project: projectOfGroup(currentGroup()), name: t.id }), t.members, t.hash);
  return { ...t, group: currentGroup(), editing,
    get hash() { return editing.read.value.hash; },
    get members() { return [...editing.rows.value]; },
  };
});
// The definitions that read the roles: the current project's for the 割り当て
// tab, and every group's for the 使っているチーム column.
export const teams = signal<TeamCard[]>(TEAMS);
export const hqTeams: HqTeam[] = [];
if (GROUPED && HQ) {
  (HQ.teams || []).forEach((g) => {
    const key = groupKeyOf(g);
    ((g.list && g.list.items) || []).forEach((item) => {
      const sum = item.summary;
      hqTeams.push({
        id: item.name || '', group: key,
        members: ('members' in sum ? sum.members || [] : []).map((m) => ({ name: m.name, saved: m.role || '', draft: m.role || '' })),
      });
    });
  });
}
// A row a person adds gets an id of its own: start past the ones the island
// carries, so a new row never takes an opened row's place.
uid = FIRST.reduce((m, r) => Math.max(m, r.uid || 0), uid);
export const nextUid = (): number => { uid += 1; return uid; };

// ---- lookups ---------------------------------------------------------
export const byUid = (id: number | string): RoleRow | undefined => roles.value.find((r) => r.uid === Number(id));
export const live = (r: RoleRow | null | undefined, l: Place): boolean => !!(r && r[l] && r[l].saved);
export const eff = (r: RoleRow): Place => (live(r, 'project') ? 'project' : live(r, 'global') ? 'global' : (r.project ? 'project' : 'global'));
// rowGroup is the place a row is written in: the project's path (or `global`) on
// a screen with projects listed, and this project's side (`project`/`global`)
// on the one-project screen (⑥-2, d-a143).
const rowGroup = (r: RoleRow): string => (GROUPED ? r.group : eff(r));
// rowRef reads a row the way the address writes it (⑥-2, d-a143).
const rowRef = (x: RoleRow): { name: string; group: string } => ({ name: curName(x), group: rowGroup(x) });
const here = HQ && HQ.projects && HQ.projects.current ? (HQ.projects.current.path || '') : (GROUPED ? CURRENT : '');
export const cur = (r: RoleRow): ReadonlyDeep<EditableDocument> => r[eff(r)]!.draft;
export const curName = (r: RoleRow): string => (r.markdown ? r.name || '' : (r[eff(r)]!.saved || r[eff(r)]!.draft).name);
export const savedRoles = (): RoleRow[] => roles.value.filter((r) => live(r, 'project') || live(r, 'global'));
export const roleByName = (n: string): RoleRow | undefined => savedRoles().find((r) => curName(r) === n && (!GROUPED || r.group === CURRENT)) || savedRoles().find((r) => curName(r) === n && r.group === 'global');
// whoReads is the members that name a role: the row's own project's teams, and
// for a global row every project's (spec 4 章). Each answer says which project
// the team belongs to.
export const whoReads = (n: string, group: string) => {
  const pool = GROUPED ? hqTeams.filter((t) => (group === 'global' ? true : t.group === group)) : teams.value;
  return pool.flatMap((t) => t.members.filter((m) => m.saved === n).map((m) => ({ t, m })));
};
export const projectNameOf = (key: string): string => {
  if (key === 'global') return 'グローバル';
  if (key === '' || key === CURRENT) return currentGroupName();
  const found = HQ && ((HQ.projects || {}).projects || []).find((x) => x.path === key);
  return found ? found.name : String(key || '').split('/').filter(Boolean).pop() || '';
};
export const pathOf = (l: Place, d: Pick<EditableDocument, 'name'>): string => DIR[l] + (d.name || '…') + '.md';
// pathOfRow is the file one panel stands behind: the answer's own path when the
// row is another project's, and the one this project's places build.
export const pathOfRow = (r: RoleRow, l: Place, d: Pick<EditableDocument, 'name'>): string => r[l]?.editing.read.value.path || (r.paths && r.paths[l]) || pathOf(l, d);

// ---- the rows, changed ----------------------------------------------
// withRow puts a new row in place of one: the array is new, so every part that
// reads roles is drawn again, and the row itself is the new one (5 章 2).
export const withRow = (id: number | string, fn: (row: RoleRow) => RoleRow): void => {
  const one = Number(id);
  roles.value = roles.value.map((r) => (r.uid === one ? fn(r) : r));
};
export function changeDraft(id: number | string, l: Place, draft: EditableDocument): void {
  byUid(id)?.[l]?.editing.change(draft);
}
export function touch(savedKeys: string[] = []): void {
  allEntries().forEach(({ r, l, e }) => { if (!savedKeys.includes(keyOf(r, l))) e.editing.dismissSaved(); });
  teams.value.forEach((t) => { if (!savedKeys.includes('t' + t.id)) t.editing.dismissSaved(); });
  redraw();
}
// redraw draws the rows and the band again without moving the 保存しました the
// drawing already shows. A check that answers is not a change the person made
// (scheduleCheck, r-b41c): a save's notice stands until the person's next change,
// and a check answering a beat after the save must not take it away — the notice
// was drawn and erased a few milliseconds later, and a walk that waits for it
// then never saw it (RB41C-5).
function redraw(): void {
  roles.value = roles.value.slice();
  tellDirty();
}

// ---- server checks ---------------------------------------------------
const timers: Record<string, ReturnType<typeof setTimeout>> = {};
const keyOf = (r: RoleRow, l: Place): string => r.uid + l;
const entryAt = (key: string): RoleEntry | undefined => allEntries().find(({ r, l }) => keyOf(r, l) === key)?.e;
export function checkOf(r: RoleRow, l: Place): CheckState | null {
  const e = r[l];
  if (!e) return null;
  const c = e.editing.validation.value;
  return { issues: issuesOf(r, l), markdown: c.preview ? clone(c.preview) : { lines: [], chars: 0 },
    diff: c.effect?.diff ? clone(c.effect.diff) : { added: [], removedBefore: [], removedAtEnd: false },
    readers: c.readers ? clone(c.readers) : null, size: c.size, limit: c.limit };
}
export function issuesOf(r: RoleRow, l: Place): RuleIssue[] {
  const e = r[l];
  return e ? roleProblems(e.draft, e.editing.read.value.problems.map((p) => ({ ...p }))) : [];
}
export function rowOfPanel(r: RoleRow, l: Place): RulePanel | null {
  const e = r[l], c = checkOf(r, l);
  if (!e || !c) return null;
  return { saved: e.saved, draft: e.draft };
}
export async function askCheck(r: RoleRow | null | undefined, l: Place, cb?: (() => void) | null): Promise<void> {
  const e = r?.[l];
  if (!e) return;
  await e.editing.check();
  cb?.();
}
export function scheduleCheck(r: RoleRow, l: Place): void {
  const k = keyOf(r, l);
  clearTimeout(timers[k]);
  timers[k] = setTimeout(() => askCheck(byUid(r.uid), l, () => redraw()), 300);
}
export const clearTimer = (r: RoleRow, l: Place): void => clearTimeout(timers[keyOf(r, l)]);
export const okOf = (key: string): boolean => key.startsWith('t') ? !!teams.value.find((t) => 't' + t.id === key)?.editing.read.value.savedLine : !!entryAt(key)?.editing.read.value.savedLine;
export const refusalOf = (key: string): { at: 'name' | 'doc'; text: string } | null => {
  const state = entryAt(key)?.editing.read.value;
  return state?.refusal ? { at: state.refusalAt || 'doc', text: state.refusal } : null;
};
export const teamErrorOf = (id: string): string => teams.value.find((t) => t.id === id)?.editing.read.value.refusal || '';

// ---- URL -------------------------------------------------------------
// openNewPanel opens the row a screen shows for a role no document holds: its
// one empty rule stands open and its check runs once, the way ＋ ロールを追加
// does (spec-roles 1.2.4, 1.2.10). Until the check comes back the preview would
// say 0 行 · 0 文字, which is not what the row holds.
export function openNewPanel(r: RoleRow): void {
  const l = eff(r), e = r[l];
  if (!e) return;
  if (!e.editing.read.value.checked) askCheck(r, l, () => touch());
}

export function parseURL(): void {  const path = location.pathname;
  const tab: RoleState['tab'] = /\/roles\/assign$/.test(path) ? 'assign' : /\/roles\/yaml$/.test(path) ? 'yaml' : 'roles';
  const qs = new URLSearchParams(location.search);
  const next: Partial<RoleState> = { tab };
  if (tab === 'roles') {
    const f = qs.get('filter');
    next.filter = (f === 'issues') ? f : null;
    next.q = qs.get('q') || '';
    const open = new Set(state.value.open);
    // An address with no place (the old link) names both rows of that name: the
    // project's and the global one (d-a143). A place opens its own row only.
    readOpen(qs).forEach(({ name, group: g }) => {
      rowsFor({ name, group: g }, roles.value, rowRef, here).forEach((r) => {
        open.add(r.uid);
        if (r.noDoc) openNewPanel(r);
      });
    });
    next.open = open;
  } else if (tab === 'assign') {
    next.roleHi = qs.get('role') || null;
  } else {
    const f = qs.get('file');
    if (f) {
      const m = /^(project|global)\.(.+)$/.exec(f);
      if (m) {
        const r = roleByName(m[2]);
        const place: Place = m[1] === 'project' ? 'project' : 'global';
        if (r && r[place] && r[place]!.saved && eff(r) === place) next.yfile = r.uid + place;
      }
    }
  }
  patch(next);
}

export function syncURL(push: boolean): void {
  const s = state.value;
  let path = '/roles';
  const q = new URLSearchParams();
  if (s.tab === 'assign') {
    path = '/roles/assign';
    if (s.roleHi) q.set('role', s.roleHi);
  } else if (s.tab === 'yaml') {
    path = '/roles/yaml';
    if (s.yfile) { const m = /^(\d+)(project|global)$/.exec(s.yfile); if (m) { const r = byUid(m[1]); const place: Place = m[2] === 'project' ? 'project' : 'global'; if (r) q.set('file', place + '.' + curName(r)); } }
  } else {
    if (s.filter) q.set('filter', s.filter);
    if (s.q) q.set('q', s.q);
    const open = [...s.open].map((u) => byUid(u)).filter((r): r is RoleRow => !!r).map((r) => rowAddress(curName(r), rowGroup(r)));
    if (open.length) writeOpen(q, open);

  }
  const url = path + (q.toString() ? '?' + q.toString() : '');
  if (push) history.pushState(null, '', url); else history.replaceState(null, '', url);
}

// ---- the list's filters ---------------------------------------------
export const FILTERS: Array<[RoleState['filter'], string]> = [[null, 'すべて'], ['issues', '問題・警告']];
export const allEntries = (): FileEntry[] => roles.value.flatMap((r) => PLACES.flatMap((l) => r[l] ? [{ r, l, e: r[l]! }] : []));
export function ruleShown(x: EditableDocument, issues: RuleIssue[]): boolean {
  const s = state.value;
  return (!s.filter || issues.length > 0) && (!s.q || [x.name, x.title, x.text].some((v) => v.toLowerCase().includes(s.q.toLowerCase())));
}
export const filtering = (): boolean => !!(state.value.filter || state.value.q);
export const rowIssues = (r: RoleRow): RuleIssue[] => PLACES.filter((l) => r[l]).flatMap((l) => issuesOf(r, l));

export function filterCount(f: RoleState['filter']): number {
  return roles.value.filter((r) => !f || rowIssues(r).length > 0).length;
}

// ---- the text representation and conflict operations ------------------
export const filesList = (): SavedFileEntry[] => allEntries().filter((entry): entry is SavedFileEntry => !!entry.e.saved && live(entry.r, entry.l) && (!GROUPED || entry.r.group === CURRENT || entry.r.group === 'global'));
export const docText = (r: RoleRow, l: Place): string => r[l]?.editing.yaml.read.value.draft || '';
export const conflictAt = (key: string) => entryAt(key)?.editing.read.value.conflict || null;
export function pressOverwrite(id: number | string, l: Place): void { void saveRole(id, l, true); }
export function pressRevertConflict(id: number | string, l: Place): void {
  const r = byUid(id); r?.[l]?.editing.readConflict(); void askCheck(r, l, () => touch()); touch();
}
export function pressUndoConflict(id: number | string, l: Place): void {
  const r = byUid(id); r?.[l]?.editing.undoConflict(); void askCheck(r, l, () => touch()); touch();
}
export function yamlSaved(r: RoleRow, l: Place, answer: { json: { hashes?: Record<string, string> | null } | null }): void {
  rebase(answer.json, r.group || ''); void askCheck(byUid(r.uid), l, () => redraw()); tellDirty();
}

// pressToggle opens or closes a row. A row a person presses into opens in front
// of them; the caret goes back to the line they pressed (5 章 5).
export function pressToggle(idPressed: number | string): void {
  const r = byUid(idPressed);
  if (!r) return;
  const open = new Set(state.value.open);
  if (open.has(r.uid)) open.delete(r.uid);
  else { open.add(r.uid); if (r.noDoc) openNewPanel(r); }
  patch({ open });
  askFocus(r.uid, '.sum');
  touch();
  syncURL(false);
}

export function pressFilter(f: RoleState['filter']): void {
  patch({ filter: f || null });
  touch();
  syncURL(false);
}

export function pressNew(group?: string): void {
  const d: EditableDocument = { name: '', title: '', text: '## 責務\n' };
  // The head a person pressed says the place the row stands in (r-410c): the
  // row is that group's, and there is no switch inside it to move it.
  const key = group === undefined ? roleGroups()[0].key : group;
  const place: Place = key === 'global' ? 'global' : 'project';
  const nr: RoleRow = { uid: nextUid(), isNew: true, group: key, project: null, global: null };
  nr[place] = entryOf(place, key, { saved: null, draft: d, hash: '', text: '' });
  // The row stands at the top of the head it was added under.
  const order = roleGroups().map((g) => g.key);
  const rank = order.indexOf(key);
  let at = roles.value.findIndex((r) => groupOfRow(r) === key);
  if (at < 0) {
    at = roles.value.findIndex((r) => order.indexOf(groupOfRow(r)) > rank);
    if (at < 0) at = roles.value.length;
  }
  roles.value = [...roles.value.slice(0, at), nr, ...roles.value.slice(at)];
  const open = new Set(state.value.open);
  open.add(nr.uid);
  patch({ open, filter: null, q: '' });
  touch();
  askCheck(nr, place, () => { askFocus(nr.uid, '[data-f=name]'); touch(); });
}

export function pressPmode(id: number | string, l: Place, v: string): void {
  // The switch's button is the one the person pressed: it keeps the focus
  // through the draw, so nothing is put back (5 章 5).
  patch({ pmode: { ...state.value.pmode, [id + l]: v } });
  touch();
}

// basePlaceOf is the document a duplicate or a copy of one row stands on, named
// the way the server reads it: the place the document stands in, its name, and,
// for a row under a listed project's head, that project. Without the project
// the server reads the file of the project it was started in, and the copy is
// written from another project's lines (r-a028).
const basePlaceOf = (r: RoleRow, l: Place): ResourceIdentity<'role'> => resourceIdentity('role', { place: l, name: r[l]?.saved?.name, project: projectOfGroup(l === 'global' ? 'global' : groupOfRow(r)) });

export function pressDup(id: number | string, l: Place): void {
  // Duplicate the saved document and retain its source identity.
  const r = byUid(id);
  const src = r && r[l] && r[l].saved;
  if (!src) return;
  const d: EditableDocument = { ...src };
  d.name += '-2';
  const nr: RoleRow = { uid: nextUid(), isNew: true, group: r.group, project: null, global: null };
  nr[l] = entryOf(l, r.group, { saved: null, draft: d, base: basePlaceOf(r, l), hash: '', text: '' });
  const at = roles.value.indexOf(r);
  roles.value = [...roles.value.slice(0, at + 1), nr, ...roles.value.slice(at + 1)];
  const open = new Set(state.value.open);
  open.add(nr.uid);
  patch({ open });
  touch();
  askCheck(nr, l, () => { askFocus(nr.uid, '[data-f=name]'); touch(); });
}

export function pressCopy(id: number | string, l: Place): void {
  const r = byUid(id);
  if (!r || !r[l] || !r[l].saved) return;
  const to = other(l);

  // The list draws each place as its own row under its head; the copy stands in
  // the place it was copied to, not in the row it came from (r-f781, r-410c).
  const group = copyGroupOf(to, groupOfRow(r), CURRENT);
  const nr: RoleRow = { uid: nextUid(), isNew: true, group, project: null, global: null };
  nr[to] = entryOf(to, group, { saved: null, draft: clone(r[l]!.saved!), base: basePlaceOf(r, l), hash: '', text: '' });
  const order = roleGroups().map((g) => g.key);
  const rank = order.indexOf(group);
  let at = roles.value.findIndex((x) => groupOfRow(x) === group);
  if (at < 0) {
    at = roles.value.findIndex((x) => order.indexOf(groupOfRow(x)) > rank);
    if (at < 0) at = roles.value.length;
  }
  roles.value = [...roles.value.slice(0, at), nr, ...roles.value.slice(at)];
  // The row the person pressed closes, and the copy under the other place's
  // head is the one open row they read (d-022a).
  const open = new Set(state.value.open);
  open.delete(Number(id));
  open.add(nr.uid);
  patch({ open });
  touch();
  // The copy stands off the screen when the other place's head is below it: its
  // name comes to the band, all of it chosen (d-022a).
  askCheck(nr, to, () => { askFocus(nr.uid, '[data-f=name]', { select: true, scroll: true }); touch(); });
}

// canCopy is whether a row's document can be copied into the other place: the
// other place must not already hold that name, in this row or in the row the
// copy would stand beside (r-f781).
export function canCopy(r: RoleRow, l: Place): boolean {
  const e = r[l];
  if (!e || !e.saved) return false;
  const to = other(l);
  if (r[to]) return false;
  const group = copyGroupOf(to, groupOfRow(r), CURRENT);
  return !roles.value.some((x) => groupOfRow(x) === group && x[to] && x[to]!.draft.name === e.saved!.name);
}



export function pressDiscard(id: number | string, l: Place): void {
  const r = byUid(id);
  if (!r) return;
  const entry = r[l];
  if (entry && entry.saved) {
    entry.editing.discard();
  } else {
    entry?.editing.dispose();
    withRow(id, (old) => ({ ...old, [l]: null }));
  }
  const after = byUid(id);
  if (after && !after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== Number(id));
    const open = new Set(state.value.open);
    open.delete(Number(id));
    patch({ open });
  }
  touch();
}

// pressSearch is the list's search: the letters go into the state the list is
// drawn from. The field is the same element after the draw, so the cursor stays
// where the person put it (5 章 5).
export function pressSearch(value: string): void {
  patch({ q: value });
  touch();
  syncURL(false);
}

export function pressTab(t: RoleState['tab']): void {
  patch({ tab: t });
  touch();
  syncURL(true);
}

export function pressRoleHi(name: string): void {
  patch({ roleHi: state.value.roleHi === name ? null : name });
  touch();
  syncURL(false);
}

// pressGoto walks to the assign tab and puts the cursor on the member the row
// named (spec-roles 2「割り当てのタブ」).
export function pressGoto(team: string, member: string, i: number): void {
  patch({ tab: 'assign' });
  touch();
  syncURL(true);
  askGoto(team, member, i);
  history.replaceState(null, '', `${location.pathname}${location.search}#${team}.m-${i}`);
}

// pickRole is one member's role picked in the assign tab: the pick is the
// screen's draft until the team's 保存 writes it.
export function pickRole(id: string, i: number, value: string): void {
  const t = teams.value.find((x) => x.id === id);
  if (!t) return;
  const name = t.members[Number(i)]?.name;
  t.editing.change((members) => members.map((m) => m.name === name ? { ...m, role: value } : m));
  touch();
}

export function pressYfile(key: string | null): void {
  patch({ yfile: key });
  touch();
  syncURL(false);
}

// setYfile is the file the tab shows when the address names none: the first one.
export function setYfile(key: string | null): void {
  patch({ yfile: key });
}

// ---- writes ----------------------------------------------------------
// rebase puts the files a save's answer read again under the rows' feet
// (r-b41c). A save gives the rules their ids after it writes (d-4aab), so a
// document the person did not touch can move while another one is saved: a row
// that kept the hash it read before would be refused at its own next save as a
// change another place made. The row's own document is the answer's, and every
// other file of that project takes the hash the answer read; the draft a person
// is writing in stays where it is.
//
// The answer names one project's files, and the screen holds several projects'
// rows (d-73fc): a row of another project keeps its own file's hash, which the
// answer did not read (r-57d8).
function rebase(answer: { hashes?: Record<string, string> | null } | null, group: string): void {
  const files = bases(answer);
  if (!files.length) return;
  for (const r of roles.value) {
    if ((r.group || '') !== group) continue;
    for (const b of files) {
      const l = b.place === 'project' ? 'project' : b.place === 'global' ? 'global' : null;
      const e = l ? r[l] : null;
      if (e?.saved?.name === b.name && e.hash !== b.hash) e.editing.rebase(b.hash);
    }
  }
}

export async function doSave(id: number | string, l: Place): Promise<void> { await saveRole(id, l); }
async function saveRole(id: number | string, l: Place, overwrite = false): Promise<void> {
  const r = byUid(id), e = r?.[l];
  if (!r || !e) return;
  const old = e.saved?.name;
  const keepRename = e.editing.read.value.rename;
  const answer = await (overwrite ? e.editing.overwrite() : e.editing.save());
  if (!answer?.json || byUid(id)?.[l]?.editing !== e.editing) return;
  const newName = e.saved?.name || '';
  withRow(id, (row) => { const next = { ...row }; delete next.isNew; delete next.noDoc; return next; });
  if (old && old !== newName && keepRename) {
    teams.value.forEach((team) => team.editing.renameRole(old, newName));
    const otherPlace = other(l);
    if (!GROUPED && r[otherPlace]) {
      const split: RoleRow = { uid: nextUid(), project: null, global: null, group: r.group };
      split[otherPlace] = r[otherPlace];
      const at = roles.value.findIndex((x) => x.uid === Number(id));
      roles.value = [...roles.value.slice(0, at + 1), split, ...roles.value.slice(at + 1)];
      withRow(id, (row) => ({ ...row, [otherPlace]: null }));
    }
  }
  rebase(answer.json, r.group || '');
  await askCheck(byUid(id), l);
  touch([keyOf(r, l)]); syncURL(false); apiSaved();
}
export async function doDelete(id: number | string, l: Place): Promise<void> {
  const r = byUid(id), e = r?.[l];
  if (!r || !e?.saved) return;
  const answer = await e.editing.remove();
  if (!answer || byUid(id)?.[l]?.editing !== e.editing) { touch(); return; }
  clearTimer(r, l); e.editing.dispose();
  withRow(id, (row) => ({ ...row, [l]: null }));
  const after = byUid(id);
  if (after && !after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== Number(id));
    const open = new Set(state.value.open); open.delete(Number(id)); patch({ open, tab: 'roles' });
  } else patch({ tab: 'roles' });
  touch(); syncURL(false);
}
export async function doAssignSave(id: string): Promise<void> {
  const team = teams.value.find((t) => t.id === id);
  if (!team?.editing.dirty.value) return;
  const answer = await team.editing.save();
  if (!answer?.json) { touch(); return; }
  for (const r of savedRoles()) await askCheck(r, eff(r));
  touch(['t' + id]); apiSaved();
}

// ---- 帯への報告と持ち越し (spec-nav.md 1.4, 1.5) ---------------------
// dirty is whether the screen holds something no save wrote yet: a row's draft,
// a row a person added, a pick in the assign tab, or a YAML text.
export function dirty(): boolean {
  const rows = roles.value.some((r) => PLACES.some((l) => r[l] && r[l]!.editing.dirty.value));
  const members = teams.value.some((t) => t.editing.dirty.value);
  const ytexts = allEntries().some(({ e }) => e.editing.yaml.dirty.value);
  return rows || members || ytexts;
}

export function tellDirty(): void {
  setDirty(dirty());
}

// lost names what leaving the page now would throw away: changed's words for
// each row that differs (r-1e9a, d-021e).
export function lost(): string[] {
  const out: string[] = [];
  for (const r of roles.value) for (const l of PLACES) {
    const e = r[l];
    if (!e || !e.editing.dirty.value) continue;
    out.push(...e.editing.lost.value);
  }
  for (const t of teams.value) for (const m of t.members) {
    if (!m.changed) continue;
    out.push(...t.editing.lost.value);
  }
  for (const { e } of allEntries()) {
    if (!e.editing.yaml.dirty.value) continue;
    out.push(...e.editing.yaml.lost.value);
  }
  return [...new Set(out)];
}

// take is the draft the band puts aside: every row's draft that is not what the
// file holds, every row a person added, every pick that was made, every YAML
// text that was typed — each with the hash of the file it was read from, so a
// file that moved outside is not written over by a page that read another one.
export function take(): CarriedRoles {
  return {
    entries: allEntries().filter(({ e }) => !e.saved || e.editing.dirty.value || e.editing.yaml.dirty.value).map(({ e }) => e.editing.take()),
    assignments: teams.value.filter((t) => t.editing.dirty.value).map((t) => t.editing.take()),
  };
}

// restore puts the carried draft back, skipping every part whose file moved
// outside since the draft was made (spec-nav.md 1.5).
export function restore(carried: CarriedRoles): void {
  for (const carry of carried.entries || []) {
    const existing = allEntries().find(({ e }) => e.editing.matches(carry));
    if (existing) { if (existing.e.editing.restore(carry)) void askCheck(existing.r, existing.l); continue; }
    const group = carry.target.place === 'global' ? 'global' : carry.target.project || '';
    if (!roleGroups().some((g) => g.key === group)) continue;
    const l = carry.target.place;
    const editing = restoreRole(resourceIdentity('role', { place: l, project: projectOfGroup(group) }), carry);
    if (!editing) continue;
    const row: RoleRow = { uid: nextUid(), group, isNew: true, project: null, global: null };
    row[l] = entryView(editing);
    const at = roles.value.findIndex((r) => groupOfRow(r) === group);
    const order = roleGroups().map((g) => g.key);
    const before = at >= 0 ? at : roles.value.findIndex((r) => order.indexOf(groupOfRow(r)) > order.indexOf(group));
    const index = before >= 0 ? before : roles.value.length;
    roles.value = [...roles.value.slice(0, index), row, ...roles.value.slice(index)];
    patch({ open: new Set([...state.value.open, row.uid]) });
    void askCheck(row, l);
  }
  for (const carry of carried.assignments || []) teams.value.find((t) => t.editing.matches(carry))?.editing.restore(carry);
  // Read the preceding UI's carry format, but only write editing's format.
  (carried.rows || []).forEach((c) => {
    let r = roles.value.find((x) => {
      if ((x.group || '') !== (c.group || '')) return false;
      const entry = x[c.place];
      return c.saved
        ? !!entry?.saved && entry.saved.name === c.saved
        : !!entry && !entry.saved && (x.noDoc || x.isNew || entry.draft.name === c.name);
    });
    if (!r && c.isNew && c.saved === null && c.hash === '' && roleGroups().some((g) => g.key === (c.group || ''))) {
      r = { uid: nextUid(), group: c.group || '', isNew: true, project: null, global: null };
      r[c.place] = entryOf(c.place, r.group, { saved: null, draft: { name: '', title: '', text: '## 責務\n' }, hash: '', text: '' });
      roles.value = [...roles.value, r];
      patch({ open: new Set([...state.value.open, r.uid]) });
    }
    if (!r) return;
    const e = r[c.place];
    if (!e) return;
    if (c.hash && e.hash !== c.hash) return;
    e.editing.restore({ hash: c.hash, draft: c.draft });
    if (c.isNew) withRow(r.uid, (one) => ({ ...one, isNew: true }));
    clearTimer(r, c.place);
    askCheck(byUid(r.uid), c.place, null);
  });
  (carried.ytexts || []).forEach((c) => {
    const r = roles.value.find((x) => { if ((x.group || '') !== (c.group || '')) return false; const entry = x[c.place]; return !!entry && (entry.saved || entry.draft).name === c.name; });
    if (!r) return;
    const e = r[c.place];
    if (!e) return;
    if (c.hash && e.hash !== c.hash) return;
    e.editing.yaml.restore({ hash: c.hash, draft: c.text });
  });
  (carried.members || []).forEach((c) => {
    const t = teams.value.find((x) => x.id === c.team);
    if (!t || (c.hash && t.hash !== c.hash)) return;
    const at = t.members.findIndex((x) => x.name === c.name && x.saved === c.saved);
    if (at >= 0) {
      t.editing.change((members) => members.map((m, i) => i === at ? { ...m, role: c.draft } : m));
    }
  });
  touch();
}

// boot reads the address, asks every saved document's check, and keeps what the
// band carried from the screen before this one (spec-nav.md 1.5).
export async function boot(): Promise<void> {
  parseURL();
  await Promise.all(roles.value.flatMap((r) => PLACES.filter((l) => r[l] && r[l].saved).map((l) => askCheck(r, l))));
  const carried = takeDraft('roles');
  if (carried) restore(carried);
  touch();
}
