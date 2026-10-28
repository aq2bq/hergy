// lib/deliver/state.js — 変更を伝える の数と状態（r-a8fa）。
//
// 帯の数は GET /api/delivery/count、シートの中身は GET /api/delivery、送るのは
// POST /api/delivery。どれも lib/api.js を通す。サーバーの答えは signal に置き、
// 部品（lib/deliver/view.js）がそれを描く（webui-preact-design 5 章 1・2）。
//
// 文はサーバーのもの（deliver.Compare）で、ここはそれを並べるだけ。保存は
// シートを開かず、送りもしない: hergy:saved を聞いて、帯の数を読み直す。
import { signal, computed } from '../preact.js';
import { delivery, deliveryCount, deliver as postDelivery, problemText, Refusal } from '../api.js';
import type { ApiAnswer } from '../api.js';

type DeliveryAnswer = ApiAnswer<'listDelivery'>;
type Focusable = Element & { focus: (options?: FocusOptions) => void };

// 帯の数の読み直し（画面が見えている間だけ）。
export const POLL = 15000;

// count is the band's number: the running members whose prompt is not the one
// hergy last handed them. A reading that fails leaves the last number.
export const count = signal<number>(0);

// opened is whether the sheet is on show; answer is what it shows, and note the
// one sentence the sheet has to say beside the acts (a refusal, or herdr not
// being readable).
export const opened = signal<boolean>(false);
export const answer = signal<DeliveryAnswer | null>(null);
export const note = signal<string>('');
export const sending = signal<boolean>(false);

// chosen is the members a person picked, by name. A pick goes out in the order
// the answer lists the members, not in the order they were clicked.
export const chosen = signal<Set<string>>(new Set());

// toast is the words of the one notice at the bottom, and toastOn whether they
// are on show (they stay in the tree after they hide, as they always did).
export const toast = signal<string>('');
export const toastOn = signal<boolean>(false);

// pickable is the members a send may reach, in the answer's own order.
export const pickable = computed((): string[] => {
  const out: string[] = [];
  const a = answer.value;
  ((a && a.teams) || []).forEach((g) => (g.members || []).forEach((m) => {
    if (m.can_send) out.push(m.name);
  }));
  return out;
});

// blocked is herdr saying it cannot hand a prompt to anyone: the sheet says so
// and nothing can be sent (brief 2).
export const blocked = computed((): boolean => !!(answer.value && answer.value.herdr && answer.value.herdr.ok === false));

// picks is what the send would reach now, and sendLabel the button's words.
export const picks = computed((): string[] => pickable.value.filter((name: string) => chosen.value.has(name)));
export const canSend = computed((): boolean => !sending.value && !blocked.value && picks.value.length > 0);
export const sendLabel = computed((): string => (picks.value.length
  ? `選んだ ${picks.value.length} 人に伝える`
  : '伝える人を選んでください'));

export function toggle(name: string): void {
  const next = new Set(chosen.value);
  if (next.has(name)) next.delete(name); else next.add(name);
  chosen.value = next;
}

// ---- 読む ---------------------------------------------------------------
export async function readCount() {
  try {
    const a = await deliveryCount();
    count.value = Number(a && a.count) || 0;
  } catch (e) {
    // 読めないときは、前の数をそのまま置く
  }
}

// ---- シート -------------------------------------------------------------
let opening = false;
let opener: Focusable | null = null;

export async function openSheet() {
  if (opening) return;
  opening = true;
  try {
    const a = await delivery();
    // The person pressed something; that is the element to come back to.
    const active = document.activeElement;
    opener = active && typeof (active as Focusable).focus === 'function' ? active as Focusable : null;
    answer.value = a;
    note.value = a && a.herdr && a.herdr.ok === false ? a.herdr.text || '' : '';
    chosen.value = new Set(pickable.value);
    opened.value = true;
  } catch (e) {
    // 読めないときは、シートを開かない
  } finally {
    opening = false;
  }
}

export function close() {
  if (!opened.value) return;
  opened.value = false;
  answer.value = null;
  note.value = '';
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
    await postDelivery(names);
    close();
    showToast(`${names.join('・')} に伝えました`);
    readCount();
  } catch (e) {
    if (e instanceof Refusal && e.status === 422) {
      // 1 人でも拒めば、誰にも送っていない (brief 2)。シートは閉じない。
      note.value = `誰にも送っていません。${problemText(e, '')}`;
    } else if (e instanceof Refusal && e.status) {
      note.value = `誰にも送っていません。${problemText(e, e.text || '')}`;
    } else {
      note.value = '送れませんでした。';
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
