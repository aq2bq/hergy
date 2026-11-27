// activity/state.ts — アクティビティ画面の state（webui-preact-design 4 章・5 章）。
//
// サーバーの答えはここだけに入る（5 章 2）。部品（board.tsx）は signal を読んで
// 描くだけで、捕まえておいた要素に書き込まない。 /api/activity と
// /api/activity/{team}、それに本部の読み（/api/hq/running、チーム定義の turns）
// を読む。サーバーが判定した状態と時刻を、画面のラベルに変換する。
import { term } from '../lib/terms.generated.js';
import { memberStateWord, teamStateWord } from '../lib/state-words.js';
import { signal, computed } from '../lib/preact.js';
import { resource } from '../lib/resource';
import { read, problemText } from '../lib/api.js';
import { teamStatusReadFailure } from '../lib/screen-words.js';
import * as A from '../lib/hq.js';
import { placeGroup, rowAddress } from '../lib/address.js';
import type { components } from '../../../apicontract/api-types';

type ActivityList = components['schemas']['ActivityList'];
type ActivityItem = components['schemas']['ActivityItem'];
type ActivityNote = components['schemas']['ActivityNote'];
type ActivityDocument = components['schemas']['ActivityDocument'];
type ActivityOpened = components['schemas']['ActivityOpened'];
type ActivityNode = components['schemas']['ActivityNode'];
type ActivitySummary = components['schemas']['ActivitySummary'];
type HQRunning = components['schemas']['HQRunning'];
type HQMember = components['schemas']['HQMember'];
type HQWorker = components['schemas']['HQWorker'];
type ActivityDefinition = components['schemas']['TeamDocument'];

export type CrewMember = {
  name: string;
  herdr: string;
  state: string;
  at: number | null;
  doing: string;
  question?: string;
};
type Worker = { name: string; state: string; doing: string; from: string };
export type ActivityTeam = {
  name: string;
  project: string;
  projectName: string;
  workspace: string;
  definition: string;
  place: ActivityItem['place'];
  state: string;
  since: components['schemas']['ActivitySince'] | null;
  turn: components['schemas']['ActivityTurn'] | null;
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
const ST = teamStateWord;
const NS = memberStateWord;
const teamStates: Record<ActivitySummary['state'], string> = {
  'waiting-for-permission': 'answer', handed: 'turn', stalled: 'stalled', working: 'working',
};
const nodeStates: Record<ActivityNode['state'], string> = {
  'waiting-for-permission': 'blocked', handed: 'handed', working: 'active', idle: 'idle', 'not-running': 'absent',
};
const RANK: Record<string, number> = { answer: 0, turn: 1, stalled: 2, working: 3 };

const words = JSON.parse((document.getElementById('activity-words') || {}).textContent || '{}') as ActivityWords; // Embedded JSON is typed here at its screen boundary.
const enc = (s: unknown): string => encodeURIComponent(String(s == null ? '' : s));

// selected is the team a person picked, by name. It survives the ten-second
// reading (spec 6 章): the reading replaces the answers, not the pick.
export const selected = signal<string | null>(null);

export function openedTeam(): { name: string; group: string } {
  if (!A) return { name: '', group: '' };
  const one = A.readOpen(new URLSearchParams(location.search))[0];
  return one || { name: '', group: '' };
}

function turnsOf(def: ActivityDefinition | null): [string, string][] {
  return ((def && def.turns) || []).map((t) => [t.from, t.to]);
}
function memberOrderOf(def: ActivityDefinition | null, nodes: CrewMember[]): string[] {
  const named = ((def && def.members) || []).map((m) => m.name);
  const rest = (nodes || []).map((n) => n.name).filter((n) => named.indexOf(n) < 0);
  return named.concat(rest);
}
function lastMoment(t: Pick<ActivityTeam, 'crew'>): number | null {
  const times = t.crew.flatMap((member) => member.at === null ? [] : [member.at]);
  return times.length ? Math.max(...times) : null;
}

// sheetOf reads one team as the board draws it: the members in the definition's
// order with their live state, the workers under the member that added them, the
// turns of team.yaml, and the sentence of the waiting row (spec 6 章).
function sheetOf(item: ActivityItem, docs: Map<string, ActivityDocument>): ActivityTeam {
  const doc = docs.get(item.name);
  const run = A ? A.runningTeam(item.name) : null;
  const defName = item.summary.definition || item.name;
  const def = (A && A.definitionOf(item.project, defName, item.place)) || null;
  const nodes: CrewMember[] = doc ? (doc.nodes || []).map((n) => ({
    name: n.name, herdr: n.herdr || (item.name + '-' + n.name), state: nodeStates[n.state],
    at: n.at, doing: n.doing || '', question: n.question,
  })) : ((run && run.members) || []).map((m: HQMember) => ({
    name: m.name, herdr: m.herdr || (item.name + '-' + m.name), state: m.state, at: null, doing: '',
  }));
  const order = memberOrderOf(def, nodes);
  const byName = new Map(nodes.map((n) => [n.name, n] as const));
  const crew = order.flatMap((name) => {
    const member = byName.get(name);
    return member ? [member] : [];
  });
  const workers: Worker[] = doc ? (doc.hired || []).map((worker) => ({
    name: worker.name, state: nodeStates[worker.state], doing: worker.doing || '', from: worker.from || '',
  })) : ((run && run.workers) || []).map((w: HQWorker) => ({
    name: w.name, state: w.live ? 'active' : 'idle', doing: '', from: '',
  }));
  const state = teamStates[item.summary.state];
  return {
    name: item.name, project: item.project, projectName: item.project_name || (A ? A.projectName(item.project) : ''),
    workspace: (item.workspace && item.workspace.label) || (run && run.workspace && run.workspace.label) || '',
    definition: defName, place: item.place, state, since: item.summary.since || null, turn: item.summary.turn || null,
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
  // The band's activity numbers come from the headquarters reading alone (r-ea17,
  // d-4d88): the same reading the list's 動き column uses, so the band and the
  // cards cannot count two ways (api.md 3.5).
  const running = await read<HQRunning>('/api/hq/running', { signal: stop }).catch(() => null);
  if (running && A) { A.setRunning(running); A.bandActivity(); }

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
    const pick = openedTeam();
    const found = pick.name ? teams.find((t) => t.name === pick.name && (!pick.group || t.project === pick.group)) : null;
    selected.value = found ? found.name : (teams[0] ? teams[0].name : null);
  }
  return { items, note, docs, teams };
}

// reading is the screen's one reading. A ten-second tick that lands while one is
// out starts nothing (refresh), and every answer that arrives after a newer
// request is dropped by the resource itself.
export const reading = resource(readBoard);

export const fetchError = computed(() => reading.error.value ? problemText(reading.error.value, teamStatusReadFailure()) : '');

export const teams = computed<ActivityTeam[]>(() => (reading.value.value ? reading.value.value.teams : []));
export const empty = computed(() => reading.value.value !== null && teams.value.length === 0);
export const note = computed<ActivityNote>(() => (reading.value.value && reading.value.value.note) || { ok: true, text: '' });
export const noneText = words.none || `並べた${term('project')}に、動いている${term('team')}はありません。${term('team-definition')}の画面で${term('start')}できます。`;
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
    // The team's own project is the place it stands in; the activity screen
    // reads it back as one row (⑥-2).
    A.writeOpen(q, [rowAddress(name, t.project || placeGroup('project'))]);
    history.replaceState(null, '', location.pathname + '?' + q.toString());
  }
}

export { ST, NS, lastMoment, words };
