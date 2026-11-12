// teams/prompt-source.ts — 渡る文を読む唯一のモジュール (⑤ r-ba9b, d-002b).
//
// 規則:
// - 画面はここを通してだけ渡る文を読む。比較 API とワーカーの検査の URL を直に
//   呼ばない。
// - 読み直しは 1 つのキーで決める: dkey(r)・プロンプトの入力（place・name・hash・
//   ops・new/base/carry）・読む相手（メンバー名 / ワーカーの from と purpose）・
//   選んだ実行中のチーム・runningRevision(r)。runningRevision は st.running から
//   この定義のチームだけを取ったもの（チーム名・プロジェクト・メンバーの名前と
//   live）。だから起動・終了・別名の増減でもキーが変わる（n-f6a0）。
// - 応答の順は lib/resource.ts が守る（新しい要求が前を取り消し、取り消し後の
//   答えは採らない）。このモジュールも、答えを入れる前に「頼んだキー = 今の
//   キー」を確かめ、違えば捨てる。読みが始まったら前の答えは捨てる（失敗しても
//   古い文を残さない）。
// - 打つたびに読まない。読みは検査と同じ 260 ミリ秒のまとめで 1 回にする。
// - 欄に出す「なぜこの名前か」の言葉は、このモジュールの noteOf だけが作る。
// - 選んだチームはこのモジュールが持つ（プロンプトの欄と全文と差分で 1 つ）。
// - ワーカーのプレビューもここが読む。名前と依頼は hire のときに決まるので、
//   画面の仮の値（WORKER_NAME・WORKER_TASK）で組み、画面が <> で描く。
import { signal, useEffect } from '../lib/preact.js';
import { readPromptComparison, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { resource } from '../lib/resource.js';
import type { Reading } from '../lib/resource.js';
import { st } from '../lib/hq/state.js';
import { CURRENT, dkey, promptComparisonInput } from './state.js';
import type { PromptSegment, Rec } from './state.js';

type Answer = Schemas['PromptComparison'];
type Body = Schemas['PromptComparisonRequest'];
type CheckAnswer = Schemas['TeamCheckAnswer'];

const promptRequestBrand: unique symbol = Symbol('PromptRequest');
export type PromptRequest = Readonly<{ [promptRequestBrand]: true; body: Body }>;

// The worker's name and task are chosen at hire; the preview uses values that
// cannot be mistaken for the purpose or a line, and draws <> in their place.
export const WORKER_NAME = 'zz-worker-name-0';
export const WORKER_TASK = 'zzTaskPlaceholder0';

// PromptAsk is what a screen reads: a member's text. (The worker's preview is
// its own hook.)
export type PromptAsk = { member: string };

// PromptState is what a screen draws: the server's answer (or the last one while
// a new read is out), the note's words, and how the read stands.
export type PromptState = Readonly<{
	answer: Answer | null;
	segments: PromptSegment[];
	note: string;
	loading: boolean;
	error: string;
}>;

// WorkerPromptState is the worker preview: the segments the hire's text is made
// of, and the two placeholders the screen draws as slots.
export type WorkerPromptState = Readonly<{
	segments: PromptSegment[];
	team: string;
	name: string;
	task: string;
	loading: boolean;
	err: string;
}>;

// Slot is one read's state: the key it stands for, what the read was asked
// with, and the answer it last showed. kick reads once for a key.
type Slot<T> = {
	runKey: string;
	key: string;
	payload: unknown;
	timer: ReturnType<typeof setTimeout> | null;
	shown: { value: T | null };
	loading: { value: boolean };
	error: { value: string };
	kick: (key: string) => Promise<void>;
};

// Each page uses one component. Tests inject the debounce duration at creation,
// rather than writing the live component's state or timing from outside.
export function createPromptSource({ wait = 260 }: { wait?: number } = {}) {
  const members = new Map<string, Slot<Answer>>();
  const workers = new Map<string, Slot<CheckAnswer>>();

  // promptRun is the running team a person picked, per definition (r-0b69). Empty
  // means nothing was picked; the server then takes the only running team.
  const promptRun = signal<Record<string, string>>({});
  const runOf = (key: string): string => promptRun.value[key] || '';
  function setRun(key: string, team: string): void {
  	if (runOf(key) === team) return;
  	promptRun.value = { ...promptRun.value, [key]: team };
  }
  const pickedRun = (r: Rec): string => runOf(dkey(r));
  const pickRun = (r: Rec, team: string): void => setRun(dkey(r), team);

  // liveTeams is the running teams of this definition, as herdr's last reading
  // named them. A reading that failed says nothing about the teams: it answers
  // none, and the pick is not cleared by it.
  function liveTeams(r: Rec): { team: string; project?: string; members?: { name: string; live?: boolean }[] }[] {
  	if (!st.running.value.herdr || !st.running.value.herdr.ok) return [];
  	const project = r.project && r.project !== 'global' ? r.project : CURRENT;
  	return structuredClone(st.running.value.teams || []).filter((t) => t.definition === r.name.value
  		&& (t.place || 'project') === r.place.value
  		&& (r.place.value === 'global' || !project || t.project === project));
  }

  // runningRevision is the live teams of this definition as one string: the part
  // of the key that makes a start, a stop or an alias a reason to read again.
  function runningRevision(r: Rec): string {
  	return liveTeams(r)
  		.map((t) => [t.team, t.project || '', ...(t.members || []).map((m) => `${m.name}:${m.live ? '1' : '0'}`)].join('\u0000'))
  		.sort()
  		.join('\u0001');
  }

  // useKeepPick lets a pick go when its team stops, whichever prompt is open: the
  // worker's preview follows the same pick, so it cannot wait for the member's
  // answer to clear it. A reading that failed clears nothing.
  function useKeepPick(r: Rec): void {
  	const picked = pickedRun(r);
  	const revision = runningRevision(r);
  	useEffect(() => {
  		if (!picked || !st.running.value.herdr || !st.running.value.herdr.ok) return;
  		if (!liveTeams(r).some((one) => one.team === picked)) setRun(dkey(r), '');
  	}, [picked, revision]);
  }

  // promptKey is everything a change of which makes a member's text another text.
  function promptKey(r: Rec, ask: PromptAsk): string {
  	return [dkey(r), JSON.stringify(promptComparisonInput(r)), ask.member, pickedRun(r), runningRevision(r)].join('\u0002');
  }

  // workerKey is the same for one worker: its from and purpose are the reader,
  // and a change of the definition (ops, hash, runs) or the picked team reads
  // again too.
  function workerKey(r: Rec, from: string, purpose: string): string {
  	return [dkey(r), JSON.stringify(promptComparisonInput(r)), 'worker', from, purpose, pickedRun(r), runningRevision(r)].join('\u0002');
  }

  function schedule(slot: Slot<unknown>, key: string, body: unknown): void {
  	slot.payload = body;
  	if (slot.key === key) return;
  	slot.key = key;
  	if (slot.timer) clearTimeout(slot.timer);
  	slot.timer = setTimeout(() => {
  		slot.timer = null;
  		if (slot.key === key) void slot.kick(key);
  	}, wait);
  }

  // makeSlot holds one reading through lib/resource.ts. read answers undefined on
  // a refusal; the error signal says why, and the slot says it in words.
  function makeSlot<T>(runKey: string, read: Reading<T, [unknown]>, after?: (answer: T) => void): Slot<T> {
  	const slot: Slot<T> = {
  		runKey, key: '', payload: null, timer: null,
  		shown: signal<T | null>(null), loading: signal(false), error: signal(''),
  		kick: async () => undefined,
  	};
  	slot.kick = async (key: string) => {
  		const body = slot.payload;
  		if (!body) return;
  		// The read starts: the text it replaces is not this key's any more.
  		slot.shown.value = null;
  		slot.error.value = '';
  		slot.loading.value = true;
  		const answer = await read.run(body);
  		if (slot.key !== key) return; // the key moved again: this answer is not shown
  		slot.loading.value = false;
  		if (answer !== undefined && answer !== null) {
  			slot.shown.value = answer;
  			if (after) after(answer);
  		} else if (read.error.value) slot.error.value = problemText(read.error.value, '本文を読めませんでした');
  	};
  	return slot;
  }

  function memberSlotFor(r: Rec, ask: PromptAsk): Slot<Answer> {
  	const k = dkey(r) + '|' + ask.member;
  	const had = members.get(k);
  	if (had) return had;
  	const reading = resource<Answer, [unknown]>(async (stop, body) => {
  		const answer = await readPromptComparison({ [promptRequestBrand]: true, body: body as Body }, { signal: stop });
  		if (!answer.json) throw new Error('本文の応答がありません');
  		return answer.json;
  	});
  	const made = makeSlot<Answer>(dkey(r), reading, (answer) => {
  		// A pick that no longer runs is let go, so the next read takes the only
  		// team the server named (r-0b69). A failed reading says nothing about
  		// the runs: the pick stands until one that knows.
  		if (answer.status === 'error') return;
  		const picked = pickedRun(r);
  		if (picked && !(answer.runs || []).some((run) => run.team === picked)) setRun(dkey(r), '');
  	});
  	members.set(k, made);
  	return made;
  }

  function workerSlotFor(r: Rec, from: string, purpose: string): Slot<CheckAnswer> {
  	const k = dkey(r) + '|worker|' + from + '|' + purpose;
  	const had = workers.get(k);
  	if (had) return had;
  	const reading = resource<CheckAnswer, [unknown]>(async (stop, body) => {
  		const answer = await r.editing.workerPreview(body as WorkerAsk, stop);
  		if (!answer) throw new Error('本文の応答がありません');
  		return answer;
  	});
  	const made = makeSlot<CheckAnswer>(dkey(r), reading);
  	workers.set(k, made);
  	return made;
  }

  // workerBody is the check the worker's preview sends: the draft, the picked
  // running team, and the placeholder name and task.
  type WorkerAsk = { from: string; purpose: string; name: string; task: string; runningTeam: string };
  function workerBody(r: Rec, from: string, purpose: string): WorkerAsk {
    return { from, purpose, name: WORKER_NAME, task: WORKER_TASK, runningTeam: pickedRun(r) };
  }

  // reloadPrompt reads again now (the full panel's 再取得). It is a convenience:
  // the key's own changes read without it.
  function reloadPrompt(r: Rec, ask: PromptAsk): void {
  	const slot = memberSlotFor(r, ask);
  	const body = { ...promptComparisonInput(r), member: ask.member, running_team: pickedRun(r) };
  	slot.key = '';
  	schedule(slot, promptKey(r, ask), body);
  }

  // usePrompt reads one member's text and the note about its name.
  function usePrompt(r: Rec, ask: PromptAsk): PromptState {
  	useKeepPick(r);
  	const slot = memberSlotFor(r, ask);
  	const key = promptKey(r, ask);
  	const body = { ...promptComparisonInput(r), member: ask.member, running_team: pickedRun(r) };
  	useEffect(() => { schedule(slot, key, body); }, [slot, key]);
  	const answer = slot.shown.value ? structuredClone(slot.shown.value) : null;
  	const error = slot.error.value;
  	return {
  		answer,
  		segments: answer ? answer.segments : [],
  		note: noteOf(answer, r.name.value, error),
  		loading: slot.loading.value,
  		error,
  	};
  }

  // useWorkerPrompt reads one worker's preview. Null when the row is not a worker
  // with a purpose and a member that hires it.
  function useWorkerPrompt(r: Rec, i: number): WorkerPromptState | null {
  	useKeepPick(r);
  	const w = r.draft.value.workers[i];
  	const ok = !!(w && w.purpose && w.from);
  	const slot = ok ? workerSlotFor(r, w.from, w.purpose) : null;
  	const key = ok ? workerKey(r, w.from, w.purpose) : '';
  	const body = ok ? workerBody(r, w.from, w.purpose) : null;
  	useEffect(() => { if (slot) schedule(slot, key, body); }, [slot, key]);
  	if (!slot) return null;
  	const answer = slot.shown.value ? structuredClone(slot.shown.value) : null;
  	return {
  		segments: answer ? answer.prompt || [] : [],
  		team: (answer && answer.prompt_team) || r.name.value,
  		name: WORKER_NAME,
  		task: WORKER_TASK,
  		loading: slot.loading.value,
  		err: slot.error.value,
  	};
  }

  return Object.freeze({ pickedRun, pickRun, liveTeams, runningRevision, promptKey, workerKey, reloadPrompt, usePrompt, useWorkerPrompt });
}
export const { pickedRun, pickRun, liveTeams, runningRevision, promptKey, workerKey, reloadPrompt, usePrompt, useWorkerPrompt } = createPromptSource();

// noteOf is the one place the column's note words are made. The server says
// which name and why (status, kind, runs, selected); the words are here.
export function noteOf(answer: Answer | null, name: string, error: string): string {
	if (error) return `${error} 文を表示できません。`;
	if (!answer) return '';
	if (answer.status === 'error') return `${answer.reason || 'チームの状態を読めませんでした'} そのため、定義名 ${name} で作っています。`;
	if (answer.selected) return `実行中のチーム ${answer.team} の名前で作っています。`;
	const runs = answer.runs || [];
	if (runs.length > 1) return `動いているチームが ${runs.length} つあるため、定義名 ${name} で作っています。全文と差分のパネルでチームを選ぶと、その名前で作ります。`;
	if (runs.length === 0) return `この定義から動いているチームが無いため、定義名 ${name} で作っています。`;
	return `定義名 ${name} で作っています。`;
}
