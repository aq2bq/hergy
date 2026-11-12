// lib/hq/state.js — 本部の読みと、画面をまたぐ小さな状態（n-7f3c、モデルは d-73fc）。
//
// ここが持つのは、本部の API（/api/hq/…）から読んだ状態と、その読み方だけ:
// 並べたプロジェクト、動いているチーム、ワークスペース、探す欄が読む一覧
// （チーム定義・ロール・プロファイル）、そして 10 秒ごとの読み。帯の数
// （プロジェクト N・アクティビティ N・待ち N）もここが書く（spec 2 章）。
//
// 描くのは lib/hq/figure.js（図）、lib/hq/sheets.js（欄と探す欄）、
// lib/hq/run.js（チーム定義の一覧の「動き」の欄）。入口は lib/hq.js で、
// 画面は lib/hq.js から import して読む。
import { runDanger } from '../danger.js';
import type { DangerOptions } from '../danger.js';
import { signal } from '../preact.js';
import { read, call, problemText } from '../api.js';
import { bandCounts, bandActivity } from '../band-counts.js';
import type { components } from '../../../../apicontract/api-types';

type HQProjects = components['schemas']['HQProjects'];
type HQRunning = components['schemas']['HQRunning'];
type HQRunningTeam = components['schemas']['HQRunningTeam'];
type HQWorkspace = components['schemas']['HQWorkspace'];
type HQTeamGroups = components['schemas']['HQTeamGroups'];
type HQRoleGroups = components['schemas']['HQRoleGroups'];
type HQProfileGroups = components['schemas']['HQProfileGroups'];
type HQTeamGroup = components['schemas']['HQTeamGroup'];
type HQRoleGroup = components['schemas']['HQRoleGroup'];
type HQProfileGroup = components['schemas']['HQProfileGroup'];
type TeamItem = components['schemas']['TeamItem'];
type TeamDocument = components['schemas']['TeamDocument'];
type AnyGroup = HQTeamGroup | HQRoleGroup | HQProfileGroup;
type HQLaunch = components['schemas']['HQLaunch'];
type GroupKind = 'teams' | 'roles' | 'profiles';
type GroupSets = { teams: HQTeamGroup[]; roles: HQRoleGroup[]; profiles: HQProfileGroup[] };
export type RunningTeam = Omit<HQRunningTeam, 'not_up'> & { not_up?: string[]; provisional?: boolean };
type RunningState = Omit<HQRunning, 'teams'> & { teams: RunningTeam[] };

export { bandActivity } from '../band-counts.js';

export const POLL = 10000;
// esc is the words a sheet puts in its markup (the sheets that are still strings:
// the run cell of the team definition's list).
const ESCAPE: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
export const esc = (s: unknown): string => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ESCAPE[c]);

// st holds the headquarters' readings. Every field is a signal of its own, and a
// reading puts a new value in: the trees that read one are drawn again, and no
// screen keeps a number to know that it must (webui-components 1 章). groups は、
// プロジェクトごとの組の配列（API の groups と同じ形）。組は最後にグローバルが 1 つ。
export const st = {
  projects: signal<HQProjects | null>(null),
  running: signal<RunningState>({ herdr: { ok: true, text: '' }, teams: [] }),
  workspaces: signal<HQWorkspace[]>([]),
  groups: signal<GroupSets>({ teams: [], roles: [], profiles: [] }),
  loaded: signal<boolean>(false),
  groupsLoaded: signal<boolean>(false),
};

// setRunning puts the running teams the activity screen read into the same
// reading the list's 動き column uses (api.md 3.5).
export function setRunning(running: HQRunning): void {
  st.running.value = running;
}

export const groupKey = (g: AnyGroup): string => (g && g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupLabel = (g: AnyGroup): string | undefined => (g && g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const groupDisplay = (g: AnyGroup): string | undefined => (g && g.global ? g.display : (g.project ? g.project.display : ''));
// projectOfGroup is the key a row of one group carries in the URL and in the API:
// the project's path, or "global".
export const projectOfGroup = (g: AnyGroup): string => (g && g.global ? 'global' : (g.project ? g.project.path : ''));

export const projectName = (path: string): string => {
  if (!path) return '';
  const found = ((st.projects.value && st.projects.value.projects) || []).find((p) => p.path === path);
  if (found) return found.name;
  if (path === 'global') return 'グローバル';
  return String(path).split('/').filter(Boolean).pop() || path;
};

// ---- 読み -------------------------------------------------------------
// readJSON is one reading that leaves the state as it was when the server says
// no: a page that lost the headquarters keeps what it had.
export async function readJSONOr<T>(url: string): Promise<T | null> {
  try { return await read<T>(url); } catch { return null; }
}

export function groupOf(kind: 'teams'): HQTeamGroup[];
export function groupOf(kind: 'roles'): HQRoleGroup[];
export function groupOf(kind: 'profiles'): HQProfileGroup[];
export function groupOf(kind: GroupKind): AnyGroup[];
export function groupOf(kind: GroupKind): AnyGroup[] {
  return st.groups.value[kind] || [];
}
export const itemGroup = (_kind: GroupKind, _name: string, group: string): AnyGroup | null => groupOf(_kind).find((g) => groupKey(g) === group) || null;

// runningFor is the running teams a team definition has: its own, of its place
// and its project (spec 5 章).
export function runningFor(definition: string, place: string, project: string): RunningTeam[] {
  return (st.running.value.teams || []).filter((t) => t.definition === definition
    && (t.place || 'project') === (place || 'project')
    && (place === 'global' || !project || t.project === project));
}
export const runningTeam = (name: string): RunningTeam | null => st.running.value.teams.find((t) => t.team === name) || null;

function teamDocumentAt(item: TeamItem, place: string): TeamDocument | null {
  const row = item.places[place];
  return row ? row.doc : null;
}

// lastWorkspaceOf is where a team definition ran last, as the launch record wrote
// it (the workspace whose last_team is this definition). Empty when the record
// has none.
export function lastWorkspaceOf(definition: string): string {
  const ws = (st.workspaces.value || []).find((w) => w.last_team === definition);
  return ws ? ws.label : '';
}

// definitionOf is one team definition's document, from the groups the search
// reads: the activity's figure uses its turns (spec 6 章). An explicit place
// selects only that source; callers without a place keep the effective lookup.
export function definitionOf(project: string, definition: string, place?: 'project' | 'global'): TeamDocument | null {
  const groups = groupOf('teams');
  if (place) {
    const group = groups.find((g) => place === 'global' ? g.global : !g.global && projectOfGroup(g) === project);
    const item = group?.list?.items?.find((x) => x.name === definition);
    return item ? teamDocumentAt(item, place) : null;
  }
  for (const g of groups) {
    if (project && projectOfGroup(g) !== project && !g.global) continue;
    const item = ((g.list && g.list.items) || []).find((x) => x.name === definition);
    if (!item) continue;
    const places = item.places || {};
    const eff = item.effective || (places.project ? 'project' : 'global');
    return teamDocumentAt(item, eff);
  }
  return null;
}

// globalDefinition is one global team definition's own document, for the start
// sheet of a global definition (its members do not depend on the project).
export function globalDefinition(definition: string): TeamDocument | null {
  const g = groupOf('teams').find((x) => x.global);
  if (!g) return null;
  const item = ((g.list && g.list.items) || []).find((x) => x.name === definition);
  if (!item) return null;
  return teamDocumentAt(item, 'global') || teamDocumentAt(item, item.effective);
}

// definitionDoc is the definition's own reading for a start sheet: the project's
// own for a project's definition, the global one otherwise.
export function definitionDoc(definition: string, project: string): TeamDocument | null {
  if (project) {
    const own = definitionOf(project, definition);
    if (own) return own;
  }
  return globalDefinition(definition) || definitionOf('', definition);
}

// refreshCounts is what a save, delete, copy or rename asks for: the projects
// reading again, so the band counts the rows that are there now (ac-7fe6).
let pending: number | null = null;
export function refreshCounts(): void {
  if (pending !== null) return;
  pending = window.setTimeout(() => {
    pending = null;
    void readJSONOr<HQProjects>('/api/hq/projects').then((projects) => {
      if (!projects) return;
      st.projects.value = projects;
      bandCounts();
    });
  }, 150);
}

// A write on any screen asks the band to count again. One listener, so no screen
// has to remember (r-ea17).
document.addEventListener('hergy:wrote', () => refreshCounts());

// say tells the screens a reading came: the activity screen reads its own view
// again when one does. What the parts draw is the signals themselves.
export function say() {
  document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
}

// readCore is what every screen needs at once: the projects, the running teams
// and the workspaces. The groups a search reads are read when the search is
// opened (readGroups), so a page that never searches does not make every
// project's catalog be read.
export async function readCore() {
  const [projects, running, workspaces] = await Promise.all([
    readJSONOr<HQProjects>('/api/hq/projects'),
    readJSONOr<HQRunning>('/api/hq/running'),
    readJSONOr<components['schemas']['HQWorkspaces']>('/api/hq/workspaces'),
  ]);
  if (projects) st.projects.value = projects;
  if (running) st.running.value = running;
  if (workspaces) st.workspaces.value = workspaces.workspaces || [];
  st.loaded.value = true;
  bandActivity();
  bandCounts();
  say();
}

// readGroups reads the lists a search looks through: the team definitions, roles
// and profiles of every listed project and the global layer.
export async function readGroups() {
  if (st.groupsLoaded.value) return;
  const [teams, roles, profiles] = await Promise.all([
    readJSONOr<HQTeamGroups>('/api/hq/teams'),
    readJSONOr<HQRoleGroups>('/api/hq/roles'),
    readJSONOr<HQProfileGroups>('/api/hq/profiles'),
  ]);
  st.groups.value = {
    teams: (teams && teams.groups) || [],
    roles: (roles && roles.groups) || [],
    profiles: (profiles && profiles.groups) || [],
  };
  st.groupsLoaded.value = true;
  say();
}

// readAll is both readings, in order: a start or a stop asks for the groups too,
// because a project's row may have moved with it.
export async function readAll() {
  await readCore();
  await readGroups();
}

// readRunning is the ten-second reading: the teams and the workspaces, which move
// without a reload. The groups and the projects stay as they were.
export async function readRunning() {
  const [running, workspaces] = await Promise.all([
    readJSONOr<HQRunning>('/api/hq/running'),
    readJSONOr<components['schemas']['HQWorkspaces']>('/api/hq/workspaces'),
  ]);
  if (running) st.running.value = mergeRunning(running);
  if (workspaces) st.workspaces.value = workspaces.workspaces || [];
  bandActivity();
  say();
}

// sameTeam is the team a row is about: its name and its project. Two readings of
// one team are one row, whether one of them is the provisional line a start adds
// before herdr has a tab (r-5b96 audit S1).
const sameTeam = (t: Pick<HQRunningTeam, 'team' | 'project'>): string => t.team + '\u0000' + (t.project || '');
function mergeRunning(running: HQRunning): RunningState {
  const real: RunningTeam[] = (running.teams || []).slice();
  const seen = new Set(real.map(sameTeam));
  // A provisional row stands only for a team the real reading has not found yet:
  // once it is found, the provisional one goes at once.
  const provisional = (st.running.value.teams || []).filter((t) => t.provisional && !seen.has(sameTeam(t)));
  return Object.assign({}, running, { teams: real.concat(provisional) });
}

// noteStarting tells the list a team is starting now, before herdr has a tab for
// it: the row says 起動の途中 at once, and the next reading replaces this with the
// team as herdr sees it (r-5b96 audit R5).
export function noteStarting(launch: HQLaunch): void {
  const teams = (st.running.value.teams || []).filter((t) => sameTeam(t) !== sameTeam(launch));
  teams.push({
    team: launch.team, definition: launch.definition, place: launch.place,
    project: launch.project, project_name: launch.project_name,
    workspace: launch.workspace, tab: launch.tab,
    starting: true, launch: launch.id, members: [], workers: [], waiting: [],
    provisional: true,
  });
  st.running.value = { ...st.running.value, teams };
  bandActivity();
  say();
  // The tab is made within the first second; a few readings bring the team in
  // without waiting for the ten-second one, whether or not the sheet is open.
  [600, 1500, 3000, 6000].forEach((ms) => setTimeout(readRunning, ms));
}

// ---- 書き（並べる・外す） ---------------------------------------------
// postProject lists one project. The sheet shows the server's own sentence when
// it says no; the answer is the refusal, or null.
export async function postProject(path: string, at?: number): Promise<string | null> {
  try {
    const request = at === undefined ? { path } : { path, at };
    const answer = await call<HQProjects>('POST', '/api/hq/projects', request);
    // 取り消す puts the project back: the answer is the list as it now reads.
    if (answer.json) st.projects.value = answer.json;
    return null;
  } catch (e) { return problemText(e, '並べられませんでした'); }
}
// unlistProject takes one project out and leaves 取り消す in its row: the notice
// puts it back at the position the answer named (d-2cce). The screen does not
// reload; the reading is the answer's. The notice follows the project's own row:
// the pressed row's element stands for the next project once the list is drawn
// again, so the row is found by path after the redraw (d-aeeb).
export function unlistProject(path: string, host?: HTMLElement | null): Promise<string | null> {
  return new Promise((resolve) => {
    let at = -1;
    // The section the pressed row stands in: where the notice goes when the
    // project leaves the screen as well as the list (no launch record).
    const section = (host && host.closest('.hq-sec')) as HTMLElement | null;
    const opts: DangerOptions = {
      run: async () => {
        try {
          const answer = await call<HQProjects>('DELETE', `/api/hq/projects?path=${encodeURIComponent(String(path))}`);
          const json = answer.json;
          if (json) {
            at = json.removed_at ?? -1;
            st.projects.value = json;
          }
          opts.host = (await rowOfProject(path, host ?? null)) ?? section ?? host ?? null;
          resolve(null);
        } catch (e) { resolve(problemText(e, '外せませんでした')); }
      },
      // The server names the position, 0 included; a position it did not name
      // is said, not passed over.
      restore: () => at >= 0
        ? postProject(path, at)
        : Promise.resolve('外した位置が分からず、並びに戻せませんでした'),
      said: '並びから外しました',
      host,
    };
    runDanger('unlistProject', opts);
  });
}
// rowOfProject is the row that names one project once the list has been drawn
// again, so the notice stands in that project's row wherever it moved. The row
// that was pressed may be the one drawn before the answer (the list is redrawn a
// turn later), and a moved row is a new element: the pressed one is passed over
// until the redrawn row stands (a bounded wait), and position is never used.
async function rowOfProject(path: string, pressed: HTMLElement | null): Promise<HTMLElement | null> {
  let last: HTMLElement | null = null;
  for (let tries = 0; tries < 12; tries++) {
    const rows = [...document.querySelectorAll<HTMLElement>('.hq-sheet .hq-li[data-path]')]
      .filter((row) => row.dataset.path === path);
    if (rows.length) last = rows[rows.length - 1];
    const fresh = rows.find((row) => row !== pressed);
    if (fresh) return fresh;
    await new Promise<void>((done) => {
      if (typeof requestAnimationFrame === 'function') requestAnimationFrame(() => done());
      else setTimeout(done, 0);
    });
  }
  return last;
}
// listProject lists one project. Without projects:, the project serve was started
// in is listed first, so listing another one does not drop the teams a person is
// looking at (r-5b96 audit 7).
export async function listProject(path: string): Promise<string | null> {
  const p = st.projects.value;
  if (p && p.mode !== 'listed' && p.current && p.current.path && p.current.path !== path) {
    const first = await postProject(p.current.path);
    if (first) return first;
  }
  return postProject(path);
}

// ---- the ten-second reading ------------------------------------------
let timer = 0;
export function startReads(): void {
  if (!timer) timer = setInterval(() => { if (document.visibilityState === 'visible') readRunning(); }, POLL);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') readRunning(); });
}
