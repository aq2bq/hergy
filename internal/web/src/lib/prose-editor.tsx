import { Field } from './field.js';
import { useState } from './preact.js';

export const chars = (text: string): number => Array.from(text.replace(/\r\n/g, '\n')).length;

export function title(text: string, fallback = ''): string {
  const line = text.split('\n')[0].trim();
  return line.replace(/^#{1,6}(?:[ \t]+|$)/, '').trim() || fallback;
}

// Heading offsets only support editing and counting; prose has no semantic IDs.
export function blocks(text: string): { heading: string; start: number; end: number }[] {
  const out: { heading: string; start: number; end: number }[] = [];
  let offset = 0;
  let fence = '';
  for (const line of text.split('\n')) {
    const marker = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) fence = '';
    } else if (!fence) {
      const heading = line.match(/^\s{0,3}#{1,6}(?:\s+|$)(.*)/);
      if (heading) out.push({ heading: heading[1], start: offset, end: text.length });
    }
    offset += line.length + 1;
  }
  if (!out.length || out[0].start !== 0) out.unshift({ heading: '本文', start: 0, end: text.length });
  out.forEach((block, i) => { block.end = out[i + 1]?.start ?? text.length; });
  return out;
}

export function ProseEditor({ text, budget = true, onChange }: { text: string; budget?: boolean; onChange: (text: string) => void }) {
  const [outline, setOutline] = useState(false);
  const [cursor, setCursor] = useState(0);
  const parts = blocks(text);
  const current = parts.find((part, i) => cursor >= part.start && (cursor < part.end || i === parts.length - 1));
  const count = chars(text);
  return <div class="prose-editor">
    <div class="prose-tools"><span class={budget && count > 1200 ? 'sev warn' : ''}>{budget ? `${count.toLocaleString()} / 1,200 字` : `散文 ${count.toLocaleString()} 字`}</span>
      <button type="button" class="btn sm" title={outline ? '全文を編集する' : '見出しだけを表示する'} aria-label={outline ? '全文を編集する' : '見出しだけを表示する'} aria-pressed={outline} onClick={() => setOutline(!outline)}>{outline ? '✎' : '☷'}</button></div>
    {outline ? <ul class="prose-outline">{parts.map((part) => <li><span>{part.heading}</span><span>{chars(text.slice(part.start, part.end))} 字</span></li>)}</ul>
      : <Field as="textarea" aria-label="散文の全文" value={text} spellcheck={false} onInput={(event: Event) => { const el = event.currentTarget; if (!(el instanceof HTMLTextAreaElement)) return; setCursor(el.selectionStart); onChange(el.value); }} onSelect={(event: Event) => { if (event.currentTarget instanceof HTMLTextAreaElement) setCursor(event.currentTarget.selectionStart); }} />}
    {!outline && current ? <p class="prose-current">{current.heading} · {chars(text.slice(current.start, current.end))} 字</p> : null}
  </div>;
}
