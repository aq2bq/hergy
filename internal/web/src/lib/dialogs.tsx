// dialogs.tsx — the one confirmation the screens share (r-ccf8, d-aeeb): the
// band's page mounts it once for every screen, never a screen or a tab. The
// same place holds a notice that lost the spot it was pressed in, so the notice
// still says what it did and can take it back (d-2cb2).
import type { JSX } from 'preact';
import { render } from './preact.js';
import { confirmView } from './confirm.js';

const dialogHostBrand: unique symbol = Symbol('DialogHost');
export type DialogHost = { readonly [dialogHostBrand]: true };
const host: DialogHost = { [dialogHostBrand]: true };
function Dialogs(): JSX.Element { return confirmView(host); }

// belowBand is one shared host below the band. A host that is gone is made
// again: the band is drawn again when a reading lands, and a guest inside it
// goes with the drawing.
function belowBand(className: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`.${className}`);
  if (found) return found;
  const host = document.createElement('div');
  host.className = className;
  (document.querySelector('.bar') || document.body).appendChild(host);
  return host;
}

// undoFallback is where a notice stands once the field or row it was pressed in
// is drawn again: the same place as the confirmation, never a screen's own.
export function undoFallback(): HTMLElement | null {
  if (typeof document === 'undefined') return null;
  return belowBand('undo-fallback');
}

let mounted = false;
export function mountDialogs(): void {
  if (mounted || typeof document === 'undefined') return;
  render(<Dialogs />, belowBand('dialogs-host'));
  mounted = true;
}
