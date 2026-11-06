// danger.ts — 危ない操作の振り分けの持ち主 (r-ccf8 / d-021e / d-2cce / d-aeeb).
//
// 取り返せない操作は ConfirmDialog で確かめ、取り返せる操作はすぐ実行して
// その場に取り戻し方を出す。どちらでもない操作（起動をやめる）はそのまま。
// 決まりは d-021e（取り消せず何かを失う操作は確かめる、取り消せる操作はすぐ
// 実行して取り消し方を示す）。振り分けの表はここ 1 つだけ。
import { askConfirm } from './confirm.js';
import type { ConfirmAsk } from './confirm.js';
import { offerUndo } from './undo.js';
import { h } from './preact.js';
import { read } from './api.js';

export type DangerAction =
  | 'deleteTeam' | 'deleteRole' | 'deleteProfile' | 'stopTeam' | 'discardRow'
  | 'unlistProject' | 'revertDraft' | 'removeMember' | 'removeWorker' | 'removeTurn' | 'conflictRevert'
  | 'leaveWithDraft' | 'discardRoleEdit'
  | 'abortStart';

// DANGER is the one table: what a dangerous action does before it runs.
export const DANGER: Record<DangerAction, 'confirm' | 'undoable' | 'none'> = {
  deleteTeam: 'confirm', deleteRole: 'confirm', deleteProfile: 'confirm', stopTeam: 'confirm',
  unlistProject: 'undoable', revertDraft: 'undoable', removeMember: 'undoable', removeWorker: 'undoable',
  removeTurn: 'undoable', conflictRevert: 'undoable',
  leaveWithDraft: 'confirm', discardRoleEdit: 'confirm',
  discardRow: 'confirm',
  abortStart: 'none',
};

export type DangerOptions = {
  run: () => void | Promise<void>;
  // confirm actions: the words the person reads (called only for 'confirm').
  confirm?: { title: string; label: string; body?: unknown };
  // undoable actions: what to put back and where the notice stands.
  host?: HTMLElement | null;
  restore?: () => void;
  said?: string;
  // loss names the thing whose loss the server answers; the confirmation's body
  // is the server's lines, not the screen's words (r-ccf8, ac-ddb6).
  loss?: { kind: 'team' | 'role' | 'profile' | 'activity'; name: string; place?: string; project?: string };
  // lost is what the owner says a discard throws away, in words (changed.lost).
  lost?: string[];
  // cancel is what the asker takes back after やめる (a focus that goes back).
  cancel?: () => void;
};

// runDanger asks the table and does the one thing it says.
export function runDanger(action: DangerAction, opts: DangerOptions): void {
  const kind = DANGER[action];
  if (kind === 'none') { void opts.run(); return; }
  if (kind === 'undoable') {
    const offer = () => { if (opts.restore) offerUndo({ said: opts.said || '実行しました', restore: opts.restore }, opts.host); };
    const done = opts.run();
    if (done && typeof (done as Promise<unknown>).then === 'function') void (done as Promise<unknown>).then(offer);
    else offer();
    return;
  }
  if (!opts.confirm) return;
  const ask = { ...opts.confirm, cancel: opts.cancel };
  if (opts.loss) {
    const loss = opts.loss;
    const q = new URLSearchParams();
    if (loss.place) q.set('place', loss.place);
    if (loss.project) q.set('project', loss.project);
    const url = `/api/danger/${encodeURIComponent(loss.kind)}/${encodeURIComponent(loss.name)}${q.toString() ? `?${q}` : ''}`;
    void read<{ lines: string[] }>(url).then((answer) => {
      const lines = (answer && answer.lines) || [];
      askConfirm({ ...ask, body: h('div', {}, lines.map((line) => h('p', {}, line))), run: opts.run } as ConfirmAsk);
    }).catch((err: unknown) => {
      askConfirm({ ...ask, body: h('p', { role: 'alert' }, `消えるものを読めませんでした: ${String(err)}`), run: () => undefined } as ConfirmAsk);
    });
    return;
  }
  if (opts.lost) {
    askConfirm({ ...ask, body: h('div', {}, [...opts.lost.map((one) => h('p', {}, one)), h('p', {}, h('b', {}, '取り消せません。'))]), run: opts.run } as ConfirmAsk);
    return;
  }
  askConfirm({ ...ask, body: ask.body ?? h('div', {}), run: opts.run } as ConfirmAsk);
}
