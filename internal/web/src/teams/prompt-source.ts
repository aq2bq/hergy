// teams/prompt-source.ts — メンバーに渡る文を読む唯一のモジュール (⑤ r-ba9b,
// d-002b).
//
// 規則:
// - 画面はここを通してだけ渡る文を読む。比較 API の URL を直に呼ばない。
// - 読み直しは 1 つのキーで決める: dkey(r)・プロンプトの入力（place・name・hash・
//   ops・new/base/carry）・読む相手・選んだ実行中のチーム・runningRevision(r)。
//   runningRevision は st.running からこの定義のチームだけを取ったもの（チーム
//   名・プロジェクト・メンバーの名前と live）。だから起動・終了・別名の増減でも
//   キーが変わる（n-f6a0）。
// - 応答の順は lib/resource.ts が守る（新しい要求が前を取り消し、取り消し後の
//   答えは採らない）。このモジュールも、答えを入れる前に「頼んだキー = 今の
//   キー」を確かめ、違えば捨てる。読みが始まったら前の答えは捨てる（失敗しても
//   古い文を残さない）。
// - 打つたびに読まない。読みは検査と同じ 260 ミリ秒のまとめで 1 回にする。
// - 欄に出す「なぜこの名前か」の言葉は、このモジュールの noteOf だけが作る。
// - 選んだチームはこのモジュールが持つ（プロンプトの欄と全文と差分で 1 つ）。
import { signal, useEffect } from '../lib/preact.js';
import { call, problemText } from '../lib/api.js';
import type { Schemas } from '../lib/api.js';
import { resource } from '../lib/resource.js';
import { st } from '../lib/hq/state.js';
import { CURRENT, dkey, promptComparisonInput } from './state.js';
import type { PromptSegment, Rec } from './state.js';

type Answer = Schemas['PromptComparison'];
type Body = Schemas['PromptComparisonRequest'];

// PROMPT_READ.wait is the pause before a read: the check's own 260 ms. The unit
// tests set it to 0 (they drive every answer by hand and must not wait).
export const PROMPT_READ = { wait: 260 };

// PromptAsk is what a screen reads: a member's text. (The worker's preview uses
// the check until its own owner step.)
export type PromptAsk = { member: string };

// PromptState is what a screen draws: the server's answer (or the last one while
// a new read is out), the note's words, and how the read stands.
export type PromptState = {
	answer: Answer | null;
	segments: PromptSegment[];
	note: string;
	loading: boolean;
	error: string;
};

type Store = {
	runKey: string;
	key: string;
	payload: Body | null;
	shown: { value: Answer | null };
	loading: { value: boolean };
	error: { value: string };
	reading: { value: { value: Answer | null }; loading: { value: boolean }; error: { value: unknown }; run: (body: Body) => Promise<Answer | undefined>; abort: () => void };
	timer: ReturnType<typeof setTimeout> | null;
};

const stores = new Map<string, Store>();

// promptRun is the running team a person picked, per definition (r-0b69). Empty
// means nothing was picked; the server then takes the only running team.
const promptRun = signal<Record<string, string>>({});
export const runOf = (key: string): string => promptRun.value[key] || '';
export function setRun(key: string, team: string): void {
	if (runOf(key) === team) return;
	promptRun.value = { ...promptRun.value, [key]: team };
}
export const pickedRun = (r: Rec): string => runOf(dkey(r));
export const pickRun = (r: Rec, team: string): void => setRun(dkey(r), team);

// runningRevision is the live teams of this definition as one string: the part
// of the key that makes a start, a stop or an alias a reason to read again.
export function runningRevision(r: Rec): string {
	const project = r.project && r.project !== 'global' ? r.project : CURRENT;
	const teams = (st.running.value.teams || [])
		.filter((t) => t.definition === r.name.value
			&& (t.place || 'project') === r.place.value
			&& (r.place.value === 'global' || !project || t.project === project))
		.map((t) => [t.team, t.project || '', ...(t.members || []).map((m) => `${m.name}:${m.live ? '1' : '0'}`)].join('\u0000'))
		.sort();
	return teams.join('\u0001');
}

// promptKey is everything a change of which makes the text another text.
export function promptKey(r: Rec, ask: PromptAsk): string {
	return [dkey(r), JSON.stringify(promptComparisonInput(r)), ask.member, runOf(dkey(r)), runningRevision(r)].join('\u0002');
}

function storeFor(r: Rec, ask: PromptAsk): Store {
	const k = dkey(r) + '|' + ask.member;
	const had = stores.get(k);
	if (had) return had;
	const reading = resource<Answer, [Body]>(async (stop, body) => {
		const answer = await call<Answer>('POST', '/api/team/prompt-comparison', body, { signal: stop });
		if (!answer.json) throw new Error('本文の応答がありません');
		return answer.json;
	});
	const made: Store = {
		runKey: dkey(r), key: '', payload: null,
		shown: signal<Answer | null>(null), loading: signal(false), error: signal(''),
		reading, timer: null,
	};
	stores.set(k, made);
	return made;
}

function schedule(store: Store, key: string, body: Body): void {
	store.payload = body;
	if (store.key === key) return;
	store.key = key;
	if (store.timer) clearTimeout(store.timer);
	store.timer = setTimeout(() => {
		store.timer = null;
		if (store.key === key) void load(store, key);
	}, PROMPT_READ.wait);
}

async function load(store: Store, key: string): Promise<void> {
	const body = store.payload;
	if (!body) return;
	// The read starts: the text it replaces is not this key's any more.
	store.shown.value = null;
	store.error.value = '';
	store.loading.value = true;
	const answer = await store.reading.run(body);
	if (store.key !== key) return; // the key moved again: this answer is not shown
	store.loading.value = false;
	if (answer) {
		store.shown.value = answer;
		// A pick that no longer runs is let go, so the next read takes the only
		// team the server named (r-0b69, ⑤ r-ba9b). A reading that failed says
		// nothing about which teams run: the pick stands until one that knows.
		const picked = runOf(store.runKey);
		if (answer.status !== 'error' && picked && !(answer.runs || []).some((run) => run.team === picked)) setRun(store.runKey, '');
		return;
	}
	if (store.reading.error.value) store.error.value = problemText(store.reading.error.value, '本文を読めませんでした');
}

// reloadPrompt reads again now (the full panel's 再取得). It is a convenience:
// the key's own changes read without it.
export function reloadPrompt(r: Rec, ask: PromptAsk): void {
	const store = storeFor(r, ask);
	store.key = '';
	const body = { ...promptComparisonInput(r), member: ask.member, running_team: pickedRun(r) };
	schedule(store, promptKey(r, ask), body as Body);
}

// usePrompt reads one member's text and the note about its name.
export function usePrompt(r: Rec, ask: PromptAsk): PromptState {
	const store = storeFor(r, ask);
	const key = promptKey(r, ask);
	const body = { ...promptComparisonInput(r), member: ask.member, running_team: pickedRun(r) } as Body;
	useEffect(() => { schedule(store, key, body); }, [store, key]);
	const answer = store.shown.value;
	const error = store.error.value;
	return {
		answer,
		segments: answer ? answer.segments : [],
		note: noteOf(answer, r.name.value, error),
		loading: store.loading.value,
		error,
	};
}

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
