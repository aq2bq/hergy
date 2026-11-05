// undo.tsx — すぐ実行した操作の取り戻し方を、その場に 1 行で示す (d-79e3 の A,
// d-021e). ④ の持ち物の置き場所で、保存していない編集の「元に戻す」が最初の
// 利用者になる。1 つだけ覚え、新しければ置き換える。
import type { JSX } from 'preact';
import { render, signal } from './preact.js';

export type Undoable = { said: string; restore: () => void };
export const undo = signal<Undoable | null>(null);
export function offerUndo(one: Undoable): void { undo.value = one; }
export function takeUndo(): void {
  const one = undo.value;
  undo.value = null;
  if (one) one.restore();
}
export function UndoNotice(): JSX.Element | null {
  const one = undo.value;
  if (!one) return null;
  return <div class="undo" role="status"><span>{one.said} を取り消す</span><button type="button" class="btn sm" onClick={takeUndo}>取り消す</button></div>;
}

// mountUndo puts the one notice under the band, where every screen draws it:
// the page mounts it once through nav.js, not a screen or a tab (r-1e9a,
// the same lesson as the confirmation's one place in r-ccf8).
let mounted = false;
export function mountUndo(): void {
  if (mounted || typeof document === 'undefined') return;
  const host = document.querySelector('.bar') || document.body;
  const el = document.createElement('div');
  el.className = 'undo-host';
  host.appendChild(el);
  render(<UndoNotice />, el);
  mounted = true;
}
