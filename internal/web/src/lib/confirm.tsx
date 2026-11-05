import type { ComponentChildren, JSX } from 'preact';
import { useEffect, useRef } from './preact.js';
import { signal } from './preact.js';

// The one confirmation every screen uses before something that cannot be taken
// back (r-d480, d-021e): a modal dialog away from the button that asked, with
// 「やめる」 focused first, so a second press of the same place and a second
// Enter never run it. Esc and 「やめる」 close it with nothing done, and a press
// on the backdrop does not close it.
export type ConfirmAsk = {
  title: string;
  body: ComponentChildren;
  label: string;
  run: () => void | Promise<void>;
};

export const confirmAsk = signal<ConfirmAsk | null>(null);
export const askConfirm = (ask: ConfirmAsk): void => { confirmAsk.value = ask; };
export const closeConfirm = (): void => { confirmAsk.value = null; };

export function ConfirmDialog(): JSX.Element | null {
  const ask = confirmAsk.value;
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const node = dialog.current;
    if (!ask || !node) return;
    if (!node.open) node.showModal();
    cancel.current?.focus();
    // Nothing closes it here: an ask that goes away removes the element, and a
    // re-render of the screen must not shut a dialog the person is reading
    // (r-d480).
    return undefined;
  }, [ask]);
  if (!ask) return null;
  return <dialog class="confirm" ref={dialog} aria-labelledby="confirm-title"
    onCancel={(event) => { event.preventDefault(); closeConfirm(); }}>
    <h2 id="confirm-title">{ask.title}</h2>
    <div class="confirm-body">{ask.body}</div>
    <footer>
      <button class="btn" type="button" ref={cancel} onClick={closeConfirm}>やめる</button>
      <button class="btn danger solid" type="button" onClick={() => { const run = ask.run; closeConfirm(); void run(); }}>{ask.label}</button>
    </footer>
  </dialog>;
}
