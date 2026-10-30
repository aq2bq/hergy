// band.ts — 行を帯の下に出す（d-943f、r-e39c、d-022a）。
//
// 開いた行が画面の外にできると、押した人は何ができたかを見られない。だから
// 行を開く押し込み（コピー）は、行の上端を帯のすぐ下へ運ぶ。行の頭ではなく行
// そのものの上端を運ぶのは、開いた行の頭が帯に貼り付く画面（チーム定義）で、
// ブラウザのスクロールの固定がほかの開いた行を残したあと、頭が帯の所に立ち
// 行が画面の上に出てしまうから（r-410c）。
export function bringUnderBand(row: HTMLElement | null): void {
  if (!row || !row.querySelector(':scope > .sum')) return;
  const stick = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--stick')) || 62;
  const delta = row.getBoundingClientRect().top - stick;
  if (Math.abs(delta) < 2) return;
  const quiet = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  window.scrollBy({ top: delta, behavior: quiet ? 'auto' : 'smooth' });
}
