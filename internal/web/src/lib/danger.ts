// danger.ts — 危ない操作の振り分けの持ち主 (r-ccf8 / d-021e / d-2cce / d-aeeb).
//
// 取り返せない操作は ConfirmDialog で確かめ、取り返せる操作はすぐ実行して
// その場に取り戻し方を出す。どちらでもない操作（起動をやめる）はそのまま。
// 決まりは d-021e（取り消せず何かを失う操作は確かめる、取り消せる操作はすぐ
// 実行して取り消し方を示す）。振り分けの表はここ 1 つだけ。
import { askConfirm } from './confirm.js';
import type { ConfirmAsk } from './confirm.js';
import { offerUndo } from './undo.js';

export type DangerAction =
  | 'deleteTeam' | 'deleteRole' | 'deleteProfile' | 'stopTeam'
  | 'unlistProject' | 'revertDraft' | 'removeMember' | 'removeWorker' | 'removeTurn' | 'conflictRevert'
  | 'abortStart';

// DANGER is the one table: what a dangerous action does before it runs.
export const DANGER: Record<DangerAction, 'confirm' | 'undoable' | 'none'> = {
  deleteTeam: 'confirm', deleteRole: 'confirm', deleteProfile: 'confirm', stopTeam: 'confirm',
  unlistProject: 'undoable', revertDraft: 'undoable', removeMember: 'undoable', removeWorker: 'undoable',
  removeTurn: 'undoable', conflictRevert: 'undoable',
  abortStart: 'none',
};

export type DangerOptions = {
  run: () => void | Promise<void>;
  // confirm actions: the words the person reads (called only for 'confirm').
  confirm?: ConfirmAsk;
  // undoable actions: what to put back and where the notice stands.
  host?: HTMLElement | null;
  restore?: () => void;
  said?: string;
};

// runDanger asks the table and does the one thing it says.
export function runDanger(action: DangerAction, opts: DangerOptions): void {
  const kind = DANGER[action];
  if (kind === 'none') { void opts.run(); return; }
  if (kind === 'undoable') {
    void opts.run();
    if (opts.restore) offerUndo({ said: opts.said || '実行しました', restore: opts.restore }, opts.host);
    return;
  }
  if (opts.confirm) askConfirm(opts.confirm);
}
