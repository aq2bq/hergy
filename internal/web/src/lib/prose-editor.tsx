import { Field } from './field.js';
import { useState } from './preact.js';
import type { ComponentChildren } from 'preact';
import { Measure, num } from './measure.js';

export const chars = (text: string): number => Array.from(text.replace(/\r\n/g, '\n')).length;

export function title(text: string, fallback = ''): string {
  const line = text.split('\n')[0].trim();
  return line.replace(/^#{1,6}(?:[ \t]+|$)/, '').trim() || fallback;
}

// Heading offsets only support editing and counting; prose has no semantic IDs.
export function blocks(text: string): { heading: string; depth: number; start: number; end: number }[] {
  const out: { heading: string; depth: number; start: number; end: number }[] = [];
  let offset = 0;
  let fence = '';
  for (const line of text.split('\n')) {
    const marker = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) fence = '';
    } else if (!fence) {
      const heading = line.match(/^\s{0,3}(#{1,6})(?:\s+|$)(.*)/);
      if (heading) out.push({ heading: heading[2], depth: heading[1].length, start: offset, end: text.length });
    }
    offset += line.length + 1;
  }
  if (!out.length || out[0].start !== 0) out.unshift({ heading: '本文', depth: 1, start: 0, end: text.length });
  out.forEach((block, i) => { block.end = out[i + 1]?.start ?? text.length; });
  return out;
}


export type ProseHeading = { kind: string; name: string; title?: string; path?: string; members?: string; template?: number; state?: string; id?: string; compact?: boolean; fullPath?: string };
export function ProseIdentity({ heading }: { heading: ProseHeading }) {
 return <h2 id={heading.id} tabIndex={-1}><span class="prose-kind">{heading.kind}</span><span>{heading.name}</span></h2>;
}
export function ProseHeader({ heading, text, children, position }: { heading: ProseHeading; text: string; children?: ComponentChildren; position?: string }) {
 const count = chars(text), template = heading.template || 0, total = count + template;
 return <header class="prose-head">
  {!heading.compact ? <ProseIdentity heading={heading} /> : null}
  <p class="prose-title" title={heading.title}>{heading.title && heading.title !== heading.name ? heading.title : '題なし'}</p>
  <code class="path" title={heading.fullPath || heading.path}>{heading.path || '保存先を確認しています'}</code>
  {!heading.compact ? <p class="prose-readers" title={heading.members}>{heading.members ? `使うメンバー: ${heading.members}` : '使うメンバーなし'}</p> : null}
  <div class="prose-budget" title={template ? `内訳: 規則 ${num(count)} 字・ひな形 ${num(template)} 字` : undefined}><Measure chars={total} limit={1200} /><span class={total > 1200 ? 'over' : ''}>{total > 1200 ? `${num(total - 1200)} 字超過` : `残り ${num(1200 - total)} 字`}</span></div>
  <div class="prose-status"><span class={position ? 'prose-current' : undefined} title={position} aria-live="polite">{position || heading.state || '保存済み'}</span>{children}</div>
 </header>;
}

export function ProseEditor({ text, heading, onChange }: { text: string; heading: ProseHeading; onChange: (text: string) => void }) {
 const [outline, setOutline] = useState(false), [cursor, setCursor] = useState(0), [focused, setFocused] = useState(false);
 const parts = blocks(text);
 const current = parts.find((part, i) => cursor >= part.start && (cursor < part.end || i === parts.length - 1));
 const position = (event: Event) => { if (event.currentTarget instanceof HTMLTextAreaElement) setCursor(event.currentTarget.selectionStart); };
 return <div class="prose-editor">
  <ProseHeader heading={heading} text={text} position={focused && !outline && text && current ? `入力位置: ${current.heading}（${num(chars(text.slice(current.start, current.end)))} 字）` : undefined}><div class="prose-modes" role="group" aria-label={`${heading.name} の表示`}>
   <button type="button" class="btn sm" title="全文" aria-label="全文" aria-pressed={!outline} onClick={() => setOutline(false)}><svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="3" width="14" height="18" rx="2"/><path d="M8 7h8M8 11h8M8 15h5"/></svg></button>
   <button type="button" class="btn sm" title="見出しだけ" aria-label="見出しだけ" aria-pressed={outline} onClick={() => { setOutline(true); setFocused(false); }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5h13M8 12h13M8 19h13"/><circle cx="3" cy="5" r="1"/><circle cx="3" cy="12" r="1"/><circle cx="3" cy="19" r="1"/></svg></button>
  </div></ProseHeader>
  {outline ? <ul class="prose-outline" aria-label={`${heading.name} の見出し`}>{parts.map((part) => <li style={{ paddingLeft: `${(part.depth - 1) * 14}px` }}><span>{part.heading}</span><span>{num(chars(text.slice(part.start, part.end)))} 字</span></li>)}</ul>
   : <Field as="textarea" aria-label={`${heading.name} の規則の全文`} placeholder={heading.kind === 'チーム定義' ? 'チームの規則は任意です。本文を入力して保存できます。' : '本文を入力してください。'} value={text} spellcheck={false}
     onInput={(event: Event) => { position(event); if (event.currentTarget instanceof HTMLTextAreaElement) onChange(event.currentTarget.value); }}
     onSelect={position} onKeyUp={position} onClick={position} onFocus={(event: Event) => { setFocused(true); position(event); }} onBlur={() => setFocused(false)} />}
 </div>;
}
