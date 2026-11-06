// undo.tsx — すぐ実行した操作の取り戻し方を、その場に 1 行で示す (d-79e3 の A,
// d-021e). ④ の持ち物の置き場所で、保存していない編集の「元に戻す」が最初の
// 利用者になる。1 つだけ覚え、新しければ置き換える。
import type { JSX } from 'preact';
import { render, signal } from './preact.js';

export type Undoable = { said: string; restore: () => void | Promise<string | null> };
export const undo = signal<Undoable | null>(null);
let hostEl: HTMLElement | null = null;
// offerUndo shows the one notice in the field that was pressed (d-021e): the
// caller hands the element the 元に戻す stands in, and the notice is drawn there.
export function offerUndo(one: Undoable, host?: HTMLElement | null): void {
  undo.value = one;
  if (hostEl) { render(null, hostEl); hostEl.remove(); hostEl = null; }
  if (!host) return;
  hostEl = document.createElement('div');
  hostEl.className = 'undo-host';
  host.appendChild(hostEl);
  render(<UndoNotice />, hostEl);
}
// takeUndo takes the one offer back. A restore that has to ask the server
// answers with the sentence it was refused with (or empty when it went
// through): the notice then says why it could not go back, instead of closing
// as though it had (r-ccf8, d-2cce).
export function takeUndo(): void {
  const one = undo.value;
  if (!one) return;
  undo.value = null;
  const host = hostEl;
  const done = one.restore();
  if (!done || typeof (done as Promise<unknown>).then !== 'function') { closeHost(host); return; }
  void (done as Promise<string | null>).then((no) => {
    if (no && host && host.isConnected) {
      if (hostEl === host) render(<div class="undo" role="alert"><span>{no}</span></div>, host);
      return;
    }
    closeHost(host);
  });
}
function closeHost(host: HTMLElement | null): void {
  if (hostEl === host) hostEl = null;
  if (host) { render(null, host); host.remove(); }
}
// dropUndo closes the notice without taking anything back: the operation it
// offered is already undone, or a newer one stands in its place (④ 段 4b-2).
export function dropUndo(): void {
  const host = hostEl;
  hostEl = null;
  if (host) { render(null, host); host.remove(); }
}
export function UndoNotice(): JSX.Element | null {
  const one = undo.value;
  if (!one) return null;
  return <div class="undo" role="status"><span>{one.said}</span><button type="button" class="btn sm" onClick={takeUndo}>取り消す</button></div>;
}

