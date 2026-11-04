import { signal } from './preact.js';
export const widths = signal<Record<string, string>>({});
export function WidthButtons({ id }: { id: string }) {
 return <span class="prose-width" role="group" aria-label="欄の幅">
  {(['narrow', 'wide', ''] as const).map((value, i) => {
   const label = ['狭く', '広く', '均等'][i];
   return <button type="button" class="btn sm" title={label} aria-label={label} aria-pressed={value ? widths.value[id] === value : undefined} disabled={!value && !Object.values(widths.value).some(Boolean)} onClick={() => { widths.value = value ? { ...widths.value, [id]: widths.value[id] === value ? '' : value } : {}; }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d={i === 0 ? 'M3 4v16M21 4v16M6 12h12M9 9l3 3-3 3M15 9l-3 3 3 3' : i === 1 ? 'M3 4v16M21 4v16M6 12h12M9 9l-3 3 3 3M15 9l3 3-3 3' : 'M3 4v16M9 4v16M15 4v16M21 4v16'} /></svg></button>;
  })}
 </span>;
}
