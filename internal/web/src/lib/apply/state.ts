// lib/apply/state.ts — 反映の数と状態（n-4b22、設計は apply-design.md）。
//
// 帯の数は GET /api/apply/count、欄の中身は GET /api/apply、送るのは
// POST /api/apply（202 で起動を返す）。どれも lib/api.js を通す。サーバーの答えは
// signal に置き、部品（lib/apply/view.js）がそれを描く（webui-preact-design 5 章 1・2）。
//
// ずれと反映の仕方はサーバーが決める（送り直す・起動する・起動し直す・反映できない）。
// ここはそれを並べ、人が選んだものを送るだけ。送ったあとの進み具合と、起動の途中の
// 問いは、起動の欄（lib/hq/sheets.js の runStart）が同じ形で出す。
import { signal, computed } from '../preact.js';
import { applyList, applyCount, applyMembers, problemText, Refusal } from '../api.js';
import { runStart } from '../hq/sheets.js';
import type { ApplyList, ApplyTeam } from './types';

// 帯の数の読み直し（画面が見えている間だけ）。
export const POLL = 15000;

// count is the band's number: every member whose definition change is not in
// the running team yet, whether or not it can be applied now.
export const count = signal<number>(0);

// all is the last reading of the whole answer. The screens that show a team's
// own number (the 動き column, the activity screen's team) read it here; a
// reading that fails leaves the last one.
export const all = signal<ApplyList | null>(null);

// opened is whether the sheet is on show; answer is what it shows, and note the
// one sentence the sheet has to say beside the acts (a refusal, or herdr not
// being readable).
export const opened = signal<boolean>(false);
export const answer = signal<ApplyList | null>(null);
export const note = signal<string>('');
export const sending = signal<boolean>(false);

// scope is the team a screen narrowed the sheet to, or '' for every team.
export const scope = signal<string>('');

// chosen is the members a person picked, by herdr name. A pick goes out in the
// order the answer lists the members, not in the order they were clicked.
export const chosen = signal<Set<string>>(new Set());

// toast is the words of the one notice at the bottom, and toastOn whether they
// are on show (they stay in the tree after they hide, as they always did).
export const toast = signal<string>('');
export const toastOn = signal<boolean>(false);

// teams is the answer's teams, narrowed to the sheet's scope.
export const teams = computed((): ApplyTeam[] => (answer.value && answer.value.teams ? answer.value.teams : [])
  .filter((g) => !scope.value || g.team === scope.value));

// pickable is the members a send may reach, in the answer's own order.
export const pickable = computed((): string[] => {
  const out: string[] = [];
  teams.value.forEach((g) => (g.members || []).forEach((m) => {
    if (m.can_apply) out.push(m.name);
  }));
  return out;
});

// blocked is herdr saying it cannot read the running teams: the sheet says so
// and nothing can be sent.
export const blocked = computed((): boolean => !!(answer.value && answer.value.herdr && answer.value.herdr.ok === false));

// picks is what the send would reach now, and applyLabel the button's words.
export const picks = computed((): string[] => pickable.value.filter((name: string) => chosen.value.has(name)));
export const canApply = computed((): boolean => !sending.value && !blocked.value && picks.value.length > 0);
export const applyLabel = computed((): string => (picks.value.length
  ? `選んだ ${picks.value.length} 人に反映する`
  : '反映する人を選んでください'));

// teamCount is how many members of one team are not applied yet: the number a
// screen puts beside that team (the 動き column, the activity screen).
export function teamCount(team: string): number {
  const a = all.value;
  let n = 0;
  ((a && a.teams) || []).forEach((g) => { if (g.team === team) n += (g.members || []).length; });
  return n;
}

export function toggle(name: string): void {
  const next = new Set(chosen.value);
  if (next.has(name)) next.delete(name); else next.add(name);
  chosen.value = next;
}

// ---- 読む ---------------------------------------------------------------
export async function readCount() {
  try {
    const a = await applyCount();
    count.value = Number(a && a.count) || 0;
  } catch (e) {
    // 読めないときは、前の数をそのまま置く
  }
}

// readTeams is the whole answer, for the screens that show a team's own number.
export async function readTeams() {
  try {
    all.value = await applyList();
  } catch (e) {
    // 読めないときは、前の答えをそのまま置く
  }
}

// ---- 欄 ----------------------------------------------------------------
let opening = false;
let opener: Focusable | null = null;
type Focusable = Element & { focus: (options?: FocusOptions) => void };

// openSheet opens the sheet, narrowed to one team when a screen named it.
export async function openSheet(team?: string) {
  if (opening) return;
  opening = true;
  try {
    const a = await applyList();
    // The person pressed something; that is the element to come back to.
    const active = document.activeElement;
    opener = active && typeof (active as Focusable).focus === 'function' ? active as Focusable : null;
    scope.value = team || '';
    answer.value = a;
    all.value = a;
    note.value = a && a.herdr && a.herdr.ok === false ? a.herdr.text || '' : '';
    chosen.value = new Set(pickable.value);
    opened.value = true;
  } catch (e) {
    // 読めないときは、欄を開かない
  } finally {
    opening = false;
  }
}

export function close() {
  if (!opened.value) return;
  opened.value = false;
  answer.value = null;
  note.value = '';
  scope.value = '';
  chosen.value = new Set();
  const back = opener;
  opener = null;
  if (back && back.isConnected && typeof back.focus === 'function') back.focus({ preventScroll: true });
}

export async function send(): Promise<void> {
  if (!opened.value || sending.value) return;
  const names = picks.value;
  if (!names.length) return;
  sending.value = true;
  try {
    const env = await applyMembers(names);
    close();
    showToast(`反映しました: ${names.join('・')}`);
    if (env.launch) runStart(env.launch);
    readCount();
    readTeams();
  } catch (e) {
    if (e instanceof Refusal && e.status) {
      // 1 人でも拒めば、誰にも反映していない。欄は閉じない。
      note.value = `反映していません。${problemText(e, e.text || '')}`;
    } else {
      note.value = '反映できませんでした。';
    }
  } finally {
    sending.value = false;
  }
}

let toastTimer = 0;
export function showToast(text: string): void {
  toast.value = text;
  toastOn.value = true;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { toastOn.value = false; }, 4000);
}
