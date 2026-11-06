// page-position.ts — ページの位置とスクロールの持ち主 (r-6ff8 / d-c5ce / d-656c / d-a9ed / d-022a / d-fe34).
//
// hergy がページ (window) を動かすのは、利用者が行き先を名指しした操作と、
// 操作の結果が画面の外にできたときだけ。表示の切り替え・読み直し・答えが届く・
// フィルタ・明暗では動かさない (n-83f0, d-c5ce)。どちらかは下の PAGE_POSITION の表 1 つ
// だけが持ち、画面は movePage(act, el) を呼ぶ。'stay' の操作では movePage は
// 何もしない。window の scrollBy・scrollTo・scrollIntoView はこのファイルの外に
// 置かない (tools/check/owners.go)。
//
// 行き先の出し方は 2 つ。行は上端を帯のすぐ下へ (d-943f, r-410c, d-022a)、
// それ以外の要素は見えている範囲へ (nearest / center)。
//
// ホイール (d-656c, d-a1e1)。フォーカスの無い textarea の上の縦のホイールは
// ページへ送る。dialog の中では、ページの代わりに本文 (いちばん近い、スクロール
// する祖先) へ送る。Ctrl/⌘ と横のホイールはブラウザに任せる。
//
// 欄の中のスクロール (scrollTop / scrollLeft) は、その欄の持ち主の中だけで動かす。
// どの欄かは tools/check/owners.yml の許可リストに理由と決定を添えて載せる。
// ページの位置を保つための高さ (d-fe34 の .dbody) は器の持ち主 (画面) が持つ。
// page-position.ts は高さを書かない。

// 操作ごとの行き先。'stay' はページを動かさない。'band' は行の上端を帯の
// すぐ下へ、'nearest' | 'center' はその要素を画面の中へ。
export type PageAt = 'band' | 'nearest' | 'center' | 'stay';
export type PageAction =
  | 'copyRow' | 'newRow' | 'dupRow' | 'editSelection'
  | 'waitingRow' | 'assignPerson' | 'personFromURL' | 'openByURL'
  | 'openRow' | 'switchTab' | 'switchDoc' | 'switchView'
  | 'problemLine' | 'filter' | 'reread' | 'saveAnswer' | 'theme' | 'select';

// PAGE_POSITION は 1 つの表。どこを動かすか、なぜか、どの決定か。
export const PAGE_POSITION: Record<PageAction, { at: PageAt; why: string; d: string }> = {
  copyRow: { at: 'band', why: 'コピーは押した行と別の置き場所にできる。行を画面の中に出す', d: 'd-022a' },
  newRow: { at: 'band', why: '新しく作った行が画面の外にできる', d: 'd-c5ce' },
  dupRow: { at: 'band', why: '複製した行が画面の外にできる', d: 'd-c5ce' },
  editSelection: { at: 'band', why: '利用者が「選択した属性を編集」で行き先を名指しした', d: 'd-c5ce' },
  waitingRow: { at: 'nearest', why: '待っている行から、そのチームの札へ行く', d: 'd-c5ce' },
  assignPerson: { at: 'center', why: '人の札から、その人の割り当ての行へ行く', d: 'd-c5ce' },
  personFromURL: { at: 'center', why: 'URL が人を名指しした (#team.member)', d: 'd-c5ce' },
  openByURL: { at: 'band', why: 'URL が行・札を名指しした', d: 'd-c5ce' },
  openRow: { at: 'stay', why: '行を開くのは表示の切り替え', d: 'd-2b33' },
  switchTab: { at: 'stay', why: 'タブ (トポロジー／規則、一覧／YAML／割り当て) は表示の切り替え', d: 'd-fe34・d-6fab' },
  switchDoc: { at: 'stay', why: '並べる・文書名・すべて表示は表示の切り替え', d: 'ac-c66a' },
  switchView: { at: 'stay', why: '全文・今の文・差。欄の中だけ先頭へ戻す (d-a9ed)', d: 'd-a9ed' },
  problemLine: { at: 'stay', why: '行き先は欄の中。欄の持ち主が欄の中だけ動かす', d: 'd-c5ce' },
  filter: { at: 'stay', why: '絞り込み・探すことは表示の切り替え', d: 'd-c5ce' },
  reread: { at: 'stay', why: '読み直し・検査の答え・保存の答え・10 秒の読み', d: 'd-c5ce' },
  saveAnswer: { at: 'stay', why: '保存の答えは表示の切り替え', d: 'd-c5ce' },
  theme: { at: 'stay', why: '明暗の切り替えは表示の切り替え', d: 'd-c5ce' },
  select: { at: 'stay', why: '選択と右の列 (⑥) は表示の切り替え', d: 'd-c5ce' },
};

// movePage は表を引いて、'stay' なら何もしない。
export function movePage(act: PageAction, el: Element | null, opts?: { extra?: number; instant?: boolean }): void {
  const at = PAGE_POSITION[act].at;
  if (at === 'stay' || !el) return;
  const quiet = !!opts?.instant || (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  if (at === 'band') {
    const stick = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--stick')) || 62;
    const delta = el.getBoundingClientRect().top - stick - (opts?.extra ?? 0);
    if (Math.abs(delta) < 2) return;
    window.scrollBy({ top: delta, behavior: quiet ? 'auto' : 'smooth' });
    return;
  }
  el.scrollIntoView({ block: at, behavior: quiet ? 'auto' : 'smooth' });
}

// r-ddb7 (d-656c): the wheel over an input nobody is typing in sends the page,
// not the input. Only an unfocused textarea's vertical wheel is taken; a focused
// input keeps its own scrolling, and Ctrl/⌘ (zoom, pinch) and horizontal wheels
// are left to the browser.
const LINE = 16;

// Inside a modal the body of the modal takes the place of the page; outside it,
// the page itself is what the wheel sends, whatever scrollers stand around the
// input (r-ddb7, ac-6dec).
function scrollHostOf(target: HTMLElement): HTMLElement | null {
  const dialog = target.closest<HTMLElement>('dialog[open]');
  if (!dialog) return null;
  for (let node: HTMLElement | null = target.parentElement; node; node = node.parentElement) {
    const style = getComputedStyle(node);
    if ((style.overflowY === 'auto' || style.overflowY === 'scroll') && node.scrollHeight > node.clientHeight) return node;
    if (node === dialog) return dialog;
  }
  return dialog;
}

// One wheel's amount in pixels: a line wheel is a line, and a page wheel is the
// height of what it moves, the way the browser's own scroll is (lead's condition 2).
function pixels(event: WheelEvent, height: number): number {
  if (event.deltaMode === 1) return event.deltaY * LINE;
  if (event.deltaMode === 2) return event.deltaY * (height || LINE);
  return event.deltaY;
}

export function installWheel(): void {
  document.addEventListener('wheel', (event) => {
    if (event.ctrlKey || event.metaKey) return;
    if (Math.abs(event.deltaY) < Math.abs(event.deltaX)) return;
    const target = event.target;
    if (!(target instanceof HTMLTextAreaElement)) return;
    if (document.activeElement === target) return;
    event.preventDefault();
    const host = scrollHostOf(target);
    const amount = pixels(event, host ? host.clientHeight : window.innerHeight);
    if (host) host.scrollTop += amount;
    else window.scrollBy(0, amount);
  }, { passive: false, capture: true });
}
