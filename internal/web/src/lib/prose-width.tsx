import { signal } from './preact.js';
export const widths = signal<Record<string, string>>({});
export function EqualWidths() {
 return <button type="button" class="btn sm prose-equal" title="幅をそろえる（すべての欄を元の等しい幅に戻す）" aria-label="均等" disabled={!Object.values(widths.value).some(Boolean)} onClick={() => { widths.value = {}; }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 4v16M9 4v16M15 4v16M21 4v16" /></svg></button>;
}
export function WidthButtons({ id }: { id: string }) {
 return <span class="prose-width" role="group" aria-label="欄の幅">
  {(['narrow', 'wide'] as const).map((value, i) => {
   const label = i === 0 ? '狭く（見出しだけにする）' : '広く';
   return <button type="button" class="btn sm" title={label} aria-label={label} aria-pressed={widths.value[id] === value} onClick={() => { widths.value = { ...widths.value, [id]: widths.value[id] === value ? '' : value }; }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d={i === 0 ? 'M9 4v16M15 4v16M1 12h6m-3-3 3 3-3 3M23 12h-6m3-3-3 3 3 3' : 'M8 3H3v5M16 3h5v5M3 16v5h5M21 16v5h-5M9 9 3 3M15 9l6-6M9 15l-6 6M15 15l6 6'} /></svg></button>;
  })}
 </span>;
}
