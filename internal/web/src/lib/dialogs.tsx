// dialogs.tsx — the one confirmation the screens share (r-ccf8, d-aeeb): the
// band's page mounts it once for every screen, never a screen or a tab.
import type { JSX } from 'preact';
import { render } from './preact.js';
import { ConfirmDialog } from './confirm.js';

export function Dialogs(): JSX.Element { return <ConfirmDialog />; }

let mounted = false;
export function mountDialogs(): void {
  if (mounted || typeof document === 'undefined') return;
  const host = document.createElement('div');
  host.className = 'dialogs-host';
  (document.querySelector('.bar') || document.body).appendChild(host);
  render(<Dialogs />, host);
  mounted = true;
}
