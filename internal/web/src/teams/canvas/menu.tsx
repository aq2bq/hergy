// teams/canvas/menu.tsx — the blank space's menu（モック 2026-10-23）。
//
// A double press on the canvas opens the two ways of adding at that place: a
// member, or a worker under one member.
import { useLayoutEffect, useRef } from '../../lib/preact.js';
import type { JSX } from 'preact';
import { rolesIn } from '../state.js';
import type { Rec } from '../state.js';
import { menuOf } from './shared.js';

export function Menu({ r }: { r: Rec }): JSX.Element | null {
  const box = useRef<HTMLDivElement>(null);
  const m = menuOf(r);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !m) return;
    const cv = el.closest('.cv-canvas');
    if (!cv) return;
    const cr = cv.getBoundingClientRect();
    const w = el.offsetWidth, h = el.offsetHeight;
    el.style.left = `${Math.max(6, Math.min(m.x, cr.width - w - 6))}px`;
    el.style.top = `${Math.max(6, Math.min(m.y, cr.height - h - 6))}px`;
    const first = el.querySelector<HTMLElement>('button');
    if (first) first.focus();
  });
  if (!m) return null;
  const t = r.draft.value;
  return (
    <div class="cv-menu" role="menu" aria-label={m.workersOnly ? 'ワーカーを足す' : 'ここに足す'} ref={box} style={{ left: `${m.x}px`, top: `${m.y}px` }}>
      {m.workersOnly ? '' : <button type="button" role="menuitem" data-cv-act="menu-m" disabled={!rolesIn()}>メンバーを足す</button>}
      <div class={'cv-mh' + (m.workersOnly ? ' cv-mh0' : '')}>ワーカーを足す（どのメンバーの？）</div>
      <div class="cv-mw">{t.members.map((x) => <button type="button" role="menuitem" data-cv-act="menu-w" data-from={x.name}>{x.name}</button>)}</div>
    </div>
  );
}
