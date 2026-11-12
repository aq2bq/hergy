import { term } from './terms.generated.js';
import type { DangerDecision } from './danger.js';
// undo.tsx — すぐ実行した操作の取り戻し方を、その場に 1 行で示す (d-79e3 の A,
// d-021e). ④ の持ち物の置き場所で、保存していない編集の「元に戻す」が最初の
// 利用者になる。1 つだけ覚え、新しければ置き換える。
//
// 知らせの置き場が描き直しで消えたら（欄や行が作り直されたとき）、帯の下の
// 同じ入れ物へ移す。置き場があるうちは押した場所のまま（d-2cb2）。
import type { JSX } from 'preact';
import { render, signal, computed } from './preact.js';
import { undoFallback } from './dialogs.js';

export type Undoable = { said: string; restore: () => void | Promise<string | null> };
const currentUndo = signal<Undoable | null>(null);
export const undo = computed(() => currentUndo.value ? Object.freeze({ said: currentUndo.value.said }) : null);
let hostEl: HTMLElement | null = null;
// watch looks at the sheet's own drawings while a notice stands: when the host
// is taken away, the notice moves to the shared place below the band.
let watch: MutationObserver | null = null;
function stopWatch(): void {
  if (watch) { watch.disconnect(); watch = null; }
}
// place puts the notice in one host and answers the host it made, so the watch
// can tell its own notice from a newer one.
function place(host: HTMLElement): HTMLElement {
  const one = document.createElement('div');
  one.className = 'undo-host';
  host.appendChild(one);
  hostEl = one;
  render(<UndoNotice />, one);
  return one;
}
// offerUndo shows the one notice in the field that was pressed (d-021e): the
// caller hands the element the 元に戻す stands in, and the notice is drawn there.
export function offerUndo(_decision: DangerDecision, one: Undoable, host?: HTMLElement | null): void {
  currentUndo.value = one;
  stopWatch();
  if (hostEl) { render(null, hostEl); hostEl.remove(); hostEl = null; }
  if (!host) return;
  const mine = place(host);
  if (typeof MutationObserver !== 'function' || typeof document === 'undefined' || !document.body) return;
  watch = new MutationObserver(() => {
    if (hostEl !== mine || mine.isConnected) return;
    stopWatch();
    const there = undoFallback();
    if (there) place(there);
  });
  watch.observe(document.body, { childList: true, subtree: true });
}
// takeUndo takes the one offer back. A restore that has to ask the server
// answers with the sentence it was refused with (or empty when it went
// through): the notice then says why it could not go back, instead of closing
// as though it had (r-ccf8, d-2cce).
export function takeUndo(): void {
  const one = currentUndo.value;
  if (!one) return;
  currentUndo.value = null;
  stopWatch();
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
  stopWatch();
  if (hostEl === host) hostEl = null;
  if (host) { render(null, host); host.remove(); }
}
// dropUndo closes the notice without taking anything back: the operation it
// offered is already undone, or a newer one stands in its place (④ 段 4b-2).
export function dropUndo(): void {
  const host = hostEl;
  hostEl = null;
  stopWatch();
  if (host) { render(null, host); host.remove(); }
}
function UndoNotice(): JSX.Element | null {
  const one = currentUndo.value;
  if (!one) return null;
  return <div class="undo" role="status"><span>{one.said}</span><button type="button" class="btn sm" onClick={takeUndo}>{term('undo')}</button></div>;
}
