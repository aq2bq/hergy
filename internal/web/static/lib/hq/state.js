// lib/hq/state.js — 本部の読みと、画面をまたぐ小さな約束（n-7f3c、モデルは d-73fc）。
//
// ここが持つのは、本部の API（/api/hq/…）から読んだ状態と、その読み方だけ:
// 並べたプロジェクト、動いているチーム、ワークスペース、検索が読む一覧
// （チーム定義・ロール・プロファイル）、そして 10 秒ごとの読み。帯の数
// （プロジェクト N・活動 N・待ち N）もここが書く（spec 2 章）。
//
// 描くのは lib/hq/figure.js（図）、lib/hq/sheets.js（欄と検索）、
// lib/hq/run.js（チーム定義の一覧の「動き」の欄）。入口は lib/hq.js で、
// 画面は lib/hq.js から import して読む。
import { signal } from '../../vendor/preact.js';
import { read, call, problemText } from '../api.js';
import { setActivity, setCount } from '../nav.js';

export const POLL = 10000;
// esc is the words a sheet puts in its markup (the sheets that are still strings:
// the run cell of the team definition's list).
export const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

// groups は、プロジェクトごとの組の配列（API の groups と同じ形）。組は最後に
// グローバルが 1 つ。
export const st = {
  projects: null,
  running: { herdr: { ok: true, text: '' }, teams: [] },
  workspaces: [],
  groups: { teams: [], roles: [], profiles: [] },
  loaded: false,
};

// rev is the number that says a reading changed. The parts that draw from st read
// it, so a reading redraws them without a screen asking.
export const rev = signal(0);

export const groupKey = (g) => (g && g.global ? 'global' : (g.project ? g.project.path : ''));
export const groupLabel = (g) => (g && g.global ? 'グローバル' : (g.project ? g.project.name : ''));
export const groupDisplay = (g) => (g && g.global ? g.display : (g.project ? g.project.display : ''));
// projectOfGroup is the key a row of one group carries in the URL and in the API:
// the project's path, or "global".
export const projectOfGroup = (g) => (g && g.global ? 'global' : (g.project ? g.project.path : ''));

// token joins a row's name and its group for the address: a name alone is the one
// the screen was opened at (the old way), and a name with a group says which
// project's row it is (spec 4 章).
export const token = (name, group) => (group ? name + '|' + group : name);
export function parseOpen(value) {
  return String(value || '').split(',').filter(Boolean).map((one) => {
    const i = one.lastIndexOf('|');
    if (i < 0) return { name: one, group: '' };
    return { name: one.slice(0, i), group: one.slice(i + 1) };
  });
}

export const projectName = (path) => {
  if (!path) return '';
  const found = ((st.projects && st.projects.projects) || []).find((p) => p.path === path);
  if (found) return found.name;
  if (path === 'global') return 'グローバル';
  return String(path).split('/').filter(Boolean).pop() || path;
};

// ---- 読み -------------------------------------------------------------
// readJSON is one reading that leaves the state as it was when the server says
// no: a page that lost the headquarters keeps what it had.
export async function readJSONOr(url) {
  try { return await read(url); } catch (e) { return null; }
}

export const groupOf = (kind) => st.groups[kind] || [];
export const itemGroup = (kind, name, group) => groupOf(kind).find((g) => groupKey(g) === group) || null;

// runningFor is the running teams a team definition has: its own, of its place
// and its project (spec 5 章).
export function runningFor(definition, place, project) {
  return (st.running.teams || []).filter((t) => t.definition === definition
    && (t.place || 'project') === (place || 'project')
    && (place === 'global' || !project || t.project === project));
}
export const runningTeam = (name) => (st.running.teams || []).find((t) => t.team === name) || null;

// lastWorkspaceOf is where a team definition ran last, as the launch record wrote
// it (the workspace whose last_team is this definition). Empty when the record
// has none.
export function lastWorkspaceOf(definition) {
  const ws = (st.workspaces || []).find((w) => w.last_team === definition);
  return ws ? ws.label : '';
}

// definitionOf is one team definition's document, from the groups the search
// reads: the activity's figure uses its turns (spec 6 章).
export function definitionOf(project, definition) {
  const groups = groupOf('teams');
  for (const g of groups) {
    if (project && projectOfGroup(g) !== project && !g.global) continue;
    const item = (g.list ? g.list.items : []).find((x) => x.name === definition);
    if (!item) continue;
    const places = item.places || {};
    const eff = item.effective || (places.project ? 'project' : 'global');
    return (places[eff] || {}).doc || null;
  }
  return null;
}

// globalDefinition is one global team definition's own document, for the start
// sheet of a global definition (its members do not depend on the project).
export function globalDefinition(definition) {
  const g = groupOf('teams').find((x) => x.global);
  if (!g) return null;
  const item = (g.list ? g.list.items : []).find((x) => x.name === definition);
  if (!item) return null;
  const places = item.places || {};
  return (places.global || places[item.effective] || {}).doc || null;
}

// definitionDoc is the definition's own reading for a start sheet: the project's
// own for a project's definition, the global one otherwise.
export function definitionDoc(definition, project) {
  if (project) {
    const own = definitionOf(project, definition);
    if (own) return own;
  }
  return globalDefinition(definition) || definitionOf('', definition);
}

// ---- 帯 ---------------------------------------------------------------
// bandActivity keeps the band's two numbers the reading's own: 活動 N（動いている
// チーム）と 待ち N（人間を待っているメンバー）(spec 2 章, spec-activity 2.1).
export function bandActivity() {
  const teams = st.running.teams || [];
  const waiting = teams.reduce((n, t) => n + (t.waiting || []).length, 0);
  setActivity(teams.length, waiting, !!st.running.herdr.ok && teams.length > 0);
}

// bandCounts writes the projects reading's totals into the band, so the four
// numbers are the listed projects' and the global layer's (spec 2 章). The server
// writes them too; this keeps them right after a project is listed or removed
// without a reload.
export function bandCounts() {
  const p = st.projects;
  if (!p) return;
  if (p.mode !== 'listed') return;
  const totals = { teams: 0, roles: 0, profiles: 0 };
  ((p.projects || []).filter((x) => x.listed)).forEach((x) => {
    totals.teams += x.definitions || 0;
    totals.roles += x.roles || 0;
    totals.profiles += x.profiles || 0;
  });
  totals.teams += (p.global && p.global.definitions) || 0;
  totals.roles += (p.global && p.global.roles) || 0;
  totals.profiles += (p.global && p.global.profiles) || 0;
  setCount('teams', totals.teams);
  setCount('roles', totals.roles);
  setCount('profiles', totals.profiles);
}

// say tells the screens a reading changed: the parts that draw it read rev.
export function say() {
  rev.value += 1;
  document.dispatchEvent(new CustomEvent('hergy:hq', { detail: st }));
}

// readCore is what every screen needs at once: the projects, the running teams
// and the workspaces. The groups a search reads are read when the search is
// opened (readGroups), so a page that never searches does not make every
// project's catalog be read.
export async function readCore() {
  const [projects, running, workspaces] = await Promise.all([
    readJSONOr('/api/hq/projects'),
    readJSONOr('/api/hq/running'),
    readJSONOr('/api/hq/workspaces'),
  ]);
  if (projects) st.projects = projects;
  if (running) st.running = running;
  if (workspaces) st.workspaces = workspaces.workspaces || [];
  st.loaded = true;
  bandActivity();
  bandCounts();
  say();
}

// readGroups reads the lists a search looks through: the team definitions, roles
// and profiles of every listed project and the global layer.
export async function readGroups() {
  if (st.groupsLoaded) return;
  const [teams, roles, profiles] = await Promise.all([
    readJSONOr('/api/hq/teams'),
    readJSONOr('/api/hq/roles'),
    readJSONOr('/api/hq/profiles'),
  ]);
  st.groups.teams = (teams && teams.groups) || [];
  st.groups.roles = (roles && roles.groups) || [];
  st.groups.profiles = (profiles && profiles.groups) || [];
  st.groupsLoaded = true;
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
  const [running, workspaces] = await Promise.all([readJSONOr('/api/hq/running'), readJSONOr('/api/hq/workspaces')]);
  if (running) st.running = mergeRunning(running);
  if (workspaces) st.workspaces = workspaces.workspaces || [];
  bandActivity();
  say();
}

// sameTeam is the team a row is about: its name and its project. Two readings of
// one team are one row, whether one of them is the provisional line a start adds
// before herdr has a tab (r-5b96 audit S1).
const sameTeam = (t) => t.team + '\u0000' + (t.project || '');
function mergeRunning(running) {
  const real = (running.teams || []).slice();
  const seen = new Set(real.map(sameTeam));
  // A provisional row stands only for a team the real reading has not found yet:
  // once it is found, the provisional one goes at once.
  const provisional = (st.running.teams || []).filter((t) => t.provisional && !seen.has(sameTeam(t)));
  return Object.assign({}, running, { teams: real.concat(provisional) });
}

// noteStarting tells the list a team is starting now, before herdr has a tab for
// it: the row says 起動の途中 at once, and the next reading replaces this with the
// team as herdr sees it (r-5b96 audit R5).
export function noteStarting(launch) {
  const teams = (st.running.teams || []).filter((t) => sameTeam(t) !== sameTeam(launch));
  teams.push({
    team: launch.team, definition: launch.definition, place: launch.place,
    project: launch.project, project_name: launch.project_name,
    workspace: launch.workspace, tab: launch.tab,
    starting: true, launch: launch.id, members: [], workers: [], waiting: [],
    provisional: true,
  });
  st.running.teams = teams;
  bandActivity();
  say();
  // The tab is made within the first second; a few readings bring the team in
  // without waiting for the ten-second one, whether or not the sheet is open.
  [600, 1500, 3000, 6000].forEach((ms) => setTimeout(readRunning, ms));
}

// ---- 書き（並べる・外す） ---------------------------------------------
// postProject lists one project. The sheet shows the server's own sentence when
// it says no; the answer is the refusal, or null.
export async function postProject(path) {
  try { await call('POST', '/api/hq/projects', { path }); return null; } catch (e) { return problemText(e, '並べられませんでした'); }
}
export async function unlistProject(path) {
  try { await call('DELETE', '/api/hq/projects?path=' + encodeURIComponent(String(path))); return null; } catch (e) { return problemText(e, '外せませんでした'); }
}
// listProject lists one project. Without projects:, the project serve was started
// in is listed first, so listing another one does not drop the teams a person is
// looking at (r-5b96 audit 7).
export async function listProject(path) {
  const p = st.projects;
  if (p && p.mode !== 'listed' && p.current && p.current.path && p.current.path !== path) {
    const first = await postProject(p.current.path);
    if (first) return first;
  }
  return postProject(path);
}

// ---- the ten-second reading ------------------------------------------
let timer = 0;
export function startReads() {
  if (!timer) timer = setInterval(() => { if (document.visibilityState === 'visible') readRunning(); }, POLL);
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') readRunning(); });
}
