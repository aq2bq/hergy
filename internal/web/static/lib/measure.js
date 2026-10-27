// lib/measure.js — the one size bar the role list and the team list show
// (d-00a9, d-b87a), as a Preact component since the rewrite
// (webui-preact-design 段階 1-4). A document's own size in characters against
// the provisional budget: the bar the roles list drew, the number as
// 1,234 / 3,000 字, and the warning colour once the document is over it.
//
// The bar's width is set through the style object, not as a style="…" attribute:
// the page's CSP does not take an inline attribute, and a DOM set is not one

//
// It is the one bar the role list and the team list draw.
import { html } from '../vendor/preact.js';

export const num = (n) => Number(n || 0).toLocaleString('ja-JP');

export function Measure({ chars, limit }) {
  const over = chars > limit;
  const w = Math.min(100, (chars / (limit * 1.25)) * 100);
  return html`<span class=${'amt' + (over ? ' over' : '')}><span class=${'meter' + (over ? ' over' : '')}><i data-w=${w} style=${{ width: w + '%' }}></i></span><span class="v">${num(chars)}<span class="lim"> / ${num(limit)} 字</span></span></span>`;
}
