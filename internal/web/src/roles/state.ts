// roles/state.js — ロールの画面の state（webui-preact-design 4 章・5 章）。
//
// 一覧の行、割り当てのチーム、YAML のファイル、そしてサーバーのチェックの答えは、
// すべてここに signal で置く（5 章 2）。部品（view.js / assign.js / yaml.js）は
// signal を読んで描き、値が変われば Preact が自分で描き直す。だから描き直しを
// 呼ぶ関数は無い（依頼書 1）。行の下書きは共有部品（ruledoc）が書き換え、変える
// たびに onChange で知らせるので、画面はそのたびに新しい行（新しい下書きの写し）を
// signal に入れる。
import { signal } from '../lib/preact.js';
import { resource } from '../lib/resource';
import { takeDraft, setCount, setDirty } from '../lib/nav.js';
import { parseOpen, token as hqToken } from '../lib/hq.js';
import { copyGroupOf } from '../lib/copy';
import {
  check as apiCheck, save as apiSave, del as apiDel, saved as apiSaved,
  roleModel, teamModel, problemText, Refusal, bases,
} from '../lib/api.js';
import { conflictOf } from '../lib/conflict.js';
import type { Conflict } from '../lib/conflict.js';
import {
  fromJSON, toDoc, rulesOf, inP, nextKey, observeKeys, openRules, openRuleKey, renameCarries,
} from '../lib/ruledoc.js';
import type { EditableDocument, EditableRule, RuleIssue, RulePanel } from '../lib/ruledoc.js';
import type { Answer, RawAnswer, Request, RoleRow as ApiRoleRow, TeamCard as ApiTeamCard } from '../lib/api.js';
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
  saved: EditableDocument | null;
  draft: EditableDocument;
  base?: EditableDocument;
  basePlace?: NonNullable<components['schemas']['RoleSaveRequest']['base']>;
  hash: string;
  text: string;
  ytext?: string;
  __savedIssues?: RuleIssue[];
};
export type RoleRow = {
  uid: number;
  group: string;
  project: RoleEntry | null;
  global: RoleEntry | null;
  paths?: Partial<Record<Place, string>>;
  markdown?: string;
  name?: string;
  noDoc?: boolean;
  isNew?: boolean;
};
type RoleMember = { name: string; saved: string; draft: string };
type TeamCard = Omit<ApiTeamCard, 'group'> & { group: string };
type HqTeam = { id: string; group: string; members: RoleMember[] };
type RoleState = {
  tab: 'roles' | 'assign' | 'yaml';
  open: Set<number>;
  filter: 'off' | 'issues' | null;
  q: string;
  roleHi: string | null;
  confirm: Record<string, 1>;
  ok: Record<string, 1>;
  yfile: string | null;
  pmode: Record<string, string>;
  refusals: Record<string, { at: 'name' | 'doc'; text: string }>;
  teamErrors: Record<string, string>;
  // conflicts stand where a save was refused because the file moved: the file as
  // the answer showed it, and the draft the panel held before 元に戻す (r-3c77).
  conflicts: Record<string, ConflictHold>;
};
// ConflictHold is one refused save: what the server answered, whether the person
// has read the file's own text in (undone), and the draft to put back with
// 取り消す.
type ConflictHold = {
  c: Conflict;
  undone?: boolean;
  before?: { draft: EditableDocument; saved: EditableDocument | null; hash: string; text: string };
};
type CheckState = {
  issues: RuleIssue[];
  markdown: components['schemas']['RolePreview'];
  diff: components['schemas']['RoleDiff'];
  turns: components['schemas']['RoleTurn'][] | null;
  readers: components['schemas']['RoleReader'][] | null;
  size?: number;
  limit?: number;
};
type FileEntry = { r: RoleRow; l: Place; e: RoleEntry };
export type SavedFileEntry = { r: RoleRow; l: Place; e: RoleEntry & { saved: EditableDocument } };
type RoleCheckRequest = Request<'role', 'check'>;
type CarriedRole = { place: Place; saved: string | null; name: string; hash: string; draft: EditableDocument; isNew?: boolean; noDoc?: boolean };
type CarriedYaml = { place: Place; name: string; hash: string; text: string };
type CarriedMember = { team: string; hash: string; name: string; saved: string; draft: string };
type CarriedRoles = { rows?: CarriedRole[]; ytexts?: CarriedYaml[]; members?: CarriedMember[] };

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

const refusalAt = (err: unknown): unknown => {
  if (typeof err !== 'object' || err === null || !('json' in err)) return undefined;
  const json = err.json;
  return typeof json === 'object' && json !== null && 'at' in json ? json.at : undefined;
};

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

// ---- the model -------------------------------------------------------
// rowsOf is every row the list draws for one place: each group's answer read
// with the same api.js model as the one project's, and every row given its
// project's key and its own uid, so two projects' rows never share one. place
// keeps only one side of a name (r-410c): a list always draws its places'
// heads, so a name that stands in both is two rows.
function rowsOf(model: RoleModel, group: string, place: Place): RoleRow[] {
  const out: RoleRow[] = [];
  for (const r of model.roles) {
    const row: RoleRow = { uid: r.uid, group, project: null, global: null };
    if (r.markdown) {
      if (place !== 'project') continue;
      row.markdown = r.markdown;
      row.name = r.name || '';
      out.push(row);
      continue;
    }
    row.paths = {};
    for (const l of PLACES) {
      if (l !== place) { row[l] = null; continue; }
      if (r[l]) {
        const draft = fromJSON(r[l]);
        // A role no layer holds a document for opens as a new one (spec-roles
        // 1.2.10): nothing is saved yet, so the first 保存 writes the file, and
        // the row counts no rules until it is saved.
        const saved = r.noDoc ? null : draft;
        row[l] = { saved, draft: clone(draft), hash: r[l].hash || '', text: r[l].text || '' };
        if (r[l].path) row.paths[l] = r[l].path;
      } else { row[l] = null; }
    }
    if (!row[place]) continue;
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
      out.push(...rowsOf(roleModel(g.list), key, g.global ? 'global' : 'project'));
    }
  } else {
    const model = roleModel(ISLAND.role || null);
    out.push(...rowsOf(model, '', 'project'));
    out.push(...rowsOf(model, 'global', 'global'));
  }
  out.forEach((r) => { r.uid = ++uid; });
  return out;
}

// ---- the screen's state ---------------------------------------------
// roles is every row; state is what the screen knows about itself; checks and
// ychecks are the server's answers; teams is the assign tab's cards. Each is a
// signal, and every change puts a new value in (5 章 2).
const FIRST = initialRoles();
export const roles = signal<RoleRow[]>(FIRST);
export const state = signal<RoleState>({
  tab: 'roles', open: new Set(), filter: null, q: '', roleHi: null,
  confirm: {}, ok: {}, yfile: null, pmode: {}, refusals: {}, teamErrors: {}, conflicts: {},
});
export const checks = signal<Record<string, CheckState>>({});
export const ychecks = signal<Record<string, components['schemas']['Problem'][] | undefined>>({});
// okShowing is the 保存しました the current drawing shows: a save's answer shows
// it once, until the next change (the part that says a document is saved).
export const okShowing = signal<Record<string, 1>>({});
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

const TEAMS: TeamCard[] = teamModel(ISLAND.team || null).teams.map((t) => ({ ...t, group: currentGroup() }));
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
export const cur = (r: RoleRow): EditableDocument => r[eff(r)]!.draft;
export const curName = (r: RoleRow): string => (r.markdown ? r.name || '' : (r[eff(r)]!.saved || r[eff(r)]!.draft).name);
export const savedRoles = (): RoleRow[] => roles.value.filter((r) => live(r, 'project') || live(r, 'global'));
export const roleByName = (n: string): RoleRow | undefined => savedRoles().find((r) => curName(r) === n);
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
export const pathOf = (l: Place, d: Pick<EditableDocument, 'name'>): string => DIR[l] + (d.name || '…') + '.yaml';
// pathOfRow is the file one panel stands behind: the answer's own path when the
// row is another project's, and the one this project's places build.
export const pathOfRow = (r: RoleRow, l: Place, d: Pick<EditableDocument, 'name'>): string => (r.paths && r.paths[l]) || pathOf(l, d);

// ---- the rows, changed ----------------------------------------------
// withRow puts a new row in place of one: the array is new, so every part that
// reads roles is drawn again, and the row itself is the new one (5 章 2).
export const withRow = (id: number | string, fn: (row: RoleRow) => RoleRow): void => {
  const one = Number(id);
  roles.value = roles.value.map((r) => (r.uid === one ? fn(r) : r));
};
// withEntry puts a new place's entry in place of one row's, and the row with it.
export const withEntry = (id: number | string, l: Place, fn: (entry: RoleEntry) => RoleEntry): void => withRow(id, (r) => (r[l] ? { ...r, [l]: fn(r[l]!) } : r));

// touch is what a change that the rows do not show by themselves goes through:
// the 保存しました that stands for one drawing, the band's dot and its number.
export function touch(): void {
  okShowing.value = { ...state.value.ok };
  if (Object.keys(state.value.ok).length) patch({ ok: {} });
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
  setCount('roles', savedRoles().length);
}

// ---- server checks ---------------------------------------------------
// checkers are one resource per document place: a check is a reading like any
// other, so the newest request is the one whose answer is kept, and the one
// before it is dropped (5 章 3). The screen keeps no numbering of its own.
type RoleCheckData = Partial<Answer<'role', 'check'>>;
type RoleCheckReading = RawAnswer<RoleCheckData>;

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isProblem(value: unknown): value is components['schemas']['Problem'] {
  if (!record(value)) return false;
  return (value.sev === 'err' || value.sev === 'warn')
    && typeof value.at === 'string'
    && typeof value.text === 'string'
    && typeof value.section === 'number'
    && typeof value.rule === 'number'
    && (value.field === undefined || typeof value.field === 'string')
    && (value.line === undefined || typeof value.line === 'number')
    && (value.definition === undefined || typeof value.definition === 'string')
    && (value.role === undefined || typeof value.role === 'string')
    && (value.rule_id === undefined || typeof value.rule_id === 'string');
}

function isProblemList(value: unknown): value is components['schemas']['Problem'][] {
  return Array.isArray(value) && value.every(isProblem);
}

function isRolePreviewLine(value: unknown): value is components['schemas']['RolePreviewLine'] {
  return record(value)
    && typeof value.text === 'string'
    && typeof value.part === 'string'
    && typeof value.section === 'number'
    && typeof value.rule === 'number';
}

function isRolePreview(value: unknown): value is components['schemas']['RolePreview'] {
  return record(value)
    && typeof value.chars === 'number'
    && (value.lines === null || (Array.isArray(value.lines) && value.lines.every(isRolePreviewLine)));
}

function isRoleDiff(value: unknown): value is components['schemas']['RoleDiff'] {
  return record(value)
    && (value.added === null || (Array.isArray(value.added) && value.added.every((line) => typeof line === 'number')))
    && (value.removedBefore === null || (Array.isArray(value.removedBefore) && value.removedBefore.every((line) => typeof line === 'number')))
    && typeof value.removedAtEnd === 'boolean';
}

function isRoleTurn(value: unknown): value is components['schemas']['RoleTurn'] {
  return record(value)
    && typeof value.section === 'number'
    && typeof value.rule === 'number'
    && typeof value.arrow === 'string'
    && typeof value.definition === 'string'
    && typeof value.ok === 'boolean';
}

function isRoleReader(value: unknown): value is components['schemas']['RoleReader'] {
  return record(value) && typeof value.definition === 'string' && typeof value.member === 'string';
}

function isRoleEffect(value: unknown): value is components['schemas']['RoleEffect'] {
  return record(value)
    && isRoleDiff(value.diff)
    && Array.isArray(value.turns)
    && value.turns.every(isRoleTurn);
}

function isRoleCheckData(value: unknown): value is RoleCheckData {
  if (!record(value)) return false;
  return (value.problems === undefined || isProblemList(value.problems))
    && (value.preview === undefined || isRolePreview(value.preview))
    && (value.effect === undefined || isRoleEffect(value.effect))
    && (value.readers === undefined || (Array.isArray(value.readers) && value.readers.every(isRoleReader)))
    && (value.size === undefined || typeof value.size === 'number')
    && (value.limit === undefined || typeof value.limit === 'number');
}

function roleCheckData(value: unknown): RoleCheckData {
  return isRoleCheckData(value) ? value : {};
}

const checkers = new Map<string, ReturnType<typeof resource<RoleCheckReading, [RoleCheckRequest]>>>();
function checkerFor(k: string): ReturnType<typeof resource<RoleCheckReading, [RoleCheckRequest]>> {
  let one = checkers.get(k);
  if (one) return one;
  one = resource<RoleCheckReading, [RoleCheckRequest]>(async (signal, payload) => {
    try {
      return await apiCheck('role', payload, { signal });
    } catch (err) {
      // A refusal about the text still carries what is wrong with it.
      if (err instanceof Refusal) {
        return { ok: false, status: err.status, text: err.text, json: roleCheckData(err.json) };
      }
      throw err;
    }
  });
  checkers.set(k, one);
  return one;
}
const timers: Record<string, ReturnType<typeof setTimeout>> = {};
const keyOf = (r: RoleRow, l: Place): string => r.uid + l;
const ruleKeyAt = (d: EditableDocument, si: number, ri: number): number => { const s = d.sections[si]; if (!s) return 0; const x = s.rules[ri]; return x ? x._k : 0; };

function convertProblems(d: EditableDocument, list: components['schemas']['Problem'][]): RuleIssue[] {
  return (list || []).map((p) => ({
    sev: p.sev, k: (p.section ?? -1) >= 0 ? ruleKeyAt(d, p.section ?? -1, p.rule ?? -1) : 0,
    f: p.field || '', text: p.text || '', definition: p.definition || '',
  }));
}

export function issuesOf(r: RoleRow, l: Place): RuleIssue[] { const c = checks.value[keyOf(r, l)]; return c ? c.issues : []; }

// rowOf is one panel as the shared part reads it: the draft and the saved
// document, and the check the server last answered (its problems, the Markdown,
// the diff, the arrows).
export function rowOfPanel(r: RoleRow, l: Place): RulePanel | null {
  const e = r[l], c = checks.value[keyOf(r, l)];
  if (!e) return null;
  return {
    uid: r.uid, l, e, saved: e.saved, draft: e.draft,
    issues: c ? c.issues : [], markdown: c ? c.markdown : { lines: [], chars: 0 },
    diff: c ? c.diff : { added: [], removedBefore: [], removedAtEnd: false }, turns: c ? c.turns : [],
    savedIssues: e.__savedIssues,
  };
}

export async function askCheck(r: RoleRow | null | undefined, l: Place, cb?: (() => void) | null): Promise<void> {
  if (!r) return;
  const e = r[l];
  if (!e) return;
  const k = keyOf(r, l);
  // A check that a newer one has replaced answers nothing (the resource dropped
  // it), so only the newest reading reaches the panel.
  const answer = await checkerFor(k).run({
    place: l, original: e.saved ? e.saved.name : null,
    project: r.group && r.group !== 'global' ? r.group : undefined,
    doc: toDoc(e.draft, e.saved || e.base || null),
  });
  const c = answer && answer.json;
  if (!c || !c.problems) return;
  const issues = convertProblems(e.draft, c.problems);
  checks.value = {
    ...checks.value,
    [k]: {
      issues,
      markdown: c.preview || { lines: [], chars: 0 },
      diff: c.effect?.diff || { added: [], removedBefore: [], removedAtEnd: false },
      turns: c.effect?.turns || [], readers: c.readers || null, size: c.size, limit: c.limit,
    },
  };
  // A save's refusal under ロール名 is the same answer, older: the check speaks
  // for the name field now, so a refusal it no longer holds goes (d-841e).
  const shown = state.value.refusals[k];
  if (shown && shown.at === 'name' && !issues.some((m) => m.f === 'name')) {
    const next = { ...state.value.refusals };
    delete next[k];
    patch({ refusals: next });
  }
  if (cb) cb();
}

export function scheduleCheck(r: RoleRow, l: Place): void {
  const k = keyOf(r, l);
  clearTimeout(timers[k]);
  timers[k] = setTimeout(() => askCheck(r, l, () => { updateRowSummary(r.uid); redraw(); }), 300);
}
export const clearTimer = (r: RoleRow, l: Place): void => clearTimeout(timers[keyOf(r, l)]);

// updateRowSummary is what a check's answer changes about the row itself: the
// saved document's own problems are what a save is held to (X3).
function updateRowSummary(id: number | string): void {
  const r = byUid(id);
  if (!r) return;
  const entries: Partial<Record<Place, RoleEntry>> = {};
  PLACES.forEach((l) => {
    const entry = r[l];
    if (!entry || !entry.saved) return;
    const c = checks.value[keyOf(r, l)];
    if (c) entries[l] = { ...entry, __savedIssues: c.issues };
  });
  if (Object.keys(entries).length) withRow(id, (old) => ({ ...old, ...entries }));
}

export const okOf = (k: string): boolean => !!okShowing.value[k];
export const refusalOf = (k: string): RoleState['refusals'][string] | null => state.value.refusals[k] || null;
export const teamErrorOf = (id: string): string => state.value.teamErrors[id] || '';

// ---- URL -------------------------------------------------------------
// openNewPanel opens the row a screen shows for a role no document holds: its
// one empty rule stands open and its check runs once, the way ＋ ロールを追加
// does (spec-roles 1.2.4, 1.2.10). Until the check comes back the preview would
// say 0 行 · 0 文字, which is not what the row holds.
export function openNewPanel(r: RoleRow): void {
  const l = eff(r), e = r[l];
  const first = e && e.draft.sections[0] && e.draft.sections[0].rules[0];
  if (!first) return;
  openRuleKey(`${r.uid}${l}:${first._k}`);
  if (!checks.value[keyOf(r, l)]) askCheck(r, l, () => touch());
}

export function parseURL(): void {
  const path = location.pathname;
  const tab: RoleState['tab'] = /\/roles\/assign$/.test(path) ? 'assign' : /\/roles\/yaml$/.test(path) ? 'yaml' : 'roles';
  const qs = new URLSearchParams(location.search);
  const next: Partial<RoleState> = { tab };
  if (tab === 'roles') {
    const f = qs.get('filter');
    next.filter = (f === 'off' || f === 'issues') ? f : null;
    next.q = qs.get('q') || '';
    const openedRaw = (qs.get('open') || '');
    const openedRows = parseOpen(openedRaw) || [];
    const open = new Set(state.value.open);
    openedRows.forEach(({ name, group: g }) => {
      // A row is found by the name it shows and, from a search result, by the
      // project it stands in: a saved name, a written one, or the name of a
      // document only Markdown holds (spec-roles 2 節, spec 3 章).
      const r = roles.value.find((x) => curName(x) === name && (!g || x.group === g)) || roles.value.find((x) => curName(x) === name);
      if (!r) return;
      open.add(r.uid);
      if (r.noDoc) openNewPanel(r);
    });
    next.open = open;
    // A rule is given as <role>.<place>.<id>, or as its id alone when the
    // opening names the role (the team screen links so, d-8bbf).
    const isPlace = (value: string | undefined): value is Place => value === 'project' || value === 'global';
    (qs.get('rule') || '').split(',').filter(Boolean).forEach((spec) => {
      const m = /^(.*)\.(project|global)\.(.+)$/.exec(spec);
      const place = m && isPlace(m[2]) ? m[2] : null;
      const wanted: Array<{ name: string; place: Place | null; id: string }> = m && place
        ? [{ name: m[1], place, id: m[3] }]
        : openedRows.map(({ name }) => ({ name, place: null, id: spec }));
      wanted.forEach(({ name, place, id }) => {
        const r = roleByName(name);
        if (!r) return;
        const places: Place[] = place ? [place] : [eff(r)];
        places.forEach((l) => {
          const entry = r[l];
          if (!entry || !entry.saved) return;
          const hit = rulesOf(entry.saved).find((x) => x.id === id);
          if (hit) { open.add(r.uid); openRuleKey(`${r.uid}${l}:${hit._k}`); }
        });
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
    const open = [...s.open].map((u) => { const r = byUid(u); return r ? (GROUPED ? hqToken(curName(r), r.group) : curName(r)) : ''; }).filter(Boolean);
    if (open.length) q.set('open', open.join(','));
    const rule = [...openRules()].map((kk) => {
      const m = /^(\d+)(project|global):(\d+)$/.exec(kk);
      if (!m) return '';
      const r = byUid(m[1]);
      const place: Place = m[2] === 'project' ? 'project' : 'global';
      const entry = r && r[place];
      if (!r || !entry || !entry.saved) return '';
      const hit = rulesOf(entry.saved).find((x) => x._k === Number(m[3]));
      return hit && hit.id ? `${curName(r)}.${place}.${hit.id}` : '';
    }).filter(Boolean);
    if (rule.length) q.set('rule', rule.join(','));
  }
  const url = path + (q.toString() ? '?' + q.toString() : '');
  if (push) history.pushState(null, '', url); else history.replaceState(null, '', url);
}

// ---- the list's filters ---------------------------------------------
export const FILTERS: Array<[RoleState['filter'], string]> = [[null, 'すべて'], ['off', 'オフがある'], ['issues', '問題・警告']];
export const allEntries = (): FileEntry[] => roles.value.flatMap((r) => PLACES.flatMap((l) => r[l] ? [{ r, l, e: r[l]! }] : []));
export function ruleShown(x: EditableRule, issues: RuleIssue[]): boolean {
  const s = state.value;
  if (s.filter === 'off' && inP(x)) return false;
  if (s.filter === 'issues' && !issues.some((m) => m.k === x._k)) return false;
  if (s.q) { const q = s.q.toLowerCase(); if (![x.id, x.text, x.note].some((v) => String(v).toLowerCase().includes(q))) return false; }
  return true;
}
export const filtering = (): boolean => !!(state.value.filter || state.value.q);
export const rowIssues = (r: RoleRow): RuleIssue[] => PLACES.filter((l) => r[l]).flatMap((l) => issuesOf(r, l));

export function filterCount(f: RoleState['filter']): number {
  if (f === 'off') return roles.value.filter((r) => PLACES.some((l) => r[l] && rulesOf(r[l].draft).some((x) => !inP(x)))).length;
  return allEntries().reduce((c, { r, l, e }) => {
    const is = issuesOf(r, l);
    return c + rulesOf(e.draft).filter((x) => f === null || is.some((m) => m.k === x._k)).length;
  }, 0);
}

// ---- the YAML tab's own state ---------------------------------------
// A file's problems are what the file nav marks a file with; the part checks the
// text and hands them back (yamltab.js).
export const ykey = (place: Place, name: string): string => place + '.' + name;
export const ychecksOf = (place: Place, name: string): components['schemas']['Problem'][] | undefined => ychecks.value[ykey(place, name)];
export const setYchecks = (place: Place, name: string, list: components['schemas']['Problem'][] | undefined): void => { ychecks.value = { ...ychecks.value, [ykey(place, name)]: list }; };
// filesList is the YAML tab's files: this serve's project and the global layer,
// which are the two places the tab writes to. Another project's rows are read and
// written in the list, not here.
export const filesList = (): SavedFileEntry[] => allEntries().filter((entry): entry is SavedFileEntry => !!entry.e.saved && live(entry.r, entry.l) && (!GROUPED || entry.r.group === CURRENT || entry.r.group === 'global'));
export const docText = (r: RoleRow, l: Place): string => { const e = r[l]; return e && e.ytext !== undefined ? e.ytext : (e && e.text) || ''; };

// ---- operations ------------------------------------------------------
// A refusal stands where its field does: a name's refusal under the name field,
// the document's in the panel's footer (spec-roles 1.2.9).
export function showRefusal(key: string, at: unknown, text: string): void {
  patch({ refusals: { ...state.value.refusals, [key]: { at: at === 'name' ? 'name' : 'doc', text: text || '保存できませんでした' } } });
  touch();
}
export function clearRefusal(key: string): void {
  if (!state.value.refusals[key]) return;
  const next = { ...state.value.refusals };
  delete next[key];
  patch({ refusals: next });
  touch();
}

// ---- a save refused because the file moved (r-3c77, d-a34a) ---------------
// conflictAt is the notice one panel stands under: the file as the answer showed
// it, or null when the file is the one the panel read.
export const conflictAt = (key: string): ConflictHold | null => state.value.conflicts[key] || null;
export function clearConflict(key: string): void {
  if (!state.value.conflicts[key]) return;
  const next = { ...state.value.conflicts };
  delete next[key];
  patch({ conflicts: next });
}

// pressOverwrite writes what the person wrote over the file the answer showed:
// the save carries the hash of that file, so a file that moved again is refused
// the same way instead of being written over unseen.
export function pressOverwrite(id: number | string, l: Place): void {
  const hold = conflictAt(id + l);
  if (!hold) return;
  doSave(id, l, hold.c.file.hash || '');
}

// pressRevertConflict reads the file the answer carried into the panel: the
// panel then stands on the file as it is, and the draft the person wrote is kept
// for 取り消す.
export function pressRevertConflict(id: number | string, l: Place): void {
  const key = id + l;
  const hold = conflictAt(key);
  const r = byUid(id);
  const entry = r && r[l];
  if (!hold || !entry || !hold.c.file.doc) return;
  const doc = fromJSON(hold.c.file.doc);
  const before = { draft: clone(entry.draft), saved: entry.saved ? clone(entry.saved) : null, hash: entry.hash, text: entry.text };
  withEntry(id, l, (e) => ({ ...e, draft: clone(doc), saved: clone(doc), hash: hold.c.file.hash || '', text: hold.c.file.text || '' }));
  patch({ conflicts: { ...state.value.conflicts, [key]: { c: hold.c, undone: true, before } } });
  askCheck(byUid(id), l, () => touch());
  touch();
}

// pressUndoConflict puts the words the person wrote back, over the file the panel
// has just read: their draft stands as a change to that file, ready to save.
export function pressUndoConflict(id: number | string, l: Place): void {
  const key = id + l;
  const hold = conflictAt(key);
  const before = hold && hold.before;
  if (!hold || !before) return;
  withEntry(id, l, (e) => ({ ...e, draft: clone(before.draft), saved: before.saved ? clone(before.saved) : null, hash: before.hash, text: before.text }));
  clearConflict(key);
  askCheck(byUid(id), l, () => touch());
  touch();
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
  const d: EditableDocument = { name: '', title: '', revision: 1, sections: [{ _s: 0, heading: '使命', level: 0, intro: '', rules: [{ _k: nextKey(), id: '', text: '', note: '', in_prompt: true, off_since: '' }] }] };
  // The head a person pressed says the place the row stands in (r-410c): the
  // row is that group's, and there is no switch inside it to move it.
  const key = group === undefined ? roleGroups()[0].key : group;
  const place: Place = key === 'global' ? 'global' : 'project';
  const nr: RoleRow = { uid: nextUid(), isNew: true, group: key, project: null, global: null };
  nr[place] = { saved: null, draft: d, hash: '', text: '' };
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
  openRuleKey(`${nr.uid}${place}:${d.sections[0].rules[0]._k}`);
  touch();
  askCheck(nr, place, () => { askFocus(nr.uid, '[data-f=name]'); touch(); });
}

// pressPickPlace is kept for a row that was made before r-410c and still holds
// both places in one record: it moves the record to the place a person picked.
export function pressPickPlace(id: number | string, l: Place, x: Place): void {
  if (x === l) return;
  withRow(id, (r) => ({ ...r, [x]: r[l], [l]: null }));
  touch();
}

export function pressPmode(id: number | string, l: Place, v: string): void {
  // The switch's button is the one the person pressed: it keeps the focus
  // through the draw, so nothing is put back (5 章 5).
  patch({ pmode: { ...state.value.pmode, [id + l]: v } });
  touch();
}

export function pressDup(id: number | string, l: Place): void {
  // The copy keeps the places its rules stood in (their _k), so the server
  // writes the new file from the original's own lines (spec-roles 1.2.9).
  const r = byUid(id);
  const src = r && r[l] && r[l].saved;
  if (!src) return;
  const d = clone(src);
  d.name += '-2';
  const nr: RoleRow = { uid: nextUid(), isNew: true, group: r.group, project: null, global: null };
  nr[l] = { saved: null, draft: d, base: clone(src), basePlace: { place: l, name: src.name }, hash: '', text: '' };
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
  const e: RoleEntry = { saved: null, draft: clone(r[l]!.saved!), base: clone(r[l]!.saved!), basePlace: { place: l, name: r[l]!.saved!.name }, hash: '', text: '' };
  // The list draws each place as its own row under its head; the copy stands in
  // the place it was copied to, not in the row it came from (r-f781, r-410c).
  const group = copyGroupOf(to, groupOfRow(r), CURRENT);
  const nr: RoleRow = { uid: nextUid(), isNew: true, group, project: null, global: null };
  nr[to] = e;
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

export function pressDel(id: number | string, l: Place): void {
  patch({ confirm: { ...state.value.confirm, [id + l]: 1 } });
  askFocus(id, '[data-act=del-yes]');
  touch();
}

export function pressDelNo(id: number | string, l: Place): void {
  const next = { ...state.value.confirm };
  delete next[id + l];
  patch({ confirm: next });
  touch();
}

export function pressDiscard(id: number | string, l: Place): void {
  const next = { ...state.value.confirm };
  delete next[id + l];
  patch({ confirm: next });
  const r = byUid(id);
  if (!r) return;
  const entry = r[l];
  if (entry && entry.saved) {
    withEntry(id, l, (e) => ({ ...e, draft: clone(entry.saved!) }));
  } else {
    withRow(id, (old) => ({ ...old, [l]: null }));
  }
  const after = byUid(id);
  if (after && !after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== id);
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
  // The select is the same element after the draw, so the cursor stays in it:
  // nothing puts it back (5 章 5).
  teams.value = teams.value.map((one) => (one.id !== id
    ? one
    : { ...one, members: one.members.map((m, j) => (j === Number(i) ? { ...m, draft: value } : m)) }));
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

export function pressYamlRevert(r: RoleRow | null | undefined, l: Place): void {
  // The text goes back to the saved one, and the shared part checks the text it
  // now shows: a result for the thrown-away text must not stay.
  if (r && r[l]) withEntry(r.uid, l, (e) => { const next = { ...e }; delete next.ytext; return next; });
}

// ---- writes ----------------------------------------------------------
// rebase puts the files a save's answer read again under the rows' feet
// (r-b41c). A save gives the rules their ids after it writes (d-4aab), so a
// document the person did not touch can move while another one is saved: a row
// that kept the hash it read before would be refused at its own next save as a
// change another place made. The row's own document is the answer's, and every
// other file takes the hash the answer read; the draft a person is writing in
// stays where it is.
function rebase(answer: { hashes?: Record<string, string> | null } | null): void {
  const files = bases(answer);
  if (!files.length) return;
  roles.value = roles.value.map((r) => {
    let row = r;
    for (const b of files) {
      const l: Place | null = b.place === 'project' ? 'project' : b.place === 'global' ? 'global' : null;
      const e = l ? row[l] : null;
      if (!l || !e || !e.saved || e.saved.name !== b.name || e.hash === b.hash) continue;
      row = { ...row, [l]: { ...e, hash: b.hash } };
    }
    return row;
  });
}

export async function doSave(id: number | string, l: Place, over?: string): Promise<void> {
  const r = byUid(id);
  const e = r && r[l];
  if (!e) return;
  const key = id + l;
  // Whether the rename carries into the teams that read the role is the rule
  // document's state, not a box the screen reads back (5 章 2).
  const keepRename = renameCarries(key);
  // over is the hash the conflict's answer carried: 上書き sends the file that
  // was seen, and a file that moved again is refused the same way (r-3c77).
  const payload: components['schemas']['RoleSaveRequest'] = { place: l, original: e.saved ? e.saved.name : null, doc: toDoc(e.draft, e.saved || e.base || null), rename_users: keepRename, hash: over === undefined ? (e.hash || '') : over };
  if (r.group && r.group !== 'global') payload.project = r.group;
  if (e.basePlace) payload.base = e.basePlace;
  let res: RawAnswer<Answer<'role', 'save'>>;
  try {
    res = await apiSave('role', e.draft.name || (e.saved ? e.saved.name : '_'), payload);
  } catch (err) {
    const c = conflictOf(err);
    if (c) {
      // The file moved: the panel keeps its draft and the notice stands above
      // the save band (r-3c77, d-a34a).
      patch({ conflicts: { ...state.value.conflicts, [key]: { c } } });
      return;
    }
    showRefusal(key, refusalAt(err), problemText(err, '保存できませんでした'));
    return;
  }
  clearRefusal(key);
  clearConflict(key);
  const json = res.json;
  if (!json) return;
  const now = byUid(id);
  const cur0 = now && now[l];
  if (!cur0) return;
  const old = cur0.saved;
  const newName = cur0.draft.name;
  // The written document comes back with the answer: hergy wrote the day a rule
  // was switched off (off_since), so the screen takes the document the server
  // made rather than the one it sent (d-85dc).
  const saved = json.doc ? fromJSON(json.doc) : clone(cur0.draft);
  withEntry(id, l, (one) => {
    const next = { ...one, hash: json.hash || one.hash, saved, draft: clone(saved) };
    // The written text comes back with the answer, so the YAML tab shows the file
    // this save just made (a moved line, a rule the write re-emitted).
    if (typeof json.text === 'string') next.text = json.text;
    delete next.base;
    delete next.basePlace;
    delete next.ytext;
    return next;
  });
  withRow(id, (one) => {
    const next = { ...one };
    delete next.isNew;
    delete next.noDoc;
    return next;
  });
  if (old && old.name !== newName && keepRename) {
    teams.value = teams.value.map((t) => ({
      ...t,
      members: t.members.map((m) => ({ ...m, saved: m.saved === old.name ? newName : m.saved, draft: m.draft === old.name ? newName : m.draft })),
    }));
    const o = other(l);
    if (!GROUPED && now[o]) {
      const split: RoleRow = { uid: nextUid(), project: null, global: null, group: now.group };
      split[o] = now[o];
      const at = roles.value.findIndex((x) => x.uid === id);
      roles.value = [...roles.value.slice(0, at + 1), split, ...roles.value.slice(at + 1)];
      withRow(id, (one) => ({ ...one, [o]: null }));
    }
  }
  setYchecks(l, newName, undefined);
  patch({ ok: { ...state.value.ok, [key]: 1 } });
  rebase(json);
  await askCheck(byUid(id), l, null);
  touch();
  // A rename changes the name the address opens the row by, so the address
  // follows the save (spec-roles 2 節).
  syncURL(false);
  apiSaved();
}

export async function doDelete(id: number | string, l: Place): Promise<void> {
  const r = byUid(id);
  const e = r && r[l];
  if (!e || !e.saved) return;
  try {
    await apiDel('role', e.saved.name, l, e.hash || '', undefined, r.group && r.group !== 'global' ? r.group : undefined);
  } catch (err) {
    showRefusal(id + l, refusalAt(err), problemText(err, '削除できませんでした'));
    return;
  }
  setYchecks(l, e.saved.name, undefined);
  withRow(id, (one) => ({ ...one, [l]: null }));
  const after = byUid(id);
  if (!after) return;
  if (!after.project && !after.global) {
    roles.value = roles.value.filter((x) => x.uid !== Number(id));
    const open = new Set(state.value.open);
    open.delete(Number(id));
    patch({ open, tab: 'roles' });
  } else {
    patch({ tab: 'roles' });
  }
  touch();
  syncURL(false);
}

export async function doAssignSave(id: string): Promise<void> {
  const t = teams.value.find((x) => x.id === id);
  if (!t) return;
  const changes = t.members.filter((m) => m.draft !== m.saved).map((m) => ({ member: m.name, role: m.draft }));
  if (!changes.length) return;
  // The pick travels as the team kind's own op, so the save writes one line of
  // team.yaml and no other (spec-boundary 6, spec-teams 1.3). The place is the
  // card's own: a global definition is saved in the global file, and every
  // project reading it changes (d-0f32, r-f8b9).
  let res: RawAnswer<Answer<'team', 'save'>>;
  try {
    const request: Request<'team', 'save'> = { place: t.place || 'project', part: 'team', hash: t.hash || '', doc: { ops: changes.map((m) => ({ kind: 'memberRole', member: m.member, role: m.role })) } };
    res = await apiSave('team', id, request);
  } catch (err) {
    patch({ teamErrors: { ...state.value.teamErrors, [id]: problemText(err, '保存できませんでした') } });
    touch();
    return;
  }
  const json = res.json;
  if (!json) return;
  const nextErrors = { ...state.value.teamErrors };
  delete nextErrors[id];
  patch({ teamErrors: nextErrors, ok: { ...state.value.ok, ['t' + id]: 1 } });
  teams.value = teams.value.map((one) => (one.id !== id ? one : {
    ...one,
    hash: json.hash || one.hash,
    members: one.members.map((m) => ({ ...m, saved: m.draft })),
  }));
  for (const r of savedRoles()) await askCheck(r, eff(r), null);
  touch();
  apiSaved();
}

// ---- 帯への報告と持ち越し (spec-nav.md 1.4, 1.5) ---------------------
// dirty is whether the screen holds something no save wrote yet: a row's draft,
// a row a person added, a pick in the assign tab, or a YAML text.
function rowDirty(r: RoleRow): boolean {
  return PLACES.some((l) => { const entry = r[l]; return !!entry && (!entry.saved || JSON.stringify(entry.draft) !== JSON.stringify(entry.saved)); });
}

export function dirty(): boolean {
  const rows = roles.value.some(rowDirty);
  const members = teams.value.some((t) => t.members.some((m) => m.draft !== m.saved));
  const ytexts = allEntries().some(({ e }) => e.ytext !== undefined && e.ytext !== (e.text || ''));
  return rows || members || ytexts;
}

export function tellDirty(): void {
  setDirty(dirty());
}

// take is the draft the band puts aside: every row's draft that is not what the
// file holds, every row a person added, every pick that was made, every YAML
// text that was typed — each with the hash of the file it was read from, so a
// file that moved outside is not written over by a page that read another one.
export function take(): CarriedRoles {
  const rows: CarriedRole[] = [], ytexts: CarriedYaml[] = [], members: CarriedMember[] = [];
  roles.value.forEach((r) => {
    PLACES.forEach((l) => {
      const e = r[l];
      if (!e) return;
      if (!e.saved) {
        rows.push({ place: l, saved: null, name: e.draft.name, hash: e.hash || '', draft: e.draft, isNew: !!r.isNew, noDoc: !!r.noDoc });
      } else if (JSON.stringify(e.draft) !== JSON.stringify(e.saved)) {
        rows.push({ place: l, saved: e.saved.name, name: e.draft.name, hash: e.hash || '', draft: e.draft });
      }
      if (e.ytext !== undefined && e.ytext !== (e.text || '')) {
        ytexts.push({ place: l, name: (e.saved || e.draft).name, hash: e.hash || '', text: e.ytext });
      }
    });
  });
  teams.value.forEach((t) => t.members.forEach((m) => {
    if (m.draft !== m.saved) members.push({ team: t.id, hash: t.hash || '', name: m.name, saved: m.saved, draft: m.draft });
  }));
  return { rows, ytexts, members };
}

// restore puts the carried draft back, skipping every part whose file moved
// outside since the draft was made (spec-nav.md 1.5).
export function restore(carried: CarriedRoles): void {
  (carried.rows || []).forEach((c) => {
    const r = roles.value.find((x) => {
      const entry = x[c.place];
      return c.saved
        ? !!entry?.saved && entry.saved.name === c.saved
        : !!entry && !entry.saved && (x.noDoc || x.isNew || entry.draft.name === c.name);
    });
    if (!r) return;
    const e = r[c.place];
    if (!e) return;
    if (c.hash && e.hash !== c.hash) return;
    observeKeys(rulesOf(c.draft));
    withEntry(r.uid, c.place, (one) => ({ ...one, draft: clone(c.draft) }));
    if (c.isNew) withRow(r.uid, (one) => ({ ...one, isNew: true }));
    clearTimer(r, c.place);
    askCheck(byUid(r.uid), c.place, null);
  });
  (carried.ytexts || []).forEach((c) => {
    const r = roles.value.find((x) => { const entry = x[c.place]; return !!entry && (entry.saved || entry.draft).name === c.name; });
    if (!r) return;
    const e = r[c.place];
    if (!e) return;
    if (c.hash && e.hash !== c.hash) return;
    withEntry(r.uid, c.place, (one) => ({ ...one, ytext: c.text }));
  });
  (carried.members || []).forEach((c) => {
    const t = teams.value.find((x) => x.id === c.team);
    if (!t || (c.hash && t.hash !== c.hash)) return;
    const at = t.members.findIndex((x) => x.name === c.name && x.saved === c.saved);
    if (at >= 0) {
      teams.value = teams.value.map((one) => (one.id !== t.id ? one : { ...one, members: one.members.map((m, i) => (i === at ? { ...m, draft: c.draft } : m)) }));
    }
  });
  touch();
}

// boot reads the address, asks every saved document's check, and keeps what the
// band carried from the screen before this one (spec-nav.md 1.5).
export async function boot(): Promise<void> {
  parseURL();
  await Promise.all(roles.value.flatMap((r) => PLACES.filter((l) => r[l] && r[l].saved).map((l) => askCheck(r, l, () => { updateRowSummary(r.uid); }))));
  roles.value.forEach((r) => updateRowSummary(r.uid));
  const carried = takeDraft('roles');
  if (carried) restore(carried);
  touch();
}
