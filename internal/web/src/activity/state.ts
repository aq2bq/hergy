// activity/state.ts — アクティビティ画面の state（webui-preact-design 4 章・5 章）。
//
// サーバーの答えはここだけに入る（5 章 2）。部品（board.tsx）は signal を読んで
// 描くだけで、捕まえておいた要素に書き込まない。読むのは /api/activity と
// /api/activity/{team}、それに本部の読み（/api/hq/running、チーム定義の turns）
// で、状態の判定はサーバの 1 つをそのまま使う。長さ・時刻の言い方もサーバが
// 作る（V1）。この画面は文を 1 つも作らない。
import { signal, computed } from '../lib/preact.js';
import { resource } from '../lib/resource';
import { read } from '../lib/api.js';
import * as A from '../lib/hq.js';
import { setActivity } from '../lib/nav.js';
import type { components } from '../../../apicontract/api-types';

type ActivityList = components['schemas']['ActivityList'];
type ActivityItem = components['schemas']['ActivityItem'];
type ActivityNote = components['schemas']['ActivityNote'];
type ActivityDocument = components['schemas']['ActivityDocument'];
type ActivityOpened = components['schemas']['ActivityOpened'];
type ActivityNode = components['schemas']['ActivityNode'];
type HQRunning = components['schemas']['HQRunning'];
type HQMember = components['schemas']['HQMember'];
type HQWorker = components['schemas']['HQWorker'];
type ActivityDefinition = components['schemas']['TeamDocument'];

export type CrewMember = {
  name: string;
  herdr: string;
  state: string;
  at: string;
  since: string;
  doing: string;
};
type Worker = { name: string; state: string; doing: string; from: string };
export type ActivityTeam = {
  name: string;
  project: string;
  projectName: string;
  workspace: string;
  definition: string;
  state: string;
  since: components['schemas']['ActivitySince'] | null;
  crew: CrewMember[];
  workers: Worker[];
  turns: [string, string][];
  starting: boolean;
  error: string;
  question: string;
};
type ActivityBoard = {
  items: ActivityItem[];
  note: ActivityNote;
  docs: Map<string, ActivityDocument>;
  teams: ActivityTeam[];
};
type ActivityWords = { none?: string };

// The headquarters' reading is a module: the screen imports it (5 章 1).
export const POLL = 10000;
// The screen's fixed words (2.2, 2.5). The server's answer carries the state's
// name (answer, idle, …) only; these are the letters a person reads.
const ST: Record<string, string> = { answer: '許可待ち', turn: '人間の返事待ち', stalled: '止まっている', working: '仕事中' };
const NS: Record<string, string> = { blocked: '許可待ち', handed: '人間の返事待ち', active: '仕事中', idle: '止まっている', absent: '動いていない' };
const RANK: Record<string, number> = { answer: 0, turn: 1, stalled: 2, working: 3 };

const words = JSON.parse((document.getElementById('activity-words') || {}).textContent || '{}') as ActivityWords; // Embedded JSON is typed here at its screen boundary.
const enc = (s: unknown): string => encodeURIComponent(String(s == null ? '' : s));

// selected is the team a person picked, by name. It survives the ten-second
// reading (spec 6 章): the reading replaces the answers, not the pick.
export const selected = signal<string | null>(null);

function openTeam(): { name: string; group: string } {
  if (!A) return { name: '', group: '' };
  const raw = (new URLSearchParams(location.search).get('open') || '');
  const one = A.parseOpen(raw)[0];
  return one || { name: '', group: '' };
}

function turnsOf(def: ActivityDefinition | null): [string, string][] {
  return ((def && def.turns) || []).map((t) => [t.from, t.to]);
}
function memberOrderOf(def: ActivityDefinition | null, nodes: ActivityNode[]): string[] {
  const named = ((def && def.members) || []).map((m) => m.name);
  const rest = (nodes || []).map((n) => n.name).filter((n) => named.indexOf(n) < 0);
  return named.concat(rest);
}
function lastMoment(t: Pick<ActivityTeam, 'crew'>): string {
  const at = (t.crew || []).map((m) => m.at).filter(Boolean).sort();
  return at.length ? at[at.length - 1] : '';
}

// sheetOf reads one team as the board draws it: the members in the definition's
// order with their live state, the workers under the member that added them, the
// turns of team.yaml, and the sentence of the waiting row (spec 6 章).
function sheetOf(item: ActivityItem, docs: Map<string, ActivityDocument>): ActivityTeam {
  const doc = docs.get(item.name);
  const run = A ? A.runningTeam(item.name) : null;
  const defName = item.summary.definition || item.name;
  const def = (A && A.definitionOf(item.project, defName)) || null;
  const nodes: ActivityNode[] = doc ? (doc.nodes || []) : ((run && run.members) || []).map((m: HQMember) => ({ name: m.name, herdr: m.herdr, state: m.state, at: '' }));
  const order = memberOrderOf(def, nodes);
  const byName = new Map(nodes.map((n): [string, ActivityNode] => [n.name, n]));
  const crew = order.filter((n) => byName.has(n)).map((name): CrewMember => {
    const n = byName.get(name)!;
    return { name, herdr: n.herdr || (item.name + '-' + name), state: n.state || 'idle', at: n.at || '', since: n.since_at || n.at || '', doing: n.doing || '' };
  });
  const hired: Pick<ActivityNode, 'name' | 'state'>[] = doc ? (doc.hired || []) : ((run && run.workers) || []).map((w: HQWorker) => ({ name: w.name, state: w.live ? 'active' : 'idle' }));
  const workers = hired.map((w): Worker => ({
    name: w.name,
    state: w.state || 'idle',
    doing: ('doing' in w && typeof w.doing === 'string' && w.doing) || '',
    from: ('from' in w && typeof w.from === 'string' && w.from) || '',
  }));
  const state = item.summary.state || 'idle';
  return {
    name: item.name, project: item.project, projectName: item.project_name || (A ? A.projectName(item.project) : ''),
    workspace: (item.workspace && item.workspace.label) || (run && run.workspace && run.workspace.label) || '',
    definition: defName, state, since: item.summary.since || null,
    crew, workers, turns: turnsOf(def),
    starting: !!(run && run.starting),
    error: item.error || '',
    question: (doc && doc.question) || '',
  };
}

function sortTeams(rows: ActivityTeam[]): ActivityTeam[] {
  return rows.slice().sort((a, b) => (RANK[a.state] - RANK[b.state]) || a.name.localeCompare(b.name));
}

// readBoard is the whole reading in one request of the resource's: the board,
// the headquarters reading and each team's sheet answer together or not at all,
// so the pick and the scroll are never drawn against answers of another moment.
async function readBoard(stop: AbortSignal): Promise<ActivityBoard> {
  // The figures' turns are the team definitions' own, so the groups are read
  // with the activity (the other screens read them when their search opens).
  if (A && A.readGroups) await A.readGroups();
  const answer = await read<ActivityList>('/api/activity', { signal: stop });
  const items = answer.items || [];
  const note = answer.herdr || { ok: true, text: '' };
  setActivity(answer.running || 0, answer.waiting || 0, !!(answer.herdr && answer.herdr.ok) && (answer.running || 0) > 0);

  // The headquarters reading is the same one the list's 動き column uses, so a
  // team's state cannot read two ways (api.md 3.5).
  const running = await read<HQRunning>('/api/hq/running', { signal: stop }).catch(() => null);
  if (running && A) A.setRunning(running);

  // A sheet that could not be read keeps the one the screen already had: the
  // figure of that team stays as it was instead of falling back to the run's
  // members (a screen that goes backwards would look like a change).
  const before = reading.value.value;
  const docs = new Map<string, ActivityDocument>(before ? before.docs : []);
  const wanted = items.map((t) => t.name);
  await Promise.all(items.map(async (t) => {
    const one = await read<ActivityOpened>('/api/activity/' + enc(t.name), { signal: stop }).catch(() => null);
    if (one && one.doc) docs.set(t.name, one.doc);
  }));
  for (const name of Array.from(docs.keys())) if (wanted.indexOf(name) < 0) docs.delete(name);

  const teams = sortTeams(items.map((t) => sheetOf(t, docs)));
  // The pick is the address's (an opened team) while the screen has none of it,
  // and the first team otherwise (spec 6 章).
  if (!selected.value || !teams.some((t) => t.name === selected.value)) {
    const pick = openTeam();
    const found = pick.name ? teams.find((t) => t.name === pick.name && (!pick.group || t.project === pick.group)) : null;
    selected.value = found ? found.name : (teams[0] ? teams[0].name : null);
  }
  return { items, note, docs, teams };
}

// reading is the screen's one reading. A ten-second tick that lands while one is
// out starts nothing (refresh), and every answer that arrives after a newer
// request is dropped by the resource itself.
export const reading = resource(readBoard);

export const teams = computed<ActivityTeam[]>(() => (reading.value.value ? reading.value.value.teams : []));
export const note = computed<ActivityNote>(() => (reading.value.value && reading.value.value.note) || { ok: true, text: '' });
export const noneText = words.none || '並べたプロジェクトに、動いているチームはありません。チーム定義の画面で起動できます。';
// current is the opened team: the pick, or the first team while the pick is not
// in the reading.
export const current = computed<ActivityTeam | null>(() => {
  const rows = teams.value;
  return rows.find((t) => t.name === selected.value) || rows[0] || null;
});

// refresh starts a reading unless one is out. The screen leaves a tick that
// lands mid-reading alone, as it always has: a reading of this size answers in
// a moment, and two at once would draw two moments over each other.
export function refresh(): Promise<ActivityBoard | undefined> | undefined {
  if (reading.loading.value) return undefined;
  return reading.run();
}

// pick is the press of a team: the address keeps the opened team, so a reload
// and a shared link stand where the person stands (spec 6 章).
export function pick(name: string): void {
  selected.value = name;
  const t = teams.value.find((x) => x.name === name);
  if (t && A) {
    const q = new URLSearchParams(location.search);
    q.set('open', A.token(name, t.project));
    history.replaceState(null, '', location.pathname + '?' + q.toString());
  }
}

export { ST, NS, lastMoment, words };
