import { Field } from './field.js';
import { useState, useRef } from './preact.js';
import type { ComponentChildren } from 'preact';
import { Measure, num } from './measure.js';

import { blocks } from './prose-blocks.js';
import { TransferMenu, TransferGrip, dropTransfer } from './prose-transfer-view.js';
import type { TransferSource } from './prose-transfer.js';
import { WidthButtons, widths } from './prose-width.js';
export { blocks } from './prose-blocks.js';

export const chars = (text: string): number => Array.from(text.replace(/\r\n/g, '\n')).length;

export function title(text: string, fallback = ''): string {
  const line = text.split('\n')[0].trim();
  return line.replace(/^#{1,6}(?:[ \t]+|$)/, '').trim() || fallback;
}

// Heading offsets only support editing and counting; prose has no semantic IDs.



export type ProseHeading = { kind: string; name: string; title?: string; path?: string; members?: string; template?: number; state?: string; id?: string; compact?: boolean; fullPath?: string };
export function ProseIdentity({ heading }: { heading: ProseHeading }) {
 return <h2 data-width={widths.value[heading.id || '']} id={heading.id} tabIndex={-1}><span class="prose-kind">{heading.kind}</span><span class="prose-label" title={heading.name}>{heading.name}</span>{heading.id ? <WidthButtons id={heading.id} /> : null}</h2>;
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

export function ProseEditor({ text, heading, onChange, transfer }: { text: string; heading: ProseHeading; onChange: (text: string) => void; transfer?: TransferSource }) {
 const [outline, setOutline] = useState(false), [cursor, setCursor] = useState(0), [focused, setFocused] = useState(false);
 const parts = blocks(text);
 const narrow = widths.value[heading.id || ''] === 'narrow', showOutline = outline || narrow;
 const input = useRef<HTMLTextAreaElement>(null);
 const [menu, setMenu] = useState<{index: number; target?: string; at?: number; move?: boolean; children?: boolean} | null>(null);
 const current = parts.find((part, i) => cursor >= part.start && (cursor < part.end || i === parts.length - 1));
 const position = (event: Event) => { if (event.currentTarget instanceof HTMLTextAreaElement) setCursor(event.currentTarget.selectionStart); };
 return <div class="prose-editor" onDragOver={(e) => { if (transfer) e.preventDefault(); }} onDrop={(e) => { if (transfer) dropTransfer(e, transfer.id, -1); }}>
 {menu && transfer ? <TransferMenu source={transfer} index={menu.index} target={menu.target} at={menu.at} move={menu.move} children={menu.children} close={() => setMenu(null)} /> : null}
  <ProseHeader heading={heading} text={text} position={focused && !showOutline && text && current ? `入力位置: ${current.heading}（${num(chars(text.slice(current.start, current.end)))} 字）` : undefined}><div class="prose-modes" role="group" aria-label={`${heading.name} の表示`}>
   <button type="button" class="btn sm" title="全文" aria-label="全文" aria-pressed={!showOutline} onClick={() => { setOutline(false); if (heading.id) widths.value = { ...widths.value, [heading.id]: '' }; }}><svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="3" width="14" height="18" rx="2"/><path d="M8 7h8M8 11h8M8 15h5"/></svg></button>
   <button type="button" class="btn sm" title="見出しだけ" aria-label="見出しだけ" aria-pressed={showOutline} onClick={() => { setOutline(true); setFocused(false); }}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5h13M8 12h13M8 19h13"/><circle cx="3" cy="5" r="1"/><circle cx="3" cy="12" r="1"/><circle cx="3" cy="19" r="1"/></svg></button>
  </div></ProseHeader>
  {showOutline ? <ul class="prose-outline" aria-label={`${heading.name} の見出し`}>{parts.map((part, index) => <li onDragOver={(e) => { if (transfer) e.preventDefault(); }} onDrop={(e) => { if (transfer) dropTransfer(e, transfer.id, part.start); }} style={{ paddingLeft: `${Math.max(0, part.depth - 1) * (narrow ? 2 : 14)}px` }}><span class="prose-block-name">{transfer ? <TransferGrip source={transfer} index={index} open={(choice) => setMenu({ index, ...choice })} /> : null}<button type="button" class="prose-heading-link" title={part.heading} onClick={() => { setOutline(false); if (heading.id) widths.value = { ...widths.value, [heading.id]: '' }; requestAnimationFrame(() => { input.current?.focus(); input.current?.setSelectionRange(part.start, part.start); }); }}>{part.heading}</button></span><span>{num(chars(text.slice(part.start, part.end)))} 字</span>{transfer ? <button type="button" class="prose-more" title={`${part.heading} を写す・移す`} aria-label={`${part.heading} を写す・移す`} onClick={() => setMenu({ index })}>⋯</button> : null}</li>)}</ul>
   : <Field as="textarea" fieldRef={input} aria-label={`${heading.name} の規則の全文`} placeholder={heading.kind === 'チーム定義' ? 'チームの規則は任意です。本文を入力して保存できます。' : '本文を入力してください。'} value={text} spellcheck={false}
     onInput={(event: Event) => { position(event); if (event.currentTarget instanceof HTMLTextAreaElement) onChange(event.currentTarget.value); }}
     onSelect={position} onKeyUp={position} onClick={position} onFocus={(event: Event) => { setFocused(true); position(event); }} onBlur={() => setFocused(false)} />}
 </div>;
}
