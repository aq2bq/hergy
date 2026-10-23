// measure.js — the one size bar the role list and the team list show (d-00a9,
// d-b87a). A document's own size in characters against the provisional budget:
// the bar as the roles list has drawn it, the number as 1,234 / 3,000 字, and
// the warning colour once the document is over the budget.
window.hergyMeasure = (() => {
  'use strict';

  const num = (n) => Number(n || 0).toLocaleString('ja-JP');

  function html(chars, limit) {
    const over = chars > limit;
    const w = Math.min(100, chars / (limit * 1.25) * 100);
    return `<span class="amt ${over ? 'over' : ''}"><span class="meter ${over ? 'over' : ''}"><i data-w="${w}"></i></span><span class="v">${num(chars)}<span class="lim"> / ${num(limit)} 字</span></span></span>`;
  }

  return { html, num };
})();
